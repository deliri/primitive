package jsonio

import (
	"context"
	jsonv2 "encoding/json/v2"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// ownedCodecDestination borrows one operation's writer and context. It never
// closes the writer; the caller owns interruption of an already blocked Write.
type ownedCodecDestination struct {
	ctx         context.Context
	destination io.Writer
}

func (w ownedCodecDestination) Write(data []byte) (int, error) {
	if err := contextstate.Validate(w.ctx); err != nil {
		return 0, &nativeJSONError{cause: err}
	}
	n, err := (checkedJSONDestination{destination: w.destination}).Write(data)
	if cancelled := contextstate.Validate(w.ctx); cancelled != nil {
		return n, &nativeJSONError{cause: errors.Join(cancelled, err)}
	}
	return n, err
}

// WriteValidated streams one validated native value without a top-level LF.
// Native custom streaming methods run only under explicit RootOwnedMethods.
// Policy/type/owner refusals occur before writes. Native grammar is incremental;
// a later hook, writer or cancellation failure leaves provisional bytes, which
// the caller must discard. No whole-value copy or sequence is retained here.
// Unlike MarshalValidated, this door cannot inspect a completed projection:
// declared-field and opaque-hook null emission belongs to document.Validate
// and its owning codec. PointerNulls governs decoding and whole-value output.
// Source: https://pkg.go.dev/encoding/json/v2#MarshalWrite
func WriteValidated[Document core.Validatable](ctx context.Context, destination io.Writer, document Document, policy OwnedCodecPolicy) error {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	if err := validateOwnedEncoding(document, policy); err != nil {
		return err
	}
	if core.WriterIsNil(destination) {
		return core.ErrJSONContract
	}
	writer := ownedCodecDestination{ctx: ctx, destination: destination}
	err := jsonv2.MarshalWrite(writer, document, jsonv2.Deterministic(true), jsonv2.WithMarshalers(ownedMarshalGuards(ctx)))
	return objectReadOutcome(ctx, err)
}

// EncodeOwned streams one validated native value at the token cursor's current
// array/object value position. The cursor owns JSON grammar, including commas.
// An admitted native streaming hook may publish a provisional prefix before
// refusal. The existing cursor's sink owns interruption of blocked writes;
// context is checked before native value encoding and before completion.
// As in WriteValidated, the document owner validates emitted null policy;
// this streaming door makes no completed-projection null-inspection promise.
// Source: https://pkg.go.dev/encoding/json/v2#MarshalEncode
func EncodeOwned[Document core.Validatable](ctx context.Context, encoder *TokenEncoder, document Document, policy OwnedCodecPolicy) error {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	if err := encoder.Validate(); err != nil {
		return err
	}
	if err := validateOwnedEncoding(document, policy); err != nil {
		return err
	}
	err := jsonv2.MarshalEncode(encoder.encoder, document, jsonv2.Deterministic(true), jsonv2.WithMarshalers(ownedMarshalGuards(ctx)))
	return objectReadOutcome(ctx, err)
}
