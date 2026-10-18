package engineer

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

//go:embed prompts/base.md prompts/*-strategy.md prompts/live-question.md prompts/post-race.md prompts/setup-research.md prompts/manifest.json prompts/tools.json
var bundle embed.FS

const PromptVersion = "1.0.0"

var profileFile = map[string]string{
	"scheduled_strategy":  "prompts/scheduled-strategy.md",
	"continuous_strategy": "prompts/continuous-strategy.md",
	"live_question":       "prompts/live-question.md",
	"post_race":           "prompts/post-race.md",
	"setup_research":      "prompts/setup-research.md",
}

// VerifyBundle checks that the embedded prompt files still match the authored
// manifest. It never rewrites or regenerates them.
func VerifyBundle() error {
	raw, err := bundle.ReadFile("prompts/manifest.json")
	if err != nil {
		return err
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}
	hashes, _ := m["sha256"].(map[string]any)
	for name, hv := range hashes {
		want, _ := hv.(string)
		data, err := bundle.ReadFile("prompts/" + name)
		if err != nil {
			return fmt.Errorf("prompt %s missing: %w", name, err)
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != want {
			return fmt.Errorf("prompt %s hash %s does not match manifest %s", name, got, want)
		}
	}
	return nil
}

// Assemble builds the system text in the documented order: base, exactly one
// profile file, then a short application-generated output-schema instruction.
func Assemble(profile string) (string, error) {
	base, err := bundle.ReadFile("prompts/base.md")
	if err != nil {
		return "", err
	}
	file, ok := profileFile[profile]
	if !ok {
		return "", fmt.Errorf("unknown profile %q", profile)
	}
	prof, err := bundle.ReadFile(file)
	if err != nil {
		return "", err
	}
	inst := "\n\nReturn exactly one JSON object matching contract.AgentOutput schema 1.0.0. " +
		"Required fields: schema_version, invocation_id, session_id, branch_id, observation_seq, decision, confidence, notify, urgency, driver_message, plan, rationale_summary, evidence_ids, assumptions, predictions, recheck, analysis, limitations, sources, setup_changes. " +
		"plan requires action, pit_lap_start, pit_lap_end, compound, alternatives, candidate_id. " +
		"driver_message is at most 35 words and 240 characters; notify=false requires driver_message=null. " +
		"Evidence IDs may only cite observations, events, estimates or tool artifacts supplied in this invocation. " +
		"Do not include Markdown fences or prose outside the JSON object."
	return string(base) + "\n\n" + string(prof) + inst, nil
}

type toolSpec struct {
	Name        string
	Description string
	Profiles    []string
	InputSchema json.RawMessage
}

func loadTools() ([]toolSpec, error) {
	raw, err := bundle.ReadFile("prompts/tools.json")
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	arr, _ := doc["tools"].([]any)
	var out []toolSpec
	for _, tv := range arr {
		t, _ := tv.(map[string]any)
		s := toolSpec{Name: str(t, "name"), Description: str(t, "description")}
		if rawSchema, ok := t["input_schema"]; ok {
			b, _ := json.Marshal(rawSchema)
			s.InputSchema = b
		}
		if ps, ok := t["profiles"].([]any); ok {
			for _, p := range ps {
				if name, ok := p.(string); ok {
					s.Profiles = append(s.Profiles, name)
				}
			}
		}
		out = append(out, s)
	}
	return out, nil
}

func str(m map[string]any, k string) string { s, _ := m[k].(string); return s }

func toolsFor(profile string) ([]string, error) {
	specs, err := loadTools()
	if err != nil {
		return nil, err
	}
	var out []string
	for _, t := range specs {
		for _, p := range t.Profiles {
			if p == profile {
				out = append(out, t.Name)
			}
		}
	}
	return out, nil
}
