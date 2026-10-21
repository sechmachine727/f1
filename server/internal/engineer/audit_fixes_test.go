package engineer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"f1tele/server/internal/config"
	"f1tele/server/internal/contract"
	"f1tele/server/internal/history"
	"f1tele/server/internal/state"

	"github.com/cloudwego/eino/components/tool"
)

func testManager(t *testing.T, endpoint contract.Endpoint, rules []contract.TriggerRule) (*Manager, *config.Store, *state.Reducer, *history.Catalog) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("F1TELE_DATA_DIR", dir)
	store, err := config.Load(config.Overrides{DataDir: dir, ConfigPath: filepath.Join(dir, "config.json")})
	if err != nil {
		t.Fatal(err)
	}
	cfg := store.Config()
	cfg.Agent.Enabled = true
	keyEnv := "F1TELE_TEST_AGENT_KEY"
	t.Setenv(keyEnv, "sk-test")
	endpoint.APIKeyEnv = &keyEnv
	id := endpoint.ID
	cfg.Agent.EndpointID = &id
	cfg.Agent.Endpoints = []contract.Endpoint{endpoint}
	if rules != nil {
		cfg.Agent.Rules = rules
	}
	rev := store.Revision()
	if _, err := store.ApplyPatch(contract.ConfigPatch{Config: cfg}, &rev); err != nil {
		t.Fatal(err)
	}
	cat, err := history.OpenCatalog(filepath.Join(dir, "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cat.Close() })
	red := state.New("ctx", time.Now)
	m := NewManager(store, red, cat, &Invoker{Backend: ToolBackend{Engine: nil}}, nil)
	m.Activity = &Activity{Catalog: cat}
	return m, store, red, cat
}

func patchAgentConfig(t *testing.T, store *config.Store, mutate func(*contract.Config)) {
	t.Helper()
	cfg := store.Config()
	mutate(&cfg)
	rev := store.Revision()
	if _, err := store.ApplyPatch(contract.ConfigPatch{Config: cfg}, &rev); err != nil {
		t.Fatal(err)
	}
}

func waitInvocation(t *testing.T, cat *history.Catalog, id string) contract.Invocation {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var status string
		if err := cat.DB().QueryRow("SELECT status FROM invocations WHERE id=?", id).Scan(&status); err == nil && status != "running" {
			m := &Manager{Catalog: cat}
			items, _, err := m.Invocations(context.Background(), "", "", "", 100, "")
			if err != nil {
				t.Fatal(err)
			}
			for _, item := range items {
				if item.ID == id {
					return item
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("invocation %s did not finish", id)
	return contract.Invocation{}
}

// TestSessionChangeResetsSchedulerState covers Q1 to Q2: pending events and
// rule counters must not survive the new logical session.
func TestSessionChangeResetsSchedulerState(t *testing.T) {
	rules := []contract.TriggerRule{{ID: "laps", Enabled: true,
		Predicate: contract.Predicate{Kind: "lap", EveryCompletedLaps: intPtr(2)}}}
	m, store, _, _ := testManager(t, contract.Endpoint{ID: "e", Protocol: "chat_completions", BaseURL: "http://127.0.0.1:1/v1", Model: "m"}, rules)
	patchAgentConfig(t, store, func(cfg *contract.Config) { cfg.Agent.MaxCallsPerHour = 1 })
	res, ok, _ := m.budget.Reserve(1, 1)
	if !ok {
		t.Fatal("seed reservation blocked")
	}
	m.budget.Reconcile(res, contract.Usage{ProviderRequests: 1})
	m.sessionID = "q1"
	m.sched = NewScheduler("b1", 0)
	p := 0
	lap1 := 1
	q1 := state.Snapshot{Context: contract.Context{SessionID: "q1", BranchID: "b1", PlayerCarIndex: &p, SeasonEvidence: []string{}},
		ObservationSeq: 10, Lap: &lap1, Session: map[string]float64{},
		Events: []contract.Event{{ID: "e1", Seq: "5", Type: "weather_changed", Severity: "important", EvidenceIDs: []string{}}}}
	m.Tick(q1)
	if scope := m.sched.Scope(); len(scope.Events) != 1 || scope.Events[0].ID != "e1" {
		t.Fatalf("q1 pending events = %#v", scope.Events)
	}
	q2 := state.Snapshot{Context: contract.Context{SessionID: "q2", BranchID: "b2", PlayerCarIndex: &p, SeasonEvidence: []string{}},
		ObservationSeq: 3, Lap: &lap1, Session: map[string]float64{},
		Events: []contract.Event{{ID: "e2", Seq: "6", Type: "weather_changed", Severity: "important", EvidenceIDs: []string{}}}}
	m.Tick(q2)
	m.mu.Lock()
	gotLap, gotEvt := m.lastLap, m.lastEvt
	m.mu.Unlock()
	if gotLap != 1 || gotEvt != "e2" {
		t.Fatalf("session cursors not reset: lap=%d evt=%q", gotLap, gotEvt)
	}
	if len(m.sched.inFlight) != 0 {
		t.Fatalf("stale rule counter dispatched in q2: %#v", m.sched.inFlight)
	}
	scope := m.sched.Scope()
	if scope.Branch != "b2" || len(scope.Events) != 1 || scope.Events[0].ID != "e2" {
		t.Fatalf("q2 scheduler state = %#v", scope)
	}
	if m.rules.laps["laps"] != 1 {
		t.Fatalf("rule lap counter survived session change: %d", m.rules.laps["laps"])
	}
}

// TestSessionChangeResetsLapCursor proves the lap cursor resets even when the
// new session's first lap number is lower than the previous session's last.
func TestSessionChangeResetsLapCursor(t *testing.T) {
	rules := []contract.TriggerRule{{ID: "rare", Enabled: true,
		Predicate: contract.Predicate{Kind: "lap", EveryCompletedLaps: intPtr(100)}}}
	m, _, _, _ := testManager(t, contract.Endpoint{ID: "e", Protocol: "chat_completions", BaseURL: "http://127.0.0.1:1/v1", Model: "m"}, rules)
	m.sessionID = "q1"
	m.sched = NewScheduler("b1", 0)
	p := 0
	lap5, lap1 := 5, 1
	m.Tick(state.Snapshot{Context: contract.Context{SessionID: "q1", BranchID: "b1", PlayerCarIndex: &p, SeasonEvidence: []string{}},
		ObservationSeq: 10, Lap: &lap5, Session: map[string]float64{}})
	m.Tick(state.Snapshot{Context: contract.Context{SessionID: "q2", BranchID: "b2", PlayerCarIndex: &p, SeasonEvidence: []string{}},
		ObservationSeq: 3, Lap: &lap1, Session: map[string]float64{}})
	m.mu.Lock()
	got := m.lastLap
	m.mu.Unlock()
	if got != 1 {
		t.Fatalf("lap cursor survived session change: %d", got)
	}
	if len(m.sched.inFlight) != 0 {
		t.Fatalf("unexpected dispatch: %#v", m.sched.inFlight)
	}
}

// TestRulesFiltersAndCooldownEnforceSettings covers the exposed rule fields.
func TestRulesFiltersAndCooldownEnforceSettings(t *testing.T) {
	rules := NewRules([]contract.TriggerRule{{ID: "r", Enabled: true,
		Predicate:       contract.Predicate{Kind: "event", EventTypes: []string{"safety_car"}},
		Filters:         contract.RuleFilters{ModeFamilies: []string{"career"}, CarScope: "player", MinimumSeverity: "critical", Validity: "valid"},
		CooldownSeconds: 10}})
	now := time.Unix(1_700_000_000, 0)
	rules.now = func() time.Time { return now }
	car := 1
	if got := rules.TriggerWithContext(TriggerContext{ModeFamily: "time_trial", PlayerCar: 0, Events: []PendingEvent{{Type: "safety_car", Severity: "critical"}}}); len(got) != 0 {
		t.Fatalf("mode filter ignored: %#v", got)
	}
	if got := rules.TriggerWithContext(TriggerContext{ModeFamily: "career", PlayerCar: 0, Events: []PendingEvent{{Type: "safety_car", Severity: "important"}}}); len(got) != 0 {
		t.Fatalf("severity filter ignored: %#v", got)
	}
	if got := rules.TriggerWithContext(TriggerContext{ModeFamily: "career", PlayerCar: 0, Events: []PendingEvent{{Type: "safety_car", Severity: "critical", CarIndex: &car}}}); len(got) != 0 {
		t.Fatalf("car scope filter ignored: %#v", got)
	}
	if got := rules.TriggerWithContext(TriggerContext{ModeFamily: "career", PlayerCar: 0, Events: []PendingEvent{{Type: "safety_car", Severity: "critical"}}}); len(got) != 1 {
		t.Fatalf("critical session event not triggered: %#v", got)
	}
	if got := rules.TriggerWithContext(TriggerContext{ModeFamily: "career", PlayerCar: 0, Events: []PendingEvent{{Type: "safety_car", Severity: "critical"}}}); len(got) != 0 {
		t.Fatalf("cooldown ignored: %#v", got)
	}
	now = now.Add(11 * time.Second)
	if got := rules.TriggerWithContext(TriggerContext{ModeFamily: "career", PlayerCar: 0, Events: []PendingEvent{{Type: "safety_car", Severity: "critical"}}}); len(got) != 1 {
		t.Fatalf("cooldown did not expire: %#v", got)
	}
}

// TestLiveConfigRevisionRebuildsDispatchState proves saved settings take effect
// without a restart.
func TestLiveConfigRevisionRebuildsDispatchState(t *testing.T) {
	m, store, _, _ := testManager(t, contract.Endpoint{ID: "e", Protocol: "chat_completions", BaseURL: "http://127.0.0.1:1/v1", Model: "m"}, nil)
	patchAgentConfig(t, store, func(cfg *contract.Config) {
		cfg.Agent.MaxCallsPerHour = 7
		cfg.Agent.MinimumDispatchGapSecs = 33
		cfg.Agent.Rules = []contract.TriggerRule{{ID: "new", Enabled: true, Predicate: contract.Predicate{Kind: "manual"}}}
		cfg.Agent.Continuous = contract.ContinuousConfig{Enabled: true, CadenceSeconds: 5, MaxMinutes: 3}
	})
	cfg, rev := store.Snapshot()
	m.syncConfig(cfg, rev, time.Now())
	if got, _ := m.budget.Remaining(); got != 7 {
		t.Fatalf("budget revision not applied: remaining=%d", got)
	}
	if m.sched.minGap != 33*time.Second {
		t.Fatalf("dispatch gap not applied: %s", m.sched.minGap)
	}
	if len(m.rules.cfg) != 1 || m.rules.cfg[0].ID != "new" {
		t.Fatalf("rules revision not applied: %#v", m.rules.cfg)
	}
	if !m.contCfg.Enabled || !m.continuous.Enabled {
		t.Fatalf("continuous revision not applied: %#v %#v", m.contCfg, m.continuous)
	}
}

func TestTickDoesNotReprocessEvents(t *testing.T) {
	m, _, _, _ := testManager(t, contract.Endpoint{ID: "e", Protocol: "chat_completions", BaseURL: "http://127.0.0.1:1/v1", Model: "m"}, []contract.TriggerRule{{ID: "off", Enabled: false, Predicate: contract.Predicate{Kind: "manual"}}})
	player := 0
	snap := state.Snapshot{
		Context:        contract.Context{SessionID: "s", BranchID: "b", PlayerCarIndex: &player, SeasonEvidence: []string{}},
		ObservationSeq: 10,
		Events: []contract.Event{
			{ID: "e1", Seq: "1", Type: "race_control", Severity: "routine"},
			{ID: "e2", Seq: "1", Type: "race_control", Severity: "routine"},
		},
	}
	m.Tick(snap)
	if m.lastEvtSeq != 1 || len(m.lastEvtIDs) != 2 {
		t.Fatalf("event cursor = %d ids=%v", m.lastEvtSeq, m.lastEvtIDs)
	}
	m.Tick(snap)
	if m.lastEvtSeq != 1 || len(m.lastEvtIDs) != 2 {
		t.Fatalf("event cursor moved on an unchanged snapshot: %d ids=%v", m.lastEvtSeq, m.lastEvtIDs)
	}
}

func TestMaxToolCallsIsEnforced(t *testing.T) {
	tools, err := BuildTools("live_question", ToolBackend{}, contract.Observation{})
	if err != nil {
		t.Fatal(err)
	}
	tools = limitTools(tools, 1)
	args := `{"from_seq":"0","to_seq":"10","car_indices":[0],"channels":["car.speed_mps"],"max_points":10}`
	invokable := tools[0].(tool.InvokableTool)
	if _, err := invokable.InvokableRun(context.Background(), args); err != nil {
		t.Fatalf("first tool call rejected: %v", err)
	}
	if _, err := invokable.InvokableRun(context.Background(), args); err == nil || !strings.Contains(err.Error(), "tool call limit") {
		t.Fatalf("second tool call not limited: %v", err)
	}
}

func TestBudgetCountsEveryProviderRequest(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	b := NewBudget(contract.AgentConfig{MaxCallsPerHour: 3, MaxTokensPerHour: 1000}, func() time.Time { return now })
	res, ok, reason := b.Reserve(10, 10)
	if !ok {
		t.Fatalf("reserve blocked: %s", reason)
	}
	b.Reconcile(res, contract.Usage{ProviderRequests: 3})
	if _, ok, _ := b.Reserve(10, 10); ok {
		t.Fatal("three provider requests counted as one")
	}
}

func TestCostCeilingEnforced(t *testing.T) {
	maxCost := 1.0
	price := 1.0
	cfg := contract.AgentConfig{MaxCallsPerHour: 10, MaxTokensPerHour: 100_000_000, MaxCostUSDPerHour: &maxCost}
	b := NewBudget(cfg, nil)
	b.Configure(cfg, &contract.Endpoint{InputUSDPerMillion: &price, OutputUSDPerMillion: &price})
	res, ok, _ := b.Reserve(600_000, 0)
	if !ok {
		t.Fatal("first cost reservation blocked")
	}
	b.Reconcile(res, contract.Usage{ProviderRequests: 1})
	if _, ok, reason := b.Reserve(600_000, 0); ok || !strings.Contains(reason, "cost ceiling") {
		t.Fatalf("cost ceiling not enforced: ok=%v reason=%q", ok, reason)
	}
	noPrice := NewBudget(cfg, nil)
	noPrice.Configure(cfg, &contract.Endpoint{})
	if _, ok, reason := noPrice.Reserve(10, 10); ok || !strings.Contains(reason, "prices") {
		t.Fatalf("missing prices not rejected: ok=%v reason=%q", ok, reason)
	}
}

func TestMinimumDispatchGapEnforced(t *testing.T) {
	s := NewScheduler("b", 0)
	s.SetDispatchGap(10 * time.Second)
	now := time.Unix(1_700_000_000, 0)
	if _, err := s.Dispatch("i-1", 1, now); err != nil {
		t.Fatal(err)
	}
	s.Fail("i-1", "done")
	if _, err := s.Dispatch("i-2", 2, now.Add(5*time.Second)); err == nil {
		t.Fatal("dispatch inside the minimum gap was accepted")
	}
	if _, err := s.Dispatch("i-3", 3, now.Add(11*time.Second)); err != nil {
		t.Fatalf("dispatch after the gap rejected: %v", err)
	}
}

func TestContinuousModeDispatchesWithoutEvents(t *testing.T) {
	m, store, red, _ := testManager(t, contract.Endpoint{ID: "e", Protocol: "chat_completions", BaseURL: "http://127.0.0.1:1/v1", Model: "m"}, nil)
	patchAgentConfig(t, store, func(cfg *contract.Config) {
		cfg.Agent.Continuous = contract.ContinuousConfig{Enabled: true, CadenceSeconds: 5, MaxMinutes: 15}
	})
	for i := 0; i < 60; i++ {
		res, ok, _ := m.budget.Reserve(1, 1)
		if !ok {
			break
		}
		m.budget.Reconcile(res, contract.Usage{ProviderRequests: 1})
	}
	m.Tick(red.Snapshot(nil, nil))
	if len(m.sched.inFlight) != 1 || !m.sched.IsBudgetBlocked() {
		t.Fatalf("continuous dispatch = %#v blocked=%v", m.sched.inFlight, m.sched.IsBudgetBlocked())
	}
}

func TestEarlyFailureReleasesReservation(t *testing.T) {
	m, _, red, _ := testManager(t, contract.Endpoint{ID: "e", Protocol: "chat_completions", BaseURL: "http://127.0.0.1:1/v1", Model: "m"}, nil)
	cfg := m.Store.Config()
	missing := "missing"
	cfg.Agent.EndpointID = &missing
	cfg.Agent.Endpoints = nil
	before, _ := m.budget.Remaining()
	inv := contract.Invocation{ID: "inv-missing", Profile: "live_question", SessionID: red.SessionID(), BranchID: red.BranchID(), Status: "running", StartedAtMs: int64Ptr(time.Now().UnixMilli())}
	if err := m.store(inv); err != nil {
		t.Fatal(err)
	}
	res, ok, _ := m.budget.Reserve(100, 100)
	if !ok {
		t.Fatal("reserve blocked")
	}
	m.execute(context.Background(), inv, cfg, red.Snapshot(nil, nil), nil, Invocation{}, nil, false, res)
	got := waitInvocation(t, m.Catalog, inv.ID)
	if got.Status != "failed" || got.Error == nil || got.Error.Code != "failed" || !strings.Contains(got.Error.Message, "endpoint_missing") {
		t.Fatalf("invocation = %#v", got)
	}
	after, _ := m.budget.Remaining()
	if before != after {
		t.Fatalf("reservation leaked: before=%d after=%d", before, after)
	}
}

func TestStartScheduledStoreFailureReleasesReservation(t *testing.T) {
	cat, err := history.OpenCatalog(filepath.Join(t.TempDir(), "closed.db"))
	if err != nil {
		t.Fatal(err)
	}
	cat.Close()
	cfg := contract.AgentConfig{MaxCallsPerHour: 5, MaxTokensPerHour: 1000}
	m := &Manager{budget: NewBudget(cfg, nil), sched: NewScheduler("b", 0), Catalog: cat,
		active: map[string]context.CancelFunc{}, rules: NewRules(nil)}
	res, ok, _ := m.budget.Reserve(100, 100)
	if !ok {
		t.Fatal("reserve blocked")
	}
	m.startScheduled(Invocation{ID: "inv-store", Branch: "b"}, []string{"x"}, "scheduled_strategy", contract.Config{Agent: cfg},
		state.Snapshot{Context: contract.Context{SessionID: "s", BranchID: "b"}, Session: map[string]float64{}}, res)
	if remaining, _ := m.budget.Remaining(); remaining != 5 {
		t.Fatalf("store failure leaked reservation: remaining=%d", remaining)
	}
}

func TestOverflowRecoveryTargetsAndExplicitFailure(t *testing.T) {
	var mainRequests, compactions int
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), summaryPrompt) {
			compactions++
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"choices":[{"message":{"content":"{\"current_plan\":\"hold\"}"}}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`)
			return
		}
		mainRequests++
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":{"message":"maximum context length exceeded"}}`)
	}))
	defer provider.Close()
	m, _, red, cat := testManager(t, contract.Endpoint{ID: "e", Protocol: "chat_completions", BaseURL: provider.URL, Model: "m"}, nil)
	convID := conversationID("live_question", red.SessionID(), red.BranchID())
	if err := cat.AppendAgentMessage(context.Background(), history.AgentMessage{ConversationID: convID, Role: "user",
		Content: jsonString(strings.Repeat("x", 20_000)), CreatedAtMs: 1}); err != nil {
		t.Fatal(err)
	}
	cfg := m.Store.Config()
	snap := red.Snapshot(nil, nil)
	snap.ObservationSeq = 42
	snap.Metrics = []contract.Metric{{ID: "m-only", Channel: "m.only", Measurement: contract.Available(1.0, "unit", "test")}}
	snap.Events = []contract.Event{}
	inv := contract.Invocation{ID: "inv-overflow", Profile: "live_question", SessionID: red.SessionID(), BranchID: red.BranchID(), Status: "running", StartedAtMs: int64Ptr(time.Now().UnixMilli())}
	if err := m.store(inv); err != nil {
		t.Fatal(err)
	}
	res, ok, _ := m.budget.Reserve(100, 100)
	if !ok {
		t.Fatal("reserve blocked")
	}
	m.execute(context.Background(), inv, cfg, snap, nil, Invocation{}, nil, false, res)
	got := waitInvocation(t, m.Catalog, inv.ID)
	if got.Status != "failed" || got.Error == nil || !strings.Contains(got.Error.Message, "recovery exhausted") {
		t.Fatalf("overflow exhaustion not explicit: %#v", got)
	}
	events, err := m.Catalog.ListAgentEvents(context.Background(), inv.ID)
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, e := range events {
		kinds = append(kinds, e.Kind)
	}
	if mainRequests != 3 || compactions != 2 {
		t.Fatalf("provider sequence main=%d compaction=%d, want 3 and 2 (events %v)", mainRequests, compactions, kinds)
	}
	var compactionCount int
	if err := m.Catalog.DB().QueryRow("SELECT compaction_count FROM invocations WHERE id=?", inv.ID).Scan(&compactionCount); err != nil || compactionCount != 2 {
		t.Fatalf("compaction_count=%d err=%v", compactionCount, err)
	}
	var targets []int
	for _, e := range events {
		if e.Kind != "compaction_started" {
			continue
		}
		var payload struct {
			Trigger          string `json:"trigger"`
			RecentTailTokens int    `json:"recent_tail_tokens"`
		}
		if err := json.Unmarshal(e.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Trigger == "overflow" {
			targets = append(targets, payload.RecentTailTokens)
		}
	}
	if len(targets) != 2 || targets[0] <= targets[1] || targets[1] <= 0 {
		t.Fatalf("overflow targets = %#v, want 70%% then 50%%", targets)
	}
	if a, b, c := recoveryTailTokens(10_000, 70, "", contract.AgentInput{}), recoveryTailTokens(10_000, 50, "", contract.AgentInput{}), recoveryTailTokens(10_000, 35, "", contract.AgentInput{}); !(a > b && b > c && c > 0) {
		t.Fatalf("recovery target sequence not descending: %d %d %d", a, b, c)
	}
}

func TestResponsesReplaysHistoryAndPersistsToolResults(t *testing.T) {
	turn := 0
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		turn++
		w.Header().Set("Content-Type", "application/json")
		if turn == 1 {
			if !strings.Contains(string(body), "call_old") {
				t.Errorf("durable tool result not replayed: %s", string(body))
			}
			json.NewEncoder(w).Encode(map[string]any{
				"id": "resp_1", "object": "response", "created_at": 1, "model": "m", "status": "completed",
				"output": []any{map[string]any{"type": "function_call", "id": "fc_1", "call_id": "call_1", "name": "read_telemetry_window",
					"arguments": `{"from_seq":"0","to_seq":"10","car_indices":[0],"channels":["car.speed_mps"],"max_points":10}`}},
				"output_text": "",
			})
			return
		}
		if !strings.Contains(string(body), "call_1") {
			t.Errorf("new tool result not replayed: %s", string(body))
		}
		o := validOutput()
		raw, _ := json.Marshal(o)
		json.NewEncoder(w).Encode(map[string]any{
			"id": "resp_2", "object": "response", "created_at": 2, "model": "m", "status": "completed",
			"output": []any{map[string]any{"type": "message", "id": "msg_1", "role": "assistant",
				"content": []any{map[string]any{"type": "output_text", "text": string(raw)}}}}, "output_text": string(raw),
		})
	}))
	defer provider.Close()
	var persisted []history.AgentMessage
	inv := &Invoker{Backend: ToolBackend{}}
	in := inputFixture()
	historyMessages := []history.AgentMessage{
		{Role: "user", Content: json.RawMessage(`"prior question"`), CreatedAtMs: 1},
		{Role: "assistant", Content: json.RawMessage(`{"tool_calls":[{"id":"call_old","type":"function","function":{"name":"read_telemetry_window","arguments":"{}"}}]}`), CreatedAtMs: 2},
		{Role: "tool", Content: json.RawMessage(`"old result"`), ToolCallID: "call_old", CreatedAtMs: 3},
	}
	_, err := inv.RunRequest(context.Background(), InvokeRequest{Profile: "scheduled_strategy", Input: in,
		Endpoint: contract.Endpoint{ID: "e", Protocol: "responses", BaseURL: provider.URL, Model: "m", ContextLimitTokens: 16384},
		APIKey:   "sk-test", Snapshot: stateSnapshot(), SystemPrompt: "system", History: historyMessages,
		Activity: &Activity{}, MessageSink: func(m history.AgentMessage) { persisted = append(persisted, m) }})
	if err != nil {
		t.Fatal(err)
	}
	var toolResult, toolCall bool
	for _, m := range persisted {
		if m.Role == "tool" && m.ToolCallID == "call_1" {
			toolResult = true
		}
		if m.Role == "assistant" && strings.Contains(string(m.Content), "call_1") {
			toolCall = true
		}
	}
	if !toolResult || !toolCall {
		t.Fatalf("persisted messages missing tool call/result: %#v", persisted)
	}
}

