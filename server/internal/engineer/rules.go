package engineer

import (
	"sync"
	"time"

	"f1tele/server/internal/contract"
)

// Rules evaluates typed trigger predicates. ANY fires when one leaf is
// satisfied; ALL latches its leaves until the rule dispatches.
type Rules struct {
	mu     sync.Mutex
	cfg    []contract.TriggerRule
	sat    map[string]map[string]bool
	laps   map[string]int
	last   map[string]time.Time
	cool   map[string]time.Time
	manual map[string]bool
	now    func() time.Time
}

func NewRules(cfg []contract.TriggerRule) *Rules {
	return &Rules{cfg: cfg, sat: map[string]map[string]bool{}, laps: map[string]int{}, last: map[string]time.Time{},
		cool: map[string]time.Time{}, manual: map[string]bool{}, now: time.Now}
}

// TriggerContext carries the live scope needed to apply rule filters.
type TriggerContext struct {
	ModeFamily string
	PlayerCar  int
	Events     []PendingEvent
	Laps       int
}

func (r *Rules) Trigger(eventTypes []string, laps int) []string {
	events := make([]PendingEvent, 0, len(eventTypes))
	for _, t := range eventTypes {
		events = append(events, PendingEvent{Type: t, Severity: "critical"})
	}
	return r.TriggerWithContext(TriggerContext{Events: events, Laps: laps, PlayerCar: -1})
}

func (r *Rules) TriggerWithContext(in TriggerContext) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	now := r.now()
	for _, rule := range r.cfg {
		if !rule.Enabled {
			continue
		}
		if !r.modeAllowed(rule, in.ModeFamily) {
			continue
		}
		if last, ok := r.cool[rule.ID]; ok && rule.CooldownSeconds > 0 && now.Sub(last) < time.Duration(rule.CooldownSeconds)*time.Second {
			continue
		}
		events := r.filterEvents(rule, in.Events, in.PlayerCar)
		if r.eval(rule.ID, "0", rule.Predicate, events, in.Laps) {
			r.reset(rule.ID)
			r.cool[rule.ID] = now
			out = append(out, rule.ID)
		}
	}
	return out
}

// Configure applies edited rules immediately and clears latches for the old set.
func (r *Rules) Configure(cfg []contract.TriggerRule) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cfg = cfg
	r.clearLocked()
}

// Reset clears per-rule latches when the logical session changes.
func (r *Rules) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clearLocked()
}

func (r *Rules) clearLocked() {
	r.sat = map[string]map[string]bool{}
	r.laps = map[string]int{}
	r.last = map[string]time.Time{}
	r.cool = map[string]time.Time{}
	r.manual = map[string]bool{}
}

func (r *Rules) modeAllowed(rule contract.TriggerRule, mode string) bool {
	if len(rule.Filters.ModeFamilies) == 0 {
		return true
	}
	for _, m := range rule.Filters.ModeFamilies {
		if m == mode {
			return true
		}
	}
	return false
}

func (r *Rules) filterEvents(rule contract.TriggerRule, events []PendingEvent, player int) []PendingEvent {
	// Live state only emits events from validated packets, so validity=valid is
	// already satisfied. selected is the player car in the live context.
	if rule.Filters.Validity == "invalid" {
		return nil
	}
	min := severityRank(rule.Filters.MinimumSeverity)
	out := make([]PendingEvent, 0, len(events))
	for _, e := range events {
		if severityRank(e.Severity) < min {
			continue
		}
		if (rule.Filters.CarScope == "player" || rule.Filters.CarScope == "selected") && e.CarIndex != nil && *e.CarIndex != player {
			continue
		}
		out = append(out, e)
	}
	return out
}

func severityRank(s string) int {
	switch s {
	case "critical":
		return 3
	case "important":
		return 2
	case "routine":
		return 1
	default:
		return 0
	}
}

func (r *Rules) eval(id, path string, p contract.Predicate, events []PendingEvent, laps int) bool {
	key := id + "/" + path
	if r.sat[id] == nil {
		r.sat[id] = map[string]bool{}
	}
	switch p.Kind {
	case "manual":
		if r.manual[id] {
			delete(r.manual, id)
			return true
		}
		return false
	case "lap":
		n := 1
		if p.EveryCompletedLaps != nil {
			n = *p.EveryCompletedLaps
		}
		r.laps[id] += laps
		if r.laps[id] >= n {
			r.laps[id] = 0
			r.sat[id][key] = true
		}
		return r.sat[id][key]
	case "interval":
		if p.Seconds == nil {
			return false
		}
		now := time.Now()
		if last, ok := r.last[id]; ok && now.Sub(last) < time.Duration(*p.Seconds)*time.Second {
			return false
		}
		r.last[id] = now
		return true
	case "event":
		for _, e := range events {
			for _, want := range p.EventTypes {
				if e.Type == want {
					r.sat[id][key+":"+e.Type] = true
				}
			}
		}
		for _, want := range p.EventTypes {
			if !r.sat[id][key+":"+want] {
				return false
			}
		}
		return true
	case "any":
		for i, ch := range p.Children {
			if r.eval(id, path+"."+string(rune('a'+i)), ch, events, laps) {
				return true
			}
		}
		return false
	case "all":
		ok := true
		for i, ch := range p.Children {
			if !r.eval(id, path+"."+string(rune('a'+i)), ch, events, laps) {
				ok = false
			}
		}
		return ok
	}
	return false
}

func (r *Rules) reset(id string) {
	for k := range r.sat[id] {
		delete(r.sat[id], k)
	}
	r.laps[id] = 0
}

func (r *Rules) Manual(id string) { r.mu.Lock(); r.manual[id] = true; r.mu.Unlock() }
