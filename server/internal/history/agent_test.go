package history

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func openAgentCatalog(t *testing.T) *Catalog {
	t.Helper()
	c, err := OpenCatalog(filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func TestAgentEventsAndCompaction(t *testing.T) {
	c := openAgentCatalog(t)
	ctx := context.Background()
	id, err := c.AppendAgentEvent(ctx, AgentEvent{InvocationID: "i-1", SessionID: "s", BranchID: "b", Profile: "live_question", Kind: "queued", AtMs: 1, Payload: json.RawMessage(`{"message":"hello"}`)})
	if err != nil || id == 0 {
		t.Fatalf("append event %d %v", id, err)
	}
	events, err := c.ListAgentEvents(ctx, "i-1")
	if err != nil || len(events) != 1 || string(events[0].Payload) != `{"message":"hello"}` {
		t.Fatalf("events %#v %v", events, err)
	}
	later, err := c.AgentEventsAfter(ctx, id, "s", "b", 10)
	if err != nil || len(later) != 0 {
		t.Fatalf("after events %#v %v", later, err)
	}
	cmp := AgentCompaction{ID: "cmp-1", ConversationID: "conv", InvocationID: "i-1", Trigger: "threshold", BeforeTokens: 100, AfterTokens: 20, Summary: json.RawMessage(`{"current_plan":"hold"}`), CreatedAtMs: 2}
	if err := c.SaveCompaction(ctx, cmp); err != nil {
		t.Fatal(err)
	}
	got, err := c.LatestCompaction(ctx, "conv")
	if err != nil || got == nil || got.ID != "cmp-1" {
		t.Fatalf("compaction %#v %v", got, err)
	}
	if err := c.AppendAgentMessage(ctx, AgentMessage{ConversationID: "conv", Role: "user", Content: json.RawMessage(`"hello"`), CreatedAtMs: 1}); err != nil {
		t.Fatal(err)
	}
	msgs, err := c.AgentMessages(ctx, "conv", 10)
	if err != nil || len(msgs) != 1 || msgs[0].Role != "user" {
		t.Fatalf("messages %#v %v", msgs, err)
	}
}

func TestPromptRevisionsAndLogs(t *testing.T) {
	c := openAgentCatalog(t)
	ctx := context.Background()
	first, err := c.CreatePromptRevision(ctx, "live_question", "first prompt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.CreatePromptRevision(ctx, "live_question", "second prompt"); err != nil {
		t.Fatal(err)
	}
	if err := c.SelectPromptRevision(ctx, "live_question", first.ID); err != nil {
		t.Fatal(err)
	}
	if err := c.DeletePromptRevision(ctx, "live_question", first.ID); err != ErrPromptCurrent {
		t.Fatalf("delete current err %v", err)
	}
	if err := c.ResetPrompt(ctx, "live_question"); err != nil {
		t.Fatal(err)
	}
	cur, err := c.CurrentPrompt(ctx, "live_question")
	if err != nil || cur != nil {
		t.Fatalf("current after reset %#v %v", cur, err)
	}
	revs, err := c.PromptRevisions(ctx, "live_question")
	if err != nil || len(revs) != 2 {
		t.Fatalf("revisions %#v %v", revs, err)
	}
	if _, err := c.DB().Exec(`INSERT INTO invocations(id, session_id, branch_id, status, profile, started_at_ms, usage_json, trigger_ids_json) VALUES(?,?,?,?,?,?,?,?)`,
		"i-log", "s", "b", "accepted", "live_question", time.Now().UnixMilli(), `{"input_tokens":10,"output_tokens":5}`, `["manual"]`); err != nil {
		t.Fatal(err)
	}
	if err := c.SetInvocationMetadata(ctx, "i-log", "conv", "ep", "model", first.ID, first.SHA256); err != nil {
		t.Fatal(err)
	}
	rows, next, err := c.ListAgentLogs(ctx, AgentLogFilter{SessionID: "s", Limit: 10})
	if err != nil || next != "" || len(rows) != 1 || rows[0].Model != "model" || rows[0].PromptRevisionID != first.ID {
		t.Fatalf("logs %#v next %q err %v", rows, next, err)
	}
}
