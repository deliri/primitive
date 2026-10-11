package jsonio

import (
	"context"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// ObjectDecoder owns one Go decoder and its closed-object admission hooks.
// Go owns framing, buffers and cursor. The synchronous caller lends the source
// lifetime and must not use this decoder concurrently. No documents, native
// file custody or mirrored parser phases are retained here.
type ObjectDecoder[Document core.Validatable] struct {
	decoder   *jsontext.Decoder
	nullGuard *jsonv2.Unmarshalers
}

func NewObjectDecoder[Document core.Validatable](request ObjectSourceRequest) (*ObjectDecoder[Document], error) {
	if err := errors.Join(request.Validate(), validateObjectType[Document]()); err != nil {
		return nil, err
	}
	return &ObjectDecoder[Document]{decoder: jsontext.NewDecoder(checkedJSONSource{source: request.Source}), nullGuard: jsonv2.UnmarshalFromFunc[any](rejectObjectNull)}, nil
}

func (d *ObjectDecoder[Document]) Validate() error {
	if d == nil || d.decoder == nil || d.nullGuard == nil {
		return core.ErrJSONContract
	}
	return nil
}

// Reset delegates replacement of the borrowed source directly to Go. Type
// admission and hooks belong to construction; every decoded value still passes
// the same unknown-member, null, context and Validate gates as Objects.
func (d *ObjectDecoder[Document]) Reset(ctx context.Context, request ObjectSourceRequest) error {
	if err := errors.Join(contextstate.Validate(ctx), d.Validate(), request.Validate()); err != nil {
		return err
	}
	d.decoder.Reset(checkedJSONSource{source: request.Source})
	return contextstate.Validate(ctx)
}

// Decode returns one validated object or a zero-value refusal. Only a bare
// io.EOF ends a complete source; callers must consume that closure before
// treating a prefix as the complete document sequence.
func (d *ObjectDecoder[Document]) Decode(ctx context.Context) (Document, error) {
	if err := errors.Join(contextstate.Validate(ctx), d.Validate()); err != nil {
		var zero Document
		return zero, err
	}
	return readObject[Document](ctx, d.decoder, d.nullGuard)
}
