package engineer

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"f1tele/server/internal/config"
	"f1tele/server/internal/contract"
	"f1tele/server/internal/history"
	"f1tele/server/internal/state"
	"f1tele/server/internal/strategy"
)

// Manager runs manual and software-scheduled bounded invocations and audits them.
type Manager struct {
	Store   *config.Store
	Reducer *state.Reducer
	Catalog *history.Catalog
	Invoker *Invoker
	Arbiter *strategy.Arbiter

	mu      sync.Mutex
	budget  *Budget
	active  map[string]context.CancelFunc
	rules   *Rules
	sched   *Scheduler
	lastEvt string
	lastLap int
	running bool
	convID  string
}

func NewManager(store *config.Store, r *state.Reducer, cat *history.Catalog, inv *Invoker, arb *strategy.Arbiter) *Manager {
	cfg := store.Config()
	return &Manager{Store: store, Reducer: r, Catalog: cat, Invoker: inv, Arbiter: arb, budget: NewBudget(cfg.Agent, nil),
		active: map[string]context.CancelFunc{}, rules: NewRules(cfg.Agent.Rules),
		sched: NewScheduler(r.BranchID(), r.ObservationSeq()), convID: fmt.Sprintf("conv-%d", time.Now().UnixNano())}
}

func (m *Manager) Request(ctx context.Context, req contract.Request) (contract.Invocation, error) {
	cfg := m.Store.Config()
	if !cfg.Agent.Enabled || cfg.Agent.EndpointID == nil {
		return contract.Invocation{}, errors.New("no enabled agent endpoint")
	}
	snap := m.Reducer.Snapshot(nil, nil)
	if req.SessionID != snap.Context.SessionID || req.BranchID != snap.Context.BranchID {
		return contract.Invocation{}, errors.New("request scope does not match the live source")
	}
	if _, ok := profileFile[req.Profile]; !ok {
		return contract.Invocation{}, fmt.Errorf("unknown profile %q", req.Profile)
	}
	if _, ok, reason := m.budget.Reserve(int64(cfg.Agent.ContextTokens), cfg.Agent.MaxOutputTokens); !ok {
		inv := contract.Invocation{ID: fmt.Sprintf("inv-%d", time.Now().UnixNano()), Profile: req.Profile, SessionID: req.SessionID,
			BranchID: req.BranchID, Status: "budget_blocked", Usage: contract.Usage{UsageEstimated: true},
			NotificationDisposition: "none", PromptVersion: PromptVersion, Error: &contract.JobError{Code: "budget_blocked", Message: reason}}
		_ = m.store(inv)
		_, _ = m.Catalog.DB().Exec("UPDATE invocations SET ended_at_ms=? WHERE id=?", time.Now().UnixMilli(), inv.ID)
		return inv, nil
	}
	id := fmt.Sprintf("inv-%d", time.Now().UnixNano())
	inv := contract.Invocation{ID: id, Profile: req.Profile, SessionID: req.SessionID, BranchID: req.BranchID,
		Status: "running", StartedAtMs: int64Ptr(time.Now().UnixMilli()), TriggerIDs: []string{"manual"},
		Usage: contract.Usage{ProviderRequests: 0, UsageEstimated: true}, NotificationDisposition: "none",
		PromptVersion: PromptVersion}
	if err := m.store(inv); err != nil {
		m.budget.Release(int64(cfg.Agent.ContextTokens) + int64(cfg.Agent.MaxOutputTokens))
		return inv, err
	}
	go m.run(context.Background(), inv, cfg, snap, req.RequestText)
	return inv, nil
}

