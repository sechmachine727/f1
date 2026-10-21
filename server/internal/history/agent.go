package history

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"time"

	"f1tele/server/internal/contract"
)

type AgentEvent struct {
	Seq           int64           `json:"seq"`
	ID            string          `json:"id"`
	InvocationID  string          `json:"invocation_id"`
	SessionID     string          `json:"session_id"`
	BranchID      string          `json:"branch_id"`
	Profile       string          `json:"profile"`
	Kind          string          `json:"kind"`
	AtMs          int64           `json:"at_ms"`
	DurationMs    *int64          `json:"duration_ms"`
	Payload       json.RawMessage `json:"payload"`
	PayloadSHA256 string          `json:"payload_sha256"`
	Truncated     bool            `json:"truncated"`
}

type AgentCompaction struct {
	ID             string          `json:"id"`
	ConversationID string          `json:"conversation_id"`
	InvocationID   string          `json:"invocation_id"`
	Trigger        string          `json:"trigger"`
	Focus          string          `json:"focus"`
	FromSeq        int64           `json:"from_seq"`
	ToSeq          int64           `json:"to_seq"`
	BeforeTokens   int64           `json:"before_tokens"`
	AfterTokens    int64           `json:"after_tokens"`
	Summary        json.RawMessage `json:"summary"`
	Model          string          `json:"model"`
	Usage          contract.Usage  `json:"usage"`
	CreatedAtMs    int64           `json:"created_at_ms"`
}

type AgentMessage struct {
	Seq            int64           `json:"-"`
	ID             string          `json:"id"`
	ConversationID string          `json:"conversation_id"`
	Role           string          `json:"role"`
	Content        json.RawMessage `json:"content"`
	ToolCallID     string          `json:"tool_call_id,omitempty"`
	InvocationID   string          `json:"invocation_id,omitempty"`
	CompactedInto  string          `json:"compacted_into,omitempty"`
	CreatedAtMs    int64           `json:"created_at_ms"`
}

