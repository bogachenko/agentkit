package skill

import (
	"testing"
	"testing/fstest"
)

func validManifest(id ID) Manifest {
	return Manifest{
		ID:              id,
		Name:            "WB Ranking Audit",
		Description:     "Explains marketplace ranking audit workflow.",
		Version:         "1.0.0",
		InstructionFile: "SKILL.md",
		ReferenceFiles: []string{
			"references/tool_workflow.md",
			"references/limitations.md",
		},
		Tags: []string{
			"marketplace",
			"audit",
		},
	}
}

func validSkillFS() fstest.MapFS {
	return fstest.MapFS{
		"SKILL.md": {
			Data: []byte("Use this skill to audit marketplace ranking evidence."),
		},
		"references/tool_workflow.md": {
			Data: []byte("First collect facts, then analyze ranking factors."),
		},
		"references/limitations.md": {
			Data: []byte("Do not invent unavailable marketplace metrics."),
		},
	}
}

func TestManifestValidateAcceptsValidManifest(t *testing.T) {
	manifest := validManifest(ID("wb-ranking-audit"))

	if err := manifest.Validate(); err != nil {
		t.Fatalf("expected valid manifest, got error: %v", err)
	}
}

func TestManifestValidateRejectsEmptyID(t *testing.T) {
	manifest := validManifest(ID(""))
	manifest.ID = ""

	if err := manifest.Validate(); err == nil {
		t.Fatal("expected error for empty skill id")
	}
}

func TestManifestValidateRejectsAbsoluteInstructionPath(t *testing.T) {
	manifest := validManifest(ID("wb-ranking-audit"))
	manifest.InstructionFile = "/SKILL.md"

	if err := manifest.Validate(); err == nil {
		t.Fatal("expected error for absolute instruction path")
	}
}

func TestManifestValidateRejectsParentPathReference(t *testing.T) {
	manifest := validManifest(ID("wb-ranking-audit"))
	manifest.ReferenceFiles = []string{
		"../secret.md",
	}

	if err := manifest.Validate(); err == nil {
		t.Fatal("expected error for parent path reference")
	}
}

func TestManifestValidateRejectsDuplicateReferenceFiles(t *testing.T) {
	manifest := validManifest(ID("wb-ranking-audit"))
	manifest.ReferenceFiles = []string{
		"references/tool_workflow.md",
		"references/tool_workflow.md",
	}

	if err := manifest.Validate(); err == nil {
		t.Fatal("expected error for duplicate reference files")
	}
}

func TestManifestValidateRejectsDuplicateTags(t *testing.T) {
	manifest := validManifest(ID("wb-ranking-audit"))
	manifest.Tags = []string{
		"audit",
		"audit",
	}

	if err := manifest.Validate(); err == nil {
		t.Fatal("expected error for duplicate tags")
	}
}

func TestLoaderLoadReadsInstructionAndReferences(t *testing.T) {
	loader := Loader{
		FS: validSkillFS(),
	}

	value, err := loader.Load(validManifest(ID("wb-ranking-audit")))
	if err != nil {
		t.Fatalf("expected load to succeed, got error: %v", err)
	}

	if value.Instructions == "" {
		t.Fatal("expected instructions to be loaded")
	}

	if len(value.References) != 2 {
		t.Fatalf("expected 2 references, got %d", len(value.References))
	}

	if value.References[0].Path != "references/tool_workflow.md" {
		t.Fatalf("expected first reference path to preserve manifest order, got %q", value.References[0].Path)
	}
}

func TestLoaderLoadRejectsNilFS(t *testing.T) {
	loader := Loader{}

	if _, err := loader.Load(validManifest(ID("wb-ranking-audit"))); err == nil {
		t.Fatal("expected error for nil fs")
	}
}

func TestLoaderLoadRejectsMissingInstructionFile(t *testing.T) {
	loader := Loader{
		FS: fstest.MapFS{},
	}

	if _, err := loader.Load(validManifest(ID("wb-ranking-audit"))); err == nil {
		t.Fatal("expected error for missing instruction file")
	}
}

func TestLoaderLoadRejectsEmptyInstructionFile(t *testing.T) {
	loader := Loader{
		FS: fstest.MapFS{
			"SKILL.md": {
				Data: []byte("   "),
			},
			"references/tool_workflow.md": {
				Data: []byte("workflow"),
			},
			"references/limitations.md": {
				Data: []byte("limitations"),
			},
		},
	}

	if _, err := loader.Load(validManifest(ID("wb-ranking-audit"))); err == nil {
		t.Fatal("expected error for empty instruction file")
	}
}

func TestSkillValidateRejectsReferenceCountMismatch(t *testing.T) {
	value := Skill{
		Manifest:     validManifest(ID("wb-ranking-audit")),
		Instructions: "instructions",
		References: []Reference{
			{
				Path:    "references/tool_workflow.md",
				Content: "workflow",
			},
		},
	}

	if err := value.Validate(); err == nil {
		t.Fatal("expected error for reference count mismatch")
	}
}

func TestRegistryRegisterStoresSkill(t *testing.T) {
	loader := Loader{
		FS: validSkillFS(),
	}

	value, err := loader.Load(validManifest(ID("wb-ranking-audit")))
	if err != nil {
		t.Fatalf("expected load to succeed, got error: %v", err)
	}

	registry := NewRegistry()

	if err := registry.Register(value); err != nil {
		t.Fatalf("expected register to succeed, got error: %v", err)
	}

	got, exists := registry.Get(ID("wb-ranking-audit"))
	if !exists {
		t.Fatal("expected registered skill to exist")
	}

	if got.Manifest.ID != ID("wb-ranking-audit") {
		t.Fatalf("expected skill id wb-ranking-audit, got %q", got.Manifest.ID)
	}
}

func TestRegistryRegisterRejectsDuplicateSkill(t *testing.T) {
	loader := Loader{
		FS: validSkillFS(),
	}

	value, err := loader.Load(validManifest(ID("wb-ranking-audit")))
	if err != nil {
		t.Fatalf("expected load to succeed, got error: %v", err)
	}

	registry := NewRegistry()

	if err := registry.Register(value); err != nil {
		t.Fatalf("expected first register to succeed, got error: %v", err)
	}

	if err := registry.Register(value); err == nil {
		t.Fatal("expected duplicate skill error")
	}
}

func TestRegistryListReturnsStableOrder(t *testing.T) {
	loader := Loader{
		FS: validSkillFS(),
	}

	first, err := loader.Load(validManifest(ID("b-skill")))
	if err != nil {
		t.Fatalf("expected first load to succeed, got error: %v", err)
	}

	second, err := loader.Load(validManifest(ID("a-skill")))
	if err != nil {
		t.Fatalf("expected second load to succeed, got error: %v", err)
	}

	registry := NewRegistry()

	if err := registry.Register(first); err != nil {
		t.Fatalf("register first: %v", err)
	}

	if err := registry.Register(second); err != nil {
		t.Fatalf("register second: %v", err)
	}

	items := registry.List()

	if len(items) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(items))
	}

	if items[0].Manifest.ID != ID("a-skill") || items[1].Manifest.ID != ID("b-skill") {
		t.Fatalf("expected stable order [a-skill b-skill], got [%s %s]", items[0].Manifest.ID, items[1].Manifest.ID)
	}
}
