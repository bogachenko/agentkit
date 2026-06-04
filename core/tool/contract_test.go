package tool

import "testing"

func TestNameValidateAcceptsNonEmptyName(t *testing.T) {
	name := Name("search_products")

	if err := name.Validate(); err != nil {
		t.Fatalf("expected valid name, got error: %v", err)
	}
}

func TestNameValidateRejectsEmptyName(t *testing.T) {
	name := Name("   ")

	if err := name.Validate(); err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestNewCallNormalizesNilArgs(t *testing.T) {
	call := NewCall(Name("search_products"), nil)

	if call.Args == nil {
		t.Fatal("expected nil args to be normalized to empty map")
	}
}

func TestCallValidateRejectsNilArgs(t *testing.T) {
	call := Call{
		Name: Name("search_products"),
		Args: nil,
	}

	if err := call.Validate(); err == nil {
		t.Fatal("expected error for nil args")
	}
}

func TestCallValidateAcceptsValidCall(t *testing.T) {
	call := NewCall(Name("search_products"), map[string]any{
		"query": "box",
	})

	if err := call.Validate(); err != nil {
		t.Fatalf("expected valid call, got error: %v", err)
	}
}

func TestResultValidateRejectsEmptyName(t *testing.T) {
	result := NewResult(Name(""), map[string]any{
		"ok": true,
	})

	if err := result.Validate(); err == nil {
		t.Fatal("expected error for empty result name")
	}
}

func TestContractValidateAcceptsValidContract(t *testing.T) {
	contract := Contract{
		Name:        Name("search_products"),
		Description: "Searches products in a read-only catalog.",
		InputSchema: map[string]any{
			"type": "object",
		},
		OutputSchema: map[string]any{
			"type": "object",
		},
		ReadOnly:         true,
		RequiresApproval: false,
	}

	if err := contract.Validate(); err != nil {
		t.Fatalf("expected valid contract, got error: %v", err)
	}
}

func TestContractValidateRejectsMissingDescription(t *testing.T) {
	contract := Contract{
		Name: Name("search_products"),
		InputSchema: map[string]any{
			"type": "object",
		},
		OutputSchema: map[string]any{
			"type": "object",
		},
	}

	if err := contract.Validate(); err == nil {
		t.Fatal("expected error for missing description")
	}
}

func TestContractValidateRejectsMissingInputSchema(t *testing.T) {
	contract := Contract{
		Name:        Name("search_products"),
		Description: "Searches products in a read-only catalog.",
		OutputSchema: map[string]any{
			"type": "object",
		},
	}

	if err := contract.Validate(); err == nil {
		t.Fatal("expected error for missing input schema")
	}
}

func TestContractValidateRejectsMissingOutputSchema(t *testing.T) {
	contract := Contract{
		Name:        Name("search_products"),
		Description: "Searches products in a read-only catalog.",
		InputSchema: map[string]any{
			"type": "object",
		},
	}

	if err := contract.Validate(); err == nil {
		t.Fatal("expected error for missing output schema")
	}
}
