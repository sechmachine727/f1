package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"f1tele/server/internal/analysis"
	"f1tele/server/internal/auth"
	"f1tele/server/internal/config"
	"f1tele/server/internal/contract"
	"f1tele/server/internal/history"
	"f1tele/server/internal/protocol"
	"f1tele/server/internal/state"
	"f1tele/server/internal/strategy"

	"github.com/coder/websocket"
)

func deps(t *testing.T) (Deps, *state.Reducer) {
	t.Helper()
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
	t.Cleanup(func() { cat.Close() })
	cfg := store.Config()
	red := state.New("ctx", time.Now)
	rec := history.NewRecorder(cfg.Recording, cat, dir, nil)
	return Deps{
		Config: store, Catalog: cat, Reducer: red, Recorder: rec,
		Engine:     &analysis.Engine{Catalog: cat, DataDir: dir},
		Evaluator:  strategy.NewEvaluator(&analysis.Engine{Catalog: cat, DataDir: dir}, cfg),
		Arbiter:    strategy.NewArbiter(cfg.Notifications, cfg.ReferenceProfiles, cfg.CapacityProfiles),
		InstanceID: "inst-test", Version: "1.0.0", Hub: NewHub(),
	}, red
}

func TestHealthNeedsNoCredentialButRejectsCrossOrigin(t *testing.T) {
	d, _ := deps(t)
	srv := httptest.NewServer(New(d).Handler(nil))
	defer srv.Close()
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/health", nil)
	req.Header.Set("Origin", "http://evil.example")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-origin status %d", resp.StatusCode)
	}
	resp.Body.Close()
	req, _ = http.NewRequest("GET", srv.URL+"/api/v1/health", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("same-origin status %d", resp.StatusCode)
	}
	var h contract.Health
	if err := json.NewDecoder(resp.Body).Decode(&h); err != nil {
		t.Fatal(err)
	}
	if h.State != "awaiting_telemetry" || !h.Features["history"] || h.Features["engineer"] {
		t.Fatalf("health %#v", h)
	}
}

func TestStreamFingerprintIgnoresObservationTime(t *testing.T) {
	first := contract.Available(42.0, "m/s", "t")
	second := contract.Available(42.0, "m/s", "t")
	a, b := int64(1000), int64(2000)
	first.AsOfMs, second.AsOfMs = &a, &b
	if printOf(first) != printOf(second) {
		t.Fatal("unchanged value must not produce a delta just because it was re-observed")
	}
	second.Value = 43.0
	if printOf(first) == printOf(second) {
		t.Fatal("value change must produce a delta")
	}
}

