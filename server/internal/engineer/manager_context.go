package engineer

import (
	"context"
	"errors"
	"fmt"
	"time"

	"f1tele/server/internal/contract"
)

func (m *Manager) ContextUsage(ctx context.Context, sessionID, branchID string) (map[string]any, error) {
	convID := conversationID("live_question", sessionID, branchID)
	messages, total, err := m.conversation(ctx, convID, 0)
	if err != nil {
		return nil, err
	}
	cfg := m.Store.Config()
	endpoint, _ := endpointByID(cfg, deref(cfg.Agent.EndpointID))
	limit := effectiveLimit(cfg, endpoint)
	latest, _ := m.Catalog.LatestCompaction(ctx, convID)
	out := map[string]any{"conversation_id": convID, "messages": len(messages), "token_estimate": total, "context_limit": limit}
	if latest != nil {
		out["last_compaction"] = latest
	}
	return out, nil
}

func (m *Manager) Compact(ctx context.Context, sessionID, branchID, focus string) (map[string]any, error) {
	cfg, rev := m.Store.Snapshot()
	m.syncConfig(cfg, rev, time.Now())
	if !cfg.Agent.Enabled || cfg.Agent.EndpointID == nil {
		return nil, errors.New("no enabled agent endpoint")
	}
	endpoint, ok := endpointByID(cfg, deref(cfg.Agent.EndpointID))
	if !ok {
		return nil, errors.New("configured agent endpoint is missing")
	}
	convID := conversationID("live_question", sessionID, branchID)
	parent := contract.Invocation{ID: fmt.Sprintf("manual-cmp-%d", time.Now().UnixNano()), SessionID: sessionID, BranchID: branchID, Profile: "compaction"}
	cmp, err := m.compact(ctx, parent, cfg, endpoint, m.Store.Secret(deref(endpoint.APIKeyEnv)), convID, "manual", focus, 0, 0)
	if err != nil {
		return nil, err
	}
	return map[string]any{"compaction": cmp, "conversation_id": convID}, nil
}
