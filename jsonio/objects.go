package jsonio

import (
	"context"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"io"
	"iter"
	"reflect"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// ObjectSourceRequest names a caller-owned source of closed typed objects.
// The admitted Go type owns field meaning and record size; this capability
// retains one object, never the sequence or a raw copy of each document.
type ObjectSourceRequest struct{ Source io.Reader }

func (r ObjectSourceRequest) Validate() error {
	if core.ReaderIsNil(r.Source) {
		return core.ErrJSONContract
	}
	return nil
}

// Objects decodes directly through Go into the declared type, rejects unknown
// members and validates before publication. Memory follows the declared type
// and Go's scalar/parser buffers. A type with collection fields admits their
// retention; callers requiring constant sequence memory use scalar records.
// The source must own interruptible reads. A prefix is provisional until EOF.
func Objects[Document core.Validatable](ctx context.Context, request ObjectSourceRequest) iter.Seq2[Document, error] {
	return func(yield func(Document, error) bool) {
		var zero Document
		if err := errors.Join(contextstate.Validate(ctx), request.Validate(), validateObjectType[Document]()); err != nil {
			yield(zero, err)
			return
		}
		decoder := jsontext.NewDecoder(checkedJSONSource{source: request.Source})
		nullGuard := jsonv2.UnmarshalFromFunc[any](rejectObjectNull)
		for {
			document, err := readObject[Document](ctx, decoder, nullGuard)
			if err == io.EOF {
				return
			}
			if err != nil {
				yield(zero, err)
				return
			}
			if !yield(document, nil) {
				return
			}
		}
	}
}

func readObject[Document core.Validatable](ctx context.Context, decoder *jsontext.Decoder, nullGuard *jsonv2.Unmarshalers) (Document, error) {
	var document Document
	if err := prepareObjectRead(ctx, decoder); err != nil {
		return document, err
	}
	readErr := jsonv2.UnmarshalDecode(decoder, &document, jsonv2.RejectUnknownMembers(true), jsonv2.WithUnmarshalers(nullGuard))
	if err := objectReadOutcome(ctx, readErr); err != nil {
		var zero Document
		return zero, err
	}
	if err := document.Validate(); err != nil {
		var zero Document
		return zero, errors.Join(core.ErrJSONContract, err)
	}
	return document, nil
}

func prepareObjectRead(ctx context.Context, decoder *jsontext.Decoder) error {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	kind := decoder.PeekKind()
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	if kind != jsontext.KindBeginObject && kind != jsontext.KindInvalid {
		return core.ErrJSONContract
	}
	return nil
}

func objectReadOutcome(ctx context.Context, readErr error) error {
	if err := contextstate.Validate(ctx); err != nil {
		return errors.Join(err, readErr)
	}
	if readErr == io.EOF {
		return io.EOF
	}
	if readErr != nil {
		return errors.Join(core.ErrJSONContract, readErr)
	}
	return nil
}

type ObjectDestinationRequest struct{ Destination io.Writer }

func (r ObjectDestinationRequest) Validate() error {
	if core.WriterIsNil(r.Destination) {
		return core.ErrJSONContract
	}
	return nil
}

// ObjectEncoder owns one reusable Go encoder. The destination owns its
// interruptible write lifetime. A failed encode may have provisional bytes;
// callers discard that output rather than claiming a completed record.
type ObjectEncoder[Document core.Validatable] struct{ encoder *jsontext.Encoder }

func NewObjectEncoder[Document core.Validatable](request ObjectDestinationRequest) (*ObjectEncoder[Document], error) {
	if err := errors.Join(request.Validate(), validateObjectType[Document]()); err != nil {
		return nil, err
	}
	return &ObjectEncoder[Document]{encoder: jsontext.NewEncoder(checkedJSONDestination{destination: request.Destination})}, nil
}

func (e *ObjectEncoder[Document]) Validate() error {
	if e == nil || e.encoder == nil {
		return core.ErrJSONContract
	}
	return nil
}

// Encode emits Go's deterministic object shape and one top-level LF. Context
// is checked before execution and before returning a completion observation.
func (e *ObjectEncoder[Document]) Encode(ctx context.Context, document Document) error {
	if err := errors.Join(contextstate.Validate(ctx), e.Validate()); err != nil {
		return err
	}
	if value := reflect.ValueOf(document); value.Kind() == reflect.Pointer && value.IsNil() {
		return core.ErrJSONContract
	}
	if err := document.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	if err := jsonv2.MarshalEncode(e.encoder, document, jsonv2.Deterministic(true)); err != nil {
		return errors.Join(core.ErrJSONContract, err, contextstate.Validate(ctx))
	}
	return contextstate.Validate(ctx)
}

// This capability uses Go's declared struct fields, rather than delegating the
// closed root protocol to an opaque custom codec. Nested nominal field codecs
// remain owned by their types. Both value and pointer method sets are checked.
func validateObjectType[Document core.Validatable]() error {
	root := reflect.TypeFor[Document]()
	if root.Kind() == reflect.Pointer {
		root = root.Elem()
	}
	if root.Kind() != reflect.Struct {
		return core.ErrJSONContract
	}
	for _, candidate := range [...]reflect.Type{root, reflect.PointerTo(root)} {
		for _, hook := range [...]reflect.Type{
			reflect.TypeFor[interface{ MarshalJSON() ([]byte, error) }](), reflect.TypeFor[interface{ UnmarshalJSON([]byte) error }](),
			reflect.TypeFor[jsonv2.MarshalerTo](), reflect.TypeFor[jsonv2.UnmarshalerFrom](),
		} {
			if candidate.Implements(hook) {
				return core.ErrJSONContract
			}
		}
	}
	return nil
}

func rejectObjectNull(decoder *jsontext.Decoder, _ any) error {
	if decoder.PeekKind() == jsontext.KindNull {
		return core.ErrJSONContract
	}
	// Go's unmarshaler dispatch continues with its normal declared-field codec.
	return errors.ErrUnsupported
}

var (
	_ core.Validatable = ObjectSourceRequest{}
	_ core.Validatable = ObjectDestinationRequest{}
)
