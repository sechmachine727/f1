package engineer

import (
	"testing"
	"time"

	"f1tele/server/internal/contract"
)

func TestInvocationLimits(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	cfg := contract.AgentConfig{MaxCallsPerHour: 3, MaxTokensPerHour: 1000}
	b := NewBudget(cfg, func() time.Time { return now })
	for i := 0; i < 3; i++ {
		res, ok, _ := b.Reserve(100, 200)
		if !ok {
			t.Fatalf("request %d rejected", i)
		}
		b.Reconcile(res, contract.Usage{ProviderRequests: 1})
	}
	if _, ok, reason := b.Reserve(10, 10); ok || reason == "" {
		t.Fatal("request ceiling not enforced")
	}
	// Unknown usage reserves the full estimate: token ceiling engages.
	now = now.Add(61 * time.Minute)
	b2 := NewBudget(cfg, func() time.Time { return now })
	var n int
	for i := 0; i < 10; i++ {
		res, ok, _ := b2.Reserve(300, 100)
		if !ok {
			n = i
			break
		}
		b2.Reconcile(res, contract.Usage{})
	}
	if n == 0 {
		t.Fatal("token ceiling never engaged with unknown usage")
	}
}

func TestContinuousStopsOnce(t *testing.T) {
	c := &Continuous{}
	now := time.Unix(1_700_000_000, 0)
	c.Enable(10*time.Second, 15*time.Minute, now)
	if !c.Tick(now.Add(14 * time.Minute)) {
		t.Fatal("stopped early")
	}
	if c.Tick(now.Add(15 * time.Minute)) {
		t.Fatal("still enabled after 15 minutes")
	}
	if c.Tick(now.Add(16 * time.Minute)) {
		t.Fatal("restarted automatically")
	}
}
