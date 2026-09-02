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

func TestZeroExtentBoundariesDoNotConfuseAStallWithEOF(t *testing.T) {
	t.Parallel()

	t.Run("bodyless request still discovers a body after one stall", func(t *testing.T) {
		t.Parallel()

		request := &http.Request{Body: io.NopCloser(&delayedProbeReader{emptyReads: 1})}
		gotErr := refuseRequestBody(request)
		if !errors.Is(gotErr, core.ErrExchangeRequest) ||
			!errors.Is(gotErr, core.ErrExchangeContract) {
			t.Fatalf("refuseRequestBody(stalled body) error = %v, want %v and %v",
				gotErr, core.ErrExchangeRequest, core.ErrExchangeContract)
		}
	})

	t.Run("bodyless request refuses an unending stall", func(t *testing.T) {
		t.Parallel()

		request := &http.Request{Body: io.NopCloser(emptyForeverReader{})}
		gotErr := refuseRequestBody(request)
		if !errors.Is(gotErr, core.ErrExchangeRequest) ||
			!errors.Is(gotErr, io.ErrNoProgress) {
			t.Fatalf("refuseRequestBody(empty reader) error = %v, want %v and %v",
				gotErr, core.ErrExchangeRequest, io.ErrNoProgress)
		}
	})

	t.Run("zero response extent still discovers a byte after one stall", func(t *testing.T) {
		t.Parallel()

		gotErr := probeEmptyResponseSource(
			context.Background(), &delayedProbeReader{emptyReads: 1},
		)
		if !errors.Is(gotErr, core.ErrExchangeResponse) ||
			!errors.Is(gotErr, core.ErrExchangeBodyLimit) {
			t.Fatalf("probeEmptyResponseSource(stalled source) error = %v, want %v and %v",
				gotErr, core.ErrExchangeResponse, core.ErrExchangeBodyLimit)
		}
	})

	t.Run("zero response extent refuses an unending stall", func(t *testing.T) {
		t.Parallel()

		gotErr := probeEmptyResponseSource(context.Background(), emptyForeverReader{})
		if !errors.Is(gotErr, core.ErrExchangeResponse) ||
			!errors.Is(gotErr, io.ErrNoProgress) {
			t.Fatalf("probeEmptyResponseSource(empty reader) error = %v, want %v and %v",
				gotErr, core.ErrExchangeResponse, io.ErrNoProgress)
		}
	})
}

// retainingWriter is a plain io.Writer. It deliberately implements neither
// io.ReaderFrom nor any other fast path, so io.CopyBuffer must route the copy
// through the supplied buffer.
type retainingWriter struct {
	written bytes.Buffer
}

func (w *retainingWriter) Write(payload []byte) (int, error) {
	return w.written.Write(payload)
}

// TestCopyBufferUsesTheSuppliedBufferOnlyForPlainWriters pins the documented
// io.CopyBuffer dispatch that decides whether the supplied buffer participates at
// all. io.CopyBuffer states that the buffer is unused when the destination
// implements io.ReaderFrom, so a destination such as io.Discard or bytes.Buffer
// exercises none of the pooled extent. Any measurement that claims to show
// buffer reuse must use a plain writer.
func TestCopyBufferUsesTheSuppliedBufferOnlyForPlainWriters(t *testing.T) {
	t.Parallel()

	payload := bytes.Repeat([]byte("stream-proof-"), 4096)
	cases := []struct {
		destination func() io.Writer
		name        string
		wantUsed    bool
	}{
		{
			name:        "plain writer routes the copy through the buffer",
			destination: func() io.Writer { return &retainingWriter{} },
			wantUsed:    true,
		},
		{
			name:        "io.Discard destination never touches the buffer",
			destination: func() io.Writer { return io.Discard },
		},
		{
			name:        "bytes.Buffer destination never touches the buffer",
			destination: func() io.Writer { return bytes.NewBuffer(nil) },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			buffer := make([]byte, TransferBufferBytes)
			sentinel := bytes.Repeat([]byte{0xA5}, TransferBufferBytes)
			copy(buffer, sentinel)
			limited := &io.LimitedReader{
				R: bytes.NewReader(payload),
				N: int64(len(payload)),
			}

			count, gotErr := io.CopyBuffer(tc.destination(), limited, buffer)
			if gotErr != nil || count != int64(len(payload)) {
				t.Fatalf(
					"io.CopyBuffer() = (%d, %v), want (%d, nil)",
					count,
					gotErr,
					len(payload),
				)
			}
			gotUsed := !bytes.Equal(buffer, sentinel)
			if gotUsed != tc.wantUsed {
				t.Fatalf(
					"io.CopyBuffer() used the supplied buffer = %t, want %t",
					gotUsed,
					tc.wantUsed,
				)
			}
		})
	}
}
