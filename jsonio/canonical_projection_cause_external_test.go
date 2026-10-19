package jsonio_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/jsonio"
)

func TestCanonicalProjectionPreservesNativeCancellationCauseBeforeWriting(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(io.ErrClosedPipe)
	var output bytes.Buffer
	err := jsonio.WriteCanonicalProjection(ctx, &output, canonicalProjection{A: "cause", Z: 3}, jsonio.OwnedCodecPolicy{Root: jsonio.RootOwnedMethods})
	if !errors.Is(err, context.Canceled) || !errors.Is(err, io.ErrClosedPipe) || output.Len() != 0 {
		t.Fatalf("cancelled projection = (%q, %v), want no bytes, cancellation and native cause", output.String(), err)
	}
}

func TestCanonicalProjectionPreservesCancellationCauseAfterNativeAcceptance(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)
	document := canonicalProjection{A: "accepted bytes", Z: 9007199254740993}
	want, err := jsonio.AppendCanonicalProjection(nil, document, jsonio.OwnedCodecPolicy{Root: jsonio.RootOwnedMethods}, jsonio.CanonicalPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	output := &canonicalCauseDestination{cancel: cancel}
	err = jsonio.WriteCanonicalProjection(ctx, output, document, jsonio.OwnedCodecPolicy{Root: jsonio.RootOwnedMethods})
	if !errors.Is(err, context.Canceled) || !errors.Is(err, io.ErrClosedPipe) || !errors.Is(err, io.ErrUnexpectedEOF) || !bytes.Equal(output.bytes.Bytes(), want) {
		t.Fatalf("accepted projection = (%q, %v), want actual canonical bytes %q, cancellation, native cause and destination refusal", output.bytes.Bytes(), err, want)
	}
}

type canonicalCauseDestination struct {
	bytes  bytes.Buffer
	cancel context.CancelCauseFunc
}

func (w *canonicalCauseDestination) Write(data []byte) (int, error) {
	n, err := w.bytes.Write(data)
	if err != nil {
		return n, err
	}
	w.cancel(io.ErrClosedPipe)
	return n, io.ErrUnexpectedEOF
}
