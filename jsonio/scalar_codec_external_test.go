package jsonio_test

import (
	"bytes"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
)

func TestCompactScalarCodecLayerTriadPreservesNativeBytes(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name  string
		token jsonio.Token
		want  string
	}{
		{"neutral empty text", jsonio.Token{Kind: jsonio.TokenString}, `""`},
		{"quote and slash text", jsonio.Token{Kind: jsonio.TokenString, Text: "\"\\"}, `"\"\\"`},
		{"line and control text", jsonio.Token{Kind: jsonio.TokenString, Text: "a\n\x00b"}, `"a\n\u0000b"`},
		{"unicode text", jsonio.Token{Kind: jsonio.TokenString, Text: "東京🙂"}, `"東京🙂"`},
		{"HTML data native quoting", jsonio.Token{Kind: jsonio.TokenString, Text: "<>&"}, `"<>&"`},
		{"exact minimum signed number", jsonio.Token{Kind: jsonio.TokenNumber, Text: "-9223372036854775808"}, "-9223372036854775808"},
		{"exact maximum unsigned number", jsonio.Token{Kind: jsonio.TokenNumber, Text: "18446744073709551615"}, "18446744073709551615"},
		{"number beyond float range", jsonio.Token{Kind: jsonio.TokenNumber, Text: "1e400"}, "1e400"},
		{"negative zero spelling", jsonio.Token{Kind: jsonio.TokenNumber, Text: "-0"}, "-0"},
		{"true scalar", jsonio.Token{Kind: jsonio.TokenTrue}, "true"},
		{"false neutral scalar", jsonio.Token{Kind: jsonio.TokenFalse}, "false"},
		{"explicit null scalar", jsonio.Token{Kind: jsonio.TokenNull}, "null"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			got, err := jsonio.MarshalScalar(row.token)
			decoded, decodeErr := jsonio.DecodeScalar(got)
			if err != nil || decodeErr != nil || string(got) != row.want || decoded != row.token || bytes.HasSuffix(got, []byte{'\n'}) {
				t.Fatalf("compact scalar=%q/%v decoded %v/%v, want exact %q/%v withoutLF", got, err, decoded, decodeErr, row.want, row.token)
			}
		})
	}
	for _, raw := range []string{"", " ", "{}", "[]", "[1]", "true false", `"text"0`, "01", "+1", "NaN", "1e", `"unterminated`, `"\ud800"`, "\xff", "nullx"} {
		t.Run("refused_"+fmt.Sprintf("%q", raw), func(t *testing.T) {
			t.Parallel()
			got, err := jsonio.DecodeScalar([]byte(raw))
			if got != (jsonio.Token{}) || !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("scalar refusal(%q)=%v/%v, want zero/contract", raw, got, err)
			}
		})
	}
	for _, token := range []jsonio.Token{{}, {Kind: jsonio.TokenObjectStart}, {Kind: jsonio.TokenObjectEnd}, {Kind: jsonio.TokenArrayStart}, {Kind: jsonio.TokenArrayEnd}, {Kind: jsonio.TokenString, Text: "\xff"}, {Kind: jsonio.TokenNumber, Text: "01"}, {Kind: jsonio.TokenTrue, Text: "true"}, {Kind: jsonio.TokenKind(255)}} {
		got, err := jsonio.MarshalScalar(token)
		if got != nil || !errors.Is(err, core.ErrJSONContract) {
			t.Fatalf("scalar encode refusal=%q/%v, want nil/contract", got, err)
		}
	}
}

