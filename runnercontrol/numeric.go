package runnercontrol

import (
	"errors"
	"math"

	"github.com/deliri/primitive/v2026/core"
)

func checkedUint16FromUint64(value uint64) (uint16, error) {
	if value > math.MaxUint16 {
		return 0, errors.Join(core.ErrNumericOverflow, errors.New("uint64 does not fit uint16"))
	}
	return uint16(value), nil // #nosec G115 -- the preceding bound proves the narrowing conversion.
}
