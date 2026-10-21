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
	// Provider continuation replay stays disabled: every run replays the durable
	// transcript instead of PreviousResponseID, so compaction cannot carry stale
	// provider state into the next turn.
	messages := replayAgenticMessages(req.History)
	messages = append(messages, schema.UserAgenticMessage(string(body)))
	it := runner.Run(ctx, messages)
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
		var toolCalls []schema.ToolCall
		for _, block := range msg.ContentBlocks {
			if block.AssistantGenText != nil && strings.TrimSpace(block.AssistantGenText.Text) != "" {
				final = block.AssistantGenText.Text
			}
			if block.FunctionToolCall != nil {
				toolCalls = append(toolCalls, schema.ToolCall{ID: block.FunctionToolCall.CallID, Type: "function",
					Function: schema.FunctionCall{Name: block.FunctionToolCall.Name, Arguments: block.FunctionToolCall.Arguments}})
				req.Activity.Emit(ctx, history.AgentEvent{
					InvocationID: req.Input.InvocationID, SessionID: req.Input.Observation.Context.SessionID,
					BranchID: req.Input.Observation.Context.BranchID, Profile: req.Profile, Kind: "tool_call",
				}, map[string]any{"id": block.FunctionToolCall.CallID, "name": block.FunctionToolCall.Name, "arguments": block.FunctionToolCall.Arguments})
			}
			if block.FunctionToolResult != nil && req.MessageSink != nil {
				content, _ := json.Marshal(agenticToolResultText(block.FunctionToolResult))
				req.MessageSink(history.AgentMessage{Role: "tool", Content: content, ToolCallID: block.FunctionToolResult.CallID, InvocationID: req.Input.InvocationID})
			}
		}
		if len(toolCalls) > 0 {
			requests++
		}
		if len(toolCalls) > 0 && req.MessageSink != nil {
			content, _ := json.Marshal(map[string]any{"tool_calls": toolCalls})
			req.MessageSink(history.AgentMessage{Role: "assistant", Content: content, InvocationID: req.Input.InvocationID})
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

func agenticToolResultText(result *schema.FunctionToolResult) string {
	var b strings.Builder
	for _, block := range result.Content {
		if block.Type != schema.FunctionToolResultContentBlockTypeText || block.Text == nil {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(block.Text.Text)
	}
	if b.Len() == 0 {
		raw, _ := json.Marshal(result)
		return string(raw)
	}
	return b.String()
}

// replayAgenticMessages restores the durable transcript as typed Responses
// messages, including assistant tool calls and their tool results.
func replayAgenticMessages(messages []history.AgentMessage) []*schema.AgenticMessage {
	out := make([]*schema.AgenticMessage, 0, len(messages))
	for _, h := range messages {
		if h.CompactedInto != "" {
			continue
		}
		switch h.Role {
		case "user":
			out = append(out, schema.UserAgenticMessage(rawText(h.Content)))
		case "assistant":
			var text string
			if json.Unmarshal(h.Content, &text) == nil {
				out = append(out, &schema.AgenticMessage{Role: schema.AgenticRoleTypeAssistant,
					ContentBlocks: []*schema.ContentBlock{schema.NewContentBlock(&schema.AssistantGenText{Text: text})}})
				continue
			}
			var obj struct {
				Text      string            `json:"text"`
				ToolCalls []schema.ToolCall `json:"tool_calls"`
			}
			_ = json.Unmarshal(h.Content, &obj)
			blocks := make([]*schema.ContentBlock, 0, len(obj.ToolCalls)+1)
			if obj.Text != "" {
				blocks = append(blocks, schema.NewContentBlock(&schema.AssistantGenText{Text: obj.Text}))
			}
			for _, call := range obj.ToolCalls {
				blocks = append(blocks, schema.NewContentBlock(&schema.FunctionToolCall{CallID: call.ID, Name: call.Function.Name, Arguments: call.Function.Arguments}))
			}
			if len(blocks) > 0 {
				out = append(out, &schema.AgenticMessage{Role: schema.AgenticRoleTypeAssistant, ContentBlocks: blocks})
			}
		case "tool":
			out = append(out, &schema.AgenticMessage{Role: schema.AgenticRoleTypeUser, ContentBlocks: []*schema.ContentBlock{
				schema.NewContentBlock(&schema.FunctionToolResult{CallID: h.ToolCallID, Content: []*schema.FunctionToolResultContentBlock{
					{Type: schema.FunctionToolResultContentBlockTypeText, Text: &schema.UserInputText{Text: rawText(h.Content)}},
				}}),
			}})
		}
	}
	return out
}
