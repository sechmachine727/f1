// Package state owns the single serialized live reducer.
package state

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strconv"
	"sync"
	"time"

	"f1tele/server/internal/contract"
	"f1tele/server/internal/protocol"
)

const (
	highRateTTL   = 1000 * time.Millisecond
	statusTTL     = 3000 * time.Millisecond
	staleAfter    = 2000 * time.Millisecond
	reorderWindow = 128
)

type clock func() time.Time

type key struct {
	packet uint8
	field  string
	car    int
}

type sample struct {
	value    protocol.Value
	seq      uint64
	frame    uint32
	observed time.Time
	source   string
}

type Participant struct {
	Index        int    `json:"index"`
	Name         string `json:"name"`
	TeamID       int    `json:"team_id"`
	DriverID     int    `json:"driver_id"`
	RaceNumber   int    `json:"race_number"`
	AI           bool   `json:"ai"`
	MyTeam       bool   `json:"my_team"`
	Restricted   bool   `json:"restricted"`
	NumColours   int    `json:"num_colours"`
	TelemetrySet bool   `json:"your_telemetry"`
}

// Snapshot is an immutable copy of current live state.
type Snapshot struct {
	Context        contract.Context
	Metrics        []contract.Metric
	Events         []contract.Event
	Participants   []Participant
	ObservationSeq uint64
	SessionTimeMs  *int64
	LastPacketAt   time.Time
	LastAcceptedAt time.Time
	RecordingState string
	Lap            *int
	Session        map[string]float64
}

type Observer interface {
	OnPacket(seq uint64, at time.Time, data []byte, packet *protocol.Packet, valid bool, reason string)
	OnBranch(b contract.Branch)
}

type Reducer struct {
	mu        sync.RWMutex
	now       clock
	sourceCtx string
	observer  Observer

	sessionID       string
	fragmentID      string
	branchID        string
	branches        []contract.Branch
	uid             uint64
	seq             uint64
	lastPacket      time.Time
	lastAccept      time.Time
	lastMono        time.Time
	lastOverall     uint32
	lastSessionTime float64
	maxFrame        uint32
	revision        int

	format                               int
	gameYear                             int
	major                                int
	minor                                int
	formula                              int
	trackID                              int
	sessionType                          int
	gameMode                             int
	online                               int
	season                               *int
	seasonEvidence                       []string
	reg2026                              *bool
	linkSeason, linkWeekend, linkSession uint64
	playerCar                            int

	fields       map[key]sample
	carCount     int
	participants []Participant
	events       []contract.Event
	eventSeen    map[string]uint32
	laps         map[int]int
	lastLapMs    map[int]int64
	curveFrame   uint32
	recording    string
	gaps         []string
}

func New(sourceCtx string, now clock) *Reducer {
	if now == nil {
		now = time.Now
	}
	r := &Reducer{
		sourceCtx: sourceCtx, now: now, recording: "idle",
		fields: map[key]sample{}, eventSeen: map[string]uint32{}, laps: map[int]int{}, lastLapMs: map[int]int64{},
		sessionID: newID("ses"), branchID: newID("br"), fragmentID: newID("frg"),
	}
	r.branches = []contract.Branch{{ID: r.branchID, SessionID: r.sessionID, IsSurviving: true}}
	return r
}

func newID(prefix string) string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return prefix + "-" + hex.EncodeToString(b)
}

func (r *Reducer) SetObserver(o Observer) { r.mu.Lock(); r.observer = o; r.mu.Unlock() }

// Apply reduces one validated datagram. Invalid packets are reported to the
// observer but never mutate live state.
func (r *Reducer) Apply(seq uint64, at time.Time, data []byte, p *protocol.Packet, invalidReason string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq = seq
	r.lastPacket = at
	r.lastAccept = at
	r.lastMono = at
	if invalidReason != "" || p == nil {
		if r.observer != nil {
			r.observer.OnPacket(seq, at, data, p, false, invalidReason)
		}
		return
	}
	h := p.Header
	prevOverall := r.lastOverall
	r.format = int(h.Format)
	r.gameYear = int(h.GameYear)
	r.major = int(h.MajorVersion)
	r.minor = int(h.MinorVersion)
	r.playerCar = int(h.PlayerCarIndex)
	if r.carCount == 0 || p.CarCount() > r.carCount {
		r.carCount = p.CarCount()
	}
	r.observeSession(h, p)
	r.checkRewind(seq, h, prevOverall)
	r.observeParticipants(p)
	r.observeFields(seq, at, p)
	if h.PacketID == 16 {
		r.resolveSeason(p)
	}
	r.observeEvents(seq, at, p)
	r.observeLaps(seq, at, p)
	if r.observer != nil {
		r.observer.OnPacket(seq, at, data, p, true, "")
	}
}

