package jsonio_test

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"io"
	"reflect"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
)

type CanonicalDeclaredCounts struct {
	Z uint64 `json:"z"`
	A int64  `json:"a"`
}

type canonicalDeclaredDocument struct {
	Tail string                `json:"tail,omitempty"`
	Rows []canonicalProjection `json:"rows"`
	CanonicalDeclaredCounts
	Nested     canonicalProjection `json:"nested"`
	Astral     string              `json:"𐀀"`
	PrivateUse string              `json:"\ue000"`
}

func TestCanonicalDeclaredProjectionMatchesGoForEmbeddedFieldsAndUnicodeOrder(t *testing.T) {
	t.Parallel()
	document := canonicalDeclaredDocument{CanonicalDeclaredCounts: CanonicalDeclaredCounts{Z: 9007199254740993, A: -9223372036854775808}, Rows: []canonicalProjection{{Z: 8, A: "雪"}, {Z: 9, A: "\n\""}}, Nested: canonicalProjection{Z: 7, A: "<>&"}, Astral: "astral", PrivateUse: "private"}
	for _, tail := range []string{"", "tail"} {
		document.Tail = tail
		want, err := jsonv2.Marshal(document)
		if err != nil {
			t.Fatal(err)
		}
		canonical := jsontext.Value(want)
		if err := canonical.Canonicalize(jsontext.CanonicalizeRawInts(false)); err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer
		err = jsonio.WriteCanonicalProjection(t.Context(), &output, document, jsonio.OwnedCodecPolicy{Root: jsonio.RootOwnedMethods})
		if err != nil || !bytes.Equal(output.Bytes(), canonical) {
			t.Fatalf("declared stream = (%q, %v), want native canonical %q", output.Bytes(), err, canonical)
		}
	}
}

type canonicalProjectionOwner struct{ rows []canonicalProjection }

func (o canonicalProjectionOwner) MarshalJSON() ([]byte, error) { return nil, io.ErrClosedPipe }

type canonicalProjectionOwnerWire struct {
	Rows []canonicalProjection `json:"rows"`
}

func TestCanonicalDeclaredProjectionUsesTypedOwnerProjection(t *testing.T) {
	t.Parallel()
	owner := canonicalProjectionOwner{rows: []canonicalProjection{{Z: 9007199254740993, A: "owner"}}}
	codec, err := jsonio.ProjectCanonical(func(value canonicalProjectionOwner) (canonicalProjectionOwnerWire, error) {
		return canonicalProjectionOwnerWire{Rows: value.rows}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	err = jsonio.WriteCanonicalProjection(t.Context(), &output, owner, jsonio.OwnedCodecPolicy{Root: jsonio.RootOwnedMethods}, codec)
	want, nativeErr := jsonv2.Marshal(canonicalProjectionOwnerWire{Rows: owner.rows})
	if nativeErr != nil {
		t.Fatal(nativeErr)
	}
	canonical := jsontext.Value(want)
	if err := canonical.Canonicalize(jsontext.CanonicalizeRawInts(false)); err != nil {
		t.Fatal(err)
	}
	if err != nil || !bytes.Equal(output.Bytes(), canonical) {
		t.Fatalf("owned projection = (%q, %v), want native %q", output.Bytes(), err, canonical)
	}
	if _, err := jsonio.ProjectCanonical[canonicalProjectionOwner, canonicalProjectionOwnerWire](nil); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil projection error = %v", err)
	}
	if _, err := jsonio.ProjectCanonical(func(value canonicalProjectionOwner) (canonicalProjectionOwner, error) { return value, nil }); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("recursive owner projection error = %v", err)
	}
}

type canonicalDeclaredDepth struct {
	Z    int                     `json:"z"`
	Next *canonicalDeclaredDepth `json:"next,omitempty"`
}

