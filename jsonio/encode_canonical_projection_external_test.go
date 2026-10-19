package jsonio_test

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
)

func TestCanonicalProjectionCursorMatchesNativeScalarAndObjectBytes(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	encoder, err := jsonio.NewTokenEncoder(jsonio.ObjectDestinationRequest{Destination: &output})
	if err != nil {
		t.Fatal(err)
	}
	if err := encoder.Encode(t.Context(), jsonio.Token{Kind: jsonio.TokenArrayStart}); err != nil {
		t.Fatal(err)
	}
	policy := jsonio.OwnedCodecPolicy{Root: jsonio.RootOwnedMethods, PointerNulls: jsonio.PointerNullPermitted}
	canonicalCursorValue(t, encoder, uint64(9007199254740993), policy)
	canonicalCursorValue(t, encoder, int64(math.MinInt64), policy)
	canonicalCursorValue(t, encoder, 1.0, policy)
	canonicalCursorValue(t, encoder, "雪😀\n\"<>&", policy)
	canonicalCursorValue(t, encoder, false, policy)
	canonicalCursorValue(t, encoder, canonicalProjection{Z: 7, A: "a"}, policy)
	canonicalCursorValue(t, encoder, []uint64{}, policy)
	if err := encoder.Encode(t.Context(), jsonio.Token{Kind: jsonio.TokenArrayEnd}); err != nil {
		t.Fatal(err)
	}
	// Each independently encoded native value supplies the oracle; integer
	// spellings above 2^53 must survive Go's canonicalization without rounding.
	want := jsontext.Value(`[9007199254740993,-9223372036854775808,1,"雪😀\n\"<>&",false,{"z":7,"a":"a"},[]]`)
	if err := want.Canonicalize(jsontext.CanonicalizeRawInts(false)); err != nil {
		t.Fatal(err)
	}
	want = append(want, '\n')
	if !bytes.Equal(output.Bytes(), want) {
		t.Fatalf("canonical cursor = %q, want %q", output.Bytes(), want)
	}
}

func canonicalCursorValue[Value any](t *testing.T, encoder *jsonio.TokenEncoder, value Value, policy jsonio.OwnedCodecPolicy) {
	t.Helper()
	if err := jsonio.EncodeCanonicalProjection(t.Context(), encoder, value, policy); err != nil {
		t.Fatal(err)
	}
}

func TestCanonicalProjectionRefusalLeavesNativeCursorUnchanged(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	encoder, err := jsonio.NewTokenEncoder(jsonio.ObjectDestinationRequest{Destination: &output})
	if err != nil {
		t.Fatal(err)
	}
	if err := encoder.Encode(t.Context(), jsonio.Token{Kind: jsonio.TokenArrayStart}); err != nil {
		t.Fatal(err)
	}
	policy := jsonio.OwnedCodecPolicy{Root: jsonio.RootOwnedMethods, PointerNulls: jsonio.PointerNullPermitted}
	canonicalCursorValue(t, encoder, 1, policy)
	if err := jsonio.EncodeCanonicalProjection(t.Context(), encoder, "\xff", policy); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("invalid UTF-8 error = %v", err)
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if err := jsonio.EncodeCanonicalProjection(cancelled, encoder, 9, policy); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled projection error = %v", err)
	}
	canonicalCursorValue(t, encoder, 2, policy)
	if err := encoder.Encode(t.Context(), jsonio.Token{Kind: jsonio.TokenArrayEnd}); err != nil {
		t.Fatal(err)
	}
	want, err := jsonv2.Marshal([]int{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	want = append(want, '\n')
	if !bytes.Equal(output.Bytes(), want) {
		t.Fatalf("cursor after refusals = %q, want %q", output.Bytes(), want)
	}
	if err := jsonio.EncodeCanonicalProjection(t.Context(), nil, 1, policy); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil cursor error = %v", err)
	}
}

func TestCanonicalProjectionPreservesNativeClosedWriterCause(t *testing.T) {
	t.Parallel()
	file, err := os.Create(filepath.Join(t.TempDir(), "closed.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	encoder, err := jsonio.NewTokenEncoder(jsonio.ObjectDestinationRequest{Destination: file})
	if err != nil {
		t.Fatal(err)
	}
	err = jsonio.EncodeCanonicalProjection(t.Context(), encoder, 1, jsonio.OwnedCodecPolicy{Root: jsonio.RootOwnedMethods})
	var cause *fs.PathError
	if !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, fs.ErrClosed) || !errors.As(err, &cause) {
		t.Fatalf("closed writer error = %v, want JSON identity and native PathError", err)
	}
}
