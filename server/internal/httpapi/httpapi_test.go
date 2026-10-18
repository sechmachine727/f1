package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"f1tele/server/internal/analysis"
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

func TestHealthNeedsNoOriginOrCredential(t *testing.T) {
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
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var h contract.Health
	if err := json.NewDecoder(resp.Body).Decode(&h); err != nil {
		t.Fatal(err)
	}
	if h.State != "awaiting_telemetry" || !h.Features["history"] || h.Features["engineer"] {
		t.Fatalf("health %#v", h)
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
