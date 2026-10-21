package history

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
)

// SampleMethodVersion identifies the stored per-lap sample format.
const SampleMethodVersion = "lap-samples-v1"

// LapSamples is one car's stored input trace for one completed lap. Each point
// is [session_time_ms, throttle, brake, gear, ers_j, deploy_mode,
// overtake_available, overtake_active]; unavailable values are null.
type LapSamples struct {
	CarIndex   int
	Lap        int
	StartMs    int64
	LapMs      int64
	PointCount int
	Points     [][]any
}

// UpsertLapSamples replaces one lap's stored samples.
func (c *Catalog) UpsertLapSamples(ctx context.Context, sessionID, branchID string, s LapSamples) error {
	raw, err := json.Marshal(s.Points)
	if err != nil {
		return err
	}
	_, err = c.db.ExecContext(ctx, `INSERT INTO lap_samples(session_id, branch_id, car_idx, lap, start_ms, lap_ms, points, point_count, method_version)
		VALUES(?,?,?,?,?,?,?,?,?)
		ON CONFLICT(session_id, branch_id, car_idx, lap) DO UPDATE SET start_ms=excluded.start_ms, lap_ms=excluded.lap_ms,
		points=excluded.points, point_count=excluded.point_count, method_version=excluded.method_version`,
		sessionID, branchID, s.CarIndex, s.Lap, s.StartMs, s.LapMs, string(raw), len(s.Points), SampleMethodVersion)
	return err
}

// SessionLapSamples reads stored samples for the named cars. An empty from and
// to means every stored lap. The second result is the number of laps held for
// the session, which differs from the returned rows when a range or car filter
// excludes them.
func (c *Catalog) SessionLapSamples(ctx context.Context, sessionID, branchID string, cars []int, from, to int) ([]LapSamples, int, error) {
	branches, err := c.branchLineage(ctx, sessionID, branchID)
	if err != nil {
		return nil, 0, err
	}
	if len(branches) == 0 {
		branches = []string{branchID}
	}
	rank := make(map[string]int, len(branches))
	args := make([]any, 0, len(branches)+2)
	args = append(args, sessionID)
	for _, b := range branches {
		rank[b] = len(rank)
		args = append(args, b)
	}
	in := questionMarks(len(branches))
	var held int
	if err := c.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT car_idx || ':' || lap) FROM lap_samples WHERE session_id=? AND branch_id IN (`+in+`)`, args...).Scan(&held); err != nil {
		return nil, 0, err
	}
	if len(cars) == 0 {
		return []LapSamples{}, held, nil
	}
	placeholders := ""
	for i, car := range cars {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args = append(args, car)
	}
	query := fmt.Sprintf(`SELECT branch_id, car_idx, lap, start_ms, lap_ms, points FROM lap_samples
		WHERE session_id=? AND branch_id IN (%s) AND car_idx IN (%s)`, in, placeholders)
	if from > 0 {
		query += " AND lap>=?"
		args = append(args, from)
	}
	if to > 0 {
		query += " AND lap<=?"
		args = append(args, to)
	}
	query += " ORDER BY car_idx, lap"
	rows, err := c.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, held, err
	}
	defer rows.Close()
	byLap := map[string]LapSamples{}
	branchOf := map[string]string{}
	for rows.Next() {
		var s LapSamples
		var branch string
		var raw string
		if err := rows.Scan(&branch, &s.CarIndex, &s.Lap, &s.StartMs, &s.LapMs, &raw); err != nil {
			return nil, held, err
		}
		if err := json.Unmarshal([]byte(raw), &s.Points); err != nil {
			return nil, held, err
		}
		s.PointCount = len(s.Points)
		k := strconv.Itoa(s.CarIndex) + ":" + strconv.Itoa(s.Lap)
		if old, ok := branchOf[k]; ok && rank[old] >= rank[branch] {
			continue
		}
		byLap[k] = s
		branchOf[k] = branch
	}
	out := make([]LapSamples, 0, len(byLap))
	for _, s := range byLap {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CarIndex != out[j].CarIndex {
			return out[i].CarIndex < out[j].CarIndex
		}
		return out[i].Lap < out[j].Lap
	})
	return out, held, rows.Err()
}

// SessionRestrictedCars reports the cars whose participant telemetry setting
// hides other cars' channels.
func (c *Catalog) SessionRestrictedCars(ctx context.Context, sessionID string) (map[int]bool, error) {
	rows, err := c.db.QueryContext(ctx, `SELECT car_idx FROM session_participants WHERE session_id=? AND your_telemetry=0`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int]bool{}
	for rows.Next() {
		var car int
		if err := rows.Scan(&car); err == nil {
			out[car] = true
		}
	}
	return out, rows.Err()
}
