package jsonio_test

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/jsonio"
	"github.com/deliri/primitive/v2026/testserial"
)

func TestJSONTokenEncoderExactScalarAndContainerMatrix(t *testing.T) {
	t.Parallel()
	scalars := []struct {
		name  string
		token jsonio.Token
		wire  string
	}{
		{"empty string", jsonio.Token{Kind: jsonio.TokenString}, `""`},
		{"escaped string", jsonio.Token{Kind: jsonio.TokenString, Text: "quote\"\n\t\x00\\"}, ""},
		{"unicode string", jsonio.Token{Kind: jsonio.TokenString, Text: "雪😀<>&"}, ""},
		{"integer beyond float precision", jsonio.Token{Kind: jsonio.TokenNumber, Text: "18446744073709551616"}, "18446744073709551616"},
		{"exponent beyond float range", jsonio.Token{Kind: jsonio.TokenNumber, Text: "-1.2300e+400"}, "-1.2300e+400"},
		{"true", jsonio.Token{Kind: jsonio.TokenTrue}, "true"},
		{"false", jsonio.Token{Kind: jsonio.TokenFalse}, "false"},
		{"null", jsonio.Token{Kind: jsonio.TokenNull}, "null"},
	}
	for _, scalar := range scalars {
		if scalar.token.Kind == jsonio.TokenString {
			value, err := jsonv2.Marshal(scalar.token.Text)
			if err != nil {
				t.Fatal(err)
			}
			scalar.wire = string(value)
		}
		for layout := 0; layout < 8; layout++ {
			t.Run(fmt.Sprintf("%s/layout-%d", scalar.name, layout), func(t *testing.T) {
				t.Parallel()
				var out bytes.Buffer
				encoder, err := jsonio.NewTokenEncoder(jsonio.ObjectDestinationRequest{Destination: &out})
				if err != nil {
					t.Fatal(err)
				}
				key := jsonio.Token{Kind: jsonio.TokenString, Text: "value"}
				beginArray := jsonio.Token{Kind: jsonio.TokenArrayStart}
				endArray := jsonio.Token{Kind: jsonio.TokenArrayEnd}
				beginObject := jsonio.Token{Kind: jsonio.TokenObjectStart}
				endObject := jsonio.Token{Kind: jsonio.TokenObjectEnd}
				var tokens []jsonio.Token
				var want string
				switch layout {
				case 0:
					tokens = []jsonio.Token{scalar.token}
					want = scalar.wire + "\n"
				case 1:
					tokens = []jsonio.Token{beginArray, scalar.token, endArray}
					want = "[" + scalar.wire + "]\n"
				case 2:
					tokens = []jsonio.Token{beginObject, key, scalar.token, endObject}
					want = `{"value":` + scalar.wire + "}\n"
				case 3:
					tokens = []jsonio.Token{beginObject, key, beginArray, scalar.token, endArray, endObject}
					want = `{"value":[` + scalar.wire + "]}\n"
				case 4:
					tokens = []jsonio.Token{beginArray, beginArray, scalar.token, endArray, endArray}
					want = "[[" + scalar.wire + "]]\n"
				case 5:
					tokens = []jsonio.Token{beginArray, {Kind: jsonio.TokenNull}, scalar.token, {Kind: jsonio.TokenTrue}, endArray}
					want = "[null," + scalar.wire + ",true]\n"
				case 6:
					tokens = []jsonio.Token{beginObject, key, beginObject, key, scalar.token, endObject, endObject}
					want = `{"value":{"value":` + scalar.wire + "}}\n"
				case 7:
					tokens = []jsonio.Token{scalar.token, scalar.token}
					want = scalar.wire + "\n" + scalar.wire + "\n"
				}
				for _, token := range tokens {
					if err := encoder.Encode(t.Context(), token); err != nil {
						t.Fatal(err)
					}
				}
				if got := out.String(); got != want {
					t.Fatalf("wire=%q want=%q", got, want)
				}
			})
		}
	}
}

func TestJSONTokenEncoderRejectsInvalidTokenBeforeWriting(t *testing.T) {
	t.Parallel()
	for value := 0; value < 256; value++ {
		kind := jsonio.TokenKind(value)
		if kind.IsValid() {
			continue
		}
		t.Run(fmt.Sprint(value), func(t *testing.T) {
			t.Parallel()
			var out bytes.Buffer
			e, err := jsonio.NewTokenEncoder(jsonio.ObjectDestinationRequest{Destination: &out})
			if err != nil {
				t.Fatal(err)
			}
			if err := e.Encode(t.Context(), jsonio.Token{Kind: kind}); !errors.Is(err, core.ErrJSONContract) || out.Len() != 0 {
				t.Fatalf("error=%v output=%q", err, out.String())
			}
		})
	}
	for _, token := range []jsonio.Token{{Kind: jsonio.TokenNumber, Text: "01"}, {Kind: jsonio.TokenNumber, Text: "NaN"}, {Kind: jsonio.TokenNumber, Text: "1 2"}, {Kind: jsonio.TokenNumber, Text: "[]"}, {Kind: jsonio.TokenNumber, Text: " 1"}, {Kind: jsonio.TokenString, Text: "\xff"}, {Kind: jsonio.TokenTrue, Text: "true"}} {
		t.Run(fmt.Sprintf("%d/%q", token.Kind, token.Text), func(t *testing.T) {
			t.Parallel()
			var out bytes.Buffer
			e, err := jsonio.NewTokenEncoder(jsonio.ObjectDestinationRequest{Destination: &out})
			if err != nil {
				t.Fatal(err)
			}
			if err := e.Encode(t.Context(), token); !errors.Is(err, core.ErrJSONContract) || out.Len() != 0 {
				t.Fatalf("error=%v output=%q", err, out.String())
			}
		})
	}
}

