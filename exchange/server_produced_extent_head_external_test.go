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

type producedHeadWriter struct {
	http.ResponseWriter
	commits int
	writes  int
}

func (w *producedHeadWriter) WriteHeader(status int) {
	w.commits++
	w.ResponseWriter.WriteHeader(status)
}

func (w *producedHeadWriter) Write(data []byte) (int, error) {
	w.writes++
	return w.ResponseWriter.Write(data)
}

func (w *producedHeadWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// A zero-byte response has no final byte to withhold. Its caller must handle
// the returned error before returning control to Go's implicit-200 handler tail.
func TestWriteProducedZeroExtentFailureUsesCallerStatus(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	length, err := core.NewByteLength(0)
	if err != nil {
		t.Fatal(err)
	}
	var failureStatus core.HTTPStatusCode
	if err := failureStatus.AdmitInt(http.StatusInternalServerError); err != nil {
		t.Fatal(err)
	}
	served := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call, err := exchange.NewSocketServerCall(w, r)
		if err == nil {
			err = exchange.WriteProduced(exchange.ProducedWriteCall{Call: call, Response: exchange.ServerProducedResponse{Status: core.HTTPStatusOK(), ContentType: core.HTTPMediaTypeOctetStream(), ContentLength: &length, Produce: func(context.Context, io.Writer) error {
				reader, writer := io.Pipe()
				readClose := reader.Close()
				_, writeErr := writer.Write([]byte{1})
				return errors.Join(readClose, writeErr, writer.Close())
			}}})
			if err != nil {
				// Exchange owns the existing standard HTTP failure operation;
				// response policy remains explicit at the caller.
				failureErr := exchange.Error(call, exchange.ServerErrorResponse{Status: failureStatus, Message: "producer failed"})
				err = errors.Join(err, failureErr)
			}
		}
		served <- err
	}))
	defer server.Close()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	serverErr := <-served
	if response.StatusCode != http.StatusInternalServerError || string(body) != "producer failed\n" || readErr != nil || closeErr != nil || !errors.Is(serverErr, io.ErrClosedPipe) || !errors.Is(serverErr, core.ErrExchangeWrite) || !errors.Is(serverErr, core.ErrExchangeResponse) {
		t.Fatalf("zero producer=(%v), response=(%d,%q,%v,%v), want retained closed pipe and caller-selected failure status", serverErr, response.StatusCode, body, readErr, closeErr)
	}
}

func TestWriteProducedExactExtentHEADRefusesBeforeNativeCommit(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	length, err := core.NewByteLength(1)
	if err != nil {
		t.Fatal(err)
	}
	type observation struct {
		err      error
		produced bool
		commits  int
		writes   int
	}
	served := make(chan observation, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writer := &producedHeadWriter{ResponseWriter: w}
		call, err := exchange.NewSocketServerCall(writer, r)
		produced := false
		if err == nil {
			err = exchange.WriteProduced(exchange.ProducedWriteCall{Call: call, Response: exchange.ServerProducedResponse{
				Status: core.HTTPStatusOK(), ContentType: core.HTTPMediaTypeOctetStream(), ContentLength: &length,
				Produce: func(_ context.Context, out io.Writer) error {
					produced = true
					_, err := out.Write([]byte{1})
					return err
				},
			}})
		}
		served <- observation{err: err, produced: produced, commits: writer.commits, writes: writer.writes}
	}))
	defer server.Close()
	request, err := http.NewRequestWithContext(ctx, http.MethodHead, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
	got := <-served
	if !errors.Is(got.err, core.ErrExchangeContract) || !errors.Is(got.err, core.ErrExchangeResponse) || got.produced || got.commits != 0 || got.writes != 0 {
		t.Fatalf("HEAD produced-response=(%v,produced=%t,commits=%d,writes=%d), want typed refusal before producer or native commit", got.err, got.produced, got.commits, got.writes)
	}
}
