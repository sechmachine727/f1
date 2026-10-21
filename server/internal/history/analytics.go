package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	"f1tele/server/internal/contract"
)

// LapAnalytics is one car's derived record for one lap.
type LapAnalytics struct {
	SessionID      string                          `json:"session_id"`
	BranchID       string                          `json:"branch_id"`
	CarIndex       int                             `json:"car_index"`
	Lap            int                             `json:"lap"`
	LapMs          int64                           `json:"lap_ms"`
	Sector1Ms      int64                           `json:"sector1_ms"`
	Sector2Ms      int64                           `json:"sector2_ms"`
	Sector3Ms      int64                           `json:"sector3_ms"`
	DistanceM      float64                         `json:"distance_m"`
	AvgSpeed       float64                         `json:"avg_speed_mps"`
	TopSpeed       float64                         `json:"top_speed_mps"`
	TrapSpeed      float64                         `json:"trap_speed_mps"`
	PositionStart  int                             `json:"position_start"`
	PositionEnd    int                             `json:"position_end"`
	GapAheadStart  int64                           `json:"gap_ahead_start_ms"`
	GapLeaderStart int64                           `json:"gap_leader_start_ms"`
	Compound       string                          `json:"compound"`
	TyreAge        int                             `json:"tyre_age_laps"`
	FuelStart      float64                         `json:"fuel_start_kg"`
	FuelEnd        float64                         `json:"fuel_end_kg"`
	ErsStart       float64                         `json:"ers_start_j"`
	ErsEnd         float64                         `json:"ers_end_j"`
	PitIn          bool                            `json:"pit_in"`
	PitOut         bool                            `json:"pit_out"`
	Valid          bool                            `json:"valid"`
	TrackLimits    bool                            `json:"track_limits"`
	Complete       bool                            `json:"complete"`
	Conditions     string                          `json:"conditions"`
	CornerSpeeds   map[string]contract.Measurement `json:"corner_speeds"`
}

type Participant struct {
	CarIndex int    `json:"car_index"`
	Name     string `json:"name"`
	TeamID   int    `json:"team_id"`
	Number   int    `json:"race_number"`
	AI       bool   `json:"ai"`
	FirstSeq uint64 `json:"first_seq"`
	LastSeq  uint64 `json:"last_seq"`
}

// Stint is one continuous stint or run for a car.
type Stint struct {
	CarIndex    int     `json:"car_index"`
	Kind        string  `json:"kind"`
	Index       int     `json:"index"`
	StartLap    int     `json:"start_lap"`
	EndLap      int     `json:"end_lap"`
	Compound    string  `json:"compound"`
	Laps        int     `json:"laps"`
	BestMs      int64   `json:"best_ms"`
	MedianMs    int64   `json:"median_ms"`
	Degradation float64 `json:"degradation_ms_per_lap"`
	FuelStart   float64 `json:"fuel_start_kg"`
	FuelEnd     float64 `json:"fuel_end_kg"`
	ErsStart    float64 `json:"ers_start_j"`
	ErsEnd      float64 `json:"ers_end_j"`
}

