package jsonio

import (
	"context"
	"encoding/json/jsontext"
	"errors"
	"io"
	"iter"
	"strings"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// TokenKind is a closed lexical contract. Go owns grammar and parser state.
type TokenKind uint8

const (
	TokenUnknown TokenKind = iota
	TokenObjectStart
	TokenObjectEnd
	TokenArrayStart
	TokenArrayEnd
	TokenString
	TokenNumber
	TokenTrue
	TokenFalse
	TokenNull
	tokenKindLimit
)

func (k TokenKind) Validate() error {
	if k <= TokenUnknown || k >= tokenKindLimit {
		return core.ErrJSONContract
	}
	return nil
}

func (k TokenKind) IsValid() bool { return k.Validate() == nil }

func (TokenKind) OffWireEnum() {}

func (k TokenKind) String() string {
	if !k.IsValid() {
		return "unknown"
	}
	return [...]string{"unknown", "object start", "object end", "array start", "array end", "string", "number", "true", "false", "null"}[k]
}

// Token carries one exact decoded string or number. Delimiters and literals
// have empty Text. Zero is invalid. No complete document or array is retained.
type Token struct {
	Kind TokenKind
	Text string
}

func (t Token) Validate() error {
	if err := t.Kind.Validate(); err != nil {
		return err
	}
	if t.Kind == TokenString {
		if !utf8.ValidString(t.Text) {
			return core.ErrJSONContract
		}
		return nil
	}
	if t.Kind == TokenNumber {
		return validateNumberToken(t.Text)
	}
	if t.Text != "" {
		return core.ErrJSONContract
	}
	return nil
}

func validateNumberToken(text string) error {
	decoder := jsontext.NewDecoder(strings.NewReader(text))
	token, err := decoder.ReadToken()
	if err != nil || token.Kind() != jsontext.KindNumber || token.String() != text {
		return errors.Join(core.ErrJSONContract, err)
	}
	if _, err := decoder.ReadToken(); err != io.EOF {
		return errors.Join(core.ErrJSONContract, err)
	}
	return nil
}

// TokenRequest declares source lifetime and open-container depth. It imposes
// no document, array-item or total-transfer ceiling. Go retains parser state
// and its buffer for the largest scalar token; callers must not retain tokens
// to reconstruct an aggregate. Duplicate-name detection remains owned by Go.
type TokenRequest struct {
	Source              io.Reader
	NestingDepthMaximum uint16
}

func (r TokenRequest) Validate() error {
	if core.ReaderIsNil(r.Source) || r.NestingDepthMaximum == 0 || r.NestingDepthMaximum > core.JSONNestingDepthMaximum {
		return core.ErrJSONContract
	}
	return nil
}

// Tokens publishes synchronous typed observations of Go's decoder. A stop
// prevents further reads; cancellation and parser failure emit a zero token
// with preserved identities. A prefix is provisional until clean EOF.
func Tokens(ctx context.Context, request TokenRequest) iter.Seq2[Token, error] {
	return func(yield func(Token, error) bool) {
		if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
			yield(Token{}, err)
			return
		}
		decoder := jsontext.NewDecoder(request.Source)
		for {
			token, err := readToken(ctx, decoder, request.NestingDepthMaximum)
			if err == io.EOF {
				return
			}
			if err != nil {
				yield(Token{}, err)
				return
			}
			if !yield(token, nil) {
				return
			}
		}
	}
}

func readToken(ctx context.Context, decoder *jsontext.Decoder, maximumDepth uint16) (Token, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return Token{}, err
	}
	observed, err := decoder.ReadToken()
	if canceled := contextstate.Validate(ctx); canceled != nil {
		return Token{}, errors.Join(canceled, err)
	}
	if err == io.EOF {
		return Token{}, io.EOF
	}
	if err != nil {
		return Token{}, errors.Join(core.ErrJSONContract, err)
	}
	if decoder.StackDepth() > int(maximumDepth) {
		return Token{}, core.ErrJSONContract
	}
	return projectToken(observed)
}

func projectToken(observed jsontext.Token) (Token, error) {
	var kind TokenKind
	switch observed.Kind() {
	case jsontext.KindBeginObject:
		kind = TokenObjectStart
	case jsontext.KindEndObject:
		kind = TokenObjectEnd
	case jsontext.KindBeginArray:
		kind = TokenArrayStart
	case jsontext.KindEndArray:
		kind = TokenArrayEnd
	case jsontext.KindString:
		kind = TokenString
	case jsontext.KindNumber:
		kind = TokenNumber
	case jsontext.KindTrue:
		kind = TokenTrue
	case jsontext.KindFalse:
		kind = TokenFalse
	case jsontext.KindNull:
		kind = TokenNull
	default:
		return Token{}, core.ErrJSONContract
	}
	return Token{Kind: kind, Text: projectTokenText(kind, observed)}, nil
}

func projectTokenText(kind TokenKind, observed jsontext.Token) string {
	if kind == TokenString || kind == TokenNumber {
		return observed.String()
	}
	return ""
}

var (
	_ core.Validatable = Token{}
	_ core.Validatable = TokenKind(0)
	_ core.Validatable = TokenRequest{}
)
