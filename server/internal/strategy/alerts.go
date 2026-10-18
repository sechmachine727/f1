package strategy

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"f1tele/server/internal/contract"
	"f1tele/server/internal/state"
)

type Notification struct {
	AlertID  string
	Text     string
	Critical bool
	Spoken   bool
	Reason   string
}

type condition struct {
	id         string
	severity   string
	since      time.Time
	lastSeen   time.Time
	lastActive time.Time
	belowSince time.Time
	quoted     string
	alert      contract.Alert
	acked      bool
	announced  bool
	message    string
}

// Arbiter is the single notification authority: persistence, hysteresis,
// deduplication, acknowledgment and the routine gap live here, independent of AI.
type Arbiter struct {
	mu          sync.Mutex
	cfg         contract.NotificationConfig
	profiles    []contract.ReferenceProfile
	capacities  []contract.CapacityProfile
	conditions  map[string]*condition
	lastRoutine time.Time
	now         func() time.Time
}

func NewArbiter(cfg contract.NotificationConfig, profiles []contract.ReferenceProfile, capacities []contract.CapacityProfile) *Arbiter {
	return &Arbiter{cfg: cfg, profiles: profiles, capacities: capacities, conditions: map[string]*condition{}, now: time.Now}
}

// Acknowledge silences the current condition until it escalates.
func (a *Arbiter) Acknowledge(id string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	id = trimAlertPrefix(id)
	c, ok := a.conditions[id]
	if !ok {
		return false
	}
	c.acked = true
	c.announced = true
	c.alert.Acknowledged = true
	return true
}

func trimAlertPrefix(id string) string {
	if len(id) > 6 && id[:6] == "alert-" {
		return id[6:]
	}
	return id
}

func (a *Arbiter) Alerts() []contract.Alert {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]contract.Alert, 0, len(a.conditions))
	for _, c := range a.conditions {
		out = append(out, c.alert)
	}
	return out
}

