// Package analytics derives per-driver lap, stint and session records from the
// already-recorded datagrams. MCAP stays the source of truth; these rows make
// review and charts cheap.
package analytics

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"sync"
	"time"

	"f1tele/server/internal/contract"
	"f1tele/server/internal/history"
	"f1tele/server/internal/protocol"
)

const MethodVersion = "lap-analytics-v2"

// MaxSamplesPerLap bounds one stored input trace. A 90-second lap sampled at
// 60 Hz stores roughly one point per 100 ms after this reduction.
const MaxSamplesPerLap = 900

type carState struct {
	lap          int
	startSeq     uint64
	maxFrame     uint32
	lastTotal    float64
	haveTotal    bool
	lapDistance  float64
	lastLapDist  float64
	haveLapDist  bool
	topSpeed     float64
	lapStartMs   int64
	ersMode      float64
	haveErs      bool
	overtakeOK   bool
	overtakeOn   bool
	haveOvertake bool
	restricted   bool
	position     int
	gapAheadMs   int64
	gapLeaderMs  int64
	fuelKg       float64
	ersJ         float64
	compound     string
	tyreAge      int
	wear         float64
	startFuel    float64
	startErs     float64
	startPos     int
	startGap     int64
	startLeader  int64
	startWear    float64
	pit          bool
	invalid      bool
	trackLimits  bool
	samples      []sample
	lastSampleMs int64
}

type sample struct {
	timeMs       int64
	distance     float64
	speed        float64
	throttle     float64
	brake        float64
	gear         int
	ersJ         float64
	deployMode   int
	overtakeOK   bool
	overtakeOn   bool
	haveErs      bool
	haveOvertake bool
	restricted   bool
}

type participant struct {
	name   string
	team   int
	number int
	ai     bool
	public bool
}

// Tracker consumes packets for one live session and writes derived rows.
type Tracker struct {
	Catalog *history.Catalog
	mu      sync.Mutex
	session string
	branch  string
	track   int
	length  float64
	player  int
	cars    map[int]*carState
	people  map[int]participant
	closed  bool
}

func NewTracker(cat *history.Catalog) *Tracker {
	return &Tracker{Catalog: cat, cars: map[int]*carState{}, people: map[int]participant{}, player: 0, length: 5000}
}

// Begin resets state when the logical session or branch changes.
func (t *Tracker) Begin(sessionID, branchID string, trackID, player int, trackLength float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.session == sessionID && t.branch == branchID {
		return
	}
	if t.session != "" && !t.closed {
		t.closeLocked(context.Background())
	}
	t.session, t.branch, t.track, t.player = sessionID, branchID, trackID, player
	if trackLength > 0 {
		t.length = trackLength
	}
	t.cars = map[int]*carState{}
	t.people = map[int]participant{}
	t.closed = false
}

// Session and Branch report the current scope.
func (t *Tracker) Session() (string, string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.session, t.branch
}

