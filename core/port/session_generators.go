package port

import (
	"context"

	"github.com/bogachenko/agentkit/core/session"
)

// Compactor keeps probabilistic history rewriting behind an explicit session contract.
type Compactor interface {
	Compact(ctx context.Context, input session.CompactionInput) (session.CompactionResult, error)
}

// TitleGenerator keeps probabilistic session naming outside deterministic runtime.
type TitleGenerator interface {
	GenerateTitle(ctx context.Context, input session.TitleInput) (string, error)
}

// WorkSummaryGenerator keeps user-visible work summaries outside deterministic runtime.
type WorkSummaryGenerator interface {
	GenerateWorkSummary(ctx context.Context, input session.WorkSummaryInput) (string, error)
}
