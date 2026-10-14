package ai_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/ai"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// validatorCustodyServer is an explicit validation seam around the real native
// TLS-backed SDK. Its cause can revoke authority before any downstream effect.
type validatorCustodyServer struct {
	downstream ai.Server
	cause      error
	calls      int
}

func (s *validatorCustodyServer) Validate() error {
	if s.cause != nil {
		return s.cause
	}
	return s.downstream.Validate()
}
func (s *validatorCustodyServer) RoundTrip(ctx context.Context, request ai.Request, policy exchange.StreamPolicy) (ai.Response, error) {
	s.calls++
	return s.downstream.RoundTrip(ctx, request, policy)
}

func TestReplaceableValidatorPreservesOneAIIdentityBeforeNativeEffectsLayerTriad(t *testing.T) {
	t.Parallel()
	identities := []core.ErrorIdentity{core.ErrAIContract, core.ErrAIAuthentication, core.ErrAIBinding, core.ErrAIRateLimited, core.ErrAIUnavailable, core.ErrAIRequestRejected}
	for _, identity := range identities {
		t.Run(fmt.Sprintf("validator-identity-%d", identity), func(t *testing.T) {
			t.Parallel()
			var nativeRequests atomic.Int64
			fixture := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				nativeRequests.Add(1)
				writer.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
			}))
			defer fixture.Close()
			owner := &validatorCustodyServer{downstream: nativeClient(t, fixture, ai.OpenAI), cause: errors.Join(identity, io.ErrClosedPipe)}
			refused, err := ai.NewClient(owner)
			if refused != (ai.Client{}) || !errors.Is(err, identity) || !errors.Is(err, io.ErrClosedPipe) || owner.calls != 0 || nativeRequests.Load() != 0 {
				t.Fatalf("revoked binding = (%v,%v,calls%d,native%d), want zero client/native cause/zero effects", refused, err, owner.calls, nativeRequests.Load())
			}
			for _, other := range identities {
				if errors.Is(err, other) != (other == identity) {
					t.Fatalf("validator binding error = %v matches %v, want exactly %v", err, other, identity)
				}
			}
			owner.cause = nil
			client, err := ai.NewClient(owner)
			if err != nil || owner.calls != 0 || nativeRequests.Load() != 0 {
				t.Fatalf("neutral validated binding = (%v,calls%d,native%d), want no request effects", err, owner.calls, nativeRequests.Load())
			}
			owner.cause = errors.Join(identity, io.ErrClosedPipe)
			request := streamingRequest(t, ai.Route{Provider: ai.OpenAI, Operation: ai.Responses}, bytes.NewReader(nil), io.Discard)
			response, err := client.RoundTrip(t.Context(), request, streamPolicy())
			if !errors.Is(err, identity) || !errors.Is(err, io.ErrClosedPipe) || response.Stream.Metadata.Attempts != 0 || owner.calls != 0 || nativeRequests.Load() != 0 {
				t.Fatalf("revoked operation = (%v,%+v,calls%d,native%d), want native cause/zero admitted progress/zero downstream effects", err, response.Stream.Metadata, owner.calls, nativeRequests.Load())
			}
			for _, other := range identities {
				if errors.Is(err, other) != (other == identity) {
					t.Fatalf("validator call error = %v matches %v, want exactly %v", err, other, identity)
				}
			}
			owner.cause = nil
			response, err = client.RoundTrip(t.Context(), request, streamPolicy())
			if err != nil || response.Validate() != nil || response.Stream.Metadata.Attempts != 1 || response.Stream.Metadata.Bytes.Uint64() != 0 || owner.calls != 1 || nativeRequests.Load() != 1 {
				t.Fatalf("restored operation = (%v,%+v,calls%d,native%d), want one native zero-body request", err, response.Stream.Metadata, owner.calls, nativeRequests.Load())
			}
		})
	}
}
