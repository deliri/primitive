package runnercontrol

import (
	"errors"
	"go/token"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

// SourcePosition decodes cmd/vet's positioned diagnostic rendering. These are
// the coordinates emitted by Go, including any native //line mapping; no file
// access, physical-source attestation or recoverable byte offset is implied.
// Splitting from the right preserves colons in native filenames and drive names.
func (o GoVetObservation) SourcePosition() (token.Position, error) {
	if err := o.Validate(); err != nil {
		return token.Position{}, err
	}
	if o.Kind != GoVetObservationDiagnostic {
		return token.Position{}, core.ErrPrimitiveContract
	}
	last := strings.LastIndexByte(o.Position, ':')
	if last < 0 {
		return token.Position{}, core.ErrPrimitiveContract
	}
	column, err := strconv.Atoi(o.Position[last+1:])
	if err != nil || column <= 0 {
		return token.Position{}, errors.Join(core.ErrPrimitiveContract, err)
	}
	previous := strings.LastIndexByte(o.Position[:last], ':')
	if previous <= 0 {
		return token.Position{}, core.ErrPrimitiveContract
	}
	line, err := strconv.Atoi(o.Position[previous+1 : last])
	if err != nil || line <= 0 {
		return token.Position{}, errors.Join(core.ErrPrimitiveContract, err)
	}
	return token.Position{Filename: o.Position[:previous], Offset: -1, Line: line, Column: column}, nil
}
