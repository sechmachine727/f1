package engineer

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
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
	Store    *config.Store
	Reducer  *state.Reducer
	Catalog  *history.Catalog
	Invoker  *Invoker
	Arbiter  *strategy.Arbiter
	Activity *Activity

	mu           sync.Mutex
	budget       *Budget
	active       map[string]context.CancelFunc
	rules        *Rules
	sched        *Scheduler
	sessionID    string
	lastEvt      string
	lastEvtSeq   uint64
	lastEvtIDs   []string
	lastLap      int
	lastDispatch time.Time
	cfgRevision  int
	rulesCfg     []contract.TriggerRule
	continuous   Continuous
	contCfg      contract.ContinuousConfig
	contLast     time.Time
}

func NewManager(store *config.Store, r *state.Reducer, cat *history.Catalog, inv *Invoker, arb *strategy.Arbiter) *Manager {
	cfg, rev := store.Snapshot()
	m := &Manager{Store: store, Reducer: r, Catalog: cat, Invoker: inv, Arbiter: arb, budget: NewBudget(cfg.Agent, nil),
		active: map[string]context.CancelFunc{}, rules: NewRules(cfg.Agent.Rules),
		sched: NewScheduler(r.BranchID(), r.ObservationSeq()), sessionID: r.SessionID(), cfgRevision: -1}
	m.syncConfig(cfg, rev, time.Now())
	return m
}

// syncConfig applies saved agent settings on the next dispatch without a restart.
func (m *Manager) syncConfig(cfg contract.Config, rev int, now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rev == m.cfgRevision {
		return
	}
	endpoint, _ := endpointByID(cfg, deref(cfg.Agent.EndpointID))
	m.budget.Configure(cfg.Agent, &endpoint)
	if !reflect.DeepEqual(cfg.Agent.Rules, m.rulesCfg) {
		m.rulesCfg = cfg.Agent.Rules
		m.rules.Configure(cfg.Agent.Rules)
	}
	m.sched.SetDispatchGap(time.Duration(cfg.Agent.MinimumDispatchGapSecs) * time.Second)
	if cfg.Agent.Continuous != m.contCfg {
		m.contCfg = cfg.Agent.Continuous
		if cfg.Agent.Continuous.Enabled {
			m.continuous.Enable(time.Duration(cfg.Agent.Continuous.CadenceSeconds)*time.Second,
				time.Duration(cfg.Agent.Continuous.MaxMinutes)*time.Minute, now)
			m.contLast = time.Time{}
		} else {
			m.continuous = Continuous{}
			m.contLast = time.Time{}
		}
	}
	m.cfgRevision = rev
}

