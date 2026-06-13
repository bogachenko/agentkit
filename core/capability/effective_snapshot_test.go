package capability

import (
	"strings"
	"testing"

	"github.com/bogachenko/agentkit/core/session"
	"github.com/bogachenko/agentkit/core/skill"
	"github.com/bogachenko/agentkit/core/tool"
)

func TestEffectiveCapabilitiesSnapshotIsDeterministicAndDefensive(t *testing.T) {
	tools := []EffectiveTool{
		validEffectiveTool("capability-b", "tool-b", ToolModeAskBeforeUse),
		validEffectiveTool("capability-a", "tool-a", ToolModeEnabled),
	}
	skills := []EffectiveSkill{
		validEffectiveSkill("skill-b"),
		validEffectiveSkill("skill-a"),
	}
	sections := []PromptSection{
		{ID: "section-b", Order: 20, Markdown: "B"},
		{ID: "section-a", Order: 10, Markdown: "A"},
	}
	stale := []StaleReference{
		{Kind: StaleReferenceSkill, ID: "missing-skill", Reason: "not registered"},
		{Kind: StaleReferenceTool, ID: "missing-tool", Reason: "not registered"},
	}

	snapshot, err := NewEffectiveCapabilitiesSnapshot(
		session.ID("session-1"),
		SnapshotRevision("revision-1"),
		tools,
		skills,
		sections,
		stale,
	)
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}

	tools[0].Contract.Description = "mutated input"
	tools[0].Contract.InputSchema["properties"].(map[string]any)["mutated"] = true
	skills[0].Skill.Instructions = "mutated input"
	sections[0].Markdown = "mutated input"
	stale[0].Reason = "mutated input"

	gotTools := snapshot.Tools()
	if len(gotTools) != 2 || gotTools[0].Contract.Name != tool.Name("tool-a") || gotTools[1].Contract.Name != tool.Name("tool-b") {
		t.Fatalf("tools are not deterministic: %#v", gotTools)
	}
	if gotTools[1].Contract.Description == "mutated input" {
		t.Fatal("snapshot retained mutable tool input")
	}
	properties := gotTools[1].Contract.InputSchema["properties"].(map[string]any)
	if _, exists := properties["mutated"]; exists {
		t.Fatal("snapshot retained mutable nested schema input")
	}

	gotSkills := snapshot.Skills()
	if len(gotSkills) != 2 || gotSkills[0].Skill.Manifest.ID != skill.ID("skill-a") || gotSkills[1].Skill.Manifest.ID != skill.ID("skill-b") {
		t.Fatalf("skills are not deterministic: %#v", gotSkills)
	}
	if gotSkills[1].Skill.Instructions == "mutated input" {
		t.Fatal("snapshot retained mutable skill input")
	}

	gotSections := snapshot.PromptSections()
	if len(gotSections) != 2 || gotSections[0].ID != "section-a" || gotSections[1].ID != "section-b" {
		t.Fatalf("prompt sections are not deterministic: %#v", gotSections)
	}
	if gotSections[1].Markdown == "mutated input" {
		t.Fatal("snapshot retained mutable prompt section input")
	}

	gotStale := snapshot.StaleReferences()
	if len(gotStale) != 2 || gotStale[0].Kind != StaleReferenceSkill || gotStale[1].Kind != StaleReferenceTool {
		t.Fatalf("stale references are not deterministic: %#v", gotStale)
	}
	if gotStale[0].Reason == "mutated input" {
		t.Fatal("snapshot retained mutable stale reference input")
	}

	gotTools[0].Contract.Description = "mutated getter"
	gotTools[0].Contract.InputSchema["properties"].(map[string]any)["mutated"] = true
	gotSkills[0].Skill.Instructions = "mutated getter"
	gotSections[0].Markdown = "mutated getter"
	gotStale[0].Reason = "mutated getter"

	freshTool, exists := snapshot.Tool(tool.Name("tool-a"))
	if !exists {
		t.Fatal("expected tool-a in snapshot")
	}
	if freshTool.Contract.Description == "mutated getter" {
		t.Fatal("tool getter exposed snapshot storage")
	}
	freshProperties := freshTool.Contract.InputSchema["properties"].(map[string]any)
	if _, exists := freshProperties["mutated"]; exists {
		t.Fatal("tool getter exposed nested snapshot schema storage")
	}
	if snapshot.Skills()[0].Skill.Instructions == "mutated getter" {
		t.Fatal("skills getter exposed snapshot storage")
	}
	if snapshot.PromptSections()[0].Markdown == "mutated getter" {
		t.Fatal("prompt sections getter exposed snapshot storage")
	}
	if snapshot.StaleReferences()[0].Reason == "mutated getter" {
		t.Fatal("stale references getter exposed snapshot storage")
	}
}

