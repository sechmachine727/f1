package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"f1tele/server/internal/contract"
)

func (a *API) engineerRequest(w http.ResponseWriter, r *http.Request) {
	if a.Engineer == nil {
		writeErr(w, r, 503, "unavailable", "engineer is disabled until an endpoint is configured")
		return
	}
	var req contract.Request
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeErr(w, r, 400, "bad_request", err.Error())
		return
	}
	inv, err := a.Engineer.Request(r.Context(), req)
	if err != nil {
		writeErr(w, r, 422, "engineer_error", err.Error())
		return
	}
	writeJSON(w, 202, inv)
}

func (a *API) engineerInvocations(w http.ResponseWriter, r *http.Request) {
	if a.Engineer == nil {
		writeJSON(w, 200, map[string]any{"items": []contract.Invocation{}, "next_cursor": nil})
		return
	}
	q := r.URL.Query()
	limit := 25
	if v := q.Get("limit"); v != "" {
		json.Unmarshal([]byte(v), &limit)
	}
	items, next, err := a.Engineer.Invocations(r.Context(), q.Get("session"), q.Get("branch"), q.Get("cursor"), limit, q.Get("status"))
	if err != nil {
		writeErr(w, r, 500, "engineer_error", err.Error())
		return
	}
	if items == nil {
		items = []contract.Invocation{}
	}
	writeJSON(w, 200, map[string]any{"items": items, "next_cursor": nilIfEmpty(next)})
}

func (a *API) engineerCancel(w http.ResponseWriter, r *http.Request) {
	if a.Engineer == nil {
		writeErr(w, r, 404, "not_found", "unknown invocation")
		return
	}
	if err := a.Engineer.Cancel(r.Context(), r.PathValue("id")); err != nil {
		writeErr(w, r, 404, "not_found", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (a *API) listSetups(w http.ResponseWriter, r *http.Request) {
	if a.Setup == nil {
		writeJSON(w, 200, map[string]any{"items": []contract.SetupRecord{}, "next_cursor": nil})
		return
	}
	items, err := a.Setup.List(r.Context())
	if err != nil {
		writeErr(w, r, 500, "setup_error", err.Error())
		return
	}
	if items == nil {
		items = []contract.SetupRecord{}
	}
	writeJSON(w, 200, map[string]any{"items": items, "next_cursor": nil})
}

func (a *API) createSetup(w http.ResponseWriter, r *http.Request) {
	if a.Setup == nil {
		writeErr(w, r, 503, "unavailable", "setup research is disabled")
		return
	}
	var in contract.SetupCreate
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil || in.InvocationID == "" {
		writeErr(w, r, 400, "bad_request", "invocation_id is required")
		return
	}
	rec, err := a.Setup.Create(r.Context(), in.InvocationID)
	if err != nil {
		writeErr(w, r, 422, "setup_error", err.Error())
		return
	}
	writeJSON(w, 201, rec)
}

func (a *API) addExperiment(w http.ResponseWriter, r *http.Request) {
	if a.Setup == nil {
		writeErr(w, r, 503, "unavailable", "setup research is disabled")
		return
	}
	var in contract.ExperimentCreate
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil {
		writeErr(w, r, 400, "bad_request", err.Error())
		return
	}
	exp, err := a.Setup.AddExperiment(r.Context(), r.PathValue("id"), in)
	if err != nil {
		writeErr(w, r, 422, "setup_error", err.Error())
		return
	}
	writeJSON(w, 201, exp)
}

func (a *API) transcribe(w http.ResponseWriter, r *http.Request) {
	if a.Voice == nil {
		writeErr(w, r, 503, "unavailable", "voice is not configured")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	wav, err := io.ReadAll(r.Body)
	if err != nil {
		writeErr(w, r, 413, "too_large", err.Error())
		return
	}
	if len(wav) < 44 || string(wav[0:4]) != "RIFF" {
		writeErr(w, r, 400, "bad_request", "RIFF/WAV 16-bit PCM mono 16kHz is required")
		return
	}
	text, err := a.Voice.Transcribe(r.Context(), wav)
	if err != nil {
		writeErr(w, r, 502, "provider_error", err.Error())
		return
	}
	writeJSON(w, 200, contract.Transcript{Text: text, RequestID: "voice-" + r.Header.Get("X-Request-Id")})
}

func (a *API) speech(w http.ResponseWriter, r *http.Request) {
	if a.Voice == nil {
		writeErr(w, r, 503, "unavailable", "voice is not configured")
		return
	}
	var req contract.SpeechRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil || req.InvocationID == "" {
		writeErr(w, r, 400, "bad_request", "invocation_id is required")
		return
	}
	wav, err := a.Voice.Speech(r.Context(), req.InvocationID)
	if err != nil {
		writeErr(w, r, 502, "provider_error", err.Error())
		return
	}
	w.Header().Set("Content-Type", "audio/wav")
	w.WriteHeader(200)
	_, _ = w.Write(wav)
}

var _ = context.Background
