package analysis

import (
	"context"
	"fmt"
	"math"
	"strconv"

	"f1tele/server/internal/contract"
	"f1tele/server/internal/protocol"

	"github.com/foxglove/mcap/go/mcap"
)

func (e *Engine) Compare(ctx context.Context, q contract.CompareQuery) (contract.AnalysisResult, error) {
	if len(q.LapIDs) < 2 || len(q.LapIDs) > 4 {
		return contract.AnalysisResult{}, ErrLapCount
	}
	if len(q.Channels) == 0 || len(q.Channels) > 16 {
		return contract.AnalysisResult{}, ErrTooManyChannels
	}
	all, err := e.Catalog.Laps(ctx, q.SessionID, q.BranchID, nil)
	if err != nil {
		return contract.AnalysisResult{}, err
	}
	byID := map[string]contract.LapSummary{}
	for _, l := range all {
		byID[l.ID] = l
	}
	var laps []contract.LapSummary
	for _, id := range q.LapIDs {
		l, ok := byID[id]
		if !ok {
			return contract.AnalysisResult{}, fmt.Errorf("lap %s not found", id)
		}
		laps = append(laps, l)
	}
	res := contract.AnalysisResult{Laps: laps, Differences: []string{}, Coverage: contract.Coverage{Gaps: []string{}, StaleChannels: []string{}}}
	detail, err := e.Catalog.Session(ctx, q.SessionID)
	if err != nil {
		return res, err
	}
	res.Context = detail.Context
	for _, l := range laps {
		if !l.Valid {
			res.Differences = append(res.Differences, fmt.Sprintf("lap %d is invalid", l.Lap))
		}
		if l.PitLap || l.Neutralized {
			res.Differences = append(res.Differences, fmt.Sprintf("lap %d includes a pit or neutralized segment", l.Lap))
		}
	}
	for i := 1; i < len(laps); i++ {
		if measurementText(laps[i].Compound) != measurementText(laps[0].Compound) {
			res.Differences = append(res.Differences, fmt.Sprintf("compound differs: %s vs %s", measurementText(laps[0].Compound), measurementText(laps[i].Compound)))
		}
		if laps[i].Conditions != laps[0].Conditions {
			res.Differences = append(res.Differences, fmt.Sprintf("conditions differ: %s vs %s", laps[0].Conditions, laps[i].Conditions))
		}
	}
	res.Comparable = len(res.Differences) == 0
	grid := 200
	for _, ch := range q.Channels {
		var lapSeries []contract.Series
		maxDist := 0.0
		for _, l := range laps {
			pts, err := e.lapPoints(ctx, l, ch, q.MaxPoints)
			if err != nil {
				return res, err
			}
			lapSeries = append(lapSeries, pts)
			for _, p := range pts.Points {
				if p.DistanceM != nil && *p.DistanceM > maxDist {
					maxDist = *p.DistanceM
				}
			}
		}
		if maxDist == 0 {
			res.Coverage.Gaps = append(res.Coverage.Gaps, fmt.Sprintf("%s: no distance samples", ch))
			continue
		}
		step := maxDist / float64(grid)
		for i, s := range lapSeries {
			aligned := contract.Series{ID: s.ID + ":aligned", Channel: ch, CarIndex: intPtr(laps[i].CarIndex), Points: make([]contract.SeriesPoint, 0, grid+1)}
			for j := 0; j <= grid; j++ {
				dist := float64(j) * step
				sp := contract.SeriesPoint{Seq: strconv.Itoa(j), DistanceM: float64Ptr(dist)}
				if v, ok := interpolate(s.Points, dist); ok {
					sp.Measurement = contract.Available(v, unitOf(ch), "analysis.compare")
				} else {
					sp.Measurement = contract.Unavailable("not_received", unitOf(ch), "analysis.compare", "distance gap")
				}
				aligned.Points = append(aligned.Points, sp)
			}
			res.Series = append(res.Series, aligned)
		}
	}
	if res.Series == nil {
		res.Series = []contract.Series{}
	}
	return res, nil
}

