package filestore

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/core"
)

// Only the Reader contract is exposed. Optional methods cannot certify EOF.
type copyReaderOnly struct{ io.Reader }

type misleadingLengthReader struct {
	io.Reader
	remaining int
}

func (r misleadingLengthReader) Len() int { return r.remaining }

type terminalCopyReader struct {
	data            []byte
	err             error
	countAdjustment int
	reads           int
}

func (r *terminalCopyReader) Read(p []byte) (int, error) {
	r.reads++
	n := copy(p, r.data)
	r.data = r.data[n:]
	if r.countAdjustment < 0 {
		return -1, r.err
	}
	if r.countAdjustment > 0 {
		return len(p) + 1, r.err
	}
	if len(r.data) == 0 {
		return n, r.err
	}
	return n, nil
}

type stalledCopyReader struct {
	data  []byte
	empty int
	reads int
}

func (r *stalledCopyReader) Read(p []byte) (int, error) {
	r.reads++
	if len(r.data) > 0 {
		n := copy(p, r.data)
		r.data = r.data[n:]
		return n, nil
	}
	if r.empty > 0 {
		r.empty--
		return 0, nil
	}
	return 0, io.EOF
}

func TestBoundedCopySourceObservationLayerTriad(t *testing.T) {
	t.Parallel()
	// These rows distinguish observed EOF from declarations, and destination
	// bytes from the one overflow byte consumed solely to prove the ceiling.
	cases := []struct {
		name        string
		source      func() io.Reader
		maximum     uint64
		known       uint64
		extentKnown bool
		want        []byte
		wantErr     error
		wantNative  error
	}{
		{name: "empty opaque source produces an empty receipt", source: func() io.Reader { return copyReaderOnly{bytes.NewReader(nil)} }, maximum: 1},
		{name: "opaque exact ceiling requires an actual eof observation", source: func() io.Reader { return copyReaderOnly{bytes.NewReader([]byte{0, 255})} }, maximum: 2, want: []byte{0, 255}},
		{name: "one byte below ceiling must not require filling it", source: func() io.Reader { return copyReaderOnly{bytes.NewReader([]byte{0})} }, maximum: 2, want: []byte{0}},
		{name: "one byte above ceiling is consumed but never written", source: func() io.Reader { return copyReaderOnly{bytes.NewReader([]byte{0, 255, 7})} }, maximum: 2, want: []byte{0, 255}, wantErr: core.ErrFilestoreSize},
		{name: "one byte reader cannot be rejected at exact ceiling", source: func() io.Reader { return iotest.OneByteReader(bytes.NewReader([]byte{0, 255})) }, maximum: 2, want: []byte{0, 255}},
		{name: "half reader cannot be rejected at exact ceiling", source: func() io.Reader { return iotest.HalfReader(bytes.NewReader([]byte{0, 255, 7})) }, maximum: 3, want: []byte{0, 255, 7}},
		{name: "data and eof in one read do not trigger another read", source: func() io.Reader { return &terminalCopyReader{data: []byte{0, 255}, err: io.EOF} }, maximum: 2, want: []byte{0, 255}},
		{name: "limited reader larger than actual source is not overflow", source: func() io.Reader { return io.LimitReader(bytes.NewReader([]byte{0, 255}), 3) }, maximum: 2, want: []byte{0, 255}},
		{name: "limited reader's own end is the supplied source end", source: func() io.Reader { return io.LimitReader(bytes.NewReader([]byte{0, 255, 7}), 2) }, maximum: 2, want: []byte{0, 255}},
		{name: "section larger than backing reader is not overflow", source: func() io.Reader { return io.NewSectionReader(bytes.NewReader([]byte{0, 255}), 0, 3) }, maximum: 2, want: []byte{0, 255}},
		{name: "section ending before backing reader limits the supplied source", source: func() io.Reader { return io.NewSectionReader(bytes.NewReader([]byte{0, 255, 7}), 0, 2) }, maximum: 2, want: []byte{0, 255}},
		{name: "false zero Len cannot hide an extra byte", source: func() io.Reader { return misleadingLengthReader{Reader: bytes.NewReader([]byte{0, 255, 7})} }, maximum: 2, want: []byte{0, 255}, wantErr: core.ErrFilestoreSize},
		{name: "false positive Len cannot invent an extra byte", source: func() io.Reader { return misleadingLengthReader{Reader: bytes.NewReader([]byte{0, 255}), remaining: 1} }, maximum: 2, want: []byte{0, 255}},
		{name: "negative unrelated Len cannot invalidate a conforming Reader", source: func() io.Reader {
			return misleadingLengthReader{Reader: bytes.NewReader([]byte{0, 255}), remaining: -1}
		}, maximum: 2, want: []byte{0, 255}},
		{name: "native error before data retains typed refusal", source: func() io.Reader { return iotest.ErrReader(fs.ErrPermission) }, maximum: 2, wantErr: core.ErrFilestoreSource, wantNative: fs.ErrPermission},
		{name: "native error with data preserves consumed prefix", source: func() io.Reader { return &terminalCopyReader{data: []byte{0}, err: fs.ErrPermission} }, maximum: 2, want: []byte{0}, wantErr: core.ErrFilestoreSource, wantNative: fs.ErrPermission},
		{name: "joined eof is not clean eof before ceiling", source: func() io.Reader {
			return &terminalCopyReader{data: []byte{0}, err: errors.Join(io.EOF, fs.ErrPermission)}
		}, maximum: 2, want: []byte{0}, wantErr: core.ErrFilestoreSource, wantNative: fs.ErrPermission},
		{name: "joined eof is not clean eof at ceiling", source: func() io.Reader {
			return &terminalCopyReader{data: []byte{0, 255}, err: errors.Join(io.EOF, fs.ErrPermission)}
		}, maximum: 2, want: []byte{0, 255}, wantErr: core.ErrFilestoreSource, wantNative: fs.ErrPermission},
		{name: "probe native failure cannot become clean eof", source: func() io.Reader {
			return io.MultiReader(bytes.NewReader([]byte{0, 255}), iotest.ErrReader(errors.Join(io.EOF, fs.ErrPermission)))
		}, maximum: 2, want: []byte{0, 255}, wantErr: core.ErrFilestoreSource, wantNative: fs.ErrPermission},
		{name: "overflow and native probe failure are both retained", source: func() io.Reader {
			return io.MultiReader(bytes.NewReader([]byte{0, 255}), &terminalCopyReader{data: []byte{7}, err: fs.ErrPermission})
		}, maximum: 2, want: []byte{0, 255}, wantErr: core.ErrFilestoreSize, wantNative: fs.ErrPermission},
		{name: "negative reader count retains native error and writes nothing", source: func() io.Reader { return &terminalCopyReader{countAdjustment: -1, err: fs.ErrPermission} }, maximum: 2, wantErr: core.ErrFilestoreSource, wantNative: fs.ErrPermission},
		{name: "excessive reader count retains native error and writes nothing", source: func() io.Reader { return &terminalCopyReader{countAdjustment: 1, err: fs.ErrPermission} }, maximum: 2, wantErr: core.ErrFilestoreSource, wantNative: fs.ErrPermission},
		{name: "one below no progress ceiling can finish empty", source: func() io.Reader { return &stalledCopyReader{empty: core.ReaderConsecutiveEmptyReadMaximum - 1} }, maximum: 2},
		{name: "exact no progress ceiling is refused", source: func() io.Reader { return &stalledCopyReader{empty: core.ReaderConsecutiveEmptyReadMaximum} }, maximum: 2, wantErr: core.ErrFilestoreSource, wantNative: io.ErrNoProgress},
		{name: "one above no progress ceiling is bounded", source: func() io.Reader { return &stalledCopyReader{empty: core.ReaderConsecutiveEmptyReadMaximum + 1} }, maximum: 2, wantErr: core.ErrFilestoreSource, wantNative: io.ErrNoProgress},
		{name: "probe tolerates one below no progress ceiling", source: func() io.Reader {
			return &stalledCopyReader{data: []byte{0, 255}, empty: core.ReaderConsecutiveEmptyReadMaximum - 1}
		}, maximum: 2, want: []byte{0, 255}},
		{name: "probe refuses exact no progress ceiling", source: func() io.Reader {
			return &stalledCopyReader{data: []byte{0, 255}, empty: core.ReaderConsecutiveEmptyReadMaximum}
		}, maximum: 2, want: []byte{0, 255}, wantErr: core.ErrFilestoreSource, wantNative: io.ErrNoProgress},
		{name: "known empty observation does not invent bytes", source: func() io.Reader { return bytes.NewReader(nil) }, maximum: 2, extentKnown: true},
		{name: "known extent shrinking by one is unexpected eof", source: func() io.Reader { return bytes.NewReader([]byte{0}) }, maximum: 2, known: 2, extentKnown: true, want: []byte{0}, wantErr: core.ErrFilestoreSource, wantNative: io.ErrUnexpectedEOF},
		{name: "known exact extent still proves actual eof", source: func() io.Reader { return copyReaderOnly{bytes.NewReader([]byte{0, 255})} }, maximum: 2, known: 2, extentKnown: true, want: []byte{0, 255}},
		{name: "known extent growing below ceiling returns observed bytes", source: func() io.Reader { return bytes.NewReader([]byte{0, 255}) }, maximum: 3, known: 1, extentKnown: true, want: []byte{0, 255}},
		{name: "known extent growing past ceiling cannot widen it", source: func() io.Reader { return bytes.NewReader([]byte{0, 255, 7}) }, maximum: 2, known: 2, extentKnown: true, want: []byte{0, 255}, wantErr: core.ErrFilestoreSize},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			maximum, err := core.NewByteCount(tc.maximum)
			if err != nil {
				t.Fatal(err)
			}
			source := tc.source()
			var destination bytes.Buffer
			got, gotErr := copyBounded(boundedCopyRequest{ctx: t.Context(), source: source, destination: &destination, maximum: maximum, kind: streamDestinationCaller, knownExtent: tc.known, extentKnown: tc.extentKnown})
			if (gotErr == nil) != (tc.wantErr == nil) || tc.wantErr != nil && !errors.Is(gotErr, tc.wantErr) || tc.wantNative != nil && !errors.Is(gotErr, tc.wantNative) {
				t.Fatalf("copy = (%d,%v), want (%d,%v,%v)", got.Uint64(), gotErr, len(tc.want), tc.wantErr, tc.wantNative)
			}
			if got.Uint64() != uint64(len(tc.want)) || !bytes.Equal(destination.Bytes(), tc.want) {
				t.Fatalf("receipt/bytes = (%d,%v), want (%d,%v)", got.Uint64(), destination.Bytes(), len(tc.want), tc.want)
			}
			if stalled, ok := source.(*stalledCopyReader); ok {
				wantReads := core.ReaderConsecutiveEmptyReadMaximum
				if len(tc.want) > 0 {
					wantReads++
				}
				if stalled.reads != wantReads {
					t.Fatalf("reads = %d, want %d including exact progress threshold", stalled.reads, wantReads)
				}
			}
		})
	}
}

