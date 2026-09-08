package exchange

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type delayedProbeReader struct {
	emptyReads int
	reads      int
}

type emptyForeverReader struct{}

func (emptyForeverReader) Read([]byte) (int, error) { return 0, nil }

func (r *delayedProbeReader) Read(buffer []byte) (int, error) {
	if r.reads < r.emptyReads {
		r.reads++
		return 0, nil
	}
	buffer[0] = 'x'
	return 1, nil
}

func TestDownloadOverflowProbeDoesNotConfuseAStallWithEOF(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		want       error
		name       string
		emptyReads int
	}{
		{
			name:       "one transient empty read still discovers overflow",
			emptyReads: 1,
			want:       core.ErrExchangeBodyLimit,
		},
		{
			name:       "last admitted empty read still discovers overflow",
			emptyReads: core.ReaderConsecutiveEmptyReadMaximum - 1,
			want:       core.ErrExchangeBodyLimit,
		},
		{
			name:       "the shared empty-read ceiling refuses no progress",
			emptyReads: core.ReaderConsecutiveEmptyReadMaximum,
			want:       io.ErrNoProgress,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			reader := &delayedProbeReader{emptyReads: testCase.emptyReads}
			written, gotErr := probeDownloadEnd(downloadCopyRequest{
				context: context.Background(), source: reader,
			}, 7)
			if written != 7 || !errors.Is(gotErr, testCase.want) {
				t.Fatalf("probeDownloadEnd() = (%d, %v), want (7, %v)",
					written, gotErr, testCase.want)
			}
		})
	}
}

func TestDownloadTransferRefusesAnUnendingEmptyReader(t *testing.T) {
	t.Parallel()

	limit, err := core.NewByteCount(1)
	if err != nil {
		t.Fatalf("core.NewByteCount(1) error = %v, want nil", err)
	}
	written, gotErr := copyDownload(downloadCopyRequest{
		context: context.Background(), source: emptyForeverReader{},
		destination: io.Discard, limit: limit,
	})
	if written != 0 || !errors.Is(gotErr, io.ErrNoProgress) {
		t.Fatalf("copyDownload(empty reader) = (%d, %v), want (0, %v)",
			written, gotErr, io.ErrNoProgress)
	}
}

// Both zero-extent probes must distinguish actual EOF from bounded stalls.
// The shared row domain is finite: EOF, a byte, and the empty-read threshold.
func TestZeroExtentBoundariesDoNotConfuseAStallWithEOF(t *testing.T) {
	t.Parallel()
	doors := []struct {
		name     string
		probe    func(context.Context, io.Reader) error
		boundary error
		overflow error
	}{
		{name: "request body absence", boundary: core.ErrExchangeRequest, overflow: core.ErrExchangeContract, probe: func(ctx context.Context, r io.Reader) error {
			return refuseRequestBody((&http.Request{Body: io.NopCloser(r)}).WithContext(ctx))
		}},
		{name: "response zero extent", boundary: core.ErrExchangeResponse, overflow: core.ErrExchangeBodyLimit, probe: probeEmptyResponseSource},
	}
	cases := []struct {
		name         string
		emptyReads   int
		eof          bool
		wantOverflow bool
		wantNative   error
	}{
		{name: "actual EOF creates no body evidence", eof: true},
		{name: "immediate byte contradicts absence", wantOverflow: true},
		{name: "transient stall cannot impersonate EOF", emptyReads: 1, wantOverflow: true},
		{name: "last tolerated stall still discovers overflow", emptyReads: core.ReaderConsecutiveEmptyReadMaximum - 1, wantOverflow: true},
		{name: "exact empty-read ceiling refuses no progress", emptyReads: core.ReaderConsecutiveEmptyReadMaximum, wantNative: io.ErrNoProgress},
		{name: "above empty-read ceiling cannot spend another read", emptyReads: core.ReaderConsecutiveEmptyReadMaximum + 1, wantNative: io.ErrNoProgress},
	}
	for _, door := range doors {
		t.Run(door.name, func(t *testing.T) {
			t.Parallel()
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					stalled := &delayedProbeReader{emptyReads: tc.emptyReads}
					var source io.Reader = stalled
					if tc.eof {
						source = bytes.NewReader(nil)
					}
					gotErr := door.probe(t.Context(), source)
					wantErr := tc.wantNative
					if tc.wantOverflow {
						wantErr = door.overflow
					}
					if !errors.Is(gotErr, wantErr) || errors.Is(gotErr, door.boundary) != !tc.eof || errors.Is(gotErr, io.ErrNoProgress) != (tc.wantNative == io.ErrNoProgress) {
						t.Fatalf("zero extent error = %v, want %v with exact boundary and no-progress identities", gotErr, wantErr)
					}
					wantStalls := min(tc.emptyReads, core.ReaderConsecutiveEmptyReadMaximum)
					if stalled.reads != wantStalls {
						t.Fatalf("empty reads = %d, want %d", stalled.reads, wantStalls)
					}
				})
			}
		})
	}
}

