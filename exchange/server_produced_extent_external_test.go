package exchange_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestWriteProducedExactExtentLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		length     uint64
		chunks     []string
		cause      error
		ignore     bool
		want       string
		wantErr    error
		wantBefore int
	}{
		{name: "neutral/empty extent seals only an empty producer", length: 0},
		{name: "positive/one byte stays held until producer success", length: 1, chunks: []string{"a"}, want: "a"},
		{name: "positive/two bytes release their final byte last", length: 2, chunks: []string{"ab"}, want: "ab", wantBefore: 1},
		{name: "positive/separate writes preserve the final position", length: 2, chunks: []string{"a", "b"}, want: "ab", wantBefore: 1},
		{name: "positive/empty writes after the final byte add no material", length: 1, chunks: []string{"a", ""}, want: "a"},
		{name: "negative/no bytes cannot seal a one byte extent", length: 1, wantErr: io.ErrUnexpectedEOF},
		{name: "negative/a prefix cannot seal two declared bytes", length: 2, chunks: []string{"a"}, want: "a", wantErr: io.ErrUnexpectedEOF, wantBefore: 1},
		{name: "negative/empty extent refuses material before headers", length: 0, chunks: []string{"a"}, wantErr: core.ErrExchangeBodyLimit},
		{name: "negative/one write cannot exceed the extent", length: 1, chunks: []string{"ab"}, wantErr: core.ErrExchangeBodyLimit},
		{name: "negative/extra write cannot release the held byte", length: 2, chunks: []string{"ab", "c"}, want: "a", wantErr: core.ErrExchangeBodyLimit, wantBefore: 1},
		{name: "negative/ignored excess remains a sticky refusal", length: 2, chunks: []string{"ab", "c", ""}, ignore: true, want: "a", wantErr: core.ErrExchangeBodyLimit, wantBefore: 1},
		{name: "negative/late producer cause keeps final byte unreleased", length: 2, chunks: []string{"ab"}, cause: io.ErrClosedPipe, want: "a", wantErr: io.ErrClosedPipe, wantBefore: 1},
		{name: "negative/one byte producer cause sends no body", length: 1, chunks: []string{"a"}, cause: io.ErrClosedPipe, wantErr: io.ErrClosedPipe},
		{name: "negative/empty producer cause commits no framing", length: 0, cause: io.ErrClosedPipe, wantErr: io.ErrClosedPipe},
		{name: "negative/refusal after a prefix preserves its native cause", length: 3, chunks: []string{"a"}, cause: io.ErrClosedPipe, want: "a", wantErr: io.ErrClosedPipe, wantBefore: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			call, err := exchange.NewSocketServerCall(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
			if err != nil {
				t.Fatal(err)
			}
			length, err := core.NewByteLength(tc.length)
			if err != nil {
				t.Fatal(err)
			}
			before := -1
			err = exchange.WriteProduced(exchange.ProducedWriteCall{Call: call, Response: exchange.ServerProducedResponse{
				Status: core.HTTPStatusOK(), ContentType: core.HTTPMediaTypeOctetStream(), ContentLength: &length,
				Produce: func(_ context.Context, out io.Writer) error {
					var writeErr error
					for _, chunk := range tc.chunks {
						_, e := io.WriteString(out, chunk)
						if e != nil && !tc.ignore {
							writeErr = e
							break
						}
					}
					before = recorder.Body.Len()
					return errors.Join(writeErr, tc.cause)
				},
			}})
			if !errors.Is(err, tc.wantErr) || recorder.Body.String() != tc.want || before != tc.wantBefore {
				t.Fatalf("WriteProduced()=(%q,%v), producer prefix=%d; want (%q,%v), prefix=%d", recorder.Body.String(), err, before, tc.want, tc.wantErr, tc.wantBefore)
			}
			if tc.wantErr != nil && (!errors.Is(err, core.ErrExchangeResponse) || !errors.Is(err, core.ErrExchangeWrite)) {
				t.Fatalf("WriteProduced() error=%v, want owning response/write identities", err)
			}
			wantLength := strconv.FormatUint(tc.length, 10)
			if tc.length == 0 && tc.wantErr != nil {
				wantLength = ""
			}
			if got := recorder.Header().Get("Content-Length"); got != wantLength {
				t.Fatalf("Content-Length=%q, want %q", got, wantLength)
			}
		})
	}
}

func TestWriteProducedExactExtentClientEOFPrecedesHandlerCleanup(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	cleanup := make(chan struct{})
	produced := make(chan error, 1)
	joined := make(chan struct{})
	payload := []byte("sealed artifact bytes")
	length, err := core.NewByteLength(uint64(len(payload)))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(joined)
		call, err := exchange.NewSocketServerCall(w, r)
		if err == nil {
			err = exchange.WriteProduced(exchange.ProducedWriteCall{Call: call, Response: exchange.ServerProducedResponse{Status: core.HTTPStatusOK(), ContentType: core.HTTPMediaTypeOctetStream(), ContentLength: &length, Produce: func(_ context.Context, out io.Writer) error {
				_, err := out.Write(payload)
				return err
			}}})
		}
		produced <- err
		select {
		case <-cleanup:
		case <-ctx.Done():
		}
	}))
	defer server.Close()
	defer close(cleanup)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	got, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if serverErr := <-produced; serverErr != nil || readErr != nil || closeErr != nil || !bytes.Equal(got, payload) || response.ContentLength != int64(len(payload)) {
		t.Fatalf("framed client=(%q,%v,%v), producer=%v, length=%d; want %q and EOF before cleanup", got, readErr, closeErr, serverErr, response.ContentLength, payload)
	}
	select {
	case <-joined:
		t.Fatal("handler cleanup joined before its release, want caller EOF while cleanup remains held")
	default:
	}
}

