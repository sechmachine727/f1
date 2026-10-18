// Package analysis serves bounded historical range queries, comparisons and exports.
package analysis

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"f1tele/server/internal/contract"
	"f1tele/server/internal/history"
	"f1tele/server/internal/protocol"

	"github.com/foxglove/mcap/go/mcap"
)

var (
	ErrTooManyCars     = errors.New("at most 24 cars per query")
	ErrTooManyChannels = errors.New("at most 16 channels per query")
	ErrTooManyPoints   = errors.New("max_points exceeds 20000 per series")
	ErrRange           = errors.New("from must be less than to")
	ErrLapCount        = errors.New("comparison needs two to four laps")
)

type Engine struct {
	Catalog *history.Catalog
	DataDir string
	Now     func() time.Time
}

func Validate(q contract.Query) error {
	if len(q.CarIndices) > 24 {
		return ErrTooManyCars
	}
	if len(q.Channels) == 0 || len(q.Channels) > 16 {
		return ErrTooManyChannels
	}
	if q.MaxPoints > 20000 {
		return ErrTooManyPoints
	}
	if q.ToSessionTimeMs <= q.FromSessionTimeMs {
		return ErrRange
	}
	return nil
}

type point struct {
	seq       uint64
	sessionMs int64
	distance  *float64
	value     float64
	available bool
}

func (e *Engine) Query(ctx context.Context, q contract.Query) (contract.AnalysisResult, error) {
	if err := Validate(q); err != nil {
		return contract.AnalysisResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	paths, err := e.Catalog.RecordingsForSession(ctx, q.SessionID)
	if err != nil {
		return contract.AnalysisResult{}, err
	}
	if len(paths) == 0 {
		return contract.AnalysisResult{}, history.ErrNotFound
	}
	segs, err := e.Catalog.InheritedSegments(ctx, q.BranchID)
	if err != nil {
		return contract.AnalysisResult{}, err
	}
	detail, err := e.Catalog.Session(ctx, q.SessionID)
	if err != nil {
		return contract.AnalysisResult{}, err
	}
	cars := q.CarIndices
	if len(cars) == 0 {
		cars = []int{0}
	}
	res := contract.AnalysisResult{Context: detail.Context, Laps: []contract.LapSummary{}, Differences: []string{}, Coverage: contract.Coverage{Gaps: []string{}, StaleChannels: []string{}}}
	points := map[string][]point{}
	restricted := map[int]bool{}
	scanErr := scanFiles(ctx, paths, func(m *mcap.Message) error {
		if !inSegments(uint64(m.Sequence), segs) {
			return nil
		}
		p, err := protocol.Decode(m.Data)
		if err != nil {
			res.Coverage.Gaps = append(res.Coverage.Gaps, fmt.Sprintf("seq %d undecodable: %v", m.Sequence, err))
			return nil
		}
		sm := int64(math.Round(p.Header.SessionTime * 1000))
		if sm < q.FromSessionTimeMs || sm >= q.ToSessionTimeMs {
			return nil
		}
		if p.Header.PacketID == 4 {
			for car := 0; car < p.CarCount(); car++ {
				if v, ok := p.CarField("m_yourTelemetry", car); ok && v.F == 0 && car != int(p.Header.PlayerCarIndex) {
					restricted[car] = true
				}
			}
		}
		for _, car := range cars {
			dist := distanceAt(p, car)
			for _, ch := range q.Channels {
				spec, ok := protocol.Lookup(ch)
				if !ok || spec.PacketID != p.Header.PacketID {
					continue
				}
				mv, ok := extract(p, spec, car)
				if !ok {
					continue
				}
				key := fmt.Sprintf("%s@%d", ch, car)
				pts := points[key]
				if len(pts) > 0 && pts[len(pts)-1].seq == uint64(m.Sequence) {
					continue
				}
				points[key] = append(pts, point{seq: uint64(m.Sequence), sessionMs: sm, distance: dist, value: mv, available: !restricted[car]})
			}
		}
		return nil
	})
	if scanErr != nil {
		return res, scanErr
	}
	for _, car := range cars {
		for _, ch := range q.Channels {
			key := fmt.Sprintf("%s@%d", ch, car)
			pts := points[key]
			if len(pts) == 0 {
				res.Differences = append(res.Differences, fmt.Sprintf("%s has no samples in range", key))
				continue
			}
			dec := decimate(pts, q.MaxPoints)
			series := contract.Series{ID: key + ":" + q.BranchID, Channel: ch, CarIndex: intPtr(car), Points: make([]contract.SeriesPoint, 0, len(dec))}
			for _, pt := range dec {
				sp := contract.SeriesPoint{Seq: strconv.FormatUint(pt.seq, 10), SessionTimeMs: int64Ptr(pt.sessionMs), DistanceM: pt.distance}
				if pt.available {
					sp.Measurement = contract.Available(pt.value, unitOf(ch), "analysis.raw")
				} else {
					sp.Measurement = contract.Unavailable("restricted", unitOf(ch), "analysis.raw", "participant restricts this channel")
				}
				series.Points = append(series.Points, sp)
			}
			res.Series = append(res.Series, series)
		}
	}
	if res.Series == nil {
		res.Series = []contract.Series{}
	}
	return res, nil
}

func scanFiles(ctx context.Context, paths []string, fn func(*mcap.Message) error) error {
	for _, path := range paths {
		if err := scanFile(ctx, path, fn); err != nil {
			if errors.Is(err, io.EOF) {
				continue
			}
			return err
		}
	}
	return nil
}

func scanFile(ctx context.Context, path string, fn func(*mcap.Message) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	r, err := mcap.NewReader(f)
	if err != nil {
		if unfinalized(err) {
			// Active recordings have no footer yet. Scan the data section until the
			// writer catches up instead of treating the session as unreadable.
			if _, seekErr := f.Seek(0, io.SeekStart); seekErr != nil {
				return seekErr
			}
			return scanUnfinalized(ctx, f, fn)
		}
		return fmt.Errorf("read %s: %w", path, err)
	}
	defer r.Close()
	it, err := r.Messages()
	if err != nil {
		if unfinalized(err) {
			if _, seekErr := f.Seek(0, io.SeekStart); seekErr != nil {
				return seekErr
			}
			return scanUnfinalized(ctx, f, fn)
		}
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		_, _, m, err := it.Next(nil)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if m == nil {
			return nil
		}
		if err := fn(m); err != nil {
			return err
		}
	}
}

func unfinalized(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "at end of file") || strings.Contains(msg, "Invalid magic")
}

