package history

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"f1tele/server/internal/contract"
	"f1tele/server/internal/protocol"
)

func newTestCatalog(t *testing.T) (*Catalog, string) {
	t.Helper()
	dir := t.TempDir()
	cat, err := OpenCatalog(filepath.Join(dir, "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cat.Close() })
	return cat, dir
}

func TestMigrationAndSeqReservation(t *testing.T) {
	cat, dir := newTestCatalog(t)
	ctx := context.Background()
	if err := cat.EnsureSourceContext(ctx, "ctx1", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	first, err := cat.NextSeq(ctx, "ctx1")
	if err != nil {
		t.Fatal(err)
	}
	var reserved int64
	if err := cat.DB().QueryRow("SELECT next_reserved_seq FROM source_contexts WHERE id='ctx1'").Scan(&reserved); err != nil {
		t.Fatal(err)
	}
	if uint64(reserved) != first+seqBlock {
		t.Fatalf("reservation = %d want %d", reserved, first+seqBlock)
	}
	// Reopening must continue after the reserved block, never reuse it.
	path := filepath.Join(filepath.Dir(dir), "catalog.db")
	_ = path
	cat.Close()
	cat2, err := OpenCatalog(filepath.Join(dir, "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer cat2.Close()
	next, err := cat2.NextSeq(ctx, "ctx1")
	if err != nil {
		t.Fatal(err)
	}
	if next < uint64(reserved) {
		t.Fatalf("seq reused: %d < %d", next, reserved)
	}
}

func TestRecordingRoundTrip(t *testing.T) {
	cat, dir := newTestCatalog(t)
	ctx := context.Background()
	cfg, _ := defaultRecCfg()
	rec := NewRecorder(cfg, cat, dir, nil)
	rec.Start()
	rec.SetIdentity("ses-1", "frg-1", "ctx1")
	if err := cat.EnsureSourceContext(ctx, "ctx1", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if err := cat.CreateSession(ctx, "ses-1", contract.Context{SourceContextID: "ctx1", ModeFamily: "solo", SeasonEvidence: []string{}}); err != nil {
		t.Fatal(err)
	}
	if err := cat.CreateFragment(ctx, contract.Fragment{ID: "frg-1", SessionID: "ses-1", SourceContextID: "ctx1", StartedAtMs: 1}); err != nil {
		t.Fatal(err)
	}
	base := time.UnixMilli(1_700_000_000_000)
	var digests []string
	for i := 0; i < 60; i++ {
		pkt := protocol.BuildPacket(2026, 6, protocol.Header{Format: 2026, GameYear: 26, PacketVersion: 1, SessionUID: 5, SessionTime: float64(i), FrameID: uint32(i), OverallFrameID: uint32(i)}, nil, map[int]map[string]float64{0: {"m_speed": float64(200 + i)}}, nil)
		rec.Record(uint64(i+1), base.Add(time.Duration(i)*time.Millisecond), pkt)
		sum := sha256.Sum256(pkt)
		digests = append(digests, hex.EncodeToString(sum[:]))
	}
	// Duplicate bytes and an invalid datagram must be recorded unchanged.
	dup := protocol.BuildPacket(2026, 6, protocol.Header{Format: 2026, PacketVersion: 1, SessionUID: 5}, nil, map[int]map[string]float64{0: {"m_speed": 200}}, nil)
	rec.Record(61, base, dup)
	rec.Record(62, base, []byte{1, 2, 3})
	rec.Stop()
	paths, err := cat.RecordingsForSession(ctx, "ses-1")
	if err != nil || len(paths) != 1 {
		t.Fatalf("paths %v err %v", paths, err)
	}
	src, err := LoadReplay(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(src.Messages) != 62 {
		t.Fatalf("messages = %d want 62", len(src.Messages))
	}
	if src.Messages[0].ReceivedAt != base.UnixMilli() {
		t.Fatalf("timestamp = %d want %d", src.Messages[0].ReceivedAt, base.UnixMilli())
	}
	got := sha256.Sum256(src.Messages[0].Data)
	if hex.EncodeToString(got[:]) != digests[0] {
		t.Fatal("payload bytes changed")
	}
	if src.Messages[61].Data[0] != 1 || len(src.Messages[61].Data) != 3 {
		t.Fatal("invalid datagram not recorded unchanged")
	}
	n, digest, _, err := VerifyRecording(paths[0])
	if err != nil || n != 62 || digest == "" {
		t.Fatalf("verify n=%d digest=%q err=%v", n, digest, err)
	}
}

func TestStorageFailureIsolation(t *testing.T) {
	// Checkpoints are rate-limited: a stalled clock must not make every write
	// durable. Without this, a 2 kHz stream fsyncs thousands of times a second.
	t.Run("checkpoint cadence", func(t *testing.T) {
		cat, dir := newTestCatalog(t)
		cfg, _ := defaultRecCfg()
		cfg.ChunkBytes = 1
		cfg.ChunkFlushMs = 1000
		now := time.Unix(1_700_000_000, 0)
		rec := NewRecorder(cfg, cat, dir, nil)
		rec.now = func() time.Time { return now }
		rec.Start()
		rec.SetIdentity("s", "f", "c")
		ctx := context.Background()
		if err := cat.EnsureSourceContext(ctx, "c", "ip"); err != nil {
			t.Fatal(err)
		}
		_ = cat.CreateSession(ctx, "s", contract.Context{SourceContextID: "c", ModeFamily: "solo", SeasonEvidence: []string{}})
		_ = cat.CreateFragment(ctx, contract.Fragment{ID: "f", SessionID: "s", SourceContextID: "c", StartedAtMs: 1})
		pkt := protocol.BuildPacket(2026, 6, protocol.Header{Format: 2026, PacketVersion: 1, SessionUID: 5}, nil, map[int]map[string]float64{0: {"m_speed": 200}}, nil)
		deadline := time.Now().Add(2 * time.Second)
		// The first checkpoint after opening is expected.
		rec.Record(1, now, pkt)
		for time.Now().Before(deadline) {
			time.Sleep(20 * time.Millisecond)
			if durableOffset(t, cat) > 0 {
				break
			}
		}
		base := durableOffset(t, cat)
		if base == 0 {
			t.Fatal("no initial checkpoint")
		}
		for i := 0; i < 50; i++ {
			rec.Record(uint64(i+2), now, pkt)
			time.Sleep(10 * time.Millisecond)
			if offset := durableOffset(t, cat); offset != base {
				t.Fatalf("checkpointed again (%d -> %d) with a frozen clock", base, offset)
			}
			if rec, _ := rec.State(); rec != RecRecording {
				t.Fatalf("recorder left recording: %s", rec)
			}
		}
		now = now.Add(2 * time.Second)
		rec.Record(51, now, pkt)
		for time.Now().Before(deadline) {
			time.Sleep(20 * time.Millisecond)
			if durableOffset(t, cat) > 0 {
				rec.Stop()
				return
			}
		}
		t.Fatal("no checkpoint after the flush window elapsed")
	})
	t.Run("queue", func(t *testing.T) {
		cat, dir := newTestCatalog(t)
		cfg, _ := defaultRecCfg()
		cfg.QueueBytes = 1
		rec := NewRecorder(cfg, cat, dir, nil)
		rec.Start()
		rec.SetIdentity("s", "f", "c")
		rec.Record(1, time.Now(), []byte{1, 2, 3})
		if state, reason := rec.State(); state != RecStoppedLimit || reason == "" {
			t.Fatalf("state=%s reason=%q", state, reason)
		}
		rec.Stop()
	})
	t.Run("cap", func(t *testing.T) {
		cat, dir := newTestCatalog(t)
		cfg, _ := defaultRecCfg()
		cfg.CapBytes = 100
		rec := NewRecorder(cfg, cat, dir, nil)
		rec.Start()
		rec.SetIdentity("s", "f", "c")
		if err := cat.EnsureSourceContext(context.Background(), "c", "ip"); err != nil {
			t.Fatal(err)
		}
		rec.Record(1, time.Now(), make([]byte, 200))
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if state, _ := rec.State(); state == RecStoppedLimit {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		state, reason := rec.State()
		t.Fatalf("state=%s reason=%q want limit", state, reason)
	})
}

func TestStorageWarningAndReserve(t *testing.T) {
	cat, dir := newTestCatalog(t)
	cfg, _ := defaultRecCfg()
	cfg.CapBytes = 1000
	cfg.FreeReserveBytes = 1 << 40 // larger than any realistic free space
	rec := NewRecorder(cfg, cat, dir, func(string) (int64, int64, error) { return 1 << 30, 1 << 40, nil })
	rec.Start()
	rec.SetIdentity("s", "f", "c")
	if err := cat.EnsureSourceContext(context.Background(), "c", "ip"); err != nil {
		t.Fatal(err)
	}
	_ = cat.CreateSession(context.Background(), "s", contract.Context{SourceContextID: "c", ModeFamily: "solo", SeasonEvidence: []string{}})
	_ = cat.CreateFragment(context.Background(), contract.Fragment{ID: "f", SessionID: "s", SourceContextID: "c", StartedAtMs: 1})
	rec.Record(1, time.Now(), make([]byte, 100))
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if state, _ := rec.State(); state == RecStoppedLimit {
			rec.Stop()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	state, reason := rec.State()
	t.Fatalf("reserve not enforced: state=%s reason=%q", state, reason)
}

func TestWarnFraction(t *testing.T) {
	cat, dir := newTestCatalog(t)
	cfg, _ := defaultRecCfg()
	cfg.CapBytes = 1000
	rec := NewRecorder(cfg, cat, dir, nil)
	if got := rec.WarnFraction(); got != 0 {
		t.Fatalf("empty storage warn fraction = %v", got)
	}
}

func TestSurvivingBranchTimeline(t *testing.T) {
	cat, _ := newTestCatalog(t)
	ctx := context.Background()
	_ = cat.EnsureSourceContext(ctx, "c", "ip")
	_ = cat.CreateSession(ctx, "s", contract.Context{SourceContextID: "c", ModeFamily: "solo", SeasonEvidence: []string{}})
	anchor := func(n int) *string { s := strconv.Itoa(n); return &s }
	root := contract.Branch{ID: "root", SessionID: "s", IsSurviving: false}
	a := contract.Branch{ID: "a", SessionID: "s", ParentID: ptr("root"), AnchorSeq: anchor(400), IsSurviving: false}
	b := contract.Branch{ID: "b", SessionID: "s", ParentID: ptr("a"), AnchorSeq: anchor(700), IsSurviving: true}
	for _, br := range []contract.Branch{root, a, b} {
		if err := cat.CreateBranch(ctx, br); err != nil {
			t.Fatal(err)
		}
	}
	segs, err := cat.InheritedSegments(ctx, "b")
	if err != nil {
		t.Fatal(err)
	}
	if len(segs) != 3 {
		t.Fatalf("segments = %#v", segs)
	}
	if segs[0].BranchID != "root" || segs[0].ToSeq != 399 || segs[1].FromSeq != 400 || segs[1].ToSeq != 699 || segs[2].FromSeq != 700 {
		t.Fatalf("segments = %#v", segs)
	}
}

func TestLegacyImportAndRecovery(t *testing.T) {
	cat, dir := newTestCatalog(t)
	src := filepath.Join("..", "..", "..", "tests", "fixtures", "real_2025_packets.f1bin")
	before, _ := os.ReadFile(src)
	res, err := ImportF1Bin(context.Background(), cat, src, dir, "ctx-import", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(src)
	if hex.EncodeToString(sum(before)) != hex.EncodeToString(sum(after)) {
		t.Fatal("import modified the original")
	}
	if res.Records != 13 || res.Artifact == "" {
		t.Fatalf("import %#v", res)
	}
	// Truncated tail fixture.
	trunc := filepath.Join(t.TempDir(), "trunc.f1bin")
	if err := os.WriteFile(trunc, before[:len(before)-3], 0o600); err != nil {
		t.Fatal(err)
	}
	res2, err := ImportF1Bin(context.Background(), cat, trunc, dir, "ctx-import2", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if !res2.TruncatedTail {
		t.Fatal("truncated tail not reported")
	}
	// Recovery from a damaged recording keeps valid prefix and reports the tail.
	// Build a multi-chunk recording so truncation leaves a valid prefix.
	rcfg, _ := defaultRecCfg()
	rcfg.ChunkBytes = 4096
	multi := NewRecorder(rcfg, cat, dir, nil)
	multi.Start()
	multi.SetIdentity("ses-multi", "frg-multi", "ctx-multi")
	_ = cat.EnsureSourceContext(context.Background(), "ctx-multi", "ip")
	_ = cat.CreateSession(context.Background(), "ses-multi", contract.Context{SourceContextID: "ctx-multi", ModeFamily: "solo", SeasonEvidence: []string{}})
	_ = cat.CreateFragment(context.Background(), contract.Fragment{ID: "frg-multi", SessionID: "ses-multi", SourceContextID: "ctx-multi", StartedAtMs: 1})
	for i := 0; i < 300; i++ {
		pkt := protocol.BuildPacket(2026, 6, protocol.Header{Format: 2026, PacketVersion: 1, SessionUID: 5, SessionTime: float64(i), OverallFrameID: uint32(i)}, nil, map[int]map[string]float64{0: {"m_speed": 200}}, nil)
		multi.Record(uint64(i+1), time.UnixMilli(1_700_000_000_000+int64(i)), pkt)
	}
	multi.Stop()
	paths, _ := cat.RecordingsForSession(context.Background(), "ses-multi")
	damaged := filepath.Join(t.TempDir(), "damaged.mcap")
	raw, _ := os.ReadFile(paths[0])
	if err := os.WriteFile(damaged, raw[:len(raw)-40], 0o600); err != nil {
		t.Fatal(err)
	}
	rres, err := RecoverFile(damaged, filepath.Join(dir, "recovered"))
	if err != nil {
		t.Fatal(err)
	}
	if rres.Messages == 0 || !rres.LostTail {
		t.Fatalf("recovery %#v", rres)
	}
	if _, err := os.Stat(damaged); err != nil {
		t.Fatal("damaged source removed")
	}
}

// TestReplayPlayhead covers a seek on a real recording: the playhead must move
// to the requested session time and never past the end of the file.
func TestReplayPlayhead(t *testing.T) {
	cat, dir := newTestCatalog(t)
	ctx := context.Background()
	cfg, _ := defaultRecCfg()
	rec := NewRecorder(cfg, cat, dir, nil)
	rec.Start()
	rec.SetIdentity("s", "f", "c")
	if err := cat.EnsureSourceContext(ctx, "c", "ip"); err != nil {
		t.Fatal(err)
	}
	if err := cat.CreateSession(ctx, "s", contract.Context{SourceContextID: "c", ModeFamily: "solo", SeasonEvidence: []string{}}); err != nil {
		t.Fatal(err)
	}
	if err := cat.CreateFragment(ctx, contract.Fragment{ID: "f", SessionID: "s", SourceContextID: "c", StartedAtMs: 1}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 40; i++ {
		pkt := protocol.BuildPacket(2026, 6, protocol.Header{Format: 2026, PacketVersion: 1, SessionUID: 5, SessionTime: float64(i) * 0.5, OverallFrameID: uint32(i)}, nil, map[int]map[string]float64{0: {"m_speed": 200}}, nil)
		rec.Record(uint64(i+1), time.UnixMilli(1_700_000_000_000+int64(i)*500), pkt)
	}
	rec.Stop()
	paths, err := cat.RecordingsForSession(ctx, "s")
	if err != nil || len(paths) != 1 {
		t.Fatalf("paths %v err %v", paths, err)
	}
	src, err := LoadReplay(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	rp := NewReplay("r", "s", "b", 1, false, src)
	if v := rp.View(); v.PlayheadSessionTimeMs != 0 {
		t.Fatalf("fresh replay playhead %d, want 0", v.PlayheadSessionTimeMs)
	}
	rp.SeekTo(10_000)
	if got := rp.View().PlayheadSessionTimeMs; got < 10_000 || got > 20_500 {
		t.Fatalf("playhead after seek = %d ms", got)
	}
	rp.SeekTo(10_000_000)
	if got := rp.View().PlayheadSessionTimeMs; got != 19_500 {
		t.Fatalf("playhead past the end = %d, want last sample 19500", got)
	}
	// Playback advances on its own once unpaused, without touching the source.
	rp.SeekTo(0)
	paused, speed := false, 8.0
	rp.Patch(contract.ReplayPatch{Paused: &paused, Speed: &speed})
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if rp.View().PlayheadSessionTimeMs >= 1_000 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if got := rp.View().PlayheadSessionTimeMs; got < 1_000 {
		t.Fatalf("playback did not advance: %d ms", got)
	}
	rp.Close()
}

func ptr(s string) *string { return &s }

func durableOffset(t *testing.T, cat *Catalog) int64 {
	t.Helper()
	var offset int64
	if err := cat.DB().QueryRow("SELECT COALESCE(MAX(durable_offset),0) FROM recordings").Scan(&offset); err != nil {
		t.Fatal(err)
	}
	return offset
}

func sum(b []byte) []byte { h := sha256.Sum256(b); return h[:] }

func defaultRecCfg() (contract.RecordingConfig, error) {
	return contract.RecordingConfig{Enabled: true, CapBytes: 1 << 30, FreeReserveBytes: 0, WarnFraction: 0.8, QueueBytes: 1 << 20, ChunkBytes: 1 << 16, ChunkFlushMs: 100, RotateBytes: 1 << 30, AutomaticCleanup: false}, nil
}