func (c *Catalog) AppendAgentEvent(ctx context.Context, e AgentEvent) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e.ID == "" {
		e.ID = newID("evt")
	}
	payload := normalizeJSON(e.Payload)
	sum := sha256.Sum256(payload)
	duration := sql.NullInt64{}
	if e.DurationMs != nil {
		duration = sql.NullInt64{Int64: *e.DurationMs, Valid: true}
	}
	res, err := c.db.ExecContext(ctx, `INSERT INTO agent_events(id, invocation_id, session_id, branch_id, profile, kind, at_ms, duration_ms, payload_json, payload_sha256, truncated)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)`, e.ID, e.InvocationID, e.SessionID, e.BranchID, e.Profile, e.Kind, e.AtMs, duration, string(payload), hex.EncodeToString(sum[:]), boolInt(e.Truncated))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (c *Catalog) ListAgentEvents(ctx context.Context, invocationID string) ([]AgentEvent, error) {
	rows, err := c.db.QueryContext(ctx, `SELECT seq, id, invocation_id, session_id, branch_id, profile, kind, at_ms, duration_ms, payload_json, payload_sha256, truncated
		FROM agent_events WHERE invocation_id=? ORDER BY seq`, invocationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAgentEvents(rows)
}

func (c *Catalog) AgentEventsAfter(ctx context.Context, seq int64, sessionID, branchID string, limit int) ([]AgentEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	q := `SELECT seq, id, invocation_id, session_id, branch_id, profile, kind, at_ms, duration_ms, payload_json, payload_sha256, truncated
		FROM agent_events WHERE seq>?`
	args := []any{seq}
	if sessionID != "" {
		q += " AND session_id=?"
		args = append(args, sessionID)
	}
	if branchID != "" {
		q += " AND branch_id=?"
		args = append(args, branchID)
	}
	q += " ORDER BY seq LIMIT ?"
	args = append(args, limit)
	rows, err := c.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAgentEvents(rows)
}

func scanAgentEvents(rows *sql.Rows) ([]AgentEvent, error) {
	out := []AgentEvent{}
	for rows.Next() {
		var e AgentEvent
		var duration sql.NullInt64
		var invocationID sql.NullString
		var payload string
		if err := rows.Scan(&e.Seq, &e.ID, &invocationID, &e.SessionID, &e.BranchID, &e.Profile, &e.Kind, &e.AtMs, &duration, &payload, &e.PayloadSHA256, &e.Truncated); err != nil {
			return nil, err
		}
		e.InvocationID = invocationID.String
		if duration.Valid {
			e.DurationMs = &duration.Int64
		}
		e.Payload = json.RawMessage(normalizeJSON([]byte(payload)))
		out = append(out, e)
	}
	return out, rows.Err()
}

func (c *Catalog) SaveCompaction(ctx context.Context, m AgentCompaction) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	usage, _ := json.Marshal(m.Usage)
	_, err := c.db.ExecContext(ctx, `INSERT INTO agent_compactions(id, conversation_id, invocation_id, trigger, focus, from_seq, to_seq, before_tokens, after_tokens, summary_json, model, usage_json, created_at_ms)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, m.ID, m.ConversationID, m.InvocationID, m.Trigger, m.Focus, m.FromSeq, m.ToSeq, m.BeforeTokens, m.AfterTokens, string(normalizeJSON(m.Summary)), m.Model, string(usage), m.CreatedAtMs)
	return err
}

func (c *Catalog) ListCompactions(ctx context.Context, conversationID string) ([]AgentCompaction, error) {
	rows, err := c.db.QueryContext(ctx, `SELECT id, conversation_id, invocation_id, trigger, focus, from_seq, to_seq, before_tokens, after_tokens, summary_json, model, usage_json, created_at_ms
		FROM agent_compactions WHERE conversation_id=? ORDER BY created_at_ms DESC`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCompactions(rows)
}

func (c *Catalog) LatestCompaction(ctx context.Context, conversationID string) (*AgentCompaction, error) {
	rows, err := c.db.QueryContext(ctx, `SELECT id, conversation_id, invocation_id, trigger, focus, from_seq, to_seq, before_tokens, after_tokens, summary_json, model, usage_json, created_at_ms
		FROM agent_compactions WHERE conversation_id=? ORDER BY created_at_ms DESC LIMIT 1`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list, err := scanCompactions(rows)
	if err != nil || len(list) == 0 {
		return nil, err
	}
	return &list[0], nil
}

func scanCompactions(rows *sql.Rows) ([]AgentCompaction, error) {
	out := []AgentCompaction{}
	for rows.Next() {
		var m AgentCompaction
		var focus, summary, usage string
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.InvocationID, &m.Trigger, &focus, &m.FromSeq, &m.ToSeq, &m.BeforeTokens, &m.AfterTokens, &summary, &m.Model, &usage, &m.CreatedAtMs); err != nil {
			return nil, err
		}
		m.Focus = focus
		m.Summary = json.RawMessage(normalizeJSON([]byte(summary)))
		_ = json.Unmarshal([]byte(usage), &m.Usage)
		out = append(out, m)
	}
	return out, rows.Err()
}

func (c *Catalog) AppendAgentMessage(ctx context.Context, m AgentMessage) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if m.ID == "" {
		m.ID = newID("msg")
	}
	_, err := c.db.ExecContext(ctx, `INSERT INTO agent_messages(id, conversation_id, role, content_json, tool_call_id, invocation_id, created_at_ms)
		VALUES(?,?,?,?,?,?,?)`, m.ID, m.ConversationID, m.Role, string(normalizeJSON(m.Content)), nullString(m.ToolCallID), nullString(m.InvocationID), m.CreatedAtMs)
	return err
}

func (c *Catalog) AgentMessages(ctx context.Context, conversationID string, limit int) ([]AgentMessage, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	rows, err := c.db.QueryContext(ctx, `SELECT rowid, id, conversation_id, role, content_json, tool_call_id, invocation_id, COALESCE(compacted_into,''), created_at_ms
		FROM agent_messages WHERE conversation_id=? ORDER BY rowid DESC LIMIT ?`, conversationID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AgentMessage{}
	for rows.Next() {
		var m AgentMessage
		var content string
		var toolCallID, invocationID sql.NullString
		if err := rows.Scan(&m.Seq, &m.ID, &m.ConversationID, &m.Role, &content, &toolCallID, &invocationID, &m.CompactedInto, &m.CreatedAtMs); err != nil {
			return nil, err
		}
		m.ToolCallID = toolCallID.String
		m.InvocationID = invocationID.String
		m.Content = json.RawMessage(normalizeJSON([]byte(content)))
		out = append(out, m)
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, rows.Err()
}

func (c *Catalog) MarkMessagesCompacted(ctx context.Context, conversationID, summaryID string, throughRowid int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := c.db.ExecContext(ctx, `UPDATE agent_messages SET compacted_into=? WHERE conversation_id=? AND rowid<=? AND (compacted_into IS NULL OR compacted_into='')`, summaryID, conversationID, throughRowid)
	return err
}

func (c *Catalog) SetInvocationMetadata(ctx context.Context, id, conversationID, endpointID, model, promptRevisionID, promptSHA256 string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := c.db.ExecContext(ctx, `UPDATE invocations SET conversation_id=?, endpoint_id=?, model=?, prompt_revision_id=?, prompt_sha256=? WHERE id=?`,
		conversationID, endpointID, model, nullString(promptRevisionID), nullString(promptSHA256), id)
	return err
}

func (c *Catalog) UpdateInvocationAudit(ctx context.Context, id string, input json.RawMessage, latencyMs int64, compactionCount int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, err := c.db.ExecContext(ctx, `UPDATE invocations SET input_json=?, latency_ms=?, compaction_count=? WHERE id=?`, string(normalizeJSON(input)), latencyMs, compactionCount, id)
	return err
}

func (c *Catalog) ReconcileRunningInvocations(ctx context.Context) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	rows, err := c.db.QueryContext(ctx, `SELECT id FROM invocations WHERE status='running'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, id := range ids {
		_, _ = c.db.ExecContext(ctx, `UPDATE invocations SET status='interrupted', ended_at_ms=? WHERE id=?`, time.Now().UnixMilli(), id)
	}
	return ids, nil
}

func normalizeJSON(raw []byte) []byte {
	if len(raw) == 0 {
		return []byte("{}")
	}
	var v any
	if json.Unmarshal(raw, &v) != nil {
		b, _ := json.Marshal(string(raw))
		return b
	}
	b, _ := json.Marshal(v)
	return b
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