func (r *Reducer) observeSession(h protocol.Header, p *protocol.Packet) {
	uid := h.SessionUID
	if uid != 0 && uid != r.uid {
		if r.uid != 0 {
			r.newFragment(uid)
		}
		r.uid = uid
	}
	if p.Header.PacketID != 1 {
		return
	}
	if v, ok := p.Field("m_trackId"); ok {
		r.trackID = int(v.F)
	}
	if v, ok := p.Field("m_sessionType"); ok {
		r.sessionType = int(v.F)
	}
	if v, ok := p.Field("m_gameMode"); ok {
		r.gameMode = int(v.F)
	}
	if v, ok := p.Field("m_networkGame"); ok {
		r.online = int(v.F)
	}
	if v, ok := p.Field("m_formula"); ok {
		r.formula = int(v.F)
	}
	if v, ok := p.Field("m_seasonLinkIdentifier"); ok {
		r.linkSeason = uint64(v.F)
	}
	if v, ok := p.Field("m_weekendLinkIdentifier"); ok {
		r.linkWeekend = uint64(v.F)
	}
	if v, ok := p.Field("m_sessionLinkIdentifier"); ok {
		r.linkSession = uint64(v.F)
	}
	r.resolveSeason(p)
}

// resolveSeason keeps the evidence rather than accepting packet format as proof.
func (r *Reducer) resolveSeason(p *protocol.Packet) {
	evidence := []string{}
	want2026, want2025 := false, false
	if r.formula == 13 {
		want2026 = true
		evidence = append(evidence, "formula=13")
	}
	if p.Header.Format == 2026 && p.Header.PacketID == 16 {
		if v, ok := p.CarField("m_2026Regulations", r.playerCar); ok {
			b := int(v.F) == 1
			r.reg2026 = &b
		}
	}
	if r.reg2026 != nil {
		if *r.reg2026 {
			want2026 = true
		} else {
			want2025 = true
		}
		evidence = append(evidence, fmt.Sprintf("packet16.applies_2026=%t", *r.reg2026))
	}
	r.seasonEvidence = evidence
	switch {
	case want2026 && want2025:
		r.season = nil
		r.seasonEvidence = append(evidence, "conflicting regulation evidence")
	case want2026:
		v := 2026
		r.season = &v
	case want2025 && r.supportedPre2026():
		v := 2025
		r.season = &v
	default:
		r.season = nil
	}
}

func (r *Reducer) supportedPre2026() bool {
	// Formula 0 (F1 Modern) is the only supported pre-2026 car context here.
	return r.formula == 0
}

func (r *Reducer) newFragment(uid uint64) {
	r.fragmentID = newID("frg")
	r.sessionID = newID("ses")
	r.branchID = newID("br")
	r.branches = []contract.Branch{{ID: r.branchID, SessionID: r.sessionID, IsSurviving: true}}
	r.events = nil
	r.season = nil
	r.seasonEvidence = nil
	r.reg2026 = nil
	r.lastSessionTime = 0
}

func (r *Reducer) observeParticipants(p *protocol.Packet) {
	if p.Header.PacketID != 4 {
		return
	}
	if v, ok := p.Field("m_numActiveCars"); ok && int(v.F) > 0 && int(v.F) <= 24 {
		r.carCount = int(v.F)
	}
	parts := make([]Participant, r.carCount)
	for i := 0; i < r.carCount; i++ {
		pdata := Participant{Index: i}
		if v, ok := p.CarField("m_name", i); ok {
			pdata.Name = v.S
		}
		if v, ok := p.CarField("m_teamId", i); ok {
			pdata.TeamID = int(v.F)
		}
		if v, ok := p.CarField("m_driverId", i); ok {
			pdata.DriverID = int(v.F)
		}
		if v, ok := p.CarField("m_raceNumber", i); ok {
			pdata.RaceNumber = int(v.F)
		}
		if v, ok := p.CarField("m_aiControlled", i); ok {
			pdata.AI = v.F == 1
		}
		if v, ok := p.CarField("m_myTeam", i); ok {
			pdata.MyTeam = v.F == 1
		}
		if v, ok := p.CarField("m_yourTelemetry", i); ok {
			pdata.TelemetrySet = true
			pdata.Restricted = v.F == 0 && i != r.playerCar
		}
		parts[i] = pdata
	}
	r.participants = parts
}

