package hostfacts

import (
	"context"
	"errors"
	"math"
	"runtime/debug"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// GoMemoryLimitRequest carries the caller's positive Go runtime soft limit.
// The caller owns budgeting and the meaning of crossing that limit.
type GoMemoryLimitRequest struct {
	Limit core.ByteCount
}

func (r GoMemoryLimitRequest) Validate() error {
	return validateGoMemoryLimit(r.Limit)
}

// GoMemoryLimitResult records the previous limit returned by Go and the limit
// applied by this operation. A later caller may change the process-wide limit.
// This observation does not claim a hard memory ceiling or durable completion.
type GoMemoryLimitResult struct {
	Previous core.ByteCount
	Applied  core.ByteCount
}

func (r GoMemoryLimitResult) Validate() error {
	return errors.Join(validateGoMemoryLimit(r.Previous), validateGoMemoryLimit(r.Applied))
}

func validateGoMemoryLimit(limit core.ByteCount) error {
	value, err := limit.Uint64()
	if err != nil || value > math.MaxInt64 {
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
	limit, err := request.Limit.Uint64()
	if err != nil {
		return GoMemoryLimitResult{}, err
	}
	previous, err := core.NewByteCount(uint64(debug.SetMemoryLimit(int64(limit))))
	if err != nil {
		return GoMemoryLimitResult{}, errors.Join(core.ErrHostFactsObservation, err)
	}
	result := GoMemoryLimitResult{Previous: previous, Applied: request.Limit}
	return result, result.Validate()
}

var _ core.Validatable = GoMemoryLimitRequest{}
var _ core.Validatable = GoMemoryLimitResult{}
