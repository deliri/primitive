package jsonio_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
)

type projectionRefusingReader struct{ cause error }

func (r projectionRefusingReader) Read(p []byte) (int, error) {
	return copy(p, `{"a":"provisional","z":1}`), r.cause
}

type projectionRefusingWriter struct{ cause error }

func (w projectionRefusingWriter) Write(p []byte) (int, error) { return len(p), w.cause }

func TestProjectionCursorStreamsArrayAndRetainsNativeRefusals(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	cursor, err := jsonio.NewTokenDecoder(jsonio.TokenRequest{Source: strings.NewReader(`[{"a":"first","z":9007199254740993,"extra":[]},{"a":"second","z":2}]`), NestingDepthMaximum: 64})
	if err != nil {
		t.Fatal(err)
	}
	if token, err := cursor.Decode(ctx); err != nil || token.Kind != jsonio.TokenArrayStart {
		t.Fatalf("array start=%+v %v", token, err)
	}
	for i, want := range []canonicalProjection{{A: "first", Z: 9007199254740993}, {A: "second", Z: 2}} {
		if kind, err := cursor.PeekKind(ctx); err != nil || kind != jsonio.TokenObjectStart {
			t.Fatalf("peek %d: %v %v", i, kind, err)
		}
		got, err := jsonio.DecodeProjectionAt[canonicalProjection](ctx, cursor, jsonio.MembersIgnoreExtra)
		if err != nil || got != want {
			t.Fatalf("record %d: %+v %v", i, got, err)
		}
	}
	if token, err := cursor.Decode(ctx); err != nil || token.Kind != jsonio.TokenArrayEnd {
		t.Fatalf("array close=%+v %v", token, err)
	}
	if kind, err := cursor.PeekKind(ctx); kind != jsonio.TokenUnknown || err != io.EOF {
		t.Fatalf("EOF=%v %v", kind, err)
	}
	for _, peek := range []bool{false, true} {
		cursor, err := jsonio.NewTokenDecoder(jsonio.TokenRequest{Source: projectionRefusingReader{cause: io.ErrClosedPipe}, NestingDepthMaximum: 64})
		if err != nil {
			t.Fatal(err)
		}
		if peek {
			kind, err := cursor.PeekKind(ctx)
			if kind != jsonio.TokenUnknown || !errors.Is(err, io.ErrClosedPipe) || errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("native peek=%v %v", kind, err)
			}
		} else {
			got, err := jsonio.DecodeProjectionAt[canonicalProjection](ctx, cursor, jsonio.MembersDeclaredOnly)
			if got != (canonicalProjection{}) || !errors.Is(err, io.ErrClosedPipe) || errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("native decode=%+v %v", got, err)
			}
		}
	}
	late, err := jsonio.NewTokenDecoder(jsonio.TokenRequest{Source: io.MultiReader(strings.NewReader(`{"z":1,"a":"`), iotest.ErrReader(io.ErrClosedPipe)), NestingDepthMaximum: 64})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := jsonio.DecodeProjectionAt[canonicalProjection](ctx, late, jsonio.MembersDeclaredOnly); got != (canonicalProjection{}) || !errors.Is(err, io.ErrClosedPipe) || errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("late native decode=%+v %v", got, err)
	}
	cancelled, cancel := context.WithCancelCause(ctx)
	cancel(io.ErrNoProgress)
	if got, err := jsonio.DecodeProjectionAt[canonicalProjection](cancelled, cursor, jsonio.MembersDeclaredOnly); got != (canonicalProjection{}) || !errors.Is(err, context.Canceled) || !errors.Is(err, io.ErrNoProgress) {
		t.Fatalf("cancelled=%+v %v", got, err)
	}
}

func TestWriteProjectionNativeFormattingAndWriteIdentity(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		layout jsonio.Layout
		want   string
	}{
		{jsonio.LayoutCompact, `{"z":1,"a":"x"}`},
		{jsonio.LayoutTwoSpaces, "{\n  \"z\": 1,\n  \"a\": \"x\"\n}"},
		{jsonio.LayoutTabs, "{\n\t\"z\": 1,\n\t\"a\": \"x\"\n}"},
	} {
		var output bytes.Buffer
		if err := jsonio.WriteProjection(t.Context(), &output, canonicalProjection{Z: 1, A: "x"}, jsonio.OwnedCodecPolicy{}, test.layout); err != nil || output.String() != test.want {
			t.Fatalf("layout=%v output=%q err=%v", test.layout, output.String(), err)
		}
	}
	err := jsonio.WriteProjection(t.Context(), projectionRefusingWriter{cause: io.ErrClosedPipe}, canonicalProjection{}, jsonio.OwnedCodecPolicy{}, jsonio.LayoutTwoSpaces)
	if !errors.Is(err, io.ErrClosedPipe) || errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("writer identity=%v", err)
	}
	var output bytes.Buffer
	if err := jsonio.WriteProjection(t.Context(), &output, canonicalProjection{}, jsonio.OwnedCodecPolicy{}, jsonio.Layout(255)); !errors.Is(err, core.ErrJSONContract) || output.Len() != 0 {
		t.Fatalf("invalid layout writes=%d err=%v", output.Len(), err)
	}
	if err := jsonio.WriteProjection[*canonicalProjection](t.Context(), &output, nil, jsonio.OwnedCodecPolicy{}, jsonio.LayoutCompact); !errors.Is(err, core.ErrJSONContract) || output.Len() != 0 {
		t.Fatalf("nil projection writes=%d err=%v", output.Len(), err)
	}
}