// Observe folds one accepted datagram into the running lap state.
func (t *Tracker) Observe(ctx context.Context, seq uint64, p *protocol.Packet, cx contract.Context, session map[string]float64) {
	if cx.SessionID == "" || cx.BranchID == "" {
		return
	}
	t.Begin(cx.SessionID, cx.BranchID, derefInt(cx.TrackID), derefInt(cx.PlayerCarIndex), session["track_length_m"])
	t.mu.Lock()
	defer t.mu.Unlock()
	switch p.Header.PacketID {
	case 4:
		for car := 0; car < p.CarCount(); car++ {
			name, _ := p.CarField("m_name", car)
			team, _ := p.CarField("m_teamId", car)
			number, _ := p.CarField("m_raceNumber", car)
			ai, _ := p.CarField("m_aiControlled", car)
			public := true
			if v, ok := p.CarField("m_yourTelemetry", car); ok {
				public = v.F != 0
			}
			if name.S == "" && team.F == 0 && number.F == 0 {
				continue
			}
			t.people[car] = participant{name: name.S, team: int(team.F), number: int(number.F), ai: ai.F != 0, public: public}
			t.car(car).restricted = car != t.player && !public
			_ = t.Catalog.UpsertParticipant(ctx, cx.SessionID, car, name.S, int(team.F), int(number.F), ai.F != 0, public, seq)
		}
	case 6:
		sessionMs := int64(math.Round(p.Header.SessionTime * 1000))
		for car := 0; car < p.CarCount(); car++ {
			st := t.car(car)
			var speed float64
			if v, ok := p.CarField("m_speed", car); ok {
				speed = v.F / 3.6
				if speed > st.topSpeed {
					st.topSpeed = speed
				}
			}
			throttle, okThrottle := p.CarField("m_throttle", car)
			brake, okBrake := p.CarField("m_brake", car)
			gear, okGear := p.CarField("m_gear", car)
			if !okThrottle || !okBrake || !okGear || st.lap <= 0 || st.lapDistance <= 0 {
				continue
			}
			st.samples = append(st.samples, sample{
				timeMs: sessionMs, distance: st.lapDistance, speed: speed,
				throttle: throttle.F, brake: brake.F, gear: int(gear.F),
				ersJ: st.ersJ, deployMode: int(st.ersMode), haveErs: st.haveErs,
				overtakeOK: st.overtakeOK, overtakeOn: st.overtakeOn, haveOvertake: st.haveOvertake,
				restricted: st.restricted,
			})
			// A long lap can exceed the live buffer. Halving keeps the whole lap
			// shape, both ends and every shift or brake change; truncation used to
			// drop the newest samples and leave the chart short.
			if len(st.samples) > 4000 {
				st.samples = compactSamples(st.samples)
			}
		}
	case 7:
		for car := 0; car < p.CarCount(); car++ {
			st := t.car(car)
			if v, ok := p.CarField("m_fuelInTank", car); ok {
				st.fuelKg = v.F
			}
			if v, ok := p.CarField("m_ersStoreEnergy", car); ok {
				st.ersJ = v.F
				st.haveErs = true
			}
			if v, ok := p.CarField("m_ersDeployMode", car); ok {
				st.ersMode = v.F
			}
			if v, ok := p.CarField("m_actualTyreCompound", car); ok {
				st.compound = protocol.CompoundLabel(int(v.F))
			}
			if v, ok := p.CarField("m_tyresAgeLaps", car); ok {
				st.tyreAge = int(v.F)
			}
		}
	case 10:
		for car := 0; car < p.CarCount(); car++ {
			st := t.car(car)
			maxWear := 0.0
			for w := 0; w < 4; w++ {
				if v, ok := p.CarField(fmt.Sprintf("m_tyresWear[%d]", w), car); ok && v.F > maxWear {
					maxWear = v.F
				}
			}
			for _, name := range []string{"m_frontLeftWingDamage", "m_frontRightWingDamage", "m_rearWingDamage", "m_floorDamage", "m_diffuserDamage", "m_sidepodDamage"} {
				if v, ok := p.CarField(name, car); ok && v.F >= 100 {
					st.trackLimits = true
				}
			}
			st.wear = maxWear
		}
	case 16:
		for car := 0; car < p.CarCount(); car++ {
			st := t.car(car)
			if v, ok := p.CarField("m_overtakeAvailable", car); ok {
				st.overtakeOK = v.F != 0
				st.haveOvertake = true
			}
			if v, ok := p.CarField("m_overtakeActive", car); ok {
				st.overtakeOn = v.F != 0
			}
		}
	case 11:
		t.observeHistory(ctx, seq, p)
	case 2:
		t.observeLap(ctx, seq, p)
	}
}

func (t *Tracker) car(i int) *carState {
	st := t.cars[i]
	if st == nil {
		st = &carState{}
		t.cars[i] = st
	}
	return st
}

