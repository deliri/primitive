package timematch

import (
	"errors"
	"net/http"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestGoogleRevocationConvergesWithoutHidingFailures(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		status  int
		body    string
		want    RevokeOutcome
		wantErr error
	}{
		{"new revocation acknowledged", 200, "", RevokedAtProvider, nil},
		{"already invalid credential reaches terminal token outcome", 400, `{"error":"invalid_token","error_description":"private-marker"}`, RevokeTokenAlreadyInvalid, nil},
		{"already invalid with future metadata", 400, `{"error":"invalid_token","future":true}`, RevokeTokenAlreadyInvalid, nil},
		{"invalid grant retains consent classification", 400, `{"error":"invalid_grant"}`, RevokeInvalid, core.ErrTimeMatchConsent},
		{"invalid request is not already invalid", 400, `{"error":"invalid_request"}`, RevokeInvalid, core.ErrTimeMatchResponse},
		{"invalid client is not already invalid", 400, `{"error":"invalid_client"}`, RevokeInvalid, core.ErrTimeMatchResponse},
		{"unsupported type is not already invalid", 400, `{"error":"unsupported_token_type"}`, RevokeInvalid, core.ErrTimeMatchResponse},
		{"server refusal cannot prove revocation", 503, `{"error":"invalid_token"}`, RevokeInvalid, core.ErrTimeMatchResponse},
		{"unauthorized invalid token is not documented success", 401, `{"error":"invalid_token"}`, RevokeInvalid, core.ErrTimeMatchResponse},
		{"duplicate errors refuse", 400, `{"error":"invalid_request","error":"invalid_token"}`, RevokeInvalid, core.ErrTimeMatchResponse},
		{"null response refuses", 400, `null`, RevokeInvalid, core.ErrTimeMatchResponse},
		{"malformed response refuses", 400, `{"error":`, RevokeInvalid, core.ErrTimeMatchResponse},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, fixture := newNative(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) { writeJSON(w, tc.status, tc.body) })
			token := secret(t, "refresh-only-fixture")
			defer func() {
				if closeErr := token.Close(); closeErr != nil {
					t.Errorf("token.Close()=%v,want nil", closeErr)
				}
			}()
			got, err := client.Revoke(t.Context(), RevokeRequest{Provider: ProviderGoogle, Token: token, Policy: fixturePolicy(t)})
			if got != tc.want || !errors.Is(err, tc.wantErr) || (tc.wantErr == nil && err != nil) {
				t.Fatalf("Revoke()=(%v,%v), want(%v,%v)", got, err, tc.want, tc.wantErr)
			}
			requests := fixture.record()
			if len(requests) != 1 || requests[0].path != "/revoke" || requests[0].query != "" || string(requests[0].body) != "token=refresh-only-fixture" {
				t.Fatalf("revocation requests=%v, want one body-only token POST", requests)
			}
			if !token.IsSet() {
				t.Fatal("borrowed token destroyed, want caller custody retained")
			}
			noLeak(t, err, "refresh-only-fixture", "private-marker")
		})
	}
}
