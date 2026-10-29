package runprotocol

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
)

func encodeFactFixture(t testing.TB, facts []RunFact) []byte {
	t.Helper()
	var out bytes.Buffer
	e, err := jsonio.NewObjectEncoder[RunFact](jsonio.ObjectDestinationRequest{Destination: &out})
	if err != nil {
		t.Fatal(err)
	}
	for _, fact := range facts {
		if err := e.Encode(t.Context(), fact); err != nil {
			t.Fatal(err)
		}
	}
	return out.Bytes()
}
func bindCompactBytes(t testing.TB, r RunReceipt, b []byte, count uint64) RunReceipt {
	t.Helper()
	r.Facts = RunFactSequence{Count: count, Bytes: receiptLength(t, uint64(len(b))), Digest: core.SHA256Of(b)}
	return r
}
func TestRunFactStreamLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		edit    func(testing.TB, *RunReceipt, []RunFact, []byte) []byte
		wantErr error
	}{
		{"exact typed compact stream", func(_ testing.TB, _ *RunReceipt, _ []RunFact, b []byte) []byte { return b }, nil},
		{"source refusal retains empty compact stream", func(t testing.TB, r *RunReceipt, _ []RunFact, _ []byte) []byte {
			r.Summary = RunSummary{}
			r.Source.Captured = nil
			r.Worker = nil
			r.Budget = nil
			r.Outcome = OutcomeNonAccepting
			*r = bindCompactBytes(t, *r, nil, 0)
			return nil
		}, nil},
		{"same size content change cannot borrow digest", func(_ testing.TB, _ *RunReceipt, _ []RunFact, b []byte) []byte {
			v := bytes.Clone(b)
			i := bytes.Index(v, []byte("TestNative"))
			v[i] = 'X'
			return v
		}, core.ErrRunProtocolConflict},
		{"wrong observed source cannot borrow command", func(_ testing.TB, r *RunReceipt, _ []RunFact, b []byte) []byte {
			r.Source.Captured.Digest = core.SHA256Of([]byte("foreign source"))
			return b
		}, core.ErrRunProtocolConflict},
		{"ordinal gap with honest bytes still refuses", func(t testing.TB, r *RunReceipt, f []RunFact, _ []byte) []byte {
			f[2].Ordinal++
			b := encodeFactFixture(t, f)
			*r = bindCompactBytes(t, *r, b, uint64(len(f)))
			return b
		}, core.ErrRunProtocolConflict},
		{"duplicate ordinal with honest bytes still refuses", func(t testing.TB, r *RunReceipt, f []RunFact, _ []byte) []byte {
			f[2].Ordinal = f[1].Ordinal
			b := encodeFactFixture(t, f)
			*r = bindCompactBytes(t, *r, b, uint64(len(f)))
			return b
		}, core.ErrRunProtocolConflict},
		{"sequence cannot start after one", func(t testing.TB, r *RunReceipt, f []RunFact, _ []byte) []byte {
			f[0].Ordinal = 2
			b := encodeFactFixture(t, f)
			*r = bindCompactBytes(t, *r, b, uint64(len(f)))
			return b
		}, core.ErrRunProtocolConflict},
		{"declared count cannot omit actual tail", func(_ testing.TB, r *RunReceipt, _ []RunFact, b []byte) []byte { r.Facts.Count--; return b }, core.ErrRunProtocolConflict},
		{"declared count cannot invent missing tail", func(_ testing.TB, r *RunReceipt, _ []RunFact, b []byte) []byte { r.Facts.Count++; return b }, core.ErrRunProtocolConflict},
		{"declared extent larger cannot borrow unchanged content", func(t testing.TB, r *RunReceipt, _ []RunFact, b []byte) []byte {
			r.Facts.Bytes = receiptLength(t, uint64(len(b)+1))
			return b
		}, core.ErrRunProtocolConflict},
		{"declared complete prefix cannot hide extra source", func(t testing.TB, r *RunReceipt, _ []RunFact, b []byte) []byte {
			prefix := b[:len(b)-1]
			*r = bindCompactBytes(t, *r, prefix, r.Facts.Count)
			return b
		}, core.ErrRunProtocolConflict},
		{"canonical frame cannot omit terminal LF", func(t testing.TB, r *RunReceipt, _ []RunFact, b []byte) []byte {
			b = b[:len(b)-1]
			*r = bindCompactBytes(t, *r, b, r.Facts.Count)
			return b
		}, core.ErrRunProtocolConflict},
		{"canonical frame cannot borrow alternate whitespace", func(t testing.TB, r *RunReceipt, _ []RunFact, b []byte) []byte {
			b = append([]byte(" "), b...)
			*r = bindCompactBytes(t, *r, b, r.Facts.Count)
			return b
		}, core.ErrRunProtocolConflict},
		{"honest digest cannot authorize false test summary", func(_ testing.TB, r *RunReceipt, _ []RunFact, b []byte) []byte {
			r.Summary.Tests.StartedCount = 2
			r.Summary.Tests.Passed = 2
			return b
		}, core.ErrRunProtocolConflict},
		{"honest digest cannot authorize false package summary", func(_ testing.TB, r *RunReceipt, _ []RunFact, b []byte) []byte {
			r.Summary.Packages.StartedCount = 2
			r.Summary.Packages.Passed = 2
			return b
		}, core.ErrRunProtocolConflict},
		{"synthetic result list cannot manufacture native execution", func(t testing.TB, r *RunReceipt, f []RunFact, _ []byte) []byte {
			f = append(f[:1], f[2:]...)
			for i := range f {
				f[i].Ordinal = uint64(i + 1)
			}
			b := encodeFactFixture(t, f)
			*r = bindCompactBytes(t, *r, b, uint64(len(f)))
			return b
		}, core.ErrRunProtocolConflict},
		{"test run cannot borrow explicit mutation fuzz result", func(t testing.TB, r *RunReceipt, f []RunFact, _ []byte) []byte {
			f[2].Result.Kind = RunResultFuzz
			name := receiptLabel(t, "FuzzNative")
			f[2].Result.Name = &name
			b := encodeFactFixture(t, f)
			*r = bindCompactBytes(t, *r, b, uint64(len(f)))
			return b
		}, core.ErrRunProtocolConflict},
		{"seed name prefix stays an ordinary test fact", func(t testing.TB, r *RunReceipt, f []RunFact, _ []byte) []byte {
			n := receiptLabel(t, "FuzzNative/seed")
			f[2].Result.Name = &n
			b := encodeFactFixture(t, f)
			*r = bindCompactBytes(t, *r, b, uint64(len(f)))
			return b
		}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r, f, b := compactReceiptFixture(t)
			b = tc.edit(t, &r, f, b)
			err := VerifyRunFacts(t.Context(), RunFactStreamRequest{Source: bytes.NewReader(b), Receipt: r})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("VerifyRunFacts = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

type compactFailureReader struct {
	data   []byte
	cause  error
	cancel context.CancelCauseFunc
	reads  int
}

func (r *compactFailureReader) Read(p []byte) (int, error) {
	r.reads++
	n := copy(p, r.data)
	r.data = r.data[n:]
	if len(r.data) > 0 {
		return n, nil
	}
	if r.cancel != nil {
		r.cancel(r.cause)
	}
	return n, r.cause
}

type compactPanicReader struct{}

func (compactPanicReader) Read([]byte) (int, error) { panic(io.ErrClosedPipe) }
func TestRunFactStreamNativeFailures(t *testing.T) {
	t.Parallel()
	r, _, body := compactReceiptFixture(t)
	for _, tc := range []struct {
		name   string
		source func() io.Reader
		want   error
	}{
		{"late failure after complete records", func() io.Reader { return io.MultiReader(bytes.NewReader(body), iotest.ErrReader(io.ErrClosedPipe)) }, io.ErrClosedPipe},
		{"complete bytes and error in same native read", func() io.Reader { return &compactFailureReader{data: bytes.Clone(body), cause: io.ErrClosedPipe} }, io.ErrClosedPipe},
		{"partial bytes and source refusal", func() io.Reader {
			return &compactFailureReader{data: bytes.Clone(body[:len(body)/2]), cause: io.ErrClosedPipe}
		}, io.ErrClosedPipe},
		{"joined EOF cannot hide competing failure", func() io.Reader {
			return io.MultiReader(bytes.NewReader(body), iotest.ErrReader(errors.Join(io.EOF, io.ErrClosedPipe)))
		}, io.ErrClosedPipe},
		{"recovered native source panic remains refusal", func() io.Reader { return io.MultiReader(bytes.NewReader(body), compactPanicReader{}) }, core.ErrFilestoreCallbackPanic},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := VerifyRunFacts(t.Context(), RunFactStreamRequest{Source: tc.source(), Receipt: r})
			if !errors.Is(err, tc.want) || !errors.Is(err, core.ErrRunProtocolContract) {
				t.Fatalf("native compact source = %v, want contract/%v", err, tc.want)
			}
		})
	}
}
func TestRunFactStreamClosedNativeFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "compact-source")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	r, _, b := compactReceiptFixture(t)
	err = VerifyRunFacts(t.Context(), RunFactStreamRequest{Source: io.MultiReader(bytes.NewReader(b), file), Receipt: r})
	native, ok := errors.AsType[*os.PathError](err)
	if !ok || native.Op != "read" || native.Path != path || !errors.Is(err, os.ErrClosed) || !errors.Is(err, core.ErrRunProtocolContract) {
		t.Fatalf("closed compact source = (%v,%v), want original closed PathError", native, err)
	}
}
func TestRunFactStreamCancellation(t *testing.T) {
	t.Parallel()
	for _, before := range []bool{true, false} {
		name := "after source progress"
		if before {
			name = "before source progress"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r, _, b := compactReceiptFixture(t)
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			reader := &compactFailureReader{data: bytes.Clone(b), cause: io.ErrClosedPipe, cancel: cancel}
			if before {
				cancel(io.ErrClosedPipe)
			}
			err := VerifyRunFacts(ctx, RunFactStreamRequest{Source: reader, Receipt: r})
			if !errors.Is(err, context.Canceled) || !errors.Is(err, io.ErrClosedPipe) || before && reader.reads != 0 || !before && reader.reads == 0 {
				t.Fatalf("cancelled compact source = (%d,%v), want native cancellation/cause and matching progress", reader.reads, err)
			}
		})
	}
}

