// Package engineer owns bounded model scheduling, exact prompt assembly and output acceptance.
package engineer

import (
	"errors"
	"fmt"

	"sync"
	"time"

	"f1tele/server/internal/contract"
)

type PendingEvent struct {
	Seq      uint64
	ID       string
	Type     string
	Severity string
	CarIndex *int
}

type Scope struct {
	Branch         string
	DispatchCursor uint64
	EvidenceCursor *uint64
	Previous       *contract.AgentOutput
	LastAttempt    *contract.Attempt
	ProviderState  any
	Events         []PendingEvent
	LastInherited  *uint64
	BranchReset    bool
}

// DispatchQuestion runs a manual question. It preempts a scheduled run and
// never advances the strategy evidence cursor.
func (s *Scheduler) DispatchQuestion(id string, through uint64, now time.Time) (Invocation, error) {
	s.mu.Lock()
	if s.active != nil {
		s.active.Status = "cancelled"
		s.active = nil
	}
	s.mu.Unlock()
	return s.Dispatch(id, through, now)
}

// NewSession clears all cursors, accepted advice and opaque continuation.
func (s *Scheduler) NewSession(branch string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scope = Scope{Branch: branch}
	s.active = nil
	s.inFlight = map[string]*Invocation{}
	s.lastStart = time.Time{}
}

// SetDispatchGap applies the live minimum gap between AI dispatches.
func (s *Scheduler) SetDispatchGap(gap time.Duration) {
	s.mu.Lock()
	s.minGap = gap
	s.mu.Unlock()
}

// ChangeEndpoint clears opaque provider state and keeps portable accepted advice.
func (s *Scheduler) ChangeEndpoint() {
	s.mu.Lock()
	s.scope.ProviderState = nil
	s.mu.Unlock()
}

// Continuous expires at its configured ceiling and never restarts itself.
type Continuous struct {
	Enabled bool
	Started time.Time
	Max     time.Duration
	Cadence time.Duration
}

func (c *Continuous) Enable(cadence, max time.Duration, now time.Time) {
	c.Enabled = true
	c.Started = now
	c.Cadence = cadence
	c.Max = max
}

func (c *Continuous) Tick(now time.Time) bool {
	if c.Enabled && now.Sub(c.Started) >= c.Max {
		c.Enabled = false
	}
	return c.Enabled
}

type Invocation struct {
	ID     string
	Branch string
	Scope  Scope
	Status string
	Start  time.Time
	Window uint64
}

// Scheduler is the software-owned dispatch/evidence state machine. It never
// consumes pending evidence on failure, cancellation or supersession.
type Scheduler struct {
	mu               sync.Mutex
	scope            Scope
	inFlight         map[string]*Invocation
	active           *Invocation
	minGap           time.Duration
	lastStart        time.Time
	ProviderRequests int
	TokensUsed       int
	BudgetBlocked    bool
}

func NewScheduler(branch string, dispatchCursor uint64) *Scheduler {
	return &Scheduler{scope: Scope{Branch: branch, DispatchCursor: dispatchCursor}, inFlight: map[string]*Invocation{}}
}

func (s *Scheduler) Scope() Scope {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.scope
	c.Events = append([]PendingEvent(nil), s.scope.Events...)
	return c
}

func (s *Scheduler) Event(e PendingEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scope.Events = append(s.scope.Events, e)
}

// Dispatch freezes the input window and advances the scheduling cursor.
func (s *Scheduler) Dispatch(id string, through uint64, now time.Time) (Invocation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.BudgetBlocked {
		return Invocation{}, errors.New("budget blocked")
	}
	if s.active != nil {
		return Invocation{}, errors.New("an invocation is already running")
	}
	if s.minGap > 0 && !s.lastStart.IsZero() && now.Sub(s.lastStart) < s.minGap {
		return Invocation{}, errors.New("minimum dispatch gap")
	}
	if through < s.scope.DispatchCursor {
		through = s.scope.DispatchCursor
	}
	inv := Invocation{ID: id, Branch: s.scope.Branch, Scope: s.scope, Status: "running", Start: now, Window: through}
	inv.Scope.Events = nil
	for _, e := range s.scope.Events {
		if e.Seq <= through {
			inv.Scope.Events = append(inv.Scope.Events, e)
		}
	}
	s.scope.DispatchCursor = through
	s.lastStart = now
	s.active = &inv
	s.inFlight[id] = &inv
	return inv, nil
}

func (s *Scheduler) Cancel(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	inv, ok := s.inFlight[id]
	if !ok {
		return
	}
	inv.Status = "cancelled"
	if s.active != nil && s.active.ID == id {
		s.active = nil
	}
}

// Accept advances evidence only for a current-branch strategy result.
func (s *Scheduler) Accept(id string, through uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	inv, ok := s.inFlight[id]
	if !ok {
		return errors.New("unknown invocation")
	}
	if inv.Branch != s.scope.Branch {
		inv.Status = "superseded"
		return errors.New("superseded by branch change")
	}
	if inv.Status == "cancelled" {
		return errors.New("cancelled invocation cannot be accepted")
	}
	if inv.Status != "running" {
		return fmt.Errorf("invocation is %s", inv.Status)
	}
	inv.Status = "accepted"
	s.scope.EvidenceCursor = uint64Ptr(through)
	s.scope.LastAttempt = &contract.Attempt{InvocationID: id, Status: "accepted", EndedAtMs: time.Now().UnixMilli()}
	// Consume evidence represented by this window only.
	var left []PendingEvent
	for _, e := range s.scope.Events {
		if e.Seq > through {
			left = append(left, e)
		}
	}
	s.scope.Events = left
	if s.active != nil && s.active.ID == id {
		s.active = nil
	}
	return nil
}

func (s *Scheduler) Fail(id, code string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	inv, ok := s.inFlight[id]
	if !ok {
		return
	}
	inv.Status = "failed"
	s.scope.LastAttempt = &contract.Attempt{InvocationID: id, Status: "failed", EndedAtMs: time.Now().UnixMilli(), ErrorCode: &code}
	if s.active != nil && s.active.ID == id {
		s.active = nil
	}
}

func (s *Scheduler) BudgetBlockedNow(v bool) {
	s.mu.Lock()
	s.BudgetBlocked = v
	s.mu.Unlock()
}

func (s *Scheduler) IsBudgetBlocked() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.BudgetBlocked
}

// Flashback switches branches: parent evidence before the anchor is retained,
// child cursors and provider continuation are cleared.
func (s *Scheduler) Flashback(branch string, anchor uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, inv := range s.inFlight {
		if inv.Status == "running" {
			inv.Status = "superseded"
		}
	}
	inherited := anchor - 1
	keep := []PendingEvent{}
	for _, e := range s.scope.Events {
		if e.Seq < anchor {
			keep = append(keep, e)
		}
	}
	s.scope = Scope{Branch: branch, Events: keep, LastInherited: &inherited, Previous: nil, EvidenceCursor: nil, ProviderState: nil, BranchReset: true}
	s.active = nil
	s.lastStart = time.Time{}
}

func (s *Scheduler) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scope = Scope{Branch: s.scope.Branch}
	s.active = nil
	s.lastStart = time.Time{}
}

func (s *Scheduler) CountRequest(tokens int) {
	s.mu.Lock()
	s.ProviderRequests++
	s.TokensUsed += tokens
	s.mu.Unlock()
}

func (s *Scheduler) RequestCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ProviderRequests
}

func uint64Ptr(v uint64) *uint64 { return &v }