func (r *Reducer) observeFields(seq uint64, at time.Time, p *protocol.Packet) {
	h := p.Header
	if h.PacketID == 2 {
		// The delta is split into minutes and milliseconds; publish the sum so
		// the pit wall can compare gaps without recombining parts.
		for car := 0; car < p.CarCount(); car++ {
			mins, ok1 := p.CarField("m_deltaToCarInFrontMinutesPart", car)
			ms, ok2 := p.CarField("m_deltaToCarInFrontMSPart", car)
			if !ok1 || !ok2 {
				continue
			}
			r.store(key{h.PacketID, "timing.gap_ahead_s", car}, sample{value: protocol.Value{F: mins.F*60 + ms.F/1000}, seq: seq, frame: h.OverallFrameID, observed: at, source: "packet2"})
			leadMins, ok3 := p.CarField("m_deltaToRaceLeaderMinutesPart", car)
			leadMs, ok4 := p.CarField("m_deltaToRaceLeaderMSPart", car)
			if ok3 && ok4 {
				r.store(key{h.PacketID, "timing.gap_leader_s", car}, sample{value: protocol.Value{F: leadMins.F*60 + leadMs.F/1000}, seq: seq, frame: h.OverallFrameID, observed: at, source: "packet2"})
			}
		}
	}
	if h.OverallFrameID != r.maxFrame && int32(h.OverallFrameID-r.maxFrame) > 0 {
		r.maxFrame = h.OverallFrameID
	}
	r.lastOverall = h.OverallFrameID
	for _, spec := range protocol.ByPacket(h.PacketID) {
		if spec.Field == "" || len(spec.Field) > 8 && spec.Field[:8] == "derived:" {
			continue
		}
		if spec.CarIndexed {
			n := p.CarCount()
			if n == 0 {
				continue
			}
			for car := 0; car < n; car++ {
				v, ok := p.CarField(spec.Field, car)
				if !ok {
					continue
				}
				r.store(key{h.PacketID, spec.Name, car}, sample{value: v, seq: seq, frame: h.OverallFrameID, observed: at, source: "packet" + strconv.Itoa(int(h.PacketID))})
			}
		} else {
			v, ok := p.Field(spec.Field)
			if !ok {
				continue
			}
			r.store(key{h.PacketID, spec.Name, -1}, sample{value: v, seq: seq, frame: h.OverallFrameID, observed: at, source: "packet" + strconv.Itoa(int(h.PacketID))})
		}
	}
}

func (r *Reducer) store(k key, s sample) {
	old, ok := r.fields[k]
	if ok {
		if old.frame == s.frame && old.value == s.value {
			return
		}
		if old.frame != 0 && int32(s.frame-old.frame) <= 0 {
			return
		}
	}
	r.fields[k] = s
}

