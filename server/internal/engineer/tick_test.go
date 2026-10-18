package engineer

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"f1tele/server/internal/config"
	"f1tele/server/internal/contract"
	"f1tele/server/internal/state"
)

// TestScheduledEventStaysPending proves a triggered dispatch that hits the
// budget keeps its Safety Car event for the next window.
func TestScheduledEventStaysPending(t *testing.T) {
	cfg, err := config.Defaults()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Agent.Enabled = true
	ep := "e1"
	cfg.Agent.EndpointID = &ep
	cfg.Agent.Endpoints = []contract.Endpoint{{ID: "e1", Protocol: "chat_completions", BaseURL: "http://127.0.0.1:1/v1", Model: "m", ContextLimitTokens: 8192}}
	dir := t.TempDir()
	t.Setenv("F1TELE_DATA_DIR", dir)
	store, err := config.Load(config.Overrides{DataDir: dir, ConfigPath: filepath.Join(dir, "config.json")})
	if err != nil {
		t.Fatal(err)
	}
	rev := store.Revision()
	if _, err := store.ApplyPatch(contract.ConfigPatch{Config: cfg}, &rev); err != nil {
		t.Fatal(err)
	}
	m := &Manager{rules: NewRules(cfg.Agent.Rules), sched: NewScheduler("b", 10), active: map[string]context.CancelFunc{},
		budget: NewBudget(cfg.Agent, nil), Store: store}
	p := 0
	snap := state.Snapshot{Context: contract.Context{SessionID: "s", BranchID: "b", PlayerCarIndex: &p, SeasonEvidence: []string{}},
		ObservationSeq: 20, Session: map[string]float64{},
		Events: []contract.Event{{ID: "e-sc", Seq: "15", Type: "safety_car", Severity: "important", EvidenceIDs: []string{}}}}
	// Exhaust the request budget so the trigger cannot dispatch.
	for i := 0; i < cfg.Agent.MaxCallsPerHour; i++ {
		res, ok, _ := m.budget.Reserve(10, 10)
		if !ok {
			break
		}
		m.budget.Reconcile(res, contract.Usage{})
	}
	m.Tick(snap)
	scope := m.sched.Scope()
	if len(scope.Events) != 1 || scope.Events[0].ID != "e-sc" {
		t.Fatalf("event lost on budget block: %#v", scope.Events)
	}
	if scope.EvidenceCursor != nil {
		t.Fatal("budget block advanced evidence")
	}
	_ = time.Now
}