func (m *Manager) Request(ctx context.Context, req contract.Request) (contract.Invocation, error) {
	cfg, rev := m.Store.Snapshot()
	m.syncConfig(cfg, rev, time.Now())
	if !cfg.Agent.Enabled || cfg.Agent.EndpointID == nil {
		return contract.Invocation{}, errors.New("no enabled agent endpoint")
	}
	if _, ok := profileFile[req.Profile]; !ok {
		return contract.Invocation{}, fmt.Errorf("unknown profile %q", req.Profile)
	}
	var snap state.Snapshot
	var recordedObs *contract.Observation
	recorded := false
	branchID := req.BranchID
	if req.Profile == "post_race" && (req.ObservationSeq == nil || *req.ObservationSeq == "0") {
		var obs contract.Observation
		var err error
		snap, obs, err = m.recordedSource(ctx, req)
		if err != nil {
			return contract.Invocation{}, err
		}
		recordedObs = &obs
		recorded = true
		branchID = snap.Context.BranchID
	} else {
		snap = m.Reducer.Snapshot(nil, nil)
		if req.SessionID != snap.Context.SessionID || req.BranchID != snap.Context.BranchID {
			return contract.Invocation{}, errors.New("request scope does not match the live source")
		}
	}
	res, ok, reason := m.budget.Reserve(int64(cfg.Agent.ContextTokens), cfg.Agent.MaxOutputTokens)
	if !ok {
		inv := contract.Invocation{ID: fmt.Sprintf("inv-%d", time.Now().UnixNano()), Profile: req.Profile, SessionID: req.SessionID,
			BranchID: branchID, Status: "budget_blocked", Usage: contract.Usage{UsageEstimated: true},
			NotificationDisposition: "none", PromptVersion: PromptVersion, Error: &contract.JobError{Code: "budget_blocked", Message: reason}}
		_ = m.store(inv)
		_, _ = m.Catalog.DB().Exec("UPDATE invocations SET ended_at_ms=? WHERE id=?", time.Now().UnixMilli(), inv.ID)
		return inv, nil
	}
	id := fmt.Sprintf("inv-%d", time.Now().UnixNano())
	inv := contract.Invocation{ID: id, Profile: req.Profile, SessionID: req.SessionID, BranchID: branchID,
		Status: "running", StartedAtMs: int64Ptr(time.Now().UnixMilli()), TriggerIDs: []string{"manual"},
		Usage: contract.Usage{ProviderRequests: 0, UsageEstimated: true}, NotificationDisposition: "none",
		PromptVersion: PromptVersion}
	if err := m.store(inv); err != nil {
		m.budget.Release(res)
		return inv, err
	}
	m.Activity.Emit(context.Background(), history.AgentEvent{InvocationID: id, SessionID: inv.SessionID, BranchID: inv.BranchID, Profile: inv.Profile, Kind: "queued"}, map[string]any{"trigger_ids": inv.TriggerIDs})
	go m.run(context.Background(), inv, cfg, snap, req.RequestText, recordedObs, recorded, res)
	return inv, nil
}

func (m *Manager) run(ctx context.Context, inv contract.Invocation, cfg contract.Config, snap state.Snapshot, text *string, recordedObs *contract.Observation, recorded bool, res Reservation) {
	m.execute(ctx, inv, cfg, snap, text, Invocation{}, recordedObs, recorded, res)
}

