package session

import (
	"fmt"
	"strings"
)

// StateDelta makes session state changes explicit instead of mutating state silently.
type StateDelta map[string]any

// Validation prevents unnamed state changes from entering durable session history.
func (d StateDelta) Validate() error {
	for key := range d {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("session state delta contains empty key")
		}
	}

	return nil
}

// ApplyTo returns a new state map so callers do not mutate persisted state implicitly.
func (d StateDelta) ApplyTo(state map[string]any) (map[string]any, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}

	next := map[string]any{}
	for key, value := range state {
		next[key] = value
	}

	for key, value := range d {
		next[key] = value
	}

	return next, nil
}
