package exchange_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestWriteProducedStreamsPastDocumentCeilingAndRetainsFailures(t *testing.T) {
	t.Parallel()
	const chunks = 256
	const chunkBytes = 8192
	tests := []struct {
		name      string
		producer  func(context.Context, io.Writer) error
		wantError error
		wantBytes int
	}{
		{name: "large incremental response", producer: func(_ context.Context, writer io.Writer) error {
			var chunk [chunkBytes]byte
			for range chunks {
				if _, err := writer.Write(chunk[:]); err != nil {
					return err
				}
			}
			return nil
		}, wantBytes: chunks * chunkBytes},
		{name: "producer refusal after progress", producer: func(_ context.Context, writer io.Writer) error {
			_, _ = writer.Write([]byte("ok"))
			return io.ErrUnexpectedEOF
		}, wantError: io.ErrUnexpectedEOF, wantBytes: 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			call, err := exchange.NewSocketServerCall(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
			if err != nil {
				t.Fatalf("NewSocketServerCall() error = %v, want nil", err)
			}
			err = exchange.WriteProduced(exchange.ProducedWriteCall{Call: call, Response: exchange.ServerProducedResponse{
				Produce: tc.producer, ContentType: core.HTTPMediaTypeJSON(), Status: core.HTTPStatusOK(),
			}})
			if !errors.Is(err, tc.wantError) || recorder.Code != http.StatusOK || recorder.Body.Len() != tc.wantBytes || recorder.Header().Get("Content-Length") != "" {
				t.Fatalf("WriteProduced() = (error %v, status %d, bytes %d, length %q), want (%v, %d, %d, empty)", err, recorder.Code, recorder.Body.Len(), recorder.Header().Get("Content-Length"), tc.wantError, http.StatusOK, tc.wantBytes)
			}
		})
	}
}
