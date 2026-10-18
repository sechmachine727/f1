package engineer

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"f1tele/server/internal/analysis"
	"f1tele/server/internal/contract"
	"f1tele/server/internal/state"
	"f1tele/server/internal/strategy"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
)

type ToolBackend struct {
	Engine   *analysis.Engine
	Eval     *strategy.Evaluator
	Snapshot func() state.Snapshot
	Search   func(ctx context.Context, q map[string]any) (contract.SearchResult, error)
	Fetch    func(ctx context.Context, url, sourceID string) (contract.SourceInspection, error)
}

type boundTool struct {
	name   string
	desc   string
	schema *jsonschema.Schema
	run    func(ctx context.Context, args map[string]any) contract.ToolResult
}

func (t *boundTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: t.name, Desc: t.desc, ParamsOneOf: schema.NewParamsOneOfByJSONSchema(t.schema)}, nil
}

func (t *boundTool) InvokableRun(ctx context.Context, args string, opts ...tool.Option) (string, error) {
	var m map[string]any
	if err := json.Unmarshal([]byte(args), &m); err != nil {
		return "", err
	}
	raw, err := json.Marshal(t.run(ctx, m))
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// BuildTools registers exactly the tools.json entries allowed for the profile.
func BuildTools(profile string, b ToolBackend, obs contract.Observation) ([]tool.BaseTool, error) {
	specs, err := loadTools()
	if err != nil {
		return nil, err
	}
	var out []tool.BaseTool
	for _, spec := range specs {
		allowed := false
		for _, p := range spec.Profiles {
			if p == profile {
				allowed = true
			}
		}
		if !allowed {
			continue
		}
		var sc jsonschema.Schema
		if err := json.Unmarshal(spec.InputSchema, &sc); err != nil {
			return nil, err
		}
		t := &boundTool{name: spec.Name, desc: spec.Description, schema: &sc}
		switch spec.Name {
		case "read_telemetry_window":
			t.run = func(ctx context.Context, args map[string]any) contract.ToolResult {
				return b.readWindow(ctx, args, obs)
			}
		case "compare_laps":
			t.run = func(ctx context.Context, args map[string]any) contract.ToolResult {
				return b.compareLaps(ctx, args, obs)
			}
		case "evaluate_strategy":
			t.run = func(ctx context.Context, args map[string]any) contract.ToolResult { return b.evaluate(ctx, args, obs) }
		case "search_setups":
			t.run = func(ctx context.Context, args map[string]any) contract.ToolResult { return b.search(ctx, args) }
		case "fetch_setup_source":
			t.run = func(ctx context.Context, args map[string]any) contract.ToolResult { return b.fetch(ctx, args) }
		}
		out = append(out, t)
	}
	return out, nil
}

func (b ToolBackend) readWindow(ctx context.Context, args map[string]any, obs contract.Observation) contract.ToolResult {
	if b.Engine == nil {
		return toolErr("unavailable", "history is not available in this process", "none", "report the limitation")
	}
	from := str(args, "from_seq")
	to := str(args, "to_seq")
	cars := intsArg(args, "car_indices")
	channels := stringsArg(args, "channels")
	max := intArg(args, "max_points")
	if len(channels) > 16 || len(cars) > 24 || max > 20000 {
		return toolErr("invalid_window", "request exceeds channel, car or point limits", "narrow_range", "reduce channels, cars or points")
	}
	q := contract.Query{SessionID: obs.Context.SessionID, BranchID: obs.Context.BranchID, CarIndices: cars, Channels: channels, FromSessionTimeMs: 0, ToSessionTimeMs: 1 << 62, MaxPoints: max, Alignment: "time"}
	res, err := b.Engine.Query(ctx, q)
	if err != nil {
		return toolErr("window_failed", err.Error(), "narrow_range", "retry with fewer channels or a narrower range")
	}
	fromN, _ := strconv.ParseUint(from, 10, 64)
	toN, _ := strconv.ParseUint(to, 10, 64)
	for i := range res.Series {
		var kept []contract.SeriesPoint
		for _, p := range res.Series[i].Points {
			seq, _ := strconv.ParseUint(p.Seq, 10, 64)
			if seq >= fromN && seq <= toN {
				kept = append(kept, p)
			}
		}
		res.Series[i].Points = kept
	}
	out := contract.ReadWindowResult{Context: res.Context, Window: contract.Range{FromSeq: from, ToSeq: to}, Series: res.Series, Events: []contract.Event{}, Estimates: []contract.Estimate{}, Coverage: res.Coverage}
	return contract.ToolResult{Status: "success", Summary: "telemetry window returned", NextActions: []string{}, Artifacts: []contract.Ref{}, Data: out, Error: nil}
}

func (b ToolBackend) compareLaps(ctx context.Context, args map[string]any, obs contract.Observation) contract.ToolResult {
	if b.Engine == nil {
		return toolErr("unavailable", "history is not available in this process", "none", "report the limitation")
	}
	q := contract.CompareQuery{SessionID: obs.Context.SessionID, BranchID: obs.Context.BranchID, LapIDs: stringsArg(args, "lap_ids"), Channels: stringsArg(args, "channels"), MaxPoints: intArg(args, "max_points")}
	res, err := b.Engine.Compare(ctx, q)
	if err != nil {
		return toolErr("compare_failed", err.Error(), "none", "report the limitation")
	}
	return contract.ToolResult{Status: "success", Summary: "lap comparison returned", NextActions: []string{}, Artifacts: []contract.Ref{}, Data: res, Error: nil}
}

func (b ToolBackend) evaluate(ctx context.Context, args map[string]any, obs contract.Observation) contract.ToolResult {
	if b.Eval == nil || b.Snapshot == nil {
		return toolErr("unavailable", "strategy is not available in this process", "none", "report the limitation")
	}
	snap := b.Snapshot()
	wins, _ := args["pit_windows"].([]any)
	var pw []contract.PitWindowIn
	for _, w := range wins {
		m, _ := w.(map[string]any)
		pw = append(pw, contract.PitWindowIn{FirstLap: intArg(m, "first_lap"), LastLap: intArg(m, "last_lap"), Compound: str(m, "compound")})
	}
	res, err := b.Eval.Evaluate(ctx, contract.StrategyQuery{SessionID: obs.Context.SessionID, BranchID: obs.Context.BranchID, ObservationSeq: str(args, "observation_seq"), PitWindows: pw}, snap)
	if err != nil {
		if errors.Is(err, strategy.ErrUnsupportedMode) {
			return toolErr("unsupported_mode", err.Error(), "none", "use lap comparison instead")
		}
		return toolErr("strategy_failed", err.Error(), "none", "report the limitation")
	}
	return contract.ToolResult{Status: "success", Summary: "deterministic strategy candidates returned", NextActions: []string{}, Artifacts: []contract.Ref{}, Data: res, Error: nil}
}

func (b ToolBackend) search(ctx context.Context, args map[string]any) contract.ToolResult {
	if b.Search == nil {
		return toolErr("search_disabled", "no search provider is configured", "manual", "supply a direct source URL")
	}
	res, err := b.Search(ctx, args)
	if err != nil {
		return toolErr("search_failed", err.Error(), "manual", "report the provider error")
	}
	return contract.ToolResult{Status: "success", Summary: "search results require source inspection before numeric claims", NextActions: []string{"fetch_setup_source"}, Artifacts: []contract.Ref{}, Data: res, Error: nil}
}

func (b ToolBackend) fetch(ctx context.Context, args map[string]any) contract.ToolResult {
	if b.Fetch == nil {
		return toolErr("fetch_disabled", "source inspection is unavailable", "manual", "report the limitation")
	}
	res, err := b.Fetch(ctx, str(args, "url"), str(args, "source_id"))
	if err != nil {
		return toolErr("fetch_failed", err.Error(), "manual", "keep the source uninspected")
	}
	return contract.ToolResult{Status: "success", Summary: "source inspected", NextActions: []string{}, Artifacts: []contract.Ref{}, Data: res, Error: nil}
}

func toolErr(code, hint, retry, stop string) contract.ToolResult {
	return contract.ToolResult{Status: "error", Summary: "tool call failed", NextActions: []string{}, Artifacts: []contract.Ref{}, Data: nil, Error: &contract.ToolError{Code: code, CauseHint: hint, Retry: retry, StopCondition: stop}}
}

func intArg(m map[string]any, k string) int {
	switch v := m[k].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}
func stringsArg(m map[string]any, k string) []string {
	arr, _ := m[k].([]any)
	var out []string
	for _, x := range arr {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
func intsArg(m map[string]any, k string) []int {
	arr, _ := m[k].([]any)
	var out []int
	for _, x := range arr {
		if f, ok := x.(float64); ok {
			out = append(out, int(f))
		}
	}
	return out
}
