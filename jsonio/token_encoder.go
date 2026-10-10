package jsonio

import (
	"context"
	"encoding/json/jsontext"
	"errors"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// TokenEncoder writes one typed token at a time. Go owns JSON grammar,
// buffering, and duplicate-name detection. Memory follows open containers and
// the largest scalar, never array length. The destination owns interruptible
// writes. Failed output is provisional and must be discarded by its owner.
type TokenEncoder struct{ encoder *jsontext.Encoder }

func NewTokenEncoder(request ObjectDestinationRequest) (*TokenEncoder, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	return &TokenEncoder{encoder: jsontext.NewEncoder(checkedJSONDestination{destination: request.Destination})}, nil
}

func (e *TokenEncoder) Validate() error {
	if e == nil || e.encoder == nil {
		return core.ErrJSONContract
	}
	return nil
}

// Encode preserves exact number spellings and emits Go's top-level LF when a
// value completes. A successful token write does not seal an open container.
func (e *TokenEncoder) Encode(ctx context.Context, token Token) error {
	if err := errors.Join(contextstate.Validate(ctx), e.Validate(), token.Validate()); err != nil {
		return err
	}
	var err error
	if token.Kind == TokenNumber {
		// Validate admits exactly one numeric scalar. WriteValue leaves grammar
		// to Go without rounding a caller's integer or exponent through float64.
		err = e.encoder.WriteValue(jsontext.Value(token.Text))
	} else {
		err = e.encoder.WriteToken(encodedToken(token))
	}
	if err != nil {
		return errors.Join(core.ErrJSONContract, err, contextstate.Validate(ctx))
	}
	return contextstate.Validate(ctx)
}

func encodedToken(token Token) jsontext.Token {
	switch token.Kind {
	case TokenObjectStart:
		return jsontext.BeginObject
	case TokenObjectEnd:
		return jsontext.EndObject
	case TokenArrayStart:
		return jsontext.BeginArray
	case TokenArrayEnd:
		return jsontext.EndArray
	case TokenString:
		return jsontext.String(token.Text)
	case TokenTrue:
		return jsontext.True
	case TokenFalse:
		return jsontext.False
	case TokenNull:
		return jsontext.Null
	default:
		return jsontext.Token{}
	}
}

var _ core.Validatable = (*TokenEncoder)(nil)