// Observe evaluates deterministic conditions and returns at most one spoken notification.
func (a *Arbiter) Observe(snap state.Snapshot) []Notification {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := a.now()
	seen := map[string]observed{}
	car := 0
	if snap.Context.PlayerCarIndex != nil {
		car = *snap.Context.PlayerCarIndex
	}
	for _, ch := range []string{"tyre.fl.wear_pct", "tyre.fr.wear_pct", "tyre.rl.wear_pct", "tyre.rr.wear_pct"} {
		if v, ok := metric(snap, ch, car); ok {
			seen[ch] = observed{value: v, label: cornerLabel(ch), kind: "wear"}
		}
	}
	for _, ch := range []string{"tyre.fl.surface_c", "tyre.fr.surface_c", "tyre.rl.surface_c", "tyre.rr.surface_c"} {
		if v, ok := metric(snap, ch, car); ok {
			seen[ch] = observed{value: v, label: cornerLabel(ch), kind: "surface"}
		}
	}
	if v, ok := metric(snap, "fuel.finish_margin_laps", car); ok {
		seen["fuel.finish_margin_laps"] = observed{value: v, label: "Fuel finish margin", kind: "fuel"}
	}
	if v, ok := metric(snap, "energy.store_j", car); ok {
		seen["energy.store_j"] = observed{value: v, label: "Energy store", kind: "energy"}
	}
	var notes []Notification
	for id, o := range seen {
		sev, profileID, msg := a.classify(id, o, snap)
		if sev == "" {
			// Inside the band: start the clear window once, and keep it running
			// even if calm samples keep arriving.
			if c, ok := a.conditions[id]; ok && c.belowSince.IsZero() {
				c.belowSince = now
			}
			continue
		}
		c, ok := a.conditions[id]
		if !ok {
			c = &condition{id: id, since: now}
			a.conditions[id] = c
		}
		escalated := severityRank(sev) > severityRank(c.severity)
		c.severity = sev
		c.lastSeen = now
		c.lastActive = now
		c.alert = contract.Alert{
			ID: "alert-" + id, ConditionID: id, SessionID: snap.Context.SessionID, BranchID: snap.Context.BranchID,
			Severity: sev, State: "active", Summary: msg, EvidenceIDs: []string{fmt.Sprintf("seq:%d", snap.ObservationSeq)},
			FirstSeenMs: c.since.UnixMilli(), LastSeenMs: now.UnixMilli(), Acknowledged: c.acked, ReferenceProfileID: profileID,
		}
		required := time.Duration(a.cfg.CautionPersistenceMs) * time.Millisecond
		if sev == "critical" {
			required = time.Duration(a.cfg.CriticalPersistenceMs) * time.Millisecond
		}
		if now.Sub(c.since) < required {
			continue
		}
		if c.acked && !escalated {
			continue
		}
		if sev != "critical" && now.Sub(a.lastRoutine) < time.Duration(a.cfg.RoutineGapSeconds)*time.Second {
			notes = append(notes, Notification{AlertID: c.alert.ID, Text: msg, Reason: "routine gap"})
			continue
		}
		if c.announced && !escalated {
			continue
		}
		c.announced = true
		c.message = msg
		notes = append(notes, Notification{AlertID: c.alert.ID, Text: msg, Critical: sev == "critical", Spoken: true})
		if sev != "critical" {
			a.lastRoutine = now
		}
	}
	// Clear conditions that have been unclassified or unseen for the clear
	// window. Hysteresis keeps a condition active while it stays above the
	// threshold, and does not downgrade until the window elapses.
	for id, c := range a.conditions {
		window := time.Duration(a.cfg.ClearPersistenceMs) * time.Millisecond
		switch {
		case !c.belowSince.IsZero() && now.Sub(c.belowSince) >= window:
		case c.belowSince.IsZero():
			if _, seenNow := seen[id]; seenNow {
				continue
			}
			if now.Sub(c.lastSeen) < window {
				continue
			}
		default:
			continue
		}
		c.alert.State = "cleared"
		delete(a.conditions, id)
		notes = append(notes, Notification{AlertID: c.alert.ID, Text: c.message + " cleared", Reason: "cleared"})
	}
	// Group simultaneous tyre conditions into one message so the driver does
	// not hear one sentence per corner.
	return groupNotes(notes)
}

type observed struct {
	value float64
	label string
	kind  string
}

func (a *Arbiter) classify(id string, o observed, snap state.Snapshot) (string, *string, string) {
	switch o.kind {
	case "wear", "surface":
		profile := a.profileFor(id, snap)
		if profile == nil {
			return "", nil, ""
		}
		sev := classifyValue(o.value, profile)
		if sev == "" {
			return "", nil, ""
		}
		unit := profile.Unit
		return sev, &profile.ID, fmt.Sprintf("%s %s %.0f %s", o.label, sevWord(sev), o.value, unit)
	case "fuel":
		if o.value < 0 {
			return "critical", nil, fmt.Sprintf("Fuel finish margin %.1f laps: projected shortfall", o.value)
		}
		if o.value < 0.5 {
			return "important", nil, fmt.Sprintf("Fuel finish margin %.1f laps", o.value)
		}
	case "energy":
		if p := a.capacityProfile(snap); p != nil {
			soc := o.value / p.CapacityJ * 100
			if soc < 10 {
				return "critical", &p.ID, fmt.Sprintf("Energy store %.0f%% of sourced capacity", soc)
			}
			if soc < 20 {
				return "important", &p.ID, fmt.Sprintf("Energy store %.0f%% of sourced capacity", soc)
			}
		}
	}
	return "", nil, ""
}