func TestEffectiveCapabilitiesSnapshotRejectsDisabledAndDuplicateTools(t *testing.T) {
	disabled := validEffectiveTool("capability-a", "tool-a", ToolModeDisabled)
	_, err := NewEffectiveCapabilitiesSnapshot(
		session.ID("session-1"),
		SnapshotRevision("revision-1"),
		[]EffectiveTool{disabled},
		nil,
		nil,
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "cannot use disabled mode") {
		t.Fatalf("expected disabled effective tool rejection, got %v", err)
	}

	duplicateA := validEffectiveTool("capability-a", "tool-a", ToolModeEnabled)
	duplicateB := validEffectiveTool("capability-b", "tool-a", ToolModeAskBeforeUse)
	_, err = NewEffectiveCapabilitiesSnapshot(
		session.ID("session-1"),
		SnapshotRevision("revision-1"),
		[]EffectiveTool{duplicateA, duplicateB},
		nil,
		nil,
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "duplicate effective tool") {
		t.Fatalf("expected duplicate effective tool rejection, got %v", err)
	}
}

func TestConditionalPromptSectionRequiresValidatedPredicates(t *testing.T) {
	section := ConditionalPromptSection{
		ID:               "pdf-workflow",
		Order:            100,
		Markdown:         "Use the complete PDF workflow.",
		RequiresAllTools: []tool.Name{"pdf_open", "pdf_save"},
	}
	if err := section.Validate(); err != nil {
		t.Fatalf("valid conditional prompt section: %v", err)
	}

	section.RequiresAllTools = append(section.RequiresAllTools, "pdf_open")
	if err := section.Validate(); err == nil || !strings.Contains(err.Error(), "duplicate requires_all_tools tool") {
		t.Fatalf("expected duplicate predicate rejection, got %v", err)
	}

	withoutPredicates := ConditionalPromptSection{
		ID:       "invalid",
		Order:    100,
		Markdown: "No availability predicate.",
	}
	if err := withoutPredicates.Validate(); err == nil || !strings.Contains(err.Error(), "requires at least one tool or skill predicate") {
		t.Fatalf("expected missing predicate rejection, got %v", err)
	}
}

func validEffectiveTool(capabilityID ID, name tool.Name, mode ToolMode) EffectiveTool {
	return EffectiveTool{
		CapabilityID: capabilityID,
		Contract: tool.Contract{
			Name:        name,
			Description: "Valid tool contract",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"value": map[string]any{"type": "string"},
				},
			},
			OutputSchema: map[string]any{
				"type": "object",
			},
		},
		Mode: mode,
	}
}

func validEffectiveSkill(id skill.ID) EffectiveSkill {
	return EffectiveSkill{
		Skill: skill.Skill{
			Manifest: skill.Manifest{
				ID:              id,
				Name:            "Valid skill",
				Description:     "Valid skill description",
				Version:         "1.0.0",
				InstructionFile: "SKILL.md",
			},
			Instructions: "Validated skill instructions",
		},
	}
}