func TestCanonicalDeclaredProjectionDepthFollowsNativeCursor(t *testing.T) {
	t.Parallel()
	for _, depth := range []int{63, 64, 65} {
		root := &canonicalDeclaredDepth{}
		cursor := root
		for i := 1; i < depth; i++ {
			cursor.Next = &canonicalDeclaredDepth{}
			cursor = cursor.Next
		}
		var output bytes.Buffer
		err := jsonio.WriteCanonicalProjection(t.Context(), &output, root, jsonio.OwnedCodecPolicy{Root: jsonio.RootOwnedMethods, PointerNulls: jsonio.PointerNullPermitted})
		if depth <= 64 {
			if err != nil {
				t.Fatalf("depth %d error = %v, want admitted", depth, err)
			}
		} else {
			var observed *jsonio.DepthError
			if !errors.Is(err, core.ErrJSONContract) || !errors.As(err, &observed) || observed.Maximum != 64 || observed.Observed != 65 {
				t.Fatalf("depth %d error = %v, want observed native depth 65", depth, err)
			}
		}
	}
	cyclic := &canonicalDeclaredDepth{}
	cyclic.Next = cyclic
	var output bytes.Buffer
	var observed *jsonio.DepthError
	if err := jsonio.WriteCanonicalProjection(t.Context(), &output, cyclic, jsonio.OwnedCodecPolicy{Root: jsonio.RootOwnedMethods}); !errors.As(err, &observed) {
		t.Fatalf("cyclic declaration error = %v, want native depth refusal", err)
	}
}

func TestCanonicalDeclaredProjectionRefusesUnsupportedDeclarationsAndPreservesCauses(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	policy := jsonio.OwnedCodecPolicy{Root: jsonio.RootOwnedMethods}
	if err := jsonio.WriteCanonicalProjection(t.Context(), &output, struct{ Value int }{}, policy); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("untagged declaration error = %v", err)
	}
	conflicting := reflect.New(reflect.StructOf([]reflect.StructField{
		{Name: "One", Type: reflect.TypeFor[int](), Tag: reflect.StructTag(`json:"same"`)},
		{Name: "Two", Type: reflect.TypeFor[int](), Tag: reflect.StructTag(`json:"same"`)},
	})).Elem().Interface()
	conflictCodec, err := jsonio.ProjectCanonical(func(canonicalProjectionOwner) (any, error) { return conflicting, nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := jsonio.WriteCanonicalProjection(t.Context(), &output, canonicalProjectionOwner{}, policy, conflictCodec); !errors.Is(err, core.ErrJSONContract) || errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("conflicting names error = %v", err)
	}
	var nilDocument *canonicalProjection
	if err := jsonio.WriteCanonicalProjection(t.Context(), &output, nilDocument, policy); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil document error = %v", err)
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if err := jsonio.WriteCanonicalProjection(cancelled, &output, canonicalProjection{}, policy); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled stream error = %v", err)
	}
	if err := jsonio.WriteCanonicalProjection(t.Context(), &output, canonicalProjection{}, policy, jsonio.CanonicalProjectionCodec{}); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("unconstructed codec error = %v", err)
	}
	reader, writer := io.Pipe()
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	err = jsonio.WriteCanonicalProjection(t.Context(), writer, canonicalProjection{}, policy)
	if closeErr := writer.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("closed pipe error = %v", err)
	}
}

func FuzzCanonicalDeclaredProjectionMatchesNativeWire(f *testing.F) {
	f.Add("雪\n\"<>&", uint64(9007199254740993))
	f.Add("", uint64(0))
	f.Add("\xff", uint64(18446744073709551615))
	f.Fuzz(func(t *testing.T, text string, number uint64) {
		document := canonicalDeclaredDocument{CanonicalDeclaredCounts: CanonicalDeclaredCounts{Z: number, A: -1}, Rows: []canonicalProjection{{A: text, Z: number}}, Nested: canonicalProjection{A: text, Z: number}, Astral: text, PrivateUse: text}
		var output bytes.Buffer
		err := jsonio.WriteCanonicalProjection(t.Context(), &output, document, jsonio.OwnedCodecPolicy{Root: jsonio.RootOwnedMethods})
		want, nativeErr := jsonv2.Marshal(document)
		if nativeErr != nil {
			if !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("native refusal = %v, stream refusal = %v", nativeErr, err)
			}
			return
		}
		canonical := jsontext.Value(want)
		if err := canonical.Canonicalize(jsontext.CanonicalizeRawInts(false)); err != nil {
			t.Fatal(err)
		}
		if err != nil || !bytes.Equal(output.Bytes(), canonical) {
			t.Fatalf("declared stream = (%q, %v), want native %q", output.Bytes(), err, canonical)
		}
		var decoded canonicalDeclaredDocument
		if err := jsonv2.Unmarshal(output.Bytes(), &decoded); err != nil || decoded.Z != number || decoded.Nested.A != text || len(decoded.Rows) != 1 || decoded.Rows[0].Z != number {
			t.Fatalf("round trip = (%+v, %v), want exact typed source", decoded, err)
		}
	})
}
