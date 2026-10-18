package engineer

import (
	"sync"
	"time"

	"f1tele/server/internal/contract"
)

// Budget enforces rolling 60-minute provider-request and token ceilings.
// Unknown usage consumes the reservation; the caller reconciles actual usage.
type Budget struct {
	mu        sync.Mutex
	now       func() time.Time
	cfg       contract.AgentConfig
	requests  []time.Time
	tokens    []tokenSpend
	reservedR int
	reservedT int64
	blocked   bool
	reason    string
}

type tokenSpend struct {
	at     time.Time
	tokens int64
}

func NewBudget(cfg contract.AgentConfig, now func() time.Time) *Budget {
	if now == nil {
		now = time.Now
	}
	return &Budget{cfg: cfg, now: now}
}

func (b *Budget) pruneLocked(t time.Time) {
	cut := t.Add(-time.Hour)
	i := 0
	for i < len(b.requests) && b.requests[i].Before(cut) {
		i++
	}
	b.requests = b.requests[i:]
	j := 0
	for j < len(b.tokens) && b.tokens[j].at.Before(cut) {
		j++
	}
	b.tokens = b.tokens[j:]
}

// Reserve blocks when the request, token or configured cost ceiling is hit.
func (b *Budget) Reserve(inputEstimate int64, maxOutput int) (int64, bool, string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	t := b.now()
	b.pruneLocked(t)
	cost := inputEstimate + int64(maxOutput)
	if len(b.requests)+b.reservedR+1 > b.cfg.MaxCallsPerHour {
		b.blocked, b.reason = true, "provider request ceiling reached"
		return 0, false, b.reason
	}
	var used int64
	for _, s := range b.tokens {
		used += s.tokens
	}
	if used+int64(b.reservedT)+cost > int64(b.cfg.MaxTokensPerHour) {
		b.blocked, b.reason = true, "token ceiling reached"
		return 0, false, b.reason
	}
	b.reservedR++
	b.reservedT += cost
	b.blocked, b.reason = false, ""
	return cost, true, ""
}

// Reconcile replaces the reservation with actual usage. Unknown usage keeps the
// full reservation, which is the conservative reading.
func (b *Budget) Reconcile(reserved int64, usage contract.Usage) {
	b.mu.Lock()
	defer b.mu.Unlock()
	t := b.now()
	b.reservedR--
	if b.reservedR < 0 {
		b.reservedR = 0
	}
	b.reservedT -= reserved
	if b.reservedT < 0 {
		b.reservedT = 0
	}
	b.requests = append(b.requests, t)
	actual := reserved
	if usage.TotalTokens != nil {
		actual = *usage.TotalTokens
	}
	b.tokens = append(b.tokens, tokenSpend{at: t, tokens: actual})
}

// Release returns a reservation that produced no provider request.
func (b *Budget) Release(reserved int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.reservedR--
	if b.reservedR < 0 {
		b.reservedR = 0
	}
	b.reservedT -= reserved
	if b.reservedT < 0 {
		b.reservedT = 0
	}
}

func (b *Budget) Remaining() (requests int, tokens int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pruneLocked(b.now())
	var used int64
	for _, s := range b.tokens {
		used += s.tokens
	}
	requests = b.cfg.MaxCallsPerHour - len(b.requests) - b.reservedR
	tokens = int64(b.cfg.MaxTokensPerHour) - used - b.reservedT
	return
}

func (b *Budget) Blocked() (bool, string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.blocked, b.reason
}