type tokenRefusalWriter struct {
	count  int
	cause  error
	cancel context.CancelFunc
	calls  int
}

func (w *tokenRefusalWriter) Write(p []byte) (int, error) {
	w.calls++
	if w.cancel != nil {
		w.cancel()
	}
	switch w.count {
	case -1:
		return -1, w.cause
	case 1:
		return len(p) + 1, w.cause
	case 2:
		return len(p), w.cause
	default:
		return 0, w.cause
	}
}

func TestJSONTokenEncoderPreservesWriteAndCancellationCauses(t *testing.T) {
	t.Parallel()
	for _, count := range []int{-1, 0, 1, 2} {
		for _, cancel := range []bool{false, true} {
			t.Run(fmt.Sprintf("count-%d/cancel-%t", count, cancel), func(t *testing.T) {
				t.Parallel()
				ctx, stop := context.WithCancel(t.Context())
				defer stop()
				w := &tokenRefusalWriter{count: count, cause: io.ErrClosedPipe}
				if cancel {
					w.cancel = stop
				}
				e, err := jsonio.NewTokenEncoder(jsonio.ObjectDestinationRequest{Destination: w})
				if err != nil {
					t.Fatal(err)
				}
				err = e.Encode(ctx, jsonio.Token{Kind: jsonio.TokenString, Text: "value"})
				if !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, io.ErrClosedPipe) || w.calls != 1 {
					t.Fatalf("error=%v calls=%d", err, w.calls)
				}
				if cancel && !errors.Is(err, context.Canceled) {
					t.Fatalf("lost cancellation: %v", err)
				}
			})
		}
	}
	w := &tokenRefusalWriter{}
	e, err := jsonio.NewTokenEncoder(jsonio.ObjectDestinationRequest{Destination: w})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Encode(t.Context(), jsonio.Token{Kind: jsonio.TokenNull}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short write: %v", err)
	}
	var out bytes.Buffer
	e, err = jsonio.NewTokenEncoder(jsonio.ObjectDestinationRequest{Destination: &out})
	if err != nil {
		t.Fatal(err)
	}
	ctx, stop := context.WithCancel(t.Context())
	stop()
	if err := e.Encode(ctx, jsonio.Token{Kind: jsonio.TokenNull}); !errors.Is(err, context.Canceled) || out.Len() != 0 {
		t.Fatalf("pre-cancel: %v bytes=%d", err, out.Len())
	}
}

func TestJSONTokenEncoderRejectsGrammarAndNilCapabilities(t *testing.T) {
	t.Parallel()
	for _, request := range []jsonio.ObjectDestinationRequest{{}, {Destination: (*bytes.Buffer)(nil)}} {
		if e, err := jsonio.NewTokenEncoder(request); e != nil || !errors.Is(err, core.ErrJSONContract) {
			t.Fatalf("nil destination encoder=%v error=%v", e, err)
		}
	}
	for _, e := range []*jsonio.TokenEncoder{nil, {}} {
		if err := e.Encode(t.Context(), jsonio.Token{Kind: jsonio.TokenNull}); !errors.Is(err, core.ErrJSONContract) {
			t.Fatalf("nil encoder: %v", err)
		}
	}
	for _, tokens := range [][]jsonio.Token{
		{{Kind: jsonio.TokenArrayEnd}}, {{Kind: jsonio.TokenObjectEnd}},
		{{Kind: jsonio.TokenObjectStart}, {Kind: jsonio.TokenNumber, Text: "1"}},
		{{Kind: jsonio.TokenArrayStart}, {Kind: jsonio.TokenObjectEnd}},
		{{Kind: jsonio.TokenObjectStart}, {Kind: jsonio.TokenString, Text: "x"}, {Kind: jsonio.TokenObjectEnd}},
		{{Kind: jsonio.TokenObjectStart}, {Kind: jsonio.TokenString, Text: "x"}, {Kind: jsonio.TokenNull}, {Kind: jsonio.TokenString, Text: "x"}},
	} {
		e, err := jsonio.NewTokenEncoder(jsonio.ObjectDestinationRequest{Destination: io.Discard})
		if err != nil {
			t.Fatal(err)
		}
		for i, token := range tokens {
			err = e.Encode(t.Context(), token)
			if i < len(tokens)-1 && err != nil {
				t.Fatal(err)
			}
		}
		if !errors.Is(err, core.ErrJSONContract) {
			t.Fatalf("grammar accepted: %v", tokens)
		}
	}
}

