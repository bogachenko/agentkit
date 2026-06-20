package runtime

import "strings"

type ActiveTaskState struct {
	Active            bool
	Marketplace       string
	ResultKind        string
	OperationID       string
	OriginalRequest   string
	LastResultSummary string
	AvailableActions  []string
	Metadata          map[string]string
}

func (s ActiveTaskState) IsZero() bool {
	return !s.Active && strings.TrimSpace(s.Marketplace) == "" && strings.TrimSpace(s.ResultKind) == "" &&
		strings.TrimSpace(s.OperationID) == "" && strings.TrimSpace(s.OriginalRequest) == "" && strings.TrimSpace(s.LastResultSummary) == "" &&
		len(s.AvailableActions) == 0 && len(s.Metadata) == 0
}

func activeTaskFromAskUser(input ClassifierInput, message string) ActiveTaskState {
	original := firstNonEmpty(input.ActiveTask.OriginalRequest, input.UserPrompt)
	metadata := map[string]string{"source": "semantic_ask_user"}
	for key, value := range input.ActiveTask.Metadata {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			metadata[key] = value
		}
	}

	return ActiveTaskState{
		Active:            true,
		Marketplace:       input.ActiveTask.Marketplace,
		ResultKind:        input.ActiveTask.ResultKind,
		OperationID:       input.ActiveTask.OperationID,
		OriginalRequest:   original,
		LastResultSummary: strings.TrimSpace(message),
		AvailableActions:  []string{"answer_pending_question", "continue", "retry", "rerun"},
		Metadata:          metadata,
	}
}
