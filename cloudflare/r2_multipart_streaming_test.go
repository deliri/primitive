package cloudflare

import (
	"context"
	"io"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// The second source read cannot proceed until the actual TLS peer receives a
// prefix. Buffering the source before transport therefore cannot pass. All
// payloads are small, generated on demand, and discarded by the receiving peer.
func TestR2MultipartUploadStreamsBeforeSourceFinishes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		bytes int64
	}{
		{name: "tail crosses one copy window", bytes: exchange.TransferBufferBytes + 1},
		{name: "many windows retain the same read bound", bytes: 2*1024*1024 + 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ack := make(chan struct{})
			var requests atomic.Int32
			var received atomic.Int64
			peer := testExchange(t, func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				var prefix [1]byte
				if _, err := io.ReadFull(r.Body, prefix[:]); err != nil {
					t.Error(err)
					return
				}
				close(ack)
				n, err := io.Copy(io.Discard, r.Body)
				received.Store(n + 1)
				if err != nil || n+1 != tc.bytes || prefix[0] != 'x' || r.ContentLength != tc.bytes {
					t.Errorf("wire=%d/%d/%v, want exact %d", n+1, r.ContentLength, err, tc.bytes)
				}
				w.Header().Set("ETag", `"streamed-part"`)
			})
			client, err := NewR2Client(peer)
			if err != nil {
				t.Fatal(err)
			}
			server := testR2Server(t, R2JurisdictionDefault)
			grant, err := server.PresignMultipart(t.Context(), multipartIntent(t, R2MultipartPart))
			if err != nil {
				t.Fatal(err)
			}
			length, err := core.NewByteLength(uint64(tc.bytes))
			if err != nil {
				t.Fatal(err)
			}
			policy := testPolicy()
			timeout, err := policy.OperationTimeout.Stdlib()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), timeout)
			defer cancel()
			source := multipartAcknowledgedSource{ctx: ctx, acknowledged: ack}
			source.remaining.Store(tc.bytes)
			got, err := client.UploadPart(ctx, grant, &source, length, policy)
			if err != nil || got != (R2CompletedPart{PartNumber: 1, ETag: `"streamed-part"`}) {
				t.Fatalf("receipt=%+v/%v, want exact provider receipt", got, err)
			}
			if source.remaining.Load() != 0 || received.Load() != tc.bytes || requests.Load() != 1 || source.calls.Load() < 2 || source.maximumRead.Load() > exchange.TransferBufferBytes {
				t.Fatalf("remaining=%d received=%d attempts=%d reads=%d max_read=%d, want complete single transfer in bounded windows", source.remaining.Load(), received.Load(), requests.Load(), source.calls.Load(), source.maximumRead.Load())
			}
		})
	}
}

type multipartAcknowledgedSource struct {
	ctx          context.Context
	acknowledged <-chan struct{}
	remaining    atomic.Int64
	maximumRead  atomic.Int64
	calls        atomic.Int32
}

func (r *multipartAcknowledgedSource) Read(p []byte) (int, error) {
	call := r.calls.Add(1)
	r.maximumRead.Store(max(r.maximumRead.Load(), int64(len(p))))
	if call > 1 {
		select {
		case <-r.acknowledged:
		case <-r.ctx.Done():
			return 0, r.ctx.Err()
		}
	}
	if r.remaining.Load() == 0 {
		return 0, io.EOF
	}
	n := int(min(int64(len(p)), r.remaining.Load()))
	for i := range p[:n] {
		p[i] = 'x'
	}
	r.remaining.Add(-int64(n))
	return n, nil
}
