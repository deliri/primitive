package exchange_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// The 64KiB bound belongs to this short developer campaign, not the API.
func FuzzWriteProducedExtentSemanticClosure(f *testing.F) {
	for _, seed := range []struct {
		body    []byte
		length  uint16
		split   uint16
		failure bool
		unknown bool
		head    bool
	}{
		{body: []byte{}, length: 0},
		{body: []byte("a"), length: 1},
		{body: []byte("ab"), length: 2, split: 1},
		{body: []byte("ab"), length: 2, failure: true},
		{body: []byte("ab"), length: 1},
		{body: []byte("ab"), length: 3},
		{body: []byte("unframed"), unknown: true},
		{body: []byte("unframed"), unknown: true, failure: true},
		{body: []byte("a"), length: 1, head: true},
		{body: []byte("unframed"), unknown: true, head: true},
	} {
		length, err := core.NewByteLength(uint64(seed.length))
		if err != nil {
			f.Fatal(err)
		}
		request := exchange.ServerProducedResponse{Status: core.HTTPStatusOK(), ContentType: core.HTTPMediaTypeOctetStream(), ContentLength: &length, Produce: func(context.Context, io.Writer) error { return nil }}
		if err := request.Validate(); err != nil {
			f.Fatal(err)
		}
		f.Add(seed.body, seed.length, seed.split, seed.failure, seed.unknown, seed.head)
	}
	f.Fuzz(func(t *testing.T, body []byte, declared, split uint16, failure, unknown, head bool) {
		if len(body) > 64<<10 {
			return
		}
		length, err := core.NewByteLength(uint64(declared))
		if err != nil {
			t.Fatal(err)
		}
		boundary := min(int(split), len(body))
		var cause error
		if failure {
			cause = io.ErrClosedPipe
		}
		produced := false
		request := exchange.ServerProducedResponse{Status: core.HTTPStatusOK(), ContentType: core.HTTPMediaTypeOctetStream(), ContentLength: &length, Produce: func(_ context.Context, out io.Writer) error {
			produced = true
			_, first := out.Write(body[:boundary])
			_, second := out.Write(body[boundary:])
			return errors.Join(first, second, cause)
		}}
		if unknown {
			request.ContentLength = nil
		}
		recorder := httptest.NewRecorder()
		method := http.MethodGet
		if head {
			method = http.MethodHead
		}
		call, err := exchange.NewSocketServerCall(recorder, httptest.NewRequest(method, "/", nil))
		if err != nil {
			t.Fatal(err)
		}
		gotErr := exchange.WriteProduced(exchange.ProducedWriteCall{Call: call, Response: request})
		if head && !unknown {
			if !errors.Is(gotErr, core.ErrExchangeContract) || produced || recorder.Body.Len() != 0 || recorder.Header().Get("Content-Length") != "" {
				t.Fatalf("HEAD exact admission=(%v,produced=%t,bytes=%d,framing=%q), want no producer or native commit", gotErr, produced, recorder.Body.Len(), recorder.Header().Get("Content-Length"))
			}
			return
		}
		if failure && !errors.Is(gotErr, io.ErrClosedPipe) {
			t.Fatalf("producer cause=%v, want closed pipe retained", gotErr)
		}
		if unknown {
			if !bytes.Equal(recorder.Body.Bytes(), body) || recorder.Header().Get("Content-Length") != "" || !errors.Is(gotErr, cause) {
				t.Fatalf("unknown extent=(%q,%q,%v), want complete input %q and cause %v", recorder.Body.Bytes(), recorder.Header().Get("Content-Length"), gotErr, body, cause)
			}
			return
		}
		if len(body) == int(declared) && !failure {
			if gotErr != nil || !bytes.Equal(recorder.Body.Bytes(), body) || recorder.Header().Get("Content-Length") != strconv.Itoa(int(declared)) {
				t.Fatalf("exact extent=(%q,%q,%v), want input %q and nil", recorder.Body.Bytes(), recorder.Header().Get("Content-Length"), gotErr, body)
			}
			return
		}
		if !errors.Is(gotErr, core.ErrExchangeWrite) || !errors.Is(gotErr, core.ErrExchangeResponse) {
			t.Fatalf("refused producer=%v, want typed response/write refusal", gotErr)
		}
		if declared != 0 && recorder.Body.Len() >= int(declared) {
			t.Fatalf("failed producer released %d bytes for declared %d, want incomplete native HTTP body", recorder.Body.Len(), declared)
		}
		if len(body) < int(declared) && !failure && !errors.Is(gotErr, io.ErrUnexpectedEOF) {
			t.Fatalf("short producer=%v, want unexpected EOF", gotErr)
		}
		if len(body) > int(declared) && !errors.Is(gotErr, core.ErrExchangeBodyLimit) {
			t.Fatalf("long producer=%v, want body-limit refusal", gotErr)
		}
	})
}
