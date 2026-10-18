package engineer

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"f1tele/server/internal/contract"
)

var ErrInvalidOutput = errors.New("invalid agent output")

// ParseAndValidate enforces the terminal JSON contract and scope echo.
func ParseAndValidate(text string, in contract.AgentInput) (contract.AgentOutput, error) {
	text = strings.TrimSpace(text)
	var out contract.AgentOutput
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return out, fmt.Errorf("%w: %v", ErrInvalidOutput, err)
	}
	if out.SchemaVersion != contract.SchemaVersion {
		return out, fmt.Errorf("%w: schema_version", ErrInvalidOutput)
	}
	if out.InvocationID != in.InvocationID || out.SessionID != in.Observation.Context.SessionID || out.BranchID != in.Observation.Context.BranchID || out.ObservationSeq != in.Observation.ObservationSeq {
		return out, fmt.Errorf("%w: echoed scope mismatch", ErrInvalidOutput)
	}
	if !out.Notify && out.DriverMessage != nil {
		return out, fmt.Errorf("%w: notify=false requires a null driver_message", ErrInvalidOutput)
	}
	if out.DriverMessage != nil {
		if len([]rune(*out.DriverMessage)) > 240 || len(strings.Fields(*out.DriverMessage)) > 35 {
			return out, fmt.Errorf("%w: driver_message too long", ErrInvalidOutput)
		}
	}
	allowed := map[string]bool{}
	for _, m := range in.Observation.Metrics {
		allowed[m.ID] = true
	}
	for _, e := range in.Observation.Events {
		allowed[e.ID] = true
		allowed["seq:"+e.Seq] = true
	}
	for _, e := range in.Observation.Estimates {
		allowed[e.ID] = true
	}
	for _, id := range out.EvidenceIDs {
		if !allowed[id] {
			return out, fmt.Errorf("%w: unknown evidence id %s", ErrInvalidOutput, id)
		}
	}
	if out.Urgency == "critical" {
		critical := false
		for _, e := range in.Observation.Events {
			if e.Severity == "critical" {
				critical = true
			}
		}
		if !critical {
			return out, fmt.Errorf("%w: critical urgency without deterministic critical evidence", ErrInvalidOutput)
		}
	}
	if out.Plan.Action == "pit" {
		if out.Plan.PitLapStart == nil || out.Plan.PitLapEnd == nil || out.Plan.Compound == nil {
			return out, fmt.Errorf("%w: pit plan needs a window and compound", ErrInvalidOutput)
		}
		if *out.Plan.PitLapStart < 1 || *out.Plan.PitLapEnd < *out.Plan.PitLapStart {
			return out, fmt.Errorf("%w: invalid pit window", ErrInvalidOutput)
		}
	}
	return out, nil
}