func TestConfigPreconditionAndRedaction(t *testing.T) {
	d, _ := deps(t)
	srv := httptest.NewServer(New(d).Handler(nil))
	defer srv.Close()
	cfg := d.Config.Config()
	body, _ := json.Marshal(contract.ConfigPatch{Config: cfg})
	req, _ := http.NewRequest("PATCH", srv.URL+"/api/v1/config", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 428 {
		t.Fatalf("missing precondition status %d", resp.StatusCode)
	}
	resp.Body.Close()
	get, err := http.Get(srv.URL + "/api/v1/config")
	if err != nil {
		t.Fatal(err)
	}
	defer get.Body.Close()
	var view contract.ConfigView
	if err := json.NewDecoder(get.Body).Decode(&view); err != nil {
		t.Fatal(err)
	}
	if len(view.SecretsConfigured) != 0 {
		t.Fatalf("unexpected secrets %#v", view.SecretsConfigured)
	}
	for _, key := range []string{"http", "udp", "data_dir", "recording.cap_bytes"} {
		if view.Sources[key] == "" {
			t.Fatalf("config view has no source for %s: %#v", key, view.Sources)
		}
	}
	if view.UDPAdvertiseHost == "" {
		t.Fatal("config view has no game-facing UDP host")
	}
}

func TestAuthenticationGateAndOrigin(t *testing.T) {
	d, _ := deps(t)
	t.Setenv("F1TELE_AUTH_PASSWORD", "test-password")
	d.Auth = auth.New(d.Config.DataDir())
	if _, err := d.Auth.Ensure(); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(New(d).Handler(nil))
	defer srv.Close()
	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/config", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated config status %d", resp.StatusCode)
	}
	resp.Body.Close()

	login, err := http.Post(srv.URL+"/api/v1/auth/login", "application/json", strings.NewReader("{\"password\":\"test-password\"}"))
	if err != nil {
		t.Fatal(err)
	}
	defer login.Body.Close()
	if login.StatusCode != http.StatusOK || len(login.Cookies()) == 0 {
		t.Fatalf("login status %d cookies %d", login.StatusCode, len(login.Cookies()))
	}
	cookie := login.Cookies()[0]
	req, _ = http.NewRequest("GET", srv.URL+"/api/v1/config", nil)
	req.AddCookie(cookie)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("authenticated config status %d", resp.StatusCode)
	}
	resp.Body.Close()

	req, _ = http.NewRequest("GET", srv.URL+"/api/v1/config", nil)
	req.AddCookie(cookie)
	req.Header.Set("Origin", "http://evil.example")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("cross-origin authenticated status %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestPromptRevisionRoutes(t *testing.T) {
	d, _ := deps(t)
	t.Setenv("F1TELE_AUTH_PASSWORD", "test-password")
	d.Auth = auth.New(d.Config.DataDir())
	if _, err := d.Auth.Ensure(); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(New(d).Handler(nil))
	defer srv.Close()
	login, err := http.Post(srv.URL+"/api/v1/auth/login", "application/json", strings.NewReader("{\"password\":\"test-password\"}"))
	if err != nil {
		t.Fatal(err)
	}
	defer login.Body.Close()
	cookie := login.Cookies()[0]
	body := "custom race engineer prompt"
	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/agent/prompts/live_question/revisions", strings.NewReader("{\"body\":\""+body+"\"}"))
	req.AddCookie(cookie)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("create prompt status %d: %s", resp.StatusCode, raw)
	}
	var rev history.PromptRevision
	if err := json.NewDecoder(resp.Body).Decode(&rev); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	req, _ = http.NewRequest("DELETE", srv.URL+"/api/v1/agent/prompts/live_question/revisions/"+rev.ID, nil)
	req.AddCookie(cookie)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("delete current prompt status %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestActivityStreamWritesSSEFrames(t *testing.T) {
	d, _ := deps(t)
	srv := httptest.NewServer(New(d).Handler(nil))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/v1/engineer/activity/stream")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content type %q", ct)
	}
	reader := bufio.NewReader(resp.Body)
	line, err := reader.ReadString('\n')
	if err != nil || line != "retry: 3000\n" {
		t.Fatalf("first SSE line %q err %v", line, err)
	}
	line, err = reader.ReadString('\n')
	if err != nil || line != "\n" {
		t.Fatalf("SSE separator %q err %v", line, err)
	}
}

