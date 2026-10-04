package cloudflare

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

type prefixAcknowledgingWriter struct {
	acknowledged chan struct{}
	destination  bytes.Buffer
	signaled     bool
}

func (w *prefixAcknowledgingWriter) Write(p []byte) (int, error) {
	n, err := w.destination.Write(p)
	if n > 0 && !w.signaled {
		w.signaled = true
		close(w.acknowledged)
	}
	return n, err
}

func TestR2ReadDeliversPrefixBeforeProviderCanFinish(t *testing.T) {
	t.Parallel()
	ack := make(chan struct{})
	client := testExchange(t, func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.WriteString(w, "prefix"); err != nil {
			t.Error(err)
			return
		}
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Error(err)
			return
		}
		// The provider cannot emit the tail until the public SDK destination
		// observes the prefix. A whole-body buffer deadlocks this real TLS path.
		select {
		case <-ack:
		case <-r.Context().Done():
			return
		case <-t.Context().Done():
			return
		}
		if _, err := io.WriteString(w, "tail"); err != nil {
			t.Error(err)
		}
	})
	server := testR2Server(t, R2JurisdictionDefault)
	grant, err := server.Presign(t.Context(), testR2Intent(t, exchange.MethodGet, "stream"))
	if err != nil {
		t.Fatal(err)
	}
	r2, err := NewR2Client(client)
	if err != nil {
		t.Fatal(err)
	}
	backstop, err := temporal.DurationFromSeconds(10)
	if err != nil {
		t.Fatal(err)
	}
	policy := testPolicy()
	policy.OperationTimeout = backstop
	destination := prefixAcknowledgingWriter{acknowledged: ack}
	result, err := r2.Read(t.Context(), grant, &destination, policy)
	if err != nil || destination.destination.String() != "prefixtail" || result.Metadata.Bytes.Uint64() != 10 || !destination.signaled {
		t.Fatalf("stream=(%+v,%v,%q), want prefix acknowledged before exact tail", result, err, destination.destination.String())
	}
}

func TestR2ReadFailurePreservesPartialExtentLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr   error
		name      string
		body      string
		declared  string
		status    int
		wantBytes uint64
		canceled  bool
	}{
		{name: "complete body has exact acknowledged extent", status: http.StatusOK, body: "prefix", wantBytes: 6},
		{name: "truncated body retains prefix beside typed EOF", status: http.StatusOK, body: "prefix", declared: "7", wantErr: io.ErrUnexpectedEOF, wantBytes: 6},
		{name: "absent object does not become empty success", status: http.StatusNotFound, body: "provider diagnostic", wantErr: core.ErrExchangeResponse},
		{name: "empty object remains zero byte success", status: http.StatusOK},
		{name: "canceled request makes no provider call", status: http.StatusOK, canceled: true, wantErr: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := testExchange(t, func(w http.ResponseWriter, r *http.Request) {
				if tc.declared != "" {
					w.Header().Set("Content-Length", tc.declared)
				}
				w.WriteHeader(tc.status)
				if _, err := io.WriteString(w, tc.body); err != nil {
					t.Error(err)
				}
			})
			server := testR2Server(t, R2JurisdictionDefault)
			grant, err := server.Presign(t.Context(), testR2Intent(t, exchange.MethodGet, "object"))
			if err != nil {
				t.Fatal(err)
			}
			r2, err := NewR2Client(client)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.canceled {
				cancel()
			}
			var destination bytes.Buffer
			got, gotErr := r2.Read(ctx, grant, &destination, testPolicy())
			if tc.status == http.StatusNotFound {
				var statusError exchange.StatusError
				if !errors.As(gotErr, &statusError) {
					t.Fatalf("Read status error = %v, want exchange.StatusError", gotErr)
				}
				code, statusErr := statusError.Status().Int()
				if statusErr != nil || code != tc.status {
					t.Fatalf("Read status = (%d,%v), want %d", code, statusErr, tc.status)
				}
			}
			if !errors.Is(gotErr, tc.wantErr) || got.Metadata.Bytes.Uint64() != tc.wantBytes || uint64(destination.Len()) != tc.wantBytes {
				t.Fatalf("Read=(%+v,%v,%q), want %v/%d acknowledged bytes", got, gotErr, destination.String(), tc.wantErr, tc.wantBytes)
			}
		})
	}
}

