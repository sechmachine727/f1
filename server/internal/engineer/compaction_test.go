package engineer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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
	cmp, err := m.compact(context.Background(), parent, cfg, endpoint, "key", conv, "manual", "preserve wet plan", 1000)
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