func TestRecordedPostRaceUsesCatalogEvidence(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var doc struct {
			Messages []struct {
				Content any `json:"content"`
			} `json:"messages"`
		}
		_ = json.Unmarshal(body, &doc)
		var in contract.AgentInput
		for _, msg := range doc.Messages {
			if s, ok := msg.Content.(string); ok && strings.Contains(s, `"invocation_id"`) {
				_ = json.Unmarshal([]byte(s), &in)
			}
		}
		if in.InvocationID == "" {
			t.Errorf("agent input not found in provider request: %s", string(body))
		}
		msg := "Recorded review complete."
		out := contract.AgentOutput{SchemaVersion: contract.SchemaVersion, InvocationID: in.InvocationID,
			SessionID: in.Observation.Context.SessionID, BranchID: in.Observation.Context.BranchID, ObservationSeq: in.Observation.ObservationSeq,
			Decision: "review", Confidence: "medium", Notify: true, Urgency: "routine", DriverMessage: &msg,
			Plan: contract.Plan{Action: "review", Alternatives: []string{}}, RationaleSummary: "Stored laps reviewed.",
			EvidenceIDs: []string{in.Observation.Metrics[0].ID}, Assumptions: []string{}, Predictions: []contract.Prediction{},
			Recheck: contract.Recheck{EventTypes: []string{}}, Analysis: "ok", Limitations: []string{}, Sources: []contract.Source{}, SetupChanges: []contract.SetupChange{}}
		raw, _ := json.Marshal(out)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": string(raw)}}}})
	}))
	defer provider.Close()
	m, _, _, cat := testManager(t, contract.Endpoint{ID: "e", Protocol: "chat_completions", BaseURL: provider.URL, Model: "m"}, nil)
	ctx := context.Background()
	if err := cat.EnsureSourceContext(ctx, "src", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if err := cat.CreateSession(ctx, "hist", contract.Context{SessionID: "hist", BranchID: "br", SourceContextID: "src", ModeFamily: "career", SeasonEvidence: []string{}}); err != nil {
		t.Fatal(err)
	}
	if err := cat.CreateBranch(ctx, contract.Branch{ID: "br", SessionID: "hist", IsSurviving: true}); err != nil {
		t.Fatal(err)
	}
	for _, l := range []contract.LapSummary{
		{ID: "l-0-2", SessionID: "hist", BranchID: "br", CarIndex: 0, Lap: 2, StartSeq: "10", EndSeq: "20", LapTimeMs: contract.Available(int64(90000), "ms", "test"), Compound: contract.Available("C3", "compound", "test"), Valid: true, Complete: true, Gaps: []string{}},
		{ID: "l-1-3", SessionID: "hist", BranchID: "br", CarIndex: 1, Lap: 3, StartSeq: "30", EndSeq: "40", LapTimeMs: contract.Available(int64(91000), "ms", "test"), Compound: contract.Available("C4", "compound", "test"), Valid: true, Complete: true, Gaps: []string{}},
	} {
		if err := cat.InsertLap(ctx, l); err != nil {
			t.Fatal(err)
		}
	}
	if err := cat.UpsertLapSamples(ctx, "hist", "br", history.LapSamples{CarIndex: 0, Lap: 2, StartMs: 0, LapMs: 90000, Points: [][]any{{0.0, 0.9, 0.0, 4.0, 4_000_000.0, "MEDIUM", true, false}}}); err != nil {
		t.Fatal(err)
	}
	seq := "0"
	text := "Review the selected laps.\nSelected laps: 0:2"
	inv, err := m.Request(ctx, contract.Request{Profile: "post_race", SessionID: "hist", BranchID: "br", ObservationSeq: &seq, RequestText: &text})
	if err != nil {
		t.Fatal(err)
	}
	got := waitInvocation(t, m.Catalog, inv.ID)
	if got.Status != "accepted" || got.NotificationDisposition != "delivered" || got.Output == nil {
		t.Fatalf("recorded invocation = %#v", got)
	}
	var inputJSON string
	if err := m.Catalog.DB().QueryRow("SELECT input_json FROM invocations WHERE id=?", inv.ID).Scan(&inputJSON); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inputJSON, "lap:0:2:time") || !strings.Contains(inputJSON, "samples:0:2:throttle") {
		t.Fatalf("recorded evidence missing from input: %s", inputJSON)
	}
	bad := "Review.\nSelected laps: 9:9"
	if _, err := m.Request(ctx, contract.Request{Profile: "post_race", SessionID: "hist", BranchID: "br", ObservationSeq: &seq, RequestText: &bad}); err == nil || !strings.Contains(err.Error(), "not catalogued") {
		t.Fatalf("invalid selected lap not rejected: %v", err)
	}
}

