package port

import "time"

// Clock makes time explicit so runtime behavior stays deterministic in tests.
type Clock interface {
	Now() time.Time
}