func TestCompactTypedScalarCodecsPreserveFullIntegerRange(t *testing.T) {
	t.Parallel()
	for _, value := range []int64{math.MinInt64, math.MinInt64 + 1, -1, 0, 1, math.MaxInt64 - 1, math.MaxInt64} {
		data, err := jsonio.MarshalInteger(value)
		got, decodeErr := jsonio.DecodeInteger(data)
		native, nativeErr := jsonv2.Marshal(value)
		if err != nil || decodeErr != nil || nativeErr != nil || got != value || !bytes.Equal(data, native) {
			t.Fatalf("signed scalar=%q/%d errors %v/%v/%v, want exact %d/native", data, got, err, decodeErr, nativeErr, value)
		}
	}
	for _, value := range []uint64{0, 1, math.MaxInt64, math.MaxInt64 + 1, math.MaxUint64 - 1, math.MaxUint64} {
		data, err := jsonio.MarshalUnsignedInteger(value)
		got, decodeErr := jsonio.DecodeUnsignedInteger(data)
		if err != nil || decodeErr != nil || got != value {
			t.Fatalf("unsigned scalar=%q/%d errors %v/%v, want exact %d", data, got, err, decodeErr, value)
		}
	}
	for _, value := range []bool{false, true} {
		data, err := jsonio.MarshalBoolean(value)
		got, decodeErr := jsonio.DecodeBoolean(data)
		if err != nil || decodeErr != nil || got != value {
			t.Fatalf("boolean scalar=%q/%t errors %v/%v, want exact %t", data, got, err, decodeErr, value)
		}
	}
	for _, text := range []string{"", "ordinary", "\"\\\n", "東京🙂"} {
		data, err := jsonio.MarshalText(text)
		got, decodeErr := jsonio.DecodeText(data)
		if err != nil || decodeErr != nil || got != text {
			t.Fatalf("text scalar=%q/%q errors %v/%v, want exact %q", data, got, err, decodeErr, text)
		}
	}
	for _, raw := range []string{"null", "true", `"1"`, "[]", "{}", "1 2", "9223372036854775808", "-9223372036854775809", "1.5"} {
		got, err := jsonio.DecodeInteger([]byte(raw))
		if got != 0 || !errors.Is(err, core.ErrJSONContract) {
			t.Fatalf("integer refusal(%q)=%d/%v, want zero/contract", raw, got, err)
		}
	}
	for _, raw := range []string{"null", "true", `"1"`, "[]", "{}", "-1", "18446744073709551616"} {
		got, err := jsonio.DecodeUnsignedInteger([]byte(raw))
		if got != 0 || !errors.Is(err, core.ErrJSONContract) {
			t.Fatalf("unsigned refusal(%q)=%d/%v, want zero/contract", raw, got, err)
		}
	}
	for _, raw := range []string{"null", "true", "1", "[]", "{}"} {
		got, err := jsonio.DecodeText([]byte(raw))
		if got != "" || !errors.Is(err, core.ErrJSONContract) {
			t.Fatalf("text refusal(%q)=%q/%v, want zero/contract", raw, got, err)
		}
	}
	for _, raw := range []string{"null", `"true"`, "1", "[]", "{}"} {
		got, err := jsonio.DecodeBoolean([]byte(raw))
		if got || !errors.Is(err, core.ErrJSONContract) {
			t.Fatalf("boolean refusal(%q)=%t/%v, want zero/contract", raw, got, err)
		}
	}
}

func TestCompactTextWholeValueOwnershipContinuesBeyondNativeWindows(t *testing.T) {
	t.Parallel()
	for _, extent := range []int{0, 32767, 32768, 32769, 262151, 4194305} {
		t.Run(fmt.Sprintf("owned_text_%d_bytes", extent), func(t *testing.T) {
			t.Parallel()
			text := strings.Repeat("x", extent)
			data, err := jsonio.MarshalText(text)
			got, decodeErr := jsonio.DecodeText(data)
			if err != nil || decodeErr != nil || got != text || len(data) != extent+2 {
				t.Fatalf("owned text extent=%d/%d errors %v/%v, want exact %d bytes plus quotes", len(got), len(data), err, decodeErr, extent)
			}
		})
	}
}

