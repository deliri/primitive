package cloudflare

import (
	"errors"
	"net/http"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// Exhaust the complete 3 x 2 x 2 envelope truth domain: success is absent,
// false or true; errors are absent or present; the result is empty or valid.
// Representation, identity and byte-bound hostility are separately fuzzed.
func TestAPIEnvelopeTruthDomainExhaustiveHandoff(t *testing.T) {
	t.Parallel()
	yes, no := true, false
	for _, tc := range []struct {
		success        *bool
		name           string
		issue          bool
		result         bool
		wantCapability bool
		wantRefusal    bool
	}{
		{name: "absent success with no facts cannot invent authority"},
		{name: "absent success cannot authorize valid-looking result", result: true},
		{name: "absent success preserves error instead of authority", issue: true},
		{name: "absent success with conflicting facts fails closed", issue: true, result: true},
		{name: "false success with empty facts remains typed refusal", success: &no, wantRefusal: true},
		{name: "false success cannot leak valid-looking result", success: &no, result: true, wantRefusal: true},
		{name: "false success retains provider error facts", success: &no, issue: true, wantRefusal: true},
		{name: "false success and provider errors outweigh populated result", success: &no, issue: true, result: true, wantRefusal: true},
		{name: "true success cannot invent missing upload result", success: &yes},
		{name: "true success and valid result issue exact authority", success: &yes, result: true, wantCapability: true},
		{name: "true success contradicts provider errors without result", success: &yes, issue: true},
		{name: "true success contradicts provider errors beside valid result", success: &yes, issue: true, result: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			envelope := apiEnvelope[imageDirectUploadWire]{Success: tc.success}
			if tc.result {
				envelope.Result = imageDirectUploadWire{ID: "draft", UploadURL: "https://" + core.CloudflareImagesUploadHost + "/one-use"}
			}
			if tc.issue {
				envelope.Errors = []APIIssue{{Code: 1000, Message: "provider refusal"}}
			}
			wire, err := core.MarshalCanonicalJSONDocument(envelope)
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			client, err := exchange.NewClient(&http.Client{Transport: responseTransport{data: wire, calls: &calls}})
			if err != nil {
				t.Fatal(err)
			}
			server, err := NewImagesServer(client, testOptions(t))
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := server.Close(); err != nil {
					t.Error(err)
				}
			}()
			got, gotErr := server.CreateDirectUpload(t.Context(), ImageDirectUploadRequest{}, testPolicy())
			if calls != 1 {
				t.Fatalf("producer calls=%d, want one", calls)
			}
			if (gotErr == nil) != tc.wantCapability {
				t.Fatalf("authority error=%v, want issued=%t", gotErr, tc.wantCapability)
			}
			var refusal APIRefusal
			if isRefusal := errors.As(gotErr, &refusal); isRefusal != tc.wantRefusal {
				t.Fatalf("typed refusal=%t, want %t", isRefusal, tc.wantRefusal)
			}
			if tc.wantRefusal && len(refusal.Issues) != len(envelope.Errors) {
				t.Fatalf("retained refusal issues=%d, want %d", len(refusal.Issues), len(envelope.Errors))
			}
			if tc.wantCapability {
				if got.ID.value != envelope.Result.ID || got.endpoint.String() != envelope.Result.UploadURL {
					t.Fatalf("issued identity = %q, endpoint matches = %v, want %q and true", got.ID.value, got.endpoint.String() == envelope.Result.UploadURL, envelope.Result.ID)
				}
			} else if !errors.Is(gotErr, core.ErrCloudflareResponse) || got != (ImageUpload{}) {
				t.Fatalf("rejected result=(%v,%v), want zero authority and typed response failure", got, gotErr)
			}
		})
	}
}
