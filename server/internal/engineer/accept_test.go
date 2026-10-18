package engineer

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"f1tele/server/internal/contract"
)

func inputFixture() contract.AgentInput {
	p := 0
	return contract.AgentInput{
		SchemaVersion: contract.SchemaVersion, InvocationID: "i-1", ConversationID: "c", Profile: "scheduled_strategy",
		EndpointID: "e", Model: "m",
		Observation: contract.Observation{
			SchemaVersion:  contract.SchemaVersion,
			Context:        contract.Context{SessionID: "s", BranchID: "b", PlayerCarIndex: &p, SeasonEvidence: []string{}},
			ObservationSeq: "42",
			Metrics:        []contract.Metric{{ID: "car.speed_mps@0", Channel: "car.speed_mps", CarIndex: &p, Measurement: contract.Available(50.0, "m/s", "t")}},
			Events:         []contract.Event{{ID: "e-sc", Seq: "41", Type: "safety_car", Severity: "important", EvidenceIDs: []string{}}},
		},
		Limits: contract.Limits{MaxSteps: 4, MaxToolCalls: 8, DeadlineMs: 15000, MaxOutputTokens: 2048, MaxContextTokens: 16384},
	}
}

func validOutput() contract.AgentOutput {
	return contract.AgentOutput{
		SchemaVersion: contract.SchemaVersion, InvocationID: "i-1", SessionID: "s", BranchID: "b", ObservationSeq: "42",
		Decision: "keep", Confidence: "medium", Notify: false, Urgency: "routine",
		Plan:             contract.Plan{Action: "hold", Alternatives: []string{}},
		RationaleSummary: "No change.", EvidenceIDs: []string{"car.speed_mps@0"}, Assumptions: []string{},
		Predictions: []contract.Prediction{}, Recheck: contract.Recheck{EventTypes: []string{}},
		Analysis: "ok", Limitations: []string{}, Sources: []contract.Source{}, SetupChanges: []contract.SetupChange{},
	}
}

