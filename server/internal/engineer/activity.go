package engineer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"f1tele/server/internal/history"
)

const maxActivityPayload = 1 << 20

type Activity struct {
	Catalog   *history.Catalog
	Broadcast func(history.AgentEvent)
	Now       func() time.Time
}

func (a *Activity) Emit(ctx context.Context, e history.AgentEvent, payload any) history.AgentEvent {
	if a == nil || a.Catalog == nil {
		return e
	}
	if e.AtMs == 0 {
		now := time.Now
		if a.Now != nil {
			now = a.Now
		}
		e.AtMs = now().UnixMilli()
	}
	e.Payload, e.Truncated = capPayload(redact(payload))
	seq, err := a.Catalog.AppendAgentEvent(ctx, e)
	if err == nil {
		e.Seq = seq
		if a.Broadcast != nil {
			a.Broadcast(e)
		}
	}
	return e
}

func (a *Activity) Reconcile(ctx context.Context) {
	if a == nil || a.Catalog == nil {
		return
	}
	ids, err := a.Catalog.ReconcileRunningInvocations(ctx)
	if err != nil {
		return
	}
	for _, id := range ids {
		a.Emit(ctx, history.AgentEvent{InvocationID: id, Kind: "interrupted"}, map[string]any{"reason": "server restarted before completion"})
	}
}

func capPayload(payload any) (json.RawMessage, bool) {
	if payload == nil {
		return json.RawMessage("{}"), false
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		raw = []byte("{}")
	}
	if len(raw) <= maxActivityPayload {
		return json.RawMessage(raw), false
	}
	sum := sha256.Sum256(raw)
	preview := raw[:4096]
	out, _ := json.Marshal(map[string]any{
		"truncated": true,
		"bytes":     len(raw),
		"sha256":    hex.EncodeToString(sum[:]),
		"preview":   string(preview),
	})
	return json.RawMessage(out), true
}

func redact(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, val := range t {
			lk := strings.ToLower(k)
			if strings.Contains(lk, "api_key") || lk == "authorization" || strings.Contains(lk, "provider_state") || strings.Contains(lk, "opaque") {
				out[k] = "[redacted]"
				continue
			}
			out[k] = redact(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i := range t {
			out[i] = redact(t[i])
		}
		return out
	default:
		return v
	}
}
