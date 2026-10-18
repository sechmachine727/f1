package setup

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"f1tele/server/internal/contract"
)

func (s *Service) List(ctx context.Context) ([]contract.SetupRecord, error) {
	rows, err := s.Catalog.DB().QueryContext(ctx, "SELECT id, session_id, branch_id, invocation_id, output_json, created_at_ms FROM setup_records ORDER BY created_at_ms DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []contract.SetupRecord
	for rows.Next() {
		var rec contract.SetupRecord
		var raw string
		if err := rows.Scan(&rec.ID, &rec.SessionID, &rec.BranchID, &rec.InvocationID, &raw, &rec.CreatedAtMs); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(raw), &rec.Output)
		rec.Sources = []contract.SourceInspection{}
		rec.Experiments = []contract.Experiment{}
		srows, _ := s.Catalog.DB().QueryContext(ctx, "SELECT source_json, inspected, truncated FROM setup_sources WHERE record_id=?", rec.ID)
		for srows != nil && srows.Next() {
			var sj string
			var inspected, truncated int
			if err := srows.Scan(&sj, &inspected, &truncated); err == nil {
				var si contract.SourceInspection
				_ = json.Unmarshal([]byte(sj), &si)
				si.Inspected, si.Truncated = inspected == 1, truncated == 1
				rec.Sources = append(rec.Sources, si)
			}
		}
		if srows != nil {
			srows.Close()
		}
		erows, _ := s.Catalog.DB().QueryContext(ctx, "SELECT id, record_id, baseline_json, trial_json, conditions, outcome, created_at_ms FROM setup_experiments WHERE record_id=?", rec.ID)
		for erows != nil && erows.Next() {
			var e contract.Experiment
			var bj, tj string
			if err := erows.Scan(&e.ID, &e.RecordID, &bj, &tj, &e.Conditions, &e.Outcome, &e.CreatedAtMs); err == nil {
				_ = json.Unmarshal([]byte(bj), &e.BaselineLapIDs)
				_ = json.Unmarshal([]byte(tj), &e.TrialLapIDs)
				rec.Experiments = append(rec.Experiments, e)
			}
		}
		if erows != nil {
			erows.Close()
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (s *Service) Create(ctx context.Context, invocationID string) (contract.SetupRecord, error) {
	var raw string
	var sessionID, branchID string
	err := s.Catalog.DB().QueryRowContext(ctx, "SELECT output_json, session_id, branch_id FROM invocations WHERE id=? AND status='accepted'", invocationID).Scan(&raw, &sessionID, &branchID)
	if errors.Is(err, sql.ErrNoRows) {
		return contract.SetupRecord{}, errors.New("accepted invocation not found")
	}
	if err != nil {
		return contract.SetupRecord{}, err
	}
	var out contract.AgentOutput
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return contract.SetupRecord{}, err
	}
	rec := contract.SetupRecord{ID: fmt.Sprintf("setup-%d", time.Now().UnixNano()), SessionID: sessionID, BranchID: branchID, InvocationID: invocationID, Output: out, Sources: []contract.SourceInspection{}, Experiments: []contract.Experiment{}, CreatedAtMs: time.Now().UnixMilli()}
	outRaw, _ := json.Marshal(out)
	if _, err := s.Catalog.DB().ExecContext(ctx, "INSERT INTO setup_records(id, session_id, branch_id, invocation_id, output_json, created_at_ms) VALUES(?,?,?,?,?,?)", rec.ID, sessionID, branchID, invocationID, string(outRaw), rec.CreatedAtMs); err != nil {
		return rec, err
	}
	for i, src := range out.Sources {
		si := contract.SourceInspection{Source: src, Inspected: false, Text: "", Truncated: false}
		rawSI, _ := json.Marshal(si)
		s.Catalog.DB().ExecContext(ctx, "INSERT INTO setup_sources(id, record_id, source_json, content_hash, inspected, truncated) VALUES(?,?,?,?,?,?)", fmt.Sprintf("%s-src-%d", rec.ID, i), rec.ID, string(rawSI), nil, 0, 0)
	}
	return rec, nil
}

func (s *Service) AddExperiment(ctx context.Context, recordID string, in contract.ExperimentCreate) (contract.Experiment, error) {
	var exists int
	if err := s.Catalog.DB().QueryRowContext(ctx, "SELECT COUNT(1) FROM setup_records WHERE id=?", recordID).Scan(&exists); err != nil || exists == 0 {
		return contract.Experiment{}, errors.New("setup record not found")
	}
	e := contract.Experiment{ID: fmt.Sprintf("exp-%d", time.Now().UnixNano()), RecordID: recordID, BaselineLapIDs: in.BaselineLapIDs, TrialLapIDs: in.TrialLapIDs, Conditions: in.Conditions, Outcome: in.Outcome, CreatedAtMs: time.Now().UnixMilli()}
	bj, _ := json.Marshal(e.BaselineLapIDs)
	tj, _ := json.Marshal(e.TrialLapIDs)
	_, err := s.Catalog.DB().ExecContext(ctx, "INSERT INTO setup_experiments(id, record_id, baseline_json, trial_json, conditions, outcome, created_at_ms) VALUES(?,?,?,?,?,?,?)", e.ID, recordID, string(bj), string(tj), e.Conditions, e.Outcome, e.CreatedAtMs)
	return e, err
}
