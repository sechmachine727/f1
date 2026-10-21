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

func TestFlashbackClearsTyreAccumulators(t *testing.T) {
	cat, tr, cx := setup(t)
	ctx := context.Background()
	session := map[string]float64{"track_length_m": 5000}
	wear := func(v float64) *protocol.Packet {
		return packet(10, 2, nil, map[int]map[string]float64{0: {
			"m_tyresWear[0]": v, "m_tyresWear[1]": v, "m_tyresWear[2]": v, "m_tyresWear[3]": v,
		}})
	}
	temps := func(surface, core float64) *protocol.Packet {
		return packet(6, 3, nil, map[int]map[string]float64{0: {
			"m_speed": 200, "m_throttle": 0.8, "m_brake": 0, "m_gear": 7,
			"m_tyresSurfaceTemperature[0]": surface, "m_tyresSurfaceTemperature[1]": surface,
			"m_tyresSurfaceTemperature[2]": surface, "m_tyresSurfaceTemperature[3]": surface,
			"m_tyresInnerTemperature[0]": core, "m_tyresInnerTemperature[1]": core,
			"m_tyresInnerTemperature[2]": core, "m_tyresInnerTemperature[3]": core,
		}})
	}
	start := func() *protocol.Packet {
		return packet(2, 1, nil, map[int]map[string]float64{0: {"m_currentLapNum": 1, "m_lapDistance": 100, "m_totalDistance": 100}})
	}
	tr.Observe(ctx, 1, start(), cx, session)
	tr.Observe(ctx, 2, wear(10), cx, session)
	tr.Observe(ctx, 3, temps(80, 90), cx, session)
	tr.resetFlashback()
	tr.Observe(ctx, 4, start(), cx, session)
	tr.Observe(ctx, 5, wear(20), cx, session)
	tr.Observe(ctx, 6, temps(100, 110), cx, session)
	if st := tr.car(0); st.wearN != 4 || st.surfaceN != 4 || st.coreN != 4 {
		t.Fatalf("tyre samples wear=%d surface=%d core=%d", st.wearN, st.surfaceN, st.coreN)
	}
	tr.Observe(ctx, 7, packet(2, 7, nil, map[int]map[string]float64{0: {"m_currentLapNum": 2, "m_lastLapTimeInMS": 90000, "m_lapDistance": 0}}), cx, session)
	laps, err := cat.LapAnalytics(ctx, "s", "b", 0)
	if err != nil || len(laps) != 1 {
		t.Fatalf("laps %d err %v", len(laps), err)
	}
	if laps[0].TyreWearPct != 20 || laps[0].TyreSurfaceC != 100 || laps[0].TyreCoreC != 110 {
		t.Fatalf("flashback tyre averages %#v", laps[0])
	}
}

func TestCloseStoresIncompleteLap(t *testing.T) {
	cat, tr, cx := setup(t)
	ctx := context.Background()
	session := map[string]float64{"track_length_m": 5000}
	tr.Observe(ctx, 1, packet(2, 1, nil, map[int]map[string]float64{0: {"m_currentLapNum": 1, "m_lapDistance": 500, "m_totalDistance": 500}}), cx, session)
	tr.Observe(ctx, 2, packet(6, 2, nil, map[int]map[string]float64{0: {"m_speed": 200, "m_throttle": 0.8, "m_brake": 0, "m_gear": 7}}), cx, session)
	if err := tr.Close(ctx); err != nil {
		t.Fatal(err)
	}
	laps, err := cat.LapAnalytics(ctx, "s", "b", 0)
	if err != nil || len(laps) != 1 {
		t.Fatalf("partial laps %d err %v", len(laps), err)
	}
	if laps[0].Lap != 1 || laps[0].Complete {
		t.Fatalf("partial lap %#v", laps[0])
	}
}