type finiteGeneratedReader struct {
	left        int64
	maximumRead int
	calls       int
}

func (r *finiteGeneratedReader) Read(p []byte) (int, error) {
	r.calls++
	if len(p) > r.maximumRead {
		r.maximumRead = len(p)
	}
	if r.left == 0 {
		return 0, io.EOF
	}
	n := int(min(int64(len(p)), r.left))
	for i := range p[:n] {
		p[i] = 'x'
	}
	r.left -= int64(n)
	return n, nil
}

type countingSink struct {
	bytes        int64
	maximumWrite int
}

func (w *countingSink) Write(p []byte) (int, error) {
	w.bytes += int64(len(p))
	w.maximumWrite = max(w.maximumWrite, len(p))
	return len(p), nil
}

func TestMultipartReaderStreamsAcrossManyCopyWindows(t *testing.T) {
	t.Parallel()
	const extent int64 = 33*1024*1024 + 1
	source := finiteGeneratedReader{left: extent}
	sink := countingSink{}
	reader := exactSource{source: &source, remaining: extent}
	n, err := io.Copy(&sink, &reader)
	if err != nil || n != extent || sink.bytes != extent || source.left != 0 || !reader.complete.Load() || source.maximumRead > exchange.TransferBufferBytes || sink.maximumWrite > exchange.TransferBufferBytes {
		t.Fatalf("stream=(n=%d,err=%v,source=%+v,sink=%+v), want exact extent with bounded windows", n, err, source, sink)
	}
}

type rejectingWriter struct {
	remaining int
	bytes     int
}

func (w *rejectingWriter) Write(p []byte) (int, error) {
	n := min(w.remaining, len(p))
	w.remaining -= n
	w.bytes += n
	if n < len(p) {
		return n, io.ErrClosedPipe
	}
	return n, nil
}

func TestAuthenticatedWebhookWriterFailureRetainsOnlyAcknowledgedPrefix(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	secret, err := ParseStreamWebhookSecret([]byte("writer-failure-key"))
	if err != nil {
		t.Fatal(err)
	}
	receiver, err := NewStreamWebhookReceiver(secret)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := receiver.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := secret.Close(); err != nil {
		t.Fatal(err)
	}
	body := []byte(strings.Repeat("x", 100))
	signature := streamTestSignature(t, []byte("writer-failure-key"), webhookTestSeconds, body)
	destination := rejectingWriter{remaining: 7}
	request := testWebhookRequest(t, body, &destination, core.CloudflareStreamSignatureHeader, signature)
	got, err := receiver.Receive(StreamWebhookReceiveRequest{WebhookReceiveRequest: request, Scratch: testScratch(t, directory)})
	if !errors.Is(err, io.ErrClosedPipe) || got.Bytes.Uint64() != 7 || destination.bytes != 7 {
		t.Fatalf("Receive=(%+v,%v,%d), want writer failure with seven acknowledged bytes", got, err, destination.bytes)
	}
}

// HTTP transports can finish a response while their request-body reader exits.
// Completion is published across that ownership boundary and must be race-free.
func TestUploadCompletionObservationIsSafeAcrossTransportOwnership(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	reader := &exactSource{source: strings.NewReader("")}
	start := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		select {
		case <-ctx.Done():
			done <- ctx.Err()
			return
		case <-start:
		}
		var probe [1]byte
		_, err := reader.Read(probe[:])
		done <- err
	}()
	close(start)
	// This observation deliberately occurs before the join; it is permitted
	// to see either state, while the final observation must see completion.
	_ = reader.complete.Load()
	select {
	case err := <-done:
		if !errors.Is(err, io.EOF) || !reader.complete.Load() {
			t.Fatalf("source terminal state = (%v, %v), want EOF and completion", err, reader.complete.Load())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("source worker exit = absent, want joined completion")
	}
}
