package port

// IDGenerator avoids hidden random ID creation inside deterministic core logic.
type IDGenerator interface {
	NewID() string
}
