package timematch

import (
	json "encoding/json/v2"
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

// The HTTP provider is local. Native JSON decoding is an independent error-code
// oracle; the public Revoke operation must preserve its status-dependent class.
func FuzzGoogleRevocationResponseSemanticClosure(f *testing.F) {
	for _, code := range []string{"invalid_token", "invalid_grant", "invalid_request", "consent_required", "interaction_required"} {
		wire, err := json.Marshal(struct {
			Error string `json:"error"`
		}{Error: code})
		if err != nil {
			f.Fatalf("providerseed=%v,wantnil", err)
		}
		f.Add(uint8(1), wire)
	}
	f.Add(uint8(0), []byte{})
	f.Add(uint8(1), []byte(`{"error":"invalid_request","error":"invalid_token"}`))
	f.Add(uint8(2), []byte(`null`))
	f.Add(uint8(3), []byte(`{"error":`))
	f.Fuzz(func(t *testing.T, selector uint8, body []byte) {
		statuses := [4]int{200, 400, 401, 503}
		status := statuses[int(selector)%len(statuses)]
		var document struct {
			Error string `json:"error"`
		}
		decodeErr := json.Unmarshal(body, &document)
		want := RevokeInvalid
		var wantErr error = core.ErrTimeMatchResponse
		if status == 200 {
			want, wantErr = RevokedAtProvider, nil
		} else if decodeErr == nil {
			if status == 400 && document.Error == "invalid_token" {
				want, wantErr = RevokeTokenAlreadyInvalid, nil
			} else if (status == 400 || status == 401) && (document.Error == "invalid_grant" || document.Error == "consent_required" || document.Error == "interaction_required") {
				wantErr = core.ErrTimeMatchConsent
			}
		}
		client, fixture := newNative(t, func(w http.ResponseWriter, _ *http.Request, _ []byte) { writeJSON(w, status, string(body)) })
		token := secret(t, "borrowed-revocation-secret")
		got, err := client.Revoke(t.Context(), RevokeRequest{Provider: ProviderGoogle, Token: token, Policy: fixturePolicy(t)})
		borrowed := token.IsSet()
		closeErr := token.Close()
		if closeErr != nil || !borrowed {
			t.Fatalf("borrowedtoken=%t,cleanup=%v,wanttrue,nil", borrowed, closeErr)
		}
		if got != want || !errors.Is(err, wantErr) {
			t.Fatalf("Revoke(status%d,decoded%q,error%v)=(%v,%v),want(%v,%v)", status, document.Error, decodeErr, got, err, want, wantErr)
		}
		if len(fixture.record()) != 1 {
			t.Fatalf("requests%d,wantone single-attempt revocation", len(fixture.record()))
		}
		noLeak(t, err, "borrowed-revocation-secret")
	})
}
