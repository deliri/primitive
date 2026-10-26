package jsonio

import (
	"context"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/core"
)

// RootNullPolicy controls native null-to-zero decoding of concrete wire
// projections. Product owners still validate meaning before receiver commit.
type RootNullPolicy uint8

const (
	RootNullRefused RootNullPolicy = iota
	RootNullNativeZero
)

func (p RootNullPolicy) Validate() error {
	if p > RootNullNativeZero {
		return core.ErrJSONContract
	}
	return nil
}
func (p RootNullPolicy) IsValid() bool { return p.Validate() == nil }
func (RootNullPolicy) OffWireEnum()    {}
func (p RootNullPolicy) String() string {
	switch p {
	case RootNullRefused:
		return "root null refused"
	case RootNullNativeZero:
		return "native root null zero"
	default:
		return core.UnknownEnumDiagnostic
	}
}

// ProjectionReadPolicy selects Go's structural wire admission. Unlike owned
// validated codecs, projections leave nested null semantics to the declared
// field codecs and their owner. Opaque root methods need explicit admission.
type ProjectionReadPolicy struct {
	Members   MemberPolicy
	Root      RootCodecPolicy
	RootNulls RootNullPolicy
}

func (p ProjectionReadPolicy) Validate() error {
	return errors.Join(p.Members.Validate(), p.Root.Validate(), p.RootNulls.Validate())
}

// DecodeValueAt decodes one concrete projection at the existing native cursor.
// Go owns grammar, UTF-8, duplicate names and its native nesting limit. Declared
// fields stream directly; an explicitly admitted opaque byte codec owns its
// one-value allocation. No surrounding sequence or extra raw copy is retained.
// Source: https://pkg.go.dev/encoding/json/v2#UnmarshalDecode
func DecodeValueAt[Document any](ctx context.Context, cursor *TokenDecoder, policy ProjectionReadPolicy) (Document, error) {
	var zero Document
	if err := errors.Join(projectionContextError(ctx), policy.Validate(), validateOwnedCodecType[Document](OwnedCodecPolicy{Members: policy.Members, Root: policy.Root})); err != nil {
		return zero, err
	}
	if err := cursor.prepare(ctx); err != nil {
		return zero, err
	}
	defer func() { cursor.source.ctx = nil }()
	kind := cursor.decoder.PeekKind()
	if kind == jsontext.KindInvalid {
		_, err := cursor.decoder.ReadToken()
		if err == nil {
			err = core.ErrJSONContract
		}
		return zero, cursor.source.observedError(objectReadOutcome(ctx, err))
	}
	if err := cursor.source.observedError(nil); err != nil {
		return zero, err
	}
	if kind == jsontext.KindNull && policy.RootNulls == RootNullRefused {
		return zero, core.ErrJSONContract
	}
	if policy.Root == RootDeclaredFields && kind != jsontext.KindBeginObject && kind != jsontext.KindNull {
		return zero, core.ErrJSONContract
	}
	guards := jsonv2.UnmarshalFromFunc[any](func(_ *jsontext.Decoder, _ any) error {
		if err := projectionContextError(ctx); err != nil {
			return &nativeJSONError{cause: err}
		}
		return errors.ErrUnsupported
	})
	var document Document
	err := jsonv2.UnmarshalDecode(cursor.decoder, &document, jsonv2.RejectUnknownMembers(policy.Members == MembersDeclaredOnly), jsonv2.WithUnmarshalers(guards))
	if err := errors.Join(objectReadOutcome(ctx, err), projectionContextError(ctx), cursor.source.refusal); err != nil {
		return zero, err
	}
	return document, nil
}

// DecodeValue is the borrowed-byte equivalent of projection reader admission.
// Native grammar is checked before any explicitly admitted opaque root codec.
// Product meaning and receiver commit remain with the caller; refusal is zero.
func DecodeValue[Document any](data []byte, policy ProjectionReadPolicy) (Document, error) {
	var zero Document
	if err := errors.Join(policy.Validate(), validateOwnedCodecType[Document](OwnedCodecPolicy{Members: policy.Members, Root: policy.Root})); err != nil {
		return zero, err
	}
	if err := ValidateBytes(data); err != nil {
		return zero, err
	}
	kind := jsontext.Value(data).Kind()
	if kind == jsontext.KindNull && policy.RootNulls == RootNullRefused {
		return zero, core.ErrJSONContract
	}
	if policy.Root == RootDeclaredFields && kind != jsontext.KindBeginObject && kind != jsontext.KindNull {
		return zero, core.ErrJSONContract
	}
	var document Document
	if err := jsonv2.Unmarshal(data, &document, jsonv2.RejectUnknownMembers(policy.Members == MembersDeclaredOnly)); err != nil {
		return zero, jsonOperationError(err)
	}
	return document, nil
}

// DecodeProjectionReader publishes exactly one structurally admitted wire
// value only after clean EOF. A trailing value or late source refusal withdraws
// the candidate. The caller lends context and interruptible source lifetime.
func DecodeProjectionReader[Document any](ctx context.Context, source io.Reader, policy ProjectionReadPolicy) (Document, error) {
	var zero Document
	if err := errors.Join(projectionContextError(ctx), policy.Validate()); err != nil {
		return zero, err
	}
	cursor, err := NewTokenDecoder(TokenRequest{Source: source, NestingDepthMaximum: core.JSONNestingDepthMaximum})
	if err != nil {
		return zero, err
	}
	document, err := DecodeValueAt[Document](ctx, cursor, policy)
	if err != nil {
		return zero, err
	}
	if err := cursor.RequireEOF(ctx); err != nil {
		return zero, err
	}
	return document, nil
}

// IsSemanticError observes Go's native typed-decode failure without matching
// diagnostics. Native error details remain available through errors.As.
func IsSemanticError(err error) bool { _, ok := errors.AsType[*jsonv2.SemanticError](err); return ok }

// ValidateBytes admits exactly one complete native value without canonicalizing
// or copying its bytes. Go owns lexical checks and the native token cursor.
func ValidateBytes(data []byte) error {
	depthErr := checkCanonicalDepth(data, CanonicalPolicy{NestingDepthMaximum: core.JSONNestingDepthMaximum})
	return errors.Join(depthErr, checkSingleValue(data))
}

var _ core.Validatable = ProjectionReadPolicy{}
