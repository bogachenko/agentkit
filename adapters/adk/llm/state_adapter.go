package llm

import "google.golang.org/adk/session"

type State struct {
	state session.State
}

func NewState(state session.State) *State {
	return &State{state: state}
}

func (s *State) Get(key string) (any, bool, error) {
	if s == nil || s.state == nil {
		return nil, false, nil
	}
	value, err := s.state.Get(key)
	if err != nil {
		if err == session.ErrStateKeyNotExist {
			return nil, false, nil
		}
		return nil, false, err
	}
	return value, true, nil
}

func (s *State) Set(key string, value any) error {
	if s == nil || s.state == nil {
		return nil
	}
	return s.state.Set(key, value)
}
