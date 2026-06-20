package runtime

import "strings"

type RunLedger struct {
	TaskID               string
	UserGoal             string
	CurrentPhase         string
	Warnings             []string
	Steps                []RunLedgerStep
	DataRefs             []string
	Artifacts            []RunLedgerArtifact
	CompletedObjectives  []string
	FailedObjectives     []string
	OpenQuestions        []string
	AvailableNextActions []string
}

type RunLedgerStep struct {
	ID           string
	Kind         string
	ToolName     string
	Status       string
	Summary      string
	Error        string
	Marketplace  string
	WorkflowRole string
	WorkflowID   string
	OperationID  string
	ResultKind   string
}

type RunLedgerArtifact struct {
	ID      string
	Kind    string
	Name    string
	Ref     string
	Summary string
}

type RunLedgerSummary struct {
	Present              bool
	TaskID               string
	UserGoal             string
	CurrentPhase         string
	CompletedSteps       []string
	AvailableData        []string
	Artifacts            []RunLedgerArtifact
	CompletedObjectives  []string
	FailedObjectives     []string
	OpenQuestions        []string
	AvailableNextActions []string
	BlockersOrErrors     []string
	Warnings             []string
}

func (l *RunLedger) IsZero() bool {
	return l == nil || strings.TrimSpace(l.TaskID) == "" && strings.TrimSpace(l.UserGoal) == "" && strings.TrimSpace(l.CurrentPhase) == "" &&
		len(l.Warnings) == 0 && len(l.Steps) == 0 && len(l.DataRefs) == 0 && len(l.Artifacts) == 0 && len(l.CompletedObjectives) == 0 &&
		len(l.FailedObjectives) == 0 && len(l.OpenQuestions) == 0 && len(l.AvailableNextActions) == 0
}

func (l *RunLedger) Summary() RunLedgerSummary {
	if l == nil || l.IsZero() {
		return RunLedgerSummary{}
	}

	summary := RunLedgerSummary{
		Present:              true,
		TaskID:               l.TaskID,
		UserGoal:             l.UserGoal,
		CurrentPhase:         l.CurrentPhase,
		AvailableData:        limitStrings(l.DataRefs, 8),
		Artifacts:            limitArtifacts(l.Artifacts, 6),
		CompletedObjectives:  limitStrings(l.CompletedObjectives, 8),
		FailedObjectives:     limitStrings(l.FailedObjectives, 8),
		OpenQuestions:        limitStrings(l.OpenQuestions, 6),
		AvailableNextActions: limitStrings(l.AvailableNextActions, 10),
		Warnings:             limitStrings(l.Warnings, 6),
	}

	for _, step := range l.Steps {
		if len(summary.CompletedSteps) < 8 {
			if text := firstNonEmpty(step.Summary, step.Kind, step.ToolName); text != "" {
				summary.CompletedSteps = append(summary.CompletedSteps, text)
			}
		}
		if len(summary.BlockersOrErrors) < 6 {
			if err := strings.TrimSpace(step.Error); err != "" {
				summary.BlockersOrErrors = append(summary.BlockersOrErrors, err)
			}
		}
	}

	return summary
}

func (s RunLedgerSummary) IsZero() bool {
	return !s.Present && strings.TrimSpace(s.TaskID) == "" && strings.TrimSpace(s.UserGoal) == "" && strings.TrimSpace(s.CurrentPhase) == "" &&
		len(s.CompletedSteps) == 0 && len(s.AvailableData) == 0 && len(s.Artifacts) == 0 && len(s.CompletedObjectives) == 0 &&
		len(s.FailedObjectives) == 0 && len(s.OpenQuestions) == 0 && len(s.AvailableNextActions) == 0 && len(s.BlockersOrErrors) == 0 && len(s.Warnings) == 0
}

func MergeRunLedgers(existing, incoming *RunLedger) *RunLedger {
	if existing == nil {
		return cloneRunLedger(incoming)
	}
	if incoming == nil {
		return cloneRunLedger(existing)
	}

	merged := cloneRunLedger(existing)
	merged.TaskID = firstNonEmpty(merged.TaskID, incoming.TaskID)
	merged.UserGoal = firstNonEmpty(merged.UserGoal, incoming.UserGoal)
	merged.CurrentPhase = firstNonEmpty(incoming.CurrentPhase, merged.CurrentPhase)
	merged.Warnings = mergeStrings(merged.Warnings, incoming.Warnings)
	merged.Steps = append(merged.Steps, incoming.Steps...)
	merged.DataRefs = mergeStrings(merged.DataRefs, incoming.DataRefs)
	merged.Artifacts = mergeArtifacts(merged.Artifacts, incoming.Artifacts)
	merged.CompletedObjectives = mergeStrings(merged.CompletedObjectives, incoming.CompletedObjectives)
	merged.FailedObjectives = mergeStrings(merged.FailedObjectives, incoming.FailedObjectives)
	merged.OpenQuestions = mergeStrings(merged.OpenQuestions, incoming.OpenQuestions)
	merged.AvailableNextActions = mergeStrings(merged.AvailableNextActions, incoming.AvailableNextActions)
	return merged
}

func cloneRunLedger(source *RunLedger) *RunLedger {
	if source == nil {
		return nil
	}
	clone := *source
	clone.Warnings = append([]string(nil), source.Warnings...)
	clone.Steps = append([]RunLedgerStep(nil), source.Steps...)
	clone.DataRefs = append([]string(nil), source.DataRefs...)
	clone.Artifacts = append([]RunLedgerArtifact(nil), source.Artifacts...)
	clone.CompletedObjectives = append([]string(nil), source.CompletedObjectives...)
	clone.FailedObjectives = append([]string(nil), source.FailedObjectives...)
	clone.OpenQuestions = append([]string(nil), source.OpenQuestions...)
	clone.AvailableNextActions = append([]string(nil), source.AvailableNextActions...)
	return &clone
}

func limitStrings(values []string, limit int) []string {
	values = compactStrings(values)
	if len(values) > limit {
		return values[:limit]
	}
	return values
}

func limitArtifacts(values []RunLedgerArtifact, limit int) []RunLedgerArtifact {
	values = mergeArtifacts(nil, values)
	if len(values) > limit {
		return values[:limit]
	}
	return values
}

func compactStrings(values []string) []string {
	// ponytail: semantic lists are tiny; switch to map-backed dedupe if ledgers become large.
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || containsString(out, value) {
			continue
		}
		out = append(out, value)
	}
	return out
}

func mergeStrings(existing, incoming []string) []string {
	return compactStrings(append(append([]string(nil), existing...), incoming...))
}

func mergeArtifacts(existing, incoming []RunLedgerArtifact) []RunLedgerArtifact {
	out := append([]RunLedgerArtifact(nil), existing...)
	for _, artifact := range incoming {
		if artifact.ID == "" && artifact.Ref == "" && artifact.Name == "" {
			continue
		}
		if containsArtifact(out, artifact) {
			continue
		}
		out = append(out, artifact)
	}
	return out
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func containsArtifact(values []RunLedgerArtifact, needle RunLedgerArtifact) bool {
	for _, value := range values {
		if firstNonEmpty(value.ID, value.Ref, value.Name) == firstNonEmpty(needle.ID, needle.Ref, needle.Name) {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}
