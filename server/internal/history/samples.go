
package history

import (
	"context"
	"encoding/json"
	"fmt"
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
	var held int
	if err := c.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM lap_samples WHERE session_id=? AND branch_id=?`, sessionID, branchID).Scan(&held); err != nil {
		return nil, 0, err
	}
	if len(cars) == 0 {
		return []LapSamples{}, held, nil
	}
	args := []any{sessionID, branchID}
	placeholders := ""
	for i, car := range cars {
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
		args = append(args, car)
	}
	query := fmt.Sprintf(`SELECT car_idx, lap, start_ms, lap_ms, points FROM lap_samples
		WHERE session_id=? AND branch_id=? AND car_idx IN (%s)`, placeholders)
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
	out := []LapSamples{}
	for rows.Next() {
		var s LapSamples
		var raw string
		if err := rows.Scan(&s.CarIndex, &s.Lap, &s.StartMs, &s.LapMs, &raw); err != nil {
			return nil, held, err
		}
		if err := json.Unmarshal([]byte(raw), &s.Points); err != nil {
			return nil, held, err
		}
		s.PointCount = len(s.Points)
		out = append(out, s)
	}
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


