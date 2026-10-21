package history

import (
	"errors"
	"io"
	"os"
	"sync"
	"time"

	"f1tele/server/internal/contract"
	"f1tele/server/internal/protocol"
	"f1tele/server/internal/state"

	"github.com/foxglove/mcap/go/mcap"
)

// ReplaySource yields recorded datagrams in file order for an independent reducer.
type ReplaySource struct {
	Messages []ReplayMessage
	stream   *replayStream
	cursor   int
}

type ReplayMessage struct {
	Seq        uint64
	ReceivedAt int64
	Data       []byte
}

// replayStream reads one MCAP at a time so a long session is never fully
// buffered. Segment ranges keep the selected branch's surviving timeline.
type replayStream struct {
	paths  []string
	segs   []Segment
	next   int
	file   *os.File
	rd     *mcap.Reader
	it     mcap.MessageIterator
	closed bool
}

func LoadReplay(path string) (*ReplaySource, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r, err := mcap.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	it, err := r.Messages()
	if err != nil {
		return nil, err
	}
	src := &ReplaySource{}
	for {
		_, _, m, err := it.Next(nil)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if m == nil {
			break
		}
		src.Messages = append(src.Messages, ReplayMessage{Seq: uint64(m.Sequence), ReceivedAt: int64(m.LogTime / 1e6), Data: append([]byte(nil), m.Data...)})
	}
	return src, nil
}

// LoadReplaySession streams every recording file for a session in order,
// optionally filtered to a branch's segment ranges.
func LoadReplaySession(paths []string, segs []Segment) (*ReplaySource, error) {
	if len(paths) == 0 {
		return nil, errors.New("no recording for session")
	}
	f, err := os.Open(paths[0])
	if err != nil {
		return nil, err
	}
	f.Close()
	return &ReplaySource{stream: &replayStream{paths: paths, segs: segs}}, nil
}

func (s *ReplaySource) nextMessage() (ReplayMessage, bool) {
	if s.stream != nil {
		return s.stream.nextMessage()
	}
	if s.cursor >= len(s.Messages) {
		return ReplayMessage{}, false
	}
	m := s.Messages[s.cursor]
	s.cursor++
	return m, true
}

func (s *ReplaySource) rewind() {
	s.cursor = 0
	if s.stream != nil {
		s.stream.reset()
	}
}

func (s *ReplaySource) close() {
	if s.stream != nil {
		s.stream.close()
	}
}

func (s *replayStream) reset() {
	s.closeFile()
	s.next = 0
	s.closed = false
}

func (s *replayStream) close() {
	s.closed = true
	s.closeFile()
}

func (s *replayStream) closeFile() {
	if s.rd != nil {
		s.rd.Close()
	}
	if s.file != nil {
		_ = s.file.Close()
	}
	s.rd, s.file, s.it = nil, nil, nil
}