type countedCopyWriter struct {
	accepted        bytes.Buffer
	maximum         int
	err             error
	countAdjustment int
	writes          int
}

func (w *countedCopyWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.countAdjustment < 0 {
		return -1, w.err
	}
	if w.countAdjustment > 0 {
		return len(p) + 1, w.err
	}
	n, _ := w.accepted.Write(p[:min(len(p), w.maximum)])
	return n, w.err
}

func TestBoundedCopyGoWriterAccountingLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		payload    []byte
		maximum    int
		adjustment int
		cause      error
		wantCount  uint64
		wantErr    error
		wantWrites int
	}{
		{name: "empty source never calls destination", maximum: 2},
		{name: "full write acknowledges every binary byte", payload: []byte{0, 255}, maximum: 2, wantCount: 2, wantWrites: 1},
		{name: "short nil write is not retried", payload: []byte{0, 255}, maximum: 1, wantCount: 1, wantErr: io.ErrShortWrite, wantWrites: 1},
		{name: "zero nil write is not retried", payload: []byte{0, 255}, wantErr: io.ErrShortWrite, wantWrites: 1},
		{name: "partial native failure retains its exact prefix", payload: []byte{0, 255}, maximum: 1, cause: fs.ErrPermission, wantCount: 1, wantErr: fs.ErrPermission, wantWrites: 1},
		{name: "full native failure is not success", payload: []byte{0, 255}, maximum: 2, cause: fs.ErrPermission, wantCount: 2, wantErr: fs.ErrPermission, wantWrites: 1},
		{name: "zero native failure is not replaced by short write", payload: []byte{0, 255}, cause: fs.ErrPermission, wantErr: fs.ErrPermission, wantWrites: 1},
		{name: "negative nil count never enters receipt", payload: []byte{0, 255}, adjustment: -1, wantErr: io.ErrShortWrite, wantWrites: 1},
		{name: "excessive nil count never enters receipt", payload: []byte{0, 255}, adjustment: 1, wantErr: io.ErrShortWrite, wantWrites: 1},
		{name: "negative count retains native cause", payload: []byte{0, 255}, adjustment: -1, cause: fs.ErrPermission, wantErr: fs.ErrPermission, wantWrites: 1},
		{name: "excessive count retains native cause", payload: []byte{0, 255}, adjustment: 1, cause: fs.ErrPermission, wantErr: fs.ErrPermission, wantWrites: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			maximum, err := core.NewByteCount(2)
			if err != nil {
				t.Fatal(err)
			}
			destination := countedCopyWriter{maximum: tc.maximum, err: tc.cause, countAdjustment: tc.adjustment}
			got, gotErr := copyBounded(boundedCopyRequest{ctx: t.Context(), source: bytes.NewReader(tc.payload), destination: &destination, maximum: maximum, kind: streamDestinationCaller})
			if (gotErr == nil) != (tc.wantErr == nil) || tc.wantErr != nil && (!errors.Is(gotErr, tc.wantErr) || !errors.Is(gotErr, core.ErrFilestoreDestination)) {
				t.Fatalf("error = %v, want destination/%v", gotErr, tc.wantErr)
			}
			if got.Uint64() != tc.wantCount || !bytes.Equal(destination.accepted.Bytes(), tc.payload[:tc.wantCount]) || destination.writes != tc.wantWrites {
				t.Fatalf("receipt/prefix/writes = (%d,%v,%d), want (%d,%v,%d)", got.Uint64(), destination.accepted.Bytes(), destination.writes, tc.wantCount, tc.payload[:tc.wantCount], tc.wantWrites)
			}
			if tc.adjustment == 0 {
				native := countedCopyWriter{maximum: tc.maximum, err: tc.cause}
				nativeCount, nativeErr := io.Copy(&native, copyReaderOnly{bytes.NewReader(tc.payload)})
				if uint64(nativeCount) != got.Uint64() || native.writes != destination.writes || !bytes.Equal(native.accepted.Bytes(), destination.accepted.Bytes()) || !errors.Is(gotErr, nativeErr) {
					t.Fatalf("Go Copy = (%d,%v,%d), primitive = (%d,%v,%d)", nativeCount, nativeErr, native.writes, got.Uint64(), gotErr, destination.writes)
				}
			}
		})
	}
}
