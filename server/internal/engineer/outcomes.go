package engineer

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"f1tele/server/internal/contract"
	"f1tele/server/internal/history"
)

// previousAccepted returns the newest accepted advice for one branch. Advice on
// an abandoned branch stays in the catalog but is never carried forward.
func (m *Manager) previousAccepted(ctx context.Context, sessionID, branchID, excludeID string) (*contract.AgentOutput, error) {
	var raw string
	err := m.Catalog.DB().QueryRowContext(ctx, `SELECT output_json FROM invocations
		WHERE session_id=? AND branch_id=? AND status='accepted' AND output_json IS NOT NULL
		AND profile IN ('scheduled_strategy','continuous_strategy') AND id<>?
		ORDER BY COALESCE(started_at_ms,0) DESC LIMIT 1`, sessionID, branchID, excludeID).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out contract.AgentOutput
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RecordOutcomes compares a stored recommendation with the laps observed after
// it. Rows only move forward in lap coverage, so an earlier observation can
// never rewrite later evidence; the advice itself is never rewritten.
func RecordOutcomes(ctx context.Context, cat *history.Catalog, prev contract.AgentOutput, laps []contract.LapSummary, currentLap int) ([]string, error) {
	outcomes := compareOutcomes(prev, laps, currentLap)
	raw, err := json.Marshal(outcomes)
	if err != nil {
		return nil, err
	}
	_, err = cat.DB().ExecContext(ctx, `INSERT INTO strategy_outcomes(invocation_id, session_id, branch_id, observed_through_lap, outcomes_json, created_at_ms)
		VALUES(?,?,?,?,?,?)
		ON CONFLICT(invocation_id) DO UPDATE SET outcomes_json=excluded.outcomes_json, observed_through_lap=excluded.observed_through_lap
		WHERE excluded.observed_through_lap > strategy_outcomes.observed_through_lap`,
		prev.InvocationID, prev.SessionID, prev.BranchID, currentLap, string(raw), time.Now().UnixMilli())
	if err != nil {
		return nil, err
	}
	var stored string
	if err := cat.DB().QueryRowContext(ctx, `SELECT outcomes_json FROM strategy_outcomes WHERE invocation_id=?`, prev.InvocationID).Scan(&stored); err != nil {
		return nil, err
	}
	var out []string
	if err := json.Unmarshal([]byte(stored), &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []string{}
	}
	return out, nil
}

// compareOutcomes states only what the catalog observed. A missing stop is not
// claimed until the planned window has closed.
func compareOutcomes(prev contract.AgentOutput, laps []contract.LapSummary, currentLap int) []string {
	out := []string{}
	cutoff, err := strconv.ParseUint(prev.ObservationSeq, 10, 64)
	if err != nil {
		return out
	}
	var after []contract.LapSummary
	for _, l := range laps {
		end, err := strconv.ParseUint(l.EndSeq, 10, 64)
		if err == nil && end > cutoff {
			after = append(after, l)
		}
	}
	var pit *contract.LapSummary
	for i := range after {
		if after[i].PitLap {
			pit = &after[i]
			break
		}
	}
	plan := prev.Plan
	switch {
	case plan.Action == "pit" && plan.PitLapStart != nil:
		if pit != nil {
			if pit.Lap != *plan.PitLapStart {
				out = append(out, fmt.Sprintf("Planned the stop on lap %d; observed it on lap %d.", *plan.PitLapStart, pit.Lap))
			} else {
				out = append(out, fmt.Sprintf("Planned the stop on lap %d; observed on schedule.", *plan.PitLapStart))
			}
			if plan.Compound != nil {
				if observed, ok := compoundAfter(after, pit.Lap); ok && !strings.EqualFold(strings.TrimSpace(observed), strings.TrimSpace(*plan.Compound)) {
					out = append(out, fmt.Sprintf("Planned %s after the stop; observed %s.", *plan.Compound, observed))
				}
			}
		} else if plan.PitLapEnd != nil && plan.PitLapStart != nil && currentLap > *plan.PitLapEnd {
			out = append(out, fmt.Sprintf("Planned a stop by lap %d; no stop observed through lap %d.", *plan.PitLapEnd, currentLap))
		} else if plan.PitLapEnd == nil && currentLap > *plan.PitLapStart {
			out = append(out, fmt.Sprintf("Planned a stop on lap %d; no stop observed through lap %d.", *plan.PitLapStart, currentLap))
		}
	case plan.Action == "hold" && pit != nil:
		out = append(out, fmt.Sprintf("Advice was to stay out; a stop was observed on lap %d.", pit.Lap))
	}
	return out
}

func compoundAfter(laps []contract.LapSummary, pitLap int) (string, bool) {
	for _, l := range laps {
		if l.Lap <= pitLap || l.Compound.Availability != "available" {
			continue
		}
		if s, ok := l.Compound.Value.(string); ok && s != "" {
			return s, true
		}
	}
	return "", false
}