func (s *replayStream) open(path string) bool {
	if s.closed {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	rd, err := mcap.NewReader(f)
	if err != nil {
		f.Close()
		return false
	}
	it, err := rd.Messages()
	if err != nil {
		rd.Close()
		f.Close()
		return false
	}
	s.file, s.rd, s.it = f, rd, it
	return true
}

func (s *replayStream) nextMessage() (ReplayMessage, bool) {
	for {
		if s.closed {
			return ReplayMessage{}, false
		}
		if s.it == nil {
			if s.next >= len(s.paths) {
				return ReplayMessage{}, false
			}
			path := s.paths[s.next]
			s.next++
			if !s.open(path) {
				continue
			}
		}
		_, _, m, err := s.it.Next(nil)
		if err != nil || m == nil {
			s.closeFile()
			continue
		}
		if !seqInSegments(uint64(m.Sequence), s.segs) {
			continue
		}
		return ReplayMessage{Seq: uint64(m.Sequence), ReceivedAt: int64(m.LogTime / 1e6), Data: append([]byte(nil), m.Data...)}, true
	}
}

func seqInSegments(seq uint64, segs []Segment) bool {
	if len(segs) == 0 {
		return true
	}
	for _, seg := range segs {
		if seq >= seg.FromSeq && (seg.ToSeq == 0 || seq <= seg.ToSeq) {
			return true
		}
	}
	return false
}

// Replay is isolated from live state and live dispatch.
type Replay struct {
	mu      sync.Mutex
	id      string
	session string
	branch  string
	speed   float64
	paused  bool
	skip    bool
	source  *ReplaySource
	reducer *state.Reducer
	pending *ReplayMessage
	drained bool
	cursor  int64
	stop    chan struct{}
}

func NewReplay(id, sessionID, branchID string, speed float64, skip bool, src *ReplaySource) *Replay {
	return &Replay{id: id, session: sessionID, branch: branchID, speed: speed, skip: skip, paused: true, source: src, reducer: state.New("replay:"+id, nil)}
}

func (r *Replay) ID() string { return r.id }

func (r *Replay) apply() {
	if !r.pull() {
		return
	}
	m := *r.pending
	p, _ := protocol.Decode(m.Data)
	r.reducer.Apply(m.Seq, time.UnixMilli(m.ReceivedAt), m.Data, p, "")
	r.cursor = m.ReceivedAt
	r.pending = nil
}

// pull loads the next datagram without consuming it.
func (r *Replay) pull() bool {
	if r.pending != nil {
		return true
	}
	if r.drained {
		return false
	}
	m, ok := r.source.nextMessage()
	if !ok {
		r.drained = true
		return false
	}
	r.pending = &m
	return true
}

func (r *Replay) View() contract.Replay {
	r.mu.Lock()
	defer r.mu.Unlock()
	snap := r.reducer.Snapshot(nil, nil)
	t := 0
	if snap.SessionTimeMs != nil {
		t = int(*snap.SessionTimeMs)
	}
	return contract.Replay{ID: r.id, SessionID: r.session, BranchID: r.branch, SourceID: "replay:" + r.id, Speed: r.speed, Paused: r.paused, PlayheadSessionTimeMs: t, SkipLongPauses: r.skip}
}

// Seek rebuilds state from the beginning up to the requested session time.
func (r *Replay) SeekTo(sessionTimeMs int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reducer = state.New("replay:"+r.id, nil)
	r.source.rewind()
	r.pending = nil
	r.drained = false
	r.cursor = 0
	for r.pull() {
		r.apply()
		s := r.reducer.Snapshot(nil, nil)
		if s.SessionTimeMs != nil && *s.SessionTimeMs >= sessionTimeMs {
			return
		}
	}
}

func (r *Replay) Step(n int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := 0; i < n && r.pull(); i++ {
		r.apply()
	}
}

func (r *Replay) Patch(p contract.ReplayPatch) {
	r.mu.Lock()
	speed, paused := r.speed, r.paused
	if p.Speed != nil && *p.Speed >= 0.25 && *p.Speed <= 8 {
		speed = *p.Speed
	}
	if p.Paused != nil {
		paused = *p.Paused
	}
	r.speed, r.paused = speed, paused
	r.mu.Unlock()
	if p.PositionMs != nil {
		r.SeekTo(int64(*p.PositionMs))
	}
	if paused {
		r.stopClock()
	} else {
		r.mu.Lock()
		if r.cursor == 0 && r.pull() {
			r.cursor = r.pending.ReceivedAt - 1
		}
		r.mu.Unlock()
		r.startClock()
	}
}

// startClock advances the playhead at the requested speed. Replay time follows
// the recorded receive timestamps, so a paused game stays paused instead of
// racing through a quiet stretch.
func (r *Replay) startClock() {
	r.mu.Lock()
	if r.stop != nil {
		r.mu.Unlock()
		return
	}
	stop := make(chan struct{})
	r.stop = stop
	r.mu.Unlock()
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		last := time.Now()
		for {
			select {
			case <-stop:
				return
			case now := <-ticker.C:
				stepMs := float64(now.Sub(last).Milliseconds()) * r.Speed()
				last = now
				if stepMs <= 0 {
					continue
				}
				r.advance(stepMs)
			}
		}
	}()
}

func (r *Replay) stopClock() {
	r.mu.Lock()
	if r.stop != nil {
		close(r.stop)
		r.stop = nil
	}
	r.mu.Unlock()
}

func (r *Replay) Speed() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.speed
}

// advance consumes messages whose recorded time falls inside the elapsed
// window. Large recorded gaps are skipped only when the user asked for it.
func (r *Replay) advance(stepMs float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.paused || !r.pull() {
		return
	}
	target := r.cursor + int64(stepMs)
	for r.pull() {
		next := r.pending.ReceivedAt
		if next > target {
			break
		}
		if r.skip && next-r.cursor > 2000 {
			// Jump past a recorded pause (formation, red flag) instead of
			// waiting it out at playback speed.
			r.cursor = next - 1
		}
		r.apply()
	}
	if r.cursor < target {
		r.cursor = target
	}
}

// Close stops playback and releases the goroutine.
func (r *Replay) Close() {
	r.stopClock()
	r.mu.Lock()
	r.source.close()
	r.mu.Unlock()
}
