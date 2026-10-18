package engineer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"f1tele/server/internal/contract"
	"f1tele/server/internal/state"
)

// TestResponsesProtocol exercises the Responses adapter against a fake endpoint:
// a function call must come back as a correlated tool result and then a terminal
// JSON object. It is a protocol check, not provider qualification.
func TestResponsesProtocol(t *testing.T) {
	turn := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" && r.URL.Path != "/responses" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		turn++
		w.Header().Set("Content-Type", "application/json")
		if turn == 1 {
			body := map[string]any{
				"id": "resp_1", "object": "response", "created_at": 1, "model": "m", "status": "completed",
				"output": []any{map[string]any{
					"type": "function_call", "id": "fc_1", "call_id": "call_1", "name": "read_telemetry_window",
					"arguments": `{"from_seq":"0","to_seq":"10","car_indices":[0],"channels":["car.speed_mps"],"max_points":10}`,
				}},
				"output_text": "",
			}
			json.NewEncoder(w).Encode(body)
			return
		}
		// The second request must carry the tool output for the same call id.
		raw, _ := json.Marshal(jsonRequestBody(t, r))
		if !strings.Contains(string(raw), "call_1") {
			t.Errorf("tool call id not preserved: %s", string(raw))
		}
		out := validOutput()
		o, _ := json.Marshal(out)
		json.NewEncoder(w).Encode(map[string]any{
			"id": "resp_2", "object": "response", "created_at": 2, "model": "m", "status": "completed",
			"output": []any{map[string]any{"type": "message", "id": "msg_1", "role": "assistant",
				"content": []any{map[string]any{"type": "output_text", "text": string(o)}}}}, "output_text": string(o),
		})
	}))
	defer srv.Close()

	endpoint := contract.Endpoint{ID: "e", Protocol: "responses", BaseURL: srv.URL + "/v1", Model: "m", ContextLimitTokens: 16384}
	inv := &Invoker{Backend: ToolBackend{}}
	in := inputFixture()
	out, requests, err := inv.Run(context.Background(), "scheduled_strategy", in, endpoint, "sk-test", stateSnapshot())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "\"invocation_id\":\"i-1\"") {
		t.Fatalf("terminal output %q", out)
	}
	if requests < 2 {
		t.Fatalf("provider requests = %d, want a tool turn plus a final turn", requests)
	}
}

func jsonRequestBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Fatalf("request body: %v", err)
	}
	return body
}

func stateSnapshot() state.Snapshot {
	p := 0
	return state.Snapshot{Context: contract.Context{SessionID: "s", BranchID: "b", PlayerCarIndex: &p, SeasonEvidence: []string{}},
		ObservationSeq: 42, Metrics: []contract.Metric{}, Events: []contract.Event{}, Session: map[string]float64{}}
}
