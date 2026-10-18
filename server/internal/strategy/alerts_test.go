package strategy

import (
	"testing"
	"time"

	"f1tele/server/internal/config"
	"f1tele/server/internal/contract"
	"f1tele/server/internal/state"
)

func policyArbiter(t *testing.T) *Arbiter {
	t.Helper()
	cfg, err := config.Defaults()
	if err != nil {
		t.Fatal(err)
	}
	a := NewArbiter(cfg.Notifications, cfg.ReferenceProfiles, cfg.CapacityProfiles)
	now := time.Unix(1_700_000_000, 0)
	a.now = func() time.Time { return now }
	return a
}

func setNow(a *Arbiter, t time.Time) { a.now = func() time.Time { return t } }

func policySnap(surface map[string]float64, wear float64) state.Snapshot {
	p := 0
	metrics := []contract.Metric{}
	for pos, v := range surface {
		metrics = append(metrics, contract.Metric{ID: "tyre." + pos + ".surface_c@0", Channel: "tyre." + pos + ".surface_c", CarIndex: &p, Measurement: contract.Available(v, "degC", "t")})
	}
	for _, pos := range []string{"fl", "fr", "rl", "rr"} {
		metrics = append(metrics, contract.Metric{ID: "tyre." + pos + ".wear_pct@0", Channel: "tyre." + pos + ".wear_pct", CarIndex: &p, Measurement: contract.Available(wear, "%", "t")})
	}
	return state.Snapshot{Context: contract.Context{SessionID: "s", BranchID: "b", PlayerCarIndex: &p, RegulationSeason: intp(2025), FormulaCode: intp(0), SeasonEvidence: []string{}},
		ObservationSeq: 5, Metrics: metrics, Events: []contract.Event{}, Session: map[string]float64{}, RecordingState: "recording"}
}

func TestNotificationPolicy(t *testing.T) {
	a := policyArbiter(t)
	start := time.Unix(1_700_000_000, 0)

	// Caution needs 3 seconds above the threshold.
	snap := policySnap(map[string]float64{"fl": 106}, 10)
	if n := a.Observe(snap); len(n) != 0 {
		t.Fatalf("caution announced at 0s: %#v", n)
	}
	setNow(a, start.Add(2*time.Second))
	if n := a.Observe(snap); len(n) != 0 {
		t.Fatalf("caution announced at 2s: %#v", n)
	}
	setNow(a, start.Add(3*time.Second))
	n := a.Observe(snap)
	if len(n) != 1 || n[0].Text == "" {
		t.Fatalf("caution missing at 3s: %#v", n)
	}

	// Critical needs only 500 ms.
	cstart := start.Add(10 * time.Second)
	setNow(a, cstart)
	crit := policySnap(map[string]float64{"fl": 90, "rr": 112}, 10)
	a.Observe(crit)
	setNow(a, cstart.Add(600*time.Millisecond))
	if n := a.Observe(crit); len(n) != 1 || !n[0].Critical {
		t.Fatalf("critical missing after 600ms: %#v", n)
	}

	// Clearance needs the 5-second window, and a drop below the band must not
	// leave the alert stuck on forever.
	clearAt := cstart.Add(20 * time.Second)
	setNow(a, clearAt)
	calm := policySnap(map[string]float64{"fl": 90, "rr": 90}, 10)
	a.Observe(calm)
	setNow(a, clearAt.Add(4*time.Second))
	if len(a.Alerts()) == 0 {
		t.Fatal("cleared before the 5s window")
	}
	setNow(a, clearAt.Add(6*time.Second))
	a.Observe(calm)
	if got := a.Alerts(); len(got) != 0 {
		t.Fatalf("conditions stuck after clearance: %#v", got)
	}
	// Two-unit hysteresis: 104 stays caution, not critical.
	setNow(a, start.Add(60*time.Second))
	hyst := policySnap(map[string]float64{"fr": 104}, 10)
	n = a.Observe(hyst)
	setNow(a, start.Add(64*time.Second))
	n = a.Observe(hyst)
	if len(n) != 1 || n[0].Critical {
		t.Fatalf("hysteresis misclassified 104C: %#v", n)
	}

	// Grouping: several tyre conditions in one tick produce one sentence.
	grouped := policySnap(map[string]float64{"fl": 106, "fr": 106, "rl": 106}, 10)
	setNow(a, start.Add(120*time.Second))
	a.Observe(grouped)
	setNow(a, start.Add(124*time.Second))
	out := a.Observe(grouped)
	spoken := 0
	for _, x := range out {
		if x.Spoken {
			spoken++
		}
	}
	if spoken > 1 {
		t.Fatalf("grouped tyres spoke %d times: %#v", spoken, out)
	}

	// Routine gap: two routine calls inside 30 seconds yield one spoken message.
	a2 := policyArbiter(t)
	setNow(a2, start)
	w1 := policySnap(map[string]float64{"fl": 106}, 76)
	a2.Observe(w1)
	setNow(a2, start.Add(4*time.Second))
	first := a2.Observe(w1)
	if len(first) == 0 {
		t.Fatal("no initial warning")
	}
	setNow(a2, start.Add(8*time.Second))
	w2 := policySnap(map[string]float64{"fr": 106}, 76)
	a2.Observe(w2)
	setNow(a2, start.Add(12*time.Second))
	second := a2.Observe(w2)
	for _, x := range second {
		if x.Spoken && !x.Critical {
			t.Fatalf("routine announcement inside the 30s gap: %#v", x)
		}
	}
}