func (r *Reducer) observeEvents(seq uint64, at time.Time, p *protocol.Packet) {
	h := p.Header
	if h.PacketID != 3 {
		return
	}
	code := eventCode(p)
	details := p.EventDetails()
	if code == "" {
		return
	}
	fingerprint := fmt.Sprintf("%d|%s|%v", h.OverallFrameID, code, details)
	if last, ok := r.eventSeen[fingerprint]; ok && last == h.OverallFrameID {
		return
	}
	if r.maxFrame != 0 && int32(h.OverallFrameID-r.maxFrame) < -reorderWindow {
		return
	}
	r.eventSeen[fingerprint] = h.OverallFrameID
	if len(r.eventSeen) > 2048 {
		r.eventSeen = map[string]uint32{fingerprint: h.OverallFrameID}
	}
	evt := contract.Event{
		ID: newID("evt"), Seq: strconv.FormatUint(seq, 10),
		Type: "race_control", Severity: "routine",
		SessionTimeMs: int64p(int64(math.Round(h.SessionTime * 1000))),
		EvidenceIDs:   []string{fmt.Sprintf("seq:%d", seq)}, Summary: code,
	}
	if v, ok := details["vehicleIdx"]; ok {
		if n, ok := v.(int); ok && n >= 0 && n < 24 {
			evt.CarIndex = intp(n)
		}
	}
	switch code {
	case "FLBK":
		evt.Type = "flashback"
		evt.Summary = "Flashback"
		anchor := int64(math.Round(float64(details["flashbackSessionTime"].(float64)) * 1000))
		r.newBranch(seq, anchor)
	case "SCAR":
		typ := detailInt(details, "safetyCarType")
		state := detailInt(details, "eventType")
		if state == 0 {
			if typ == 2 {
				evt.Type = "virtual_safety_car"
			} else {
				evt.Type = "safety_car"
			}
			evt.Severity = "important"
			evt.Summary = "Safety Car deployed"
		} else {
			evt.Summary = "Safety Car state changed"
		}
	case "RDFL":
		evt.Type = "red_flag"
		evt.Severity = "important"
		evt.Summary = "Red flag"
	case "CHQF":
		evt.Type = "race_control"
		evt.Summary = "Chequered flag"
	case "COLL":
		if detailInt(details, "severity") >= 1 {
			evt.Severity = "important"
		}
		evt.Summary = "Collision"
	case "RTMT":
		evt.Severity = "important"
		evt.Summary = "Retirement"
	case "PENA":
		evt.Summary = "Penalty"
	case "SPTP":
		evt.Summary = "Speed trap"
	case "FTLP":
		evt.Summary = "Fastest lap"
	case "OVTK":
		evt.Summary = "Overtake"
	case "DRSE", "DRSD":
		evt.Summary = "DRS state changed"
	default:
		evt.Type = "other"
	}
	r.events = append(r.events, evt)
	if len(r.events) > 512 {
		r.events = append([]contract.Event(nil), r.events[len(r.events)-512:]...)
	}
}

func eventCode(p *protocol.Packet) string {
	if len(p.Data) < 33 {
		return ""
	}
	v := ""
	for _, b := range p.Data[29:33] {
		if b == 0 {
			break
		}
		v += string(b)
	}
	return v
}

func (r *Reducer) observeLaps(seq uint64, at time.Time, p *protocol.Packet) {
	if p.Header.PacketID != 2 {
		return
	}
	for car := 0; car < p.CarCount(); car++ {
		v, ok := p.CarField("m_currentLapNum", car)
		if !ok {
			continue
		}
		lap := int(v.F)
		prev := r.laps[car]
		if prev != 0 && lap == prev+1 {
			lm := int64(0)
			if t, ok := p.CarField("m_lastLapTimeInMS", car); ok {
				lm = int64(t.F)
			}
			r.lastLapMs[car] = lm
			scope := "rival"
			if car == r.playerCar {
				scope = "player"
			}
			evt := contract.Event{
				ID: newID("evt"), Seq: strconv.FormatUint(seq, 10), Type: "lap_completed", Severity: "routine",
				CarIndex: intp(car), Summary: fmt.Sprintf("Lap %d completed (%s)", lap-1, scope),
				EvidenceIDs: []string{fmt.Sprintf("seq:%d", seq)},
			}
			if car == r.playerCar {
				evt.Summary = fmt.Sprintf("Lap %d completed", lap-1)
			}
			r.events = append(r.events, evt)
		}
		r.laps[car] = lap
	}
}

func detailInt(m map[string]any, key string) int {
	v, ok := m[key]
	if !ok {
		return 0
	}
	if n, ok := v.(int); ok {
		return n
	}
	return 0
}

func (r *Reducer) checkRewind(seq uint64, h protocol.Header, prevOverall uint32) {
	st := h.SessionTime
	if r.lastSessionTime > 0 && st+1.0 < r.lastSessionTime && int32(h.OverallFrameID-prevOverall) > 0 {
		r.newBranch(seq, int64(math.Round(st*1000)))
	}
	if st >= r.lastSessionTime {
		r.lastSessionTime = st
	}
}

