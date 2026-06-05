package compaction

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/bogachenko/agentkit/core/llm"
)

const (
	DefaultMinEventsBetweenCompactions     = 8
	DefaultSummaryStateKey                 = "compacted_summary"
	DefaultLastCompactedEventIndexStateKey = "last_compacted_event_index"
	DefaultLastCompactedIndexStateKey      = "last_compacted_event_index"
	DefaultTokenThreshold                  = 60000
	DefaultRetainRecentEvents              = 8
	DefaultMaxToolResultChars              = 4000
	DefaultMaxTextChars                    = 12000
	DefaultMaxSummaryChars                 = 12000
)

type State interface {
	Get(key string) (any, bool, error)
	Set(key string, value any) error
}

type TokenCounter interface {
	Count(text string) int
}

type Config struct {
	Enabled                         bool
	TokenThreshold                  int
	RetainRecentEvents              int
	MaxToolResultChars              int
	MaxTextChars                    int
	MaxSummaryChars                 int
	SummaryStateKey                 string
	LastCompactedEventIndexStateKey string
	MinEventsBetweenCompactions     int
}

func DefaultConfig() Config {
	return Config{
		Enabled:                         true,
		TokenThreshold:                  DefaultTokenThreshold,
		RetainRecentEvents:              DefaultRetainRecentEvents,
		MaxToolResultChars:              DefaultMaxToolResultChars,
		MaxTextChars:                    DefaultMaxTextChars,
		MaxSummaryChars:                 DefaultMaxSummaryChars,
		SummaryStateKey:                 DefaultSummaryStateKey,
		LastCompactedEventIndexStateKey: DefaultLastCompactedEventIndexStateKey,
	}
}

func NormalizeConfig(cfg Config) Config {
	if cfg.MinEventsBetweenCompactions <= 0 {
		cfg.MinEventsBetweenCompactions = DefaultMinEventsBetweenCompactions
	}

	if cfg.TokenThreshold <= 0 {
		defaults := DefaultConfig()
		if !cfg.Enabled {
			defaults.Enabled = false
		}
		return defaults
	}

	if cfg.RetainRecentEvents <= 0 {
		cfg.RetainRecentEvents = DefaultRetainRecentEvents
	}
	if cfg.MaxToolResultChars <= 0 {
		cfg.MaxToolResultChars = DefaultMaxToolResultChars
	}
	if cfg.MaxTextChars <= 0 {
		cfg.MaxTextChars = DefaultMaxTextChars
	}
	if cfg.MaxSummaryChars <= 0 {
		cfg.MaxSummaryChars = DefaultMaxSummaryChars
	}
	if strings.TrimSpace(cfg.SummaryStateKey) == "" {
		cfg.SummaryStateKey = DefaultSummaryStateKey
	}
	if strings.TrimSpace(cfg.LastCompactedEventIndexStateKey) == "" {
		cfg.LastCompactedEventIndexStateKey = DefaultLastCompactedIndexStateKey
	}

	return cfg
}

type Input struct {
	PreviousSummary string
	Messages        []llm.Message
}

type Summarizer interface {
	Summarize(ctx context.Context, input Input) (string, error)
}

type ApproxTokenCounter struct{}

func (ApproxTokenCounter) Count(text string) int {
	if text == "" {
		return 0
	}
	return int(math.Ceil(float64(len([]byte(text))) / 4.0))
}

type Result struct {
	Changed                  bool
	Sanitized                bool
	Compacted                bool
	Summary                  string
	SummaryGenerated         bool
	SummaryReused            bool
	Messages                 []llm.Message
	LastCompactedIndex       int
	EstimatedTokensBefore    int
	EstimatedTokensSanitized int
	EstimatedTokensAfter     int
	MessagesBefore           int
	MessagesAfter            int
	HardFallbackUsed         bool
}

type Compactor struct {
	cfg          Config
	tokenCounter TokenCounter
	summarizer   Summarizer
}

func NewCompactor(cfg Config, counter TokenCounter, summarizer Summarizer) (*Compactor, error) {
	cfg = NormalizeConfig(cfg)
	if summarizer == nil {
		return nil, errors.New("summarizer is required")
	}
	if counter == nil {
		counter = ApproxTokenCounter{}
	}
	return &Compactor{cfg: cfg, tokenCounter: counter, summarizer: summarizer}, nil
}

func (c *Compactor) Config() Config { return c.cfg }

