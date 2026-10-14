package jsonio_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json/jsontext"
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
)

func TestCanonicalizePreservesSignedIntegerAndUnicodeBytes(t *testing.T) {
	t.Parallel()
	input := []byte(` { "z":9007199254740993,"a":"\u0061\n","float":1e+00,"negative":-9223372036854775808 } `)
	want := []byte("{\"a\":\"a\\n\",\"float\":1,\"negative\":-9223372036854775808,\"z\":9007199254740993}")
	before := bytes.Clone(input)
	got, err := jsonio.Canonicalize(input, jsonio.CanonicalPolicy{NestingDepthMaximum: 64})
	if err != nil || !bytes.Equal(got, want) || !bytes.Equal(input, before) {
		t.Fatalf("canonical=%q err=%v source=%q", got, err, input)
	}
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, ed25519.SeedSize))
	signature := ed25519.Sign(private, want)
	if !ed25519.Verify(private.Public().(ed25519.PublicKey), got, signature) {
		t.Fatal("canonical bytes changed signed preimage")
	}
	got[0] = '['
	if ed25519.Verify(private.Public().(ed25519.PublicKey), got, signature) {
		t.Fatal("mutated preimage verified")
	}
	if !bytes.Equal(input, before) {
		t.Fatal("returned output aliases borrowed input")
	}
}

func TestCanonicalizeNativeRefusalAndDepthBoundaries(t *testing.T) {
	t.Parallel()
	for _, input := range [][]byte{nil, []byte(`{"x":1,"x":2}`), []byte(`{"x":`), []byte(`{} {}`), {'"', 0xff, '"'}} {
		got, err := jsonio.Canonicalize(input, jsonio.CanonicalPolicy{NestingDepthMaximum: 64})
		if got != nil || !errors.Is(err, core.ErrJSONContract) {
			t.Fatalf("input=%q output=%q refusal=%v", input, got, err)
		}
		if bytes.Contains(input, []byte(`"x":2`)) && !jsonio.IsDuplicateName(err) {
			t.Fatal("native duplicate-name identity lost")
		}
		if bytes.Contains(input, []byte{0xff}) && (!jsonio.IsInvalidUTF8(err) || !jsonio.IsSyntacticError(err)) {
			t.Fatalf("native UTF-8 cause lost: %v", err)
		}
	}
	for _, depth := range []int{63, 64, 65} {
		input := []byte(strings.Repeat("[", depth) + "0" + strings.Repeat("]", depth))
		got, err := jsonio.Canonicalize(input, jsonio.CanonicalPolicy{NestingDepthMaximum: 64})
		if depth <= 64 {
			if err != nil || !bytes.Equal(got, input) {
				t.Fatalf("depth=%d got=%q err=%v", depth, got, err)
			}
		} else {
			var observed *jsonio.DepthError
			if got != nil || !errors.As(err, &observed) || observed.Maximum != 64 || observed.Observed != 65 {
				t.Fatalf("depth refusal: %q %v", got, err)
			}
		}
	}
	for _, max := range []uint16{0, 65, 65535} {
		if got, err := jsonio.Canonicalize([]byte(`{}`), jsonio.CanonicalPolicy{NestingDepthMaximum: max}); got != nil || !errors.Is(err, core.ErrJSONContract) {
			t.Fatalf("invalid maximum=%d: %q %v", max, got, err)
		}
	}
}

type canonicalProjection struct {
	Z uint64 `json:"z"`
	A string `json:"a"`
}

func TestCanonicalProjectionStructuralAdmissionAndAtomicAppend(t *testing.T) {
	t.Parallel()
	prefix := []byte("prefix:")
	got, err := jsonio.AppendCanonicalProjection(prefix, canonicalProjection{Z: 9007199254740993, A: "a"}, jsonio.OwnedCodecPolicy{}, jsonio.CanonicalPolicy{NestingDepthMaximum: 64})
	if err != nil || string(got) != `prefix:{"a":"a","z":9007199254740993}` {
		t.Fatalf("projection=%q err=%v", got, err)
	}
	if got, err := jsonio.AppendCanonicalProjection(prefix, canonicalProjection{A: string([]byte{0xff})}, jsonio.OwnedCodecPolicy{}, jsonio.CanonicalPolicy{NestingDepthMaximum: 64}); got != nil || !errors.Is(err, core.ErrJSONContract) || string(prefix) != "prefix:" {
		t.Fatalf("failed append=%q err=%v prefix=%q", got, err, prefix)
	}
	for _, input := range []string{`{"z":1,"a":"ok","extra":2}`, `{"z":1,"a":4}`, `{"z":1,"z":2}`, `null`, `[]`, `{"z":1} {}`} {
		got, err := jsonio.DecodeProjection[canonicalProjection]([]byte(input), jsonio.MembersDeclaredOnly)
		if got != (canonicalProjection{}) || !errors.Is(err, core.ErrJSONContract) {
			t.Fatalf("input=%q value=%+v err=%v", input, got, err)
		}
	}
	gotValue, err := jsonio.DecodeProjection[canonicalProjection]([]byte(`{"z":9007199254740993,"a":"ok","extra":2}`), jsonio.MembersIgnoreExtra)
	if err != nil || gotValue.Z != 9007199254740993 || gotValue.A != "ok" {
		t.Fatalf("foreign admission=%+v %v", gotValue, err)
	}
}

func FuzzCanonicalizeNativeBytesAndSignature(f *testing.F) {
	for _, seed := range []string{`{"z":9007199254740993,"a":1e0}`, `{"x":1,"x":2}`, `[null,true,"\ud834\udd1e"]`, `{} {}`, `{"x":`} {
		f.Add([]byte(seed))
	}
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{3}, ed25519.SeedSize))
	f.Fuzz(func(t *testing.T, input []byte) {
		want := jsontext.Value(bytes.Clone(input))
		nativeErr := want.Canonicalize(jsontext.CanonicalizeRawInts(false))
		got, err := jsonio.Canonicalize(input, jsonio.CanonicalPolicy{NestingDepthMaximum: 64})
		if err != nil {
			if got != nil || !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("failed publication=%q %v", got, err)
			}
			var depth *jsonio.DepthError
			if nativeErr == nil && !errors.As(err, &depth) {
				t.Fatalf("valid native value refused: %q %v", input, err)
			}
			return
		}
		if nativeErr != nil || !bytes.Equal(got, want) {
			t.Fatalf("native=%q %v primitive=%q %v", want, nativeErr, got, err)
		}
		if !ed25519.Verify(private.Public().(ed25519.PublicKey), got, ed25519.Sign(private, want)) {
			t.Fatal("canonical signature mismatch")
		}
	})
}
