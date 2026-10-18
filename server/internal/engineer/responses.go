package engineer

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"f1tele/server/internal/contract"

	"github.com/cloudwego/eino-ext/components/model/agenticopenai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

var errNoTerminalMessage = errors.New("provider returned no terminal message")

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }

// runResponses executes one bounded invocation through the Responses protocol.
// Provider continuation items and function-call IDs stay inside Eino's message
// history; only the terminal assistant text leaves this function.
func (i *Invoker) runResponses(ctx context.Context, profile, system string, input contract.AgentInput, endpoint contract.Endpoint, apiKey string, tools []tool.BaseTool) (string, int, error) {
	max := 2048
	cm, err := agenticopenai.NewResponsesModel(ctx, &agenticopenai.ResponsesConfig{
		APIKey: apiKey, BaseURL: endpoint.BaseURL, Model: endpoint.Model, MaxTokens: &max,
	})
	if err != nil {
		return "", 0, err
	}
	agent, err := adk.NewTypedChatModelAgent[*schema.AgenticMessage](ctx, &adk.TypedChatModelAgentConfig[*schema.AgenticMessage]{
		Name: profile, Description: "bounded race engineer", Instruction: system, Model: cm,
		ToolsConfig:   adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: tools}},
		MaxIterations: input.Limits.MaxSteps,
	})
	if err != nil {
		return "", 0, err
	}
	runner := adk.NewTypedRunner[*schema.AgenticMessage](adk.TypedRunnerConfig[*schema.AgenticMessage]{Agent: agent})
	body, err := jsonMarshal(input)
	if err != nil {
		return "", 0, err
	}
	it := runner.Run(ctx, []*schema.AgenticMessage{schema.UserAgenticMessage(string(body))})
	final := ""
	requests := 1
	for {
		ev, ok := it.Next()
		if !ok {
			break
		}
		if ev.Err != nil {
			return "", requests, ev.Err
		}
		if ev.Output == nil || ev.Output.MessageOutput == nil || ev.Output.MessageOutput.Message == nil {
			continue
		}
		msg := ev.Output.MessageOutput.Message
		for _, block := range msg.ContentBlocks {
			if block.AssistantGenText != nil && strings.TrimSpace(block.AssistantGenText.Text) != "" {
				final = block.AssistantGenText.Text
			}
			if block.FunctionToolCall != nil {
				requests++
			}
		}
	}
	if strings.TrimSpace(final) == "" {
		return "", requests, errNoTerminalMessage
	}
	return final, requests, nil
}
