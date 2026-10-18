// Package httpapi exposes the versioned v1 HTTP and WebSocket surface.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"f1tele/server/internal/analysis"
	"f1tele/server/internal/config"
	"f1tele/server/internal/contract"
	"f1tele/server/internal/history"
	"f1tele/server/internal/state"
	"f1tele/server/internal/strategy"
)

type Deps struct {
	Config        *config.Store
	Catalog       *history.Catalog
	Reducer       *state.Reducer
	Recorder      *history.Recorder
	Engine        *analysis.Engine
	Evaluator     *strategy.Evaluator
	Arbiter       *strategy.Arbiter
	InstanceID    string
	Version       string
	StartedAt     time.Time
	Stop          func()
	OnRecordStart func() error
	OnRecordStop  func()
	Engineer      EngineerAPI
	Setup         SetupAPI
	Voice         VoiceAPI
	Hub           *Hub
	Latency       *LatencyStats
}

type EngineerAPI interface {
	Request(ctx context.Context, r contract.Request) (contract.Invocation, error)
	Invocations(ctx context.Context, session, branch, cursor string, limit int, status string) ([]contract.Invocation, string, error)
	Cancel(ctx context.Context, id string) error
}

type SetupAPI interface {
	Search(ctx context.Context, q SetupQuery) (contract.SearchResult, error)
	Fetch(ctx context.Context, url, sourceID string) (contract.SourceInspection, error)
	List(ctx context.Context) ([]contract.SetupRecord, error)
	Create(ctx context.Context, invocationID string) (contract.SetupRecord, error)
	AddExperiment(ctx context.Context, recordID string, in contract.ExperimentCreate) (contract.Experiment, error)
}

type SetupQuery struct {
	Query            string
	RegulationSeason *int
	GameBuild        *string
	TrackID          *int
	Conditions       string
	Assists          string
	UseCase          string
	MaxResults       int
}

type VoiceAPI interface {
	Transcribe(ctx context.Context, wav []byte) (string, error)
	Speech(ctx context.Context, invocationID string) ([]byte, error)
}

type API struct {
	Deps
	mu        sync.Mutex
	replays   map[string]*history.Replay
	jobs      map[string]*job
	downloads map[string]string
}

type job struct {
	Job      contract.Job
	Cancel   context.CancelFunc
	Path     string
	Download string
}

func New(d Deps) *API {
	if d.Latency == nil {
		d.Latency = &LatencyStats{}
	}
	return &API{Deps: d, replays: map[string]*history.Replay{}, jobs: map[string]*job{}, downloads: map[string]string{}}
}

func (a *API) Handler(static http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", a.health)
	mux.HandleFunc("GET /api/v1/channels", a.channels)
	mux.HandleFunc("GET /api/v1/config", a.getConfig)
	mux.HandleFunc("PATCH /api/v1/config", a.patchConfig)
	mux.HandleFunc("GET /api/v1/sessions", a.listSessions)
	mux.HandleFunc("GET /api/v1/sessions/{id}", a.getSession)
	mux.HandleFunc("PATCH /api/v1/sessions/{id}", a.patchSession)
	mux.HandleFunc("GET /api/v1/sessions/{id}/branches", a.getBranches)
	mux.HandleFunc("GET /api/v1/sessions/{id}/laps", a.getLaps)
	mux.HandleFunc("GET /api/v1/careers", a.listCareers)
	mux.HandleFunc("POST /api/v1/careers", a.createCareer)
	mux.HandleFunc("POST /api/v1/analysis/query", a.analysisQuery)
	mux.HandleFunc("POST /api/v1/analysis/compare", a.analysisCompare)
	mux.HandleFunc("POST /api/v1/strategy/evaluate", a.strategyEvaluate)
	mux.HandleFunc("POST /api/v1/imports", a.importFile)
	mux.HandleFunc("POST /api/v1/exports", a.exportFile)
	mux.HandleFunc("GET /api/v1/jobs/{id}", a.getJob)
	mux.HandleFunc("DELETE /api/v1/jobs/{id}", a.deleteJob)
	mux.HandleFunc("GET /api/v1/downloads/{id}", a.download)
	mux.HandleFunc("POST /api/v1/replays", a.createReplay)
	mux.HandleFunc("PATCH /api/v1/replays/{id}", a.patchReplay)
	mux.HandleFunc("DELETE /api/v1/replays/{id}", a.deleteReplay)
	mux.HandleFunc("POST /api/v1/engineer/requests", a.engineerRequest)
	mux.HandleFunc("GET /api/v1/engineer/invocations", a.engineerInvocations)
	mux.HandleFunc("POST /api/v1/engineer/invocations/{id}/cancel", a.engineerCancel)
	mux.HandleFunc("POST /api/v1/alerts/{id}/acknowledge", a.acknowledge)
	mux.HandleFunc("GET /api/v1/setups", a.listSetups)
	mux.HandleFunc("POST /api/v1/setups", a.createSetup)
	mux.HandleFunc("POST /api/v1/setups/{id}/experiments", a.addExperiment)
	mux.HandleFunc("POST /api/v1/voice/transcriptions", a.transcribe)
	mux.HandleFunc("POST /api/v1/voice/speech", a.speech)
	mux.HandleFunc("POST /api/v1/recording/start", a.startRecording)
	mux.HandleFunc("POST /api/v1/recording/stop", a.stopRecording)
	mux.HandleFunc("POST /api/v1/history/cleanup", a.cleanup)
	mux.HandleFunc("POST /api/v1/control/stop", a.controlStop)
	mux.HandleFunc("GET /api/v1/stream", a.stream)
	if static != nil {
		mux.Handle("/", static)
	}
	return withRequestID(mux)
}

