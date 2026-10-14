package ai_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/ai"
	"github.com/deliri/primitive/v2026/ai/claude"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestProviderRefusalsRetainNativeStatusBodyAndCorrelationLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		status  int
		wantErr error
	}{
		{name: "bad request needs corrected facts", status: 400, wantErr: core.ErrAIRequestRejected},
		{name: "revoked key needs authentication repair", status: 401, wantErr: core.ErrAIAuthentication},
		{name: "permission refusal is preserved", status: 403, wantErr: core.ErrAIRequestRejected},
		{name: "foreign model is provider refusal", status: 404, wantErr: core.ErrAIRequestRejected},
		{name: "request timeout remains unavailable", status: 408, wantErr: core.ErrAIUnavailable},
		{name: "provider extent refusal is native", status: 413, wantErr: core.ErrAIRequestRejected},
		{name: "throttle retains backoff metadata", status: 429, wantErr: core.ErrAIRateLimited},
		{name: "internal service error remains unavailable", status: 500, wantErr: core.ErrAIUnavailable},
		{name: "unavailable service retains cause", status: 503, wantErr: core.ErrAIUnavailable},
		{name: "Anthropic overload remains unavailable", status: 529, wantErr: core.ErrAIUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			payload := []byte("native-provider-error-body")
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
				w.Header().Set("Retry-After", "7")
				w.Header().Set(core.AIAnthropicRequestIDHeader, "req-owned-native")
				w.WriteHeader(tc.status)
				_, _ = w.Write(payload)
			}))
			defer server.Close()
			client := nativeClient(t, server, ai.Anthropic)
			var ordinary, failure bytes.Buffer
			request := streamingRequest(t, claude.Route(), bytes.NewReader(nil), &ordinary)
			request.Stream.FailureDestination = &failure
			got, err := client.RoundTrip(t.Context(), request, streamPolicy())
			var status exchange.StatusError
			if !errors.Is(err, tc.wantErr) || !errors.As(err, &status) || !bytes.Equal(failure.Bytes(), payload) || ordinary.Len() != 0 || got.Stream.Metadata.Bytes.Uint64() != uint64(len(payload)) || got.Stream.Metadata.Attempts != 1 {
				t.Fatalf("provider refusal = error:%v status:%v failure:%d ordinary:%d metadata:%v, want typed native cause and exact isolated failure stream", err, status, failure.Len(), ordinary.Len(), got.Stream.Metadata)
			}
			gotStatus, statusErr := status.Status().Int()
			if statusErr != nil || gotStatus != tc.status {
				t.Fatalf("native status = (%d,%v), want %d", gotStatus, statusErr, tc.status)
			}
			for _, identity := range []core.ErrorIdentity{core.ErrAIContract, core.ErrAIAuthentication, core.ErrAIBinding, core.ErrAIRateLimited, core.ErrAIUnavailable, core.ErrAIRequestRejected} {
				if errors.Is(err, identity) != (identity == tc.wantErr) {
					t.Fatalf("AI identity %v match = %t, want exactly %v", identity, errors.Is(err, identity), tc.wantErr)
				}
			}
			var gotRetry, gotID string
			for _, header := range got.Stream.Metadata.Headers.Values {
				value, valueErr := header.Values[0].Value()
				if valueErr != nil {
					t.Fatalf("captured header.Value() error = %v, want nil", valueErr)
				}
				switch header.Name.String() {
				case "Retry-After":
					gotRetry = value
				case "Request-Id":
					gotID = value
				}
			}
			if gotRetry != "7" || gotID != "req-owned-native" {
				t.Fatalf("correlation = retry:%q request:%q, want 7 and req-owned-native", gotRetry, gotID)
			}
		})
	}
}

type partialFailureSink struct {
	written int
	limit   int
}

func (w *partialFailureSink) Write(p []byte) (int, error) {
	count := len(p)
	if count > w.limit-w.written {
		count = w.limit - w.written
	}
	w.written += count
	if w.written == w.limit {
		return count, io.ErrClosedPipe
	}
	return count, nil
}

