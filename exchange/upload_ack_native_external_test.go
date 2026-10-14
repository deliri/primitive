package exchange_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type uploadAckNativeTransport struct {
	transport    *http.Transport
	read, closed chan struct{}
}

func (t uploadAckNativeTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := t.transport.RoundTrip(request)
	if err == nil {
		response.Body = uploadAckNativeBody{ReadCloser: response.Body, read: t.read, closed: t.closed}
	}
	return response, err
}

type uploadAckNativeBody struct {
	io.ReadCloser
	read, closed chan struct{}
}

func (b uploadAckNativeBody) Read(p []byte) (int, error) {
	select {
	case b.read <- struct{}{}:
	default:
	}
	return b.ReadCloser.Read(p)
}
func (b uploadAckNativeBody) Close() error {
	err := b.ReadCloser.Close()
	select {
	case b.closed <- struct{}{}:
	default:
	}
	return err
}

func TestUploadNativeAcknowledgmentClosesWithoutReadingUnfinishedBody(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-request.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	transport := http.DefaultTransport.(*http.Transport).Clone()
	defer transport.CloseIdleConnections()
	read, closed := make(chan struct{}, 1), make(chan struct{}, 1)
	client := mustExchangeClient(t, &http.Client{Transport: uploadAckNativeTransport{transport: transport, read: read, closed: closed}})
	length := mustByteLength(t, 0)
	request := exchange.UploadRequest{Target: mustEndpoint(t, server.URL), Source: bytes.NewReader(nil), ContentLength: &length, ContentType: core.HTTPMediaTypeOctetStream(), ExpectedStatus: core.HTTPStatusOK(), Semantics: exchange.RequestSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt}}
	type result struct {
		response exchange.StreamResponse
		err      error
	}
	finished := make(chan result, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		response, err := exchange.Upload(exchange.UploadCall{Context: ctx, Client: client, Request: request, Policy: exchange.StreamPolicy{Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject}}})
		finished <- result{response: response, err: err}
	}()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Error("owned upload did not exit after cancellation")
		}
	}()
	select {
	case <-read:
		cancel()
		t.Error("Upload read an unneeded acknowledgment body; want close without response draining")
	case <-closed:
		select {
		case got := <-finished:
			if got.err != nil || got.response.Metadata.Attempts != 1 || got.response.Metadata.Status != core.HTTPStatusOK() || got.response.Metadata.Bytes.Uint64() != 0 {
				t.Errorf("native acknowledgment = %+v/%v, want one completed empty observation", got.response, got.err)
			}
		case <-time.After(10 * time.Second):
			t.Error("Upload closed its body but did not return")
		}
	case <-time.After(10 * time.Second):
		cancel()
		t.Error("native upload reached neither an acknowledgment read nor close")
	}
}