func (r *Reducer) newBranch(seq uint64, anchorMs int64) {
	if n := len(r.branches); n > 0 {
		if a := r.branches[n-1].AnchorSeq; a != nil && *a == strconv.FormatUint(seq, 10) {
			return
		}
	}
	parent := r.branchID
	r.branchID = newID("br")
	r.lastSessionTime = float64(anchorMs) / 1000
	for i := range r.branches {
		r.branches[i].IsSurviving = false
	}
	anchorSeq := strconv.FormatUint(seq, 10)
	b := contract.Branch{
		ID: r.branchID, SessionID: r.sessionID, ParentID: &parent,
		AnchorSeq: &anchorSeq, AnchorTimeMs: &anchorMs, IsSurviving: true,
	}
	r.branches = append(r.branches, b)
	if r.observer != nil {
		r.observer.OnBranch(b)
	}
}

func (r *Reducer) SetRecordingState(s string) {
	r.mu.Lock()
	r.recording = s
	r.mu.Unlock()
}

func (r *Reducer) RecordingState() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.recording
}

func (r *Reducer) SessionID() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sessionID
}

func (r *Reducer) BranchID() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.branchID
}

func (r *Reducer) PlayerCar() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.playerCar
}

func (r *Reducer) Context() contract.Context {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.contextLocked()
}

func (r *Reducer) contextLocked() contract.Context {
	c := contract.Context{
		SessionID: r.sessionID, FragmentID: r.fragmentID, BranchID: r.branchID,
		SourceKind: "live", SeasonEvidence: append([]string{}, r.seasonEvidence...),
		ModeFamily: modeFamily(r.gameMode, r.sessionType), Revision: r.revision,
		SourceContextID: r.sourceCtx,
	}
	if r.uid != 0 {
		uid := strconv.FormatUint(r.uid, 10)
		c.GameSessionUID = &uid
	}
	if r.format != 0 {
		v := r.format
		c.UDPFormat = &v
		g := r.gameYear
		c.GameYear = &g
		maj, min := r.major, r.minor
		c.GameMajorVersion = &maj
		c.GameMinorVersion = &min
	}
	c.RegulationSeason = r.season
	if r.linkSeason != 0 {
		c.SeasonLinkID = u64p(r.linkSeason)
	}
	if r.linkWeekend != 0 {
		c.WeekendLinkID = u64p(r.linkWeekend)
	}
	if r.linkSession != 0 {
		c.SessionLinkID = u64p(r.linkSession)
	}
	if r.gameMode != 0 {
		c.GameModeCode = intp(r.gameMode)
	}
	if r.sessionType != 0 {
		c.SessionTypeCode = intp(r.sessionType)
	}
	if r.online >= 0 && (r.gameMode != 0 || r.sessionType != 0) {
		b := r.online == 1
		c.Online = &b
	}
	if r.trackID != 0 {
		c.TrackID = intp(r.trackID)
	}
	if r.carCount > 0 {
		c.PlayerCarIndex = intp(r.playerCar)
	}
	if r.formula != 0 {
		c.FormulaCode = intp(r.formula)
	}
	return c
}

func modeFamily(gameMode, sessionType int) string {
	// Published game-mode IDs from the supplied UDP appendix. Session type 18 is
	// Time Trial; it is the only place that identifier appears.
	switch gameMode {
	case 5:
		return "time_trial"
	case 7, 15, 29:
		return "multiplayer"
	case 27, 28, 30:
		return "career"
	case 4, 6, 17, 75:
		return "solo"
	case 127:
		return "other"
	case 0:
		if sessionType == 18 {
			return "time_trial"
		}
		return "unknown"
	}
	return "unknown"
}

