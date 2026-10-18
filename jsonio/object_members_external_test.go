package jsonio_test

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
)

// Reversed declaration order makes canonical member ordering observable.
type canonicalMemberRow struct {
	Zeta  string `json:"zeta"`
	Alpha int64  `json:"alpha"`
}

func (canonicalMemberRow) Validate() error { return nil }

type refusedCanonicalRow struct{ Value string }

func (refusedCanonicalRow) Validate() error { return io.ErrClosedPipe }

// These deliberate writer-contract fixtures supplement the real descriptor
// refusal below; they do not stand in for native filesystem proof.
type memberDestination struct {
	bytes.Buffer
	delta int
	cause error
}

func (w *memberDestination) Write(data []byte) (int, error) {
	if w.delta > 0 {
		return len(data) + w.delta, w.cause
	}
	if w.delta < 0 {
		n, _ := w.Buffer.Write(data[:len(data)-1])
		return n, w.cause
	}
	n, _ := w.Buffer.Write(data)
	return n, w.cause
}

func writeMemberRows(ctx context.Context, destination io.Writer, text string, ordinal int64, count uint8) error {
	return jsonio.WriteObjectMembers(ctx, jsonio.ObjectMemberRequest{Destination: destination, Members: func(ctx context.Context, cursor *jsonio.TokenEncoder) error {
		for _, token := range [...]jsonio.Token{{Kind: jsonio.TokenString, Text: "rows"}, {Kind: jsonio.TokenArrayStart}} {
			if err := cursor.Encode(ctx, token); err != nil {
				return err
			}
		}
		for range count {
			if err := jsonio.EncodeCanonicalObject(ctx, cursor, canonicalMemberRow{Zeta: text, Alpha: ordinal}, jsonio.OwnedCodecPolicy{Root: jsonio.RootDeclaredFields, PointerNulls: jsonio.PointerNullPermitted}); err != nil {
				return err
			}
		}
		return cursor.Encode(ctx, jsonio.Token{Kind: jsonio.TokenArrayEnd})
	}})
}

func TestObjectMembersNativeFramingCanonicalRowsAndBorrowLifetime(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	if err := writeMemberRows(t.Context(), &output, "\"\\<é", 9007199254740993, 2); err != nil {
		t.Fatal(err)
	}
	want := `{"rows":[{"alpha":9007199254740993,"zeta":"\"\\<é"},{"alpha":9007199254740993,"zeta":"\"\\<é"}]}`
	if output.String() != want {
		t.Fatalf("object = %q, want exact canonical rows, preserved integer and no LF %q", output.String(), want)
	}
	var empty bytes.Buffer
	var borrowed *jsonio.TokenEncoder
	err := jsonio.WriteObjectMembers(t.Context(), jsonio.ObjectMemberRequest{Destination: &empty, Members: func(_ context.Context, cursor *jsonio.TokenEncoder) error { borrowed = cursor; return nil }})
	if err != nil || empty.String() != "{}" {
		t.Fatalf("empty object = %q, %v, want {}, nil", empty.String(), err)
	}
	if borrowed == nil {
		t.Fatal("native cursor was not lent")
	}
	if err := borrowed.Validate(); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("borrowed cursor remained valid after scope: %v", err)
	}
	if err := borrowed.Encode(t.Context(), jsonio.Token{Kind: jsonio.TokenNull}); !errors.Is(err, core.ErrJSONContract) || empty.String() != "{}" {
		t.Fatalf("escaped cursor = %q, %v; want unchanged object and refused lifetime", empty.String(), err)
	}
}

func TestObjectMembersPreservesPartialWriterAndCancellationCauses(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		delta int
		cause error
		want  error
	}{
		{name: "short native count", delta: -1, want: io.ErrShortWrite},
		{name: "complete bytes and refusal", cause: io.ErrClosedPipe, want: io.ErrClosedPipe},
		{name: "partial bytes and refusal", delta: -1, cause: io.ErrClosedPipe, want: io.ErrClosedPipe},
		{name: "impossible accepted count", delta: 1, want: core.ErrJSONContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			writer := memberDestination{delta: tc.delta, cause: tc.cause}
			err := writeMemberRows(t.Context(), &writer, "actual value", 1, 1)
			if !errors.Is(err, tc.want) {
				t.Fatalf("writer refusal = %v, want %v", err, tc.want)
			}
			if tc.delta <= 0 && writer.Len() == 0 {
				t.Fatal("fixture did not exercise actual accepted-byte progress")
			}
		})
	}
	t.Run("callback refusal with cancellation", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		var output bytes.Buffer
		err := jsonio.WriteObjectMembers(ctx, jsonio.ObjectMemberRequest{Destination: &output, Members: func(context.Context, *jsonio.TokenEncoder) error {
			cancel()
			return io.ErrClosedPipe
		}})
		if !errors.Is(err, context.Canceled) || !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("cancelled callback = %v, want cancellation and native refusal", err)
		}
	})
}

