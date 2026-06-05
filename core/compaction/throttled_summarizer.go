package compaction

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
)

// ThrottledSummarizer prevents repeated expensive summary LLM calls when the
// compactor is invoked many times over nearly the same long session history.
// Recent events are still retained by the compactor; this only avoids
// regenerating almost the same old-history summary on every model call.
type ThrottledSummarizer struct {
	inner                       Summarizer
	minEventsBetweenCompactions int

	mu                  sync.Mutex
	lastSummary         string
	callsSinceGenerated int
	cache               map[string]string
}

// NewThrottledSummarizer wraps a summarizer with cache + min-call hysteresis.
func NewThrottledSummarizer(inner Summarizer, minEventsBetweenCompactions int) Summarizer {
	if inner == nil {
		return nil
	}

	if minEventsBetweenCompactions <= 0 {
		minEventsBetweenCompactions = DefaultMinEventsBetweenCompactions
	}

	return &ThrottledSummarizer{
		inner:                       inner,
		minEventsBetweenCompactions: minEventsBetweenCompactions,
		cache:                       map[string]string{},
	}
}

func (s *ThrottledSummarizer) Summarize(ctx context.Context, input Input) (string, error) {
	if s == nil || s.inner == nil {
		return "", fmt.Errorf("compaction summarizer is nil")
	}

	key := summaryCacheKey(input)

	s.mu.Lock()
	if cached := s.cache[key]; cached != "" {
		s.mu.Unlock()
		return cached, nil
	}

	if s.lastSummary != "" && s.callsSinceGenerated < s.minEventsBetweenCompactions {
		s.callsSinceGenerated++
		summary := s.lastSummary
		s.cache[key] = summary
		s.mu.Unlock()
		return summary, nil
	}
	s.mu.Unlock()

	summary, err := s.inner.Summarize(ctx, input)
	if err != nil {
		return "", err
	}

	s.mu.Lock()
	s.lastSummary = summary
	s.callsSinceGenerated = 0
	s.cache[key] = summary
	s.mu.Unlock()

	return summary, nil
}

func summaryCacheKey(input any) string {
	data, err := json.Marshal(input)
	if err != nil {
		data = []byte(fmt.Sprintf("%#v", input))
	}

	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
