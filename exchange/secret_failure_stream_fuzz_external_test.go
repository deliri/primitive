package exchange_test

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type failureFuzzPatternReader struct {
	pattern []byte
	offset  uint64
}

func (r *failureFuzzPatternReader) Read(data []byte) (int, error) {
	for i := range data {
		if len(r.pattern) == 0 {
			data[i] = 0
		} else {
			data[i] = r.pattern[(r.offset+uint64(i))%uint64(len(r.pattern))]
		}
	}
	r.offset += uint64(len(data))
	return len(data), nil
}

type failureFuzzPatternWriter struct {
	pattern           []byte
	count             uint64
	failAt            uint64
	partial, mismatch bool
}

func (w *failureFuzzPatternWriter) Write(data []byte) (int, error) {
	n := len(data)
	if w.partial {
		n = int(min(uint64(n), w.failAt-w.count))
	}
	for i, value := range data[:n] {
		want := byte(0)
		if len(w.pattern) > 0 {
			want = w.pattern[(w.count+uint64(i))%uint64(len(w.pattern))]
		}
		if value != want {
			w.mismatch = true
		}
	}
	w.count += uint64(n)
	if w.partial && w.count == w.failAt {
		return n, io.ErrClosedPipe
	}
	return n, nil
}

type failureFuzzNativeTransport struct {
	transport http.RoundTripper
	closes    *atomic.Uint64
}

func (t failureFuzzNativeTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := t.transport.RoundTrip(request)
	if response != nil && response.Body != nil {
		response.Body = &failureFuzzNativeBody{ReadCloser: response.Body, closes: t.closes}
	}
	return response, err
}

type failureFuzzNativeBody struct {
	io.ReadCloser
	closes *atomic.Uint64
}

func (b *failureFuzzNativeBody) Close() error { b.closes.Add(1); return b.ReadCloser.Close() }

