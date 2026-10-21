package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"f1tele/server/internal/config"
	"f1tele/server/internal/contract"
	"f1tele/server/internal/history"
	"f1tele/server/internal/protocol"

	"github.com/coder/websocket"
)

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func waitReady(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			c.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("server did not become ready")
}

func fixturePackets(t *testing.T) [][]byte {
	t.Helper()
	blob, err := os.ReadFile("../../internal/protocol/testdata/golden_2026.bin")
	if err != nil {
		t.Fatal(err)
	}
	var out [][]byte
	for off := 0; off < len(blob); {
		length := int(blob[off+4]) | int(blob[off+5])<<8
		off += 6
		out = append(out, blob[off:off+length])
		off += length
	}
	return out
}

// TestLiveFixtureStream is the UDP-fixture-to-stream integration check: bytes on
// the socket must appear as an available metric over one WebSocket.
func TestLiveFixtureStream(t *testing.T) {
	dir := t.TempDir()
	httpPort, udpPort := freePort(t), freePort(t)
	t.Setenv("F1TELE_DATA_DIR", dir)
	t.Setenv("F1TELE_AUTH_PASSWORD", "integration-password")
	store, err := config.Load(config.Overrides{DataDir: dir, HTTPAddress: "127.0.0.1:" + strconv.Itoa(httpPort), UDPAddress: "127.0.0.1:" + strconv.Itoa(udpPort)})
	if err != nil {
		t.Fatal(err)
	}
	srv, err := New(store, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Run(ctx, slog.New(slog.NewTextHandler(io.Discard, nil))) }()
	waitReady(t, "127.0.0.1:"+strconv.Itoa(httpPort))
	defer func() {
		srv.Shutdown(5 * time.Second)
		cancel()
		<-done
	}()

	login, err := http.Post("http://127.0.0.1:"+strconv.Itoa(httpPort)+"/api/v1/auth/login", "application/json", strings.NewReader("{\"password\":\"integration-password\"}"))
	if err != nil {
		t.Fatal(err)
	}
	defer login.Body.Close()
	if login.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(login.Body)
		t.Fatalf("login status %d: %s", login.StatusCode, raw)
	}
	cookies := login.Cookies()
	if len(cookies) == 0 {
		t.Fatal("login set no cookie")
	}

	conn, err := net.Dial("udp", fmt.Sprintf("127.0.0.1:%d", udpPort))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for i, data := range fixturePackets(t) {
		copy(data[15:], []byte{0, 0, 0, 0})
		copy(data[23:27], []byte{byte(i), 0, 0, 0})
		if _, err := conn.Write(data); err != nil {
			t.Fatal(err)
		}
	}

	dialCtx, dcancel := context.WithTimeout(ctx, 10*time.Second)
	defer dcancel()
	ws, _, err := websocket.Dial(dialCtx, fmt.Sprintf("ws://127.0.0.1:%d/api/v1/stream", httpPort), &websocket.DialOptions{
		HTTPHeader: http.Header{"Cookie": []string{cookies[0].Name + "=" + cookies[0].Value}},
	})
	if err != nil {
		t.Fatal(err)
	}
	ws.SetReadLimit(8 << 20)
	defer ws.Close(websocket.StatusNormalClosure, "")
	// The checked-in 2026 golden corpus uses car 3 as the player car.
	sub := `{"type":"subscribe","schema_version":"1.0.0","request_id":"t","source_id":"live","car_indices":[3],"channels":["car.speed_mps"],"max_hz":60,"after_seq":null}`
	if err := ws.Write(dialCtx, websocket.MessageText, []byte(sub)); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		_, raw, err := ws.Read(dialCtx)
		if err != nil {
			t.Fatal(err)
		}
		var msg contract.StreamMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			t.Fatal(err)
		}
		for _, m := range msg.Metrics {
			if m.Channel == "car.speed_mps" && m.Measurement.Availability == "available" {
				v, ok := m.Measurement.Value.(float64)
				if !ok || v <= 0 {
					t.Fatalf("speed value %#v", m.Measurement.Value)
				}
				return
			}
		}
	}
	t.Fatal("no live speed metric reached the stream")
}

// TestReplayIsolation proves a replay seek cannot move live state or cursors.
func TestReplayIsolation(t *testing.T) {
	dir := t.TempDir()
	cat, err := history.OpenCatalog(filepath.Join(dir, "c.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer cat.Close()
	live := history.NewRecorder(contract.RecordingConfig{Enabled: true, CapBytes: 1 << 30, WarnFraction: 0.8, QueueBytes: 1 << 20, ChunkBytes: 1 << 16, ChunkFlushMs: 50, RotateBytes: 1 << 30}, cat, dir, nil)
	live.Start()
	live.SetIdentity("s", "f", "c")
	_ = cat.EnsureSourceContext(context.Background(), "c", "ip")
	_ = cat.CreateSession(context.Background(), "s", contract.Context{SourceContextID: "c", ModeFamily: "solo", SeasonEvidence: []string{}})
	_ = cat.CreateFragment(context.Background(), contract.Fragment{ID: "f", SessionID: "s", SourceContextID: "c", StartedAtMs: 1})
	for i := 0; i < 40; i++ {
		data := protocol.BuildPacket(2026, 6, protocol.Header{Format: 2026, PacketVersion: 1, SessionUID: 5, SessionTime: float64(i), OverallFrameID: uint32(i)}, nil, map[int]map[string]float64{0: {"m_speed": float64(200 + i)}}, nil)
		live.Record(uint64(i+1), time.UnixMilli(1_700_000_000_000+int64(i)), data)
	}
	live.Stop()
	paths, err := cat.RecordingsForSession(context.Background(), "s")
	if err != nil || len(paths) != 1 {
		t.Fatalf("paths %v err %v", paths, err)
	}
	src, err := history.LoadReplay(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	rp := history.NewReplay("r1", "s", "b", 1, false, src)
	before := rp.View()
	rp.SeekTo(20_000)
	rp.Patch(contract.ReplayPatch{Paused: ptr(false), Speed: ptr(4.0)})
	after := rp.View()
	if before.ID != after.ID || after.Speed != 4 {
		t.Fatalf("replay patch failed: %#v", after)
	}
	if n, _, _, err := history.VerifyRecording(paths[0]); err != nil || n != 40 {
		t.Fatalf("replay mutated the recording: n=%d err=%v", n, err)
	}
	if strings.Contains(after.SourceID, "live") {
		t.Fatal("replay claims the live source")
	}
}

func ptr[T any](v T) *T { return &v }
