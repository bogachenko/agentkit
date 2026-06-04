package llm

import (
	corellm "github.com/bogachenko/agentkit/core/llm"
	"google.golang.org/adk/session"
)

// EventToCoreMessage extracts provider-neutral conversation content from an ADK session event.
func EventToCoreMessage(event *session.Event) (corellm.Message, bool) {
	if event == nil || event.Content == nil {
		return corellm.Message{}, false
	}

	return ContentToCoreMessage(event.Content)
}

// EventsToRecentCoreMessages converts recent ADK event history without exposing ADK session internals.
func EventsToRecentCoreMessages(events session.Events, limit int) []corellm.Message {
	if events == nil || events.Len() == 0 {
		return nil
	}

	if limit <= 0 || limit > events.Len() {
		limit = events.Len()
	}

	start := events.Len() - limit
	result := make([]corellm.Message, 0, limit)

	for index := start; index < events.Len(); index++ {
		message, ok := EventToCoreMessage(events.At(index))
		if !ok {
			continue
		}

		result = append(result, message)
	}

	return result
}
