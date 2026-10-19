package analysis

import (
	"context"
	"errors"
	"sort"
	"strconv"

	"f1tele/server/internal/contract"
	"f1tele/server/internal/protocol"

	"github.com/foxglove/mcap/go/mcap"
)

// PaceSummary is the deterministic pace/trend view used by charts and prompts.
type PaceSummary struct {
	Pace             contract.Measurement
	LapCount         int
	Degradation      contract.Measurement
	ObservedTopSpeed contract.Measurement
	TrapBest         contract.Measurement
	TrapBestLap      *int
	LastCleanAverage contract.Measurement
	CornerAverage    contract.Measurement
	Notes            []string
}

func (e *Engine) Summaries(ctx context.Context, sessionID, branchID string, car int) (PaceSummary, []contract.LapSummary, error) {
	laps, err := e.Catalog.Laps(ctx, sessionID, branchID, &car)
	if err != nil {
		return PaceSummary{}, nil, err
	}
	checkpoint := e.summaryCheckpoint(ctx, sessionID, laps)
	if cached, ok := e.cachedSummary(sessionID, branchID, car, checkpoint); ok {
		return cached, laps, nil
	}
	out := PaceSummary{Notes: []string{}, TrapBestLap: nil}
	out.Pace = contract.Unavailable("not_received", "ms", "analysis.pace", "no clean laps")
	out.Degradation = contract.Unavailable("not_received", "ms/lap", "analysis.degradation", "fewer than three eligible laps")
	out.ObservedTopSpeed = contract.Unavailable("not_received", "m/s", "analysis.top_speed", "no speed samples")
	out.TrapBest = contract.Unavailable("not_received", "m/s", "analysis.trap", "no speed-trap samples")
	out.LastCleanAverage = contract.Unavailable("not_received", "m/s", "analysis.lap_average", "no clean lap with distance and time")
	out.CornerAverage = contract.Unavailable("unsupported", "m/s", "analysis.corner", "no validated corner entry and exit zones")
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
	e.speedSummary(ctx, sessionID, branchID, car, clean, &out)
	e.storeSummary(sessionID, branchID, car, checkpoint, out)
	return out, laps, nil
}

func (e *Engine) summaryCheckpoint(ctx context.Context, sessionID string, laps []contract.LapSummary) string {
	last := uint64(0)
	for _, l := range laps {
		if v, err := strconv.ParseUint(l.EndSeq, 10, 64); err == nil && v > last {
			last = v
		}
	}
	if ranges, err := e.Catalog.RecordingRanges(ctx, sessionID); err == nil {
		for _, r := range ranges {
			if r.LastSeq > last {
				last = r.LastSeq
			}
		}
	}
	return strconv.Itoa(len(laps)) + ":" + strconv.FormatUint(last, 10)
}

func (e *Engine) cachedSummary(sessionID, branchID string, car int, checkpoint string) (PaceSummary, bool) {
	e.summaryMu.Lock()
	defer e.summaryMu.Unlock()
	entry, ok := e.summaries[summaryKey(sessionID, branchID, car)]
	if !ok || entry.checkpoint != checkpoint {
		return PaceSummary{}, false
	}
	return entry.out, true
}

func (e *Engine) storeSummary(sessionID, branchID string, car int, checkpoint string, out PaceSummary) {
	e.summaryMu.Lock()
	defer e.summaryMu.Unlock()
	if e.summaries == nil {
		e.summaries = map[string]summaryEntry{}
	}
	e.summaries[summaryKey(sessionID, branchID, car)] = summaryEntry{checkpoint: checkpoint, out: out}
}

func summaryKey(sessionID, branchID string, car int) string {
	return sessionID + "|" + branchID + "|" + strconv.Itoa(car)
}

// speedSummary scans the surviving branch once for observed top speed, the
// packet-2 trap best and the last clean lap's distance.
func (e *Engine) speedSummary(ctx context.Context, sessionID, branchID string, car int, clean []contract.LapSummary, out *PaceSummary) {
	paths, err := e.Catalog.RecordingsForSession(ctx, sessionID)
	if err != nil || len(paths) == 0 {
		return
	}
	segs, err := e.Catalog.InheritedSegments(ctx, branchID)
	if err != nil {
		return
	}
	var last *contract.LapSummary
	if len(clean) > 0 {
		l := clean[len(clean)-1]
		last = &l
	}
	from, to := uint64(0), uint64(0)
	if last != nil {
		from, _ = strconv.ParseUint(last.StartSeq, 10, 64)
		to, _ = strconv.ParseUint(last.EndSeq, 10, 64)
	}
	var top, trap float64
	var trapLap int
	var totalFirst, totalLast, lapAccum, prevLapDist float64
	var haveTrap, haveTotal, haveLap bool
	speedSpec, _ := protocol.Lookup("car.speed_mps")
	trapSpec, _ := protocol.Lookup("timing.speed_trap_mps")
	err = scanFiles(ctx, paths, func(m *mcap.Message) error {
		if !inSegments(uint64(m.Sequence), segs) || len(m.Data) < 7 {
			return nil
		}
		pid := m.Data[6]
		if pid != 2 && pid != 6 {
			return nil
		}
		p, err := protocol.Decode(m.Data)
		if err != nil {
			return nil
		}
		if pid == 6 {
			if v, ok := p.CarField("m_speed", car); ok {
				s := v.F * speedSpec.Scale
				if s > top {
					top = s
				}
			}
			return nil
		}
		if v, ok := p.CarField("m_speedTrapFastestSpeed", car); ok {
			s := v.F * trapSpec.Scale
			if s > trap {
				trap = s
				haveTrap = true
			}
		}
		if v, ok := p.CarField("m_speedTrapFastestLap", car); ok && v.F >= 1 {
			trapLap = int(v.F)
		}
		seq := uint64(m.Sequence)
		if last == nil || seq < from || seq > to {
			return nil
		}
		if v, ok := p.CarField("m_totalDistance", car); ok {
			if !haveTotal {
				totalFirst, haveTotal = v.F, true
			}
			totalLast = v.F
		}
		if v, ok := p.CarField("m_lapDistance", car); ok {
			if !haveLap {
				prevLapDist, haveLap = v.F, true
			} else if v.F >= prevLapDist {
				lapAccum += v.F - prevLapDist
				prevLapDist = v.F
			} else {
				lapAccum += v.F
				prevLapDist = v.F
			}
		}
		return nil
	})
	if err != nil {
		return
	}
	if top > 0 {
		out.ObservedTopSpeed = contract.Derived(top, "m/s", "observed-max-v1")
	}
	if haveTrap && trap > 0 {
		out.TrapBest = contract.Derived(trap, "m/s", "packet2-trap-v1")
		if trapLap > 0 {
			l := trapLap
			out.TrapBestLap = &l
		}
	}
	if last != nil {
		distance := lapAccum
		if haveTotal && totalLast-totalFirst > 0 {
			distance = totalLast - totalFirst
		}
		if ms, ok := number(last.LapTimeMs.Value); ok && ms > 0 && distance > 0 {
			out.LastCleanAverage = contract.Derived(distance/(ms/1000), "m/s", "lap-distance-time-v1")
			out.Notes = append(out.Notes, "last clean lap average uses recorded distance over lap time")
		}
	}
}

// summaryCache is per-engine and keyed by the raw lap checkpoint, branch and car.
type summaryEntry struct {
	checkpoint string
	out        PaceSummary
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
