package port

import "context"

// Tracer keeps runtime tracing provider-independent.
type Tracer interface {
	Start(ctx context.Context, name string, attrs map[string]any) (context.Context, Span)
}

// Span gives core a minimal tracing contract without importing OpenTelemetry.
type Span interface {
	SetAttributes(attrs map[string]any)
	AddEvent(name string, attrs map[string]any)
	RecordError(err error)
	End()
}
