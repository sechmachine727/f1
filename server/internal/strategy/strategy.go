// Package strategy owns deterministic estimates and the driver notification arbiter.
package strategy

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"f1tele/server/internal/analysis"
	"f1tele/server/internal/contract"
	"f1tele/server/internal/protocol"
	"f1tele/server/internal/state"
)

const MethodVersion = "strategy-linear-v1"

var ErrUnsupportedMode = errors.New("strategy candidates are unsupported in this mode")

type Evaluator struct {
	Analysis *analysis.Engine
	Config   contract.Config
	window   *EnergyWindow
}

func NewEvaluator(a *analysis.Engine, cfg contract.Config) *Evaluator {
	return &Evaluator{Analysis: a, Config: cfg, window: &EnergyWindow{}}
}

// Observe feeds the rolling energy window from live snapshots.
func (e *Evaluator) Observe(snap state.Snapshot) { e.window.observe(snap, time.Now()) }

// Evaluate computes stay-out and legal pit alternatives. Missing inputs stay
// unavailable; no numeric gap is invented.
func (e *Evaluator) Evaluate(ctx context.Context, q contract.StrategyQuery, snap state.Snapshot) (contract.StrategyEvaluation, error) {
	mode := snap.Context.ModeFamily
	if mode == "time_trial" {
		return contract.StrategyEvaluation{}, ErrUnsupportedMode
	}
	laps, err := e.laps(ctx, q.SessionID, q.BranchID, q.SessionID)
	if err != nil {
		return contract.StrategyEvaluation{}, err
	}
	car := 0
	if snap.Context.PlayerCarIndex != nil {
		car = *snap.Context.PlayerCarIndex
	}
	clean := cleanLaps(laps, car)
	pace, paceOK := medianPace(clean, 5)
	deg, degOK := degradation(clean)
	est := []contract.Estimate{}
	limitations := []string{}
	if paceOK {
		est = append(est, contract.Estimate{ID: "pace", Kind: "pace", Measurement: contract.Derived(pace, "ms", "median-clean-5-v1"), Assumptions: []string{"clean laps only"}, EvidenceIDs: lapEvidence(clean), MethodVersion: "median-clean-5-v1"})
	} else {
		limitations = append(limitations, "pace unavailable: fewer than one clean lap")
	}
	if degOK {
		est = append(est, contract.Estimate{ID: "degradation", Kind: "degradation", Measurement: contract.Derived(deg, "ms/lap", "within-stint-linear-v1"), Assumptions: []string{"linear within-stint model"}, MethodVersion: "within-stint-linear-v1"})
	} else {
		limitations = append(limitations, "degradation unavailable: fewer than three eligible laps in one stint")
	}
	if v := metricValue(snap, "fuel.finish_margin_laps", car); v != nil {
		est = append(est, contract.Estimate{ID: "fuel_finish", Kind: "fuel_finish", Measurement: contract.Available(*v, "laps", "game.fuelRemainingLaps"), MethodVersion: "game-reported-v1"})
	} else {
		limitations = append(limitations, "fuel finish margin unavailable from the game")
	}
	if rate, ok := e.energyRate(snap); ok {
		est = append(est, contract.Estimate{ID: "energy_draw", Kind: "energy_draw", Measurement: contract.Derived(rate, "W", "energy-10s-window-v1"), Assumptions: []string{"draw derived from store energy over a valid window"}, MethodVersion: "energy-10s-window-v1"})
	} else {
		limitations = append(limitations, "energy draw unavailable: no valid ten-second window")
	}
	currentLap := 1
	if snap.Lap != nil {
		currentLap = *snap.Lap
	}
	totalLaps := 0
	if v, ok := snap.Session["total_laps"]; ok {
		totalLaps = int(v)
	}
	candidates := []contract.StrategyCandidate{}
	stay := contract.StrategyCandidate{ID: "stay-out", Stops: []contract.PlannedStop{}, Legality: "legal", Reasons: []string{"no stop"}, Assumptions: []string{"remain on current tyres"}, MethodVersion: MethodVersion}
	stay.PitLossMs = contract.Available(0.0, "ms", "strategy.stay-out")
	stay.RejoinPosition = contract.Unavailable("unsupported", "position", "strategy.stay-out", "stay-out does not rejoin")
	if paceOK && totalLaps > currentLap {
		remaining := float64(totalLaps - currentLap)
		pred := pace * remaining
		if degOK {
			pred += deg * remaining * (remaining - 1) / 2
		}
		stay.TotalTimeMs = contract.Derived(pred, "ms", "linear-remaining-v1")
		stay.ErrorBandMs = floatPtr(pred * 0.03)
	} else {
		stay.TotalTimeMs = contract.Unavailable("not_received", "ms", "strategy.stay-out", "pace or remaining laps unavailable")
	}
	candidates = append(candidates, stay)
	pitLoss, pitLossOK := e.pitLoss(snap.Context)
	if !pitLossOK {
		limitations = append(limitations, "pit loss unavailable: no observed pass or matching track profile")
	}
	for i, w := range q.PitWindows {
		if len(candidates) >= 16 {
			limitations = append(limitations, "candidate list capped at 16 alternatives")
			break
		}
		stop := w.FirstLap
		if stop < currentLap {
			stop = currentLap
		}
		cand := contract.StrategyCandidate{
			ID: "pit-" + strconv.Itoa(i+1), Stops: []contract.PlannedStop{{Lap: stop, Compound: w.Compound}},
			Legality: "conditional", MethodVersion: MethodVersion,
		}
		if pitLossOK {
			cand.PitLossMs = contract.Derived(pitLoss, "ms", "track-profile")
			if paceOK {
				remaining := float64(totalLaps - currentLap)
				pred := pace*remaining + pitLoss + 0.0
				cand.TotalTimeMs = contract.Derived(pred, "ms", "linear-remaining-v1")
			} else {
				cand.TotalTimeMs = contract.Unavailable("not_received", "ms", "strategy.pit", "pace unavailable")
			}
		} else {
			cand.PitLossMs = contract.Unavailable("not_received", "ms", "strategy.pit", "no observed or sourced pit loss")
			cand.TotalTimeMs = contract.Unavailable("not_received", "ms", "strategy.pit", "pit loss unavailable")
			cand.Reasons = append(cand.Reasons, "missing pit-loss evidence")
		}
		if w.Compound == "" {
			cand.Legality = "illegal"
			cand.Reasons = append(cand.Reasons, "no compound supplied for the stop")
		} else if !compoundAvailable(e.Config, snap.Context, w.Compound) {
			cand.Legality = "conditional"
			cand.Reasons = append(cand.Reasons, "compound availability is unknown; treat the stop as conditional")
			limitations = append(limitations, "tyre allocation is not published by the live stream for this session")
		}
		if totalLaps > 0 && (w.LastLap < currentLap || w.FirstLap > totalLaps) {
			cand.Legality = "illegal"
			cand.Reasons = append(cand.Reasons, "pit window is outside the remaining race")
		}
		if snap.Session["safety_car_status"] > 0 {
			cand.Assumptions = append(cand.Assumptions, "a neutralization is active; pit loss will differ from green-flag running")
			limitations = append(limitations, "pit loss under neutralization is not modelled")
		}
		cand.RejoinPosition = contract.Unavailable("not_received", "position", "strategy.pit", "rejoin position needs observed pit loss and gaps")
		cand.Assumptions = []string{"linear pace model", "pit loss from an editable sourced track profile"}
		candidates = append(candidates, cand)
	}
	for i := 0; i < len(q.PitWindows); i++ {
		for j := i + 1; j < len(q.PitWindows); j++ {
			if len(candidates) >= 16 {
				break
			}
			a, b := q.PitWindows[i], q.PitWindows[j]
			if b.FirstLap < currentLap || a.FirstLap < currentLap || a.Compound == "" || b.Compound == "" {
				continue
			}
			cand := contract.StrategyCandidate{
				ID:       "pit2-" + strconv.Itoa(i+1) + "-" + strconv.Itoa(j+1),
				Stops:    []contract.PlannedStop{{Lap: a.FirstLap, Compound: a.Compound}, {Lap: b.FirstLap, Compound: b.Compound}},
				Legality: "conditional", MethodVersion: MethodVersion,
				Assumptions: []string{"linear pace model", "two stops use both supplied windows"},
			}
			if pitLossOK && paceOK {
				remaining := float64(totalLaps - currentLap)
				cand.PitLossMs = contract.Derived(2*pitLoss, "ms", "track-profile")
				cand.TotalTimeMs = contract.Derived(pace*remaining+2*pitLoss, "ms", "linear-remaining-v1")
			} else {
				cand.PitLossMs = contract.Unavailable("not_received", "ms", "strategy.pit", "no observed or sourced pit loss")
				cand.TotalTimeMs = contract.Unavailable("not_received", "ms", "strategy.pit", "pit loss or pace unavailable")
			}
			cand.RejoinPosition = contract.Unavailable("not_received", "position", "strategy.pit", "rejoin needs observed pit loss and gaps")
			candidates = append(candidates, cand)
		}
	}
	return contract.StrategyEvaluation{Context: snap.Context, ObservationSeq: strconv.FormatUint(snap.ObservationSeq, 10), Candidates: candidates, Estimates: est, Limitations: limitations}, nil
}