func (a *Arbiter) profileFor(channel string, snap state.Snapshot) *contract.ReferenceProfile {
	compound := ""
	for _, m := range snap.Metrics {
		if m.Channel == "tyre.compound" && m.CarIndex != nil && snap.Context.PlayerCarIndex != nil && *m.CarIndex == *snap.Context.PlayerCarIndex {
			compound = fmt.Sprint(m.Measurement.Value)
		}
	}
	for i := range a.profiles {
		p := &a.profiles[i]
		if !p.Enabled || !contains(p.Channels, channel) {
			continue
		}
		if !seasonMatches(p.RegulationSeasons, snap.Context.RegulationSeason) {
			continue
		}
		if snap.Context.FormulaCode != nil && len(p.FormulaCodes) > 0 && !containsInt(p.FormulaCodes, *snap.Context.FormulaCode) {
			continue
		}
		if compound != "" && len(p.ActualCompounds) > 0 && !contains(p.ActualCompounds, compound) {
			continue
		}
		return p
	}
	return nil
}

func (a *Arbiter) capacityProfile(snap state.Snapshot) *contract.CapacityProfile {
	for i := range a.capacities {
		p := &a.capacities[i]
		if !p.Enabled || !seasonMatches(p.RegulationSeasons, snap.Context.RegulationSeason) {
			continue
		}
		if snap.Context.FormulaCode != nil && len(p.FormulaCodes) > 0 && !containsInt(p.FormulaCodes, *snap.Context.FormulaCode) {
			continue
		}
		return p
	}
	return nil
}

// classifyValue uses critical first, then caution, then cold/target.
func classifyValue(v float64, p *contract.ReferenceProfile) string {
	if p.CriticalMin != nil && v >= *p.CriticalMin {
		return "critical"
	}
	if p.CautionMin != nil && v >= *p.CautionMin {
		return "important"
	}
	if p.TargetLow != nil && p.TargetHigh != nil && v >= *p.TargetLow && v <= *p.TargetHigh {
		return ""
	}
	if p.ColdBelow != nil && v < *p.ColdBelow {
		return ""
	}
	return ""
}

func sevWord(s string) string {
	switch s {
	case "critical":
		return "critical"
	case "important":
		return "caution"
	}
	return "in range"
}

func severityRank(s string) int {
	switch s {
	case "critical":
		return 3
	case "important":
		return 2
	case "routine":
		return 1
	}
	return 0
}

func cornerLabel(ch string) string {
	switch {
	case contains2(ch, "fl."):
		return "Front left"
	case contains2(ch, "fr."):
		return "Front right"
	case contains2(ch, "rl."):
		return "Rear left"
	case contains2(ch, "rr."):
		return "Rear right"
	}
	return ch
}

func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func containsInt(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func contains2(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func metric(snap state.Snapshot, channel string, car int) (float64, bool) {
	for _, m := range snap.Metrics {
		if m.Channel != channel || m.CarIndex == nil || *m.CarIndex != car || m.Measurement.Availability != "available" {
			continue
		}
		return num(m.Measurement.Value)
	}
	return 0, false
}

// groupNotes merges tyre notifications of the same severity into one sentence.
func groupNotes(in []Notification) []Notification {
	bySeverity := map[string][]Notification{}
	var order []string
	var out []Notification
	for _, n := range in {
		if !strings.HasPrefix(n.AlertID, "alert-tyre.") || !n.Spoken {
			out = append(out, n)
			continue
		}
		sev := "routine"
		if n.Critical {
			sev = "critical"
		}
		if _, ok := bySeverity[sev]; !ok {
			order = append(order, sev)
		}
		bySeverity[sev] = append(bySeverity[sev], n)
	}
	for _, sev := range order {
		group := bySeverity[sev]
		if len(group) == 1 {
			out = append(out, group[0])
			continue
		}
		var parts []string
		for _, n := range group {
			parts = append(parts, strings.TrimPrefix(n.Text, "Tyre "))
		}
		out = append(out, Notification{AlertID: group[0].AlertID, Text: "Tyres: " + strings.Join(parts, " · "), Critical: group[0].Critical, Spoken: true})
	}
	return out
}
