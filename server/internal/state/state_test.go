package state

import (
	"math"
	"testing"
	"time"

	"f1tele/server/internal/protocol"
)

type fixedClock struct{ t time.Time }

func (c *fixedClock) now() time.Time { return c.t }

func hdr(uid uint64, frame, overall uint32, sessionTime float64) protocol.Header {
	return protocol.Header{Format: 2026, GameYear: 26, PacketVersion: 1, SessionUID: uid, SessionTime: sessionTime, FrameID: frame, OverallFrameID: overall, PlayerCarIndex: 0}
}

func close(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func feed(r *Reducer, seq uint64, c *fixedClock, data []byte) {
	p, err := protocol.Decode(data)
	if err != nil {
		panic(err)
	}
	r.Apply(seq, c.t, data, p, "")
}

func telemetryPacket(h protocol.Header, speed float64) []byte {
	return protocol.BuildPacket(2026, 6, h, nil, map[int]map[string]float64{0: {"m_speed": speed, "m_gear": 7}}, nil)
}

func TestNewerPerKeyAndLatePacketType(t *testing.T) {
	c := &fixedClock{time.UnixMilli(1_000_000)}
	r := New("ctx", c.now)
	feed(r, 1, c, telemetryPacket(hdr(7, 10, 100, 10), 300))
	status := protocol.BuildPacket(2026, 7, hdr(7, 5, 95, 9.9), map[string]float64{"m_fuelInTank": 42}, map[int]map[string]float64{0: {"m_fuelInTank": 42}}, nil)
	feed(r, 2, c, status)
	if v, ok := r.Value("fuel.mass_kg", 0); !ok || v != 42 {
		t.Fatalf("late lower packet type rejected: %v %v", v, ok)
	}
	// An older frame for the same packet key must not regress the value.
	feed(r, 3, c, telemetryPacket(hdr(7, 2, 90, 9.5), 100))
	if v, _ := r.Value("car.speed_mps", 0); !close(v, 300.0/3.6) {
		t.Fatalf("speed regressed to %v", v)
	}
}

func TestSameFrameDistinctEventsSurvive(t *testing.T) {
	c := &fixedClock{time.UnixMilli(1_000_000)}
	r := New("ctx", c.now)
	seat := protocol.BuildPacket(2026, 3, hdr(9, 1, 50, 5), nil, nil, map[string]string{"m_eventStringCode": "SCAR"})
	seat[33], seat[34] = 1, 0
	feed(r, 1, c, seat)
	red := protocol.BuildPacket(2026, 3, hdr(9, 1, 50, 5), nil, nil, map[string]string{"m_eventStringCode": "RDFL"})
	feed(r, 2, c, red)
	feed(r, 3, c, seat) // exact duplicate
	evs := r.Events()
	if len(evs) != 2 {
		t.Fatalf("events = %d, want 2: %#v", len(evs), evs)
	}
	if evs[0].Type != "safety_car" || evs[1].Type != "red_flag" {
		t.Fatalf("unexpected events %#v", evs)
	}
}

func TestFrameWrapAndStaleness(t *testing.T) {
	c := &fixedClock{time.UnixMilli(1_000_000)}
	r := New("ctx", c.now)
	feed(r, 1, c, telemetryPacket(hdr(3, 0xFFFFFFF0, 0xFFFFFFF0, 1), 200))
	c.t = c.t.Add(500 * time.Millisecond)
	feed(r, 2, c, telemetryPacket(hdr(3, 0x00000010, 0x00000010, 2), 210))
	if v, _ := r.Value("car.speed_mps", 0); !close(v, 210.0/3.6) {
		t.Fatalf("wrapped frame rejected: %v", v)
	}
	c.t = c.t.Add(1500 * time.Millisecond)
	snap := r.Snapshot([]int{0}, []string{"car.speed_mps"})
	if snap.Metrics[0].Measurement.Availability != "stale" {
		t.Fatalf("stale = %s", snap.Metrics[0].Measurement.Availability)
	}
}

func TestSameUIDRewindCreatesBranchKeepsSession(t *testing.T) {
	c := &fixedClock{time.UnixMilli(1_000_000)}
	r := New("ctx", c.now)
	feed(r, 1, c, protocol.BuildPacket(2026, 1, hdr(5, 1, 10, 100), map[string]float64{"m_trackId": 3}, nil, nil))
	feed(r, 2, c, telemetryPacket(hdr(5, 2, 20, 101), 250))
	ses := r.SessionID()
	branch := r.BranchID()
	c.t = c.t.Add(time.Second)
	feed(r, 3, c, protocol.BuildPacket(2026, 3, hdr(5, 3, 21, 60), nil, nil, map[string]string{"m_eventStringCode": "FLBK"}))
	if r.SessionID() != ses {
		t.Fatal("session replaced on rewind")
	}
	if r.BranchID() == branch {
		t.Fatal("no branch on flashback")
	}
	if len(r.Events()) != 1 {
		t.Fatalf("logs cleared on rewind: %#v", r.Events())
	}
	// Follow-up datagrams in the rewound timeline must not create branches again.
	feed(r, 4, c, telemetryPacket(hdr(5, 4, 22, 61), 240))
	if got := len(r.LastBranch()); got != 2 {
		t.Fatalf("branches = %d want 2", got)
	}
}

func TestNewUIDStartsFragment(t *testing.T) {
	c := &fixedClock{time.UnixMilli(1_000_000)}
	r := New("ctx", c.now)
	feed(r, 1, c, telemetryPacket(hdr(1, 1, 1, 1), 100))
	ses, frag := r.SessionID(), r.Context().FragmentID
	feed(r, 2, c, telemetryPacket(hdr(2, 2, 2, 1), 100))
	if r.SessionID() == ses || r.Context().FragmentID == frag {
		t.Fatal("new UID did not start a fragment")
	}
}

func TestRestrictedRivalIsNullNotZero(t *testing.T) {
	c := &fixedClock{time.UnixMilli(1_000_000)}
	r := New("ctx", c.now)
	parts := map[int]map[string]float64{0: {"m_yourTelemetry": 1}, 1: {"m_yourTelemetry": 0}}
	feed(r, 1, c, protocol.BuildPacket(2026, 4, hdr(1, 1, 1, 1), map[string]float64{"m_numActiveCars": 2}, parts, nil))
	feed(r, 2, c, protocol.BuildPacket(2026, 7, hdr(1, 2, 2, 2), nil, map[int]map[string]float64{0: {"m_fuelInTank": 50}, 1: {"m_fuelInTank": 0, "m_ersStoreEnergy": 0}}, nil))
	snap := r.Snapshot([]int{0, 1}, []string{"fuel.mass_kg", "energy.store_j"})
	byID := map[string]string{}
	for _, m := range snap.Metrics {
		byID[m.ID] = m.Measurement.Availability
	}
	if byID["fuel.mass_kg@0"] != "available" || byID["fuel.mass_kg@1"] != "restricted" {
		t.Fatalf("fuel availability %#v", byID)
	}
	if byID["energy.store_j@1"] != "restricted" {
		t.Fatalf("energy availability %#v", byID)
	}
}

func TestZeroSpeedRemainsAvailable(t *testing.T) {
	c := &fixedClock{time.UnixMilli(1_000_000)}
	r := New("ctx", c.now)
	feed(r, 1, c, telemetryPacket(hdr(1, 1, 1, 1), 0))
	snap := r.Snapshot([]int{0}, []string{"car.speed_mps"})
	if snap.Metrics[0].Measurement.Availability != "available" || snap.Metrics[0].Measurement.Value != float64(0) {
		t.Fatalf("zero speed = %#v", snap.Metrics[0].Measurement)
	}
}

func TestSeasonEvidenceConflicts(t *testing.T) {
	c := &fixedClock{time.UnixMilli(1_000_000)}
	r := New("ctx", c.now)
	// format 2026 with formula 0 and applies_2026 false -> 2025 with supported context
	ses := protocol.BuildPacket(2026, 1, hdr(1, 1, 1, 1), map[string]float64{"m_formula": 0}, nil, nil)
	feed(r, 1, c, ses)
	feed(r, 2, c, protocol.BuildPacket(2026, 16, hdr(1, 2, 2, 2), nil, map[int]map[string]float64{0: {"m_2026Regulations": 0}}, nil))
	if got := r.Context().RegulationSeason; got == nil || *got != 2025 {
		t.Fatalf("season = %v want 2025", got)
	}
	feed(r, 3, c, protocol.BuildPacket(2026, 1, hdr(1, 3, 3, 3), map[string]float64{"m_formula": 13}, nil, nil))
	if got := r.Context().RegulationSeason; got != nil {
		t.Fatalf("conflicting season = %v, want unknown", *got)
	}
	if len(r.Context().SeasonEvidence) == 0 {
		t.Fatal("conflict evidence not retained")
	}
}

func TestSameFrameDifferentPayloadNotSuppressed(t *testing.T) {
	c := &fixedClock{time.UnixMilli(1_000_000)}
	r := New("ctx", c.now)
	feed(r, 1, c, telemetryPacket(hdr(1, 1, 1, 1), 100))
	feed(r, 2, c, telemetryPacket(hdr(1, 1, 1, 1), 150))
	// Same packet/car key and frame keeps the first sample.
	if v, _ := r.Value("car.speed_mps", 0); !close(v, 100.0/3.6) {
		t.Fatalf("same-frame key changed: %v", v)
	}
}
