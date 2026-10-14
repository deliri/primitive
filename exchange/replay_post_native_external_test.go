package exchange_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestNativePostSingleAttemptCannotAcquireReplayEffectsLayerTriad(t *testing.T) {
	t.Parallel()
	key, err := exchange.ParseIdempotencyKey("owned-post-once")
	if err != nil {
		t.Fatal(err)
	}
	for _, keyed := range []bool{false, true} {
		for _, multiple := range []bool{false, true} {
			t.Run(nativePostReplayName(keyed, multiple), func(t *testing.T) {
				t.Parallel()
				var nativeCalls, attempts atomic.Uint64
				observed := make(chan struct {
					body []byte
					key  string
				}, 1)
				server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
					nativeCalls.Add(1)
					data, err := io.ReadAll(request.Body)
					if err != nil {
						t.Error(err)
					}
					observed <- struct {
						body []byte
						key  string
					}{body: data, key: request.Header.Get("Idempotency-Key")}
					writer.WriteHeader(http.StatusServiceUnavailable)
				}))
				defer server.Close()
				client := mustExchangeClient(t, server.Client())
				semantics := exchange.RequestSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt}
				if keyed {
					semantics.Replay, semantics.IdempotencyKey = exchange.ReplaySingleAttemptWithIdempotencyKey, key
				}
				policy := retryOperationPolicy(t, 2)
				if !multiple {
					policy = singleAttemptOperationPolicy(t)
				}
				payload := []byte{0, 255, 81}
				result, err := exchange.ReplayStream(exchange.StreamReplayCall{Context: t.Context(), Semantics: semantics, Policy: exchange.StreamReplayPolicy{OperationTimeout: policy.OperationTimeout, Retry: policy.Retry}, Attempt: func(ctx context.Context, _ uint64) (exchange.StreamResponse, error) {
					attempts.Add(1)
					return exchange.Upload(exchange.UploadCall{Context: ctx, Client: client, Policy: singleAttemptStreamPolicy(t), Request: exchange.UploadRequest{Target: mustEndpoint(t, server.URL), Source: bytes.NewReader(payload), ContentType: core.HTTPMediaTypeOctetStream(), ContentLength: new(mustByteLength(t, uint64(len(payload)))), ExpectedStatus: core.HTTPStatusOK(), Semantics: semantics}})
				}})
				if multiple {
					if !errors.Is(err, core.ErrExchangeRequest) || attempts.Load() != 0 || nativeCalls.Load() != 0 || result.Metadata.Attempts != 0 || result.Metadata.Status != (core.HTTPStatusCode{}) || result.Metadata.Bytes != (core.ByteLength{}) {
						t.Fatalf("refused replay = (%+v,%v), callbacks %d native requests %d; want zero effects", result, err, attempts.Load(), nativeCalls.Load())
					}
				} else {
					if !errors.Is(err, core.ErrExchangeUnavailable) || attempts.Load() != 1 || nativeCalls.Load() != 1 || result.Metadata.Attempts != 1 {
						t.Fatalf("single native post = (%+v,%v), callbacks %d requests %d; want exactly one effect and native provider refusal", result, err, attempts.Load(), nativeCalls.Load())
					}
					wantKey := ""
					if keyed {
						wantKey = key.String()
					}
					if observed := <-observed; !bytes.Equal(observed.body, payload) || observed.key != wantKey {
						t.Fatalf("native post bytes=%x key=%q, want %x key=%q", observed.body, observed.key, payload, wantKey)
					}
				}
			})
		}
	}
}

func nativePostReplayName(keyed, multiple bool) string {
	if keyed && multiple {
		return "keyed single authority refuses multiple attempts before HTTP"
	}
	if keyed {
		return "keyed single authority executes one native POST"
	}
	if multiple {
		return "single authority refuses multiple attempts before HTTP"
	}
	return "single authority executes one native POST"
}