func TestObjectMembersNativeGrammarAndRefusalBoundaries(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		tokens []jsonio.Token
		cause  error
	}{
		{name: "duplicate member", tokens: []jsonio.Token{{Kind: jsonio.TokenString, Text: "x"}, {Kind: jsonio.TokenNull}, {Kind: jsonio.TokenString, Text: "x"}, {Kind: jsonio.TokenNull}}, cause: jsontext.ErrDuplicateName},
		{name: "missing member value", tokens: []jsonio.Token{{Kind: jsonio.TokenString, Text: "x"}}},
		{name: "nonstring name", tokens: []jsonio.Token{{Kind: jsonio.TokenNull}}, cause: jsontext.ErrNonStringName},
		{name: "unclosed member array", tokens: []jsonio.Token{{Kind: jsonio.TokenString, Text: "x"}, {Kind: jsonio.TokenArrayStart}}},
		{name: "caller closes owning object", tokens: []jsonio.Token{{Kind: jsonio.TokenObjectEnd}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var output bytes.Buffer
			err := jsonio.WriteObjectMembers(t.Context(), jsonio.ObjectMemberRequest{Destination: &output, Members: func(ctx context.Context, cursor *jsonio.TokenEncoder) error {
				for _, token := range tc.tokens {
					if err := cursor.Encode(ctx, token); err != nil {
						return err
					}
				}
				return nil
			}})
			if !errors.Is(err, core.ErrJSONContract) || tc.cause != nil && !errors.Is(err, tc.cause) {
				t.Fatalf("grammar refusal = %v, want JSON contract and %v", err, tc.cause)
			}
		})
	}
	t.Run("callback native cause", func(t *testing.T) {
		t.Parallel()
		var output bytes.Buffer
		err := jsonio.WriteObjectMembers(t.Context(), jsonio.ObjectMemberRequest{Destination: &output, Members: func(context.Context, *jsonio.TokenEncoder) error { return io.ErrClosedPipe }})
		if !errors.Is(err, io.ErrClosedPipe) || !errors.Is(err, core.ErrJSONContract) {
			t.Fatalf("callback refusal = %v; want native cause and JSON identity", err)
		}
	})
	t.Run("closed native descriptor", func(t *testing.T) {
		t.Parallel()
		file, err := os.Create(filepath.Join(t.TempDir(), "closed.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		err = writeMemberRows(t.Context(), file, "actual value", 1, 1)
		var native *os.PathError
		if !errors.Is(err, os.ErrClosed) || !errors.As(err, &native) {
			t.Fatalf("closed descriptor = %v; want native closed-file cause", err)
		}
	})
}

func TestObjectMembersAdmissionAndCanonicalValueRefusalPrecedeEffects(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		request jsonio.ObjectMemberRequest
	}{
		{name: "missing destination", request: jsonio.ObjectMemberRequest{Members: func(context.Context, *jsonio.TokenEncoder) error { return io.ErrClosedPipe }}},
		{name: "missing member producer", request: jsonio.ObjectMemberRequest{Destination: io.Discard}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := jsonio.WriteObjectMembers(t.Context(), tc.request); !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("request refusal = %v; want JSON contract", err)
			}
		})
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var output bytes.Buffer
	err := jsonio.WriteObjectMembers(ctx, jsonio.ObjectMemberRequest{Destination: &output, Members: func(context.Context, *jsonio.TokenEncoder) error { return io.ErrClosedPipe }})
	if !errors.Is(err, context.Canceled) || errors.Is(err, io.ErrClosedPipe) || output.Len() != 0 {
		t.Fatalf("cancelled admission = %q, %v; want zero bytes and cancellation before callback", output.String(), err)
	}
	cursor, err := jsonio.NewTokenEncoder(jsonio.ObjectDestinationRequest{Destination: &output})
	if err != nil {
		t.Fatal(err)
	}
	if err := jsonio.EncodeCanonicalObject(t.Context(), cursor, core.ByteLength{}, jsonio.OwnedCodecPolicy{Root: jsonio.RootOwnedMethods, PointerNulls: jsonio.PointerNullPermitted}); !errors.Is(err, core.ErrJSONContract) || output.Len() != 0 {
		t.Fatalf("nonobject canonical value = %q, %v; want zero bytes and JSON contract", output.String(), err)
	}
	if err := jsonio.EncodeCanonicalObject(t.Context(), cursor, refusedCanonicalRow{Value: "provisional"}, jsonio.OwnedCodecPolicy{Root: jsonio.RootDeclaredFields, PointerNulls: jsonio.PointerNullPermitted}); !errors.Is(err, io.ErrClosedPipe) || output.Len() != 0 {
		t.Fatalf("invalid owning document = %q, %v; want no bytes and owning refusal", output.String(), err)
	}
}

func FuzzObjectMembersMatchesNativeCanonicalDocument(f *testing.F) {
	f.Add("ordinary", int64(0), uint8(0))
	f.Add("\"\\\x00é", int64(9007199254740993), uint8(2))
	f.Add(string([]byte{0xff}), int64(-9223372036854775807), uint8(1))
	f.Fuzz(func(t *testing.T, text string, ordinal int64, selection uint8) {
		if len(text) > 65537 { // Fixture budget, not a product input ceiling.
			return
		}
		count := selection % 9
		rows := make([]canonicalMemberRow, count)
		for i := range rows {
			rows[i] = canonicalMemberRow{Zeta: text, Alpha: ordinal}
		}
		document := struct {
			Rows []canonicalMemberRow `json:"rows"`
		}{Rows: rows}
		want, wantErr := json.Marshal(document)
		if wantErr == nil {
			value := jsontext.Value(want)
			wantErr = value.Canonicalize(jsontext.CanonicalizeRawInts(false))
			want = []byte(value)
		}
		var output bytes.Buffer
		err := writeMemberRows(t.Context(), &output, text, ordinal, count)
		if wantErr != nil {
			if !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("invalid document = %v; want JSON refusal matching native oracle %v", err, wantErr)
			}
			return
		}
		if err != nil || !bytes.Equal(output.Bytes(), want) {
			t.Fatalf("member stream = %q, %v; want independent native document %q", output.Bytes(), err, want)
		}
	})
}
