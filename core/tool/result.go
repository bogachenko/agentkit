package tool

// Result records structured tool output without coupling core to a concrete tool implementation.
type Result struct {
	Name   Name
	Output any
}

// Constructor keeps tool result identity explicit at call sites.
func NewResult(name Name, output any) Result {
	return Result{
		Name:   name,
		Output: output,
	}
}

// Validation prevents anonymous tool results from being written into conversation or ledger state.
func (r Result) Validate() error {
	return r.Name.Validate()
}