func (c *Compactor) Compact(ctx context.Context, req llm.Request, state State) (Result, error) {
	if state == nil {
		return Result{}, errors.New("state is required")
	}

	before := c.estimateTokenCount(req, "")
	base := Result{
		Messages:                 cloneMessages(req.Messages),
		LastCompactedIndex:       -1,
		EstimatedTokensBefore:    before,
		EstimatedTokensSanitized: before,
		EstimatedTokensAfter:     before,
		MessagesBefore:           len(req.Messages),
		MessagesAfter:            len(req.Messages),
	}

	if !c.cfg.Enabled || len(req.Messages) == 0 {
		return base, nil
	}

	previousSummary, err := c.getOptionalString(state, c.cfg.SummaryStateKey)
	if err != nil {
		return Result{}, err
	}

	sanitizedMessages := sanitizeMessages(req.Messages, c.cfg.MaxTextChars, c.cfg.MaxToolResultChars)
	sanitizedChanged := !messagesEqual(req.Messages, sanitizedMessages)
	sanitizedReq := req
	sanitizedReq.Messages = sanitizedMessages
	sanitizedTokens := c.estimateTokenCount(sanitizedReq, previousSummary)

	if sanitizedTokens < c.cfg.TokenThreshold {
		base.Changed = sanitizedChanged
		base.Sanitized = sanitizedChanged
		base.Summary = previousSummary
		base.Messages = sanitizedMessages
		base.EstimatedTokensSanitized = sanitizedTokens
		base.EstimatedTokensAfter = sanitizedTokens
		return base, nil
	}

	boundary := c.retainBoundary(sanitizedMessages)
	if boundary <= 0 {
		fallback := c.hardFallback(req, sanitizedMessages, previousSummary)
		fallback.EstimatedTokensBefore = before
		fallback.EstimatedTokensSanitized = sanitizedTokens
		fallback.Sanitized = sanitizedChanged
		return fallback, nil
	}

	lastCompactedIndex, err := c.getOptionalInt(state, c.cfg.LastCompactedEventIndexStateKey)
	if err != nil {
		return Result{}, err
	}
	candidateLast := boundary - 1

	summaryToUse := previousSummary
	summaryGenerated := false
	summaryReused := false

	if candidateLast > lastCompactedIndex || strings.TrimSpace(previousSummary) == "" {
		summaryToUse, err = c.summarizer.Summarize(ctx, Input{
			PreviousSummary: previousSummary,
			Messages:        sanitizedMessages[:boundary],
		})
		if err != nil || strings.TrimSpace(summaryToUse) == "" {
			fallback := c.hardFallback(req, sanitizedMessages, previousSummary)
			fallback.EstimatedTokensBefore = before
			fallback.EstimatedTokensSanitized = sanitizedTokens
			fallback.Sanitized = sanitizedChanged
			fallback.HardFallbackUsed = true
			return fallback, nil
		}
		summaryToUse = truncateRunes(strings.TrimSpace(summaryToUse), c.cfg.MaxSummaryChars)
		if err := state.Set(c.cfg.SummaryStateKey, summaryToUse); err != nil {
			return Result{}, fmt.Errorf("set summary state: %w", err)
		}
		if err := state.Set(c.cfg.LastCompactedEventIndexStateKey, candidateLast); err != nil {
			return Result{}, fmt.Errorf("set compacted index state: %w", err)
		}
		summaryGenerated = true
	} else {
		summaryToUse = truncateRunes(previousSummary, c.cfg.MaxSummaryChars)
		summaryReused = true
	}

	recent := cloneMessages(sanitizedMessages[boundary:])
	rewritten := compactedMessages(summaryToUse, recent)
	afterReq := req
	afterReq.Messages = rewritten
	after := c.estimateTokenCount(afterReq, "")

	for after > c.cfg.TokenThreshold && len(recent) > 1 {
		recent = recent[len(recent)/2:]
		rewritten = compactedMessages(summaryToUse, recent)
		afterReq.Messages = rewritten
		after = c.estimateTokenCount(afterReq, "")
	}

	if after > c.cfg.TokenThreshold {
		fallback := c.hardFallback(req, sanitizedMessages, summaryToUse)
		fallback.EstimatedTokensBefore = before
		fallback.EstimatedTokensSanitized = sanitizedTokens
		fallback.Sanitized = sanitizedChanged
		fallback.SummaryGenerated = summaryGenerated
		fallback.SummaryReused = summaryReused
		return fallback, nil
	}

	return Result{
		Changed:                  true,
		Sanitized:                sanitizedChanged,
		Compacted:                true,
		Summary:                  summaryToUse,
		SummaryGenerated:         summaryGenerated,
		SummaryReused:            summaryReused,
		Messages:                 rewritten,
		LastCompactedIndex:       candidateLast,
		EstimatedTokensBefore:    before,
		EstimatedTokensSanitized: sanitizedTokens,
		EstimatedTokensAfter:     after,
		MessagesBefore:           len(req.Messages),
		MessagesAfter:            len(rewritten),
	}, nil
}

