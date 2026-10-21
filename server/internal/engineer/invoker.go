package engineer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"f1tele/server/internal/contract"
	"f1tele/server/internal/history"
	"f1tele/server/internal/state"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

type Invoker struct {
	Backend ToolBackend
}

type InvokeRequest struct {
	Profile        string
	Input          contract.AgentInput
	Endpoint       contract.Endpoint
	APIKey         string
	Snapshot       state.Snapshot
	SystemPrompt   string
	PromptRevision string
	PromptHash     string
	ConversationID string
	History        []history.AgentMessage
	Generation     contract.GenerationConfig
	Activity       *Activity
	MessageSink    func(history.AgentMessage)
}

type InvokeResult struct {
	Text             string
	Requests         int
	InputTokens      *int64
	OutputTokens     *int64
	TotalTokens      *int64
	LastPromptTokens int
}

// Run retains the original bounded-call entry point for tests and callers that
// do not need conversation replay.
func (i *Invoker) Run(ctx context.Context, profile string, input contract.AgentInput, endpoint contract.Endpoint, apiKey string, snapshot state.Snapshot) (string, int, error) {
	system, _, _, err := AssembleCurrent(ctx, profile, nil)
	if err != nil {
		return "", 0, err
	}
	res, err := i.RunRequest(ctx, InvokeRequest{Profile: profile, Input: input, Endpoint: endpoint, APIKey: apiKey, Snapshot: snapshot, SystemPrompt: system})
	return res.Text, res.Requests, err
}

func (i *Invoker) RunRequest(ctx context.Context, req InvokeRequest) (InvokeResult, error) {
	if err := VerifyBundle(); err != nil {
		return InvokeResult{}, err
	}
	if req.SystemPrompt == "" {
		system, rev, hash, err := AssembleCurrent(ctx, req.Profile, nil)
		if err != nil {
			return InvokeResult{}, err
		}
		req.SystemPrompt, req.PromptRevision, req.PromptHash = system, rev, hash
	}
	tools, err := BuildTools(req.Profile, i.Backend, req.Input.Observation)
	if err != nil {
		return InvokeResult{}, err
	}
	if req.Endpoint.Protocol == "responses" {
		return i.runResponses(ctx, req, tools)
	}
	return i.runChat(ctx, req, tools)
}