func TestProviderPartialResponsePreservesAcknowledgedBytesAndNativeCause(t *testing.T) {
	t.Parallel()
	for _, status := range []int{http.StatusOK, http.StatusTooManyRequests} {
		t.Run(fmt.Sprintf("native-status-%d-partial-write", status), func(t *testing.T) {
			t.Parallel()
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
				w.WriteHeader(status)
				_, _ = io.Copy(w, &generatedSource{left: 65539})
			}))
			defer server.Close()
			client := nativeClient(t, server, ai.OpenAI)
			sink := &partialFailureSink{limit: 32769}
			request := streamingRequest(t, ai.Route{Provider: ai.OpenAI, Operation: ai.Responses}, bytes.NewReader(nil), sink)
			request.Stream.FailureDestination = sink
			got, err := client.RoundTrip(t.Context(), request, streamPolicy())
			if !errors.Is(err, io.ErrClosedPipe) || sink.written != sink.limit || got.Stream.Metadata.Bytes.Uint64() != uint64(sink.limit) {
				t.Fatalf("partial response = error:%v written:%d metadata:%d, want closed-pipe cause and %d acknowledged bytes", err, sink.written, got.Stream.Metadata.Bytes.Uint64(), sink.limit)
			}
			if status == http.StatusTooManyRequests && !errors.Is(err, core.ErrAIRateLimited) {
				t.Fatalf("partial refusal error = %v, want rate-limit identity alongside native sink failure", err)
			}
		})
	}
}

func TestOptionalProviderHeadersBindNativeAuthority(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		provider ai.Provider
		kind     ai.OptionalHeaderName
		field    string
		value    string
	}{
		{name: "OpenAI organization", provider: ai.OpenAI, kind: ai.OpenAIOrganization, field: core.AIOpenAIOrganizationHeader, value: "org-owned"},
		{name: "OpenAI project", provider: ai.OpenAI, kind: ai.OpenAIProject, field: core.AIOpenAIProjectHeader, value: "proj-owned"},
		{name: "Anthropic workspace", provider: ai.Anthropic, kind: ai.AnthropicWorkspace, field: core.AIAnthropicWorkspaceHeader, value: "wrkspc_owned"},
		{name: "Anthropic beta selection", provider: ai.Anthropic, kind: ai.AnthropicBeta, field: core.AIAnthropicBetaHeader, value: "structured-outputs-2025-11-13"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			seen := make(chan string, 1)
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				seen <- request.Header.Get(tc.field)
				w.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
			}))
			defer server.Close()
			client := nativeClient(t, server, tc.provider)
			route := claude.Route()
			if tc.provider == ai.OpenAI {
				route = ai.Route{Provider: ai.OpenAI, Operation: ai.Responses}
			}
			request := streamingRequest(t, route, bytes.NewReader(nil), io.Discard)
			value, err := exchange.NewHeaderValue(tc.value)
			if err != nil {
				t.Fatalf("NewHeaderValue() error = %v, want nil", err)
			}
			request.Options = []ai.OptionalHeader{{Name: tc.kind, Value: value}}
			_, err = client.RoundTrip(t.Context(), request, streamPolicy())
			if err != nil {
				t.Fatalf("RoundTrip(optional header) error = %v, want nil", err)
			}
			if got := <-seen; got != tc.value {
				t.Fatalf("native header = %q, want %q", got, tc.value)
			}
			request.Options = append(request.Options, request.Options[0])
			if err := request.Validate(); !errors.Is(err, core.ErrAIContract) {
				t.Fatalf("duplicate option error = %v, want contract refusal", err)
			}
			request.Options = request.Options[:1]
			request.Route.Provider = ai.Provider(3 - int(tc.provider))
			if err := request.Validate(); !errors.Is(err, core.ErrAIBinding) {
				t.Fatalf("foreign option authority error = %v, want binding refusal", err)
			}
		})
	}
}