func (c *Catalog) Stints(ctx context.Context, sessionID, branchID string) ([]Stint, error) {
	rows, err := c.db.QueryContext(ctx, `SELECT car_idx, kind, idx, start_lap, end_lap, compound, laps, COALESCE(best_ms,0), COALESCE(median_ms,0),
		COALESCE(degradation_ms_per_lap,0), COALESCE(fuel_start_kg,0), COALESCE(fuel_end_kg,0), COALESCE(ers_start_j,0), COALESCE(ers_end_j,0)
		FROM analytics_stints WHERE session_id=? AND branch_id=? ORDER BY car_idx, kind, idx`, sessionID, branchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Stint{}
	for rows.Next() {
		var s Stint
		if err := rows.Scan(&s.CarIndex, &s.Kind, &s.Index, &s.StartLap, &s.EndLap, &s.Compound, &s.Laps, &s.BestMs, &s.MedianMs,
			&s.Degradation, &s.FuelStart, &s.FuelEnd, &s.ErsStart, &s.ErsEnd); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (c *Catalog) SessionAnalytics(ctx context.Context, sessionID string) (best, theo int64, clean, total int, err error) {
	err = c.db.QueryRowContext(ctx, `SELECT COALESCE(best_lap_ms,0), COALESCE(theoretical_best_ms,0), clean_laps, total_laps FROM session_analytics WHERE session_id=?`, sessionID).Scan(&best, &theo, &clean, &total)
	return
}

func (c *Catalog) UpsertParticipant(ctx context.Context, sessionID string, car int, name string, team, number int, ai, public bool, seq uint64) error {
	if name == "" {
		name = "Car " + strconv.Itoa(car+1)
	}
	_, err := c.db.ExecContext(ctx, `INSERT INTO session_participants(session_id, car_idx, name, team_id, race_number, ai, your_telemetry, first_seq, last_seq)
		VALUES(?,?,?,?,?,?,?,?,?)
		ON CONFLICT(session_id, car_idx) DO UPDATE SET name=excluded.name, team_id=excluded.team_id,
		race_number=excluded.race_number, ai=excluded.ai, your_telemetry=excluded.your_telemetry, last_seq=excluded.last_seq`,
		sessionID, car, name, team, number, boolInt(ai), boolInt(public), seq, seq)
	return err
}

func (c *Catalog) UpsertLapAnalytics(ctx context.Context, l LapAnalytics) error {
	extra, _ := json.Marshal(l.CornerSpeeds)
	_, err := c.db.ExecContext(ctx, `INSERT INTO lap_analytics(session_id, branch_id, car_idx, lap, lap_ms, sector1_ms, sector2_ms, sector3_ms,
		distance_m, avg_speed_mps, top_speed_mps, trap_speed_mps, position_start, position_end, gap_ahead_start_ms, gap_leader_start_ms,
		compound, tyre_age_laps, fuel_start_kg, fuel_end_kg, ers_start_j, ers_end_j, pit_in, pit_out, valid, track_limits, complete, conditions, extra_json)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(session_id, branch_id, car_idx, lap) DO UPDATE SET
		lap_ms=excluded.lap_ms, sector1_ms=COALESCE(NULLIF(excluded.sector1_ms,0),lap_analytics.sector1_ms),
		 sector2_ms=COALESCE(NULLIF(excluded.sector2_ms,0),lap_analytics.sector2_ms),
		 sector3_ms=COALESCE(NULLIF(excluded.sector3_ms,0),lap_analytics.sector3_ms),
		distance_m=excluded.distance_m, avg_speed_mps=excluded.avg_speed_mps, top_speed_mps=excluded.top_speed_mps,
		position_start=excluded.position_start, position_end=excluded.position_end, gap_ahead_start_ms=excluded.gap_ahead_start_ms,
		gap_leader_start_ms=excluded.gap_leader_start_ms, compound=excluded.compound, tyre_age_laps=excluded.tyre_age_laps,
		fuel_start_kg=excluded.fuel_start_kg, fuel_end_kg=excluded.fuel_end_kg, ers_start_j=excluded.ers_start_j, ers_end_j=excluded.ers_end_j,
		pit_in=excluded.pit_in, pit_out=excluded.pit_out, valid=excluded.valid, track_limits=excluded.track_limits,
		complete=excluded.complete, conditions=excluded.conditions, extra_json=excluded.extra_json`,
		l.SessionID, l.BranchID, l.CarIndex, l.Lap, l.LapMs, l.Sector1Ms, l.Sector2Ms, l.Sector3Ms, l.DistanceM, l.AvgSpeed, l.TopSpeed,
		l.TrapSpeed, l.PositionStart, l.PositionEnd, l.GapAheadStart, l.GapLeaderStart, l.Compound, l.TyreAge, l.FuelStart, l.FuelEnd,
		l.ErsStart, l.ErsEnd, boolInt(l.PitIn), boolInt(l.PitOut), boolInt(l.Valid), boolInt(l.TrackLimits), boolInt(l.Complete), l.Conditions, string(extra))
	return err
}

// UpdateLapSectors fills the sector record from packet 11's lap history.
func (c *Catalog) UpdateLapSectors(ctx context.Context, sessionID, branchID string, car, lap int, ms, s1, s2, s3 int64, valid bool) error {
	_, err := c.db.ExecContext(ctx, `INSERT INTO lap_analytics(session_id, branch_id, car_idx, lap, lap_ms, sector1_ms, sector2_ms, sector3_ms, valid, complete)
		VALUES(?,?,?,?,?,?,?,?,?,1)
		ON CONFLICT(session_id, branch_id, car_idx, lap) DO UPDATE SET lap_ms=excluded.lap_ms, sector1_ms=excluded.sector1_ms,
		sector2_ms=excluded.sector2_ms, sector3_ms=excluded.sector3_ms, valid=excluded.valid`,
		sessionID, branchID, car, lap, ms, s1, s2, s3, boolInt(valid))
	return err
}

// UpsertTrace stores a decimated per-lap series.
func (c *Catalog) UpsertTrace(ctx context.Context, sessionID, branchID string, car, lap int, channel string, points []byte, method string) error {
	_, err := c.db.ExecContext(ctx, `INSERT INTO lap_traces(session_id, branch_id, car_idx, lap, channel, points, point_count, decimated, method_version)
		VALUES(?,?,?,?,?,?,?,1,?) ON CONFLICT DO UPDATE SET points=excluded.points, point_count=excluded.point_count, method_version=excluded.method_version`,
		sessionID, branchID, car, lap, channel, points, countPoints(points), method)
	return err
}

func countPoints(b []byte) int {
	var v []any
	if json.Unmarshal(b, &v) != nil {
		return 0
	}
	return len(v)
}

// branchLineage returns the surviving branch and its ancestors, oldest first.
// A rewind starts a child branch, so the branch alone only holds the laps
// recorded after its anchor; earlier laps live on its parents.
func (c *Catalog) branchLineage(ctx context.Context, sessionID, branchID string) ([]string, error) {
	if branchID == "" {
		return nil, nil
	}
	rows, err := c.db.QueryContext(ctx, `WITH RECURSIVE lineage(id, parent_id, depth) AS (
		SELECT id, parent_id, 0 FROM branches WHERE id=? AND session_id=?
		UNION ALL
		SELECT b.id, b.parent_id, lineage.depth+1 FROM branches b JOIN lineage ON b.id=lineage.parent_id
	) SELECT id FROM lineage ORDER BY depth DESC`, branchID, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func questionMarks(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func (c *Catalog) LapAnalytics(ctx context.Context, sessionID, branchID string, car int) ([]LapAnalytics, error) {
	branches, err := c.branchLineage(ctx, sessionID, branchID)
	if err != nil {
		return nil, err
	}
	if len(branches) == 0 {
		branches = []string{branchID}
	}
	rank := make(map[string]int, len(branches))
	args := make([]any, 0, len(branches)+2)
	args = append(args, sessionID)
	q := `SELECT session_id, branch_id, car_idx, lap, COALESCE(lap_ms,0), COALESCE(sector1_ms,0), COALESCE(sector2_ms,0), COALESCE(sector3_ms,0),
		COALESCE(distance_m,0), COALESCE(avg_speed_mps,0), COALESCE(top_speed_mps,0), COALESCE(trap_speed_mps,0), COALESCE(position_start,0), COALESCE(position_end,0),
		COALESCE(gap_ahead_start_ms,0), COALESCE(gap_leader_start_ms,0), COALESCE(compound,''), COALESCE(tyre_age_laps,0), COALESCE(fuel_start_kg,0), COALESCE(fuel_end_kg,0),
		COALESCE(ers_start_j,0), COALESCE(ers_end_j,0), pit_in, pit_out, valid, track_limits, complete, COALESCE(conditions,''), COALESCE(extra_json,'{}')
		FROM lap_analytics WHERE session_id=? AND branch_id IN (` + questionMarks(len(branches)) + `)`
	for _, b := range branches {
		rank[b] = len(rank)
		args = append(args, b)
	}
	if car >= 0 {
		q += " AND car_idx=?"
		args = append(args, car)
	}
	q += " ORDER BY car_idx, lap"
	rows, err := c.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byLap := map[string]LapAnalytics{}
	for rows.Next() {
		var l LapAnalytics
		var pitIn, pitOut, valid, limits, complete int
		var extra string
		if err := rows.Scan(&l.SessionID, &l.BranchID, &l.CarIndex, &l.Lap, &l.LapMs, &l.Sector1Ms, &l.Sector2Ms, &l.Sector3Ms,
			&l.DistanceM, &l.AvgSpeed, &l.TopSpeed, &l.TrapSpeed, &l.PositionStart, &l.PositionEnd, &l.GapAheadStart, &l.GapLeaderStart,
			&l.Compound, &l.TyreAge, &l.FuelStart, &l.FuelEnd, &l.ErsStart, &l.ErsEnd, &pitIn, &pitOut, &valid, &limits, &complete, &l.Conditions, &extra); err != nil {
			return nil, err
		}
		l.PitIn, l.PitOut, l.Valid, l.TrackLimits, l.Complete = pitIn != 0, pitOut != 0, valid != 0, limits != 0, complete != 0
		_ = json.Unmarshal([]byte(extra), &l.CornerSpeeds)
		if l.CornerSpeeds == nil {
			l.CornerSpeeds = map[string]contract.Measurement{}
		}
		k := strconv.Itoa(l.CarIndex) + ":" + strconv.Itoa(l.Lap)
		if old, ok := byLap[k]; ok && rank[old.BranchID] > rank[l.BranchID] {
			continue
		}
		byLap[k] = l
	}
	out := make([]LapAnalytics, 0, len(byLap))
	for _, l := range byLap {
		out = append(out, l)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CarIndex != out[j].CarIndex {
			return out[i].CarIndex < out[j].CarIndex
		}
		return out[i].Lap < out[j].Lap
	})
	return out, rows.Err()
}

func (c *Catalog) Participants(ctx context.Context, sessionID string) ([]Participant, error) {
	rows, err := c.db.QueryContext(ctx, `SELECT car_idx, name, COALESCE(team_id,0), COALESCE(race_number,0), ai, first_seq, last_seq
		FROM session_participants WHERE session_id=? ORDER BY car_idx`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Participant
	for rows.Next() {
		var p Participant
		var ai int
		if err := rows.Scan(&p.CarIndex, &p.Name, &p.TeamID, &p.Number, &ai, &p.FirstSeq, &p.LastSeq); err != nil {
			return nil, err
		}
		p.AI = ai != 0
		out = append(out, p)
	}
	return out, rows.Err()
}

// RebuildStints derives race stints and practice/qualifying runs.
func (c *Catalog) RebuildStints(ctx context.Context, sessionID, branchID string) error {
	laps, err := c.LapAnalytics(ctx, sessionID, branchID, -1)
	if err != nil {
		return err
	}
	if _, err := c.db.ExecContext(ctx, `DELETE FROM analytics_stints WHERE session_id=? AND branch_id=?`, sessionID, branchID); err != nil {
		return err
	}
	byCar := map[int][]LapAnalytics{}
	for _, l := range laps {
		if l.LapMs > 0 {
			byCar[l.CarIndex] = append(byCar[l.CarIndex], l)
		}
	}
	for car, ls := range byCar {
		sort.Slice(ls, func(i, j int) bool { return ls[i].Lap < ls[j].Lap })
		var group []LapAnalytics
		flush := func(kind string, idx int) error {
			if len(group) == 0 {
				return nil
			}
			return c.insertStint(ctx, sessionID, branchID, car, kind, idx, group)
		}
		idx := 0
		for _, l := range ls {
			if len(group) > 0 && (l.Compound != group[len(group)-1].Compound || l.PitIn) {
				if err := flush("stint", idx); err != nil {
					return err
				}
				idx++
				group = nil
			}
			group = append(group, l)
		}
		if err := flush("stint", idx); err != nil {
			return err
		}
		kind := sessionKind(c, sessionID)
		if kind != "practice" && kind != "qualifying" {
			continue
		}
		// Runs restart after every pit lap.
		idx = 0
		group = nil
		for _, l := range ls {
			if l.PitIn && len(group) > 0 {
				if err := flush("run", idx); err != nil {
					return err
				}
				idx++
				group = nil
			}
			group = append(group, l)
		}
		if err := flush("run", idx); err != nil {
			return err
		}
	}
	return nil
}

func (c *Catalog) insertStint(ctx context.Context, sessionID, branchID string, car int, kind string, idx int, ls []LapAnalytics) error {
	var times []int64
	for _, l := range ls {
		if l.Valid && l.Complete {
			times = append(times, l.LapMs)
		}
	}
	best, med := int64(0), int64(0)
	if len(times) > 0 {
		best = times[0]
		for _, v := range times {
			if v < best {
				best = v
			}
		}
		s := append([]int64(nil), times...)
		sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
		med = s[len(s)/2]
	}
	slope := degradation(times)
	start, end := ls[0], ls[len(ls)-1]
	_, err := c.db.ExecContext(ctx, `INSERT INTO analytics_stints(session_id, branch_id, car_idx, kind, idx, start_lap, end_lap, compound, laps, best_ms, median_ms,
		degradation_ms_per_lap, wear_start_pct, wear_end_pct, fuel_start_kg, fuel_end_kg, ers_start_j, ers_end_j, conditions)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, sessionID, branchID, car, kind, idx, start.Lap, end.Lap, end.Compound, len(ls),
		best, med, slope, 0.0, 0.0, start.FuelStart, end.FuelEnd, start.ErsStart, end.ErsEnd, start.Conditions)
	return err
}

func degradation(times []int64) float64 {
	if len(times) < 3 {
		return 0
	}
	n := float64(len(times))
	var mx, my float64
	for i, v := range times {
		mx += float64(i)
		my += float64(v)
	}
	mx, my = mx/n, my/n
	var num, den float64
	for i, v := range times {
		num += (float64(i) - mx) * (float64(v) - my)
		den += (float64(i) - mx) * (float64(i) - mx)
	}
	if den == 0 {
		return 0
	}
	return num / den
}

func sessionKind(c *Catalog, sessionID string) string {
	var code sql.NullInt64
	if err := c.db.QueryRow(`SELECT json_extract(context_json, '$.session_type_code') FROM sessions WHERE id=?`, sessionID).Scan(&code); err != nil || !code.Valid {
		return "unknown"
	}
	return SessionType(int(code.Int64))
}

func (c *Catalog) UpsertSessionAnalytics(ctx context.Context, sessionID, branchID, method string) error {
	laps, err := c.LapAnalytics(ctx, sessionID, branchID, -1)
	if err != nil {
		return err
	}
	var best, theo int64
	var clean, total int
	var avgSum, top float64
	for _, l := range laps {
		total++
		if !l.Valid || !l.Complete || l.LapMs <= 0 {
			continue
		}
		clean++
		if best == 0 || l.LapMs < best {
			best = l.LapMs
		}
		if l.Sector1Ms > 0 && l.Sector2Ms > 0 && l.Sector3Ms > 0 {
			v := l.Sector1Ms + l.Sector2Ms + l.Sector3Ms
			if theo == 0 || v < theo {
				theo = v
			}
		}
		avgSum += l.AvgSpeed
		if l.TopSpeed > top {
			top = l.TopSpeed
		}
	}
	avg := 0.0
	if clean > 0 {
		avg = avgSum / float64(clean)
	}
	_, err = c.db.ExecContext(ctx, `INSERT INTO session_analytics(session_id, branch_id, best_lap_ms, theoretical_best_ms, clean_laps, total_laps, avg_speed_mps, top_speed_mps, derived_at_ms, method_version)
		VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(session_id) DO UPDATE SET branch_id=excluded.branch_id, best_lap_ms=excluded.best_lap_ms,
		theoretical_best_ms=excluded.theoretical_best_ms, clean_laps=excluded.clean_laps, total_laps=excluded.total_laps,
		avg_speed_mps=excluded.avg_speed_mps, top_speed_mps=excluded.top_speed_mps, derived_at_ms=excluded.derived_at_ms, method_version=excluded.method_version`,
		sessionID, branchID, best, theo, clean, total, avg, top, time.Now().UnixMilli(), method)
	return err
}