func (e *Engine) lapPoints(ctx context.Context, l contract.LapSummary, ch string, max int) (contract.Series, error) {
	spec, ok := protocol.Lookup(ch)
	if !ok {
		return contract.Series{}, fmt.Errorf("unknown channel %s", ch)
	}
	paths, err := e.Catalog.RecordingsForSession(ctx, l.SessionID)
	if err != nil {
		return contract.Series{}, err
	}
	from, _ := strconv.ParseUint(l.StartSeq, 10, 64)
	to, _ := strconv.ParseUint(l.EndSeq, 10, 64)
	var distances []struct {
		ms  int64
		val float64
	}
	var pts []point
	err = scanFiles(ctx, paths, func(m *mcap.Message) error {
		seq := uint64(m.Sequence)
		if seq < from || seq > to || len(m.Data) < 7 {
			return nil
		}
		pid := m.Data[6]
		if pid != 2 && pid != spec.PacketID {
			return nil
		}
		p, err := protocol.Decode(m.Data)
		if err != nil {
			return nil
		}
		ms := int64(math.Round(p.Header.SessionTime * 1000))
		if v, ok := p.CarField("m_lapDistance", l.CarIndex); ok {
			distances = append(distances, struct {
				ms  int64
				val float64
			}{ms: ms, val: v.F})
		}
		if pid != spec.PacketID {
			return nil
		}
		v, ok := extract(p, spec, l.CarIndex)
		if !ok {
			return nil
		}
		pts = append(pts, point{seq: seq, sessionMs: ms, value: v, available: true})
		return nil
	})
	if err != nil {
		return contract.Series{}, err
	}
	for i := range pts {
		if d, ok := nearestDistance(distances, pts[i].sessionMs, 250); ok {
			pts[i].distance = &d
		}
	}
	pts = decimate(pts, max)
	series := contract.Series{ID: l.ID, Channel: ch, CarIndex: intPtr(l.CarIndex), LapID: &l.ID}
	for _, pt := range pts {
		series.Points = append(series.Points, contract.SeriesPoint{Seq: strconv.FormatUint(pt.seq, 10), SessionTimeMs: int64Ptr(pt.sessionMs), DistanceM: pt.distance, Measurement: contract.Available(pt.value, unitOf(ch), "analysis.compare")})
	}
	return series, nil
}

// nearestDistance finds the closest lap-distance sample by session time,
// bounded so a gap is never bridged by an unrelated part of the lap.
func nearestDistance(samples []struct {
	ms  int64
	val float64
}, at int64, tolerance int64) (float64, bool) {
	best := int64(1 << 62)
	var value float64
	found := false
	for _, s := range samples {
		d := s.ms - at
		if d < 0 {
			d = -d
		}
		if d < best {
			best = d
			value = s.val
			found = true
		}
	}
	if !found || best > tolerance {
		return 0, false
	}
	return value, true
}

func interpolate(pts []contract.SeriesPoint, dist float64) (float64, bool) {
	var prev, next *contract.SeriesPoint
	for i := range pts {
		p := &pts[i]
		if p.DistanceM == nil {
			continue
		}
		if *p.DistanceM <= dist {
			prev = p
		}
		if *p.DistanceM >= dist {
			next = p
			break
		}
	}
	if prev == nil || next == nil {
		return 0, false
	}
	if prev.Measurement.Availability != "available" || next.Measurement.Availability != "available" {
		return 0, false
	}
	pv, ok1 := number(prev.Measurement.Value)
	nv, ok2 := number(next.Measurement.Value)
	if !ok1 || !ok2 {
		return 0, false
	}
	if *next.DistanceM == *prev.DistanceM {
		return pv, true
	}
	if prev.SessionTimeMs != nil && next.SessionTimeMs != nil && (*next.SessionTimeMs-*prev.SessionTimeMs) > 250 {
		return 0, false
	}
	frac := (dist - *prev.DistanceM) / (*next.DistanceM - *prev.DistanceM)
	return pv + (nv-pv)*frac, true
}

func number(v any) (float64, bool) {
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

func measurementText(m contract.Measurement) string {
	if m.Availability != "available" {
		return m.Availability
	}
	if s, ok := m.Value.(string); ok {
		return s
	}
	return fmt.Sprint(m.Value)
}