// Snapshot copies state for downstream consumers.
func (r *Reducer) Snapshot(cars []int, channels []string) Snapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s := Snapshot{
		Context: r.contextLocked(), Events: append([]contract.Event(nil), r.events...),
		Participants: append([]Participant(nil), r.participants...), ObservationSeq: r.seq,
		LastPacketAt: r.lastPacket, LastAcceptedAt: r.lastAccept, RecordingState: r.recording,
		Session: map[string]float64{},
	}
	if r.carCount > 0 && r.playerCar < r.carCount {
		s.Lap = intp(r.laps[r.playerCar])
	}
	if st := r.lastSessionTime; st > 0 {
		s.SessionTimeMs = int64p(int64(math.Round(st * 1000)))
	}
	if st, ok := r.fields[key{1, "session.track_temp_c", -1}]; ok {
		s.Session["track_temp_c"] = st.value.F
	}
	for k, v := range r.fields {
		if k.field == "session.weather" {
			s.Session["weather"] = v.value.F
		}
		if k.field == "session.safety_car_status" {
			s.Session["safety_car_status"] = v.value.F
		}
		if k.field == "session.track_length_m" {
			s.Session["track_length_m"] = v.value.F
		}
	}
	s.Metrics = r.metricsLocked(cars, channels)
	return s
}

func (r *Reducer) metricsLocked(cars []int, channels []string) []contract.Metric {
	if len(cars) == 0 {
		cars = []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23}
	}
	want := map[string]bool{}
	if len(channels) == 0 {
		for _, c := range protocol.Canonical() {
			want[c.Name] = true
		}
	} else {
		for _, c := range channels {
			want[c] = true
		}
	}
	now := r.now()
	out := make([]contract.Metric, 0, len(cars)*len(want))
	for _, ch := range protocol.Canonical() {
		if !want[ch.Name] {
			continue
		}
		if ch.CarIndexed {
			for _, car := range cars {
				if car < 0 || car > 23 {
					continue
				}
				out = append(out, r.metricLocked(ch, car, now))
			}
		} else {
			out = append(out, r.metricLocked(ch, -1, now))
		}
	}
	return out
}

func (r *Reducer) metricLocked(ch protocol.ChannelSpec, car int, now time.Time) contract.Metric {
	id := ch.Name
	if car >= 0 {
		id = ch.Name + "@" + strconv.Itoa(car)
	}
	m := contract.Metric{ID: id, Channel: ch.Name}
	if car >= 0 {
		m.CarIndex = intp(car)
	}
	s, ok := r.fields[key{ch.PacketID, ch.Name, car}]
	if !ok {
		m.Measurement = contract.Unavailable("not_received", ch.Unit, "f1tele.state", "no packet received")
		return m
	}
	if ch.Restricted && car != r.playerCar && car < len(r.participants) && r.participants[car].Restricted {
		m.Measurement = contract.Unavailable("restricted", ch.Unit, "f1tele.state", "sender restricts this channel for other cars")
		m.Measurement.AsOfMs = int64p(s.observed.UnixMilli())
		return m
	}
	if ch.ValueType == "number" || ch.ValueType == "integer" {
		if !finite(s.value.F) {
			m.Measurement = contract.Unavailable("invalid", ch.Unit, "f1tele.state", "non-finite value")
			return m
		}
	}
	ttl := freshness(ch.FrequencyClass)
	if ttl > 0 && now.Sub(s.observed) > ttl {
		m.Measurement = contract.Unavailable("stale", ch.Unit, s.source, "no update within freshness window")
		m.Measurement.AsOfMs = int64p(s.observed.UnixMilli())
		return m
	}
	value := any(s.value.F*ch.Scale + ch.Offset)
	if ch.ValueType == "string" {
		value = s.value.S
	} else if ch.ValueType == "boolean" {
		value = s.value.F != 0
	}
	switch ch.Name {
	case "energy.deploy_mode":
		value = protocol.DeployModeLabel(s.value.F)
	case "aero.active_mode":
		value = protocol.ActiveAeroLabel(s.value.F)
	case "tyre.compound":
		value = protocol.CompoundLabel(int(s.value.F))
	}
	m.Measurement = contract.Available(value, ch.Unit, s.source)
	m.Measurement.AsOfMs = int64p(s.observed.UnixMilli())
	return m
}

func freshness(class string) time.Duration {
	switch class {
	case "high_rate":
		return highRateTTL
	case "status":
		return statusTTL
	}
	return 0
}

func (r *Reducer) HealthState() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.lastPacket.IsZero() {
		return "awaiting_telemetry"
	}
	if r.now().Sub(r.lastPacket) > staleAfter {
		return "stale"
	}
	return "live"
}

func (r *Reducer) LastPacketAt() time.Time {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lastPacket
}

