package jsonio

import (
	"context"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"io"
	"reflect"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// PeekKind observes Go's next token kind without consuming a valid token.
// A cached native peek refusal is consumed once and its cause preserved.
func (d *TokenDecoder) PeekKind(ctx context.Context) (TokenKind, error) {
	if err := projectionContextError(ctx); err != nil {
		return TokenUnknown, err
	}
	if err := d.prepare(ctx); err != nil {
		return TokenUnknown, err
	}
	defer func() { d.source.ctx = nil }()
	kind := d.decoder.PeekKind()
	if kind == jsontext.KindInvalid {
		_, err := d.decoder.ReadToken()
		if err == nil {
			err = core.ErrJSONContract
		}
		return TokenUnknown, d.source.observedError(objectReadOutcome(ctx, err))
	}
	if err := projectionContextError(ctx); err != nil {
		return TokenUnknown, err
	}
	if err := d.source.observedError(nil); err != nil {
		return TokenUnknown, err
	}
	return projectKind(kind)
}

// MultipleValuesError identifies a second native value where EOF was required.
type MultipleValuesError struct{}

func (*MultipleValuesError) Error() string { return "multiple JSON values" }
func (*MultipleValuesError) Unwrap() error { return core.ErrJSONContract }

// RequireEOF closes a single-document agreement without retaining another
// value. Go owns trailing whitespace, grammar and cached source refusals.
func (d *TokenDecoder) RequireEOF(ctx context.Context) error {
	_, err := d.PeekKind(ctx)
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return err
	}
	return &MultipleValuesError{}
}

// DecodeProjectionAt decodes one concrete, hook-free struct at Go's current
// value position without a raw value copy. The caller validates product meaning
// before committing it. Go owns unknown-member traversal and its native depth
// limit; TokenRequest's lower depth gate applies to token and ReadValue calls,
// not opaque nested field codecs or this typed value operation.
// Source: https://pkg.go.dev/encoding/json/v2#UnmarshalDecode
func DecodeProjectionAt[Document any](ctx context.Context, cursor *TokenDecoder, members MemberPolicy) (Document, error) {
	var zero Document
	if err := errors.Join(projectionContextError(ctx), members.Validate(), validateObjectType[Document]()); err != nil {
		return zero, err
	}
	if err := cursor.prepare(ctx); err != nil {
		return zero, err
	}
	defer func() { cursor.source.ctx = nil }()
	if err := cursor.source.observedError(prepareObjectRead(ctx, cursor.decoder)); err != nil {
		return zero, err
	}
	guards := jsonv2.UnmarshalFromFunc[any](func(_ *jsontext.Decoder, _ any) error {
		if err := projectionContextError(ctx); err != nil {
			return &nativeJSONError{cause: err}
		}
		return errors.ErrUnsupported
	})
	var document Document
	err := jsonv2.UnmarshalDecode(cursor.decoder, &document, jsonv2.RejectUnknownMembers(members == MembersDeclaredOnly), jsonv2.WithUnmarshalers(guards))
	if err := errors.Join(objectReadOutcome(ctx, err), projectionContextError(ctx), cursor.source.refusal); err != nil {
		return zero, err
	}
	return document, nil
}

// DecodeObjectAt adds the owning type's validation to structural decoding.
// Failure publishes zero; a surrounding array remains a forward native cursor.
func DecodeObjectAt[Document core.Validatable](ctx context.Context, cursor *TokenDecoder, members MemberPolicy) (Document, error) {
	var zero Document
	document, err := DecodeProjectionAt[Document](ctx, cursor, members)
	if err != nil {
		return zero, err
	}
	if err := document.Validate(); err != nil {
		return zero, errors.Join(core.ErrJSONContract, err)
	}
	return document, nil
}

// Layout names native formatting, never product-controlled JSON grammar.
type Layout uint8

const (
	LayoutCompact Layout = iota
	LayoutTwoSpaces
	LayoutTabs
)

func (l Layout) Validate() error {
	if l > LayoutTabs {
		return core.ErrJSONContract
	}
	return nil
}
func (l Layout) IsValid() bool { return l.Validate() == nil }
func (Layout) OffWireEnum()    {}
func (l Layout) String() string {
	switch l {
	case LayoutCompact:
		return "compact"
	case LayoutTwoSpaces:
		return "two spaces"
	case LayoutTabs:
		return "tabs"
	default:
		return core.UnknownEnumDiagnostic
	}
}

// WriteProjection streams one concrete projection with native formatting and
// no top-level newline. The caller owns meaning and the destination's lifetime.
// A late refusal leaves provisional bytes; no complete-value copy is retained.
func WriteProjection[Document any](ctx context.Context, destination io.Writer, document Document, policy OwnedCodecPolicy, layout Layout) error {
	if err := errors.Join(projectionContextError(ctx), validateOwnedCodecType[Document](policy), layout.Validate()); err != nil {
		return err
	}
	if core.WriterIsNil(destination) {
		return core.ErrJSONContract
	}
	if value := reflect.ValueOf(document); value.Kind() == reflect.Pointer && value.IsNil() {
		return core.ErrJSONContract
	}
	writer := ownedCodecDestination{ctx: ctx, destination: destination}
	options := []jsonv2.Options{jsonv2.Deterministic(true), jsonv2.WithMarshalers(ownedMarshalGuards(ctx))}
	switch layout {
	case LayoutTwoSpaces:
		options = append(options, jsontext.WithIndent("  "))
	case LayoutTabs:
		options = append(options, jsontext.WithIndent("\t"))
	}
	return errors.Join(objectReadOutcome(ctx, jsonv2.MarshalWrite(writer, document, options...)), projectionContextError(ctx))
}

// Contextstate owns admission of hostile Err implementations. Observe an
// optional native cancellation cause only after its standard terminal state;
// a hostile Value method cannot replace that already observed identity.
func projectionContextError(ctx context.Context) (err error) {
	err = contextstate.Validate(ctx)
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	defer func() { _ = recover() }()
	return errors.Join(err, context.Cause(ctx))
}