func FuzzCompactScalarCodecSemanticClosure(f *testing.F) {
	for _, token := range []jsonio.Token{{Kind: jsonio.TokenString, Text: "東京🙂"}, {Kind: jsonio.TokenNumber, Text: "18446744073709551615"}, {Kind: jsonio.TokenNumber, Text: "1e400"}, {Kind: jsonio.TokenFalse}, {Kind: jsonio.TokenNull}} {
		if err := token.Validate(); err != nil {
			f.Fatal(err)
		}
		data, err := jsonio.MarshalScalar(token)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	f.Add([]byte{})
	f.Add([]byte("true false"))
	f.Add([]byte(`{"duplicate":1,"duplicate":2}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := jsonio.DecodeScalar(data)
		value := jsontext.Value(data)
		kind := value.Kind()
		accepted := value.IsValid() && (kind == jsontext.KindString || kind == jsontext.KindNumber || kind == jsontext.KindTrue || kind == jsontext.KindFalse || kind == jsontext.KindNull)
		if !accepted {
			if got != (jsonio.Token{}) || !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("scalar refusal=%v/%v, want zero/contract", got, err)
			}
			return
		}
		canonical, encodeErr := jsonio.MarshalScalar(got)
		again, decodeErr := jsonio.DecodeScalar(canonical)
		second, secondErr := jsonio.MarshalScalar(again)
		if err != nil || got.Validate() != nil || encodeErr != nil || decodeErr != nil || secondErr != nil || again != got || !bytes.Equal(canonical, second) || bytes.HasSuffix(canonical, []byte{'\n'}) {
			t.Fatalf("scalar closure=%v errors %v/%v/%v/%v, want exact compact semantic closure", got, err, encodeErr, decodeErr, secondErr)
		}
	})
}

func FuzzCompactTypedScalarNativeParity(f *testing.F) {
	for _, value := range []int64{math.MinInt64, 0, math.MaxInt64} {
		data, err := jsonio.MarshalInteger(value)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	text, err := jsonio.MarshalText("東京🙂")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(text)
	boolean, err := jsonio.MarshalBoolean(true)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(boolean)
	f.Add([]byte("null"))
	f.Fuzz(func(t *testing.T, data []byte) {
		token, scalarErr := jsonio.DecodeScalar(data)
		text, textErr := jsonio.DecodeText(data)
		boolean, boolErr := jsonio.DecodeBoolean(data)
		signed, signedErr := jsonio.DecodeInteger(data)
		unsigned, unsignedErr := jsonio.DecodeUnsignedInteger(data)
		var wantText string
		var wantBool bool
		var wantSigned int64
		var wantUnsigned uint64
		nativeTextErr := jsonv2.Unmarshal(data, &wantText)
		nativeBoolErr := jsonv2.Unmarshal(data, &wantBool)
		nativeSignedErr := jsonv2.Unmarshal(data, &wantSigned)
		nativeUnsignedErr := jsonv2.Unmarshal(data, &wantUnsigned)
		textAccepted := scalarErr == nil && token.Kind == jsonio.TokenString && nativeTextErr == nil
		boolAccepted := scalarErr == nil && (token.Kind == jsonio.TokenTrue || token.Kind == jsonio.TokenFalse) && nativeBoolErr == nil
		signedAccepted := scalarErr == nil && token.Kind == jsonio.TokenNumber && nativeSignedErr == nil
		unsignedAccepted := scalarErr == nil && token.Kind == jsonio.TokenNumber && nativeUnsignedErr == nil
		if textAccepted {
			if textErr != nil || text != wantText {
				t.Fatalf("text=%q/%v, want %q", text, textErr, wantText)
			}
		} else if text != "" || !errors.Is(textErr, core.ErrJSONContract) {
			t.Fatalf("text refusal=%q/%v", text, textErr)
		}
		if boolAccepted {
			if boolErr != nil || boolean != wantBool {
				t.Fatalf("bool=%t/%v, want %t", boolean, boolErr, wantBool)
			}
		} else if boolean || !errors.Is(boolErr, core.ErrJSONContract) {
			t.Fatalf("bool refusal=%t/%v", boolean, boolErr)
		}
		if signedAccepted {
			if signedErr != nil || signed != wantSigned {
				t.Fatalf("signed=%d/%v, want %d", signed, signedErr, wantSigned)
			}
		} else if signed != 0 || !errors.Is(signedErr, core.ErrJSONContract) {
			t.Fatalf("signed refusal=%d/%v", signed, signedErr)
		}
		if unsignedAccepted {
			if unsignedErr != nil || unsigned != wantUnsigned {
				t.Fatalf("unsigned=%d/%v, want %d", unsigned, unsignedErr, wantUnsigned)
			}
		} else if unsigned != 0 || !errors.Is(unsignedErr, core.ErrJSONContract) {
			t.Fatalf("unsigned refusal=%d/%v", unsigned, unsignedErr)
		}
	})
}