func (t *Tracker) observeLap(ctx context.Context, seq uint64, p *protocol.Packet) {
	sessionMs := int64(math.Round(p.Header.SessionTime * 1000))
	for car := 0; car < p.CarCount(); car++ {
		st := t.car(car)
		lapV, ok := p.CarField("m_currentLapNum", car)
		if !ok || lapV.F <= 0 {
			continue
		}
		lap := int(lapV.F)
		if st.lap == 0 {
			st.lap, st.startSeq = lap, seq
			st.lapStartMs = sessionMs
			st.startFuel, st.startErs, st.startWear = st.fuelKg, st.ersJ, st.wear
			st.startPos = st.position
		}
		if lap > st.lap {
			t.finishLap(ctx, seq, car, p, st.lap)
			st.startSeq = seq
			st.lap = lap
			st.lapStartMs = sessionMs
			st.lapDistance = 0
			st.trackLimits = false
			st.startFuel, st.startErs, st.startWear = st.fuelKg, st.ersJ, st.wear
			st.startPos = st.position
			st.startGap, st.startLeader = st.gapAheadMs, st.gapLeaderMs
		}
		if v, ok := p.CarField("m_carPosition", car); ok {
			st.position = int(v.F)
		}
		if mins, ok := p.CarField("m_deltaToCarInFrontMinutesPart", car); ok {
			if ms, ok2 := p.CarField("m_deltaToCarInFrontMSPart", car); ok2 {
				st.gapAheadMs = int64(mins.F*60000 + ms.F)
			}
		}
		if mins, ok := p.CarField("m_deltaToRaceLeaderMinutesPart", car); ok {
			if ms, ok2 := p.CarField("m_deltaToRaceLeaderMSPart", car); ok2 {
				st.gapLeaderMs = int64(mins.F*60000 + ms.F)
			}
		}
		if v, ok := p.CarField("m_totalDistance", car); ok {
			if st.haveTotal && v.F >= st.lastTotal {
				st.lapDistance += v.F - st.lastTotal
			}
			st.lastTotal, st.haveTotal = v.F, true
		}
		if v, ok := p.CarField("m_lapDistance", car); ok {
			if !st.haveLapDist {
				st.lastLapDist, st.haveLapDist = v.F, true
			} else if v.F >= st.lastLapDist {
				st.lapDistance = v.F
			} else {
				st.lapDistance = v.F
			}
		}
		if v, ok := p.CarField("m_pitStatus", car); ok && v.F != 0 {
			st.pit = true
		}
		if v, ok := p.CarField("m_currentLapInvalid", car); ok && v.F != 0 {
			st.invalid = true
		}
	}
}

func (t *Tracker) finishLap(ctx context.Context, seq uint64, car int, p *protocol.Packet, lap int) {
	st := t.car(car)
	var lapMs int64
	if v, ok := p.CarField("m_lastLapTimeInMS", car); ok {
		lapMs = int64(v.F)
	}
	if lapMs <= 0 {
		return
	}
	distance := st.lapDistance
	if distance <= 0 {
		distance = t.length
	}
	avg := 0.0
	if lapMs > 0 {
		avg = distance / (float64(lapMs) / 1000)
	}
	top := st.topSpeed
	if len(st.samples) > 0 {
		top = math.Max(top, st.samples[len(st.samples)-1].speed)
	}
	if st.startPos == 0 {
		st.startPos = st.position
	}
	row := history.LapAnalytics{
		SessionID: t.session, BranchID: t.branch, CarIndex: car, Lap: lap,
		LapMs: lapMs, DistanceM: distance, AvgSpeed: avg, TopSpeed: top,
		PositionStart: st.startPos, PositionEnd: st.position,
		GapAheadStart: st.startGap, GapLeaderStart: st.startLeader,
		Compound: st.compound, TyreAge: st.tyreAge,
		FuelStart: st.startFuel, FuelEnd: st.fuelKg, ErsStart: st.startErs, ErsEnd: st.ersJ,
		PitIn: st.pit, Valid: !st.invalid, TrackLimits: st.trackLimits, Complete: true,
		CornerSpeeds: t.cornerSpeeds(st.samples),
	}
	if err := t.Catalog.UpsertLapAnalytics(ctx, row); err == nil && len(st.samples) > 1 {
		_ = t.Catalog.UpsertTrace(ctx, t.session, t.branch, car, lap, "car.speed_mps", decimate(st.samples, 300), MethodVersion)
	}
	if points := decimateSamples(st.samples, MaxSamplesPerLap); len(points) > 0 {
		_ = t.Catalog.UpsertLapSamples(ctx, t.session, t.branch, history.LapSamples{
			CarIndex: car, Lap: lap, StartMs: st.lapStartMs, LapMs: lapMs, Points: points,
		})
	}
	st.invalid, st.pit, st.trackLimits = false, false, false
	st.topSpeed = 0
	st.samples = nil
	st.startFuel, st.startErs, st.startWear = st.fuelKg, st.ersJ, st.wear
	st.startPos, st.startGap, st.startLeader = st.position, st.gapAheadMs, st.gapLeaderMs
	_ = seq
}

