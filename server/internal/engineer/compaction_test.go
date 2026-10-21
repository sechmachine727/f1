package engineer

import (
	"context"
	"encoding/json"
	"fmt"
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
)

func TestCompactionCallsProviderAndPreservesAudit(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"{\"current_plan\":\"hold\",\"evidence_ids\":[\"car.speed_mps@0\"]}"}}],"usage":{"prompt_tokens":120,"completion_tokens":30,"total_tokens":150}}`)
	}))
	defer provider.Close()
	dir := t.TempDir()
	t.Setenv("F1TELE_DATA_DIR", dir)
	store, err := config.Load(config.Overrides{DataDir: dir, ConfigPath: filepath.Join(dir, "config.json")})
	if err != nil {
		t.Fatal(err)
	}
	cat, err := history.OpenCatalog(filepath.Join(dir, "c.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer cat.Close()
	red := state.New("ctx", time.Now)
	m := NewManager(store, red, cat, &Invoker{}, nil)
	m.Activity = &Activity{Catalog: cat}
	conv := "conv:test:branch:live"
	for i, msg := range []history.AgentMessage{
		{Role: "user", Content: json.RawMessage(`"what is the plan"`), CreatedAtMs: 1},
		{Role: "assistant", Content: json.RawMessage(`"hold current stint"`), CreatedAtMs: 2},
		{Role: "user", Content: json.RawMessage(`"rain is coming"`), CreatedAtMs: 3},
	} {
		msg.ConversationID = conv
		msg.ID = fmt.Sprintf("m-%d", i)
		msg.InvocationID = "parent"
		if err := cat.AppendAgentMessage(context.Background(), msg); err != nil {
			t.Fatal(err)
		}
	}
	cfg := store.Config()
	endpoint := contract.Endpoint{ID: "test", Protocol: "chat_completions", BaseURL: provider.URL, Model: "fixture"}
	parent := contract.Invocation{ID: "parent", SessionID: "test", BranchID: "branch", Profile: "live_question"}
	cmp, err := m.compact(context.Background(), parent, cfg, endpoint, "key", conv, "manual", "preserve wet plan", 1000, 0)
	if err != nil {
		t.Fatal(err)
	}
	if cmp == nil || len(cmp.Summary) == 0 || cmp.Usage.TotalTokens == nil || *cmp.Usage.TotalTokens != 150 {
		t.Fatalf("compaction %#v", cmp)
	}
	latest, err := cat.LatestCompaction(context.Background(), conv)
	if err != nil || latest == nil || latest.ID != cmp.ID {
		t.Fatalf("latest %#v %v", latest, err)
	}
	events, err := cat.ListAgentEvents(context.Background(), "parent")
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, e := range events {
		kinds = append(kinds, e.Kind)
	}
	if len(kinds) < 2 || kinds[0] != "compaction_started" || kinds[len(kinds)-1] != "compaction_completed" {
		t.Fatalf("events %#v", kinds)
	}
	rows, _, err := cat.ListAgentLogs(context.Background(), history.AgentLogFilter{Profile: "compaction", Limit: 5})
	if err != nil || len(rows) != 1 {
		t.Fatalf("compaction logs %#v %v", rows, err)
	}
}

// TestCompactionKeepsAssistantToolCallInTail pins the exclusive cutoff: the
// first message of the retained tail must survive even when it is a tool call.
func TestCompactionKeepsAssistantToolCallInTail(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":"{\"current_plan\":\"hold\"}"}}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`)
	}))
	defer provider.Close()
	dir := t.TempDir()
	t.Setenv("F1TELE_DATA_DIR", dir)
	store, err := config.Load(config.Overrides{DataDir: dir, ConfigPath: filepath.Join(dir, "config.json")})
	if err != nil {
		t.Fatal(err)
	}
	cat, err := history.OpenCatalog(filepath.Join(dir, "c.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer cat.Close()
	red := state.New("ctx", time.Now)
	m := NewManager(store, red, cat, &Invoker{}, nil)
	m.Activity = &Activity{Catalog: cat}
	conv := "conv:tail:branch:live"
	messages := []history.AgentMessage{
		{Role: "user", Content: json.RawMessage(`"` + strings.Repeat("a", 2500) + `"`)},
		{Role: "assistant", Content: json.RawMessage(`"` + strings.Repeat("b", 2500) + `"`)},
		{Role: "assistant", Content: json.RawMessage(`{"tool_calls":[{"id":"call-keep","type":"function","function":{"name":"read_telemetry_window","arguments":"{}"}}]}`)},
		{Role: "tool", Content: json.RawMessage(`"tool result"`), ToolCallID: "call-keep"},
		{Role: "assistant", Content: json.RawMessage(`"final"`)},
	}
	for i := range messages {
		messages[i].ConversationID = conv
		messages[i].ID = fmt.Sprintf("m-%d", i)
		messages[i].CreatedAtMs = int64(i)
		if err := cat.AppendAgentMessage(context.Background(), messages[i]); err != nil {
			t.Fatal(err)
		}
	}
	cfg := store.Config()
	endpoint := contract.Endpoint{ID: "test", Protocol: "chat_completions", BaseURL: provider.URL, Model: "fixture", ContextLimitTokens: 1600}
	parent := contract.Invocation{ID: "parent", SessionID: "tail", BranchID: "branch", Profile: "live_question"}
	if _, err := m.compact(context.Background(), parent, cfg, endpoint, "key", conv, "manual", "keep tool turn", 0, 0); err != nil {
		t.Fatal(err)
	}
	got, err := cat.AgentMessages(context.Background(), conv, 10)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].CompactedInto == "" || got[1].CompactedInto == "" {
		t.Fatalf("messages before the tail not compacted: %#v", got[:2])
	}
	if got[2].CompactedInto != "" || !strings.Contains(string(got[2].Content), "call-keep") {
		t.Fatalf("first retained tool call was compacted: %#v", got[2])
	}
	for _, i := range []int{3, 4} {
		if got[i].CompactedInto != "" {
			t.Fatalf("tail message %d was compacted: %#v", i, got[i])
		}
	}
}