func TestEnvControlledConfigPatchConflicts(t *testing.T) {
	t.Setenv("F1TELE_UDP_ADDR", "0.0.0.0:30777")
	d, _ := deps(t)
	srv := httptest.NewServer(New(d).Handler(nil))
	defer srv.Close()
	cfg := d.Config.Config()
	cfg.UDP.Port = 30778
	body, _ := json.Marshal(contract.ConfigPatch{Config: cfg})
	req, _ := http.NewRequest("PATCH", srv.URL+"/api/v1/config", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("If-Match", strconv.Itoa(d.Config.Revision()))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 409 {
		t.Fatalf("env-controlled patch status %d", resp.StatusCode)
	}
	var e contract.ErrorBody
	if err := json.NewDecoder(resp.Body).Decode(&e); err != nil {
		t.Fatal(err)
	}
	if e.Code != "environment_locked" || !strings.Contains(e.Message, "F1TELE_UDP_ADDR") {
		t.Fatalf("error %#v", e)
	}
	if got := d.Config.Config().UDP.Port; got != 30777 {
		t.Fatalf("effective udp changed to %d", got)
	}
}

func TestRecordedAnalyticsRoutes(t *testing.T) {
	d, _ := deps(t)
	ctx := context.Background()
	if err := d.Catalog.EnsureSourceContext(ctx, "src", "ip"); err != nil {
		t.Fatal(err)
	}
	if err := d.Catalog.CreateSession(ctx, "s", contract.Context{SourceContextID: "src", SessionID: "s", BranchID: "b", ModeFamily: "race"}); err != nil {
		t.Fatal(err)
	}
	if err := d.Catalog.CreateBranch(ctx, contract.Branch{ID: "b", SessionID: "s", IsSurviving: true}); err != nil {
		t.Fatal(err)
	}
	if err := d.Catalog.UpsertParticipant(ctx, "s", 0, "LECLERC", 1, 16, false, true, 1); err != nil {
		t.Fatal(err)
	}
	for _, l := range []history.LapAnalytics{
		{SessionID: "s", BranchID: "b", CarIndex: 0, Lap: 1, LapMs: 90000, Sector1Ms: 28000, Sector2Ms: 31000, Sector3Ms: 31000, Valid: true, Complete: true},
		{SessionID: "s", BranchID: "b", CarIndex: 0, Lap: 2, LapMs: 90200, Sector1Ms: 28100, Sector2Ms: 31000, Sector3Ms: 31100, Valid: true, Complete: true},
	} {
		if err := d.Catalog.UpsertLapAnalytics(ctx, l); err != nil {
			t.Fatal(err)
		}
	}
	srv := httptest.NewServer(New(d).Handler(nil))
	defer srv.Close()
	for _, path := range []string{"/api/v1/sessions/s/analytics", "/api/v1/sessions/s/laps/1?car=0", "/api/v1/sessions/s/compare?laps=0:1,0:2"} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("%s status %d", path, resp.StatusCode)
		}
	}
	resp, err := http.Get(srv.URL + "/api/v1/sessions/s/compare?laps=0:1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 422 {
		t.Fatalf("single lap comparison status %d", resp.StatusCode)
	}
}

func TestStreamSnapshotThenDeltaContinuity(t *testing.T) {
	d, red := deps(t)
	srv := httptest.NewServer(New(d).Handler(nil))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/api/v1/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	conn.SetReadLimit(8 << 20)
	defer conn.Close(websocket.StatusNormalClosure, "")
	var first contract.StreamMessage
	decode(t, ctx, conn, &first)
	if first.Type != "snapshot" || first.BaseSeq != nil {
		t.Fatalf("first message %#v", first)
	}
	sub := "{\"type\":\"subscribe\",\"schema_version\":\"1.0.0\",\"request_id\":\"t\",\"source_id\":\"live\",\"car_indices\":[0],\"channels\":[\"car.speed_mps\"],\"max_hz\":20,\"after_seq\":null}"
	if err := conn.Write(ctx, websocket.MessageText, []byte(sub)); err != nil {
		t.Fatal(err)
	}
	data := protocol.BuildPacket(2026, 6, protocol.Header{Format: 2026, PacketVersion: 1, SessionUID: 5, SessionTime: 1, OverallFrameID: 1}, nil, map[int]map[string]float64{0: {"m_speed": 200}}, nil)
	p, _ := protocol.Decode(data)
	red.Apply(1, time.Now(), data, p, "")
	for {
		var msg contract.StreamMessage
		decode(t, ctx, conn, &msg)
		if msg.Type == "delta" {
			if msg.BaseSeq == nil {
				t.Fatal("delta without base_seq")
			}
			if n, err := strconv.Atoi(*msg.BaseSeq); err != nil || n < 1 {
				t.Fatalf("base seq %v", msg.BaseSeq)
			}
			found := false
			for _, m := range msg.Metrics {
				if m.Channel == "car.speed_mps" && m.Measurement.Availability != "not_received" {
					found = true
				}
			}
			if !found {
				t.Fatalf("delta missing telemetry: %#v", msg.Metrics)
			}
			return
		}
	}
}

