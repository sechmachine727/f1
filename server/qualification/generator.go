// Package qualification contains the rated workload generator and oracle.
package qualification

import (
	"crypto/sha256"
	"encoding/binary"
	"hash"
	"math"
	"sync"

	"f1tele/server/internal/protocol"
)

// Slot rates come from docs/rework/contracts/workload.json.
var baseRates = map[uint8]int{
	0: 60, 1: 20, 2: 60, 3: 40, 4: 10, 5: 10, 6: 60, 7: 60,
	10: 20, 11: 960, 12: 480, 13: 60, 15: 20, 16: 60,
}

// Corpus builds the fixed 2026 stress mix. It is deterministic for a seed and
// second, and never falls back to short zero-filled datagrams.
type Corpus struct {
	Seed       int64
	SessionUID uint64
	Player     uint8
	// RewindAt returns the session-time offset (seconds) applied at a slot.
	RewindAt func(second int) float64
}

// BuildSlot returns the datagram transmitted at a global slot inside one second.
func (c *Corpus) BuildSlot(second, slot int) []byte {
	base := 0
	for _, pid := range []uint8{0, 1, 2, 3, 4, 5, 6, 7, 10, 11, 12, 13, 15, 16} {
		n := baseRates[pid]
		if slot < base+n {
			return c.build(second, slot, pid, slot-base)
		}
		base += n
	}
	// Duplicates copy base indices 0..127 byte-for-byte.
	idx := slot - base
	if idx < 0 {
		idx = 0
	}
	if idx > 127 {
		idx = 127
	}
	return c.build(second, idx, c.idAt(idx), c.ordAt(idx))
}

func (c *Corpus) idAt(baseIdx int) uint8 {
	pos := 0
	for _, pid := range []uint8{0, 1, 2, 3, 4, 5, 6, 7, 10, 11, 12, 13, 15, 16} {
		n := baseRates[pid]
		if baseIdx < pos+n {
			return pid
		}
		pos += n
	}
	return 0
}

func (c *Corpus) ordAt(baseIdx int) int {
	pos := 0
	for _, pid := range []uint8{0, 1, 2, 3, 4, 5, 6, 7, 10, 11, 12, 13, 15, 16} {
		n := baseRates[pid]
		if baseIdx < pos+n {
			return baseIdx - pos
		}
		pos += n
	}
	return 0
}