// retainingWriter is a plain io.Writer. It deliberately implements neither
// io.ReaderFrom nor any other fast path, so Go's copy loop must reach Write.
type retainingWriter struct {
	written bytes.Buffer
}

func (w *retainingWriter) Write(payload []byte) (int, error) {
	return w.written.Write(payload)
}

// copyDispatchSource deliberately offers WriterTo. The bounded production
// reader must prevent that method from bypassing accounting and the limit.
type copyDispatchSource struct {
	source        *bytes.Reader
	readBytes     int
	writerToCalls int
}

func (r *copyDispatchSource) Read(p []byte) (int, error) {
	n, err := r.source.Read(p)
	r.readBytes += n
	return n, err
}
func (r *copyDispatchSource) WriteTo(w io.Writer) (int64, error) {
	r.writerToCalls++
	return r.source.WriteTo(w)
}

type copyReaderFromDestination struct {
	destination   *retainingWriter
	readFromErr   error
	readFromCalls int
	writeCalls    int
}

func (w *copyReaderFromDestination) Write(p []byte) (int, error) {
	w.writeCalls++
	return w.destination.Write(p)
}
func (w *copyReaderFromDestination) ReadFrom(r io.Reader) (int64, error) {
	w.readFromCalls++
	if w.readFromErr != nil {
		return 0, w.readFromErr
	}
	return io.Copy(&w.destination.written, r)
}

// This ratchet drives Exchange's actual copy, including its bounds and observations,
// while proving that Go still dispatches to ReaderFrom. Every destination and
// source is owned by its table row.
func TestCopyDownloadGoDispatchLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name              string
		payload           string
		limit             uint64
		readerFrom        bool
		readFromErr       error
		cancelled         bool
		wantErr           error
		wantBytes         uint64
		wantBody          string
		wantReadBytes     int
		wantReadFromCalls int
	}{
		{name: "positive plain writer receives exact binary extent", payload: "\x00\xff", limit: 2, wantBytes: 2, wantBody: "\x00\xff", wantReadBytes: 2},
		{name: "negative source WriterTo cannot bypass a plain destination limit", payload: "abc", limit: 2, wantErr: core.ErrExchangeBodyLimit, wantBytes: 2, wantBody: "ab", wantReadBytes: 3},
		{name: "positive Go ReaderFrom remains the chosen execution path", payload: "\x00\xff", limit: 2, readerFrom: true, wantBytes: 2, wantBody: "\x00\xff", wantReadBytes: 2, wantReadFromCalls: 1},
		{name: "negative ReaderFrom cannot consume beyond the admitted extent and probe", payload: "abc", limit: 2, readerFrom: true, wantErr: core.ErrExchangeBodyLimit, wantBytes: 2, wantBody: "ab", wantReadBytes: 3, wantReadFromCalls: 1},
		{name: "negative ReaderFrom refusal cannot fall back to Write", payload: "ab", limit: 2, readerFrom: true, readFromErr: io.ErrClosedPipe, wantErr: io.ErrClosedPipe, wantReadFromCalls: 1},
		{name: "neutral empty plain source creates no byte observation", limit: 1},
		{name: "neutral empty ReaderFrom source keeps Go dispatch and zero bytes", limit: 1, readerFrom: true, wantReadFromCalls: 1},
		{name: "negative cancellation precedes ReaderFrom invocation", payload: "ab", limit: 2, readerFrom: true, cancelled: true, wantErr: context.Canceled},
		{name: "negative absent bound cannot hand off to ReaderFrom", payload: "ab", readerFrom: true, wantErr: core.ErrPrimitiveContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.cancelled {
				cancel()
			}
			source := &copyDispatchSource{source: bytes.NewReader([]byte(tc.payload))}
			storage := &retainingWriter{}
			fast := &copyReaderFromDestination{destination: storage, readFromErr: tc.readFromErr}
			var destination io.Writer = storage
			if tc.readerFrom {
				destination = fast
			}
			var limit core.ByteCount
			if tc.limit != 0 {
				var err error
				limit, err = core.NewByteCount(tc.limit)
				if err != nil {
					t.Fatalf("fixture limit = %v, want nil", err)
				}
			}
			gotBytes, gotErr := copyDownload(downloadCopyRequest{context: ctx, source: source, destination: destination, limit: limit})
			if !errors.Is(gotErr, tc.wantErr) || gotBytes != tc.wantBytes || storage.written.String() != tc.wantBody {
				t.Fatalf("copy bytes/error/body = (%d, %v, %x), want (%d, %v, %x)", gotBytes, gotErr, storage.written.Bytes(), tc.wantBytes, tc.wantErr, tc.wantBody)
			}
			if source.writerToCalls != 0 || source.readBytes != tc.wantReadBytes || fast.readFromCalls != tc.wantReadFromCalls || fast.writeCalls != 0 {
				t.Fatalf("source WriterTo/read bytes/destination ReadFrom/Write = (%d, %d, %d, %d), want (0, %d, %d, 0)", source.writerToCalls, source.readBytes, fast.readFromCalls, fast.writeCalls, tc.wantReadBytes, tc.wantReadFromCalls)
			}
		})
	}
}
