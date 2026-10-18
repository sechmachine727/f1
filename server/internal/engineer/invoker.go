package engineer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"f1tele/server/internal/contract"
	"f1tele/server/internal/state"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
)

// Invoker runs one bounded Eino ADK invocation with the authored prompts and tools.
type Invoker struct {
	Backend ToolBackend
}

func buildModel(ctx context.Context, e contract.Endpoint, apiKey string) (model.ChatModel, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("endpoint %s has no configured key", e.ID)
	}
	max := 2048
	switch e.Protocol {
	case "chat_completions":
		return openai.NewChatModel(ctx, &openai.ChatModelConfig{APIKey: apiKey, BaseURL: e.BaseURL, Model: e.Model, MaxCompletionTokens: &max})
	case "responses":
		// The agentic Responses adapter uses schema.AgenticMessage; this build
		// wires the chat-completions Eino model path only. Real Responses
		// provider qualification is pending and must not be claimed.
		return nil, fmt.Errorf("responses protocol adapter is not wired in this build")
	}
	return nil, fmt.Errorf("unsupported protocol %q", e.Protocol)
}

// Run returns the terminal assistant JSON text. Streaming partial text is never
// accepted as output.
func (i *Invoker) Run(ctx context.Context, profile string, input contract.AgentInput, endpoint contract.Endpoint, apiKey string, snapshot state.Snapshot) (string, int, error) {
	if err := VerifyBundle(); err != nil {
		return "", 0, err
	}
	system, err := Assemble(profile)
	if err != nil {
		return "", 0, err
	}
	tools, err := BuildTools(profile, i.Backend, input.Observation)
	if err != nil {
		return "", 0, err
	}
	if endpoint.Protocol == "responses" {
		return i.runResponses(ctx, profile, system, input, endpoint, apiKey, tools)
	}
	cm, err := buildModel(ctx, endpoint, apiKey)
	if err != nil {
		return "", 0, err
	}
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name: profile, Description: "bounded race engineer", Instruction: system, Model: cm,
		ToolsConfig:   adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: tools}},
		MaxIterations: input.Limits.MaxSteps,
	})
	if err != nil {
		return "", 0, err
	}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent})
	user, _ := json.Marshal(input)
	it := runner.Query(ctx, string(user))
	var final string
	requests := 1
	for {
		ev, ok := it.Next()
		if !ok {
			break
		}
		if ev.Err != nil {
			return "", requests, ev.Err
		}
		if ev.Output != nil && ev.Output.MessageOutput != nil && ev.Output.MessageOutput.Message != nil {
			m := ev.Output.MessageOutput.Message
			if len(m.ToolCalls) > 0 {
				requests++
			}
			if strings.TrimSpace(m.Content) != "" {
				final = m.Content
			}
		}
	}
	if strings.TrimSpace(final) == "" {
		return "", requests, fmt.Errorf("provider returned no terminal message")
	}
	return final, requests, nil
}
