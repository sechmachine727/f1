package engineer

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"f1tele/server/internal/contract"
	"f1tele/server/internal/history"
	"f1tele/server/internal/state"
)

type lapKey struct {
	car int
	lap int
}

// recordedSource scopes post-race analysis to catalogued laps and samples
// instead of the live reducer.
func (m *Manager) recordedSource(ctx context.Context, req contract.Request) (state.Snapshot, contract.Observation, error) {
	detail, err := m.Catalog.Session(ctx, req.SessionID)
	if err != nil {
		if errors.Is(err, history.ErrNotFound) {
			return state.Snapshot{}, contract.Observation{}, fmt.Errorf("recorded session %q not found", req.SessionID)
		}
		return state.Snapshot{}, contract.Observation{}, err
	}
	branch := req.BranchID
	if branch == "" {
		branch = detail.SurvivingBranchID
	} else if !knownBranch(detail.Branches, branch) {
		return state.Snapshot{}, contract.Observation{}, fmt.Errorf("branch %q is not part of session %q", branch, req.SessionID)
	}
	if branch == "" {
		return state.Snapshot{}, contract.Observation{}, fmt.Errorf("session %q has no branch", req.SessionID)
	}
	laps, err := m.Catalog.Laps(ctx, req.SessionID, branch, nil)
	if err != nil {
		return state.Snapshot{}, contract.Observation{}, err
	}
	byKey := make(map[lapKey]contract.LapSummary, len(laps))
	for _, l := range laps {
		byKey[lapKey{car: l.CarIndex, lap: l.Lap}] = l
	}
	keys, err := selectedLaps(req.RequestText)
	if err != nil {
		return state.Snapshot{}, contract.Observation{}, err
	}
	selected := make([]contract.LapSummary, 0, len(keys))
	for _, k := range keys {
		l, ok := byKey[k]
		if !ok {
			return state.Snapshot{}, contract.Observation{}, fmt.Errorf("lap %d:%d is not catalogued in session %q", k.car, k.lap, req.SessionID)
		}
		selected = append(selected, l)
	}
	if len(selected) == 0 {
		selected = recentLaps(laps, 5)
	}
	metrics := []contract.Metric{}
	gaps := []string{}
	for _, l := range selected {
		metrics = append(metrics, lapMetrics(l)...)
		if l.LapTimeMs.Availability != "available" {
			gaps = append(gaps, fmt.Sprintf("car %d lap %d has no stored lap time", l.CarIndex, l.Lap))
		}
	}
	samples := map[lapKey]history.LapSamples{}
	if len(selected) > 0 {
		cars := make([]int, 0, len(selected))
		seen := map[int]bool{}
		from, to := selected[0].Lap, selected[0].Lap
		for _, l := range selected {
			if !seen[l.CarIndex] {
				seen[l.CarIndex] = true
				cars = append(cars, l.CarIndex)
			}
			if l.Lap < from {
				from = l.Lap
			}
			if l.Lap > to {
				to = l.Lap
			}
		}
		rows, _, err := m.Catalog.SessionLapSamples(ctx, req.SessionID, branch, cars, from, to)
		if err != nil {
			return state.Snapshot{}, contract.Observation{}, err
		}
		for _, s := range rows {
			samples[lapKey{car: s.CarIndex, lap: s.Lap}] = s
		}
	}
	for _, l := range selected {
		s, ok := samples[lapKey{car: l.CarIndex, lap: l.Lap}]
		if !ok {
			gaps = append(gaps, fmt.Sprintf("car %d lap %d has no stored samples", l.CarIndex, l.Lap))
			continue
		}
		metrics = append(metrics, sampleMetrics(s)...)
	}
	var lap *int
	for _, l := range selected {
		if lap == nil || l.Lap > *lap {
			v := l.Lap
			lap = &v
		}
	}
	cx := detail.Context
	cx.SessionID = req.SessionID
	cx.BranchID = branch
	obs := contract.Observation{
		SchemaVersion: contract.SchemaVersion, Context: cx, ObservationSeq: "0",
		CapturedAtMs: time.Now().UnixMilli(), Lap: lap, Window: contract.Range{FromSeq: "0", ToSeq: "0"},
		Metrics: metrics, Events: []contract.Event{}, Estimates: []contract.Estimate{},
		Coverage: contract.Coverage{Gaps: gaps, StaleChannels: []string{}},
	}
	snap := state.Snapshot{Context: cx, ObservationSeq: 0, Metrics: metrics, Events: []contract.Event{},
		Lap: lap, Session: map[string]float64{}}
	return snap, obs, nil
}

func knownBranch(branches []contract.Branch, id string) bool {
	for _, b := range branches {
		if b.ID == id {
			return true
		}
	}
	return false
}

