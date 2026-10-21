package analytics

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"f1tele/server/internal/contract"
	"f1tele/server/internal/history"
)

// SessionSummary is the recorded-session payload used by History, Pit wall and
// Analysis. It never reads live state.
type SessionSummary struct {
	SessionID       string                 `json:"session_id"`
	SessionType     string                 `json:"session_type"`
	SessionTypeCode *int                   `json:"session_type_code"`
	WeekendLinkID   *string                `json:"weekend_link_id"`
	TrackID         *int                   `json:"track_id"`
	Season          *int                   `json:"regulation_season"`
	StartedAtMs     int64                  `json:"started_at_ms"`
	EndedAtMs       *int64                 `json:"ended_at_ms"`
	Participants    []history.Participant  `json:"participants"`
	Laps            []history.LapAnalytics `json:"laps"`
	Stints          []history.Stint        `json:"stints"`
	BestLapMs       int64                  `json:"best_lap_ms"`
	TheoreticalBest int64                  `json:"theoretical_best_ms"`
	CleanLaps       int                    `json:"clean_laps"`
	TotalLaps       int                    `json:"total_laps"`
	AnalyticsReady  bool                   `json:"analytics_ready"`
}

func Summary(ctx context.Context, cat *history.Catalog, sessionID string) (SessionSummary, error) {
	var out SessionSummary
	detail, err := cat.Session(ctx, sessionID)
	if err != nil {
		return out, err
	}
	out.SessionID = sessionID
	out.TrackID = detail.Summary.TrackID
	out.Season = detail.Summary.RegulationSeason
	out.StartedAtMs = detail.Summary.StartedAtMs
	out.EndedAtMs = detail.Summary.EndedAtMs
	code := detail.Context.SessionTypeCode
	out.WeekendLinkID = detail.Context.WeekendLinkID
	out.SessionTypeCode = code
	if code != nil {
		out.SessionType = history.SessionType(*code)
	} else {
		out.SessionType = "unknown"
	}
	branch := detail.SurvivingBranchID
	participants, err := cat.Participants(ctx, sessionID)
	if err != nil {
		return out, err
	}
	out.Participants = participants
	laps, err := cat.LapAnalytics(ctx, sessionID, branch, -1)
	if err != nil {
		return out, err
	}
	out.Laps = laps
	out.AnalyticsReady = len(laps) > 0
	stints, err := cat.Stints(ctx, sessionID, branch)
	if err != nil {
		return out, err
	}
	out.Stints = stints
	if len(laps) > 0 {
		_ = cat.RebuildStints(ctx, sessionID, branch)
		_ = cat.UpsertSessionAnalytics(ctx, sessionID, branch, MethodVersion)
		out.Stints, _ = cat.Stints(ctx, sessionID, branch)
	}
	if best, theo, clean, total, err := cat.SessionAnalytics(ctx, sessionID); err == nil {
		out.BestLapMs, out.TheoreticalBest, out.CleanLaps, out.TotalLaps = best, theo, clean, total
	}
	return out, nil
}

// LapDetail returns one car's lap plus its stored trace points.
func LapDetail(ctx context.Context, cat *history.Catalog, sessionID, branchID string, car, lap int) (history.LapAnalytics, []byte, error) {
	laps, err := cat.LapAnalytics(ctx, sessionID, branchID, car)
	if err != nil {
		return history.LapAnalytics{}, nil, err
	}
	for _, l := range laps {
		if l.Lap == lap {
			trace, _ := cat.Trace(ctx, sessionID, branchID, car, lap, "car.speed_mps")
			return l, trace, nil
		}
	}
	return history.LapAnalytics{}, nil, fmt.Errorf("lap %d for car %d not found", lap, car)
}

// CompareLaps builds sector and corner deltas for two to four (car, lap) pairs.
type CompareLap struct {
	Car int `json:"car_index"`
	Lap int `json:"lap"`
}

type LapComparison struct {
	Laps         []history.LapAnalytics `json:"laps"`
	SectorDeltas []map[string]int64     `json:"sector_deltas"`
	CornerDeltas []map[string]float64   `json:"corner_deltas"`
	Theoretical  int64                  `json:"theoretical_best_ms"`
	Differences  []string               `json:"differences"`
	Comparable   bool                   `json:"comparable"`
}

func Compare(ctx context.Context, cat *history.Catalog, sessionID, branchID string, pairs []CompareLap) (LapComparison, error) {
	if len(pairs) < 2 || len(pairs) > 4 {
		return LapComparison{}, fmt.Errorf("select two to four laps")
	}
	all, err := cat.LapAnalytics(ctx, sessionID, branchID, -1)
	if err != nil {
		return LapComparison{}, err
	}
	byKey := map[string]history.LapAnalytics{}
	for _, l := range all {
		byKey[strconv.Itoa(l.CarIndex)+":"+strconv.Itoa(l.Lap)] = l
	}
	out := LapComparison{Laps: []history.LapAnalytics{}, SectorDeltas: []map[string]int64{}, CornerDeltas: []map[string]float64{}, Differences: []string{}}
	var bestS1, bestS2, bestS3 int64
	for _, p := range pairs {
		l, ok := byKey[strconv.Itoa(p.Car)+":"+strconv.Itoa(p.Lap)]
		if !ok {
			return out, fmt.Errorf("lap %d for car %d not found", p.Lap, p.Car)
		}
		if !l.Valid || !l.Complete || l.LapMs <= 0 {
			return out, fmt.Errorf("lap %d for car %d is invalid, incomplete, or has no lap time", p.Lap, p.Car)
		}
		out.Laps = append(out.Laps, l)
		if l.Sector1Ms > 0 && (bestS1 == 0 || l.Sector1Ms < bestS1) {
			bestS1 = l.Sector1Ms
		}
		if l.Sector2Ms > 0 && (bestS2 == 0 || l.Sector2Ms < bestS2) {
			bestS2 = l.Sector2Ms
		}
		if l.Sector3Ms > 0 && (bestS3 == 0 || l.Sector3Ms < bestS3) {
			bestS3 = l.Sector3Ms
		}
	}
	sort.SliceStable(out.Laps, func(i, j int) bool { return out.Laps[i].LapMs < out.Laps[j].LapMs })
	base := out.Laps[0]
	baseCorner := map[string]float64{}
	for k, m := range base.CornerSpeeds {
		if mv, ok := number(m.Value); ok {
			baseCorner[k] = mv
		}
	}
	for _, l := range out.Laps {
		out.SectorDeltas = append(out.SectorDeltas, map[string]int64{
			"sector1": l.Sector1Ms - base.Sector1Ms, "sector2": l.Sector2Ms - base.Sector2Ms, "sector3": l.Sector3Ms - base.Sector3Ms,
		})
		corner := map[string]float64{}
		for k, m := range l.CornerSpeeds {
			if mv, ok := number(m.Value); ok {
				if bv, ok := baseCorner[k]; ok {
					corner[k] = mv - bv
				}
			}
		}
		out.CornerDeltas = append(out.CornerDeltas, corner)
		if l.Compound != base.Compound {
			out.Differences = append(out.Differences, fmt.Sprintf("compound differs: %s vs %s", base.Compound, l.Compound))
		}
	}
	for i := 1; i < len(out.Laps); i++ {
		if out.Laps[i].Conditions != base.Conditions {
			out.Differences = append(out.Differences, "track conditions differ")
		}
	}
	out.Theoretical = bestS1 + bestS2 + bestS3
	out.Comparable = len(out.Differences) == 0
	return out, nil
}

func number(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int64:
		return float64(x), true
	}
	return 0, false
}

var _ = contract.SchemaVersion
