package setup

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"f1tele/server/internal/contract"
)

func TestExperimentAppendPreservesHypothesis(t *testing.T) {
	store, cat := storeFor(t, nil)
	svc := New(store, cat)
	ctx := context.Background()
	_ = cat.EnsureSourceContext(ctx, "c", "ip")
	_ = cat.CreateSession(ctx, "s", contract.Context{SourceContextID: "c", ModeFamily: "solo", SeasonEvidence: []string{}})
	_ = cat.CreateBranch(ctx, contract.Branch{ID: "b", SessionID: "s", IsSurviving: true})
	out := contract.AgentOutput{SchemaVersion: contract.SchemaVersion, InvocationID: "i-1", SessionID: "s", BranchID: "b", ObservationSeq: "1",
		Decision: "change", Confidence: "low", Urgency: "routine", Plan: contract.Plan{Action: "setup_experiment", Alternatives: []string{}},
		EvidenceIDs: []string{}, Assumptions: []string{}, Predictions: []contract.Prediction{}, Analysis: "x", Limitations: []string{},
		Sources:      []contract.Source{{ID: "src-1", Title: "guide", URL: "https://example.com", UseCase: "race", Quality: "community", Limitations: []string{}}},
		SetupChanges: []contract.SetupChange{{Parameter: "rear wing", ToValue: "12", Unit: "clicks", HandlingEffect: "more rear", Tradeoff: "less top speed", SourceIDs: []string{"src-1"}, Experiment: "one click"}}}
	raw, _ := json.Marshal(out)
	if _, err := cat.DB().Exec("INSERT INTO invocations(id, session_id, branch_id, status, output_json, usage_json, profile, started_at_ms) VALUES(?,?,?,?,?,?,?,?)",
		"i-1", "s", "b", "accepted", string(raw), "{}", "setup_research", time.Now().UnixMilli()); err != nil {
		t.Fatal(err)
	}
	rec, err := svc.Create(ctx, "i-1")
	if err != nil {
		t.Fatal(err)
	}
	before := rec.Output.SetupChanges[0].ToValue
	if _, err := svc.AddExperiment(ctx, rec.ID, contract.ExperimentCreate{BaselineLapIDs: []string{"l1"}, TrialLapIDs: []string{"l2"}, Conditions: "dry", Outcome: "slower by 0.2s"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddExperiment(ctx, rec.ID, contract.ExperimentCreate{BaselineLapIDs: []string{"l1"}, TrialLapIDs: []string{"l3"}, Conditions: "dry", Outcome: "no change"}); err != nil {
		t.Fatal(err)
	}
	all, err := svc.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("records = %d", len(all))
	}
	if all[0].Output.SetupChanges[0].ToValue != before {
		t.Fatal("experiment rewrote the original recommendation")
	}
	if len(all[0].Experiments) != 2 {
		t.Fatalf("experiments = %d, want both outcomes kept", len(all[0].Experiments))
	}
	if all[0].Experiments[0].Outcome == all[0].Experiments[1].Outcome {
		t.Fatal("outcomes collapsed")
	}
}
