package analytics

import (
	"math"
	"sort"
)

// tuple renders one stored sample. Restricted and never-received energy values
// stay null so a chart can show a gap instead of a false zero.
func (s sample) tuple() []any {
	var ers, mode, overtakeOK, overtakeOn any
	if !s.restricted && s.haveErs {
		ers = math.Round(s.ersJ*10) / 10
		mode = s.deployMode
	}
	if s.haveOvertake {
		overtakeOK, overtakeOn = s.overtakeOK, s.overtakeOn
	}
	return []any{
		s.timeMs,
		math.Round(s.throttle*1000) / 1000,
		math.Round(s.brake*1000) / 1000,
		s.gear,
		ers,
		mode,
		overtakeOK,
		overtakeOn,
	}
}

// compactSamples halves the live buffer while keeping the first and last
// sample and any index where gear or brake state changed. The stored trace is
// still reduced to MaxSamplesPerLap at lap close.
func compactSamples(in []sample) []sample {
	if len(in) < 3 {
		return in
	}
	out := make([]sample, 0, len(in)/2+8)
	for i, s := range in {
		changed := i > 0 && (s.gear != in[i-1].gear || (s.brake > 0) != (in[i-1].brake > 0))
		if i == 0 || i == len(in)-1 || changed || i%2 == 0 {
			out = append(out, s)
		}
	}
	return out
}

// decimateSamples bounds a lap trace while keeping the first and last sample
// and every gear or brake-state change, so a full-race view does not lose
// shifts and braking points to a plain stride.
func decimateSamples(in []sample, max int) [][]any {
	if len(in) == 0 || max <= 0 {
		return [][]any{}
	}
	if len(in) <= max {
		out := make([][]any, len(in))
		for i, s := range in {
			out[i] = s.tuple()
		}
		return out
	}
	if max <= 2 {
		// No room for interior detail; keep the two ends only.
		out := [][]any{in[0].tuple(), in[len(in)-1].tuple()}
		return out[:max]
	}
	keep := map[int]bool{0: true, len(in) - 1: true}
	for i := 1; i < len(in); i++ {
		if in[i].gear != in[i-1].gear || (in[i].brake > 0) != (in[i-1].brake > 0) {
			keep[i] = true
		}
	}
	idx := make([]int, 0, len(keep))
	for i := range keep {
		idx = append(idx, i)
	}
	sort.Ints(idx)
	if len(idx) > max {
		// Only a pathological lap produces more changes than the bound; keep
		// the ends and resample the protected set evenly without duplicates.
		trimmed := make([]int, 0, max)
		for i := 0; i < max; i++ {
			trimmed = append(trimmed, idx[int(math.Round(float64(i)*float64(len(idx)-1)/float64(max-1)))])
		}
		idx = trimmed
	}
	if len(idx) < max {
		rest := make([]int, 0, len(in)-len(idx))
		for i := 0; i < len(in); i++ {
			if !keep[i] {
				rest = append(rest, i)
			}
		}
		need := max - len(idx)
		if need > len(rest) {
			need = len(rest)
		}
		for i := 0; i < need; i++ {
			idx = append(idx, rest[int(float64(i)*float64(len(rest))/float64(need))])
		}
		sort.Ints(idx)
	}
	out := make([][]any, len(idx))
	for i, at := range idx {
		out[i] = in[at].tuple()
	}
	return out
}

