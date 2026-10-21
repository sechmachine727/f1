package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"f1tele/server/internal/contract"
)

func (c *Catalog) LastInputTokens(ctx context.Context, conversationID string) int {
	var usage string
	if err := c.db.QueryRowContext(ctx, `SELECT usage_json FROM invocations WHERE conversation_id=? AND usage_json<>'' ORDER BY COALESCE(started_at_ms,0) DESC LIMIT 1`, conversationID).Scan(&usage); err != nil {
		return 0
	}
	var u contract.Usage
	if json.Unmarshal([]byte(usage), &u) != nil || u.InputTokens == nil {
		return 0
	}
	return int(*u.InputTokens)
}

type AgentLogRow struct {
	ID                      string         `json:"id"`
	SessionID               string         `json:"session_id"`
	BranchID                string         `json:"branch_id"`
	Profile                 string         `json:"profile"`
	Status                  string         `json:"status"`
	TriggerIDs              []string       `json:"trigger_ids"`
	Model                   string         `json:"model"`
	EndpointID              string         `json:"endpoint_id"`
	ConversationID          string         `json:"conversation_id"`
	PromptRevisionID        string         `json:"prompt_revision_id,omitempty"`
	PromptSHA256            string         `json:"prompt_sha256,omitempty"`
	StartedAtMs             *int64         `json:"started_at_ms"`
	EndedAtMs               *int64         `json:"ended_at_ms"`
	LatencyMs               *int64         `json:"latency_ms"`
	CompactionCount         int            `json:"compaction_count"`
	NotificationDisposition string         `json:"notification_disposition"`
	Usage                   contract.Usage `json:"usage"`
	Decision                string         `json:"decision,omitempty"`
	DriverMessage           string         `json:"driver_message,omitempty"`
	ErrorCode               string         `json:"error_code,omitempty"`
	ErrorMessage            string         `json:"error_message,omitempty"`
	EventCount              int            `json:"event_count"`
	LastKind                string         `json:"last_kind,omitempty"`
	LastAtMs                *int64         `json:"last_at_ms"`
}

type AgentLogFilter struct {
	SessionID    string
	BranchID     string
	Profile      string
	Status       string
	Trigger      string
	Model        string
	InvocationID string
	ErrorsOnly   bool
	Compactions  bool
	FromMs       int64
	ToMs         int64
	Limit        int
	Cursor       string
}

func (c *Catalog) ListAgentLogs(ctx context.Context, f AgentLogFilter) ([]AgentLogRow, string, error) {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 25
	}
	q := `SELECT i.id, i.session_id, i.branch_id, i.profile, i.status, i.trigger_ids_json,
		COALESCE(i.model,''), COALESCE(i.endpoint_id,''), COALESCE(i.conversation_id,''),
		COALESCE(i.prompt_revision_id,''), COALESCE(i.prompt_sha256,''),
		i.started_at_ms, i.ended_at_ms, i.latency_ms, COALESCE(i.compaction_count,0),
		i.notification_disposition, i.usage_json, COALESCE(i.output_json,''), COALESCE(i.error_json,''),
		(SELECT COUNT(*) FROM agent_events e WHERE e.invocation_id=i.id),
		COALESCE((SELECT e.kind FROM agent_events e WHERE e.invocation_id=i.id ORDER BY e.seq DESC LIMIT 1),''),
		(SELECT e.at_ms FROM agent_events e WHERE e.invocation_id=i.id ORDER BY e.seq DESC LIMIT 1)
		FROM invocations i WHERE 1=1`
	args := []any{}
	if f.SessionID != "" {
		q += " AND i.session_id=?"
		args = append(args, f.SessionID)
	}
	if f.BranchID != "" {
		q += " AND i.branch_id=?"
		args = append(args, f.BranchID)
	}
	if f.Profile != "" {
		q += " AND i.profile=?"
		args = append(args, f.Profile)
	}
	if f.Status != "" {
		q += " AND i.status=?"
		args = append(args, f.Status)
	}
	if f.Trigger != "" {
		q += " AND i.trigger_ids_json LIKE ?"
		args = append(args, "%"+f.Trigger+"%")
	}
	if f.Model != "" {
		q += " AND i.model=?"
		args = append(args, f.Model)
	}
	if f.InvocationID != "" {
		q += " AND i.id=?"
		args = append(args, f.InvocationID)
	}
	if f.ErrorsOnly {
		q += " AND COALESCE(i.error_json,'')<>''"
	}
	if f.Compactions {
		q += " AND COALESCE(i.compaction_count,0)>0"
	}
	if f.FromMs > 0 {
		q += " AND COALESCE(i.started_at_ms,0)>=?"
		args = append(args, f.FromMs)
	}
	if f.ToMs > 0 {
		q += " AND COALESCE(i.started_at_ms,0)<=?"
		args = append(args, f.ToMs)
	}
	if f.Cursor != "" {
		parts := strings.SplitN(f.Cursor, "|", 2)
		if len(parts) == 2 {
			var ms int64
			_, _ = fmt.Sscanf(parts[0], "%d", &ms)
			q += " AND (COALESCE(i.started_at_ms,0)< ? OR (COALESCE(i.started_at_ms,0)=? AND i.id<?))"
			args = append(args, ms, ms, parts[1])
		}
	}
	q += " ORDER BY COALESCE(i.started_at_ms,0) DESC, i.id DESC LIMIT ?"
	args = append(args, f.Limit+1)
	rows, err := c.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	out := []AgentLogRow{}
	for rows.Next() {
		var r AgentLogRow
		var triggers, usage, output, errJSON string
		var start, end, latency, lastAt sql.NullInt64
		if err := rows.Scan(&r.ID, &r.SessionID, &r.BranchID, &r.Profile, &r.Status, &triggers, &r.Model, &r.EndpointID, &r.ConversationID,
			&r.PromptRevisionID, &r.PromptSHA256, &start, &end, &latency, &r.CompactionCount, &r.NotificationDisposition, &usage, &output, &errJSON,
			&r.EventCount, &r.LastKind, &lastAt); err != nil {
			return nil, "", err
		}
		if start.Valid {
			r.StartedAtMs = &start.Int64
		}
		if end.Valid {
			r.EndedAtMs = &end.Int64
		}
		if latency.Valid {
			r.LatencyMs = &latency.Int64
		}
		if lastAt.Valid {
			r.LastAtMs = &lastAt.Int64
		}
		_ = json.Unmarshal([]byte(triggers), &r.TriggerIDs)
		_ = json.Unmarshal([]byte(usage), &r.Usage)
		var outObj struct {
			Decision      string  `json:"decision"`
			DriverMessage *string `json:"driver_message"`
		}
		if output != "" {
			_ = json.Unmarshal([]byte(output), &outObj)
			r.Decision = outObj.Decision
			if outObj.DriverMessage != nil {
				r.DriverMessage = *outObj.DriverMessage
			}
		}
		var errObj struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		if errJSON != "" {
			_ = json.Unmarshal([]byte(errJSON), &errObj)
			r.ErrorCode, r.ErrorMessage = errObj.Code, errObj.Message
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(out) > f.Limit {
		last := out[f.Limit-1]
		ms := int64(0)
		if last.StartedAtMs != nil {
			ms = *last.StartedAtMs
		}
		next = fmt.Sprintf("%d|%s", ms, last.ID)
		out = out[:f.Limit]
	}
	return out, next, nil
}