func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			id = fmt.Sprintf("req-%d", time.Now().UnixNano())
		}
		w.Header().Set("X-Request-Id", id)
		r = r.WithContext(context.WithValue(r.Context(), ctxKeyRequestID, id))
		next.ServeHTTP(w, r)
	})
}

type ctxKey string

const ctxKeyRequestID ctxKey = "request_id"

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, r *http.Request, status int, code, msg string) {
	id, _ := r.Context().Value(ctxKeyRequestID).(string)
	writeJSON(w, status, contract.ErrorBody{Code: code, Message: msg, RequestID: id, Details: nil})
}

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	cx := a.Reducer.Context()
	var active *string
	if !a.Reducer.LastPacketAt().IsZero() {
		s := cx.SessionID
		active = &s
	}
	var last *int64
	if !a.Reducer.LastPacketAt().IsZero() {
		v := a.Reducer.LastPacketAt().UnixMilli()
		last = &v
	}
	rec, _ := a.Recorder.State()
	cfg := a.Config.Config()
	writeJSON(w, 200, contract.Health{
		InstanceID: a.InstanceID, ServerVersion: a.Version, SchemaVersion: contract.SchemaVersion,
		State: a.Reducer.HealthState(), ActiveSessionID: active, RecordingState: rec, LastPacketAtMs: last,
		Features: map[string]bool{"history": true, "analysis": true, "engineer": cfg.Agent.Enabled && len(cfg.Agent.Endpoints) > 0, "setup_research": cfg.Search.Provider != "disabled", "voice": cfg.Voice.Enabled},
	})
}

func (a *API) channels(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"items": channelCatalog(), "next_cursor": nil})
}

func (a *API) getConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, a.Config.View())
}

func (a *API) patchConfig(w http.ResponseWriter, r *http.Request) {
	ifMatch := parseIfMatch(r.Header.Get("If-Match"))
	if ifMatch == nil {
		writeErr(w, r, 428, "precondition_required", "If-Match with the current revision is required")
		return
	}
	var p contract.ConfigPatch
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&p); err != nil {
		writeErr(w, r, 400, "bad_request", err.Error())
		return
	}
	view, err := a.Config.ApplyPatch(p, ifMatch)
	if err != nil {
		if errors.Is(err, config.ErrRevision) {
			writeErr(w, r, 409, "revision_mismatch", err.Error())
			return
		}
		writeErr(w, r, 422, "unsupported_configuration", err.Error())
		return
	}
	writeJSON(w, 200, view)
}

func parseIfMatch(h string) *int {
	h = strings.Trim(h, "\"")
	if h == "" {
		return nil
	}
	v, err := strconv.Atoi(h)
	if err != nil {
		return nil
	}
	return &v
}

func (a *API) listSessions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := history.SessionFilter{Cursor: q.Get("cursor"), ModeFamily: q.Get("mode"), CareerID: q.Get("career")}
	if v := q.Get("limit"); v != "" {
		f.Limit, _ = strconv.Atoi(v)
	}
	if v := q.Get("season"); v != "" {
		n, _ := strconv.Atoi(v)
		f.Season = &n
	}
	if v := q.Get("track"); v != "" {
		n, _ := strconv.Atoi(v)
		f.TrackID = &n
	}
	items, next, err := a.Catalog.ListSessions(r.Context(), f)
	if err != nil {
		writeErr(w, r, 500, "catalog_error", err.Error())
		return
	}
	if items == nil {
		items = []contract.SessionSummary{}
	}
	writeJSON(w, 200, map[string]any{"items": items, "next_cursor": nilIfEmpty(next)})
}

