package exchange

import (
	"errors"

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

// streamContentLength projects optional caller intent into Go's HTTP framing.
func streamContentLength(length *core.ByteLength) (int64, error) {
	if length == nil {
		return declaredBodyLengthAbsent, nil
	}
	return length.Int64()
}
