package qualification

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"f1tele/server/internal/config"
	"f1tele/server/internal/contract"
	"f1tele/server/internal/history"
	"f1tele/server/internal/server"

	"github.com/coder/websocket"
	"github.com/foxglove/mcap/go/mcap"
)

var (
	duration = flag.Duration("duration", 0, "workload duration (required)")
	rate     = flag.Int("udp-rate", 2048, "datagrams per second")
	cars     = flag.Int("cars", 24, "car slots")
	seed     = flag.Int64("seed", 20261018, "workload seed")
	clients  = flag.Int("clients", 3, "browser-analogue stream clients")
	evidence = flag.String("evidence-dir", "docs/rework/evidence/release-qualification", "evidence output directory")
	rotateMB = flag.Int("rotate-mb", 0, "override rotation size for a short rotation check (0 uses defaults)")
)

func freePort(t testing.TB) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func nilLogger() *slog.Logger { return slog.New(slog.NewTextHandler(os.Stderr, nil)) }

func waitHTTP(t testing.TB, url string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", url, 200*time.Millisecond)
		if err == nil {
			c.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("server did not become ready")
}

func streamClient(ctx context.Context, url string, i int) error {
	c, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		return err
	}
	c.SetReadLimit(8 << 20)
	defer c.Close(websocket.StatusNormalClosure, "")
	hz := 20
	if i == 0 {
		hz = 60
	} else if i == 1 {
		hz = 5
	}
	sub := fmt.Sprintf(`{"type":"subscribe","schema_version":"1.0.0","request_id":"q%d","source_id":"live","car_indices":[],"channels":[],"max_hz":%d,"after_seq":null}`, i, hz)
	if err := c.Write(ctx, websocket.MessageText, []byte(sub)); err != nil {
		return err
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		readCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		_, _, err := c.Read(readCtx)
		cancel()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if errors.Is(err, context.DeadlineExceeded) {
				continue
			}
			return err
		}
	}
}

func hashPayloads(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r, err := mcap.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	it, err := r.Messages()
	if err != nil {
		return nil, err
	}
	h := sha256.New()
	for {
		_, _, m, err := it.Next(nil)
		if err == io.EOF || m == nil {
			break
		}
		if err != nil {
			return h.Sum(nil), err
		}
		h.Write(m.Data)
	}
	return h.Sum(nil), nil
}

func writeEvidence(t testing.TB, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Logf("evidence dir: %v", err)
		return
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body+string(rune(10))), 0o644); err != nil {
		t.Logf("evidence write: %v", err)
	}
}

