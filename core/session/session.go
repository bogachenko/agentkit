package session

import (
	"fmt"
	"time"
)

// Session stores durable conversation metadata without provider-specific session objects.
type Session struct {
	ID        ID
	State     map[string]any
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Validation prevents incomplete session records from entering persistence.
func (s Session) Validate() error {
	if err := s.ID.Validate(); err != nil {
		return err
	}

	if s.CreatedAt.IsZero() {
		return fmt.Errorf("session %q created_at is required", string(s.ID))
	}

	if s.UpdatedAt.IsZero() {
		return fmt.Errorf("session %q updated_at is required", string(s.ID))
	}

	if s.UpdatedAt.Before(s.CreatedAt) {
		return fmt.Errorf("session %q updated before it was created", string(s.ID))
	}

	if s.State == nil {
		return fmt.Errorf("session %q state must be an empty map, not nil", string(s.ID))
	}

	return nil
}
