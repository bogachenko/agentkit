package skill

import (
	"fmt"
	"sort"
)

// Registry gives agent builders deterministic access to validated skills without global mutable state.
type Registry struct {
	items map[ID]Skill
}

// NewRegistry creates isolated skill storage for each application or test.
func NewRegistry() *Registry {
	return &Registry{
		items: map[ID]Skill{},
	}
}

// Register validates skills before exposing them to agent builders.
func (r *Registry) Register(value Skill) error {
	if r == nil {
		return fmt.Errorf("skill registry is nil")
	}

	if err := value.Validate(); err != nil {
		return err
	}

	if _, exists := r.items[value.Manifest.ID]; exists {
		return fmt.Errorf("skill %q is already registered", string(value.Manifest.ID))
	}

	r.items[value.Manifest.ID] = value
	return nil
}

// Get returns a registered skill without exposing registry internals.
func (r *Registry) Get(id ID) (Skill, bool) {
	if r == nil {
		return Skill{}, false
	}

	value, exists := r.items[id]
	return value, exists
}

// List returns skills in stable order so agent context assembly stays deterministic.
func (r *Registry) List() []Skill {
	if r == nil {
		return nil
	}

	ids := make([]string, 0, len(r.items))
	for id := range r.items {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)

	result := make([]Skill, 0, len(ids))
	for _, id := range ids {
		result = append(result, r.items[ID(id)])
	}

	return result
}
