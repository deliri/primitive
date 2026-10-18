package lineio

import (
	"bytes"
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// RecordSuffixTrimRequest selects trailing byte values to omit from a borrowed
// record. Cutset is byte data, not a text or Unicode whitespace policy.
type RecordSuffixTrimRequest struct {
	Input  RecordFragmentRequest
	Cutset []byte
}

func (r RecordSuffixTrimRequest) Validate() error { return r.Input.Validate() }

// TrimRecordSuffix returns original coordinates through the final byte outside
// Cutset. A fully trimmed record returns the zero range. Native framing and the
// complete declared extent must succeed before any coordinates escape. The
// operation retains one fixed read window, never the suffix or record text.
func TrimRecordSuffix(ctx context.Context, request RecordSuffixTrimRequest) (result RecordRange, resultErr error) {
	defer func() {
		if resultErr != nil {
			result = RecordRange{}
			if ctx != nil {
				resultErr = errors.Join(resultErr, context.Cause(ctx))
			}
		}
	}()
	var observed, end uint64
	for fragment, err := range RecordFragments(ctx, request.Input) {
		if err != nil {
			return RecordRange{}, err
		}
		trimmed := len(fragment.Bytes)
		for trimmed > 0 && bytes.IndexByte(request.Cutset, fragment.Bytes[trimmed-1]) >= 0 {
			trimmed--
		}
		if trimmed > 0 {
			end = observed + uint64(trimmed)
		}
		observed += uint64(len(fragment.Bytes))
	}
	if err := contextstate.Validate(ctx); err != nil {
		return RecordRange{}, err
	}
	if end == 0 {
		return RecordRange{}, nil
	}
	extent, err := core.NewByteLength(end)
	if err != nil {
		return RecordRange{}, err
	}
	result = RecordRange{Offset: request.Input.Record.Offset, Bytes: extent, Framing: RecordFramingOpaque}
	return result, result.Validate()
}

var _ core.Validatable = RecordSuffixTrimRequest{}
