package analysis

import (
	"context"
	"errors"
	"sort"

	"f1tele/server/internal/contract"
)

// PaceSummary is the deterministic pace/trend view used by charts and prompts.
type PaceSummary struct {
	Pace        contract.Measurement
	LapCount    int
	Degradation contract.Measurement
	Notes       []string
}

func (e *Engine) Summaries(ctx context.Context, sessionID, branchID string, car int) (PaceSummary, []contract.LapSummary, error) {
	laps, err := e.Catalog.Laps(ctx, sessionID, branchID, &car)
	if err != nil {
		return PaceSummary{}, nil, err
	}
	out := PaceSummary{Notes: []string{}}
	out.Pace = contract.Unavailable("not_received", "ms", "analysis.pace", "no clean laps")
	out.Degradation = contract.Unavailable("not_received", "ms/lap", "analysis.degradation", "fewer than three eligible laps")
	var clean []contract.LapSummary
	var times []float64
	for _, l := range laps {
		if l.Valid && l.Complete && !l.PitLap && !l.Neutralized {
			if v, ok := number(l.LapTimeMs.Value); ok && v > 0 {
				clean = append(clean, l)
				times = append(times, v)
			}
		}
	}
	if len(times) > 0 {
		last := times
		if len(last) > 5 {
			last = last[len(last)-5:]
		}
		sorted := append([]float64(nil), last...)
		sort.Float64s(sorted)
		med := sorted[len(sorted)/2]
		out.Pace = contract.Derived(med, "ms", "median-clean-5-v1")
		out.LapCount = len(last)
		out.Notes = append(out.Notes, "pace uses the median of the last five clean eligible laps")
	}
	var fit, xs []float64
	runStart := 0
	for i := 1; i <= len(clean); i++ {
		if i == len(clean) || measurementText(clean[i].Compound) != measurementText(clean[runStart].Compound) {
			run := clean[runStart:i]
			if len(run) >= 3 {
				for j, l := range run {
					if v, ok := number(l.LapTimeMs.Value); ok {
						fit = append(fit, v)
						xs = append(xs, float64(j))
					}
				}
			}
			runStart = i
		}
	}
	if len(fit) >= 3 {
		if slope, err := linearSlope(xs, fit); err == nil {
			out.Degradation = contract.Derived(slope, "ms/lap", "within-stint-linear-v1")
			out.Notes = append(out.Notes, "degradation is a linear within-stint fit and is not calibrated across fuel or track evolution")
		}
	}
	return out, laps, nil
}

func linearSlope(xs, ys []float64) (float64, error) {
	var mx, my float64
	for i := range xs {
		mx += xs[i]
		my += ys[i]
	}
	mx /= float64(len(xs))
	my /= float64(len(ys))
	var num, den float64
	for i := range xs {
		num += (xs[i] - mx) * (ys[i] - my)
		den += (xs[i] - mx) * (xs[i] - mx)
	}
	if den == 0 {
		return 0, errors.New("zero variance")
	}
	return num / den, nil
}
