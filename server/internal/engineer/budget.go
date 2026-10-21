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
	inputUSD  *float64
	outputUSD *float64
	requests  []time.Time
	tokens    []tokenSpend
	costs     []costSpend
	reservedR int
	reservedT int64
	reservedC float64
	blocked   bool
	reason    string
}

// Reservation is the token and dollar estimate held while a provider call runs.
type Reservation struct {
	Tokens  int64
	CostUSD float64
}

type tokenSpend struct {
	at     time.Time
	tokens int64
}

type costSpend struct {
	at  time.Time
	usd float64
}

func NewBudget(cfg contract.AgentConfig, now func() time.Time) *Budget {
	if now == nil {
		now = time.Now
	}
	return &Budget{cfg: cfg, now: now}
}

// Configure applies the latest agent limits and endpoint prices.
func (b *Budget) Configure(cfg contract.AgentConfig, endpoint *contract.Endpoint) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cfg = cfg
	b.inputUSD, b.outputUSD = nil, nil
	if endpoint != nil {
		b.inputUSD, b.outputUSD = endpoint.InputUSDPerMillion, endpoint.OutputUSDPerMillion
	}
}

func tokenCost(tokens int64, price *float64) (float64, bool) {
	if price == nil {
		return 0, false
	}
	return float64(tokens) / 1_000_000 * *price, true
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
	k := 0
	for k < len(b.costs) && b.costs[k].at.Before(cut) {
		k++
	}
	b.costs = b.costs[k:]
}

// Reserve blocks when the request, token or configured cost ceiling is hit.
func (b *Budget) Reserve(inputEstimate int64, maxOutput int) (Reservation, bool, string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	t := b.now()
	b.pruneLocked(t)
	cost := inputEstimate + int64(maxOutput)
	if len(b.requests)+b.reservedR+1 > b.cfg.MaxCallsPerHour {
		b.blocked, b.reason = true, "provider request ceiling reached"
		return Reservation{}, false, b.reason
	}
	var used int64
	for _, s := range b.tokens {
		used += s.tokens
	}
	if used+int64(b.reservedT)+cost > int64(b.cfg.MaxTokensPerHour) {
		b.blocked, b.reason = true, "token ceiling reached"
		return Reservation{}, false, b.reason
	}
	estUSD := 0.0
	if b.cfg.MaxCostUSDPerHour != nil {
		in, inOK := tokenCost(inputEstimate, b.inputUSD)
		out, outOK := tokenCost(int64(maxOutput), b.outputUSD)
		if !inOK || !outOK {
			b.blocked, b.reason = true, "cost ceiling requires endpoint prices"
			return Reservation{}, false, b.reason
		}
		estUSD = in + out
		usedUSD := 0.0
		for _, s := range b.costs {
			usedUSD += s.usd
		}
		if usedUSD+b.reservedC+estUSD > *b.cfg.MaxCostUSDPerHour {
			b.blocked, b.reason = true, "cost ceiling reached"
			return Reservation{}, false, b.reason
		}
	}
	b.reservedR++
	b.reservedT += cost
	b.reservedC += estUSD
	b.blocked, b.reason = false, ""
	return Reservation{Tokens: cost, CostUSD: estUSD}, true, ""
}

// Reconcile replaces the reservation with actual usage. Unknown usage keeps the
// full reservation, which is the conservative reading.
func (b *Budget) Reconcile(reserved Reservation, usage contract.Usage) {
	b.mu.Lock()
	defer b.mu.Unlock()
	t := b.now()
	b.reservedR--
	if b.reservedR < 0 {
		b.reservedR = 0
	}
	b.reservedT -= reserved.Tokens
	if b.reservedT < 0 {
		b.reservedT = 0
	}
	b.reservedC -= reserved.CostUSD
	if b.reservedC < 0 {
		b.reservedC = 0
	}
	n := usage.ProviderRequests
	if n < 1 {
		n = 1
	}
	for i := 0; i < n; i++ {
		b.requests = append(b.requests, t)
	}
	actual := reserved.Tokens
	if usage.TotalTokens != nil {
		actual = *usage.TotalTokens
	}
	b.tokens = append(b.tokens, tokenSpend{at: t, tokens: actual})
	actualUSD := reserved.CostUSD
	if usage.CostUSD != nil {
		actualUSD = *usage.CostUSD
	} else if usage.InputTokens != nil && usage.OutputTokens != nil {
		if in, ok := tokenCost(*usage.InputTokens, b.inputUSD); ok {
			if out, ok := tokenCost(*usage.OutputTokens, b.outputUSD); ok {
				actualUSD = in + out
			}
		}
	}
	b.costs = append(b.costs, costSpend{at: t, usd: actualUSD})
}

// Release returns a reservation that produced no provider request.
func (b *Budget) Release(reserved Reservation) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.reservedR--
	if b.reservedR < 0 {
		b.reservedR = 0
	}
	b.reservedT -= reserved.Tokens
	if b.reservedT < 0 {
		b.reservedT = 0
	}
	b.reservedC -= reserved.CostUSD
	if b.reservedC < 0 {
		b.reservedC = 0
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

// Available reports whether a new reservation could pass every configured ceiling.
func (b *Budget) Available() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pruneLocked(b.now())
	if len(b.requests)+b.reservedR >= b.cfg.MaxCallsPerHour {
		return false
	}
	var used int64
	for _, s := range b.tokens {
		used += s.tokens
	}
	if used+b.reservedT >= int64(b.cfg.MaxTokensPerHour) {
		return false
	}
	if b.cfg.MaxCostUSDPerHour != nil {
		usedUSD := b.reservedC
		for _, s := range b.costs {
			usedUSD += s.usd
		}
		if usedUSD >= *b.cfg.MaxCostUSDPerHour {
			return false
		}
	}
	return true
}

func (b *Budget) Blocked() (bool, string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.blocked, b.reason
}
