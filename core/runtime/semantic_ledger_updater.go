package runtime

import (
	"context"
	"fmt"
	"strings"
)

func ApplySemanticStepToRunLedger(l *RunLedger, s Step) {
	if l == nil {
		return
	}
	name := string(s.ToolName)
	sid := semanticStepID(l, s)
	switch s.Kind {
	case StepKindToolCall:
		l.Steps = append(l.Steps, RunLedgerStep{ID: sid, Kind: "tool_call", ToolName: name, Status: "started", Summary: "called tool " + name})
		l.CurrentPhase = "executing"
		l.AvailableNextActions = mergeStrings(l.AvailableNextActions, []string{"await_tool_result"})
	case StepKindToolResult:
		if s.ToolResult.OK {
			summary := "tool " + name + " returned result"
			l.Steps = append(l.Steps, RunLedgerStep{ID: sid, Kind: "tool_result", ToolName: name, Status: "completed", Summary: summary})
			l.CurrentPhase = "has_data"
			l.CompletedObjectives = mergeStrings(l.CompletedObjectives, []string{summary})
			if s.ToolResult.HasEvidence {
				l.DataRefs = mergeStrings(l.DataRefs, []string{name + ":" + sid})
			}
			l.AvailableNextActions = mergeStrings(l.AvailableNextActions, []string{"answer_from_evidence", "continue_with_tools"})
			l.Artifacts = mergeArtifacts(l.Artifacts, artifactsFromToolResult(s))
			return
		}
		summary := "tool " + name + " failed"
		l.Steps = append(l.Steps, RunLedgerStep{ID: sid, Kind: "tool_result", ToolName: name, Status: "failed", Summary: summary, Error: strings.TrimSpace(s.ToolResult.ErrorMessage)})
		l.FailedObjectives = mergeStrings(l.FailedObjectives, []string{summary})
		if s.ToolResult.ErrorKind == ToolErrorValidation {
			l.CurrentPhase = "needs_retry"
			l.AvailableNextActions = mergeStrings(l.AvailableNextActions, []string{"retry_with_corrected_arguments"})
			return
		}
		l.CurrentPhase = "blocked"
		l.AvailableNextActions = mergeStrings(l.AvailableNextActions, []string{"explain_blocker"})
	case StepKindAssistantText:
		if s.Final {
			l.CompletedObjectives = mergeStrings(l.CompletedObjectives, []string{"final_answer_produced"})
			l.CurrentPhase = "answered"
			l.AvailableNextActions = mergeStrings(l.AvailableNextActions, []string{"await_user_follow_up"})
		}
	case StepKindStreamDone:
		if !containsString(l.CompletedObjectives, "final_answer_produced") {
			l.CurrentPhase = "blocked"
			l.Warnings = mergeStrings(l.Warnings, []string{"stream ended without final answer"})
		}
	}
}

func nextRunLedgerStepID(l *RunLedger) string {
	if l == nil {
		return "step_001"
	}
	return fmt.Sprintf("step_%03d", len(l.Steps)+1)
}

func semanticStepID(l *RunLedger, s Step) string {
	if id := strings.TrimSpace(string(s.ID)); id != "" {
		return id
	}
	return nextRunLedgerStepID(l)
}

func artifactsFromToolResult(s Step) []RunLedgerArtifact {
	raw, ok := s.ToolResult.Raw.(map[string]any)
	if !ok {
		return nil
	}
	out := artifactList(raw["artifacts"])
	if a, ok := artifactFromAny(raw["artifact"]); ok {
		out = append(out, a)
	}
	if a, ok := artifactFromMap(raw); ok {
		out = append(out, a)
	}
	return mergeArtifacts(nil, out)
}

func artifactList(v any) []RunLedgerArtifact {
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]RunLedgerArtifact, 0, len(items))
	for _, item := range items {
		if a, ok := artifactFromAny(item); ok {
			out = append(out, a)
		}
	}
	return out
}

func artifactFromAny(v any) (RunLedgerArtifact, bool) {
	m, ok := v.(map[string]any)
	if !ok {
		return RunLedgerArtifact{}, false
	}
	return artifactFromMap(m)
}

func artifactFromMap(m map[string]any) (RunLedgerArtifact, bool) {
	a := RunLedgerArtifact{ID: field(m, "artifact_id", "id"), Kind: field(m, "artifact_kind", "kind"), Name: field(m, "artifact_name", "name"), Ref: field(m, "artifact_ref", "ref"), Summary: field(m, "artifact_summary", "summary")}
	return a, a.ID != "" || a.Kind != "" || a.Name != "" || a.Ref != "" || a.Summary != ""
}

func field(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if s := strings.TrimSpace(fmt.Sprint(m[key])); s != "" && s != "<nil>" {
			return s
		}
	}
	return ""
}

type semanticLedgerStepProvider struct {
	inner  StepProvider
	ledger *RunLedger
}

func (p *semanticLedgerStepProvider) NextSteps(ctx context.Context, state State) ([]Step, error) {
	steps, err := p.inner.NextSteps(ctx, state)
	for _, step := range steps {
		ApplySemanticStepToRunLedger(p.ledger, step)
	}
	return steps, err
}

func (p *semanticLedgerStepProvider) AddInternalInstruction(instruction string) {
	if r, ok := p.inner.(InternalInstructionReceiver); ok && r != nil {
		r.AddInternalInstruction(instruction)
	}
}
