package engineer

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"f1tele/server/internal/contract"
	"f1tele/server/internal/history"
)

func intPtr(v int) *int       { return &v }
func strPtr(v string) *string { return &v }

func TestRecordOutcomesComparesPlanWithObservedLaps(t *testing.T) {
	cat, err := history.OpenCatalog(filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer cat.Close()
	prev := contract.AgentOutput{
		InvocationID: "inv-1", SessionID: "s", BranchID: "b", ObservationSeq: "100",
		Plan: contract.Plan{Action: "pit", PitLapStart: intPtr(20), PitLapEnd: intPtr(22), Compound: strPtr("C3")},
	}
	laps := []contract.LapSummary{
		{ID: "before", Lap: 19, EndSeq: "90", PitLap: true},
		{ID: "stop", Lap: 22, EndSeq: "200", PitLap: true, Compound: contract.Available("C3", "compound", "test")},
		{ID: "after", Lap: 23, EndSeq: "300", Compound: contract.Available("C4", "compound", "test")},
	}
	out, err := RecordOutcomes(context.Background(), cat, prev, laps, 23)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("outcomes = %#v", out)
	}
	if out[0] != "Planned the stop on lap 20; observed it on lap 22." {
		t.Fatalf("pit outcome = %q", out[0])
	}
	if out[1] != "Planned C3 after the stop; observed C4." {
		t.Fatalf("compound outcome = %q", out[1])
	}
	// A later call with older coverage must not rewrite the recorded evidence.
	again, err := RecordOutcomes(context.Background(), cat, prev, laps, 21)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 2 || again[0] != out[0] {
		t.Fatalf("outcome rewound: %#v", again)
	}
	var n int
	if err := cat.DB().QueryRow("SELECT COUNT(*) FROM strategy_outcomes").Scan(&n); err != nil || n != 1 {
		t.Fatalf("outcome rows = %d, err %v", n, err)
	}
}

func TestRecordOutcomesWaitsForClosedWindow(t *testing.T) {
	cat, err := history.OpenCatalog(filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer cat.Close()
	prev := contract.AgentOutput{InvocationID: "inv-2", SessionID: "s", BranchID: "b", ObservationSeq: "10",
		Plan: contract.Plan{Action: "pit", PitLapStart: intPtr(20), PitLapEnd: intPtr(22)}}
	if out, err := RecordOutcomes(context.Background(), cat, prev, nil, 21); err != nil || len(out) != 0 {
		t.Fatalf("outcome before window closed: %#v %v", out, err)
	}
	out, err := RecordOutcomes(context.Background(), cat, prev, nil, 24)
	if err != nil || len(out) != 1 {
		t.Fatalf("closed window outcome = %#v %v", out, err)
	}
	if out[0] != "Planned a stop by lap 22; no stop observed through lap 24." {
		t.Fatalf("missing stop = %q", out[0])
	}
}

func TestPreviousAcceptedIsBranchScoped(t *testing.T) {
	cat, err := history.OpenCatalog(filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer cat.Close()
	out := contract.AgentOutput{InvocationID: "inv-old", SessionID: "s", BranchID: "b1", ObservationSeq: "50"}
	raw, _ := json.Marshal(out)
	if _, err := cat.DB().Exec("INSERT INTO invocations(id, session_id, branch_id, status, output_json, profile, started_at_ms) VALUES(?,?,?,?,?,?,?)",
		"inv-old", "s", "b1", "accepted", string(raw), "scheduled_strategy", 1000); err != nil {
		t.Fatal(err)
	}
	question, _ := json.Marshal(contract.AgentOutput{InvocationID: "inv-q", SessionID: "s", BranchID: "b1", ObservationSeq: "60"})
	if _, err := cat.DB().Exec("INSERT INTO invocations(id, session_id, branch_id, status, output_json, profile, started_at_ms) VALUES(?,?,?,?,?,?,?)",
		"inv-q", "s", "b1", "accepted", string(question), "live_question", 2000); err != nil {
		t.Fatal(err)
	}
	m := &Manager{Catalog: cat}
	got, err := m.previousAccepted(context.Background(), "s", "b1", "inv-new")
	if err != nil || got == nil || got.InvocationID != "inv-old" {
		t.Fatalf("same branch advice = %#v %v (a live question must not become strategy continuity)", got, err)
	}
	if got, err := m.previousAccepted(context.Background(), "s", "b2", "inv-new"); err != nil || got != nil {
		t.Fatalf("new branch inherited advice: %#v %v", got, err)
	}
	if got, err := m.previousAccepted(context.Background(), "s", "b1", "inv-old"); err != nil || got != nil {
		t.Fatalf("current invocation reused as previous: %#v %v", got, err)
	}
}