func TestJSONTokenEncoderArrayMemoryIndependentOfItemCount(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, items := range []int{4096, 65536, 262144} {
		before, err := hostfacts.ObserveCollectedGoHeap(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		e, err := jsonio.NewTokenEncoder(jsonio.ObjectDestinationRequest{Destination: io.Discard})
		if err != nil {
			t.Fatal(err)
		}
		if err := e.Encode(t.Context(), jsonio.Token{Kind: jsonio.TokenArrayStart}); err != nil {
			t.Fatal(err)
		}
		for range items {
			if err := e.Encode(t.Context(), jsonio.Token{Kind: jsonio.TokenString, Text: "value"}); err != nil {
				t.Fatal(err)
			}
		}
		after, err := hostfacts.ObserveCollectedGoHeap(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		runtime.KeepAlive(e)
		retained := int64(after.Uint64()) - int64(before.Uint64())
		t.Logf("items=%d retained=%d", items, retained)
		if retained > 256<<10 {
			t.Fatalf("retained bytes=%d", retained)
		}
		if err := e.Encode(t.Context(), jsonio.Token{Kind: jsonio.TokenArrayEnd}); err != nil {
			t.Fatal(err)
		}
	}
}

func FuzzJSONTokenEncoderStringMatchesV2(f *testing.F) {
	for _, text := range []string{"", "\n\t\x00", "雪😀<>&", strings.Repeat("x", 65536)} {
		f.Add(text)
	}
	f.Fuzz(func(t *testing.T, text string) {
		var out bytes.Buffer
		e, err := jsonio.NewTokenEncoder(jsonio.ObjectDestinationRequest{Destination: &out})
		if err != nil {
			t.Fatal(err)
		}
		err = e.Encode(t.Context(), jsonio.Token{Kind: jsonio.TokenString, Text: text})
		if !utf8.ValidString(text) {
			if !errors.Is(err, core.ErrJSONContract) || out.Len() != 0 {
				t.Fatalf("invalid string: %v bytes=%d", err, out.Len())
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		want, err := jsonv2.Marshal(text)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(out.Bytes(), append(want, '\n')) {
			t.Fatalf("wire=%q want=%q", out.Bytes(), want)
		}
	})
}

func FuzzJSONTokenEncoderNumbersPreserveV2Scalar(f *testing.F) {
	for _, text := range []string{"0", "-0", "18446744073709551616", "1e400", "01", "1 2", "[]"} {
		f.Add(text)
	}
	f.Fuzz(func(t *testing.T, text string) {
		var out bytes.Buffer
		e, err := jsonio.NewTokenEncoder(jsonio.ObjectDestinationRequest{Destination: &out})
		if err != nil {
			t.Fatal(err)
		}
		err = e.Encode(t.Context(), jsonio.Token{Kind: jsonio.TokenNumber, Text: text})
		valid := len(text) > 0 && (text[0] == '-' || text[0] >= '0' && text[0] <= '9') && jsontext.Value(text).IsValid()
		var compact jsontext.Value = append(jsontext.Value(nil), text...)
		compactErr := compact.Compact()
		valid = valid && compactErr == nil && string(compact) == text
		if !valid {
			if !errors.Is(err, core.ErrJSONContract) || out.Len() != 0 {
				t.Fatalf("invalid number %q: %v bytes=%d", text, err, out.Len())
			}
			return
		}
		if err != nil || out.String() != text+"\n" {
			t.Fatalf("number %q: error=%v wire=%q", text, err, out.String())
		}
	})
}

type tokenCountingWriter struct{ bytes int }

func (w *tokenCountingWriter) Write(p []byte) (int, error) { w.bytes += len(p); return len(p), nil }

var tokenBenchmarkBytes int

func BenchmarkJSONTokenEncoderFlatArray(b *testing.B) {
	b.ReportAllocs()
	w := &tokenCountingWriter{}
	e, err := jsonio.NewTokenEncoder(jsonio.ObjectDestinationRequest{Destination: w})
	if err != nil {
		b.Fatal(err)
	}
	if err := e.Encode(b.Context(), jsonio.Token{Kind: jsonio.TokenArrayStart}); err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		if err := e.Encode(b.Context(), jsonio.Token{Kind: jsonio.TokenTrue}); err != nil {
			b.Fatal(err)
		}
	}
	if err := e.Encode(b.Context(), jsonio.Token{Kind: jsonio.TokenArrayEnd}); err != nil {
		b.Fatal(err)
	}
	tokenBenchmarkBytes = w.bytes
	if want := 5*b.N + 2; w.bytes != want {
		b.Fatalf("bytes=%d want=%d", w.bytes, want)
	}
}