func TestWriteProducedExactExtentNativeLateFailureRefusesCompleteBody(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	produced := make(chan error, 1)
	length, err := core.NewByteLength(2)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call, err := exchange.NewSocketServerCall(w, r)
		if err == nil {
			err = exchange.WriteProduced(exchange.ProducedWriteCall{Call: call, Response: exchange.ServerProducedResponse{Status: core.HTTPStatusOK(), ContentType: core.HTTPMediaTypeOctetStream(), ContentLength: &length, Produce: func(_ context.Context, out io.Writer) error {
				_, writeErr := io.WriteString(out, "ab")
				reader, writer := io.Pipe()
				readCloseErr := reader.Close()
				_, pipeErr := writer.Write([]byte("after artifact"))
				return errors.Join(writeErr, readCloseErr, pipeErr, writer.Close())
			}}})
		}
		produced <- err
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
	got, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	serverErr := <-produced
	if !errors.Is(serverErr, io.ErrClosedPipe) || !errors.Is(serverErr, core.ErrExchangeWrite) || !errors.Is(readErr, io.ErrUnexpectedEOF) || string(got) != "a" || closeErr != nil {
		t.Fatalf("late producer=(%v), client=(%q,%v,%v), want preserved closed pipe and incomplete framed body", serverErr, got, readErr, closeErr)
	}
}

func TestWriteProducedExactExtentStreamsBeyondDocumentCeiling(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	const chunks = 256
	var chunk [8192]byte
	for i := range chunk {
		chunk[i] = byte(i)
	}
	want := sha256.New()
	for range chunks {
		_, _ = want.Write(chunk[:])
	}
	length, err := core.NewByteLength(chunks * uint64(len(chunk)))
	if err != nil {
		t.Fatal(err)
	}
	produced := make(chan error, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call, err := exchange.NewSocketServerCall(w, r)
		if err == nil {
			err = exchange.WriteProduced(exchange.ProducedWriteCall{Call: call, Response: exchange.ServerProducedResponse{Status: core.HTTPStatusOK(), ContentType: core.HTTPMediaTypeOctetStream(), ContentLength: &length, Produce: func(_ context.Context, out io.Writer) error {
				for range chunks {
					if _, err := out.Write(chunk[:]); err != nil {
						return err
					}
				}
				return nil
			}}})
		}
		produced <- err
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
	got := sha256.New()
	n, readErr := io.Copy(got, response.Body)
	closeErr := response.Body.Close()
	serverErr := <-produced
	if n != int64(chunks*len(chunk)) || !bytes.Equal(got.Sum(nil), want.Sum(nil)) || readErr != nil || closeErr != nil || serverErr != nil {
		t.Fatalf("large framed response=(%d,%x,%v,%v,%v), want (%d,%x,nil,nil,nil)", n, got.Sum(nil), readErr, closeErr, serverErr, chunks*len(chunk), want.Sum(nil))
	}
}

func TestWriteProducedExactExtentPreservesIgnoredFlushFailure(t *testing.T) {
	t.Parallel()
	writer := flushFailureWriter{httptest.NewRecorder()}
	call, err := exchange.NewSocketServerCall(writer, httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	length, err := core.NewByteLength(2)
	if err != nil {
		t.Fatal(err)
	}
	err = exchange.WriteProduced(exchange.ProducedWriteCall{Call: call, Response: exchange.ServerProducedResponse{Status: core.HTTPStatusOK(), ContentType: core.HTTPMediaTypeOctetStream(), ContentLength: &length, Produce: func(_ context.Context, out io.Writer) error {
		_, _ = io.WriteString(out, "ab")
		return nil
	}}})
	if !errors.Is(err, core.ErrExchangeWrite) || !errors.Is(err, io.ErrClosedPipe) || writer.Body.String() != "a" {
		t.Fatalf("ignored injected flush failure=(%q,%v), want one incomplete byte and preserved closed pipe", writer.Body.String(), err)
	}
}

func TestWriteProducedExactExtentCancellationKeepsFinalByte(t *testing.T) {
	t.Parallel()
	for _, payload := range []string{"", "a"} {
		t.Run(strconv.Itoa(len(payload))+" byte cancellation before release", func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			recorder := httptest.NewRecorder()
			call, err := exchange.NewSocketServerCall(recorder, httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil))
			if err != nil {
				t.Fatal(err)
			}
			length, err := core.NewByteLength(uint64(len(payload)))
			if err != nil {
				t.Fatal(err)
			}
			err = exchange.WriteProduced(exchange.ProducedWriteCall{Call: call, Response: exchange.ServerProducedResponse{Status: core.HTTPStatusOK(), ContentType: core.HTTPMediaTypeOctetStream(), ContentLength: &length, Produce: func(_ context.Context, out io.Writer) error {
				_, err := io.WriteString(out, payload)
				cancel()
				return err
			}}})
			if !errors.Is(err, context.Canceled) || !errors.Is(err, core.ErrExchangeWrite) || recorder.Body.Len() != 0 {
				t.Fatalf("cancelled producer=(%q,%v), want no released final byte and typed cancellation", recorder.Body.String(), err)
			}
		})
	}
}