func (e *Evaluator) laps(ctx context.Context, sessionID, branchID, _ string) ([]contract.LapSummary, error) {
	if e.Analysis == nil {
		return nil, errors.New("analysis engine unavailable")
	}
	sum, laps, err := e.Analysis.Summaries(ctx, sessionID, branchID, 0)
	_ = sum
	return laps, err
}

func (e *Evaluator) pitLoss(cx contract.Context) (float64, bool) {
	if cx.TrackID == nil {
		return 0, false
	}
	for _, p := range e.Config.TrackProfiles {
		if !p.Enabled || p.TrackID != *cx.TrackID || p.PitLossMs == nil {
			continue
		}
		if !seasonMatches(p.RegulationSeasons, cx.RegulationSeason) {
			continue
		}
		return *p.PitLossMs, true
	}
	return 0, false
}

func seasonMatches(seasons []int, season *int) bool {
	if len(seasons) == 0 {
		return true
	}
	if season == nil {
		return false
	}
	for _, s := range seasons {
		if s == *season {
			return true
		}
	}
	return false
}

func compoundAvailable(cfg contract.Config, cx contract.Context, compound string) bool {
	return compound != ""
}

func cleanLaps(laps []contract.LapSummary, car int) []contract.LapSummary {
	var out []contract.LapSummary
	for _, l := range laps {
		if l.CarIndex != car || !l.Valid || !l.Complete || l.PitLap || l.Neutralized {
			continue
		}
		if v, ok := num(l.LapTimeMs.Value); ok && v > 0 {
			out = append(out, l)
		}
	}
	return out
}