func (m *Manager) execute(ctx context.Context, inv contract.Invocation, cfg contract.Config, snap state.Snapshot, text *string, window Invocation, recordedObs *contract.Observation, recorded bool, res Reservation) {
	providerStarted, settled := false, false
	defer func() {
		if settled {
			return
		}
		if providerStarted {
			m.budget.Reconcile(res, contract.Usage{UsageEstimated: true})
			return
		}
		m.budget.Release(res)
	}()
	if !recorded && (m.Reducer.SessionID() != inv.SessionID || m.Reducer.BranchID() != inv.BranchID) {
		m.sched.Fail(inv.ID, "superseded")
		m.finish(inv.ID, "superseded", "live session or branch changed", nil, contract.Usage{UsageEstimated: true})
		return
	}
	endpoint, ok := endpointByID(cfg, deref(cfg.Agent.EndpointID))
	if !ok {
		m.finish(inv.ID, "failed", "endpoint_missing", nil, contract.Usage{UsageEstimated: true})
		m.Activity.Emit(ctx, history.AgentEvent{InvocationID: inv.ID, SessionID: inv.SessionID, BranchID: inv.BranchID, Profile: inv.Profile, Kind: "error"}, map[string]any{"message": "endpoint_missing"})
		return
	}
	started := time.Now()
	convID := conversationID(inv.Profile, inv.SessionID, inv.BranchID)
	key := m.Store.Secret(deref(endpoint.APIKeyEnv))
	system, promptRevision, promptHash, err := AssembleCurrent(ctx, inv.Profile, m.Catalog)
	if err != nil {
		m.finish(inv.ID, "failed", err.Error(), nil, contract.Usage{UsageEstimated: true})
		return
	}
	_ = m.Catalog.SetInvocationMetadata(ctx, inv.ID, convID, endpoint.ID, endpoint.Model, promptRevision, promptHash)
	continuity := contract.Continuity{MeasuredOutcomes: []string{}, BranchReset: window.Scope.BranchReset}
	if window.Scope.LastAttempt != nil {
		continuity.LastAttempt = window.Scope.LastAttempt
	}
	playerCar := 0
	if snap.Context.PlayerCarIndex != nil {
		playerCar = *snap.Context.PlayerCarIndex
	}
	if prev, err := m.previousAccepted(ctx, inv.SessionID, inv.BranchID, inv.ID); err == nil && prev != nil {
		continuity.PreviousAccepted = prev
		seq := prev.ObservationSeq
		continuity.EvidenceCursor = &seq
		if laps, err := m.Catalog.Laps(ctx, inv.SessionID, inv.BranchID, &playerCar); err == nil {
			current := 0
			if snap.Lap != nil {
				current = *snap.Lap
			} else if !recorded {
				if v, ok := m.Reducer.Value("timing.current_lap_num", playerCar); ok {
					current = int(v)
				}
			}
			if outcomes, err := RecordOutcomes(ctx, m.Catalog, *prev, laps, current); err == nil {
				continuity.MeasuredOutcomes = outcomes
			}
		}
	}
	observation := observationFromSnapshot(snap)
	if recordedObs != nil {
		observation = *recordedObs
	}
	input := contract.AgentInput{
		SchemaVersion: contract.SchemaVersion, InvocationID: inv.ID, ConversationID: convID,
		Profile: inv.Profile, EndpointID: endpoint.ID, Model: endpoint.Model,
		Observation: BoundObservation(observation, playerCar),
		Continuity:  continuity,
		RequestText: text,
		Limits: contract.Limits{MaxSteps: cfg.Agent.MaxSteps, MaxToolCalls: cfg.Agent.MaxToolCalls,
			DeadlineMs: cfg.Agent.LiveDeadlineMs, MaxOutputTokens: cfg.Agent.MaxOutputTokens, MaxContextTokens: cfg.Agent.ContextTokens},
	}
	if inv.Profile == "post_race" || inv.Profile == "setup_research" {
		input.Limits.DeadlineMs = cfg.Agent.AnalysisDeadlineMs
	}
	limit := effectiveLimit(cfg, endpoint)
	tailTokens := limit * cfg.Agent.Compaction.RecentTailPercent / 100
	if tailTokens < 400 {
		tailTokens = 400
	}
	historyMessages, _, _ := m.conversation(ctx, convID, tailTokens)
	if cmp, _ := m.Catalog.LatestCompaction(ctx, convID); cmp != nil {
		summary := summaryText(cmp.Summary)
		historyMessages = append([]history.AgentMessage{{Role: "user", Content: jsonString("[Compacted context]\n" + summary), CreatedAtMs: cmp.CreatedAtMs}}, historyMessages...)
	}
	contextTokens := estimateTokens([]byte(system)) + conversationTokens(historyMessages) + estimateTokens(rawJSON(input))
	if last := m.Catalog.LastInputTokens(ctx, convID); last > 0 {
		if candidate := last + estimateTokens(rawJSON(input)); candidate > contextTokens {
			contextTokens = candidate
		}
	}
	compactions := 0
	if cfg.Agent.Compaction.Enabled && contextTokens >= limit*cfg.Agent.Compaction.TriggerPercent/100 {
		if cmp, err := m.compact(ctx, inv, cfg, endpoint, key, convID, "threshold", "", contextTokens, tailTokens); err == nil && cmp != nil {
			compactions++
			historyMessages, _, _ = m.conversation(ctx, convID, tailTokens)
			if latest, _ := m.Catalog.LatestCompaction(ctx, convID); latest != nil {
				historyMessages = append([]history.AgentMessage{{Role: "user", Content: jsonString("[Compacted context]\n" + summaryText(latest.Summary)), CreatedAtMs: latest.CreatedAtMs}}, historyMessages...)
			}
			contextTokens = estimateTokens([]byte(system)) + conversationTokens(historyMessages) + estimateTokens(rawJSON(input))
		}
	}
	m.Activity.Emit(ctx, history.AgentEvent{InvocationID: inv.ID, SessionID: inv.SessionID, BranchID: inv.BranchID, Profile: inv.Profile, Kind: "context_built"}, map[string]any{
		"system_prompt": system, "prompt_revision_id": promptRevision, "prompt_sha256": promptHash,
		"messages": historyMessages, "input": input, "token_estimate": contextTokens, "context_limit": limit,
	})
	if !recorded && (m.Reducer.SessionID() != inv.SessionID || m.Reducer.BranchID() != inv.BranchID) {
		m.sched.Fail(inv.ID, "superseded")
		m.finish(inv.ID, "superseded", "live session or branch changed", nil, contract.Usage{UsageEstimated: true})
		return
	}
	providerStarted = true
	cctx, cancel := context.WithTimeout(ctx, time.Duration(input.Limits.DeadlineMs)*time.Millisecond)
	defer cancel()
	m.mu.Lock()
	m.active[inv.ID] = cancel
	m.mu.Unlock()
	var result InvokeResult
	var requests int
	var inputTokens, outputTokens, totalTokens *int64
	// Successive overflow retries compact toward 70%, then 50%, then 35% of
	// the rejected request. The two-compaction cap makes 70/50 reachable and
	// stops with an explicit error before the 35% retry.
	overflowTargets := []int{70, 50, 35}
	overflowAttempt := 0
	for {
		result, err = m.Invoker.RunRequest(cctx, InvokeRequest{
			Profile: inv.Profile, Input: input, Endpoint: endpoint, APIKey: key, Snapshot: snap,
			SystemPrompt: system, PromptRevision: promptRevision, PromptHash: promptHash, ConversationID: convID,
			History: historyMessages, Generation: cfg.Agent.Generation, Activity: m.Activity,
			MessageSink: func(msg history.AgentMessage) {
				msg.ConversationID = convID
				if msg.CreatedAtMs == 0 {
					msg.CreatedAtMs = time.Now().UnixMilli()
				}
				_ = m.Catalog.AppendAgentMessage(context.Background(), msg)
			},
		})
		requests += result.Requests
		inputTokens = addToken(inputTokens, result.InputTokens)
		outputTokens = addToken(outputTokens, result.OutputTokens)
		totalTokens = addToken(totalTokens, result.TotalTokens)
		if err == nil || !isContextOverflow(err) {
			break
		}
		if compactions >= 2 || overflowAttempt >= len(overflowTargets) {
			err = fmt.Errorf("context overflow recovery exhausted after %d compactions: %w", compactions, err)
			break
		}
		target := overflowTargets[overflowAttempt]
		overflowAttempt++
		recoveryTail := recoveryTailTokens(contextTokens, target, system, input)
		if cmp, cerr := m.compact(ctx, inv, cfg, endpoint, key, convID, "overflow", "", contextTokens, recoveryTail); cerr != nil || cmp == nil {
			if cerr != nil {
				err = fmt.Errorf("context overflow recovery failed: %w", cerr)
			} else {
				err = errors.New("context overflow recovery failed: no compaction produced")
			}
			break
		}
		compactions++
		historyMessages, _, _ = m.conversation(ctx, convID, recoveryTail)
		if latest, _ := m.Catalog.LatestCompaction(ctx, convID); latest != nil {
			historyMessages = append([]history.AgentMessage{{Role: "user", Content: jsonString("[Compacted context]\n" + summaryText(latest.Summary)), CreatedAtMs: latest.CreatedAtMs}}, historyMessages...)
		}
		contextTokens = estimateTokens([]byte(system)) + conversationTokens(historyMessages) + estimateTokens(rawJSON(input))
	}
	usage := contract.Usage{ProviderRequests: requests, InputTokens: inputTokens, OutputTokens: outputTokens, TotalTokens: totalTokens, UsageEstimated: inputTokens == nil}
	if requests == 0 {
		m.budget.Release(res)
	} else {
		m.budget.Reconcile(res, usage)
	}
	settled = true
	if err != nil {
		m.sched.Fail(inv.ID, "provider_error")
		m.Activity.Emit(ctx, history.AgentEvent{InvocationID: inv.ID, SessionID: inv.SessionID, BranchID: inv.BranchID, Profile: inv.Profile, Kind: "error"}, map[string]any{"message": err.Error()})
		m.finish(inv.ID, "failed", err.Error(), nil, usage)
		_ = m.Catalog.UpdateInvocationAudit(context.Background(), inv.ID, rawJSON(input), time.Since(started).Milliseconds(), compactions)
		return
	}
	out, err := ParseAndValidate(result.Text, input)
	if err != nil {
		m.sched.Fail(inv.ID, "invalid")
		m.Activity.Emit(ctx, history.AgentEvent{InvocationID: inv.ID, SessionID: inv.SessionID, BranchID: inv.BranchID, Profile: inv.Profile, Kind: "rejected"}, map[string]any{"error": err.Error(), "output": result.Text})
		m.finish(inv.ID, "invalid", err.Error(), nil, usage)
		_ = m.Catalog.UpdateInvocationAudit(context.Background(), inv.ID, rawJSON(input), time.Since(started).Milliseconds(), compactions)
		return
	}
	disposition := "none"
	if out.Notify {
		disposition = "delivered"
	}
	// Accept only a current-branch strategy result; this advances the evidence
	// cursor by the frozen window and nothing else.
	if inv.Profile == "scheduled_strategy" || inv.Profile == "continuous_strategy" {
		if err := m.sched.Accept(inv.ID, window.Window); err != nil {
			m.finish(inv.ID, "superseded", err.Error(), nil, usage)
			_ = m.Catalog.UpdateInvocationAudit(context.Background(), inv.ID, rawJSON(input), time.Since(started).Milliseconds(), compactions)
			return
		}
	}
	m.finish(inv.ID, "accepted", "", &out, usage)
	_ = m.Catalog.UpdateInvocationAudit(context.Background(), inv.ID, rawJSON(input), time.Since(started).Milliseconds(), compactions)
	m.Activity.Emit(ctx, history.AgentEvent{InvocationID: inv.ID, SessionID: inv.SessionID, BranchID: inv.BranchID, Profile: inv.Profile, Kind: "accepted"}, map[string]any{
		"output": out, "notification_disposition": disposition, "latency_ms": time.Since(started).Milliseconds(), "compaction_count": compactions,
	})
	m.Activity.Emit(ctx, history.AgentEvent{InvocationID: inv.ID, SessionID: inv.SessionID, BranchID: inv.BranchID, Profile: inv.Profile, Kind: "notification"}, map[string]any{"disposition": disposition})
}

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
	disposition := "none"
	if out != nil && out.Notify {
		disposition = "delivered"
	}
	m.Catalog.DB().Exec("UPDATE invocations SET status=?, output_json=?, error_json=?, usage_json=?, notification_disposition=?, ended_at_ms=? WHERE id=?", status, outJSON, errJSON, string(u), disposition, time.Now().UnixMilli(), id)
}

