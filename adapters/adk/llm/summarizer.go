package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bogachenko/agentkit/core/compaction"
	corellm "github.com/bogachenko/agentkit/core/llm"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

const CompactionSystemInstruction = `You are a context compaction agent for a browser-control assistant.

Your task is to compress older session history into a compact working summary that allows the main agent to continue the browser task without losing important context.

Summarize only the history provided to you. If a previous compacted summary is provided, update it by preserving still-true information, removing stale information, and merging in new facts.

Do not mention that you are compacting, summarizing, or compressing context. Do not answer the user's task. Do not add recommendations that were not present in the provided history.

Preserve exact identifiers when available: URLs, page titles, dates, authors, visible text facts, element/action intent, tool findings, user constraints, confirmed decisions, pending blockers, and actions already taken.

Output Markdown with exactly these sections:

## Goal
The user's current goal.

## Constraints & Preferences
Important user constraints, safety requirements, scope, and preferences.

## Progress
What has already been checked, opened, clicked, searched, extracted, or decided.

## Open Items
What still needs to be checked, verified, decided, or completed.

## Key Facts
Important page facts, URLs, titles, dates, authors, visible evidence, and browser state facts.

## Tool Findings
Important findings returned by browser tools.

## Next Step
The most likely next step for the main agent.`

type ADKLLMSummarizer struct {
	Model model.LLM
}

func NewADKLLMSummarizer(model model.LLM) *ADKLLMSummarizer {
	return &ADKLLMSummarizer{Model: model}
}

func (s *ADKLLMSummarizer) Summarize(ctx context.Context, input compaction.Input) (string, error) {
	if s == nil || s.Model == nil {
		return "", fmt.Errorf("model is required")
	}

	req := &model.LLMRequest{
		Contents: []*genai.Content{
			genai.NewContentFromText(buildSummaryPrompt(input), genai.RoleUser),
		},
		Config: &genai.GenerateContentConfig{
			SystemInstruction: genai.NewContentFromText(CompactionSystemInstruction, genai.RoleUser),
			Temperature:       genai.Ptr(float32(0.2)),
		},
	}

	for resp, err := range s.Model.GenerateContent(ctx, req, false) {
		if err != nil {
			return "", err
		}
		if resp == nil || resp.Content == nil {
			continue
		}
		text := ExtractText(resp.Content)
		if strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text), nil
		}
	}

	return "", fmt.Errorf("compaction summarizer returned no summary content")
}

func buildSummaryPrompt(input compaction.Input) string {
	var b strings.Builder
	b.WriteString("service: agentkit-compaction\n")
	if strings.TrimSpace(input.PreviousSummary) != "" {
		b.WriteString("Previous compacted summary:\n")
		b.WriteString(input.PreviousSummary)
		b.WriteString("\n")
	}
	b.WriteString("History to compact:\n")
	for _, msg := range input.Messages {
		b.WriteString(neutralMessageText(msg))
		b.WriteString("\n")
	}
	return b.String()
}

func neutralMessageText(msg corellm.Message) string {
	var b strings.Builder
	b.WriteString(string(msg.Role))
	b.WriteString(":\n")
	for _, part := range msg.Parts {
		switch part.Type {
		case corellm.PartText:
			if part.Text != "" {
				b.WriteString(part.Text)
				b.WriteString("\n")
			}
		case corellm.PartFunctionCall:
			b.WriteString("function_call:")
			b.WriteString(part.Name)
			b.WriteString(" ")
			b.WriteString(anyToText(part.Args))
			b.WriteString("\n")
		case corellm.PartFunctionResponse:
			b.WriteString("function_response:")
			b.WriteString(part.Name)
			b.WriteString(" ")
			b.WriteString(anyToText(part.Result))
			b.WriteString("\n")
		}
	}
	return b.String()
}

func anyToText(v any) string {
	if v == nil {
		return ""
	}
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(data)
}