func TestExecuteUsesFrozenSnapshotAndRejectsScopeChange(t *testing.T) {
	var bodies []string
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(body))
		var doc struct {
			Messages []struct {
				Content any `json:"content"`
			} `json:"messages"`
		}
		_ = json.Unmarshal(body, &doc)
		var in contract.AgentInput
		for _, msg := range doc.Messages {
			if s, ok := msg.Content.(string); ok && strings.Contains(s, `"invocation_id"`) {
				_ = json.Unmarshal([]byte(s), &in)
			}
		}
		out := validOutput()
		out.InvocationID, out.SessionID, out.BranchID, out.ObservationSeq = in.InvocationID, in.Observation.Context.SessionID, in.Observation.Context.BranchID, in.Observation.ObservationSeq
		if len(in.Observation.Metrics) > 0 {
			out.EvidenceIDs = []string{in.Observation.Metrics[0].ID}
		}
		raw, _ := json.Marshal(out)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": string(raw)}}}})
	}))
	defer provider.Close()
	m, _, red, _ := testManager(t, contract.Endpoint{ID: "e", Protocol: "chat_completions", BaseURL: provider.URL, Model: "m"}, nil)
	cfg := m.Store.Config()
	snap := red.Snapshot(nil, nil)
	snap.ObservationSeq = 77
	snap.Metrics = []contract.Metric{{ID: "snap-only", Channel: "snap.only", Measurement: contract.Available(1.0, "unit", "test")}}
	inv := contract.Invocation{ID: "inv-snap", Profile: "live_question", SessionID: red.SessionID(), BranchID: red.BranchID(), Status: "running", StartedAtMs: int64Ptr(time.Now().UnixMilli())}
	if err := m.store(inv); err != nil {
		t.Fatal(err)
	}
	res, ok, _ := m.budget.Reserve(100, 100)
	if !ok {
		t.Fatal("reserve blocked")
	}
	m.execute(context.Background(), inv, cfg, snap, nil, Invocation{}, nil, false, res)
	if len(bodies) != 1 || !strings.Contains(bodies[0], "snap-only") || !strings.Contains(bodies[0], "observation_seq") || !strings.Contains(bodies[0], "77") {
		t.Fatalf("provider did not receive frozen snapshot: %#v", bodies)
	}
	stale := inv
	stale.ID = "inv-stale"
	stale.SessionID = "other-session"
	if err := m.store(stale); err != nil {
		t.Fatal(err)
	}
	res, ok, _ = m.budget.Reserve(100, 100)
	if !ok {
		t.Fatal("reserve blocked")
	}
	m.execute(context.Background(), stale, cfg, snap, nil, Invocation{}, nil, false, res)
	if len(bodies) != 1 {
		t.Fatalf("scope change still reached provider: %#v", bodies)
	}
	got := waitInvocation(t, m.Catalog, stale.ID)
	if got.Status != "superseded" {
		t.Fatalf("scope change status = %q", got.Status)
	}
}
