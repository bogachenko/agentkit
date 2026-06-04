package llm

import "fmt"

// Message gives runtime and adapters one provider-neutral conversation format.
type Message struct {
	Role  Role
	Parts []Part
}

// Constructor keeps message assembly explicit at call sites.
func NewMessage(role Role, parts ...Part) Message {
	return Message{
		Role:  role,
		Parts: parts,
	}
}

// Validation prevents incomplete messages from entering session state or model requests.
func (m Message) Validate() error {
	if err := m.Role.Validate(); err != nil {
		return err
	}

	if len(m.Parts) == 0 {
		return fmt.Errorf("message with role %q requires at least one part", string(m.Role))
	}

	for i, part := range m.Parts {
		if err := part.Validate(); err != nil {
			return fmt.Errorf("message part %d: %w", i, err)
		}
	}

	return nil
}
