package firestoredocuments

import (
	"bytes"
	"cmp"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
)

// The official SDK materializes a stored document as Go maps, slices and
// scalars. Primitive projects that native shape to JSON tokens and back with
// jsonio; no other JSON engine touches the document. The admitted native kinds
// are exactly the kinds Primitive's own JSON writers can produce plus the
// SDK's bytes: null, booleans, strings, INTEGER, DOUBLE (relaxed reads only),
// bytes, maps and arrays. Any other native kind refuses the whole projection.

// nativeToJSON writes one native value as one compact JSON value with
// UTF-16-ordered member names, exactly as the canonical whole-document
// encoder spells the same shape.
func nativeToJSON(ctx context.Context, native any, integerOnly bool) ([]byte, error) {
	var buffer bytes.Buffer
	err := jsonio.WriteTokenProjection(ctx, jsonio.TokenProjectionRequest{Destination: &buffer, Use: func(encoder *jsonio.TokenEncoder) error {
		return writeNativeValue(ctx, encoder, native, integerOnly)
	}})
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func writeNativeValue(ctx context.Context, encoder *jsonio.TokenEncoder, native any, integerOnly bool) error {
	switch value := native.(type) {
	case nil:
		return encoder.Encode(ctx, jsonio.Token{Kind: jsonio.TokenNull})
	case bool:
		if value {
			return encoder.Encode(ctx, jsonio.Token{Kind: jsonio.TokenTrue})
		}
		return encoder.Encode(ctx, jsonio.Token{Kind: jsonio.TokenFalse})
	case string:
		return encoder.Encode(ctx, jsonio.Token{Kind: jsonio.TokenString, Text: value})
	case int64:
		return encoder.Encode(ctx, jsonio.Token{Kind: jsonio.TokenNumber, Text: strconv.FormatInt(value, 10)})
	case float64:
		if integerOnly {
			return core.ErrJSONContract
		}
		text, err := nativeDoubleText(value)
		if err != nil {
			return err
		}
		return encoder.Encode(ctx, jsonio.Token{Kind: jsonio.TokenNumber, Text: text})
	case []byte:
		return encoder.Encode(ctx, jsonio.Token{Kind: jsonio.TokenString, Text: base64.StdEncoding.EncodeToString(value)})
	case []any:
		if err := encoder.Encode(ctx, jsonio.Token{Kind: jsonio.TokenArrayStart}); err != nil {
			return err
		}
		for _, element := range value {
			if err := writeNativeValue(ctx, encoder, element, integerOnly); err != nil {
				return err
			}
		}
		return encoder.Encode(ctx, jsonio.Token{Kind: jsonio.TokenArrayEnd})
	case map[string]any:
		if err := encoder.Encode(ctx, jsonio.Token{Kind: jsonio.TokenObjectStart}); err != nil {
			return err
		}
		names := make([]string, 0, len(value))
		for name := range value {
			names = append(names, name)
		}
		slices.SortFunc(names, compareUTF16)
		for _, name := range names {
			if err := encoder.Encode(ctx, jsonio.Token{Kind: jsonio.TokenString, Text: name}); err != nil {
				return err
			}
			if err := writeNativeValue(ctx, encoder, value[name], integerOnly); err != nil {
				return err
			}
		}
		return encoder.Encode(ctx, jsonio.Token{Kind: jsonio.TokenObjectEnd})
	default:
		return core.ErrJSONContract
	}
}

// nativeDoubleText spells a finite DOUBLE the way Go's JSON encoder does: the
// shortest round-tripping digits, exponent form outside [1e-6, 1e21) with no
// zero-padded exponent. Non-finite values have no JSON number and refuse.
func nativeDoubleText(value float64) (string, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "", core.ErrJSONContract
	}
	magnitude := math.Abs(value)
	if magnitude != 0 && (magnitude < 1e-6 || magnitude >= 1e21) {
		text := strconv.FormatFloat(value, 'e', -1, 64)
		mantissa, exponent, _ := strings.Cut(text, "e")
		sign, digits := exponent[:1], strings.TrimLeft(exponent[1:], "0")
		if digits == "" {
			digits = "0"
		}
		return mantissa + "e" + sign + digits, nil
	}
	return strconv.FormatFloat(value, 'f', -1, 64), nil
}

// compareUTF16 orders member names by UTF-16 code units (RFC 8785), the
// order Go's deterministic encoder and jsonio's canonical writer use.
func compareUTF16(a, b string) int {
	for a != "" && b != "" {
		left, leftSize := utf8.DecodeRuneInString(a)
		right, rightSize := utf8.DecodeRuneInString(b)
		if left != right {
			return cmp.Compare(utf16Key(left), utf16Key(right))
		}
		a, b = a[leftSize:], b[rightSize:]
	}
	return cmp.Compare(len(a), len(b))
}

func utf16Key(r rune) rune {
	if r >= 0xE000 && r <= 0xFFFF {
		return r + unicode.MaxRune
	}
	return r
}

// jsonToNative reads one complete JSON value into the SDK's native shape with
// every number token as INTEGER. Fractions, exponents and overflow refuse; Go
// owns grammar, duplicate names and nesting through jsonio's cursor.
func jsonToNative(ctx context.Context, data []byte) (any, error) {
	cursor, err := jsonio.NewTokenDecoder(jsonio.TokenRequest{Source: bytes.NewReader(data), NestingDepthMaximum: core.JSONNestingDepthMaximum})
	if err != nil {
		return nil, err
	}
	first, err := cursor.Decode(ctx)
	if err != nil {
		return nil, jsonDecodeRefusal(err)
	}
	value, err := readNativeValue(ctx, cursor, first)
	if err != nil {
		return nil, err
	}
	if _, err := cursor.Decode(ctx); err != io.EOF {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return value, nil
}

func jsonDecodeRefusal(err error) error {
	if err == io.EOF {
		return core.ErrJSONContract
	}
	return err
}

func readNativeValue(ctx context.Context, cursor *jsonio.TokenDecoder, token jsonio.Token) (any, error) {
	switch token.Kind {
	case jsonio.TokenNull:
		return nil, nil
	case jsonio.TokenTrue:
		return true, nil
	case jsonio.TokenFalse:
		return false, nil
	case jsonio.TokenString:
		return token.Text, nil
	case jsonio.TokenNumber:
		integer, err := strconv.ParseInt(token.Text, 10, 64)
		if err != nil {
			return nil, errors.Join(core.ErrJSONContract, err)
		}
		return integer, nil
	case jsonio.TokenArrayStart:
		elements := []any{}
		for {
			next, err := cursor.Decode(ctx)
			if err != nil {
				return nil, jsonDecodeRefusal(err)
			}
			if next.Kind == jsonio.TokenArrayEnd {
				return elements, nil
			}
			element, err := readNativeValue(ctx, cursor, next)
			if err != nil {
				return nil, err
			}
			elements = append(elements, element)
		}
	case jsonio.TokenObjectStart:
		members := map[string]any{}
		for {
			name, err := cursor.Decode(ctx)
			if err != nil {
				return nil, jsonDecodeRefusal(err)
			}
			if name.Kind == jsonio.TokenObjectEnd {
				return members, nil
			}
			if name.Kind != jsonio.TokenString {
				return nil, core.ErrJSONContract
			}
			next, err := cursor.Decode(ctx)
			if err != nil {
				return nil, jsonDecodeRefusal(err)
			}
			member, err := readNativeValue(ctx, cursor, next)
			if err != nil {
				return nil, err
			}
			members[name.Text] = member
		}
	default:
		return nil, core.ErrJSONContract
	}
}