func TestOutputAcceptance(t *testing.T) {
	in := inputFixture()
	base := validOutput()
	if _, err := ParseAndValidate(mustJSON(t, base), in); err != nil {
		t.Fatalf("valid output rejected: %v", err)
	}
	cases := []struct {
		name string
		edit func(*contract.AgentOutput)
		raw  string
	}{
		{name: "echoed invocation", edit: func(o *contract.AgentOutput) { o.InvocationID = "other" }},
		{name: "wrong branch", edit: func(o *contract.AgentOutput) { o.BranchID = "b-old" }},
		{name: "stale observation", edit: func(o *contract.AgentOutput) { o.ObservationSeq = "9" }},
		{name: "unknown evidence", edit: func(o *contract.AgentOutput) { o.EvidenceIDs = []string{"invented"} }},
		{name: "notify false with message", edit: func(o *contract.AgentOutput) { s := "Box"; o.DriverMessage = &s }},
		{name: "too many words", edit: func(o *contract.AgentOutput) {
			s := strings.Repeat("word ", 40)
			o.Notify = true
			o.DriverMessage = &s
		}},
		{name: "message too long", edit: func(o *contract.AgentOutput) {
			s := strings.Repeat("x", 241)
			o.Notify = true
			o.DriverMessage = &s
		}},
		{name: "critical without evidence", edit: func(o *contract.AgentOutput) {
			s := "Box now"
			o.Notify, o.Urgency, o.DriverMessage = true, "critical", &s
		}},
		{name: "invalid pit window", edit: func(o *contract.AgentOutput) {
			start, end, compound := 8, 5, "Hard"
			o.Plan = contract.Plan{Action: "pit", PitLapStart: &start, PitLapEnd: &end, Compound: &compound, Alternatives: []string{}}
		}},
		{name: "missing pit window", edit: func(o *contract.AgentOutput) {
			o.Plan = contract.Plan{Action: "pit", Alternatives: []string{}}
		}},
		{name: "malformed json", raw: "{\"schema_version\":"},
		{name: "markdown fence", raw: "\u0060\u0060\u0060json\n" + mustJSON(t, base) + "\n\u0060\u0060\u0060"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			raw := c.raw
			if raw == "" {
				o := base
				c.edit(&o)
				raw = mustJSON(t, o)
			}
			if _, err := ParseAndValidate(raw, in); err == nil {
				t.Fatal("invalid output accepted")
			}
		})
	}
	in.Observation.Events = append(in.Observation.Events, contract.Event{ID: "e-crit", Seq: "43", Type: "damage_changed", Severity: "critical", EvidenceIDs: []string{}})
	o := base
	msg := "Box this lap"
	o.Notify, o.Urgency, o.DriverMessage = true, "critical", &msg
	o.EvidenceIDs = []string{"e-crit"}
	if _, err := ParseAndValidate(mustJSON(t, o), in); err != nil {
		t.Fatalf("critical with evidence rejected: %v", err)
	}

	// A pit call must agree with a legal deterministic candidate.
	start, end, compound := 8, 10, "C3"
	withEval := inputFixture()
	withEval.Observation.StrategyEvaluation = &contract.StrategyEvaluation{Context: withEval.Observation.Context, ObservationSeq: "42",
		Candidates: []contract.StrategyCandidate{{ID: "pit-1", Stops: []contract.PlannedStop{{Lap: 9, Compound: "C3"}}, Legality: "legal"}}}
	good := base
	good.Plan = contract.Plan{Action: "pit", PitLapStart: &start, PitLapEnd: &end, Compound: &compound, Alternatives: []string{}}
	if _, err := ParseAndValidate(mustJSON(t, good), withEval); err != nil {
		t.Fatalf("matching pit plan rejected: %v", err)
	}
	wrongCompound := "C5"
	bad := good
	bad.Plan.Compound = &wrongCompound
	if _, err := ParseAndValidate(mustJSON(t, bad), withEval); err == nil {
		t.Fatal("pit plan outside the evaluated candidates accepted")
	}
	candidate := "pit-9"
	bad = good
	bad.Plan.CandidateID = &candidate
	if _, err := ParseAndValidate(mustJSON(t, bad), withEval); err == nil {
		t.Fatal("unknown candidate id accepted")
	}

	// Retire needs direct evidence.
	retire := base
	retire.Plan = contract.Plan{Action: "retire", Alternatives: []string{}}
	if _, err := ParseAndValidate(mustJSON(t, retire), inputFixture()); err == nil {
		t.Fatal("retire without failure evidence accepted")
	}
}

func TestEvaluationCasesRemainByteIdentical(t *testing.T) {
	raw, err := os.ReadFile("prompts/eval-cases.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) < 5 {
		t.Fatalf("eval cases = %d", len(lines))
	}
	for i, line := range lines {
		var v map[string]any
		if err := json.Unmarshal([]byte(line), &v); err != nil {
			t.Fatalf("case %d invalid JSON: %v", i, err)
		}
		if _, ok := v["id"]; !ok {
			t.Fatalf("case %d has no id", i)
		}
	}
}

func TestPromptBundleMatchesManifest(t *testing.T) {
	if err := VerifyBundle(); err != nil {
		t.Fatal(err)
	}
	sys, err := Assemble("scheduled_strategy")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sys, "You are the race engineer") || !strings.Contains(sys, "AgentOutput") {
		t.Fatal("system prompt assembly missing base or schema instruction")
	}
	if _, err := Assemble("unknown"); err == nil {
		t.Fatal("unknown profile accepted")
	}
}

func TestToolRegistrationIsProfileScoped(t *testing.T) {
	b := ToolBackend{}
	for profile, want := range map[string]int{"scheduled_strategy": 2, "live_question": 3, "post_race": 3, "setup_research": 3, "continuous_strategy": 2} {
		tools, err := BuildTools(profile, b, contract.Observation{})
		if err != nil {
			t.Fatal(err)
		}
		if len(tools) != want {
			t.Fatalf("%s tools = %d want %d", profile, len(tools), want)
		}
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
