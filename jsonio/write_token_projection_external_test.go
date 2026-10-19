package jsonio_test

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"io"
	"testing"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
)

func TestTokenProjectionBorrowsNativeCursorWithoutChangingSignedBytes(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	var loan *jsonio.TokenEncoder
	err := jsonio.WriteTokenProjection(t.Context(), jsonio.TokenProjectionRequest{Destination: &output, Use: func(cursor *jsonio.TokenEncoder) error {
		loan = cursor
		if err := cursor.Encode(t.Context(), jsonio.Token{Kind: jsonio.TokenArrayStart}); err != nil {
			return err
		}
		for _, value := range []uint64{0, 9007199254740993, 18446744073709551615} {
			if err := jsonio.EncodeCanonicalProjection(t.Context(), cursor, value, jsonio.OwnedCodecPolicy{Root: jsonio.RootOwnedMethods}); err != nil {
				return err
			}
		}
		return cursor.Encode(t.Context(), jsonio.Token{Kind: jsonio.TokenArrayEnd})
	}})
	if err != nil || output.String() != `[0,9007199254740993,18446744073709551615]` {
		t.Fatalf("token projection = (%q, %v), want exact integer preimage without LF", output.String(), err)
	}
	before := bytes.Clone(output.Bytes())
	if err := loan.Encode(t.Context(), jsonio.Token{Kind: jsonio.TokenNull}); !errors.Is(err, core.ErrJSONContract) || !bytes.Equal(output.Bytes(), before) {
		t.Fatalf("expired loan = (%q, %v), want unchanged output and refusal", output.String(), err)
	}
}

func FuzzCanonicalTokenProjectionNativeClosure(f *testing.F) {
	f.Add("雪😀\n\"<>&", uint64(9007199254740993))
	f.Add("", uint64(0))
	f.Add("\xff", uint64(18446744073709551615))
	f.Fuzz(func(t *testing.T, text string, number uint64) {
		document := canonicalProjection{Z: number, A: text}
		var output bytes.Buffer
		write := func(destination io.Writer) error {
			return jsonio.WriteTokenProjection(t.Context(), jsonio.TokenProjectionRequest{Destination: destination, Use: func(cursor *jsonio.TokenEncoder) error {
				return jsonio.EncodeCanonicalProjection(t.Context(), cursor, document, jsonio.OwnedCodecPolicy{})
			}})
		}
		err := write(&output)
		if !utf8.ValidString(text) {
			if !errors.Is(err, core.ErrJSONContract) || output.Len() != 0 {
				t.Fatalf("malformed text = (%q, %v), want zero output and JSON refusal", output.String(), err)
			}
			return
		}
		want, nativeErr := jsonv2.Marshal(document)
		if nativeErr != nil {
			t.Fatal(nativeErr)
		}
		canonical := jsontext.Value(want)
		if nativeErr := canonical.Canonicalize(jsontext.CanonicalizeRawInts(false)); nativeErr != nil {
			t.Fatal(nativeErr)
		}
		if err != nil || !bytes.Equal(output.Bytes(), canonical) {
			t.Fatalf("canonical projection = (%q, %v), want native %q", output.Bytes(), err, canonical)
		}
		var roundTrip canonicalProjection
		if err := jsonv2.Unmarshal(output.Bytes(), &roundTrip); err != nil || roundTrip != document {
			t.Fatalf("round trip = (%+v, %v), want %+v", roundTrip, err, document)
		}
		var second bytes.Buffer
		if err := write(&second); err != nil || !bytes.Equal(second.Bytes(), output.Bytes()) {
			t.Fatalf("second projection = (%q, %v), want %q", second.Bytes(), err, output.Bytes())
		}
	})
}

func TestTokenProjectionGoRejectsIncompleteAndMultipleValues(t *testing.T) {
	t.Parallel()
	for _, tokens := range [][]jsonio.Token{
		nil,
		{{Kind: jsonio.TokenArrayStart}},
		{{Kind: jsonio.TokenNull}, {Kind: jsonio.TokenNull}},
	} {
		var output bytes.Buffer
		err := jsonio.WriteTokenProjection(t.Context(), jsonio.TokenProjectionRequest{Destination: &output, Use: func(cursor *jsonio.TokenEncoder) error {
			for _, token := range tokens {
				if err := cursor.Encode(t.Context(), token); err != nil {
					return err
				}
			}
			return nil
		}})
		if !errors.Is(err, core.ErrJSONContract) {
			t.Fatalf("tokens %v error = %v, want native grammar refusal", tokens, err)
		}
	}
}

func TestTokenProjectionPreservesCallbackAndAdmissionRefusals(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	err := jsonio.WriteTokenProjection(t.Context(), jsonio.TokenProjectionRequest{Destination: &output, Use: func(*jsonio.TokenEncoder) error { return io.ErrClosedPipe }})
	if !errors.Is(err, io.ErrClosedPipe) || output.Len() != 0 {
		t.Fatalf("callback refusal = (%q, %v)", output.String(), err)
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	err = jsonio.WriteTokenProjection(cancelled, jsonio.TokenProjectionRequest{Destination: &output, Use: func(*jsonio.TokenEncoder) error { t.Fatal("cancelled request invoked projection"); return nil }})
	if !errors.Is(err, context.Canceled) || output.Len() != 0 {
		t.Fatalf("cancelled projection = (%q, %v)", output.String(), err)
	}
	for _, request := range []jsonio.TokenProjectionRequest{{}, {Destination: &output}, {Use: func(*jsonio.TokenEncoder) error { t.Fatal("nil destination invoked projection"); return nil }}} {
		if err := jsonio.WriteTokenProjection(t.Context(), request); !errors.Is(err, core.ErrJSONContract) {
			t.Fatalf("invalid request error = %v", err)
		}
	}
}
