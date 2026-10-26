package jsonio

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"io"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// TokenDecoder lends synchronous access to Go's JSON cursor. Go owns grammar,
// buffers and duplicate-name checking. Callers must not use it concurrently.
// The source owner supplies any interrupt needed for an already blocked Read.
type TokenDecoder struct {
	decoder      *jsontext.Decoder
	source       *contextJSONSource
	maximumDepth uint16
}

func NewTokenDecoder(request TokenRequest) (*TokenDecoder, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	source := &contextJSONSource{source: checkedJSONSource{source: request.Source}}
	return &TokenDecoder{decoder: jsontext.NewDecoder(source), source: source, maximumDepth: request.NestingDepthMaximum}, nil
}

func (d *TokenDecoder) Validate() error {
	if d == nil || d.decoder == nil || d.source == nil {
		return core.ErrJSONContract
	}
	return (TokenRequest{Source: d.source.source.source, NestingDepthMaximum: d.maximumDepth}).Validate()
}

func (d *TokenDecoder) prepare(ctx context.Context) error {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	if err := d.Validate(); err != nil {
		return err
	}
	if d.source.refusal != nil {
		return d.source.refusal
	}
	d.source.ctx = ctx
	return nil
}

// Decode returns one token whose text is independent of later cursor calls.
func (d *TokenDecoder) Decode(ctx context.Context) (Token, error) {
	if err := d.prepare(ctx); err != nil {
		return Token{}, err
	}
	defer func() { d.source.ctx = nil }()
	token, err := readToken(ctx, d.decoder, d.maximumDepth)
	if err := d.source.observedError(err); err != nil {
		return Token{}, err
	}
	return token, nil
}

// SkipValue consumes exactly one complete value using Go's decoder. Containers
// are traversed token by token; Go still buffers individual scalar values.
// No value bytes or collection are published or retained by Primitive.
// Source: https://pkg.go.dev/encoding/json/jsontext#Decoder.SkipValue
func (d *TokenDecoder) SkipValue(ctx context.Context) error {
	if err := d.prepare(ctx); err != nil {
		return err
	}
	defer func() { d.source.ctx = nil }()
	depth := d.decoder.StackDepth()
	kind := d.decoder.PeekKind()
	if kind != jsontext.KindBeginObject && kind != jsontext.KindBeginArray {
		return d.source.observedError(objectReadOutcome(ctx, d.decoder.SkipValue()))
	}
	for {
		_, err := d.decoder.ReadToken()
		if err := d.source.observedError(objectReadOutcome(ctx, err)); err != nil {
			return err
		}
		if d.decoder.StackDepth() > int(d.maximumDepth) {
			return core.ErrJSONContract
		}
		if d.decoder.StackDepth() == depth {
			return nil
		}
	}
}

// ReadValue explicitly captures one whole value. The returned bytes are owned
// by the caller and survive subsequent cursor reads. Memory follows that one
// requested value; use SkipValue or Decode when retention is unnecessary.
// Source: https://pkg.go.dev/encoding/json/jsontext#Decoder.ReadValue
func (d *TokenDecoder) ReadValue(ctx context.Context) ([]byte, error) {
	if err := d.prepare(ctx); err != nil {
		return nil, err
	}
	defer func() { d.source.ctx = nil }()
	depth := d.decoder.StackDepth()
	value, err := d.decoder.ReadValue()
	if err := d.source.observedError(objectReadOutcome(ctx, err)); err != nil {
		return nil, err
	}
	if err := validateValueDepth(ctx, value, int(d.maximumDepth)-depth); err != nil {
		return nil, err
	}
	return append([]byte(nil), value...), nil
}

func validateValueDepth(ctx context.Context, value []byte, maximum int) error {
	// Go already admitted value's grammar. Every open container consumes one
	// '{' or '[' byte, so when even the count of those bytes (including any
	// inside strings) is within maximum, no nesting can exceed it and a second
	// native traversal would observe nothing new.
	if bytes.Count(value, []byte{'{'})+bytes.Count(value, []byte{'['}) <= maximum {
		return contextstate.Validate(ctx)
	}
	decoder := borrowedBytesDecoder(value)
	for {
		_, err := decoder.ReadToken()
		if err == io.EOF {
			return contextstate.Validate(ctx)
		}
		if err := objectReadOutcome(ctx, err); err != nil {
			return err
		}
		if decoder.StackDepth() > maximum {
			return core.ErrJSONContract
		}
	}
}

var _ core.Validatable = (*TokenDecoder)(nil)
