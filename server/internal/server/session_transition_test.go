package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"f1tele/server/internal/config"
	"f1tele/server/internal/contract"
	"f1tele/server/internal/protocol"

	"github.com/coder/websocket"
)

func transitionHeader(uid uint64, frame, overall uint32, sessionTime float64) protocol.Header {
	return protocol.Header{Format: 2026, GameYear: 26, PacketVersion: 1, SessionUID: uid, SessionTime: sessionTime, FrameID: frame, OverallFrameID: overall, PlayerCarIndex: 0}
}

func TestStreamClearsPausedOnNewSession(t *testing.T) {
	dir := t.TempDir()
	httpPort, udpPort := freePort(t), freePort(t)
	t.Setenv("F1TELE_DATA_DIR", dir)
	t.Setenv("F1TELE_AUTH_PASSWORD", "transition-password")
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

	login, err := http.Post("http://127.0.0.1:"+strconv.Itoa(httpPort)+"/api/v1/auth/login", "application/json", strings.NewReader(`{"password":"transition-password"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer login.Body.Close()
	if login.StatusCode != http.StatusOK {
		t.Fatalf("login status %d", login.StatusCode)
	}
	cookie := login.Cookies()[0]
	ws, _, err := websocket.Dial(ctx, fmt.Sprintf("ws://127.0.0.1:%d/api/v1/stream", httpPort), &websocket.DialOptions{
		HTTPHeader: http.Header{"Cookie": []string{cookie.Name + "=" + cookie.Value}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close(websocket.StatusNormalClosure, "")
	ws.SetReadLimit(8 << 20)
	sub := `{"type":"subscribe","schema_version":"1.0.0","request_id":"t","source_id":"live","car_indices":[0],"channels":["car.speed_mps","session.paused"],"max_hz":20,"after_seq":null}`
	if err := ws.Write(ctx, websocket.MessageText, []byte(sub)); err != nil {
		t.Fatal(err)
	}

	conn, err := net.Dial("udp", fmt.Sprintf("127.0.0.1:%d", udpPort))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	send := func(data []byte) {
		if _, err := conn.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	// Q1 finishes paused with a high frame number.
	send(protocol.BuildPacket(2026, 1, transitionHeader(71, 10, 10, 100), map[string]float64{"m_gamePaused": 1, "m_sessionType": 5}, nil, nil))
	send(protocol.BuildPacket(2026, 6, transitionHeader(71, 11, 11, 101), nil, map[int]map[string]float64{0: {"m_speed": 300, "m_gear": 7}}, nil))
	// Q2 has a new UID and restarts frame numbering at the session-clock origin.
	send(protocol.BuildPacket(2026, 1, transitionHeader(72, 1, 1, 1), map[string]float64{"m_gamePaused": 0, "m_sessionType": 5}, nil, nil))
	send(protocol.BuildPacket(2026, 6, transitionHeader(72, 2, 2, 2), nil, map[int]map[string]float64{0: {"m_speed": 100, "m_gear": 5}}, nil))

	readCtx, readCancel := context.WithTimeout(ctx, 5*time.Second)
	defer readCancel()
	var last contract.StreamMessage
	for {
		_, raw, err := ws.Read(readCtx)
		if err != nil {
			t.Fatalf("stream before Q2 transition: %v; last=%#v", err, last)
		}
		if err := json.Unmarshal(raw, &last); err != nil {
			t.Fatal(err)
		}
		if last.Context == nil || last.Context.SessionID == "" {
			continue
		}
		speed, paused := metricValue(last, "car.speed_mps@0"), metricValue(last, "session.paused")
		if paused != nil && paused.Value == false && speed != nil && speed.Value == 100.0/3.6 {
			return
		}
	}
}

func metricValue(msg contract.StreamMessage, id string) *contract.Measurement {
	for _, m := range msg.Metrics {
		if m.ID == id {
			return &m.Measurement
		}
	}
	return nil
}
