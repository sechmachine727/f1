package engineer

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"f1tele/server/internal/history"

	"github.com/cloudwego/eino-ext/components/model/agenticopenai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

var errNoTerminalMessage = errors.New("provider returned no terminal message")

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }

func (i *Invoker) runResponses(ctx context.Context, req InvokeRequest, tools []tool.BaseTool) (InvokeResult, error) {
	max := req.Input.Limits.MaxOutputTokens
	if max <= 0 {
		max = 2048
	}
	var temp, topP *float32
	if req.Generation.Temperature != nil {
		v := float32(*req.Generation.Temperature)
		temp = &v
	}
	if req.Generation.TopP != nil {
		v := float32(*req.Generation.TopP)
		topP = &v
	}
	cm, err := agenticopenai.NewResponsesModel(ctx, &agenticopenai.ResponsesConfig{
		APIKey: req.APIKey, BaseURL: req.Endpoint.BaseURL, Model: req.Endpoint.Model, MaxTokens: &max,
		Temperature: temp, TopP: topP, ParallelToolCalls: req.Generation.ParallelToolCalls,
	})
	if err != nil {
		return InvokeResult{}, err
	}
	agent, err := adk.NewTypedChatModelAgent[*schema.AgenticMessage](ctx, &adk.TypedChatModelAgentConfig[*schema.AgenticMessage]{
		Name: req.Profile, Description: "bounded race engineer", Instruction: req.SystemPrompt, Model: cm,
		ToolsConfig:   adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: tools}},
		MaxIterations: req.Input.Limits.MaxSteps,
	})
	if err != nil {
		return InvokeResult{}, err
	}
	runner := adk.NewTypedRunner[*schema.AgenticMessage](adk.TypedRunnerConfig[*schema.AgenticMessage]{Agent: agent})
	body, err := jsonMarshal(req.Input)
	if err != nil {
		return InvokeResult{}, err
	}
	if req.MessageSink != nil {
		req.MessageSink(history.AgentMessage{Role: "user", Content: json.RawMessage(body), InvocationID: req.Input.InvocationID})
	}
	req.Activity.Emit(ctx, history.AgentEvent{
		InvocationID: req.Input.InvocationID, SessionID: req.Input.Observation.Context.SessionID,
		BranchID: req.Input.Observation.Context.BranchID, Profile: req.Profile, Kind: "provider_request",
	}, map[string]any{"model": req.Endpoint.Model, "protocol": "responses", "attempt": 1, "input_tokens_estimate": estimateTokens(body)})
	it := runner.Run(ctx, []*schema.AgenticMessage{schema.UserAgenticMessage(string(body))})
	final := ""
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
		msg := ev.Output.MessageOutput.Message
		if msg.ResponseMeta != nil && msg.ResponseMeta.TokenUsage != nil {
			u := msg.ResponseMeta.TokenUsage
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
		for _, block := range msg.ContentBlocks {
			if block.AssistantGenText != nil && strings.TrimSpace(block.AssistantGenText.Text) != "" {
				final = block.AssistantGenText.Text
			}
			if block.FunctionToolCall != nil {
				requests++
				req.Activity.Emit(ctx, history.AgentEvent{
					InvocationID: req.Input.InvocationID, SessionID: req.Input.Observation.Context.SessionID,
					BranchID: req.Input.Observation.Context.BranchID, Profile: req.Profile, Kind: "tool_call",
				}, map[string]any{"id": block.FunctionToolCall.CallID, "name": block.FunctionToolCall.Name, "arguments": block.FunctionToolCall.Arguments})
			}
		}
	}
	if strings.TrimSpace(final) == "" {
		req.Activity.Emit(ctx, history.AgentEvent{
			InvocationID: req.Input.InvocationID, SessionID: req.Input.Observation.Context.SessionID,
			BranchID: req.Input.Observation.Context.BranchID, Profile: req.Profile, Kind: "error",
		}, map[string]any{"message": errNoTerminalMessage.Error()})
		return InvokeResult{Requests: requests}, errNoTerminalMessage
	}
	if req.MessageSink != nil {
		content, _ := json.Marshal(final)
		req.MessageSink(history.AgentMessage{Role: "assistant", Content: content, InvocationID: req.Input.InvocationID})
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