// selectedLaps reads the frontend's "Selected laps: car:lap, ..." suffix.
func selectedLaps(text *string) ([]lapKey, error) {
	if text == nil {
		return nil, nil
	}
	i := strings.LastIndex(*text, "Selected laps:")
	if i < 0 {
		return nil, nil
	}
	raw := strings.TrimSpace((*text)[i+len("Selected laps:"):])
	if raw == "" || raw == "none" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	if len(parts) > 4 {
		return nil, errors.New("select at most four laps")
	}
	seen := map[lapKey]bool{}
	out := make([]lapKey, 0, len(parts))
	for _, p := range parts {
		car, lapText, ok := strings.Cut(strings.TrimSpace(p), ":")
		if !ok {
			return nil, fmt.Errorf("invalid selected lap %q", strings.TrimSpace(p))
		}
		carN, err := strconv.Atoi(strings.TrimSpace(car))
		if err != nil || carN < 0 {
			return nil, fmt.Errorf("invalid selected car %q", car)
		}
		lapN, err := strconv.Atoi(strings.TrimSpace(lapText))
		if err != nil || lapN < 1 {
			return nil, fmt.Errorf("invalid selected lap %q", lapText)
		}
		k := lapKey{car: carN, lap: lapN}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	return out, nil
}

func recentLaps(laps []contract.LapSummary, limit int) []contract.LapSummary {
	complete := make([]contract.LapSummary, 0, len(laps))
	for _, l := range laps {
		if l.Complete {
			complete = append(complete, l)
		}
	}
	sort.SliceStable(complete, func(i, j int) bool { return complete[i].Lap > complete[j].Lap })
	if len(complete) > limit {
		complete = complete[:limit]
	}
	for i, j := 0, len(complete)-1; i < j; i, j = i+1, j-1 {
		complete[i], complete[j] = complete[j], complete[i]
	}
	return complete
}

func lapMetrics(l contract.LapSummary) []contract.Metric {
	car := l.CarIndex
	id := fmt.Sprintf("lap:%d:%d", l.CarIndex, l.Lap)
	out := []contract.Metric{{ID: id, CarIndex: &car, Channel: "analysis.lap",
		Measurement: contract.Available(lapState(l), "state", "catalog.laps")}}
	if l.LapTimeMs.Availability == "available" {
		out = append(out, contract.Metric{ID: id + ":time", CarIndex: &car, Channel: "analysis.lap_time_ms", Measurement: l.LapTimeMs})
	}
	if l.Compound.Availability == "available" {
		out = append(out, contract.Metric{ID: id + ":compound", CarIndex: &car, Channel: "tyre.compound", Measurement: l.Compound})
	}
	return out
}

func lapState(l contract.LapSummary) string {
	state := "complete"
	if !l.Complete {
		state = "incomplete"
	}
	if !l.Valid {
		state += ",invalid"
	}
	if l.PitLap {
		state += ",pit"
	}
	if l.Neutralized {
		state += ",neutralized"
	}
	return state
}

func sampleMetrics(s history.LapSamples) []contract.Metric {
	car := s.CarIndex
	id := fmt.Sprintf("samples:%d:%d", s.CarIndex, s.Lap)
	out := []contract.Metric{{ID: id + ":points", CarIndex: &car, Channel: "analysis.sample_points",
		Measurement: contract.Available(s.PointCount, "count", "catalog.lap_samples")}}
	var throttle, brake, ers float64
	var throttleN, brakeN, ersN int
	for _, p := range s.Points {
		if len(p) < 5 {
			continue
		}
		if v, ok := numberValue(p[1]); ok {
			throttle += v
			throttleN++
		}
		if v, ok := numberValue(p[2]); ok {
			brake += v
			brakeN++
		}
		if v, ok := numberValue(p[4]); ok {
			ers += v
			ersN++
		}
	}
	if throttleN > 0 {
		out = append(out, contract.Metric{ID: id + ":throttle", CarIndex: &car, Channel: "analysis.avg_throttle",
			Measurement: contract.Available(throttle/float64(throttleN), "fraction", "catalog.lap_samples")})
	}
	if brakeN > 0 {
		out = append(out, contract.Metric{ID: id + ":brake", CarIndex: &car, Channel: "analysis.brake_fraction",
			Measurement: contract.Available(brake/float64(brakeN), "fraction", "catalog.lap_samples")})
	}
	if ersN > 0 {
		out = append(out, contract.Metric{ID: id + ":ers", CarIndex: &car, Channel: "analysis.avg_ers_j",
			Measurement: contract.Available(ers/float64(ersN), "J", "catalog.lap_samples")})
	}
	return out
}

func numberValue(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}