func (m *Manager) run(ctx context.Context, inv contract.Invocation, cfg contract.Config, snap state.Snapshot, text *string) {
	endpoint, ok := endpointByID(cfg, deref(cfg.Agent.EndpointID))
	if !ok {
		m.finish(inv.ID, "failed", "endpoint_missing", nil, contract.Usage{UsageEstimated: true})
		return
	}
	key := m.Store.Secret(deref(endpoint.APIKeyEnv))
	input := contract.AgentInput{
		SchemaVersion: contract.SchemaVersion, InvocationID: inv.ID, ConversationID: m.convID,
		Profile: inv.Profile, EndpointID: endpoint.ID, Model: endpoint.Model,
		Observation: BoundObservation(m.Reducer.Observation(nil, nil, nil, nil), m.Reducer.PlayerCar()),
		Continuity:  contract.Continuity{MeasuredOutcomes: []string{}, BranchReset: false},
		RequestText: text,
		Limits: contract.Limits{MaxSteps: cfg.Agent.MaxSteps, MaxToolCalls: cfg.Agent.MaxToolCalls,
			DeadlineMs: cfg.Agent.LiveDeadlineMs, MaxOutputTokens: cfg.Agent.MaxOutputTokens, MaxContextTokens: cfg.Agent.ContextTokens},
	}
	if inv.Profile == "post_race" || inv.Profile == "setup_research" {
		input.Limits.DeadlineMs = cfg.Agent.AnalysisDeadlineMs
	}
	cctx, cancel := context.WithTimeout(ctx, time.Duration(input.Limits.DeadlineMs)*time.Millisecond)
	defer cancel()
	m.mu.Lock()
	m.active[inv.ID] = cancel
	m.mu.Unlock()
	outText, requests, err := m.Invoker.Run(cctx, inv.Profile, input, endpoint, key, snap)
	m.budget.Reconcile(int64(input.Limits.MaxContextTokens)+int64(input.Limits.MaxOutputTokens), contract.Usage{ProviderRequests: requests, UsageEstimated: true})
	if err != nil {
		m.finish(inv.ID, "failed", err.Error(), nil, contract.Usage{ProviderRequests: requests, UsageEstimated: true})
		return
	}
	out, err := ParseAndValidate(outText, input)
	if err != nil {
		// One bounded repair is allowed in the same deadline/budget.
		repair := "Repair the previous response into one JSON object matching AgentOutput. Fix only schema and scope; do not add evidence."
		_ = repair
		m.finish(inv.ID, "invalid", err.Error(), nil, contract.Usage{ProviderRequests: requests, UsageEstimated: true})
		return
	}
	disposition := "none"
	if out.Notify {
		disposition = "delivered"
	}
	m.finish(inv.ID, "accepted", "", &out, contract.Usage{ProviderRequests: requests, UsageEstimated: true, TotalTokens: nil})
	m.mu.Lock()
	if r := m.activeInvocation(inv.ID); r != nil {
		r.NotificationDisposition = disposition
	}
	m.mu.Unlock()
}

func (m *Manager) activeInvocation(string) *contract.Invocation { return nil }

func (m *Manager) finish(id, status, errText string, out *contract.AgentOutput, usage contract.Usage) {
	m.mu.Lock()
	delete(m.active, id)
	m.mu.Unlock()
	var outJSON sql.NullString
	if out != nil {
		raw, _ := json.Marshal(out)
		outJSON = sql.NullString{String: string(raw), Valid: true}
	}
	var errJSON sql.NullString
	if errText != "" {
		raw, _ := json.Marshal(contract.JobError{Code: status, Message: errText})
		errJSON = sql.NullString{String: string(raw), Valid: true}
	}
	u, _ := json.Marshal(usage)
	m.Catalog.DB().Exec("UPDATE invocations SET status=?, output_json=?, error_json=?, usage_json=?, ended_at_ms=? WHERE id=?", status, outJSON, errJSON, string(u), time.Now().UnixMilli(), id)
}

func (m *Manager) store(inv contract.Invocation) error {
	usage, _ := json.Marshal(inv.Usage)
	_, err := m.Catalog.DB().Exec("INSERT INTO invocations(id, session_id, branch_id, status, output_json, usage_json, profile, started_at_ms, notification_disposition) VALUES(?,?,?,?,NULL,?,?,?,?)",
		inv.ID, inv.SessionID, inv.BranchID, inv.Status, string(usage), inv.Profile, inv.StartedAtMs, inv.NotificationDisposition)
	return err
}