func TestCloseThenResumeReopensTracker(t *testing.T) {
	cat, tr, cx := setup(t)
	ctx := context.Background()
	session := map[string]float64{"track_length_m": 5000}
	observe := func(seq uint64, lap int, distance, lastMs float64) {
		tr.Observe(ctx, seq, packet(2, uint32(seq), nil, map[int]map[string]float64{0: {
			"m_currentLapNum": float64(lap), "m_lastLapTimeInMS": lastMs,
			"m_lapDistance": distance, "m_totalDistance": distance,
		}}), cx, session)
	}
	observe(1, 1, 500, 0)
	if err := tr.Close(ctx); err != nil {
		t.Fatal(err)
	}
	// Resume on the same lap and close again; the partial row must refresh.
	observe(2, 1, 800, 0)
	if err := tr.Close(ctx); err != nil {
		t.Fatal(err)
	}
	laps, err := cat.LapAnalytics(ctx, "s", "b", 0)
	if err != nil {
		t.Fatal(err)
	}
	var partial *history.LapAnalytics
	for _, l := range laps {
		if l.Lap == 1 && !l.Complete {
			copy := l
			partial = &copy
		}
	}
	if partial == nil || partial.DistanceM != 800 {
		t.Fatalf("partial lap after resume = %#v", partial)
	}
	// Resume again, finish the lap, and close; the second close must rebuild
	// stints for the resumed segment.
	observe(3, 1, 900, 0)
	observe(4, 2, 0, 90000)
	if err := tr.Close(ctx); err != nil {
		t.Fatal(err)
	}
	laps, err = cat.LapAnalytics(ctx, "s", "b", 0)
	if err != nil {
		t.Fatal(err)
	}
	complete := 0
	for _, l := range laps {
		if l.Lap == 1 && l.Complete {
			complete++
		}
	}
	if complete != 1 {
		t.Fatalf("complete lap after final close = %d (%#v)", complete, laps)
	}
	stints, err := cat.Stints(ctx, "s", "b")
	if err != nil || len(stints) == 0 {
		t.Fatalf("stints after final close %#v err %v", stints, err)
	}
}

