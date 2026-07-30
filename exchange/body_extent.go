package exchange

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// admittedBodyLength projects one transport-declared body extent and refuses a
// declaration that already exceeds the operation's authorized aggregate limit.
// An absent or understated declaration remains subject to the same limit while
// bytes are read.
func admittedBodyLength(
	contentLength int64,
	limit core.ByteCount,
) (core.DeclaredBodyLength, error) {
	declared, err := core.ParseDeclaredBodyLength(contentLength)
	if err != nil {
		return core.DeclaredBodyLength{}, errors.Join(
			core.ErrExchangeContract,
			err,
		)
	}
	exceeds, err := declared.ExceedsLimit(limit)
	if err != nil {
		return core.DeclaredBodyLength{}, errors.Join(
			core.ErrExchangeContract,
			err,
		)
	}
	if exceeds {
		return core.DeclaredBodyLength{}, core.ErrExchangeBodyLimit
	}
	return declared, nil
}
