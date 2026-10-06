package jsonio

import (
	"context"
	"encoding/json/jsontext"
	"errors"
	"fmt"
	"io"
	"iter"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// Request bounds each document in a sequential JSON stream.
// Source owns its lifetime. The complete stream has no retained collection;
// Limits govern each document, including whitespace preceding its value.
type Request struct {
	// Source is the caller-owned sequential byte source.
	Source io.Reader
	// Limits bound each document independently of total stream extent.
	Limits core.StrictJSONLimits
}

// Validate refuses an absent source or invalid mechanical admission limits.
func (r Request) Validate() error {
	if core.ReaderIsNil(r.Source) {
		return errors.Join(core.ErrJSONContract, errors.New("JSON document source is absent"))
	}
	return r.Limits.Validate()
}

// Documents decodes one validated typed document at a time. Go owns JSON
// framing; Primitive owns per-document byte, shape and schema limits. Stopping
// the consumer performs no further reads. A failed read or decode emits one
// zero-value refusal and ends the sequence. A bare EOF alone closes the stream.
// The underlying source must provide its own interruptible read lifetime;
// cancellation is checked before reads and before publication.
func Documents[Document core.Validatable](ctx context.Context, request Request) iter.Seq2[Document, error] {
	return func(yield func(Document, error) bool) {
		var zero Document
		if ctx == nil {
			yield(zero, errors.Join(core.ErrJSONContract, errors.New("JSON document context is absent")))
			return
		}
		if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
			yield(zero, err)
			return
		}
		bounded := &io.LimitedReader{R: request.Source}
		decoder := jsontext.NewDecoder(bounded)
		for {
			data, err := readJSONStreamValue(ctx, decoder, bounded, request.Limits.DocumentMaximumBytes)
			if err != nil {
				if err == io.EOF {
					return
				}
				yield(zero, err)
				return
			}
			document, err := core.DecodeStrictJSONBytes[Document](data, request.Limits)
			if err := errors.Join(err, contextstate.Validate(ctx)); err != nil {
				yield(zero, err)
				return
			}
			if !yield(document, nil) {
				return
			}
		}
	}
}

func readJSONStreamValue(ctx context.Context, decoder *jsontext.Decoder, bounded *io.LimitedReader, maximum core.ByteCount) ([]byte, error) {
	maximumBytes, err := maximum.Uint64()
	if err != nil {
		return nil, err
	}
	if err := contextstate.Validate(ctx); err != nil {
		return nil, err
	}
	start := decoder.InputOffset()
	bounded.N = int64(maximumBytes) + 1 - int64(len(decoder.UnreadBuffer()))
	data, err := decoder.ReadValue()
	if bounded.N <= 0 && err != nil {
		return nil, errors.Join(&ExtentError{Maximum: maximum}, err)
	}
	if err == io.EOF {
		return nil, io.EOF
	}
	if err != nil {
		return nil, fmt.Errorf("read JSON document: %w", errors.Join(core.ErrJSONContract, err))
	}
	if decoder.InputOffset()-start > int64(maximumBytes) {
		return nil, &ExtentError{Maximum: maximum}
	}
	return data, nil
}

// ExtentError records refusal at the caller's per-document byte boundary.
// It unwraps to the shared JSON identity; callers use errors.As for the extent.
type ExtentError struct {
	// Maximum is the caller's validated document byte budget.
	Maximum core.ByteCount
}

// Validate rejects an unowned zero byte boundary.
func (e ExtentError) Validate() error { return e.Maximum.Validate() }

// Error explains the admission refusal without making prose its identity.
func (e ExtentError) Error() string { return "JSON document exceeds its admitted byte boundary" }

// Unwrap preserves the Primitive JSON error identity.
func (e ExtentError) Unwrap() error { return core.ErrJSONContract }

var (
	_ core.Validatable = Request{}
	_ core.Validatable = ExtentError{}
	_ error            = ExtentError{}
)