func medianPace(laps []contract.LapSummary, n int) (float64, bool) {
	if len(laps) == 0 {
		return 0, false
	}
	start := 0
	if len(laps) > n {
		start = len(laps) - n
	}
	var xs []float64
	for _, l := range laps[start:] {
		if v, ok := num(l.LapTimeMs.Value); ok {
			xs = append(xs, v)
		}
	}
	if len(xs) == 0 {
		return 0, false
	}
	sort.Float64s(xs)
	return xs[len(xs)/2], true
}

func degradation(laps []contract.LapSummary) (float64, bool) {
	// Stints are contiguous runs of one compound; a slope across a compound
	// change would describe the stop, not tyre wear.
	var best float64
	bestOK := false
	var run []contract.LapSummary
	flush := func() {
		if len(run) < 3 {
			return
		}
		xs := make([]float64, len(run))
		ys := make([]float64, len(run))
		for i, l := range run {
			v, ok := num(l.LapTimeMs.Value)
			if !ok {
				return
			}
			xs[i], ys[i] = float64(i), v
		}
		var mx, my float64
		for i := range xs {
			mx += xs[i]
			my += ys[i]
		}
		mx /= float64(len(xs))
		my /= float64(len(ys))
		var n, d float64
		for i := range xs {
			n += (xs[i] - mx) * (ys[i] - my)
			d += (xs[i] - mx) * (xs[i] - mx)
		}
		if d == 0 {
			return
		}
		best, bestOK = n/d, true
	}
	for _, l := range laps {
		if len(run) > 0 && measurementText(l.Compound) != measurementText(run[0].Compound) {
			flush()
			run = nil
		}
		run = append(run, l)
	}
	flush()
	return best, bestOK
}

func lapEvidence(laps []contract.LapSummary) []string {
	var out []string
	for _, l := range laps {
		out = append(out, "lap:"+l.ID)
	}
	return out
}

func metricValue(snap state.Snapshot, channel string, car int) *float64 {
	for _, m := range snap.Metrics {
		if m.Channel == channel && m.CarIndex != nil && *m.CarIndex == car && m.Measurement.Availability == "available" {
			if v, ok := num(m.Measurement.Value); ok {
				return &v
			}
		}
	}
	return nil
}

func num(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int64:
		return float64(n), true
	case int:
		return float64(n), true
	}
	return 0, false
}

func floatPtr(v float64) *float64 { return &v }

// EnergyWindow tracks store energy for the ten-second draw estimate.
type EnergyWindow struct {
	mu   sync.Mutex
	last map[int]energySample
}

type energySample struct {
	at    time.Time
	store float64
	ok    bool
}

func (w *EnergyWindow) observe(snap state.Snapshot, now time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.last == nil {
		w.last = map[int]energySample{}
	}
	for _, m := range snap.Metrics {
		if m.Channel != "energy.store_j" || m.CarIndex == nil {
			continue
		}
		v, ok := num(m.Measurement.Value)
		if !ok {
			continue
		}
		prev := w.last[*m.CarIndex]
		if prev.ok && now.Sub(prev.at) < 500*time.Millisecond {
			continue
		}
		w.last[*m.CarIndex] = energySample{at: now, store: v, ok: true}
	}
}

func (e *Evaluator) energyRate(snap state.Snapshot) (float64, bool) {
	if e.window == nil {
		return 0, false
	}
	e.window.mu.Lock()
	defer e.window.mu.Unlock()
	car := 0
	if snap.Context.PlayerCarIndex != nil {
		car = *snap.Context.PlayerCarIndex
	}
	prev, ok := e.window.last[car]
	if !ok {
		return 0, false
	}
	cur := metricValue(snap, "energy.store_j", car)
	if cur == nil {
		return 0, false
	}
	dt := time.Since(prev.at).Seconds()
	if dt <= 0 || dt > 10 {
		return 0, false
	}
	return (prev.store - *cur) / dt, true
}

var _ = strings.TrimSpace
var _ = fmt.Sprintf
var _ = math.Abs
var _ = protocol.Lookup

func measurementText(m contract.Measurement) string {
	if m.Availability != "available" {
		return m.Availability
	}
	if s, ok := m.Value.(string); ok {
		return s
	}
	return fmt.Sprint(m.Value)
}
