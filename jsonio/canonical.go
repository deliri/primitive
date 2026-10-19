package jsonio

import (
	"encoding/json/jsontext"
	"errors"
	"io"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

// CanonicalPolicy names the maximum open-container depth. Canonicalization
// follows Go's RFC 8785 implementation, preserving exact integer tokens rather
// than rounding them through IEEE-754. The caller owns the complete output;
// this operation makes no constant-memory claim for a whole value.
type CanonicalPolicy struct{ NestingDepthMaximum uint16 }

func (p CanonicalPolicy) Validate() error {
	if p.NestingDepthMaximum == 0 || p.NestingDepthMaximum > core.JSONNestingDepthMaximum {
		return core.ErrJSONContract
	}
	return nil
}

// DepthError preserves the observed native cursor depth and the request's
// admitted maximum. Its stable identity is Core's JSON contract constant.
type DepthError struct {
	Maximum  uint16
	Observed int
}

func (e *DepthError) Error() string { return "JSON nesting exceeds admitted depth" }
func (e *DepthError) Unwrap() error { return core.ErrJSONContract }

// Canonicalize borrows data and returns independent canonical bytes. Go owns
// grammar, UTF-8, duplicate names, member order, escaping and numeric spelling.
// Source: https://pkg.go.dev/encoding/json/jsontext#Value.Canonicalize
func Canonicalize(data []byte, policy CanonicalPolicy) ([]byte, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	if !utf8.Valid(data) {
		value := jsontext.Value(append([]byte(nil), data...))
		err := value.Canonicalize(jsontext.CanonicalizeRawInts(false))
		return nil, &invalidUTF8Error{cause: jsonOperationError(err)}
	}
	if err := checkCanonicalDepth(data, policy); err != nil {
		return nil, err
	}
	value := jsontext.Value(append([]byte(nil), data...))
	if err := value.Canonicalize(jsontext.CanonicalizeRawInts(false)); err != nil {
		return nil, jsonOperationError(err)
	}
	return []byte(value), nil
}

func checkCanonicalDepth(data []byte, policy CanonicalPolicy) error {
	decoder := borrowedBytesDecoder(data)
	for {
		_, err := decoder.ReadToken()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return jsonOperationError(err)
		}
		if decoder.StackDepth() > int(policy.NestingDepthMaximum) {
			return &DepthError{Maximum: policy.NestingDepthMaximum, Observed: decoder.StackDepth()}
		}
	}
}

// IsDuplicateName inspects Go's native identity without prose matching or a
// second parser. The native cause remains available through errors.Is/As.
func IsDuplicateName(err error) bool { return errors.Is(err, jsontext.ErrDuplicateName) }

type invalidUTF8Error struct{ cause error }

func (e *invalidUTF8Error) Error() string { return "invalid UTF-8 in JSON input" }
func (e *invalidUTF8Error) Unwrap() error { return e.cause }

func IsInvalidUTF8(err error) bool { _, ok := errors.AsType[*invalidUTF8Error](err); return ok }
func IsSyntacticError(err error) bool {
	_, ok := errors.AsType[*jsontext.SyntacticError](err)
	return ok
}

// AppendCanonicalProjection serializes a concrete caller-owned projection
// through Go and appends canonical bytes only after complete admission.
// Meaning is the caller's responsibility; use MarshalValidated when its owning
// type implements Validate. No untyped document is decoded or retained here.
func AppendCanonicalProjection[Document any](dst []byte, document Document, policy OwnedCodecPolicy, canonical CanonicalPolicy) ([]byte, error) {
	if err := canonical.Validate(); err != nil {
		return nil, err
	}
	data, err := MarshalProjection(document, policy)
	if err != nil {
		return nil, err
	}
	data, err = Canonicalize(data, canonical)
	if err != nil {
		return nil, err
	}
	return append(dst, data...), nil
}

var _ core.Validatable = CanonicalPolicy{}
