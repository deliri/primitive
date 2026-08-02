package exchange

import (
	"errors"
	"math"

	"github.com/deliri/primitive/v2026/core"
)

const declaredBodyLengthAbsent = -1

type declaredBodyLength struct {
	length  core.ByteLength
	present bool
}

func parseDeclaredBodyLength(value int64) (declaredBodyLength, error) {
	if value == declaredBodyLengthAbsent {
		return declaredBodyLength{}, nil
	}
	if value < declaredBodyLengthAbsent {
		return declaredBodyLength{}, errors.Join(
			core.ErrExchangeContract,
			errors.New("declared body length is not an expressible extent"),
		)
	}
	unsigned, err := core.CheckedUint64FromInt64(value)
	if err != nil {
		return declaredBodyLength{}, errors.Join(core.ErrExchangeContract, err)
	}
	length, err := core.NewByteLength(unsigned)
	if err != nil {
		return declaredBodyLength{}, errors.Join(core.ErrExchangeContract, err)
	}
	return declaredBodyLength{length: length, present: true}, nil
}

func (d declaredBodyLength) Validate() error {
	if !d.present && d.length.Uint64() != 0 {
		return errors.Join(
			core.ErrExchangeContract,
			errors.New("absent declared body length carries an extent"),
		)
	}
	if err := d.length.Validate(); err != nil {
		return errors.Join(core.ErrExchangeContract, err)
	}
	return nil
}

func (d declaredBodyLength) exceedsLimit(limit core.ByteCount) (bool, error) {
	allowed, err := limit.Uint64()
	if err != nil {
		return false, errors.Join(core.ErrExchangeContract, err)
	}
	if err := d.Validate(); err != nil {
		return false, err
	}
	return d.present && d.length.Uint64() > allowed, nil
}

func (d declaredBodyLength) reservedExtent(limit core.ByteCount) (int, error) {
	exceeds, err := d.exceedsLimit(limit)
	if err != nil {
		return 0, err
	}
	if !d.present {
		return 0, nil
	}
	value := d.length.Uint64()
	if exceeds {
		value, err = limit.Uint64()
		if err != nil {
			return 0, errors.Join(core.ErrExchangeContract, err)
		}
	}
	if value > math.MaxInt {
		return 0, errors.Join(core.ErrExchangeContract, core.ErrNumericOverflow)
	}
	return int(value), nil
}

// admittedBodyLength projects one transport-declared body extent and refuses a
// declaration that already exceeds the operation's authorized aggregate limit.
// An absent or understated declaration remains subject to the same limit while
// bytes are read.
func admittedBodyLength(
	contentLength int64,
	limit core.ByteCount,
) (declaredBodyLength, error) {
	declared, err := parseDeclaredBodyLength(contentLength)
	if err != nil {
		return declaredBodyLength{}, err
	}
	exceeds, err := declared.exceedsLimit(limit)
	if err != nil {
		return declaredBodyLength{}, err
	}
	if exceeds {
		return declaredBodyLength{}, core.ErrExchangeBodyLimit
	}
	return declared, nil
}
