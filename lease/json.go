package lease

import (
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/core"
)

type jsonStructureContract struct {
	maximumBytes int
	depth        uint16
	fields       uint16
}

func (c jsonStructureContract) limits() (core.StrictJSONLimits, error) {
	boundedMaximum, err := core.CheckedUint32FromInt(c.maximumBytes)
	if err != nil {
		return core.StrictJSONLimits{}, jsonError(err)
	}
	documentMaximum, err := core.NewByteCount(uint64(boundedMaximum))
	if err != nil {
		return core.StrictJSONLimits{}, jsonError(err)
	}
	limits := core.StrictJSONLimits{
		DocumentMaximumBytes: documentMaximum,
		NestingDepthMaximum:  c.depth,
		ObjectFieldMaximum:   c.fields,
		ArrayItemMaximum:     1,
	}
	if err := limits.Validate(); err != nil {
		return core.StrictJSONLimits{}, jsonError(err)
	}
	return limits, nil
}

func writeCanonical(destination io.Writer, data []byte) error {
	if destination == nil {
		return contractError(errors.New("lease canonical destination is nil"))
	}
	written, err := destination.Write(data)
	if err != nil {
		return contractError(err)
	}
	if written != len(data) {
		return contractError(io.ErrShortWrite)
	}
	return nil
}
