package capability

import (
	"testing"

	"github.com/bogachenko/agentkit/core/tool"
)

type testCapability struct {
	metadata Metadata
}

func (c testCapability) Metadata() Metadata {
	return c.metadata
}

func validToolContract(name tool.Name) tool.Contract {
	return tool.Contract{
		Name:        name,
		Description: "Read-only test tool.",
		InputSchema: map[string]any{
			"type": "object",
		},
		OutputSchema: map[string]any{
			"type": "object",
		},
		ReadOnly:         true,
		RequiresApproval: false,
	}
}

func validMetadata(id ID) Metadata {
	return Metadata{
		ID:          id,
		Name:        "Test capability",
		Description: "Provides test tools.",
		Tools: []tool.Contract{
			validToolContract(tool.Name("test_tool")),
		},
		RequiredSources: []SourceRequirement{
			{Name: "test_source", Required: true},
		},
		Permissions: []Permission{
			"test.read",
		},
	}
}

func TestMetadataValidateAcceptsValidMetadata(t *testing.T) {
	metadata := validMetadata(ID("test"))

	if err := metadata.Validate(); err != nil {
		t.Fatalf("expected valid metadata, got error: %v", err)
	}
}

func TestMetadataValidateRejectsEmptyID(t *testing.T) {
	metadata := validMetadata(ID(""))
	metadata.ID = ""

	if err := metadata.Validate(); err == nil {
		t.Fatal("expected error for empty capability id")
	}
}

func TestMetadataValidateRejectsEmptyName(t *testing.T) {
	metadata := validMetadata(ID("test"))
	metadata.Name = "   "

	if err := metadata.Validate(); err == nil {
		t.Fatal("expected error for empty capability name")
	}
}

func TestMetadataValidateRejectsNoTools(t *testing.T) {
	metadata := validMetadata(ID("test"))
	metadata.Tools = nil

	if err := metadata.Validate(); err == nil {
		t.Fatal("expected error for capability without tools")
	}
}

func TestMetadataValidateRejectsInvalidToolContract(t *testing.T) {
	metadata := validMetadata(ID("test"))
	metadata.Tools = []tool.Contract{
		{
			Name: tool.Name("broken_tool"),
		},
	}

	if err := metadata.Validate(); err == nil {
		t.Fatal("expected error for invalid tool contract")
	}
}

func TestMetadataValidateRejectsDuplicateTools(t *testing.T) {
	metadata := validMetadata(ID("test"))
	metadata.Tools = []tool.Contract{
		validToolContract(tool.Name("duplicate_tool")),
		validToolContract(tool.Name("duplicate_tool")),
	}

	if err := metadata.Validate(); err == nil {
		t.Fatal("expected error for duplicate tools")
	}
}

func TestMetadataValidateRejectsDuplicateSources(t *testing.T) {
	metadata := validMetadata(ID("test"))
	metadata.RequiredSources = []SourceRequirement{
		{Name: "source", Required: true},
		{Name: "source", Required: true},
	}

	if err := metadata.Validate(); err == nil {
		t.Fatal("expected error for duplicate sources")
	}
}

func TestMetadataValidateRejectsDuplicatePermissions(t *testing.T) {
	metadata := validMetadata(ID("test"))
	metadata.Permissions = []Permission{
		"test.read",
		"test.read",
	}

	if err := metadata.Validate(); err == nil {
		t.Fatal("expected error for duplicate permissions")
	}
}

func TestValidateCapabilityRejectsNilCapability(t *testing.T) {
	if err := ValidateCapability(nil); err == nil {
		t.Fatal("expected error for nil capability")
	}
}

func TestRegistryRegisterStoresMetadata(t *testing.T) {
	registry := NewRegistry()
	metadata := validMetadata(ID("test"))

	if err := registry.Register(metadata); err != nil {
		t.Fatalf("expected registration to succeed, got error: %v", err)
	}

	got, exists := registry.Get(ID("test"))
	if !exists {
		t.Fatal("expected registered capability to exist")
	}

	if got.ID != metadata.ID {
		t.Fatalf("expected id %q, got %q", metadata.ID, got.ID)
	}
}

func TestRegistryRegisterRejectsDuplicateCapability(t *testing.T) {
	registry := NewRegistry()
	metadata := validMetadata(ID("test"))

	if err := registry.Register(metadata); err != nil {
		t.Fatalf("expected first registration to succeed, got error: %v", err)
	}

	if err := registry.Register(metadata); err == nil {
		t.Fatal("expected duplicate registration error")
	}
}

func TestRegistryRegisterCapabilityUsesMetadata(t *testing.T) {
	registry := NewRegistry()
	capability := testCapability{
		metadata: validMetadata(ID("test")),
	}

	if err := registry.RegisterCapability(capability); err != nil {
		t.Fatalf("expected registration to succeed, got error: %v", err)
	}

	if _, exists := registry.Get(ID("test")); !exists {
		t.Fatal("expected registered capability to exist")
	}
}

func TestRegistryListReturnsStableOrder(t *testing.T) {
	registry := NewRegistry()

	if err := registry.Register(validMetadata(ID("b"))); err != nil {
		t.Fatalf("register b: %v", err)
	}

	if err := registry.Register(validMetadata(ID("a"))); err != nil {
		t.Fatalf("register a: %v", err)
	}

	items := registry.List()

	if len(items) != 2 {
		t.Fatalf("expected 2 capabilities, got %d", len(items))
	}

	if items[0].ID != ID("a") || items[1].ID != ID("b") {
		t.Fatalf("expected stable order [a b], got [%s %s]", items[0].ID, items[1].ID)
	}
}

func TestRegistryToolContractsReturnsStableOrder(t *testing.T) {
	registry := NewRegistry()

	first := validMetadata(ID("first"))
	first.Tools = []tool.Contract{
		validToolContract(tool.Name("z_tool")),
	}

	second := validMetadata(ID("second"))
	second.Tools = []tool.Contract{
		validToolContract(tool.Name("a_tool")),
	}

	if err := registry.Register(first); err != nil {
		t.Fatalf("register first: %v", err)
	}

	if err := registry.Register(second); err != nil {
		t.Fatalf("register second: %v", err)
	}

	contracts := registry.ToolContracts()

	if len(contracts) != 2 {
		t.Fatalf("expected 2 tool contracts, got %d", len(contracts))
	}

	if contracts[0].Name != tool.Name("a_tool") || contracts[1].Name != tool.Name("z_tool") {
		t.Fatalf("expected stable tool order [a_tool z_tool], got [%s %s]", contracts[0].Name, contracts[1].Name)
	}
}
