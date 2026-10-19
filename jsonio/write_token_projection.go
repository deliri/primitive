package jsonio

import (
	"context"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// TokenProjectionRequest borrows a destination and one synchronous projection.
// Go validates that Use emits exactly one complete JSON value. The cursor loan
// expires when Use returns. No complete document is retained by this request.
type TokenProjectionRequest struct {
	Destination io.Writer
	Use         func(*TokenEncoder) error
}

func (r TokenProjectionRequest) Validate() error {
	if core.WriterIsNil(r.Destination) || r.Use == nil {
		return core.ErrJSONContract
	}
	return nil
}

// WriteTokenProjection streams one value without a top-level LF. Native token
// grammar and canonical per-record projections can share the borrowed cursor.
// Late refusal leaves provisional bytes, which the caller must discard.
func WriteTokenProjection(ctx context.Context, request TokenProjectionRequest) error {
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return err
	}
	marshalers := jsonv2.MarshalToFunc[TokenProjectionRequest](func(native *jsontext.Encoder, projection TokenProjectionRequest) error {
		cursor := &TokenEncoder{encoder: native}
		defer func() { cursor.encoder = nil }()
		return projection.Use(cursor)
	})
	writer := ownedCodecDestination{ctx: ctx, destination: request.Destination}
	return objectReadOutcome(ctx, jsonv2.MarshalWrite(writer, request, jsonv2.WithMarshalers(marshalers)))
}

var _ core.Validatable = TokenProjectionRequest{}
