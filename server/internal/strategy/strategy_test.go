package strategy

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"f1tele/server/internal/analysis"
	"f1tele/server/internal/config"
	"f1tele/server/internal/contract"
	"f1tele/server/internal/history"
	"f1tele/server/internal/state"
)

func testArbiter(t *testing.T) *Arbiter {
	t.Helper()
	t.Setenv("F1TELE_DATA_DIR", t.TempDir())
	cfg, err := config.Defaults()
	if err != nil {
		t.Fatal(err)
	}
	return NewArbiter(cfg.Notifications, cfg.ReferenceProfiles, cfg.CapacityProfiles)
}

func TestBriefSpikeDoesNotAnnounce(t *testing.T) {
	a := testArbiter(t)
	now := time.Unix(1000, 0)
	a.now = func() time.Time { return now }
	snap := hotSnap(106)
	if n := a.Observe(snap); len(n) != 0 {
		t.Fatalf("announced immediately: %#v", n)
	}
	now = now.Add(2 * time.Second)
	if n := a.Observe(snap); len(n) != 0 {
		t.Fatalf("announced before persistence: %#v", n)
	}
	now = now.Add(2 * time.Second)
	if n := a.Observe(snap); len(n) != 1 || !n[0].Spoken {
		t.Fatalf("not announced after persistence: %#v", n)
	}
}

func TestAckSilencesUntilEscalation(t *testing.T) {
	a := testArbiter(t)
	now := time.Unix(1000, 0)
	a.now = func() time.Time { return now }
	snap := hotSnap(106)
	a.Observe(snap)
	now = now.Add(4 * time.Second)
	n := a.Observe(snap)
	if len(n) != 1 {
		t.Fatalf("expected first caution: %#v", n)
	}
	if !a.Acknowledge(n[0].AlertID) {
		t.Fatal("ack failed")
	}
	now = now.Add(40 * time.Second)
	if n := a.Observe(snap); len(n) != 0 {
		t.Fatalf("reminded after ack: %#v", n)
	}
	snap = hotSnap(112)
	if n := a.Observe(snap); len(n) != 1 || !n[0].Critical {
		t.Fatalf("escalation suppressed: %#v", n)
	}
}

func TestCriticalBypassesRoutineGap(t *testing.T) {
	a := testArbiter(t)
	now := time.Unix(1000, 0)
	a.now = func() time.Time { return now }
	a.lastRoutine = now
	snap := hotSnap(120)
	a.Observe(snap)
	now = now.Add(time.Second)
	n := a.Observe(snap)
	if len(n) != 1 || !n[0].Critical {
		t.Fatalf("critical suppressed by routine gap: %#v", n)
	}
}

func TestIndependentWearAndTemperature(t *testing.T) {
	a := testArbiter(t)
	now := time.Unix(1000, 0)
	a.now = func() time.Time { return now }
	snap := hotSnap(106)
	// fresh tyre but hot surface: wear is fine, temperature cautions.
	snap.Metrics[1].Measurement = contract.Available(8.0, "%", "test")
	a.Observe(snap)
	now = now.Add(4 * time.Second)
	n := a.Observe(snap)
	if len(n) != 1 {
		t.Fatalf("temperature not warned independently: %#v", n)
	}
}

func hotSnap(surface float64) state.Snapshot {
	p := 0

	cx := contract.Context{SessionID: "s", BranchID: "b", PlayerCarIndex: &p, RegulationSeason: intp(2025), FormulaCode: intp(0), SeasonEvidence: []string{}}
	return state.Snapshot{Context: cx, ObservationSeq: 10, Metrics: []contract.Metric{
		{ID: "tyre.fl.surface_c@0", Channel: "tyre.fl.surface_c", CarIndex: &p, Measurement: contract.Available(surface, "degC", "test")},
		{ID: "tyre.fl.wear_pct@0", Channel: "tyre.fl.wear_pct", CarIndex: &p, Measurement: contract.Available(8.0, "%", "test")},
	}, Events: []contract.Event{}, Participants: nil, Lap: nil, Session: map[string]float64{}, SessionTimeMs: nil, RecordingState: "recording"}
}

func intp(v int) *int { return &v }

func TestStrategyUnavailableInputs(t *testing.T) {
	dir := t.TempDir()
	cat, err := history.OpenCatalog(filepath.Join(dir, "c.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer cat.Close()
	ctx := context.Background()
	_ = cat.EnsureSourceContext(ctx, "c", "ip")
	_ = cat.CreateSession(ctx, "s", contract.Context{SourceContextID: "c", ModeFamily: "solo", SeasonEvidence: []string{}})
	_ = cat.CreateBranch(ctx, contract.Branch{ID: "b", SessionID: "s", IsSurviving: true})
	for i := 1; i <= 3; i++ {
		_ = cat.InsertLap(ctx, contract.LapSummary{ID: "l" + string(rune('0'+i)), SessionID: "s", BranchID: "b", CarIndex: 0, Lap: i, StartSeq: "1", EndSeq: "2", LapTimeMs: contract.Available(90000+i*100, "ms", "test"), DistanceM: contract.Unavailable("not_received", "m", "t", "x"), Compound: contract.Available("C3", "compound", "t"), Valid: true, Complete: true, Gaps: []string{}})
	}
	cfg, _ := config.Defaults()
	cfg.DataDir = &dir
	ev := NewEvaluator(&analysis.Engine{Catalog: cat, DataDir: dir}, cfg)
	p := 0
	cx := contract.Context{SessionID: "s", BranchID: "b", PlayerCarIndex: &p, ModeFamily: "solo", RegulationSeason: intp(2025), SeasonEvidence: []string{}}
	q := contract.StrategyQuery{SessionID: "s", BranchID: "b", ObservationSeq: "10", PitWindows: []contract.PitWindowIn{{FirstLap: 2, LastLap: 3, Compound: "C4"}}}
	res, err := ev.Evaluate(ctx, q, state.Snapshot{Context: cx, ObservationSeq: 10, Session: map[string]float64{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) < 2 {
		t.Fatalf("candidates %#v", res.Candidates)
	}
	if res.Candidates[1].PitLossMs.Availability == "available" {
		t.Fatal("invented pit loss")
	}
	if res.Candidates[0].TotalTimeMs.Availability == "available" && len(res.Limitations) == 0 {
		t.Fatal("expected assumptions/limitations")
	}
	cx.ModeFamily = "time_trial"
	if _, err := ev.Evaluate(ctx, q, state.Snapshot{Context: cx}); err != ErrUnsupportedMode {
		t.Fatalf("time trial err = %v", err)
	}
}

func TestDegradationNeedsThreeLaps(t *testing.T) {
	if _, ok := degradation([]contract.LapSummary{{LapTimeMs: contract.Available(1.0, "ms", "t")}, {LapTimeMs: contract.Available(2.0, "ms", "t")}}); ok {
		t.Fatal("slope from two laps")
	}
}
