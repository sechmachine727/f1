package state

import (
	"testing"
	"time"

	"f1tele/server/internal/protocol"
)

func TestInactiveCarSlotsStayUnavailable(t *testing.T) {
	c := &fixedClock{time.UnixMilli(1_000_000)}
	r := New("ctx", c.now)
	speed := protocol.BuildPacket(2026, 6, hdr(21, 1, 1, 1), nil, map[int]map[string]float64{
		0: {"m_speed": 200}, 1: {"m_speed": 100},
	}, nil)
	feed(r, 1, c, speed)
	parts := map[int]map[string]float64{0: {"m_teamId": 1, "m_raceNumber": 16}, 1: {"m_teamId": 2}}
	feed(r, 2, c, protocol.BuildPacket(2026, 4, hdr(21, 2, 2, 2), map[string]float64{"m_numActiveCars": 1}, parts, nil))
	// A later 24-slot packet must not widen the active field back to 24.
	feed(r, 3, c, speed)

	snap := r.Snapshot([]int{1}, []string{"car.speed_mps"})
	if got := snap.Metrics[0].Measurement.Availability; got == "available" {
		t.Fatalf("inactive car reported available: %#v", snap.Metrics[0].Measurement)
	}
	snap = r.Snapshot([]int{0}, []string{"car.speed_mps"})
	if snap.Metrics[0].Measurement.Availability != "available" {
		t.Fatalf("player car unavailable: %#v", snap.Metrics[0].Measurement)
	}
}

func TestPauseFreezesLastValues(t *testing.T) {
	c := &fixedClock{time.UnixMilli(1_000_000)}
	r := New("ctx", c.now)
	feed(r, 1, c, protocol.BuildPacket(2026, 1, hdr(31, 1, 1, 1), map[string]float64{"m_gamePaused": 1}, nil, nil))
	feed(r, 2, c, telemetryPacket(hdr(31, 2, 2, 2), 300))
	c.t = c.t.Add(5 * time.Second)
	snap := r.Snapshot([]int{0}, []string{"car.speed_mps", "session.paused"})
	if snap.Metrics[0].Measurement.Availability != "available" {
		t.Fatalf("paused value blanked: %#v", snap.Metrics[0].Measurement)
	}
	if snap.Metrics[1].Measurement.Value != true {
		t.Fatalf("paused flag = %#v", snap.Metrics[1].Measurement)
	}

	feed(r, 3, c, protocol.BuildPacket(2026, 1, hdr(31, 3, 3, 3), map[string]float64{"m_gamePaused": 0}, nil, nil))
	snap = r.Snapshot([]int{0}, []string{"car.speed_mps"})
	if snap.Metrics[0].Measurement.Availability != "stale" {
		t.Fatalf("after unpause = %s", snap.Metrics[0].Measurement.Availability)
	}
}
