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
}

type ReplayMessage struct {
	Seq        uint64
	ReceivedAt int64
	Data       []byte
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
	index   int
}

func NewReplay(id, sessionID, branchID string, speed float64, skip bool, src *ReplaySource) *Replay {
	return &Replay{id: id, session: sessionID, branch: branchID, speed: speed, skip: skip, paused: true, source: src, reducer: state.New("replay:"+id, nil)}
}

func (r *Replay) ID() string { return r.id }

func (r *Replay) apply() {
	m := r.source.Messages[r.index]
	p, _ := protocol.Decode(m.Data)
	r.reducer.Apply(m.Seq, time.UnixMilli(m.ReceivedAt), m.Data, p, "")
	r.index++
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
	r.index = 0
	for r.index < len(r.source.Messages) {
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
	for i := 0; i < n && r.index < len(r.source.Messages); i++ {
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
}
