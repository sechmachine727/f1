package analytics

import (
	"context"
	"testing"
)

func TestLapSamplesStoreInputsAndRestriction(t *testing.T) {
	cat, tr, cx := setup(t)
	ctx := context.Background()
	session := map[string]float64{"track_length_m": 5000}
	// Car 0 is the player and shares telemetry; car 1 hides its channels.
	tr.Observe(ctx, 1, packet(4, 1, nil, map[int]map[string]float64{
		0: {"m_teamId": 1, "m_raceNumber": 16, "m_yourTelemetry": 1},
		1: {"m_teamId": 2, "m_raceNumber": 44, "m_yourTelemetry": 0},
	}), cx, session)
	tr.Observe(ctx, 2, packet(7, 2, nil, map[int]map[string]float64{
		0: {"m_ersStoreEnergy": 2000000, "m_ersDeployMode": 3},
		1: {"m_ersStoreEnergy": 1500000, "m_ersDeployMode": 1},
	}), cx, session)
	tr.Observe(ctx, 3, packet(16, 3, nil, map[int]map[string]float64{
		0: {"m_overtakeAvailable": 1, "m_overtakeActive": 0},
	}), cx, session)
	for i := 0; i < 5; i++ {
		tr.Observe(ctx, uint64(10+i), packet(2, uint32(10+i), nil, map[int]map[string]float64{
			0: {"m_currentLapNum": 1, "m_lapDistance": float64(i) * 40, "m_totalDistance": float64(i) * 40},
			1: {"m_currentLapNum": 1, "m_lapDistance": float64(i) * 40, "m_totalDistance": float64(i) * 40},
		}), cx, session)
		tr.Observe(ctx, uint64(30+i), packet(6, uint32(30+i), nil, map[int]map[string]float64{
			0: {"m_speed": 200, "m_throttle": 0.8, "m_brake": 0, "m_gear": 7},
			1: {"m_speed": 190, "m_throttle": 0.6, "m_brake": 0.2, "m_gear": 6},
		}), cx, session)
	}
	tr.Observe(ctx, 50, packet(2, 50, nil, map[int]map[string]float64{
		0: {"m_currentLapNum": 2, "m_lastLapTimeInMS": 90000, "m_lapDistance": 0},
		1: {"m_currentLapNum": 2, "m_lastLapTimeInMS": 90500, "m_lapDistance": 0},
	}), cx, session)

	rows, held, err := cat.SessionLapSamples(ctx, "s", "b", []int{0, 1}, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if held != 2 || len(rows) != 2 {
		t.Fatalf("held %d rows %d", held, len(rows))
	}
	player, rival := rows[0], rows[1]
	if len(player.Points) == 0 || len(rival.Points) == 0 {
		t.Fatalf("missing points %#v", rows)
	}
	p0 := player.Points[0]
	if p0[1].(float64) != 0.8 || p0[2].(float64) != 0 || p0[3].(float64) != 7 {
		t.Fatalf("player inputs %#v", p0)
	}
	if p0[4] == nil || p0[5].(float64) != 3 {
		t.Fatalf("player energy %#v", p0)
	}
	if p0[6] != true || p0[7] != false {
		t.Fatalf("player overtake %#v", p0)
	}
	r0 := rival.Points[0]
	if r0[1].(float64) != 0.6 || r0[3].(float64) != 6 {
		t.Fatalf("rival inputs %#v", r0)
	}
	if r0[4] != nil || r0[5] != nil {
		t.Fatalf("restricted energy must stay null: %#v", r0)
	}
	if player.StartMs <= 0 {
		t.Fatalf("start time %d", player.StartMs)
	}
	if player.LapMs != 90000 || rival.LapMs != 90500 {
		t.Fatalf("lap times %#v", rows)
	}
	if player.CarIndex != 0 || rival.CarIndex != 1 {
		t.Fatalf("order %#v", rows)
	}
	limited, _, err := cat.SessionLapSamples(ctx, "s", "b", []int{1}, 2, 0)
	if err != nil || len(limited) != 0 {
		t.Fatalf("lap range filter: %d %v", len(limited), err)
	}
}

func TestPublicPlayerEnergyStored(t *testing.T) {
	cat, tr, cx := setup(t)
	ctx := context.Background()
	session := map[string]float64{"track_length_m": 5000}
	// The player's own channel is stored even when another participant is public.
	tr.Observe(ctx, 1, packet(4, 1, nil, map[int]map[string]float64{
		0: {"m_teamId": 1, "m_yourTelemetry": 0},
		1: {"m_teamId": 2, "m_yourTelemetry": 1},
	}), cx, session)
	tr.Observe(ctx, 2, packet(7, 2, nil, map[int]map[string]float64{
		0: {"m_ersStoreEnergy": 3000000, "m_ersDeployMode": 2},
		1: {"m_ersStoreEnergy": 1000000, "m_ersDeployMode": 0},
	}), cx, session)
	tr.Observe(ctx, 3, packet(2, 3, nil, map[int]map[string]float64{
		0: {"m_currentLapNum": 1, "m_lapDistance": 40, "m_totalDistance": 40},
		1: {"m_currentLapNum": 1, "m_lapDistance": 40, "m_totalDistance": 40},
	}), cx, session)
	// The first lap packet records the baseline distance; the second sets it.
	tr.Observe(ctx, 31, packet(2, 31, nil, map[int]map[string]float64{
		0: {"m_currentLapNum": 1, "m_lapDistance": 60, "m_totalDistance": 60},
		1: {"m_currentLapNum": 1, "m_lapDistance": 60, "m_totalDistance": 60},
	}), cx, session)
	tr.Observe(ctx, 4, packet(6, 4, nil, map[int]map[string]float64{
		0: {"m_speed": 100, "m_throttle": 1, "m_brake": 0, "m_gear": 3},
		1: {"m_speed": 100, "m_throttle": 1, "m_brake": 0, "m_gear": 3},
	}), cx, session)
	tr.Observe(ctx, 5, packet(2, 5, nil, map[int]map[string]float64{
		0: {"m_currentLapNum": 2, "m_lastLapTimeInMS": 60000, "m_lapDistance": 0},
		1: {"m_currentLapNum": 2, "m_lastLapTimeInMS": 60000, "m_lapDistance": 0},
	}), cx, session)
	rows, _, err := cat.SessionLapSamples(ctx, "s", "b", []int{0, 1}, 0, 0)
	if err != nil || len(rows) != 2 {
		t.Fatalf("rows %d err %v", len(rows), err)
	}
	if rows[0].Points[0][4] == nil {
		t.Fatalf("player energy hidden: %#v", rows[0].Points[0])
	}
	if rows[1].Points[0][4] == nil {
		t.Fatalf("public rival energy hidden: %#v", rows[1].Points[0])
	}
}

