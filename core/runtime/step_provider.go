package runtime

import (
	"context"
	"errors"
)

var ErrStepSourceDone = errors.New("runtime step source is done")

// StepProvider is the neutral source of runtime steps; ADK, tests, or other engines implement it outside core.
type StepProvider interface {
	NextSteps(ctx context.Context, state State) ([]Step, error)
}
