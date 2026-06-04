package capability

import (
	"fmt"
	"sort"

	"github.com/bogachenko/agentkit/core/tool"
)

// Registry gives runtime deterministic access to declared capabilities and their tool contracts.
type Registry struct {
	items map[ID]Metadata
}

// NewRegistry creates an isolated registry so tests and runtimes do not share mutable global state.
func NewRegistry() *Registry {
	return &Registry{
		items: map[ID]Metadata{},
	}
}

// Register validates capability metadata before making it available to runtime.
func (r *Registry) Register(metadata Metadata) error {
	if r == nil {
		return fmt.Errorf("capability registry is nil")
	}

	if err := metadata.Validate(); err != nil {
		return err
	}

	if _, exists := r.items[metadata.ID]; exists {
		return fmt.Errorf("capability %q is already registered", string(metadata.ID))
	}

	r.items[metadata.ID] = metadata
	return nil
}

// RegisterCapability keeps implementation-backed capabilities behind the same metadata validation path.
func (r *Registry) RegisterCapability(value Capability) error {
	if err := ValidateCapability(value); err != nil {
		return err
	}

	return r.Register(value.Metadata())
}

// Get returns declared metadata without exposing registry internals to callers.
func (r *Registry) Get(id ID) (Metadata, bool) {
	if r == nil {
		return Metadata{}, false
	}

	metadata, exists := r.items[id]
	return metadata, exists
}

// List returns capabilities in stable order so runtime behavior and tests stay deterministic.
func (r *Registry) List() []Metadata {
	if r == nil {
		return nil
	}

	ids := make([]string, 0, len(r.items))
	for id := range r.items {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)

	result := make([]Metadata, 0, len(ids))
	for _, id := range ids {
		result = append(result, r.items[ID(id)])
	}

	return result
}

// ToolContracts returns a deterministic flat view for runtime policy and adapter registration.
func (r *Registry) ToolContracts() []tool.Contract {
	if r == nil {
		return nil
	}

	capabilities := r.List()
	result := make([]tool.Contract, 0)

	for _, metadata := range capabilities {
		result = append(result, metadata.Tools...)
	}

	sort.Slice(result, func(i, j int) bool {
		return string(result[i].Name) < string(result[j].Name)
	})

	return result
}
