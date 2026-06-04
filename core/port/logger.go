package port

// Logger keeps core observable without binding it to a concrete logging library.
type Logger interface {
	Printf(format string, args ...any)
}
