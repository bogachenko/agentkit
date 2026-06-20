package runtime

import "strings"

func executionLedgerInstruction(input ClassifierInput) string {
	summary := effectiveLedgerSummary(input)
	if summary.IsZero() {
		return ""
	}
	return semanticInstruction("Continue the task using the compact execution ledger below.\n" + semanticRunLedgerSummaryText(summary))
}

func answerFromContextInstruction(input ClassifierInput) string {
	summary := effectiveLedgerSummary(input)
	if summary.IsZero() && input.ActiveTask.IsZero() {
		return ""
	}
	return semanticInstruction(strings.Join([]string{
		"Answer the latest user request from the compact execution memory below.",
		"Use this memory as already retrieved task context.",
		"Do not call additional tools unless the user explicitly asks to refresh, reload, retry, fetch new data, inspect a new source, export, or create a file.",
		"If the memory is insufficient, say exactly what is missing.",
		semanticRunLedgerSummaryText(summary),
		activeTaskText(input.ActiveTask),
	}, "\n"))
}

func effectiveLedgerSummary(input ClassifierInput) RunLedgerSummary {
	if !input.LedgerSummary.IsZero() {
		return input.LedgerSummary
	}
	if input.RunLedger != nil {
		return input.RunLedger.Summary()
	}
	return RunLedgerSummary{}
}

func semanticInstruction(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	return "<runtime_harness_instruction>\n" + body + "\n</runtime_harness_instruction>"
}

func semanticRunLedgerSummaryText(summary RunLedgerSummary) string {
	if summary.IsZero() {
		return ""
	}
	var b strings.Builder
	writeLine(&b, "task_id", summary.TaskID)
	writeLine(&b, "user_goal", summary.UserGoal)
	writeLine(&b, "current_phase", summary.CurrentPhase)
	writeList(&b, "completed_steps", summary.CompletedSteps)
	writeList(&b, "available_data", summary.AvailableData)
	writeArtifacts(&b, summary.Artifacts)
	writeList(&b, "completed_objectives", summary.CompletedObjectives)
	writeList(&b, "failed_objectives", summary.FailedObjectives)
	writeList(&b, "open_questions", summary.OpenQuestions)
	writeList(&b, "available_next_actions", summary.AvailableNextActions)
	writeList(&b, "blockers_or_errors", summary.BlockersOrErrors)
	writeList(&b, "warnings", summary.Warnings)
	return b.String()
}

func activeTaskText(task ActiveTaskState) string {
	if task.IsZero() {
		return ""
	}
	var b strings.Builder
	writeLine(&b, "marketplace", task.Marketplace)
	writeLine(&b, "result_kind", task.ResultKind)
	writeLine(&b, "operation_id", task.OperationID)
	writeLine(&b, "original_request", task.OriginalRequest)
	writeLine(&b, "last_result_summary", task.LastResultSummary)
	writeList(&b, "available_actions", task.AvailableActions)
	return b.String()
}

func writeLine(b *strings.Builder, key string, value string) {
	value = strings.TrimSpace(value)
	if value != "" {
		b.WriteString("- " + key + ": " + value + "\n")
	}
}

func writeList(b *strings.Builder, key string, values []string) {
	values = compactStrings(values)
	if len(values) == 0 {
		return
	}
	b.WriteString("- " + key + ":\n")
	for _, value := range values {
		b.WriteString("  - " + value + "\n")
	}
}

func writeArtifacts(b *strings.Builder, artifacts []RunLedgerArtifact) {
	if len(artifacts) == 0 {
		return
	}
	b.WriteString("- artifacts:\n")
	for _, artifact := range artifacts {
		b.WriteString("  - " + strings.Join(compactStrings([]string{artifact.ID, artifact.Kind, artifact.Name, artifact.Ref, artifact.Summary}), " | ") + "\n")
	}
}