func (c *Corpus) build(second, slot int, pid uint8, ord int) []byte {
	global := second*2048 + slot
	overall := uint32(global * 60 / 2048)
	sessionTime := float64(overall)/60.0 - c.rewind(second)
	h := protocol.Header{
		Format: 2026, GameYear: 26, MajorVersion: 1, MinorVersion: 23, PacketVersion: 1,
		PacketID: pid, SessionUID: c.SessionUID, SessionTime: sessionTime,
		FrameID: overall, OverallFrameID: overall, PlayerCarIndex: c.Player, SecondaryIndex: 255,
	}
	cars := map[int]map[string]float64{}
	scalar := map[string]float64{}
	strings := map[string]string{}
	lap := 1 + second/90
	speed := 180 + float64((int64(second)+carsSeed(c.Seed)*7)%100)
	switch pid {
	case 0, 5, 6, 7, 10, 11, 12, 13, 15, 16:
		for car := 0; car < 24; car++ {
			cars[car] = map[string]float64{}
		}
	}
	switch pid {
	case 0:
		for car := range cars {
			cars[car]["m_worldPositionX"] = float64(car)*7.5 + float64(second)
			cars[car]["m_gForceLateral"] = 1500
		}
	case 1:
		scalar["m_weather"] = 0
		scalar["m_trackTemperature"] = 40
		scalar["m_airTemperature"] = 25
		scalar["m_totalLaps"] = 57
		scalar["m_trackLength"] = 5412
		scalar["m_sessionType"] = 10
		scalar["m_trackId"] = 3
		scalar["m_formula"] = 13
		scalar["m_networkGame"] = 1
		scalar["m_gameMode"] = 7
		scalar["m_sessionTimeLeft"] = 1800
		if second >= 1800 && second < 1810 {
			scalar["m_safetyCarStatus"] = 1
		}
	case 2:
		for car := 0; car < 24; car++ {
			cars[car] = map[string]float64{
				"m_currentLapNum": float64(lap), "m_carPosition": float64(car + 1),
				"m_currentLapTimeInMS": float64((global % 90000)), "m_lastLapTimeInMS": 90000 + float64(car*10),
				"m_lapDistance":           math.Mod(float64(second*5000)/90, 5000),
				"m_speedTrapFastestSpeed": 330, "m_currentLapInvalid": 0, "m_pitStatus": 0,
			}
		}
	case 3:
		butn := []float64{0, 1, 0, 2}[ord%4]
		buf := protocol.BuildPacket(2026, 3, h, nil, nil, map[string]string{"m_eventStringCode": "BUTN"})
		binary.LittleEndian.PutUint32(buf[33:], uint32(butn))
		return buf
	case 4:
		scalar["m_numActiveCars"] = 24
		for car := 0; car < 24; car++ {
			cars[car] = map[string]float64{"m_yourTelemetry": 1, "m_teamId": float64(car / 2), "m_driverId": float64(car + 1)}
		}
	case 6:
		for car := range cars {
			cars[car]["m_speed"] = speed
			cars[car]["m_throttle"] = float64((int(overall)+car*3)%101) / 100
			cars[car]["m_brake"] = 0
			cars[car]["m_gear"] = 7
			cars[car]["m_engineRPM"] = 11000
			cars[car]["m_engineTemperature"] = 105
			for w := 0; w < 4; w++ {
				cars[car][fmtIdx("m_tyresSurfaceTemperature", w)] = 80 + float64((second/15+car+w)%31)
				cars[car][fmtIdx("m_tyresInnerTemperature", w)] = 85 + float64((second/15+car+w)%21)
				cars[car][fmtIdx("m_tyresPressure", w)] = 22.5
				cars[car][fmtIdx("m_brakesTemperature", w)] = 400
			}
		}
	case 7:
		for car := range cars {
			cars[car]["m_fuelInTank"] = 40
			cars[car]["m_fuelRemainingLaps"] = 8.5
			cars[car]["m_ersStoreEnergy"] = 3000000
			cars[car]["m_ersDeployMode"] = 1
			cars[car]["m_ersHarvestLimitPerLap"] = 4000000
			cars[car]["m_actualTyreCompound"] = 18
			cars[car]["m_tyresAgeLaps"] = float64(lap % 12)
		}
	case 8:
		scalar["m_numCars"] = 24
	case 9:
		scalar["m_numPlayers"] = 1
	case 10:
		for car := range cars {
			for w := 0; w < 4; w++ {
				cars[car][fmtIdx("m_tyresWear", w)] = math.Min(95, 2.5*float64((lap-1)%12)+float64(w)/2)
			}
		}
	case 11:
		idx := ord % 24
		scalar["m_carIdx"] = float64(idx)
		scalar["m_numLaps"] = float64(lap)
		cars[idx] = map[string]float64{"m_lapTimeInMS": 90000, "m_lapNumber": float64(lap)}
	case 12:
		idx := ord % 24
		scalar["m_carIdx"] = float64(idx)
		cars[idx] = map[string]float64{"m_actualTyreCompound": 18, "m_tyreAgeLaps": float64(lap % 12)}
	case 13:
		scalar["m_gForceLateral"] = 1.5
		scalar["m_enginePowerICE"] = 700000
	case 14:
		scalar["m_playerSessionBestDataSet.m_sessionTime"] = 88.125
	case 15:
		scalar["m_numLaps"] = float64(lap)
	case 16:
		for car := range cars {
			cars[car]["m_activeAeroMode"] = 1
			cars[car]["m_activeAeroAvailable"] = 1
			cars[car]["m_overtakeAvailable"] = 1
			cars[car]["m_overtakeActive"] = 0
			cars[car]["m_2026Regulations"] = 1
		}
	}
	return protocol.BuildPacket(2026, pid, h, scalar, cars, strings)
}

func (c *Corpus) rewind(second int) float64 {
	if c.RewindAt == nil {
		return 0
	}
	return c.RewindAt(second)
}

func carsSeed(seed int64) int64 {
	if seed < 0 {
		return -seed
	}
	return seed
}

func fmtIdx(name string, i int) string {
	return name + "[" + string(rune('0'+i)) + "]"
}

// Sender paces slots and records the independent transmitted oracle.
type Sender struct {
	mu     sync.Mutex
	Count  uint64
	Bytes  uint64
	Digest hash.Hash
	Slots  uint64
}

func NewSender() *Sender { return &Sender{Digest: sha256.New()} }

func (s *Sender) Record(data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Count++
	s.Bytes += uint64(len(data))
	s.Digest.Write(data)
	s.Slots++
}

func (s *Sender) Sum() []byte { return s.Digest.Sum(nil) }

// Oracle separates transmitted, received and recorded evidence.
type Oracle struct {
	Transmitted   *Sender
	ReceivedCount uint64
	ReceivedBytes uint64
	ReceivedHash  []byte
	RecordedCount int
	RecordedHash  string
}