type compactShortWriter struct{}

func (compactShortWriter) Write(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	return len(b) - 1, nil
}
func TestRunFactWriterFailureLayerTriad(t *testing.T) {
	t.Parallel()
	_, facts, _ := compactReceiptFixture(t)
	cases := []struct {
		name        string
		destination io.Writer
		values      []RunFact
		want        error
	}{
		{"native short write returns no proof", compactShortWriter{}, facts, io.ErrShortWrite},
		{"nil destination cannot receive facts", nil, facts, core.ErrRunProtocolContract},
		{"ordinal gap refuses before publishing that record", io.Discard, func() []RunFact { v := append([]RunFact{}, facts...); v[1].Ordinal = 3; return v }(), core.ErrRunProtocolConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := WriteRunFacts(t.Context(), RunFactWriteRequest{Destination: tc.destination, Facts: receiptFacts(tc.values)})
			if !errors.Is(err, tc.want) || got != (RunFactSequence{}) {
				t.Fatalf("failed compact write = (%+v,%v), want zero/%v", got, err, tc.want)
			}
		})
	}
}
func TestRunFactStreamExtentBoundary(t *testing.T) {
	t.Parallel()
	r, _, _ := compactReceiptFixture(t)
	for _, extent := range []uint64{math.MaxInt64, math.MaxInt64 + 1, math.MaxUint64} {
		t.Run(func() string {
			if extent == math.MaxInt64 {
				return "maximum native signed extent reaches source"
			}
			if extent == math.MaxInt64+1 {
				return "one beyond native extent refuses before source"
			}
			return "extreme unsigned extent refuses before source"
		}(), func(t *testing.T) {
			t.Parallel()
			v, err := core.NewByteLength(extent)
			if extent > math.MaxInt64 {
				if !errors.Is(err, core.ErrNumericOverflow) {
					t.Fatalf("native byte extent%d = %v, want overflow", extent, err)
				}
				return
			}
			copy := r
			copy.Facts.Bytes = v
			source := &compactFailureReader{cause: io.ErrClosedPipe}
			err = VerifyRunFacts(t.Context(), RunFactStreamRequest{Source: source, Receipt: copy})
			if !errors.Is(err, io.ErrClosedPipe) || source.reads == 0 {
				t.Fatalf("maximum compact extent = (%d,%v), want observed native source refusal", source.reads, err)
			}
		})
	}
}