func (r *Reducer) ObservationSeq() uint64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.seq
}

func (r *Reducer) Events() []contract.Event {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]contract.Event(nil), r.events...)
}

func (r *Reducer) Branch(id string) (contract.Branch, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, b := range r.branches {
		if b.ID == id {
			return b, true
		}
	}
	return contract.Branch{}, false
}

func (r *Reducer) EventsByType(types ...string) []contract.Event {
	r.mu.RLock()
	defer r.mu.RUnlock()
	set := map[string]bool{}
	for _, t := range types {
		set[t] = true
	}
	var out []contract.Event
	for _, e := range r.events {
		if set[e.Type] {
			out = append(out, e)
		}
	}
	return out
}

func (r *Reducer) Participants() []Participant {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]Participant(nil), r.participants...)
}

func (r *Reducer) CarCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.carCount == 0 {
		return 0
	}
	return r.carCount
}

func (r *Reducer) LastLapMs(car int) int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lastLapMs[car]
}

func (r *Reducer) SourceContext() string { return r.sourceCtx }

func (r *Reducer) fieldValue(channel string, car int) (sample, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	spec, ok := protocol.Lookup(channel)
	if !ok {
		return sample{}, false
	}
	s, ok := r.fields[key{spec.PacketID, channel, car}]
	return s, ok
}

// Value returns the latest value for a canonical channel and car.
func (r *Reducer) Value(channel string, car int) (float64, bool) {
	s, ok := r.fieldValue(channel, car)
	if !ok {
		return 0, false
	}
	spec, ok := protocol.Lookup(channel)
	if !ok {
		return 0, false
	}
	return s.value.F*spec.Scale + spec.Offset, true
}

func (r *Reducer) LastBranch() []contract.Branch {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]contract.Branch(nil), r.branches...)
}

func (r *Reducer) SetRecordingGap(reason string) {
	r.mu.Lock()
	r.gaps = append(r.gaps, reason)
	if len(r.gaps) > 64 {
		r.gaps = r.gaps[len(r.gaps)-64:]
	}
	r.mu.Unlock()
}

func (r *Reducer) Gaps() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]string(nil), r.gaps...)
}

func (r *Reducer) SortedParticipants() []Participant {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := append([]Participant(nil), r.participants...)
	sort.Slice(out, func(i, j int) bool { return out[i].Index < out[j].Index })
	return out
}

func finite(f float64) bool { return !math.IsNaN(f) && !math.IsInf(f, 0) }

func intp(v int) *int       { return &v }
func int64p(v int64) *int64 { return &v }
func u64p(v uint64) *string { s := strconv.FormatUint(v, 10); return &s }

// Observation builds a contract observation for the agent boundary.
func (r *Reducer) Observation(cars []int, channels []string, ests []contract.Estimate, eval *contract.StrategyEvaluation) contract.Observation {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ctx := r.contextLocked()
	m := r.metricsLocked(cars, channels)
	o := contract.Observation{
		SchemaVersion: contract.SchemaVersion, Context: ctx,
		ObservationSeq: strconv.FormatUint(r.seq, 10), CapturedAtMs: r.now().UnixMilli(),
		Window:  contract.Range{FromSeq: "0", ToSeq: strconv.FormatUint(r.seq, 10)},
		Metrics: m, Events: append([]contract.Event(nil), r.events...), Estimates: ests,
		Coverage:           contract.Coverage{Gaps: append([]string(nil), r.gaps...), StaleChannels: staleChannels(m)},
		StrategyEvaluation: eval,
	}
	if r.carCount > 0 {
		o.Lap = intp(r.laps[r.playerCar])
	}
	if st, ok := r.fields[key{1, "session.track_temp_c", -1}]; ok {
		_ = st
	}
	if st := r.lastSessionTime; st > 0 {
		o.SessionTimeMs = int64p(int64(math.Round(st * 1000)))
	}
	return o
}

func staleChannels(metrics []contract.Metric) []string {
	var out []string
	for _, m := range metrics {
		if m.Measurement.Availability == "stale" || m.Measurement.Availability == "not_received" {
			out = append(out, m.ID)
		}
	}
	if len(out) > 128 {
		out = out[:128]
	}
	if out == nil {
		out = []string{}
	}
	return out
}
