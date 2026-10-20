package analytics

import (
	"context"
	"path/filepath"
	"testing"

	"f1tele/server/internal/contract"
	"f1tele/server/internal/history"
	"f1tele/server/internal/protocol"
)

func setup(t *testing.T) (*history.Catalog, *Tracker, contract.Context) {
	t.Helper()
	cat, err := history.OpenCatalog(filepath.Join(t.TempDir(), "catalog.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cat.Close() })
	ctx := context.Background()
	if err := cat.EnsureSourceContext(ctx, "src", "ip"); err != nil {
		t.Fatal(err)
	}
	track := 3
	car := 0
	cx := contract.Context{SourceContextID: "src", SessionID: "s", BranchID: "b", TrackID: &track, PlayerCarIndex: &car, ModeFamily: "race"}
	if err := cat.CreateSession(ctx, "s", cx); err != nil {
		t.Fatal(err)
	}
	if err := cat.CreateBranch(ctx, contract.Branch{ID: "b", SessionID: "s", IsSurviving: true}); err != nil {
		t.Fatal(err)
	}
	return cat, NewTracker(cat), cx
}

func packet(pid uint8, frame uint32, scalar map[string]float64, cars map[int]map[string]float64) *protocol.Packet {
	data := protocol.BuildPacket(2026, pid, protocol.Header{Format: 2026, PacketVersion: 1, SessionUID: 9, FrameID: frame, OverallFrameID: frame, SessionTime: float64(frame)}, scalar, cars, nil)
	p, err := protocol.Decode(data)
	if err != nil {
		panic(err)
	}
	return p
}

func TestLapAnalyticsFromLapBoundaryAndHistory(t *testing.T) {
	cat, tr, cx := setup(t)
	ctx := context.Background()
	session := map[string]float64{"track_length_m": 5000}
	for i := 0; i < 5; i++ {
		p := packet(2, uint32(i+1), nil, map[int]map[string]float64{0: {
			"m_currentLapNum": 1, "m_carPosition": 3, "m_currentLapInvalid": 0, "m_pitStatus": 0,
			"m_totalDistance": float64(i) * 30, "m_lapDistance": float64(i) * 30, "m_speed": 180,
		}})
		tr.Observe(ctx, uint64(i+1), p, cx, session)
	}
	// Lap 2 begins; the previous lap is finalized with its last time.
	p := packet(2, 6, nil, map[int]map[string]float64{0: {
		"m_currentLapNum": 2, "m_lastLapTimeInMS": 90000, "m_totalDistance": 5000, "m_lapDistance": 0,
		"m_speed": 100, "m_carPosition": 2,
	}})
	tr.Observe(ctx, 6, p, cx, session)
	laps, err := cat.LapAnalytics(ctx, "s", "b", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(laps) != 1 || laps[0].Lap != 1 || laps[0].LapMs != 90000 {
		t.Fatalf("laps %#v", laps)
	}
	if laps[0].PositionStart != 3 || laps[0].PositionEnd != 3 {
		t.Fatalf("positions %d -> %d", laps[0].PositionStart, laps[0].PositionEnd)
	}
	if laps[0].DistanceM <= 0 || laps[0].AvgSpeed <= 0 {
		t.Fatalf("distance/avg %#v", laps[0])
	}
	// Packet 11 fills all three sectors for the completed lap.
	hist := map[int]map[string]float64{0: {
		"m_lapTimeInMS": 90000, "m_sector1TimeMSPart": 2345, "m_sector1TimeMinutesPart": 0,
		"m_sector2TimeMSPart": 30123, "m_sector2TimeMinutesPart": 0,
		"m_sector3TimeMSPart": 1245, "m_sector3TimeMinutesPart": 1, "m_lapValidBitFlags": 1,
	}}
	h := packet(11, 7, map[string]float64{"m_carIdx": 0, "m_numLaps": 1}, hist)
	tr.Observe(ctx, 7, h, cx, session)
	laps, _ = cat.LapAnalytics(ctx, "s", "b", 0)
	if laps[0].Sector1Ms != 2345 || laps[0].Sector2Ms != 30123 || laps[0].Sector3Ms != 61245 {
		t.Fatalf("sectors %#v", laps[0])
	}
	if err := tr.Close(ctx); err != nil {
		t.Fatal(err)
	}
	var stints int
	if err := cat.DB().QueryRow("SELECT COUNT(*) FROM analytics_stints WHERE session_id='s'").Scan(&stints); err != nil || stints == 0 {
		t.Fatalf("stints %d err %v", stints, err)
	}
	var best int64
	if err := cat.DB().QueryRow("SELECT best_lap_ms FROM session_analytics WHERE session_id='s'").Scan(&best); err != nil || best != 90000 {
		t.Fatalf("session best %d err %v", best, err)
	}
}

func TestParticipantsAndTraces(t *testing.T) {
	cat, tr, cx := setup(t)
	ctx := context.Background()
	session := map[string]float64{"track_length_m": 5412}
	p4 := packet(4, 1, map[string]float64{"m_numActiveCars": 2}, map[int]map[string]float64{
		0: {"m_teamId": 1, "m_raceNumber": 16, "m_aiControlled": 0},
		1: {"m_teamId": 2, "m_raceNumber": 44, "m_aiControlled": 1},
	})
	// Names are strings in the participant segment.
	data := protocol.BuildPacket(2026, 4, protocol.Header{Format: 2026, PacketVersion: 1, SessionUID: 9}, map[string]float64{"m_numActiveCars": 2}, map[int]map[string]float64{
		0: {"m_teamId": 1, "m_raceNumber": 16}, 1: {"m_teamId": 2, "m_raceNumber": 44},
	}, map[string]string{"m_name[0]": "LECLERC", "m_name[1]": "HAMILTON"})
	p, err := protocol.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	tr.Observe(ctx, 1, p, cx, session)
	_ = p4
	people, err := cat.Participants(ctx, "s")
	if err != nil {
		t.Fatal(err)
	}
	if len(people) != 2 || people[0].Name != "LECLERC" || people[0].Number != 16 || people[1].TeamID != 2 {
		t.Fatalf("participants %#v", people)
	}
	// A lap with enough samples produces a decimated trace.
	for i := 0; i < 50; i++ {
		p := packet(2, uint32(10+i), nil, map[int]map[string]float64{0: {
			"m_currentLapNum": 1, "m_totalDistance": float64(i) * 50, "m_lapDistance": float64(i) * 50, "m_speed": 200,
		}})
		tr.Observe(ctx, uint64(10+i), p, cx, session)
		s := packet(6, uint32(10+i), nil, map[int]map[string]float64{0: {"m_speed": 200}})
		tr.Observe(ctx, uint64(200+i), s, cx, session)
	}
	boundary := packet(2, 100, nil, map[int]map[string]float64{0: {"m_currentLapNum": 2, "m_lastLapTimeInMS": 80000, "m_totalDistance": 2500, "m_lapDistance": 0, "m_speed": 50}})
	tr.Observe(ctx, 100, boundary, cx, session)
	var points int
	if err := cat.DB().QueryRow("SELECT point_count FROM lap_traces WHERE session_id='s' AND car_idx=0 AND lap=1").Scan(&points); err != nil || points == 0 {
		t.Fatalf("trace points %d err %v", points, err)
	}
	if points > 300 {
		t.Fatalf("trace not decimated: %d", points)
	}
}

func TestProvisionalCornerSpeeds(t *testing.T) {
	_, tr, _ := setup(t)
	tr.Begin("s", "b", 3, 0, 5000)
	zones := zonesFor(3)
	if len(zones) == 0 {
		t.Fatal("no provisional zones for Bahrain")
	}
	samples := []sample{{distance: zones[0].Norm * 5000, speed: 40}, {distance: zones[0].Norm*5000 + 10, speed: 30}}
	got := tr.cornerSpeeds(samples)
	if m, ok := got["T1"]; !ok || m.Availability != "available" || m.Basis != "derived" {
		t.Fatalf("corner speeds %#v", got)
	}
	if tr.cornerSpeeds(nil) == nil {
		t.Fatal("empty samples must yield an empty map, not a panic")
	}
}

func TestNewSessionClosesPreviousAnalytics(t *testing.T) {
	cat, tr, cx := setup(t)
	ctx := context.Background()
	session := map[string]float64{"track_length_m": 5000}
	for i := 0; i < 3; i++ {
		p := packet(2, uint32(i+1), nil, map[int]map[string]float64{0: {"m_currentLapNum": 1, "m_totalDistance": float64(i) * 40, "m_lapDistance": float64(i) * 40}})
		tr.Observe(ctx, uint64(i+1), p, cx, session)
	}
	b := packet(2, 10, nil, map[int]map[string]float64{0: {"m_currentLapNum": 2, "m_lastLapTimeInMS": 90000, "m_totalDistance": 120, "m_lapDistance": 0}})
	tr.Observe(ctx, 10, b, cx, session)
	next := cx
	next.SessionID = "s2"
	next.BranchID = "b2"
	tr.Observe(ctx, 11, packet(2, 11, nil, map[int]map[string]float64{0: {"m_currentLapNum": 1}}), next, session)
	var n int
	if err := cat.DB().QueryRow("SELECT COUNT(*) FROM session_analytics WHERE session_id='s'").Scan(&n); err != nil || n != 1 {
		t.Fatalf("previous session was not closed: %d %v", n, err)
	}
}
