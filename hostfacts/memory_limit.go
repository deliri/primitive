package hostfacts

import (
	"context"
	"errors"
	"runtime/debug"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// GoMemoryLimitRequest carries the caller's nonnegative Go runtime soft limit.
// The caller owns budgeting and the meaning of crossing that limit.
// Zero is a valid native setting, not an absent request or an unbounded limit.
type GoMemoryLimitRequest struct {
	Limit core.ByteLength
}

func (r GoMemoryLimitRequest) Validate() error {
	return validateGoMemoryLimit(r.Limit)
}

// GoMemoryLimitResult records the previous limit returned by Go and the limit
// applied by this operation. A later caller may change the process-wide limit.
// This observation does not claim a hard memory ceiling or durable completion.
type GoMemoryLimitResult struct {
	Previous core.ByteLength
	Applied  core.ByteLength
	applied  bool
}

func (r GoMemoryLimitResult) Validate() error {
	if !r.applied {
		return core.ErrHostFactsObservation
	}
	return errors.Join(validateGoMemoryLimit(r.Previous), validateGoMemoryLimit(r.Applied))
}

func validateGoMemoryLimit(limit core.ByteLength) error {
	if err := limit.Validate(); err != nil {
		return errors.Join(core.ErrHostFactsContract, err)
	}
	return nil
}

// ApplyGoMemoryLimit executes one validated runtime setting operation. Terminal
// contexts are refused before mutation; cancellation after admission cannot
// undo this synchronous effect. No monitoring policy or runtime model is added.
func ApplyGoMemoryLimit(ctx context.Context, request GoMemoryLimitRequest) (GoMemoryLimitResult, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return GoMemoryLimitResult{}, err
	}
	if err := request.Validate(); err != nil {
		return GoMemoryLimitResult{}, err
	}
	previous, err := core.NewByteLength(uint64(debug.SetMemoryLimit(int64(request.Limit.Uint64()))))
	if err != nil {
		return GoMemoryLimitResult{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	result := GoMemoryLimitResult{Previous: previous, Applied: request.Limit, applied: true}
	return result, result.Validate()
}

var _ core.Validatable = GoMemoryLimitRequest{}
var _ core.Validatable = GoMemoryLimitResult{}