// Every callback exercises all three public native HTTP stream doors. Working
// memory follows the borrowed fuzz pattern and fixed copy window, not extent.
// The finite extent cap below owns this proof's execution budget only.
func FuzzNativeSecretsAndFailureStreamsConserveCustody(f *testing.F) {
	seed := []byte{0, 255, 81}
	writer := httptest.NewRecorder()
	response := exchange.ServerStreamResponse{Source: bytes.NewReader(seed), ContentType: core.HTTPMediaTypeOctetStream(), ContentLength: new(mustByteLength(f, uint64(len(seed)))), Status: core.HTTPStatusOK()}
	if err := response.Validate(); err != nil {
		f.Fatal(err)
	}
	if err := exchange.WriteStream(exchange.StreamWriteCall{Call: socketServerCallFrom(f, writer, httptest.NewRequest(http.MethodGet, "/", nil)), Response: response}); err != nil {
		f.Fatal(err)
	}
	header, err := exchange.NewBearerAuthorizationHeader(exchange.BearerAuthorization{Token: []byte("owned-wire-seed")})
	if err != nil {
		f.Fatal(err)
	}
	wire, err := header.Values[0].Value()
	if err != nil {
		f.Fatal(err)
	}
	credential := []byte(wire[len(exchange.BearerAuthorizationScheme)+1:])
	for _, extent := range []uint32{0, 1, 4095, 4096, 4097, 32767, 32768, 32769, 262151, 4194305} {
		f.Add(writer.Body.Bytes(), credential, extent, uint32(0), uint8(0), false, false)
	}
	for _, mode := range []uint8{1, 2, 3, 4, 5} {
		f.Add(writer.Body.Bytes(), credential, uint32(17), uint32(7), mode, false, false)
	}
	f.Add(writer.Body.Bytes(), credential, uint32(4097), uint32(4096), uint8(0), true, true)
	f.Add(writer.Body.Bytes(), credential, uint32(17), uint32(0), uint8(0), true, false)
	f.Add(writer.Body.Bytes(), []byte("bad\r\nkey"), uint32(17), uint32(0), uint8(0), false, false)
	f.Add(writer.Body.Bytes(), []byte("key=tail"), uint32(17), uint32(0), uint8(0), false, true)
	f.Fuzz(func(t *testing.T, pattern, material []byte, extent, prefix uint32, mode uint8, partial, bearer bool) {
		if len(pattern) > 4096 || len(material) > exchange.HeaderValueMaximumBytes+1 || extent > 4194305 || mode > 5 {
			return
		}
		failAt := uint64(prefix) % (uint64(extent) + 1)
		wantAdmitted := mode == 0 && failureFuzzCredentialGrammar(material, bearer)
		for door := range 3 {
			var calls, closes atomic.Uint64
			observed := make(chan string, 1)
			name, err := core.ParseHTTPHeaderName("X-Owned-Secret")
			if err != nil {
				t.Fatal(err)
			}
			if bearer {
				name, err = exchange.StandardHeaderAuthorization.Name()
				if err != nil {
					t.Fatal(err)
				}
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				calls.Add(1)
				observed <- request.Header.Get(name.String())
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = io.CopyN(w, &failureFuzzPatternReader{pattern: pattern}, int64(extent))
			}))
			native := server.Client()
			native.Transport = failureFuzzNativeTransport{transport: native.Transport, closes: &closes}
			client := mustExchangeClient(t, native)
			key := bytes.Clone(material)
			scheme := exchange.SecretHeaderLiteral
			if bearer {
				scheme = exchange.SecretHeaderBearer
			}
			secret := []exchange.SecretHeader{{Name: name, Value: key, Scheme: scheme}}
			var ordinary exchange.Headers
			switch mode {
			case 1:
				secret[0].Scheme = 255
			case 2:
				secret = append(secret, secret[0])
			case 3:
				value, err := exchange.NewHeaderValue("ordinary-value")
				if err != nil {
					t.Fatal(err)
				}
				ordinary.Values = []exchange.Header{{Name: name, Values: []exchange.HeaderValue{value}}}
			case 5:
				secret[0].Name = core.HTTPHeaderName{}
			}
			failure := &failureFuzzPatternWriter{pattern: pattern, partial: partial, failAt: failAt}
			success := &failureFuzzPatternWriter{pattern: pattern}
			var destination io.Writer = failure
			if mode == 4 {
				var absent *failureFuzzPatternWriter
				destination = absent
			}
			target := mustEndpoint(t, server.URL)
			policy := singleAttemptStreamPolicy(t)
			var result exchange.StreamResponse
			switch door {
			case 0:
				result, err = exchange.Upload(exchange.UploadCall{Context: t.Context(), Client: client, Policy: policy, Request: exchange.UploadRequest{Target: target, Source: bytes.NewReader(nil), ContentType: core.HTTPMediaTypeOctetStream(), Semantics: exchange.RequestSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt}, ExpectedStatus: core.HTTPStatusOK(), Headers: ordinary, SecretHeaders: secret, FailureDestination: destination}})
			case 1:
				result, err = exchange.Download(exchange.DownloadCall{Context: t.Context(), Client: client, Policy: policy, Request: exchange.DownloadRequest{Target: target, Destination: success, Semantics: exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySingleAttempt}, ExpectedStatus: core.HTTPStatusOK(), Headers: ordinary, SecretHeaders: secret, FailureDestination: destination, Buffer: make([]byte, 4096)}})
			case 2:
				got, cause := exchange.RoundTripStream(exchange.StreamRoundTripCall{Context: t.Context(), Client: client, Policy: policy, Request: exchange.StreamRoundTripRequest{Target: target, Source: bytes.NewReader(nil), Destination: success, RequestContentType: core.HTTPMediaTypeOctetStream(), Semantics: exchange.RequestSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt}, ExpectedStatus: core.HTTPStatusOK(), Headers: ordinary, SecretHeaders: secret, FailureDestination: destination, Buffer: make([]byte, 4096)}})
				result, err = exchange.StreamResponse(got), cause
			}
			clear(key)
			server.Close()
			if !wantAdmitted {
				if !errors.Is(err, core.ErrExchangeRequest) || calls.Load() != 0 || closes.Load() != 0 || failure.count != 0 || success.count != 0 || result.Metadata.Attempts != 0 || result.Metadata.Status != (core.HTTPStatusCode{}) || result.Metadata.Bytes != (core.ByteLength{}) || result.Metadata.Headers.Values != nil {
					t.Fatalf("door%d refused=(%+v,%v), calls/closes=%d/%d sink=%d/%d; want zero effects", door, result, err, calls.Load(), closes.Load(), failure.count, success.count)
				}
				continue
			}
			wantBytes := uint64(extent)
			wantPartial := partial && extent > 0
			if wantPartial {
				wantBytes = failAt
			}
			status, ok := errors.AsType[exchange.StatusError](err)
			if !ok || status.Status() != mustHTTPStatus(t, http.StatusUnauthorized) || status.Expected() != core.HTTPStatusOK() || !errors.Is(err, core.ErrExchangeAuthentication) || errors.Is(err, io.ErrClosedPipe) != wantPartial || failure.count != wantBytes || failure.mismatch || success.count != 0 || result.Metadata.Bytes.Uint64() != wantBytes || result.Metadata.Attempts != 1 || result.Metadata.Status != status.Status() || calls.Load() != 1 || closes.Load() != 1 {
				t.Fatalf("door%d observed=(%+v,%v), calls/closes=%d/%d failure=%d mismatch%t success=%d; want exact acknowledged%d bytes, one closed native response", door, result, err, calls.Load(), closes.Load(), failure.count, failure.mismatch, success.count, wantBytes)
			}
			wantWire := string(material)
			if bearer {
				wantWire = exchange.BearerAuthorizationScheme + " " + wantWire
			}
			if gotWire := <-observed; gotWire != wantWire {
				t.Fatalf("door%d wire=%q, want exact typed credential encoding", door, gotWire)
			}
		}
	})
}

// This independent RFC grammar oracle does not call SecretHeader.Validate.
func failureFuzzCredentialGrammar(material []byte, bearer bool) bool {
	if len(material) == 0 || len(material) > exchange.HeaderValueMaximumBytes {
		return false
	}
	padding := false
	for i, value := range material {
		if !bearer {
			if value < 33 || value > 126 {
				return false
			}
			continue
		}
		if value == '=' {
			if i == 0 {
				return false
			}
			padding = true
			continue
		}
		if padding {
			return false
		}
		if value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z' || value >= '0' && value <= '9' {
			continue
		}
		switch value {
		case '-', '.', '_', '~', '+', '/':
		default:
			return false
		}
	}
	return true
}