func TestRatedWorkload(t *testing.T) {
	if testing.Short() || duration.Seconds() <= 0 {
		t.Skip("rated workload requires -args -duration=<window>")
	}
	dir := t.TempDir()
	httpPort, udpPort := freePort(t), freePort(t)
	t.Setenv("F1TELE_DATA_DIR", dir)
	store, err := config.Load(config.Overrides{DataDir: dir, HTTPAddress: "127.0.0.1:" + strconv.Itoa(httpPort), UDPAddress: "127.0.0.1:" + strconv.Itoa(udpPort)})
	if err != nil {
		t.Fatal(err)
	}
	if *rotateMB > 0 {
		cfg := store.Config()
		cfg.Recording.RotateBytes = int64(*rotateMB) << 20
		rev := store.Revision()
		if _, err := store.ApplyPatch(contract.ConfigPatch{Config: cfg}, &rev); err != nil {
			t.Fatal(err)
		}
	}
	srv, err := server.New(store, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Run(ctx, slog.New(slog.NewTextHandler(os.Stderr, nil))) }()
	select {
	case err := <-done:
		t.Fatalf("server exited before ready: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	waitHTTP(t, fmt.Sprintf("127.0.0.1:%d", httpPort))
	fmt.Fprintf(os.Stderr, "rated server: http://127.0.0.1:%d\n", httpPort)

	clientErrs := make(chan error, *clients)
	var cwg sync.WaitGroup
	for i := 0; i < *clients; i++ {
		cwg.Add(1)
		go func(i int) {
			defer cwg.Done()
			if err := streamClient(ctx, fmt.Sprintf("ws://127.0.0.1:%d/api/v1/stream", httpPort), i); err != nil {
				clientErrs <- err
			}
		}(i)
	}
	time.Sleep(300 * time.Millisecond)

	corpus := &Corpus{Seed: *seed, SessionUID: uint64(*seed), Player: 0, RewindAt: func(second int) float64 {
		switch {
		case second >= 2100 && second < 2130:
			return 30
		case second >= 2160 && second < 2180:
			return 50
		}
		return 0
	}}
	probe := &RSSProbe{}
	probe.Start(time.Second)
	sender := NewSender()
	conn, err := net.Dial("udp", fmt.Sprintf("127.0.0.1:%d", udpPort))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var maxLag int64
	lagStop := make(chan struct{})
	go func() {
		tk := time.NewTicker(200 * time.Millisecond)
		defer tk.Stop()
		for {
			select {
			case <-lagStop:
				return
			case <-tk.C:
				if lp := srv.Reducer().LastPacketAt(); !lp.IsZero() {
					lag := int64(time.Since(lp))
					if lag > maxLag {
						maxLag = lag
					}
				}
			}
		}
	}()
	start := time.Now()
	total := int64(*rate) * int64(duration.Seconds())
	for slot := int64(0); slot < total; slot++ {
		second := int(slot / int64(*rate))
		within := int(slot % int64(*rate))
		data := corpus.BuildSlot(second, within)
		if _, err := conn.Write(data); err != nil {
			t.Fatalf("udp write: %v", err)
		}
		sender.Record(data)
		target := start.Add(time.Duration(float64(slot+1) / float64(*rate) * float64(time.Second)))
		if d := time.Until(target); d > 0 {
			time.Sleep(d)
		}
	}
	elapsed := time.Since(start)
	close(lagStop)
	// Freeze latency before the drain wait: idle time after the sender stops
	// is not presentation latency.
	p95, latencyN := srv.API().Latency.Percentile(0.95)
	p50, _ := srv.API().Latency.Percentile(0.50)
	latencyMax := srv.API().Latency.Max()
	_ = srv.API().Latency.WriteCSV(filepath.Join(*evidence, "latency.csv"))
	time.Sleep(1500 * time.Millisecond)

	recState, recReason := srv.Recorder().State()
	received, bytes, intakeHash := srv.Intake().Snapshot()
	peak, finalRSS, growth, rssOK := probe.Report(filepath.Join(*evidence, "rss.csv"))
	sessionID := srv.Reducer().SessionID()
	branchID := srv.Reducer().BranchID()
	srv.Shutdown(10 * time.Second)
	cancel()
	<-done
	cwg.Wait()
	close(clientErrs)
	for err := range clientErrs {
		t.Errorf("stream client: %v", err)
	}

	cat, err := history.OpenCatalog(filepath.Join(dir, "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	paths, err := cat.RecordingsForSession(context.Background(), sessionID)
	if err != nil {
		t.Fatal(err)
	}
	cat.Close()
	recorded := 0
	recHash := sha256.New()
	for _, p := range paths {
		n, _, _, err := history.VerifyRecording(p)
		if err != nil {
			t.Fatalf("verify %s: %v", p, err)
		}
		recorded += n
		h, err := hashPayloads(p)
		if err != nil {
			t.Fatal(err)
		}
		recHash.Write(h)
	}
	secs := elapsed.Seconds()
	report := fmt.Sprintf(`{"duration_s":%.1f,"slots":%d,"elapsed_slots_per_s":%.1f,"declared_slots_per_s":%d,"transmitted":%d,"transmitted_bytes":%d,"transmitted_sha256":"%s","received":%d,"received_bytes":%d,"received_sha256":"%s","recorded":%d,"recordings":%d,"session_id":"%s","branch_id":"%s","app_drops":%d,"recorder_state":"%s","recorder_reason":"%s"}`,
		secs, sender.Count, float64(sender.Count)/secs, *rate, sender.Count, sender.Bytes, hex.EncodeToString(sender.Sum()),
		received, bytes, hex.EncodeToString(intakeHash), recorded, len(paths), sessionID, branchID, int64(sender.Count-received),
		recState, strings.ReplaceAll(recReason, "\"", "'"))
	report = report[:len(report)-1] + fmt.Sprintf(",\"rss_peak_bytes\":%s,\"rss_final_bytes\":%s,\"rss_growth_last_90m_bytes\":%s,\"rss_available\":%t,\"obs_to_stream_p50_ms\":%.3f,\"obs_to_stream_p95_ms\":%.3f,\"obs_to_stream_max_ms\":%.3f,\"latency_samples\":%d,\"ingest_lag_max_ms\":%d}", fmtBytes(peak), fmtBytes(finalRSS), fmtBytes(growth), rssOK, float64(p50.Microseconds())/1000, float64(p95.Microseconds())/1000, float64(latencyMax.Microseconds())/1000, latencyN, maxLag/1e6)
	writeEvidence(t, *evidence, "rated-workload.json", report)
	t.Logf("%s", report)

	if sender.Count != received {
		t.Errorf("receiver intake %d != transmitted %d", received, sender.Count)
	}
	if hex.EncodeToString(sender.Sum()) != hex.EncodeToString(intakeHash) {
		t.Errorf("intake digest differs from transmitted digest")
	}
	if rssOK && peak > 512<<20 {
		t.Errorf("server peak RSS %d exceeds 512 MiB", peak)
	}
	if rssOK && growth > 64<<20 {
		t.Errorf("server growth %d exceeds 64 MiB", growth)
	}
	if latencyN > 0 && p95 > 250*time.Millisecond {
		t.Errorf("accepted-observation to stream p95 %s exceeds 250 ms", p95)
	}
	if uint64(recorded) != sender.Count {
		t.Errorf("recorded %d != transmitted %d", recorded, sender.Count)
	}
}