func decode(t *testing.T, ctx context.Context, conn *websocket.Conn, v any) {
	t.Helper()
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatal(err)
	}
}

func TestCleanupProtectsActiveRecording(t *testing.T) {
	d, _ := deps(t)
	srv := httptest.NewServer(New(d).Handler(nil))
	defer srv.Close()
	body := "{\"recording_ids\":[\"r1\"],\"session_ids\":[],\"confirmed\":true}"
	resp, err := http.Post(srv.URL+"/api/v1/history/cleanup", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var res contract.CleanupResult
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		t.Fatal(err)
	}
	if len(res.Failures) != 1 || res.Failures[0].Code != "not_found" {
		t.Fatalf("cleanup %#v", res)
	}
}

// TestCleanupRefusesActiveRecording covers the spec scenario: cleanup that only
// touches a protected source returns 409 and deletes nothing.
func TestCleanupRefusesActiveRecording(t *testing.T) {
	d, _ := deps(t)
	if err := d.Catalog.EnsureSourceContext(context.Background(), "c", "ip"); err != nil {
		t.Fatal(err)
	}
	if err := d.Catalog.CreateSession(context.Background(), "s", contract.Context{SourceContextID: "c", ModeFamily: "solo", SeasonEvidence: []string{}}); err != nil {
		t.Fatal(err)
	}
	if err := d.Catalog.CreateFragment(context.Background(), contract.Fragment{ID: "f", SessionID: "s", SourceContextID: "c", StartedAtMs: 1}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "active.mcap")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := d.Catalog.CreateRecording(context.Background(), contract.Recording{ID: "r-active", FragmentID: "f", State: "active", Range: contract.Range{FromSeq: "0", ToSeq: "0"}, Gaps: []string{}}, path, 1); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(New(d).Handler(nil))
	defer srv.Close()
	body := "{\"recording_ids\":[\"r-active\"],\"session_ids\":[],\"confirmed\":true}"
	resp, err := http.Post(srv.URL+"/api/v1/history/cleanup", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 409 {
		t.Fatalf("status %d, want 409", resp.StatusCode)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("protected recording file was removed")
	}
}

func TestUnknownSessionIs404(t *testing.T) {
	d, _ := deps(t)
	srv := httptest.NewServer(New(d).Handler(nil))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/v1/sessions/nope")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

// TestJobQueueLimits proves the documented concurrency limits: a second import
// queues as pending while the single slot is held, and never runs concurrently.
func TestJobQueueLimits(t *testing.T) {
	d, _ := deps(t)
	api := New(d)
	if cap(api.importSlots) != 1 || cap(api.analysisSlots) != 2 {
		t.Fatalf("slots import=%d analysis=%d", cap(api.importSlots), cap(api.analysisSlots))
	}
	api.importSlots <- struct{}{} // hold the only import slot
	defer func() { <-api.importSlots }()
	// A queued export still starts: analysis has its own two slots.
	api.setJobState("missing", "running", nil)
	if _, ok := api.jobs["missing"]; ok {
		t.Fatal("unknown job created")
	}
}

// TestSlowClientResync proves a saturated send queue cannot block other clients
// or intake: the slow client receives resync_required and the rest continue.
func TestSlowClientResync(t *testing.T) {
	h := NewHub()
	slow := &streamClient{out: make(chan contract.StreamMessage, 1), last: map[string]fingerprint{}}
	fast := &streamClient{out: make(chan contract.StreamMessage, 8), last: map[string]fingerprint{}}
	h.clients[slow] = true
	h.clients[fast] = true
	slow.out <- contract.StreamMessage{}
	h.Broadcast(contract.StreamMessage{Type: "delta"})
	select {
	case m := <-slow.out:
		if m.Type != "resync_required" {
			t.Fatalf("slow client got %q, want resync_required", m.Type)
		}
	default:
		t.Fatal("slow client queue grew without a resync signal")
	}
	select {
	case m := <-fast.out:
		if m.Type != "delta" {
			t.Fatalf("fast client got %q", m.Type)
		}
	default:
		t.Fatal("fast client missed the delta")
	}
}