func (t *Tracker) observeHistory(ctx context.Context, seq uint64, p *protocol.Packet) {
	car := 0
	if v, ok := p.Field("m_carIdx"); ok {
		car = int(v.F)
	}
	numLaps := 0
	if v, ok := p.Field("m_numLaps"); ok {
		numLaps = int(v.F)
	}
	if numLaps <= 0 {
		return
	}
	idx := (numLaps - 1) % 100
	read := func(name string) (int64, bool) {
		v, ok := p.CarField(name, idx)
		return int64(v.F), ok
	}
	ms, okMs := read("m_lapTimeInMS")
	if !okMs || ms <= 0 {
		return
	}
	s1, ok1 := read("m_sector1TimeMSPart")
	s1m, _ := read("m_sector1TimeMinutesPart")
	s2, ok2 := read("m_sector2TimeMSPart")
	s2m, _ := read("m_sector2TimeMinutesPart")
	s3, ok3 := read("m_sector3TimeMSPart")
	s3m, _ := read("m_sector3TimeMinutesPart")
	flags, _ := read("m_lapValidBitFlags")
	if ok1 {
		s1 += s1m * 60000
	}
	if ok2 {
		s2 += s2m * 60000
	}
	if ok3 {
		s3 += s3m * 60000
	}
	_ = ctx
	_ = seq
	_ = t.Catalog.UpdateLapSectors(ctx, t.session, t.branch, car, numLaps, ms, s1, s2, s3, flags&0x1 != 0)
}

func (t *Tracker) cornerSpeeds(samples []sample) map[string]contract.Measurement {
	out := map[string]contract.Measurement{}
	zones := zonesFor(t.track)
	if len(samples) == 0 || len(zones) == 0 {
		return out
	}
	for _, z := range zones {
		center := z.Norm * t.length
		lo, hi := center-50, center+50
		minSpeed, sum, n := math.Inf(1), 0.0, 0
		for _, s := range samples {
			if s.distance >= lo && s.distance <= hi {
				minSpeed = math.Min(minSpeed, s.speed)
				sum += s.speed
				n++
			}
		}
		if n == 0 {
			continue
		}
		m := contract.Derived(minSpeed, "m/s", "corner-min-provisional-v1")
		m.Basis = "derived"
		out[fmt.Sprintf("T%d", z.Number)] = m
		avg := contract.Derived(sum/float64(n), "m/s", "corner-avg-provisional-v1")
		out[fmt.Sprintf("T%d.avg", z.Number)] = avg
	}
	return out
}

// Close writes stints and the session summary. It is safe to call repeatedly.
func (t *Tracker) Close(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.closeLocked(ctx)
}

func (t *Tracker) closeLocked(ctx context.Context) error {
	if t.session == "" || t.closed {
		return nil
	}
	if err := t.Catalog.RebuildStints(ctx, t.session, t.branch); err != nil {
		return err
	}
	t.closed = true
	return t.Catalog.UpsertSessionAnalytics(ctx, t.session, t.branch, MethodVersion)
}

func decimate(in []sample, max int) []byte {
	if len(in) <= max {
		out := make([][2]float64, len(in))
		for i, s := range in {
			out[i] = [2]float64{math.Round(s.distance*10) / 10, math.Round(s.speed*100) / 100}
		}
		b, _ := json.Marshal(out)
		return b
	}
	step := float64(len(in)) / float64(max)
	var out [][2]float64
	for i := 0; i < max; i++ {
		s := in[int(float64(i)*step)]
		out = append(out, [2]float64{math.Round(s.distance*10) / 10, math.Round(s.speed*100) / 100})
	}
	b, _ := json.Marshal(out)
	return b
}

func derefInt(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func median(v []int64) int64 {
	if len(v) == 0 {
		return 0
	}
	s := append([]int64(nil), v...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	return s[len(s)/2]
}

var _ = sql.ErrNoRows
var _ = strconv.Itoa
var _ = time.Now