func (c *Compactor) hardFallback(req llm.Request, sanitizedMessages []llm.Message, summary string) Result {
	recent := sanitizedMessages
	if len(recent) > 1 {
		keep := c.cfg.RetainRecentEvents
		if keep < 1 {
			keep = 1
		}
		if keep > len(recent) {
			keep = len(recent)
		}
		recent = recent[len(recent)-keep:]
	}

	rewritten := compactedMessages(truncateRunes(summary, c.cfg.MaxSummaryChars), recent)
	candidate := req
	candidate.Messages = rewritten
	after := c.estimateTokenCount(candidate, "")

	for after > c.cfg.TokenThreshold && len(recent) > 1 {
		recent = recent[1:]
		rewritten = compactedMessages(truncateRunes(summary, c.cfg.MaxSummaryChars), recent)
		candidate.Messages = rewritten
		after = c.estimateTokenCount(candidate, "")
	}

	return Result{
		Changed:              true,
		Compacted:            true,
		Summary:              summary,
		Messages:             rewritten,
		LastCompactedIndex:   -1,
		EstimatedTokensAfter: after,
		MessagesBefore:       len(req.Messages),
		MessagesAfter:        len(rewritten),
		HardFallbackUsed:     true,
	}
}

func compactedMessages(summary string, recent []llm.Message) []llm.Message {
	rewritten := make([]llm.Message, 0, len(recent)+1)
	if strings.TrimSpace(summary) != "" {
		rewritten = append(rewritten, llm.NewMessage(
			llm.RoleAssistant,
			llm.TextPart("[INTERNAL PRIOR COMPACTED SESSION CONTEXT]\n"+strings.TrimSpace(summary)),
		))
	}
	rewritten = append(rewritten, cloneMessages(recent)...)
	return rewritten
}

func (c *Compactor) estimateTokenCount(req llm.Request, previousSummary string) int {
	var b strings.Builder
	b.WriteString(req.System)
	b.WriteString("\n")
	if previousSummary != "" {
		b.WriteString(previousSummary)
		b.WriteString("\n")
	}
	for _, runtime := range req.RuntimeContext {
		b.WriteString(runtime)
		b.WriteString("\n")
	}
	for _, msg := range req.Messages {
		b.WriteString(messageToText(msg))
		b.WriteString("\n")
	}
	return c.tokenCounter.Count(b.String())
}

func (c *Compactor) retainBoundary(messages []llm.Message) int {
	keep := c.cfg.RetainRecentEvents
	if keep < 1 {
		keep = 1
	}
	if keep > len(messages) {
		keep = len(messages)
	}
	boundary := len(messages) - keep

	lastUserIdx := -1
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == llm.RoleUser {
			lastUserIdx = i
			break
		}
	}
	if lastUserIdx >= 0 && lastUserIdx < boundary {
		boundary = lastUserIdx
	}
	return boundary
}

func sanitizeMessages(messages []llm.Message, maxTextChars int, maxToolResultChars int) []llm.Message {
	cloned := cloneMessages(messages)
	for i := range cloned {
		for j := range cloned[i].Parts {
			part := &cloned[i].Parts[j]
			switch part.Type {
			case llm.PartText:
				part.Text = truncateRunes(part.Text, maxTextChars)
			case llm.PartFunctionResponse:
				text := anyToText(part.Result)
				if len([]rune(text)) > maxToolResultChars {
					part.Result = map[string]any{
						"truncated": true,
						"max_chars": maxToolResultChars,
						"content":   truncateRunes(text, maxToolResultChars),
					}
				}
			}
		}
	}
	return cloned
}

func (c *Compactor) getOptionalString(state State, key string) (string, error) {
	value, found, err := state.Get(key)
	if err != nil {
		return "", err
	}
	if !found || value == nil {
		return "", nil
	}
	s, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("state %s is not a string", key)
	}
	return s, nil
}

func (c *Compactor) getOptionalInt(state State, key string) (int, error) {
	value, found, err := state.Get(key)
	if err != nil {
		return 0, err
	}
	if !found || value == nil {
		return -1, nil
	}
	switch v := value.(type) {
	case int:
		return v, nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("state %s is not a number", key)
	}
}

func anyToText(v any) string {
	if v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

func messageToText(msg llm.Message) string {
	var b strings.Builder
	b.WriteString(string(msg.Role))
	b.WriteString(":\n")
	for _, part := range msg.Parts {
		switch part.Type {
		case llm.PartText:
			if part.Text != "" {
				b.WriteString(part.Text)
				b.WriteString("\n")
			}
		case llm.PartFunctionCall:
			b.WriteString("function_call:")
			b.WriteString(part.Name)
			b.WriteString(" ")
			b.WriteString(anyToText(part.Args))
			b.WriteString("\n")
		case llm.PartFunctionResponse:
			b.WriteString("function_response:")
			b.WriteString(part.Name)
			b.WriteString(" ")
			b.WriteString(anyToText(part.Result))
			b.WriteString("\n")
		}
	}
	return b.String()
}

func cloneMessages(messages []llm.Message) []llm.Message {
	result := make([]llm.Message, 0, len(messages))
	for _, msg := range messages {
		copied := llm.Message{Role: msg.Role}
		if len(msg.Parts) > 0 {
			copied.Parts = make([]llm.Part, len(msg.Parts))
			copy(copied.Parts, msg.Parts)
		}
		result = append(result, copied)
	}
	return result
}

func messagesEqual(left []llm.Message, right []llm.Message) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return string(leftJSON) == string(rightJSON)
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + fmt.Sprintf("...[truncated %d chars]", len(runes)-limit)
}