func (m *Manager) store(inv contract.Invocation) error {
	usage, _ := json.Marshal(inv.Usage)
	triggers, _ := json.Marshal(inv.TriggerIDs)
	_, err := m.Catalog.DB().Exec("INSERT INTO invocations(id, session_id, branch_id, status, output_json, usage_json, profile, started_at_ms, notification_disposition, trigger_ids_json) VALUES(?,?,?,?,NULL,?,?,?,?,?)",
		inv.ID, inv.SessionID, inv.BranchID, inv.Status, string(usage), inv.Profile, inv.StartedAtMs, inv.NotificationDisposition, string(triggers))
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
	m.Activity.Emit(ctx, history.AgentEvent{InvocationID: id, Kind: "cancelled"}, map[string]any{"reason": "cancelled by user"})
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
	cfg, rev := m.Store.Snapshot()
	m.syncConfig(cfg, rev, time.Now())
	if !cfg.Agent.Enabled || cfg.Agent.EndpointID == nil {
		return
	}
	now := time.Now()
	m.mu.Lock()
	newSession := m.sessionID != "" && snap.Context.SessionID != m.sessionID
	if newSession {
		m.sched.NewSession(snap.Context.BranchID)
		m.rules.Reset()
		m.lastEvt, m.lastEvtSeq, m.lastEvtIDs, m.lastLap, m.contLast, m.lastDispatch = "", 0, nil, 0, time.Time{}, time.Time{}
	}
	m.sessionID = snap.Context.SessionID
	lastDispatch := m.lastDispatch
	m.mu.Unlock()
	// A flashback moves the surviving branch; pending evidence before the anchor
	// stays queued and all cursors reset.
	if snap.Context.BranchID != m.sched.Scope().Branch {
		m.sched.Flashback(snap.Context.BranchID, snap.ObservationSeq+1)
	}
	gap := time.Duration(cfg.Agent.MinimumDispatchGapSecs) * time.Second
	if gap > 0 && !lastDispatch.IsZero() && now.Sub(lastDispatch) < gap {
		return
	}
	contDue := false
	m.mu.Lock()
	if m.continuous.Tick(now) {
		contDue = m.contLast.IsZero() || now.Sub(m.contLast) >= m.continuous.Cadence
	}
	var evts []PendingEvent
	for _, e := range snap.Events {
		seq, _ := strconv.ParseUint(e.Seq, 10, 64)
		if seq > 0 {
			if seq < m.lastEvtSeq {
				continue
			}
			if seq == m.lastEvtSeq && containsString(m.lastEvtIDs, e.ID) {
				continue
			}
			if seq > m.lastEvtSeq {
				m.lastEvtSeq = seq
				m.lastEvtIDs = m.lastEvtIDs[:0]
			}
			m.lastEvtIDs = append(m.lastEvtIDs, e.ID)
		} else if e.ID == m.lastEvt {
			continue
		}
		m.lastEvt = e.ID
		evts = append(evts, PendingEvent{ID: e.ID, Type: e.Type, Severity: e.Severity, CarIndex: e.CarIndex})
		m.sched.Event(PendingEvent{Seq: seq, ID: e.ID, Type: e.Type, Severity: e.Severity, CarIndex: e.CarIndex})
	}
	laps := 0
	if snap.Lap != nil && *snap.Lap > m.lastLap {
		laps = *snap.Lap - m.lastLap
		m.lastLap = *snap.Lap
	}
	m.mu.Unlock()
	if len(evts) == 0 && laps == 0 && !contDue {
		return
	}
	ids := m.rules.TriggerWithContext(TriggerContext{ModeFamily: snap.Context.ModeFamily, PlayerCar: playerCarIndex(snap), Events: evts, Laps: laps})
	if len(ids) == 0 && !contDue {
		return
	}
	if m.sched.IsBudgetBlocked() {
		if !m.budget.Available() {
			return
		}
		m.sched.BudgetBlockedNow(false)
	}
	profile := "scheduled_strategy"
	triggers := ids
	if contDue {
		profile = "continuous_strategy"
		triggers = append(triggers, "continuous")
	}
	id := fmt.Sprintf("inv-%d", time.Now().UnixNano())
	inv, err := m.sched.Dispatch(id, snap.ObservationSeq, now)
	if err != nil {
		return
	}
	res, ok, _ := m.budget.Reserve(int64(cfg.Agent.ContextTokens), cfg.Agent.MaxOutputTokens)
	if !ok {
		m.sched.BudgetBlockedNow(true)
		m.sched.Fail(id, "budget_blocked")
		return
	}
	m.mu.Lock()
	m.lastDispatch = now
	if contDue {
		m.contLast = now
	}
	m.mu.Unlock()
	m.startScheduled(inv, triggers, profile, cfg, snap, res)
}

