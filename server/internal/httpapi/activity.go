package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"f1tele/server/internal/history"
)

type AgentActivityHub struct {
	mu   sync.Mutex
	subs map[chan history.AgentEvent]struct{}
}

func NewAgentActivityHub() *AgentActivityHub {
	return &AgentActivityHub{subs: map[chan history.AgentEvent]struct{}{}}
}

func (h *AgentActivityHub) Publish(e history.AgentEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- e:
		default:
		}
	}
}

func (h *AgentActivityHub) Subscribe() (<-chan history.AgentEvent, func()) {
	ch := make(chan history.AgentEvent, 64)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.subs, ch)
		h.mu.Unlock()
	}
}

func (a *API) engineerActivityStream(w http.ResponseWriter, r *http.Request) {
	if a.Catalog == nil {
		writeErr(w, r, http.StatusServiceUnavailable, "unavailable", "activity log is unavailable")
		return
	}
	sessionID, branchID := r.URL.Query().Get("session"), r.URL.Query().Get("branch")
	after, _ := strconv.ParseInt(r.Header.Get("Last-Event-ID"), 10, 64)
	if v := r.URL.Query().Get("after_seq"); v != "" {
		after, _ = strconv.ParseInt(v, 10, 64)
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, r, http.StatusInternalServerError, "stream_unsupported", "streaming is unavailable")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	_, _ = fmt.Fprint(w, "retry: 3000\n\n")
	flusher.Flush()

	if after == 0 {
		if v := r.URL.Query().Get("last"); v != "" {
			after, _ = strconv.ParseInt(v, 10, 64)
		}
	}
	if after > 0 {
		events, err := a.Catalog.AgentEventsAfter(r.Context(), after, sessionID, branchID, 200)
		if err == nil {
			for _, e := range events {
				writeAgentEvent(w, e)
			}
			flusher.Flush()
		}
	}
	ch, cancel := a.AgentActivity.Subscribe()
	defer cancel()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			_, _ = fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()
		case e := <-ch:
			if sessionID != "" && e.SessionID != "" && e.SessionID != sessionID {
				continue
			}
			if branchID != "" && e.BranchID != "" && e.BranchID != branchID {
				continue
			}
			writeAgentEvent(w, e)
			flusher.Flush()
		}
	}
}

func writeAgentEvent(w http.ResponseWriter, e history.AgentEvent) {
	raw, _ := json.Marshal(e)
	_, _ = fmt.Fprintf(w, "id: %d\ndata: %s\n\n", e.Seq, raw)
}

func (a *API) engineerLogs(w http.ResponseWriter, r *http.Request) {
	if a.Catalog == nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []history.AgentLogRow{}, "next_cursor": nil})
		return
	}
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	f := history.AgentLogFilter{
		SessionID: q.Get("session"), BranchID: q.Get("branch"), Profile: q.Get("profile"),
		Status: q.Get("status"), Trigger: q.Get("trigger"), Model: q.Get("model"),
		InvocationID: q.Get("id"), ErrorsOnly: q.Get("errors") == "true", Compactions: q.Get("compactions") == "true",
		Limit: limit, Cursor: q.Get("cursor"),
	}
	f.FromMs, _ = strconv.ParseInt(q.Get("from"), 10, 64)
	f.ToMs, _ = strconv.ParseInt(q.Get("to"), 10, 64)
	items, next, err := a.Catalog.ListAgentLogs(r.Context(), f)
	if err != nil {
		writeErr(w, r, http.StatusInternalServerError, "log_error", err.Error())
		return
	}
	if items == nil {
		items = []history.AgentLogRow{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "next_cursor": nilIfEmpty(next)})
}

func (a *API) engineerLogDetail(w http.ResponseWriter, r *http.Request) {
	if a.Catalog == nil {
		writeErr(w, r, http.StatusServiceUnavailable, "unavailable", "activity log is unavailable")
		return
	}
	id := r.PathValue("id")
	rows, _, err := a.Catalog.ListAgentLogs(r.Context(), history.AgentLogFilter{InvocationID: id, Limit: 1})
	if err != nil {
		writeErr(w, r, http.StatusInternalServerError, "log_error", err.Error())
		return
	}
	if len(rows) == 0 {
		writeErr(w, r, http.StatusNotFound, "not_found", "invocation not found")
		return
	}
	events, _ := a.Catalog.ListAgentEvents(r.Context(), id)
	compactions, _ := a.Catalog.ListCompactions(r.Context(), rows[0].ConversationID)
	messages, _ := a.Catalog.AgentMessages(r.Context(), rows[0].ConversationID, 500)
	writeJSON(w, http.StatusOK, map[string]any{
		"invocation": rows[0], "events": events, "compactions": compactions, "messages": messages,
	})
}