func (a *API) getSession(w http.ResponseWriter, r *http.Request) {
	d, err := a.Catalog.Session(r.Context(), r.PathValue("id"))
	if err != nil {
		status := 500
		if errors.Is(err, history.ErrNotFound) {
			status = 404
		}
		writeErr(w, r, status, "not_found", err.Error())
		return
	}
	writeJSON(w, 200, d)
}

func (a *API) patchSession(w http.ResponseWriter, r *http.Request) {
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&raw); err != nil {
		writeErr(w, r, 400, "bad_request", err.Error())
		return
	}
	var display, career *string
	if v, ok := raw["display_name"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err != nil || len(s) > 120 {
			writeErr(w, r, 400, "bad_request", "display_name invalid")
			return
		}
		display = &s
	}
	if v, ok := raw["career_id"]; ok {
		var s *string
		if err := json.Unmarshal(v, &s); err != nil {
			writeErr(w, r, 400, "bad_request", "career_id invalid")
			return
		}
		career = s
	}
	if err := a.Catalog.PatchSession(r.Context(), r.PathValue("id"), display, career); err != nil {
		writeErr(w, r, 500, "catalog_error", err.Error())
		return
	}
	d, err := a.Catalog.Session(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, r, 404, "not_found", err.Error())
		return
	}
	writeJSON(w, 200, d)
}

func (a *API) getBranches(w http.ResponseWriter, r *http.Request) {
	d, err := a.Catalog.Session(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, r, 404, "not_found", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"items": d.Branches, "next_cursor": nil})
}

func (a *API) getLaps(w http.ResponseWriter, r *http.Request) {
	d, err := a.Catalog.Session(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, r, 404, "not_found", err.Error())
		return
	}
	branch := r.URL.Query().Get("branch")
	if branch == "" {
		branch = d.SurvivingBranchID
	}
	var car *int
	if v := r.URL.Query().Get("car"); v != "" {
		n, _ := strconv.Atoi(v)
		car = &n
	}
	laps, err := a.Catalog.Laps(r.Context(), d.Summary.ID, branch, car)
	if err != nil {
		writeErr(w, r, 500, "catalog_error", err.Error())
		return
	}
	if laps == nil {
		laps = []contract.LapSummary{}
	}
	writeJSON(w, 200, map[string]any{"items": laps, "next_cursor": nil})
}

func (a *API) listCareers(w http.ResponseWriter, r *http.Request) {
	rows, err := a.Catalog.DB().QueryContext(r.Context(), "SELECT id,name FROM careers ORDER BY name")
	if err != nil {
		writeErr(w, r, 500, "catalog_error", err.Error())
		return
	}
	defer rows.Close()
	var items []contract.Career
	for rows.Next() {
		var c contract.Career
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			writeErr(w, r, 500, "catalog_error", err.Error())
			return
		}
		items = append(items, c)
	}
	if items == nil {
		items = []contract.Career{}
	}
	writeJSON(w, 200, map[string]any{"items": items, "next_cursor": nil})
}

func (a *API) createCareer(w http.ResponseWriter, r *http.Request) {
	var in contract.CareerCreate
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&in); err != nil || strings.TrimSpace(in.Name) == "" || len(in.Name) > 120 {
		writeErr(w, r, 400, "bad_request", "name is required")
		return
	}
	id := fmt.Sprintf("career-%d", time.Now().UnixNano())
	if _, err := a.Catalog.DB().ExecContext(r.Context(), "INSERT INTO careers(id,name) VALUES(?,?)", id, in.Name); err != nil {
		writeErr(w, r, 500, "catalog_error", err.Error())
		return
	}
	writeJSON(w, 201, contract.Career{ID: id, Name: in.Name})
}

func (a *API) analysisQuery(w http.ResponseWriter, r *http.Request) {
	var q contract.Query
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&q); err != nil {
		writeErr(w, r, 400, "bad_request", err.Error())
		return
	}
	res, err := a.Engine.Query(r.Context(), q)
	if err != nil {
		status, code := classifyAnalysisErr(err)
		writeErr(w, r, status, code, err.Error())
		return
	}
	writeJSON(w, 200, res)
}

