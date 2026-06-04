package session

import (
	"fmt"
	"strings"
)

// ID prevents session identity from being mixed with arbitrary strings across stores and adapters.
type ID string

// Validation prevents anonymous sessions from entering durable state.
func (id ID) Validate() error {
	if strings.TrimSpace(string(id)) == "" {
		return fmt.Errorf("session id is required")
	}

	return nil
}

// EventID gives every session event stable identity for append-only history and audit.
type EventID string

// Validation prevents anonymous events from entering session history.
func (id EventID) Validate() error {
	if strings.TrimSpace(string(id)) == "" {
		return fmt.Errorf("session event id is required")
	}

	return nil
}

// String allows infrastructure ports to serialize session identity without knowing session internals.
func (id ID) String() string {
	return string(id)
}
