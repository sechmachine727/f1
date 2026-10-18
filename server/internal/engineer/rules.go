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
	manual map[string]bool
}

func NewRules(cfg []contract.TriggerRule) *Rules {
	return &Rules{cfg: cfg, sat: map[string]map[string]bool{}, laps: map[string]int{}, last: map[string]time.Time{}, manual: map[string]bool{}}
}

func (r *Rules) Trigger(eventTypes []string, laps int) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, rule := range r.cfg {
		if !rule.Enabled {
			continue
		}
		if r.eval(rule.ID, "0", rule.Predicate, eventTypes, laps) {
			r.reset(rule.ID)
			out = append(out, rule.ID)
		}
	}
	return out
}

func (r *Rules) eval(id, path string, p contract.Predicate, events []string, laps int) bool {
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
				if e == want {
					r.sat[id][key+":"+e] = true
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
