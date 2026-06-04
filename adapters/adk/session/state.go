package adksession

import (
	"errors"

	adksdk "google.golang.org/adk/session"
)

// StateReader hides ADK state error semantics from hook implementations.
type StateReader struct {
	state adksdk.State
}

// NewStateReader keeps hooks decoupled from direct ADK state access.
func NewStateReader(state adksdk.State) StateReader {
	return StateReader{
		state: state,
	}
}

// GetString reads optional string state values without treating missing keys as fatal errors.
func (r StateReader) GetString(key string) (string, bool, error) {
	if r.state == nil {
		return "", false, nil
	}

	value, err := r.state.Get(key)
	if err != nil {
		if errors.Is(err, adksdk.ErrStateKeyNotExist) {
			return "", false, nil
		}

		return "", false, err
	}

	text, ok := value.(string)
	if !ok || text == "" {
		return "", false, nil
	}

	return text, true, nil
}
