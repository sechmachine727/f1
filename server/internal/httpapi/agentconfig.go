package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"f1tele/server/internal/engineer"
	"f1tele/server/internal/history"
)

func (a *API) listPrompts(w http.ResponseWriter, r *http.Request) {
	out := []map[string]any{}
	for _, profile := range engineer.PromptProfiles {
		item, err := a.promptView(r, profile)
		if err != nil {
			writeErr(w, r, http.StatusInternalServerError, "prompt_error", err.Error())
			return
		}
		out = append(out, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (a *API) promptDetail(w http.ResponseWriter, r *http.Request) {
	item, err := a.promptView(r, r.PathValue("profile"))
	if err != nil {
		writeErr(w, r, http.StatusNotFound, "prompt_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) promptView(r *http.Request, profile string) (map[string]any, error) {
	if !engineer.KnownProfile(profile) {
		return nil, &promptError{message: "unknown prompt profile " + profile}
	}
	authored, err := engineer.Authored(profile)
	if err != nil {
		return nil, err
	}
	current, err := a.Catalog.CurrentPrompt(r.Context(), profile)
	if err != nil {
		return nil, err
	}
	revisions, err := a.Catalog.PromptRevisions(r.Context(), profile)
	if err != nil {
		return nil, err
	}
	body := authored
	currentID := ""
	if current != nil {
		body = current.Body
		currentID = current.ID
	}
	return map[string]any{"profile": profile, "authored": authored, "body": body, "current_revision_id": nilIfEmpty(currentID), "revisions": revisions}, nil
}

type promptError struct{ message string }

func (e *promptError) Error() string { return e.message }

func (a *API) createPromptRevision(w http.ResponseWriter, r *http.Request) {
	profile := r.PathValue("profile")
	if !engineer.KnownProfile(profile) {
		writeErr(w, r, http.StatusNotFound, "prompt_error", "unknown prompt profile")
		return
	}
	var body struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 140<<10)).Decode(&body); err != nil {
		writeErr(w, r, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if strings.TrimSpace(body.Body) == "" || len(body.Body) > 128<<10 {
		writeErr(w, r, http.StatusUnprocessableEntity, "invalid_prompt", "prompt must be between 1 byte and 128 KiB")
		return
	}
	rev, err := a.Catalog.CreatePromptRevision(r.Context(), profile, body.Body)
	if err != nil {
		writeErr(w, r, http.StatusInternalServerError, "prompt_error", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, rev)
}

func (a *API) selectPromptRevision(w http.ResponseWriter, r *http.Request) {
	if err := a.Catalog.SelectPromptRevision(r.Context(), r.PathValue("profile"), r.PathValue("id")); err != nil {
		writeErr(w, r, http.StatusNotFound, "prompt_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *API) deletePromptRevision(w http.ResponseWriter, r *http.Request) {
	if err := a.Catalog.DeletePromptRevision(r.Context(), r.PathValue("profile"), r.PathValue("id")); err != nil {
		status := http.StatusConflict
		if err == history.ErrPromptNotFound {
			status = http.StatusNotFound
		}
		writeErr(w, r, status, "prompt_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *API) resetPrompt(w http.ResponseWriter, r *http.Request) {
	profile := r.PathValue("profile")
	if !engineer.KnownProfile(profile) {
		writeErr(w, r, http.StatusNotFound, "prompt_error", "unknown prompt profile")
		return
	}
	if err := a.Catalog.ResetPrompt(r.Context(), profile); err != nil {
		writeErr(w, r, http.StatusInternalServerError, "prompt_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (a *API) contextUsage(w http.ResponseWriter, r *http.Request) {
	if a.Engineer == nil {
		writeErr(w, r, http.StatusServiceUnavailable, "unavailable", "engineer is disabled")
		return
	}
	q := r.URL.Query()
	usage, err := a.Engineer.ContextUsage(r.Context(), q.Get("session"), q.Get("branch"))
	if err != nil {
		writeErr(w, r, http.StatusInternalServerError, "context_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, usage)
}

func (a *API) manualCompact(w http.ResponseWriter, r *http.Request) {
	if a.Engineer == nil {
		writeErr(w, r, http.StatusServiceUnavailable, "unavailable", "engineer is disabled")
		return
	}
	var body struct {
		SessionID string `json:"session_id"`
		BranchID  string `json:"branch_id"`
		Focus     string `json:"focus"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&body); err != nil {
		writeErr(w, r, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	out, err := a.Engineer.Compact(r.Context(), body.SessionID, body.BranchID, body.Focus)
	if err != nil {
		writeErr(w, r, http.StatusUnprocessableEntity, "compaction_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}