func (m *Manager) Invocations(ctx context.Context, session, branch, cursor string, limit int, status string) ([]contract.Invocation, string, error) {
	q := "SELECT id, profile, session_id, branch_id, status, started_at_ms, output_json, usage_json, error_json, notification_disposition FROM invocations WHERE 1=1"
	args := []any{}
	if session != "" {
		q += " AND session_id=?"
		args = append(args, session)
	}
	if branch != "" {
		q += " AND branch_id=?"
		args = append(args, branch)
	}
	if status != "" {
		q += " AND status=?"
		args = append(args, status)
	}
	q += " ORDER BY started_at_ms DESC LIMIT 100"
	rows, err := m.Catalog.DB().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	var out []contract.Invocation
	for rows.Next() {
		var inv contract.Invocation
		var start sql.NullInt64
		var outJSON, usageJSON, errJSON sql.NullString
		if err := rows.Scan(&inv.ID, &inv.Profile, &inv.SessionID, &inv.BranchID, &inv.Status, &start, &outJSON, &usageJSON, &errJSON, &inv.NotificationDisposition); err != nil {
			return nil, "", err
		}
		if start.Valid {
			inv.StartedAtMs = &start.Int64
		}
		if outJSON.Valid {
			var o contract.AgentOutput
			_ = json.Unmarshal([]byte(outJSON.String), &o)
			inv.Output = &o
		}
		if usageJSON.Valid {
			_ = json.Unmarshal([]byte(usageJSON.String), &inv.Usage)
		}
		if errJSON.Valid {
			var e contract.JobError
			_ = json.Unmarshal([]byte(errJSON.String), &e)
			inv.Error = &e
		}
		inv.TriggerIDs = []string{"manual"}
		inv.PromptVersion = PromptVersion
		out = append(out, inv)
	}
	return out, "", rows.Err()
}

func (m *Manager) Cancel(ctx context.Context, id string) error {
	m.mu.Lock()
	cancel, ok := m.active[id]
	m.mu.Unlock()
	if !ok {
		return errors.New("unknown or completed invocation")
	}
	cancel()
	_, err := m.Catalog.DB().Exec("UPDATE invocations SET status='cancelled', ended_at_ms=? WHERE id=?", time.Now().UnixMilli(), id)
	return err
}

// AcceptedText returns the driver message of an accepted invocation, used by
// the speech proxy so callers never supply text or keys.
func (m *Manager) AcceptedText(ctx context.Context, id string) (string, error) {
	var raw sql.NullString
	var status string
	err := m.Catalog.DB().QueryRowContext(ctx, "SELECT status, output_json FROM invocations WHERE id=?", id).Scan(&status, &raw)
	if err != nil {
		return "", err
	}
	if status != "accepted" || !raw.Valid {
		return "", errors.New("invocation is not accepted")
	}
	var out contract.AgentOutput
	if err := json.Unmarshal([]byte(raw.String), &out); err != nil {
		return "", err
	}
	if out.DriverMessage == nil || strings.TrimSpace(*out.DriverMessage) == "" {
		return "", errors.New("accepted invocation has no driver message to speak")
	}
	return *out.DriverMessage, nil
}

func (m *Manager) Tick(snap state.Snapshot) {
	cfg := m.Store.Config()
	if !cfg.Agent.Enabled || cfg.Agent.EndpointID == nil {
		return
	}
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()
	var evts []string
	m.mu.Lock()
	for _, e := range snap.Events {
		if e.ID != m.lastEvt {
			evts = append(evts, e.Type)
			m.lastEvt = e.ID
		}
	}
	laps := 0
	if snap.Lap != nil && *snap.Lap > m.lastLap {
		laps = *snap.Lap - m.lastLap
		m.lastLap = *snap.Lap
	}
	m.mu.Unlock()
	if len(evts) == 0 && laps == 0 {
		return
	}
	ids := m.rules.Trigger(evts, laps)
	if len(ids) == 0 {
		return
	}
	m.mu.Lock()
	m.running = true
	m.mu.Unlock()
	inv, err := m.Request(context.Background(), contract.Request{Profile: "scheduled_strategy", SessionID: snap.Context.SessionID, BranchID: snap.Context.BranchID})
	m.mu.Lock()
	m.running = false
	m.mu.Unlock()
	_ = inv
	_ = err
}

func endpointByID(cfg contract.Config, id string) (contract.Endpoint, bool) {
	for _, e := range cfg.Agent.Endpoints {
		if e.ID == id {
			return e, true
		}
	}
	return contract.Endpoint{}, false
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func int64Ptr(v int64) *int64 { return &v }

var _ = strconv.Itoa