func scanUnfinalized(ctx context.Context, r io.Reader, fn func(*mcap.Message) error) error {
	lx, err := mcap.NewLexer(r, &mcap.LexerOptions{MaxDecompressedChunkSize: 256 << 20})
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		tok, rec, err := lx.Next(nil)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return nil
			}
			return err
		}
		switch tok {
		case mcap.TokenMessage:
			var m mcap.Message
			if err := m.PopulateFrom(rec, true); err != nil {
				return err
			}
			if err := fn(&m); err != nil {
				return err
			}
		}
	}
}

func inSegments(seq uint64, segs []history.Segment) bool {
	for _, s := range segs {
		if seq >= s.FromSeq && (s.ToSeq == 0 || seq <= s.ToSeq) {
			return true
		}
	}
	return false
}

func distanceAt(p *protocol.Packet, car int) *float64 {
	if p.Header.PacketID != 2 {
		return nil
	}
	if v, ok := p.CarField("m_lapDistance", car); ok {
		f := v.F
		return &f
	}
	return nil
}

func extract(p *protocol.Packet, spec protocol.ChannelSpec, car int) (float64, bool) {
	var v protocol.Value
	var ok bool
	if spec.CarIndexed {
		v, ok = p.CarField(spec.Field, car)
	} else {
		v, ok = p.Field(spec.Field)
	}
	if !ok || v.IsString {
		return 0, false
	}
	if math.IsNaN(v.F) || math.IsInf(v.F, 0) {
		return 0, false
	}
	return v.F*spec.Scale + spec.Offset, true
}

func decimate(pts []point, max int) []point {
	if max <= 0 || len(pts) <= max {
		return pts
	}
	if max < 4 {
		max = 4
	}
	buckets := max / 4
	per := int(math.Ceil(float64(len(pts)) / float64(buckets)))
	out := make([]point, 0, max)
	for i := 0; i < len(pts); i += per {
		end := i + per
		if end > len(pts) {
			end = len(pts)
		}
		b := pts[i:end]
		first, last := b[0], b[len(b)-1]
		minP, maxP := b[0], b[0]
		for _, p := range b {
			if p.value < minP.value {
				minP = p
			}
			if p.value > maxP.value {
				maxP = p
			}
		}
		out = append(out, first)
		if minP.seq != first.seq {
			out = append(out, minP)
		}
		if maxP.seq != first.seq && maxP.seq != minP.seq {
			out = append(out, maxP)
		}
		if last.seq != first.seq && last.seq != minP.seq && last.seq != maxP.seq {
			out = append(out, last)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].seq < out[j].seq })
	if len(out) > max {
		out = out[:max]
	}
	return out
}

func unitOf(ch string) string {
	if s, ok := protocol.Lookup(ch); ok {
		return s.Unit
	}
	return ""
}

func intPtr(v int) *int             { return &v }
func int64Ptr(v int64) *int64       { return &v }
func float64Ptr(v float64) *float64 { return &v }
