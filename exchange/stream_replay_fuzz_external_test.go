package exchange_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// Both Download and RoundTripStream produce observations in each callback;
// ReplayStream classifies each independently. Exact input bytes and native I/O identities are
// the oracle; no Exchange classifier computes the expected result.
func FuzzReplayStreamPreservesDownloadAndRoundTripObservation(f *testing.F) {
	seed := []byte{0x00, 0xff, 0x41}
	writer := httptest.NewRecorder()
	response := exchange.ServerStreamResponse{
		Source: bytes.NewReader(seed), ContentType: core.HTTPMediaTypeOctetStream(),
		ContentLength: new(mustByteLength(f, uint64(len(seed)))), Status: core.HTTPStatusOK(),
	}
	if err := response.Validate(); err != nil {
		f.Fatalf("stream seed validation = %v, want nil", err)
	}
	if err := exchange.WriteStream(exchange.StreamWriteCall{
		Call: socketServerCallFrom(f, writer, httptest.NewRequest(http.MethodGet, "/", nil)), Response: response,
	}); err != nil {
		f.Fatalf("stream seed production write = %v, want nil", err)
	}
	if !bytes.Equal(writer.Body.Bytes(), seed) {
		f.Fatalf("stream seed output = %x, want %x", writer.Body.Bytes(), seed)
	}
	canonical := writer.Body.Bytes()
	f.Add(canonical, uint16(len(seed)), false, uint8(streamFuzzIntact), false)
	f.Add(canonical, uint16(len(seed)-1), false, uint8(streamFuzzIntact), true)
	f.Add(canonical, uint16(len(seed)+1), false, uint8(streamFuzzIntact), false)
	f.Add(canonical, uint16(0), false, uint8(streamFuzzIntact), true)
	f.Add(canonical, uint16(len(seed)), true, uint8(streamFuzzIntact), true)
	f.Add(canonical, uint16(len(seed)), true, uint8(streamFuzzIntact), false)
	f.Add(canonical, uint16(len(seed)), false, uint8(streamFuzzReadFailure), true)
	f.Add(canonical, uint16(len(seed)), false, uint8(streamFuzzReadFailure), false)
	f.Add(canonical, uint16(len(seed)), true, uint8(streamFuzzReadFailure), true)
	f.Add(canonical, uint16(len(seed)), false, uint8(streamFuzzCloseFailure), true)
	f.Add(canonical, uint16(len(seed)), false, uint8(streamFuzzCloseFailure), false)
	f.Add(canonical, uint16(len(seed)), true, uint8(streamFuzzCloseFailure), true)
	f.Add([]byte{}, uint16(1), false, uint8(streamFuzzIntact), false)
	f.Fuzz(func(t *testing.T, data []byte, window uint16, failedStatus bool, faultByte uint8, cancelAtHandoff bool) {
		const oracleMaximumBytes = 4096
		if len(data) > oracleMaximumBytes || window > oracleMaximumBytes+1 {
			return
		}
		fault := streamFuzzFault(faultByte)
		if fault != streamFuzzIntact && fault != streamFuzzReadFailure && fault != streamFuzzCloseFailure {
			return
		}
		for _, roundTrip := range []bool{false, true} {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			body := &bindingObservedBody{reader: &streamFuzzReader{source: bytes.NewReader(data), fault: fault}}
			if fault == streamFuzzCloseFailure {
				body.err = io.ErrClosedPipe
			}
			status := http.StatusOK
			if failedStatus {
				status = http.StatusServiceUnavailable
			}
			wantStatus := mustHTTPStatus(t, status)
			var providerCalls, attemptCalls int
			client := mustExchangeClient(t, &http.Client{Transport: bindingTransport(func(request *http.Request) (*http.Response, error) {
				providerCalls++
				return &http.Response{StatusCode: status, Body: body, ContentLength: -1, Request: request}, nil
			})})
			policy := singleAttemptStreamPolicy(t)
			request := exchange.DownloadRequest{
				Target:         mustEndpoint(t, "https://replay-oracle.invalid/stream"),
				Semantics:      exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySingleAttempt},
				ExpectedStatus: core.HTTPStatusOK(),
			}
			var destination bytes.Buffer
			request.Destination = io.MultiWriter(&destination)
			request.Buffer = make([]byte, int(window))
			wantBytes := len(data)
			if failedStatus {
				wantBytes = 0
			}
			wantRead := len(data)
			wantProviderCalls := 1
			wantMetadataAttempts := uint64(1)
			wantReadFailure := fault == streamFuzzReadFailure
			wantCloseFailure := fault == streamFuzzCloseFailure
			wantStatusError := failedStatus
			wantFailure := wantReadFailure || wantCloseFailure || wantStatusError
			wantExhausted := !cancelAtHandoff && wantFailure
			var produced exchange.StreamResponse
			var producedErr error
			got, gotErr := exchange.ReplayStream(exchange.StreamReplayCall{
				Context: ctx,
				Policy:  exchange.StreamReplayPolicy{OperationTimeout: policy.OperationTimeout, Retry: singleAttemptOperationPolicy(t).Retry},
				Attempt: func(attemptContext context.Context, index uint64) (exchange.StreamResponse, error) {
					attemptCalls++
					if index != 1 || attemptCalls != 1 {
						t.Fatalf("attempt index/calls = %d/%d, want 1/1", index, attemptCalls)
					}
					if roundTrip {
						response, err := exchange.RoundTripStream(exchange.StreamRoundTripCall{Context: attemptContext, Client: client, Policy: policy,
							Request: exchange.StreamRoundTripRequest{Target: request.Target, Source: bytes.NewReader(nil), Destination: request.Destination, Buffer: request.Buffer,
								RequestContentLength: new(mustByteLength(t, 0)), RequestContentType: core.HTTPMediaTypeOctetStream(),
								Semantics:      exchange.RequestSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt},
								ExpectedStatus: request.ExpectedStatus}})
						produced, producedErr = exchange.StreamResponse(response), err
						if response.DeclaredRequestBytes.Uint64() != 0 {
							t.Fatalf("empty upload declaration=%d, want 0", response.DeclaredRequestBytes.Uint64())
						}
					} else {
						produced, producedErr = exchange.Download(exchange.DownloadCall{Context: attemptContext, Client: client, Request: request, Policy: policy})
					}
					// Establish producer facts before handing them to replay.
					if (producedErr != nil) != wantFailure || errors.Is(producedErr, core.ErrExchangeContract) != wantFailure || errors.Is(producedErr, core.ErrExchangeResponse) != wantFailure || errors.Is(producedErr, core.ErrExchangeBodyLimit) || errors.Is(producedErr, io.ErrUnexpectedEOF) != wantReadFailure || errors.Is(producedErr, io.ErrClosedPipe) != wantCloseFailure {
						t.Fatalf("producer cause = %v, want failure/response/overflow/read/close = %t/%t/%t/%t/%t", producedErr, wantFailure, wantFailure, false, wantReadFailure, wantCloseFailure)
					}
					statusError, hasStatus := errors.AsType[exchange.StatusError](producedErr)
					if hasStatus != wantStatusError || hasStatus && (statusError.Status() != wantStatus || statusError.Expected() != core.HTTPStatusOK()) {
						t.Fatalf("producer status error = (%v, %t), want present %t with observed %v and required OK", statusError, hasStatus, wantStatusError, wantStatus)
					}
					if produced.Metadata.Status != wantStatus || produced.Metadata.Attempts != wantMetadataAttempts || produced.Metadata.Bytes.Uint64() != uint64(wantBytes) || produced.Metadata.Headers.Values != nil {
						t.Fatalf("producer metadata = %+v, want status %v, %d attempts, %d bytes, absent headers", produced.Metadata, wantStatus, wantMetadataAttempts, wantBytes)
					}
					if wantMetadataAttempts != 0 {
						if err := produced.Validate(); err != nil {
							t.Fatalf("admitted producer observation validation = %v, want nil", err)
						}
					}
					if providerCalls != wantProviderCalls || body.closes != wantProviderCalls || body.reads != wantRead || !bytes.Equal(destination.Bytes(), data[:wantBytes]) {
						t.Fatalf("producer calls/closes/read/output = %d/%d/%d/%x, want %d/%d/%d/%x", providerCalls, body.closes, body.reads, destination.Bytes(), wantProviderCalls, wantProviderCalls, wantRead, data[:wantBytes])
					}
					if cancelAtHandoff {
						cancel()
					}
					return produced, producedErr
				},
			})
			if (gotErr != nil) != (wantFailure || cancelAtHandoff) || errors.Is(gotErr, context.Canceled) != cancelAtHandoff || errors.Is(gotErr, core.ErrExchangeCancelled) != cancelAtHandoff || errors.Is(gotErr, core.ErrExchangeRetryExhausted) != wantExhausted {
				t.Fatalf("replay cause = %v, want failure %t, cancellation %t, exhaustion %t", gotErr, wantFailure || cancelAtHandoff, cancelAtHandoff, wantExhausted)
			}
			// Error-tree inclusion rejects lost native causes and wrong valid
			// classification; the exact original error object must remain reachable.
			if producedErr != nil && !errors.Is(gotErr, producedErr) {
				t.Fatalf("replay error = %v, want original producer refusal %v retained", gotErr, producedErr)
			}
			if got.Metadata.Status != produced.Metadata.Status || got.Metadata.Attempts != produced.Metadata.Attempts || got.Metadata.Bytes != produced.Metadata.Bytes || got.Metadata.Headers.Values != nil || attemptCalls != 1 || providerCalls != wantProviderCalls || body.closes != wantProviderCalls || !bytes.Equal(destination.Bytes(), data[:wantBytes]) {
				t.Fatalf("replay metadata/custody = %+v/%d/%d/%d/%x, want producer %+v, one callback, %d provider calls/closes, output %x", got.Metadata, attemptCalls, providerCalls, body.closes, destination.Bytes(), produced.Metadata, wantProviderCalls, data[:wantBytes])
			}
			if wantMetadataAttempts != 0 {
				if err := got.Validate(); err != nil {
					t.Fatalf("retained observation validation = %v, want nil", err)
				}
			}
			if wantExhausted {
				exhausted, ok := errors.AsType[exchange.RetryExhaustedError](gotErr)
				if !ok || exhausted.Attempts() != 1 || !errors.Is(exhausted.Cause(), producedErr) {
					t.Fatalf("exhaustion = (%v, %t), want one attempt and original cause", exhausted, ok)
				}
			}
		}
	})
}
