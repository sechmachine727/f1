package engineer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"f1tele/server/internal/contract"
	"f1tele/server/internal/history"
)

const summaryPrompt = "You compact a Formula 1 race-engineer conversation. Return one JSON object with keys: objective, current_plan, decision_timeline, accepted_advice, evidence_ids, assumptions, predictions, outcomes, event_transitions, coverage, focus, open_questions. Preserve the exact current plan, accepted advice, evidence identifiers and safety/weather/flag changes. Omit raw telemetry samples and repeated tool payloads. State truncated coverage explicitly. Do not invent measurements or advice."

func conversationID(profile, sessionID, branchID string) string {
	scope := "live"
	switch profile {
	case "post_race":
		scope = "post"
	case "setup_research":
		scope = "setup"
	}
	if scope == "live" {
		return "conv:" + sessionID + ":" + branchID + ":live"
	}
	return "conv:" + sessionID + ":" + scope
}

func effectiveLimit(cfg contract.Config, endpoint contract.Endpoint) int {
	limit := cfg.Agent.ContextTokens
	if endpoint.ContextLimitTokens > 0 && (limit <= 0 || endpoint.ContextLimitTokens < limit) {
		limit = endpoint.ContextLimitTokens
	}
	if limit <= 0 {
		limit = 16384
	}
	return limit
}

func messageText(m history.AgentMessage) string {
	var s string
	if json.Unmarshal(m.Content, &s) == nil {
		return s
	}
	return string(m.Content)
}

