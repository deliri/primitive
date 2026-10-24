package gcsobjects

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// streamFailureWriter injects an io.Writer contract outcome while retaining
// the bytes it actually accepted. This local adapter ratchet does not claim
// that a real provider produced a short native write.
type streamFailureWriter struct {
	destination bytes.Buffer
	count       int
	err         error
}

func (w *streamFailureWriter) Write(data []byte) (int, error) {
	if w.count >= 0 && w.count <= len(data) {
		_, _ = w.destination.Write(data[:w.count])
	}
	return w.count, w.err
}

func TestGCSStreamDestinationWriteFailureLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		count         int
		cause         error
		wantN         int
		wantBytes     string
		wantRemaining uint64
		wantErr       error
	}{
		{name: "complete write consumes its exact extent", count: 2, wantN: 2, wantBytes: "xy"},
		{name: "negative count refuses without underflow", count: -1, wantRemaining: 2, wantErr: core.ErrObjectStoreDestination},
		{name: "invalid count still preserves native failure", count: -1, cause: io.ErrClosedPipe, wantRemaining: 2, wantErr: io.ErrClosedPipe},
		{name: "oversized count refuses without underflow", count: 3, wantRemaining: 2, wantErr: core.ErrObjectStoreDestination},
		{name: "short nil-error write preserves prefix and refuses", count: 1, wantN: 1, wantBytes: "x", wantRemaining: 1, wantErr: io.ErrShortWrite},
		{name: "short failed write keeps native cause and prefix", count: 1, cause: io.ErrClosedPipe, wantN: 1, wantBytes: "x", wantRemaining: 1, wantErr: io.ErrClosedPipe},
		{name: "full failed write accounts progress without success", count: 2, cause: io.ErrClosedPipe, wantN: 2, wantBytes: "xy", wantErr: io.ErrClosedPipe},
		{name: "zero failed write creates no progress", cause: io.ErrClosedPipe, wantRemaining: 2, wantErr: io.ErrClosedPipe},
		{name: "zero nil-error write is not successful progress", wantRemaining: 2, wantErr: io.ErrShortWrite},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			destination := streamFailureWriter{count: tc.count, err: tc.cause}
			limited := gcsStreamDestination{destination: &destination, remaining: 2}
			n, err := limited.Write([]byte("xy"))
			if n != tc.wantN || destination.destination.String() != tc.wantBytes || limited.remaining != tc.wantRemaining {
				t.Fatalf("write=%d/%v accepted=%q remaining=%d, want %d, %q, %d", n, err, destination.destination.String(), limited.remaining, tc.wantN, tc.wantBytes, tc.wantRemaining)
			}
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("complete native write error=%v, want nil", err)
				}
			} else if !errors.Is(err, tc.wantErr) || !errors.Is(err, core.ErrObjectStoreDestination) {
				t.Fatalf("failed write error=%v, want destination identity and %v", err, tc.wantErr)
			}
		})
	}
}
