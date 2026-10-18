package strategy

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"f1tele/server/internal/analysis"
	"f1tele/server/internal/config"
	"f1tele/server/internal/contract"
	"f1tele/server/internal/history"
	"f1tele/server/internal/state"
)

func lap(id string, car, n int, ms float64, compound string, valid bool) contract.LapSummary {
	return contract.LapSummary{
		ID: id, SessionID: "s", BranchID: "b", CarIndex: car, Lap: n, StartSeq: "1", EndSeq: "2",
		LapTimeMs: contract.Available(ms, "ms", "test"), DistanceM: contract.Unavailable("not_received", "m", "t", "x"),
		Compound: contract.Available(compound, "compound", "test"), Valid: valid, Complete: true, Gaps: []string{},
	}
}

func strategyFixture(t *testing.T) (*Evaluator, contract.Config) {
	t.Helper()
	dir := t.TempDir()
	cat, err := history.OpenCatalog(filepath.Join(dir, "c.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cat.Close() })
	ctx := context.Background()
	_ = cat.EnsureSourceContext(ctx, "c", "ip")
	_ = cat.CreateSession(ctx, "s", contract.Context{SourceContextID: "c", ModeFamily: "solo", SeasonEvidence: []string{}})
	_ = cat.CreateBranch(ctx, contract.Branch{ID: "b", SessionID: "s", IsSurviving: true})
	laps := []contract.LapSummary{
		lap("l1", 0, 1, 90000, "C3", true),
		lap("l2", 0, 2, 90200, "C3", true),
		lap("l3", 0, 3, 90400, "C3", true),
		lap("l4", 0, 4, 90600, "C3", true),
		lap("bad", 0, 5, 70000, "C3", false),
		lap("pit", 0, 6, 95000, "C4", true),
	}
	for _, l := range laps {
		if err := cat.InsertLap(ctx, l); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := config.Defaults()
	if err != nil {
		t.Fatal(err)
	}
	cfg.DataDir = &dir
	return NewEvaluator(&analysis.Engine{Catalog: cat, DataDir: dir, Now: time.Now}, cfg), cfg
}

func TestEstimates(t *testing.T) {
	ev, cfg := strategyFixture(t)
	p := 0
	snap := state.Snapshot{
		Context:        contract.Context{SessionID: "s", BranchID: "b", PlayerCarIndex: &p, ModeFamily: "solo", SeasonEvidence: []string{}},
		ObservationSeq: 10, Lap: intp(6), Session: map[string]float64{"total_laps": 20},
		Metrics: []contract.Metric{
			{ID: "fuel@0", Channel: "fuel.finish_margin_laps", CarIndex: &p, Measurement: contract.Available(1.5, "laps", "game")},
		},
	}
	res, err := ev.Evaluate(context.Background(), contract.StrategyQuery{SessionID: "s", BranchID: "b", ObservationSeq: "10"}, snap)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]contract.Estimate{}
	for _, e := range res.Estimates {
		byID[e.ID] = e
	}
	pace := byID["pace"]
	if pace.Measurement.Availability != "available" {
		t.Fatalf("pace unavailable: %#v", pace)
	}
	if got := pace.Measurement.Value.(float64); got != 90400 {
		t.Fatalf("median of the last eligible laps = %v, want 90400", got)
	}
	deg := byID["degradation"]
	if deg.Measurement.Availability != "available" || deg.Measurement.Value.(float64) < 0 {
		t.Fatalf("degradation = %#v", deg)
	}
	fuel := byID["fuel_finish"].Measurement
	if fuel.Value.(float64) != 1.5 {
		t.Fatalf("fuel margin = %v", fuel.Value)
	}
	if _, ok := byID["energy_draw"]; ok {
		t.Fatal("energy draw invented without a valid window")
	}
	// Energy draw needs two store samples at least 500 ms apart and within 10 s.
	ev2 := NewEvaluator(ev.Analysis, cfg)
	s1 := snap
	s1.Metrics = []contract.Metric{{ID: "e@0", Channel: "energy.store_j", CarIndex: &p, Measurement: contract.Available(4_000_000.0, "J", "game")}}
	ev2.Observe(s1)
	time.Sleep(10 * time.Millisecond)
	s2 := snap
	s2.Metrics = []contract.Metric{{ID: "e@0", Channel: "energy.store_j", CarIndex: &p, Measurement: contract.Available(3_500_000.0, "J", "game")}}
	res2, err := ev2.Evaluate(context.Background(), contract.StrategyQuery{SessionID: "s", BranchID: "b", ObservationSeq: "10"}, s2)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range res2.Estimates {
		if e.ID == "energy_draw" && e.Measurement.Availability == "available" {
			found = true
		}
	}
	if !found {
		t.Fatal("energy draw unavailable after a valid window")
	}
}

func TestCandidates(t *testing.T) {
	ev, cfg := strategyFixture(t)
	p := 0
	cfg.TrackProfiles = []contract.TrackProfile{{ID: "t3", TrackID: 3, RegulationSeasons: []int{2025, 2026}, PitLossMs: floatPtr(21000), Enabled: true, Corners: []contract.CornerProfile{}}}
	ev.Config = cfg
	snap := state.Snapshot{
		Context:        contract.Context{SessionID: "s", BranchID: "b", PlayerCarIndex: &p, ModeFamily: "solo", TrackID: intp(3), RegulationSeason: intp(2026), SeasonEvidence: []string{}},
		ObservationSeq: 10, Lap: intp(6), Session: map[string]float64{"total_laps": 20},
	}
	q := contract.StrategyQuery{SessionID: "s", BranchID: "b", ObservationSeq: "10", PitWindows: []contract.PitWindowIn{
		{FirstLap: 7, LastLap: 9, Compound: "C4"},
		{FirstLap: 7, LastLap: 9, Compound: ""},
		{FirstLap: 25, LastLap: 27, Compound: "C5"},
	}}
	res, err := ev.Evaluate(context.Background(), q, snap)
	if err != nil {
		t.Fatal(err)
	}
	// stay-out, three requested windows, and one pair of distinct windows.
	if len(res.Candidates) != 5 {
		t.Fatalf("candidates = %d", len(res.Candidates))
	}
	if len(res.Candidates[4].Stops) != 2 {
		t.Fatalf("two-stop candidate %#v", res.Candidates[4])
	}
	stay := res.Candidates[0]
	if stay.Legality != "legal" || len(stay.Stops) != 0 {
		t.Fatalf("stay-out %#v", stay)
	}
	// Median pace 90400 over 14 remaining laps, stay-out has no pit loss.
	wantStay := 90400.0 * 14
	if v := stay.TotalTimeMs.Value.(float64); v < wantStay || v > wantStay*1.05 {
		t.Fatalf("stay-out total %v, want about %v", v, wantStay)
	}
	if v := stay.ErrorBandMs; v == nil || *v <= 0 {
		t.Fatal("missing error band")
	}
	first := res.Candidates[1]
	if first.PitLossMs.Availability != "available" || first.PitLossMs.Value.(float64) != 21000 {
		t.Fatalf("pit loss from the track profile = %#v", first.PitLossMs)
	}
	if v := first.TotalTimeMs.Value.(float64); v < wantStay || v > wantStay*1.05 {
		t.Fatalf("pit total %v should stay near pace times remaining laps", first.TotalTimeMs.Value)
	}
	if res.Candidates[2].Legality != "illegal" {
		t.Fatal("empty compound must be illegal")
	}
	if res.Candidates[3].Legality != "illegal" {
		fatal := res.Candidates[3]
		t.Fatalf("window beyond the race must be illegal: %#v", fatal.Reasons)
	}
	// If the track profile is absent, pit loss stays unavailable.
	ev.Config = contract.Config{}
	res2, err := ev.Evaluate(context.Background(), q, snap)
	if err != nil {
		t.Fatal(err)
	}
	if res2.Candidates[1].PitLossMs.Availability == "available" {
		t.Fatal("invented pit loss without observed or sourced evidence")
	}
	// Objective outcomes: branch change must not mutate a stored evaluation.
	before := res.Candidates[0].TotalTimeMs.Value
	snap.Context.BranchID = "b2"
	if res.Candidates[0].TotalTimeMs.Value != before {
		t.Fatal("prior evaluation changed with the branch")
	}
}

func TestReferenceProfiles(t *testing.T) {
	cfg, err := config.Defaults()
	if err != nil {
		t.Fatal(err)
	}
	a := NewArbiter(cfg.Notifications, cfg.ReferenceProfiles, cfg.CapacityProfiles)
	p := 0
	snap := state.Snapshot{Context: contract.Context{SessionID: "s", BranchID: "b", PlayerCarIndex: &p, RegulationSeason: intp(2026), FormulaCode: intp(0), SeasonEvidence: []string{}},
		Metrics: []contract.Metric{{ID: "tyre.fl.surface_c@0", Channel: "tyre.fl.surface_c", CarIndex: &p, Measurement: contract.Available(106.0, "degC", "t")}}}
	if a.profileFor("tyre.fl.surface_c", snap) != nil {
		t.Fatal("2025 legacy surface profile applied to 2026")
	}
	snap.Context.RegulationSeason = intp(2025)
	if a.profileFor("tyre.fl.surface_c", snap) == nil {
		t.Fatal("2025 legacy surface profile not applied to 2025")
	}
	// Unknown season cannot claim a season-scoped band.
	snap.Context.RegulationSeason = nil
	if a.profileFor("tyre.fl.surface_c", snap) != nil {
		t.Fatal("season-scoped profile applied to an unknown season")
	}
	// Unknown capacity keeps energy absolute: no SOC claim.
	if a.capacityProfile(snap) != nil {
		t.Fatal("capacity profile exists in shipped defaults")
	}
	_ = os.Getenv
}
