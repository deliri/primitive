package exchange_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestWriteProducedDeliversBeforeProducerCompletes(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	ack := make(chan struct{})
	done := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call, err := exchange.NewSocketServerCall(w, r)
		if err != nil {
			done <- err
			return
		}
		done <- exchange.WriteProduced(exchange.ProducedWriteCall{Call: call, Response: exchange.ServerProducedResponse{Status: core.HTTPStatusOK(), ContentType: core.HTTPMediaTypeJSON(), Produce: func(ctx context.Context, out io.Writer) error {
			if _, err := out.Write([]byte("progress\n")); err != nil {
				return err
			}
			select {
			case <-ack:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}}})
	}))
	defer server.Close()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, requestErr := server.Client().Do(request)
	var got [9]byte
	var readErr error
	if response != nil {
		_, readErr = io.ReadFull(response.Body, got[:])
	}
	close(ack)
	serverErr := <-done
	if response != nil {
		_ = response.Body.Close()
	}
	if requestErr != nil || readErr != nil || string(got[:]) != "progress\n" || serverErr != nil {
		t.Fatalf("progress=%q, request=%v read=%v server=%v, want progress before acknowledgement and nil errors", got, requestErr, readErr, serverErr)
	}
}

type flushFailureWriter struct{ *httptest.ResponseRecorder }

func (flushFailureWriter) FlushError() error { return io.ErrClosedPipe }

func TestWriteProducedRetainsFlushFailureAfterWrittenBytes(t *testing.T) {
	t.Parallel()
	writer := flushFailureWriter{httptest.NewRecorder()}
	call, err := exchange.NewSocketServerCall(writer, httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	err = exchange.WriteProduced(exchange.ProducedWriteCall{Call: call, Response: exchange.ServerProducedResponse{Status: core.HTTPStatusOK(), ContentType: core.HTTPMediaTypeJSON(), Produce: func(_ context.Context, out io.Writer) error {
		_, err := out.Write([]byte("written"))
		return err
	}}})
	if !errors.Is(err, core.ErrExchangeWrite) || !errors.Is(err, io.ErrClosedPipe) || writer.Body.String() != "written" {
		t.Fatalf("flush failure=%v, bytes=%q, want typed write failure preserving closed pipe and written bytes", err, writer.Body.String())
	}
}
