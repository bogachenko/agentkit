package runtime

// LedgerSummary provides deterministic counters for runtime decisions without semantic summarization.
type LedgerSummary struct {
	RunID          RunID
	TotalEntries   int
	Steps          int
	StepsCompleted int
	StepsFailed    int
	StepsBlocked   int
	RouteDecisions int
	ToolCalls      int
	ToolResults    int
	Notes          int
	LastEntryID    LedgerEntryID
	LastEntryKind  LedgerEntryKind
	LastFailure    *Failure
}

// Summary derives stable runtime counters from append-only entries without modifying ledger state.
func (l *Ledger) Summary() LedgerSummary {
	if l == nil {
		return LedgerSummary{}
	}

	summary := LedgerSummary{
		RunID: l.runID,
	}

	for _, entry := range l.entries {
		summary.TotalEntries++
		summary.LastEntryID = entry.ID
		summary.LastEntryKind = entry.Kind

		switch entry.Kind {
		case LedgerEntryKindStep:
			summary.Steps++

			if entry.Step == nil {
				continue
			}

			switch entry.Step.Status {
			case StepStatusCompleted:
				summary.StepsCompleted++
			case StepStatusFailed:
				summary.StepsFailed++
				summary.LastFailure = entry.Step.Failure
			case StepStatusBlocked:
				summary.StepsBlocked++
				summary.LastFailure = entry.Step.Failure
			}

		case LedgerEntryKindRouteDecision:
			summary.RouteDecisions++

			if entry.Decision != nil && entry.Decision.Failure != nil {
				summary.LastFailure = entry.Decision.Failure
			}

		case LedgerEntryKindToolCall:
			summary.ToolCalls++

		case LedgerEntryKindToolResult:
			summary.ToolResults++

		case LedgerEntryKindNote:
			summary.Notes++
		}
	}

	return summary
}
