package analytics

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"f1tele/server/internal/contract"
	"f1tele/server/internal/history"
	"f1tele/server/internal/protocol"
)

func TestSummaryAndCompare(t *testing.T) {
	cat, _, cx := setup(t)
	ctx := context.Background()
	if err := cat.UpsertParticipant(ctx, "s", 0, "LECLERC", 1, 16, false, 1); err != nil {
		t.Fatal(err)
	}
	if err := cat.UpsertParticipant(ctx, "s", 1, "HAMILTON", 2, 44, false, 1); err != nil {
		t.Fatal(err)
	}
	rows := []history.LapAnalytics{
		{SessionID: "s", BranchID: "b", CarIndex: 0, Lap: 1, LapMs: 90000, Sector1Ms: 28000, Sector2Ms: 31000, Sector3Ms: 31000, Valid: true, Complete: true, Compound: "C3"},
		{SessionID: "s", BranchID: "b", CarIndex: 0, Lap: 2, LapMs: 90200, Sector1Ms: 28100, Sector2Ms: 31000, Sector3Ms: 31100, Valid: true, Complete: true, Compound: "C3"},
		{SessionID: "s", BranchID: "b", CarIndex: 1, Lap: 1, LapMs: 89900, Sector1Ms: 27900, Sector2Ms: 31000, Sector3Ms: 31000, Valid: true, Complete: true, Compound: "C4"},
	}
	for _, r := range rows {
		if err := cat.UpsertLapAnalytics(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	sum, err := Summary(ctx, cat, "s")
	if err != nil {
		t.Fatal(err)
	}
	if len(sum.Participants) != 2 || len(sum.Laps) != 3 || !sum.AnalyticsReady {
		t.Fatalf("summary %#v", sum)
	}
	if sum.BestLapMs != 89900 || sum.TheoreticalBest != 89900 {
		t.Fatalf("best/theoretical %d/%d", sum.BestLapMs, sum.TheoreticalBest)
	}
	if len(sum.Stints) == 0 {
		t.Fatal("no stints derived")
	}
	cmp, err := Compare(ctx, cat, "s", "b", []CompareLap{{Car: 0, Lap: 1}, {Car: 1, Lap: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if len(cmp.Laps) != 2 || cmp.Theoretical != 89900 {
		t.Fatalf("compare %#v", cmp)
	}
	if len(cmp.Differences) == 0 {
		t.Fatal("compound difference must be reported")
	}
	if _, err := Compare(ctx, cat, "s", "b", []CompareLap{{Car: 0, Lap: 1}}); err == nil {
		t.Fatal("one lap comparison accepted")
	}
	_ = cx
}

func TestBackfillDerivesWithoutRewritingRecording(t *testing.T) {
	cat, _, cx := setup(t)
	ctx := context.Background()
	dir := t.TempDir()
	if err := cat.CreateFragment(ctx, contract.Fragment{ID: "f", SessionID: "s", SourceContextID: "src", StartedAtMs: 1}); err != nil {
		t.Fatal(err)
	}
	rec := history.NewRecorder(contract.RecordingConfig{Enabled: true, CapBytes: 1 << 30, FreeReserveBytes: 0, WarnFraction: 0.8, QueueBytes: 1 << 20, ChunkBytes: 1 << 16, ChunkFlushMs: 50, RotateBytes: 1 << 30}, cat, dir, nil)
	rec.Start()
	rec.SetIdentity("s", "f", "src")
	for i := 0; i < 40; i++ {
		data := protocol.BuildPacket(2026, 2, protocol.Header{Format: 2026, PacketVersion: 1, SessionUID: 9, FrameID: uint32(i + 1), SessionTime: float64(i)}, nil, map[int]map[string]float64{0: {
			"m_currentLapNum": 1, "m_totalDistance": float64(i) * 50, "m_lapDistance": float64(i) * 50, "m_carPosition": 1,
		}}, nil)
		rec.Record(uint64(i+1), time.UnixMilli(1_700_000_000_000+int64(i)), data)
		sp := protocol.BuildPacket(2026, 6, protocol.Header{Format: 2026, PacketVersion: 1, SessionUID: 9, FrameID: uint32(i + 1)}, nil, map[int]map[string]float64{0: {"m_speed": 200}}, nil)
		rec.Record(uint64(100+i), time.UnixMilli(1_700_000_000_000+int64(i)), sp)
	}
	boundary := protocol.BuildPacket(2026, 2, protocol.Header{Format: 2026, PacketVersion: 1, SessionUID: 9, FrameID: 100}, nil, map[int]map[string]float64{0: {
		"m_currentLapNum": 2, "m_lastLapTimeInMS": 90000, "m_totalDistance": 2000, "m_lapDistance": 0,
	}}, nil)
	rec.Record(200, time.UnixMilli(1_700_000_000_100), boundary)
	rec.Stop()
	hist := protocol.BuildPacket(2026, 11, protocol.Header{Format: 2026, PacketVersion: 1, SessionUID: 9, FrameID: 101}, map[string]float64{"m_carIdx": 0, "m_numLaps": 1}, map[int]map[string]float64{0: {
		"m_lapTimeInMS": 90000, "m_sector1TimeMSPart": 28000, "m_sector2TimeMSPart": 31000, "m_sector3TimeMSPart": 31000, "m_lapValidBitFlags": 1,
	}}, nil)
	rec.Record(201, time.UnixMilli(1_700_000_000_101), hist)
	rec.Stop()
	if err := cat.InsertLap(ctx, contract.LapSummary{ID: "l1", SessionID: "s", BranchID: "b", CarIndex: 0, Lap: 1, StartSeq: "1", EndSeq: "40",
		LapTimeMs: contract.Available(90000, "ms", "t"), DistanceM: contract.Unavailable("not_received", "m", "t", "x"),
		Compound: contract.Available("C3", "compound", "t"), Valid: true, Complete: true, Gaps: []string{}}); err != nil {
		t.Fatal(err)
	}
	paths, err := cat.RecordingsForSession(ctx, "s")
	if err != nil || len(paths) == 0 {
		t.Fatalf("recordings %v %d", err, len(paths))
	}
	before := hashFile(t, paths[0])
	Backfill(ctx, cat, nil, 10)
	var n int
	if err := cat.DB().QueryRow("SELECT COUNT(*) FROM lap_analytics WHERE session_id='s'").Scan(&n); err != nil || n == 0 {
		t.Fatalf("backfilled laps %d err %v", n, err)
	}
	if after := hashFile(t, paths[0]); after != before {
		t.Fatal("backfill rewrote the recording")
	}
	Backfill(ctx, cat, nil, 10)
	var again int
	if err := cat.DB().QueryRow("SELECT COUNT(*) FROM lap_analytics WHERE session_id='s'").Scan(&again); err != nil || again != n {
		t.Fatalf("repeat backfill duplicated rows: %d vs %d", again, n)
	}
	_ = cx
}

func hashFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

var _ = time.Now
var _ = protocol.Version
var _ = contract.SchemaVersion
