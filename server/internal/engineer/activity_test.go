package engineer

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"f1tele/server/internal/history"
)

func TestActivityRedactsAndCaps(t *testing.T) {
	cat, err := history.OpenCatalog(filepath.Join(t.TempDir(), "c.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer cat.Close()
	a := &Activity{Catalog: cat}
	a.Emit(context.Background(), history.AgentEvent{InvocationID: "i", Kind: "provider_request"}, map[string]any{
		"api_key": "secret", "nested": map[string]any{"authorization": "bearer secret"}, "large": strings.Repeat("x", maxActivityPayload),
	})
	events, err := cat.ListAgentEvents(context.Background(), "i")
	if err != nil || len(events) != 1 {
		t.Fatalf("events %#v %v", events, err)
	}
	if !events[0].Truncated || strings.Contains(string(events[0].Payload), "secret") {
		t.Fatalf("payload was not redacted and truncated: %s", events[0].Payload)
	}
}