func summaryText(raw json.RawMessage) string {
	var obj map[string]any
	if json.Unmarshal(raw, &obj) == nil {
		if s, ok := obj["text"].(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	return string(raw)
}

func jsonString(s string) json.RawMessage {
	raw, _ := json.Marshal(s)
	return raw
}

func rawJSON(v any) json.RawMessage {
	raw, _ := json.Marshal(v)
	return raw
}

func isContextOverflow(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "context") && (strings.Contains(s, "length") || strings.Contains(s, "too long") || strings.Contains(s, "maximum") || strings.Contains(s, "token limit"))
}

func conversationTokens(messages []history.AgentMessage) int {
	total := 0
	for _, m := range messages {
		total += estimateTokens(m.Content) + 8
	}
	return total
}

func (m *Manager) conversation(ctx context.Context, convID string, tailTokens int) ([]history.AgentMessage, int, error) {
	messages, err := m.Catalog.AgentMessages(ctx, convID, 1000)
	if err != nil {
		return nil, 0, err
	}
	filtered := messages[:0]
	for _, msg := range messages {
		if msg.CompactedInto == "" {
			filtered = append(filtered, msg)
		}
	}
	messages = filtered
	total := conversationTokens(messages)
	if tailTokens <= 0 {
		return messages, total, nil
	}
	kept := make([]history.AgentMessage, 0, len(messages))
	used := 0
	for i := len(messages) - 1; i >= 0; i-- {
		t := estimateTokens(messages[i].Content) + 8
		if used+t > tailTokens && len(kept) > 0 {
			break
		}
		kept = append(kept, messages[i])
		used += t
	}
	for i, j := 0, len(kept)-1; i < j; i, j = i+1, j-1 {
		kept[i], kept[j] = kept[j], kept[i]
	}
	return kept, total, nil
}

func (m *Manager) compact(ctx context.Context, parent contract.Invocation, cfg contract.Config, endpoint contract.Endpoint, key, convID, trigger, focus string, beforeTokens int) (*history.AgentCompaction, error) {
	messages, total, err := m.conversation(ctx, convID, 0)
	if err != nil {
		return nil, err
	}
	if len(messages) == 0 {
		return nil, fmt.Errorf("no conversation messages to compact")
	}
	if beforeTokens <= 0 {
		beforeTokens = total
	}
	limit := effectiveLimit(cfg, endpoint)
	tailTokens := limit * cfg.Agent.Compaction.RecentTailPercent / 100
	if tailTokens < 400 {
		tailTokens = 400
	}
	maxOut := cfg.Agent.MaxOutputTokens
	if maxOut <= 0 || maxOut > 4096 {
		maxOut = 4096
	}
	if _, ok, reason := m.budget.Reserve(int64(cfg.Agent.ContextTokens), maxOut); !ok {
		return nil, fmt.Errorf("compaction budget blocked: %s", reason)
	}
	usage := contract.Usage{UsageEstimated: true}
	defer func() { m.budget.Reconcile(int64(cfg.Agent.ContextTokens)+int64(maxOut), usage) }()
	cmpInv := contract.Invocation{
		ID: fmt.Sprintf("cmp-%d", time.Now().UnixNano()), Profile: "compaction", SessionID: parent.SessionID,
		BranchID: parent.BranchID, Status: "running", StartedAtMs: int64Ptr(time.Now().UnixMilli()),
		TriggerIDs: []string{trigger}, Usage: contract.Usage{UsageEstimated: true}, NotificationDisposition: "none", PromptVersion: PromptVersion,
	}
	if err := m.store(cmpInv); err != nil {
		return nil, err
	}
	_ = m.Catalog.SetInvocationMetadata(ctx, cmpInv.ID, convID, endpoint.ID, endpoint.Model, "", "")
	m.Activity.Emit(ctx, history.AgentEvent{
		InvocationID: parent.ID, SessionID: parent.SessionID, BranchID: parent.BranchID, Profile: parent.Profile,
		Kind: "compaction_started", AtMs: time.Now().UnixMilli(),
	}, map[string]any{"trigger": trigger, "focus": focus, "before_tokens": beforeTokens, "recent_tail_tokens": tailTokens, "compaction_invocation_id": cmpInv.ID})

	transcript := summaryTranscript(messages, 60_000)
	summary, usage, model, err := callCompaction(ctx, endpoint, key, transcript, focus, maxOut)
	if err != nil {
		m.finish(cmpInv.ID, "failed", err.Error(), nil, usage)
		m.Activity.Emit(ctx, history.AgentEvent{
			InvocationID: parent.ID, SessionID: parent.SessionID, BranchID: parent.BranchID, Profile: parent.Profile,
			Kind: "error", AtMs: time.Now().UnixMilli(),
		}, map[string]any{"stage": "compaction", "message": err.Error()})
		return nil, err
	}
	summaryJSON := json.RawMessage(summary)
	if !json.Valid(summaryJSON) {
		summaryJSON, _ = json.Marshal(map[string]any{"text": summary})
	}
	cmp := history.AgentCompaction{
		ID: fmt.Sprintf("cmp-%d", time.Now().UnixNano()), ConversationID: convID, InvocationID: parent.ID,
		Trigger: trigger, Focus: focus, BeforeTokens: int64(beforeTokens), AfterTokens: int64(estimateTokens(summaryJSON)),
		Summary: summaryJSON, Model: model, Usage: usage, CreatedAtMs: time.Now().UnixMilli(),
	}
	if err := m.Catalog.SaveCompaction(ctx, cmp); err != nil {
		m.finish(cmpInv.ID, "failed", err.Error(), nil, usage)
		return nil, err
	}
	cutoff := int64(0)
	kept, _, _ := m.conversation(ctx, convID, tailTokens)
	if len(kept) > 0 {
		cutoff = kept[0].Seq
	}
	_ = m.Catalog.MarkMessagesCompacted(ctx, convID, cmp.ID, cutoff)
	m.finish(cmpInv.ID, "accepted", "", nil, usage)
	_, _ = m.Catalog.DB().ExecContext(ctx, `UPDATE invocations SET continuation_reset_reason=? WHERE id=?`, "compaction:"+cmp.ID, parent.ID)
	m.Activity.Emit(ctx, history.AgentEvent{
		InvocationID: parent.ID, SessionID: parent.SessionID, BranchID: parent.BranchID, Profile: parent.Profile,
		Kind: "compaction_completed", AtMs: time.Now().UnixMilli(),
	}, map[string]any{"compaction_id": cmp.ID, "trigger": trigger, "before_tokens": cmp.BeforeTokens, "after_tokens": cmp.AfterTokens, "model": model, "summary": summaryJSON, "compaction_invocation_id": cmpInv.ID, "continuation_reset": true})
	return &cmp, nil
}

func summaryTranscript(messages []history.AgentMessage, maxChars int) string {
	var b strings.Builder
	for i := len(messages) - 1; i >= 0; i-- {
		m := messages[i]
		text := messageText(m)
		if m.Role == "tool" && len(text) > 2000 {
			text = text[:2000] + "...[tool payload truncated]"
		}
		line := m.Role + ": " + text + "\n"
		if b.Len()+len(line) > maxChars {
			break
		}
		b.WriteString(line)
	}
	parts := strings.Split(b.String(), "\n")
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return strings.Join(parts, "\n")
}

func callCompaction(ctx context.Context, ep contract.Endpoint, key, transcript, focus string, maxOut int) (string, contract.Usage, string, error) {
	var body any
	url := strings.TrimRight(ep.BaseURL, "/")
	protocol := ep.Protocol
	if strings.HasSuffix(url, "/chat/completions") || strings.HasSuffix(url, "/responses") {
		protocol = "custom"
	}
	system := summaryPrompt
	if strings.TrimSpace(focus) != "" {
		system += " Focus for this compaction: " + strings.TrimSpace(focus)
	}
	switch {
	case protocol == "chat_completions":
		url += "/chat/completions"
		body = map[string]any{"model": ep.Model, "messages": []map[string]any{{"role": "system", "content": system}, {"role": "user", "content": transcript}}, "max_tokens": maxOut, "temperature": 0.2}
	case protocol == "responses":
		url += "/responses"
		body = map[string]any{"model": ep.Model, "input": system + "\n\n" + transcript, "max_output_tokens": maxOut}
	case protocol == "custom":
		if strings.HasSuffix(url, "/chat/completions") {
			body = map[string]any{"model": ep.Model, "messages": []map[string]any{{"role": "system", "content": system}, {"role": "user", "content": transcript}}, "max_tokens": maxOut, "temperature": 0.2}
		} else {
			body = map[string]any{"model": ep.Model, "input": system + "\n\n" + transcript, "max_output_tokens": maxOut}
		}
	default:
		return "", contract.Usage{UsageEstimated: true}, ep.Model, fmt.Errorf("unsupported compaction protocol %q", ep.Protocol)
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return "", contract.Usage{UsageEstimated: true}, ep.Model, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := (&http.Client{Timeout: 90 * time.Second}).Do(req)
	if err != nil {
		return "", contract.Usage{UsageEstimated: true}, ep.Model, err
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", contract.Usage{UsageEstimated: true}, ep.Model, fmt.Errorf("compaction provider %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	var doc map[string]any
	if err := json.Unmarshal(payload, &doc); err != nil {
		return "", contract.Usage{UsageEstimated: true}, ep.Model, err
	}
	content := ""
	if choices, ok := doc["choices"].([]any); ok && len(choices) > 0 {
		if c, ok := choices[0].(map[string]any); ok {
			if msg, ok := c["message"].(map[string]any); ok {
				content, _ = msg["content"].(string)
			}
		}
	}
	if content == "" {
		content, _ = doc["output_text"].(string)
	}
	if content == "" {
		if output, ok := doc["output"].([]any); ok {
			for _, item := range output {
				obj, _ := item.(map[string]any)
				parts, _ := obj["content"].([]any)
				for _, part := range parts {
					p, _ := part.(map[string]any)
					if t, ok := p["text"].(string); ok {
						content += t
					}
				}
			}
		}
	}
	if strings.TrimSpace(content) == "" {
		return "", contract.Usage{UsageEstimated: true}, ep.Model, fmt.Errorf("compaction provider returned no text")
	}
	usage := contract.Usage{ProviderRequests: 1, UsageEstimated: true}
	if u, ok := doc["usage"].(map[string]any); ok {
		usage = usageFromMap(u)
	}
	return content, usage, ep.Model, nil
}

func usageFromMap(u map[string]any) contract.Usage {
	out := contract.Usage{ProviderRequests: 1, UsageEstimated: true}
	if v, ok := u["prompt_tokens"].(float64); ok {
		n := int64(v)
		out.InputTokens = &n
	}
	if v, ok := u["input_tokens"].(float64); ok {
		n := int64(v)
		out.InputTokens = &n
	}
	if v, ok := u["completion_tokens"].(float64); ok {
		n := int64(v)
		out.OutputTokens = &n
	}
	if v, ok := u["output_tokens"].(float64); ok {
		n := int64(v)
		out.OutputTokens = &n
	}
	if v, ok := u["total_tokens"].(float64); ok {
		n := int64(v)
		out.TotalTokens = &n
	}
	return out
}
