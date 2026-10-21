package analytics

import (
	"context"
	"testing"
	"time"

	"f1tele/server/internal/contract"
	"f1tele/server/internal/history"
	"f1tele/server/internal/protocol"
)

func recordForBackfill(t *testing.T, cat *history.Catalog, dir, sessionID, fragmentID string, packets [][]byte) {
	t.Helper()
	rec := history.NewRecorder(contract.RecordingConfig{Enabled: true, CapBytes: 1 << 30, FreeReserveBytes: 0,
		WarnFraction: 0.8, QueueBytes: 1 << 20, ChunkBytes: 1 << 16, ChunkFlushMs: 50, RotateBytes: 1 << 30}, cat, dir, nil)
	rec.Start()
	rec.SetIdentity(sessionID, fragmentID, "src")
	for i, raw := range packets {
		rec.Record(uint64(i+1), time.UnixMilli(1_700_000_000_000+int64(i)), raw)
	}
	rec.Stop()
}

func packetBytes(t *testing.T, pid uint8, frame uint32, scalar map[string]float64, cars map[int]map[string]float64, strings map[string]string) []byte {
	t.Helper()
	return protocol.BuildPacket(2026, pid, protocol.Header{Format: 2026, PacketVersion: 1, SessionUID: 9, FrameID: frame, OverallFrameID: frame, SessionTime: float64(frame)}, scalar, cars, strings)
}

func TestBackfillRewindUsesSurvivingBranch(t *testing.T) {
	cat, _, _ := setup(t)
	ctx := context.Background()
	if err := cat.CreateFragment(ctx, contract.Fragment{ID: "f", SessionID: "s", SourceContextID: "src", StartedAtMs: 1}); err != nil {
		t.Fatal(err)
	}
	anchor := "5"
	if err := cat.CreateBranch(ctx, contract.Branch{ID: "child", SessionID: "s", ParentID: ptrString("b"), AnchorSeq: &anchor, IsSurviving: true}); err != nil {
		t.Fatal(err)
	}
	if err := cat.SetSurviving(ctx, "s", "child"); err != nil {
		t.Fatal(err)
	}
	if err := cat.InsertLap(ctx, contract.LapSummary{ID: "l1", SessionID: "s", BranchID: "child", CarIndex: 0, Lap: 1,
		StartSeq: "1", EndSeq: "2", LapTimeMs: contract.Available(90000, "ms", "t"), Valid: true, Complete: true}); err != nil {
		t.Fatal(err)
	}
	pkts := [][]byte{
		packetBytes(t, 1, 1, map[string]float64{"m_trackLength": 3000}, nil, nil),
		packetBytes(t, 2, 2, nil, map[int]map[string]float64{0: {"m_currentLapNum": 1, "m_lapDistance": 100, "m_totalDistance": 100}}, nil),
		packetBytes(t, 2, 3, nil, map[int]map[string]float64{0: {"m_currentLapNum": 2, "m_lastLapTimeInMS": 90000, "m_lapDistance": 0, "m_totalDistance": 5000}}, nil),
		packetBytes(t, 3, 4, nil, nil, map[string]string{"m_eventStringCode": "FLBK"}),
		packetBytes(t, 2, 5, nil, map[int]map[string]float64{0: {"m_currentLapNum": 1, "m_lapDistance": 100, "m_totalDistance": 100}}, nil),
		packetBytes(t, 2, 6, nil, map[int]map[string]float64{0: {"m_currentLapNum": 2, "m_lastLapTimeInMS": 80000, "m_lapDistance": 0, "m_totalDistance": 5000}}, nil),
	}
	recordForBackfill(t, cat, t.TempDir(), "s", "f", pkts)
	Backfill(ctx, cat, nil, 10)
	laps, err := cat.LapAnalytics(ctx, "s", "child", 0)
	complete := []history.LapAnalytics{}
	for _, l := range laps {
		if l.Complete {
			complete = append(complete, l)
		}
	}
	if err != nil || len(complete) != 1 {
		t.Fatalf("complete laps %#v err %v", complete, err)
	}
	if complete[0].LapMs != 80000 {
		t.Fatalf("rewound lap was not derived from the surviving timeline: %d", complete[0].LapMs)
	}
	if complete[0].BranchID != "child" {
		t.Fatalf("derived row used branch %s", complete[0].BranchID)
	}
}

func TestBackfillUsesPacketTrackLength(t *testing.T) {
	cat, _, _ := setup(t)
	ctx := context.Background()
	if err := cat.CreateFragment(ctx, contract.Fragment{ID: "f", SessionID: "s", SourceContextID: "src", StartedAtMs: 1}); err != nil {
		t.Fatal(err)
	}
	if err := cat.InsertLap(ctx, contract.LapSummary{ID: "l1", SessionID: "s", BranchID: "b", CarIndex: 0, Lap: 1,
		StartSeq: "1", EndSeq: "2", LapTimeMs: contract.Available(90000, "ms", "t"), Valid: true, Complete: true}); err != nil {
		t.Fatal(err)
	}
	pkts := [][]byte{
		packetBytes(t, 1, 1, map[string]float64{"m_trackLength": 3000}, nil, nil),
		packetBytes(t, 2, 2, nil, map[int]map[string]float64{0: {"m_currentLapNum": 1}}, nil),
		packetBytes(t, 2, 3, nil, map[int]map[string]float64{0: {"m_currentLapNum": 2, "m_lastLapTimeInMS": 90000}}, nil),
	}
	recordForBackfill(t, cat, t.TempDir(), "s", "f", pkts)
	Backfill(ctx, cat, nil, 10)
	laps, err := cat.LapAnalytics(ctx, "s", "b", 0)
	complete := []history.LapAnalytics{}
	for _, l := range laps {
		if l.Complete {
			complete = append(complete, l)
		}
	}
	if err != nil || len(complete) != 1 {
		t.Fatalf("complete laps %#v err %v", complete, err)
	}
	if complete[0].DistanceM != 3000 {
		t.Fatalf("distance = %v, want the packet track length 3000", complete[0].DistanceM)
	}
}

func ptrString(s string) *string { return &s }