// startScheduled runs a software-triggered invocation: the frozen window comes
// from the scheduler, and only an accepted result advances evidence.
func (m *Manager) startScheduled(inv Invocation, triggerIDs []string, profile string, cfg contract.Config, snap state.Snapshot, res Reservation) {
	rec := contract.Invocation{ID: inv.ID, Profile: profile, SessionID: snap.Context.SessionID,
		BranchID: inv.Branch, Status: "running", StartedAtMs: int64Ptr(time.Now().UnixMilli()), TriggerIDs: triggerIDs,
		Usage: contract.Usage{UsageEstimated: true}, NotificationDisposition: "none", PromptVersion: PromptVersion}
	if err := m.store(rec); err != nil {
		m.budget.Release(res)
		m.sched.Fail(inv.ID, "store_failed")
		return
	}
	m.Activity.Emit(context.Background(), history.AgentEvent{InvocationID: rec.ID, SessionID: rec.SessionID, BranchID: rec.BranchID, Profile: rec.Profile, Kind: "queued"}, map[string]any{"trigger_ids": triggerIDs})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				m.sched.Fail(inv.ID, "panic")
				m.finish(inv.ID, "failed", fmt.Sprint(r), nil, contract.Usage{UsageEstimated: true})
			}
		}()
		m.execute(context.Background(), rec, cfg, snap, nil, inv, nil, false, res)
	}()
}