func classifyAnalysisErr(err error) (int, string) {
	switch {
	case errors.Is(err, analysis.ErrTooManyCars), errors.Is(err, analysis.ErrTooManyChannels), errors.Is(err, analysis.ErrTooManyPoints), errors.Is(err, analysis.ErrRange):
		return 400, "bad_request"
	case errors.Is(err, history.ErrNotFound):
		return 404, "not_found"
	case errors.Is(err, context.Canceled):
		return 499, "cancelled"
	}
	return 500, "analysis_error"
}

func (a *API) analysisCompare(w http.ResponseWriter, r *http.Request) {
	var q contract.CompareQuery
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&q); err != nil {
		writeErr(w, r, 400, "bad_request", err.Error())
		return
	}
	res, err := a.Engine.Compare(r.Context(), q)
	if err != nil {
		status, code := classifyAnalysisErr(err)
		writeErr(w, r, status, code, err.Error())
		return
	}
	writeJSON(w, 200, res)
}

func (a *API) strategyEvaluate(w http.ResponseWriter, r *http.Request) {
	var q contract.StrategyQuery
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&q); err != nil {
		writeErr(w, r, 400, "bad_request", err.Error())
		return
	}
	if q.ObservationSeq == "" {
		writeErr(w, r, 400, "bad_request", "observation_seq is required")
		return
	}
	snap := a.Reducer.Snapshot(nil, nil)
	if q.BranchID != snap.Context.BranchID {
		writeErr(w, r, 409, "stale_scope", "branch no longer matches the live source")
		return
	}
	res, err := a.Evaluator.Evaluate(r.Context(), q, snap)
	if err != nil {
		if errors.Is(err, strategy.ErrUnsupportedMode) {
			writeErr(w, r, 422, "unsupported_mode", err.Error())
			return
		}
		writeErr(w, r, 500, "strategy_error", err.Error())
		return
	}
	writeJSON(w, 200, res)
}

func (a *API) acknowledge(w http.ResponseWriter, r *http.Request) {
	if !a.Arbiter.Acknowledge(r.PathValue("id")) {
		writeErr(w, r, 404, "not_found", "alert condition is not active")
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (a *API) startRecording(w http.ResponseWriter, r *http.Request) {
	if a.OnRecordStart == nil {
		writeErr(w, r, 503, "unavailable", "recording subsystem unavailable")
		return
	}
	if err := a.OnRecordStart(); err != nil {
		writeErr(w, r, 409, "recording_error", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"recording_state": "recording"})
}

func (a *API) stopRecording(w http.ResponseWriter, r *http.Request) {
	if a.OnRecordStop != nil {
		a.OnRecordStop()
	}
	writeJSON(w, 200, map[string]any{"recording_state": "stopped_user"})
}

func (a *API) controlStop(w http.ResponseWriter, r *http.Request) {
	if a.Stop != nil {
		a.Stop()
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (a *API) cleanup(w http.ResponseWriter, r *http.Request) {
	var req contract.CleanupRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeErr(w, r, 400, "bad_request", err.Error())
		return
	}
	if !req.Confirmed {
		writeErr(w, r, 400, "confirmation_required", "confirmed=true is required")
		return
	}
	res := contract.CleanupResult{DeletedIDs: []string{}, Failures: []contract.CleanupFailure{}}
	rec, _ := a.Recorder.State()
	for _, id := range req.RecordingIDs {
		if rec == "recording" {
			res.Failures = append(res.Failures, contract.CleanupFailure{ID: id, Code: "active", Message: "recording is active"})
			continue
		}
		var path, st string
		err := a.Catalog.DB().QueryRowContext(r.Context(), "SELECT path,state FROM recordings WHERE id=?", id).Scan(&path, &st)
		if err != nil {
			res.Failures = append(res.Failures, contract.CleanupFailure{ID: id, Code: "not_found", Message: err.Error()})
			continue
		}
		if st == "active" {
			res.Failures = append(res.Failures, contract.CleanupFailure{ID: id, Code: "active", Message: "recording protected"})
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			res.Failures = append(res.Failures, contract.CleanupFailure{ID: id, Code: "remove_failed", Message: err.Error()})
			continue
		}
		if err := a.Catalog.DeleteRecordingRow(r.Context(), id); err != nil {
			res.Failures = append(res.Failures, contract.CleanupFailure{ID: id, Code: "catalog_failed", Message: err.Error()})
			continue
		}
		res.DeletedIDs = append(res.DeletedIDs, id)
	}
	writeJSON(w, 200, res)
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (a *API) dataDir() string {
	c := a.Config.Config()
	if c.DataDir != nil && *c.DataDir != "" {
		return *c.DataDir
	}
	return "."
}