func TestCloseKeepsZeroPartialDistance(t *testing.T) {
	cat, tr, cx := setup(t)
	ctx := context.Background()
	session := map[string]float64{"track_length_m": 5000}
	tr.Observe(ctx, 1, packet(2, 1, nil, map[int]map[string]float64{0: {"m_currentLapNum": 1}}), cx, session)
	if err := tr.Close(ctx); err != nil {
		t.Fatal(err)
	}
	laps, err := cat.LapAnalytics(ctx, "s", "b", 0)
	if err != nil || len(laps) != 1 {
		t.Fatalf("partial laps %d err %v", len(laps), err)
	}
	if laps[0].DistanceM != 0 {
		t.Fatalf("partial distance = %v, want the observed zero", laps[0].DistanceM)
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

func TestDamageDoesNotSetTrackLimits(t *testing.T) {
	cat, tr, cx := setup(t)
	ctx := context.Background()
	session := map[string]float64{"track_length_m": 5000}
	damage := packet(10, 1, nil, map[int]map[string]float64{0: {
		"m_frontLeftWingDamage": 100, "m_frontRightWingDamage": 100, "m_rearWingDamage": 100,
		"m_floorDamage": 100, "m_diffuserDamage": 100, "m_sidepodDamage": 100,
	}})
	tr.Observe(ctx, 1, damage, cx, session)
	tr.Observe(ctx, 2, packet(2, 2, nil, map[int]map[string]float64{0: {
		"m_currentLapNum": 1, "m_lapDistance": 100, "m_totalDistance": 100,
	}}), cx, session)
	tr.Observe(ctx, 3, packet(2, 3, nil, map[int]map[string]float64{0: {
		"m_currentLapNum": 2, "m_lastLapTimeInMS": 90000, "m_lapDistance": 0, "m_totalDistance": 5000,
	}}), cx, session)
	laps, err := cat.LapAnalytics(ctx, "s", "b", 0)
	if err != nil || len(laps) != 1 {
		t.Fatalf("laps %#v err %v", laps, err)
	}
	if laps[0].TrackLimits {
		t.Fatal("body damage was reported as a track-limit violation")
	}
}

func TestPitOutLapAndTrapSpeed(t *testing.T) {
	cat, tr, cx := setup(t)
	ctx := context.Background()
	session := map[string]float64{"track_length_m": 5000}
	tr.Observe(ctx, 1, packet(2, 1, nil, map[int]map[string]float64{0: {
		"m_currentLapNum": 1, "m_pitStatus": 0, "m_lapDistance": 100, "m_totalDistance": 100,
		"m_speedTrapFastestSpeed": 88, "m_speedTrapFastestLap": 1,
	}}), cx, session)
	tr.Observe(ctx, 2, packet(2, 2, nil, map[int]map[string]float64{0: {
		"m_currentLapNum": 1, "m_pitStatus": 1, "m_lapDistance": 120, "m_totalDistance": 120,
	}}), cx, session)
	tr.Observe(ctx, 3, packet(2, 3, nil, map[int]map[string]float64{0: {
		"m_currentLapNum": 2, "m_lastLapTimeInMS": 90000, "m_pitStatus": 1, "m_lapDistance": 0, "m_totalDistance": 5000,
	}}), cx, session)
	tr.Observe(ctx, 4, packet(2, 4, nil, map[int]map[string]float64{0: {
		"m_currentLapNum": 2, "m_pitStatus": 0, "m_lapDistance": 20, "m_totalDistance": 5020,
	}}), cx, session)
	tr.Observe(ctx, 5, packet(2, 5, nil, map[int]map[string]float64{0: {
		"m_currentLapNum": 3, "m_lastLapTimeInMS": 91000, "m_lapDistance": 0, "m_totalDistance": 6000,
	}}), cx, session)
	laps, err := cat.LapAnalytics(ctx, "s", "b", 0)
	if err != nil || len(laps) != 2 {
		t.Fatalf("laps %#v err %v", laps, err)
	}
	if !laps[0].PitIn || laps[0].PitOut {
		t.Fatalf("pit-in lap %#v", laps[0])
	}
	if laps[0].TrapSpeed < 24.4 || laps[0].TrapSpeed > 24.5 {
		t.Fatalf("trap speed = %v", laps[0].TrapSpeed)
	}
	if !laps[1].PitOut {
		t.Fatalf("pit-out lap was not flagged: %#v", laps[1])
	}
}

func TestCornerCuttingWarningSetsTrackLimits(t *testing.T) {
	cat, tr, cx := setup(t)
	ctx := context.Background()
	session := map[string]float64{"track_length_m": 5000}
	tr.Observe(ctx, 1, packet(2, 1, nil, map[int]map[string]float64{0: {
		"m_currentLapNum": 1, "m_cornerCuttingWarnings": 0, "m_lapDistance": 10, "m_totalDistance": 10,
	}}), cx, session)
	tr.Observe(ctx, 2, packet(2, 2, nil, map[int]map[string]float64{0: {
		"m_currentLapNum": 1, "m_cornerCuttingWarnings": 1, "m_lapDistance": 20, "m_totalDistance": 20,
	}}), cx, session)
	tr.Observe(ctx, 3, packet(2, 3, nil, map[int]map[string]float64{0: {
		"m_currentLapNum": 2, "m_lastLapTimeInMS": 90000,
	}}), cx, session)
	laps, err := cat.LapAnalytics(ctx, "s", "b", 0)
	if err != nil || len(laps) != 1 {
		t.Fatalf("laps %#v err %v", laps, err)
	}
	if !laps[0].TrackLimits {
		t.Fatal("corner-cutting warning did not set track limits")
	}
}
