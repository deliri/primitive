package lease

import (
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/core"
)

type jsonStructureContract struct {
	depth  uint16
	fields uint16
}

func (c jsonStructureContract) limits() (core.StrictJSONLimits, error) {
	limits := core.ExtensibleJSONLimits()
	limits.NestingDepthMaximum = c.depth
	limits.ObjectFieldMaximum = c.fields
	limits.ArrayItemMaximum = 1
	if err := limits.Validate(); err != nil {
		return core.StrictJSONLimits{}, jsonError(err)
	}
	return limits, nil
}

func writeCanonical(destination io.Writer, data []byte) error {
	if core.WriterIsNil(destination) {
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