func playerCarIndex(snap state.Snapshot) int {
	if snap.Context.PlayerCarIndex == nil {
		return -1
	}
	return *snap.Context.PlayerCarIndex
}

// observationFromSnapshot freezes the dispatch snapshot instead of reading live
// reducer state after the invocation has started.
func observationFromSnapshot(snap state.Snapshot) contract.Observation {
	seq := strconv.FormatUint(snap.ObservationSeq, 10)
	captured := time.Now().UnixMilli()
	if !snap.LastAcceptedAt.IsZero() {
		captured = snap.LastAcceptedAt.UnixMilli()
	}
	metrics := snap.Metrics
	if metrics == nil {
		metrics = []contract.Metric{}
	}
	events := snap.Events
	if events == nil {
		events = []contract.Event{}
	}
	return contract.Observation{
		SchemaVersion: contract.SchemaVersion, Context: snap.Context, ObservationSeq: seq,
		CapturedAtMs: captured, SessionTimeMs: snap.SessionTimeMs, Lap: snap.Lap,
		Window: contract.Range{FromSeq: "0", ToSeq: seq}, Metrics: metrics, Events: events,
		Estimates: []contract.Estimate{},
		Coverage:  contract.Coverage{Gaps: []string{}, StaleChannels: staleMetricIDs(metrics)},
	}
}

func staleMetricIDs(metrics []contract.Metric) []string {
	out := []string{}
	for _, m := range metrics {
		if m.Measurement.Availability == "stale" || m.Measurement.Availability == "not_received" {
			out = append(out, m.ID)
		}
	}
	if len(out) > 128 {
		out = out[:128]
	}
	return out
}

func addToken(sum *int64, v *int64) *int64 {
	if v == nil {
		return sum
	}
	if sum == nil {
		return int64Ptr(*v)
	}
	next := *sum + *v
	return &next
}

// recoveryTailTokens keeps the fixed prompt and input under the recovery
// target, leaving the remainder as verbatim recent history.
func recoveryTailTokens(contextTokens, targetPercent int, system string, input contract.AgentInput) int {
	fixed := estimateTokens([]byte(system)) + estimateTokens(rawJSON(input))
	tail := contextTokens*targetPercent/100 - fixed
	if tail < 400 {
		tail = 400
	}
	return tail
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

func containsString(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

var _ = strconv.Itoa