func (i *Invoker) runChat(ctx context.Context, req InvokeRequest, tools []tool.BaseTool) (InvokeResult, error) {
	cm, err := buildModel(ctx, req.Endpoint, req.APIKey, req.Generation, req.Input.Limits.MaxOutputTokens)
	if err != nil {
		return InvokeResult{}, err
	}
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name: req.Profile, Description: "bounded race engineer", Instruction: req.SystemPrompt, Model: cm,
		ToolsConfig:   adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: tools}},
		MaxIterations: req.Input.Limits.MaxSteps,
	})
	if err != nil {
		return InvokeResult{}, err
	}
	messages := replayMessages(req.History)
	user, _ := json.Marshal(req.Input)
	userMessage := schema.UserMessage(string(user))
	messages = append(messages, userMessage)
	if req.MessageSink != nil {
		req.MessageSink(history.AgentMessage{Role: "user", Content: json.RawMessage(user), InvocationID: req.Input.InvocationID})
	}
	req.Activity.Emit(ctx, history.AgentEvent{
		InvocationID: req.Input.InvocationID, SessionID: req.Input.Observation.Context.SessionID,
		BranchID: req.Input.Observation.Context.BranchID, Profile: req.Profile, Kind: "provider_request",
	}, map[string]any{"model": req.Endpoint.Model, "attempt": 1, "input_tokens_estimate": estimateTokens(user)})
	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent})
	it := runner.Run(ctx, messages)
	var final string
	requests := 1
	var promptTokens, completionTokens, totalTokens int
	for {
		ev, ok := it.Next()
		if !ok {
			break
		}
		if ev.Err != nil {
			req.Activity.Emit(ctx, history.AgentEvent{
				InvocationID: req.Input.InvocationID, SessionID: req.Input.Observation.Context.SessionID,
				BranchID: req.Input.Observation.Context.BranchID, Profile: req.Profile, Kind: "error",
			}, map[string]any{"message": ev.Err.Error()})
			return InvokeResult{Requests: requests}, ev.Err
		}
		if ev.Output == nil || ev.Output.MessageOutput == nil || ev.Output.MessageOutput.Message == nil {
			continue
		}
		m := ev.Output.MessageOutput.Message
		if m.ResponseMeta != nil && m.ResponseMeta.Usage != nil {
			u := m.ResponseMeta.Usage
			if u.PromptTokens > 0 {
				promptTokens = u.PromptTokens
			}
			if u.CompletionTokens > 0 {
				completionTokens += u.CompletionTokens
			}
			if u.TotalTokens > 0 {
				totalTokens = u.TotalTokens
			}
		}
		if len(m.ToolCalls) > 0 {
			requests++
			for _, call := range m.ToolCalls {
				req.Activity.Emit(ctx, history.AgentEvent{
					InvocationID: req.Input.InvocationID, SessionID: req.Input.Observation.Context.SessionID,
					BranchID: req.Input.Observation.Context.BranchID, Profile: req.Profile, Kind: "tool_call",
				}, map[string]any{"id": call.ID, "name": call.Function.Name, "arguments": call.Function.Arguments})
			}
			if req.MessageSink != nil {
				content, _ := json.Marshal(map[string]any{"tool_calls": m.ToolCalls})
				req.MessageSink(history.AgentMessage{Role: "assistant", Content: content, InvocationID: req.Input.InvocationID})
			}
		}
		if m.Role == schema.Tool {
			req.Activity.Emit(ctx, history.AgentEvent{
				InvocationID: req.Input.InvocationID, SessionID: req.Input.Observation.Context.SessionID,
				BranchID: req.Input.Observation.Context.BranchID, Profile: req.Profile, Kind: "tool_result",
			}, map[string]any{"tool_call_id": m.ToolCallID, "name": m.ToolName, "content": m.Content})
			if req.MessageSink != nil {
				content, _ := json.Marshal(m.Content)
				req.MessageSink(history.AgentMessage{Role: "tool", Content: content, ToolCallID: m.ToolCallID, InvocationID: req.Input.InvocationID})
			}
		}
		if strings.TrimSpace(m.Content) != "" {
			final = m.Content
			if req.MessageSink != nil {
				content, _ := json.Marshal(m.Content)
				req.MessageSink(history.AgentMessage{Role: "assistant", Content: content, InvocationID: req.Input.InvocationID})
			}
		}
	}
	if strings.TrimSpace(final) == "" {
		err := fmt.Errorf("provider returned no terminal message")
		req.Activity.Emit(ctx, history.AgentEvent{
			InvocationID: req.Input.InvocationID, SessionID: req.Input.Observation.Context.SessionID,
			BranchID: req.Input.Observation.Context.BranchID, Profile: req.Profile, Kind: "error",
		}, map[string]any{"message": err.Error()})
		return InvokeResult{Requests: requests}, err
	}
	out := InvokeResult{Text: final, Requests: requests, LastPromptTokens: promptTokens}
	if promptTokens > 0 {
		v := int64(promptTokens)
		out.InputTokens = &v
	}
	if completionTokens > 0 {
		v := int64(completionTokens)
		out.OutputTokens = &v
	}
	if totalTokens > 0 {
		v := int64(totalTokens)
		out.TotalTokens = &v
	}
	req.Activity.Emit(ctx, history.AgentEvent{
		InvocationID: req.Input.InvocationID, SessionID: req.Input.Observation.Context.SessionID,
		BranchID: req.Input.Observation.Context.BranchID, Profile: req.Profile, Kind: "provider_response",
	}, map[string]any{"requests": requests, "input_tokens": promptTokens, "output_tokens": completionTokens, "total_tokens": totalTokens})
	return out, nil
}

func replayMessages(historyMessages []history.AgentMessage) []*schema.Message {
	out := make([]*schema.Message, 0, len(historyMessages))
	for _, h := range historyMessages {
		if h.CompactedInto != "" {
			continue
		}
		switch h.Role {
		case "user":
			out = append(out, schema.UserMessage(rawText(h.Content)))
		case "assistant":
			var text string
			if json.Unmarshal(h.Content, &text) == nil {
				out = append(out, schema.AssistantMessage(text, nil))
				continue
			}
			var obj struct {
				Text      string            `json:"text"`
				ToolCalls []schema.ToolCall `json:"tool_calls"`
			}
			_ = json.Unmarshal(h.Content, &obj)
			out = append(out, schema.AssistantMessage(obj.Text, obj.ToolCalls))
		case "tool":
			out = append(out, &schema.Message{Role: schema.Tool, Content: rawText(h.Content), ToolCallID: h.ToolCallID})
		}
	}
	return out
}

func rawText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return string(raw)
}

func estimateTokens(raw []byte) int {
	return (len(raw) + 3) / 4
}

func buildModel(ctx context.Context, e contract.Endpoint, apiKey string, gen contract.GenerationConfig, maxOut int) (model.ChatModel, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("endpoint %s has no configured key", e.ID)
	}
	max := maxOut
	if max <= 0 {
		max = 2048
	}
	var temp, topP *float32
	if gen.Temperature != nil {
		v := float32(*gen.Temperature)
		temp = &v
	}
	if gen.TopP != nil {
		v := float32(*gen.TopP)
		topP = &v
	}
	switch e.Protocol {
	case "chat_completions":
		return openai.NewChatModel(ctx, &openai.ChatModelConfig{APIKey: apiKey, BaseURL: e.BaseURL, Model: e.Model, MaxCompletionTokens: &max, Temperature: temp, TopP: topP})
	case "responses":
		return nil, fmt.Errorf("responses protocol uses the agentic adapter")
	}
	return nil, fmt.Errorf("unsupported protocol %q", e.Protocol)
}
