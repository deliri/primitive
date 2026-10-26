package jsonio

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"errors"
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
	// Members keeps owned documents closed unless a foreign format is named.
	Members MemberPolicy
}

// Validate refuses an absent source or invalid mechanical admission limits.
func (r Request) Validate() error {
	if core.ReaderIsNil(r.Source) {
		return errors.Join(core.ErrJSONContract, errors.New("JSON document source is absent"))
	}
	return errors.Join(r.Limits.Validate(), r.Members.Validate())
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
		source := &contextJSONSource{ctx: ctx, source: checkedJSONSource{source: request.Source}}
		bounded := &io.LimitedReader{R: source}
		decoder := jsontext.NewDecoder(bounded)
		var objects documentObjects[Document]
		for {
			data, err := readJSONStreamValue(ctx, decoder, bounded, request.Limits.DocumentMaximumBytes)
			err = source.observedError(err)
			if err != nil {
				if err == io.EOF {
					return
				}
				yield(zero, err)
				return
			}
			document, err := objects.decode(ctx, data, request)
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
		return nil, jsonOperationError(err)
	}
	if decoder.InputOffset()-start > int64(maximumBytes) {
		return nil, &ExtentError{Maximum: maximum}
	}
	return data, nil
}

// documentObjects reuses one object decoder across a stream's documents.
// Typed admission has one owner for both whole-value and object-stream doors.
// Rebinding performs exactly NewObjectDecoder's source setup, so each document
// starts from a fresh native cursor while Go's buffer and caches are reused.
// Only the current document's bytes are referenced, never the sequence.
type documentObjects[Document core.Validatable] struct {
	decoder *ObjectDecoder[Document]
	source  *bytes.Reader
}

func (o *documentObjects[Document]) decode(ctx context.Context, data []byte, request Request) (Document, error) {
	var zero Document
	if err := request.Limits.ValidateDocument(data); err != nil {
		return zero, err
	}
	if o.decoder == nil {
		source := bytes.NewReader(data)
		decoder, err := NewObjectDecoder[Document](ObjectSourceRequest{Source: source, Members: request.Members})
		if err != nil {
			return zero, err
		}
		o.decoder, o.source = decoder, source
	} else {
		o.source.Reset(data)
		*o.decoder.source = contextJSONSource{source: checkedJSONSource{source: o.source}}
		o.decoder.decoder.Reset(o.decoder.source)
	}
	return o.decoder.Decode(ctx)
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
