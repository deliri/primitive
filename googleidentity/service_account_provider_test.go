package googleidentity

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"cloud.google.com/go/auth"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

const serviceAccountFixtureEmail = "worker@example.invalid"
const serviceAccountFixtureKeyID = "public-test-key"
const serviceAccountGrantType = "urn:ietf:params:oauth:grant-type:jwt-bearer"

type serviceAccountAssertionClaims struct {
	serviceAccountAudienceClaim
	Issuer   string `json:"iss"`
	Audience string `json:"aud"`
	Scope    string `json:"scope"`
	Subject  string `json:"sub,omitempty"`
	IssuedAt int64  `json:"iat"`
	Expires  int64  `json:"exp"`
}
type googleObservedTransport struct {
	base  http.RoundTripper
	calls atomic.Uint64
}

func (o *googleObservedTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	o.calls.Add(1)
	return o.base.RoundTrip(r)
}

func decodeServiceAccountAssertion(value string, key *rsa.PublicKey) (googleCloudJWTHeader, serviceAccountAssertionClaims, error) {
	var header googleCloudJWTHeader
	var claims serviceAccountAssertionClaims
	h, remainder, ok := strings.Cut(value, ".")
	if !ok {
		return header, claims, core.ErrGoogleIdentityContract
	}
	b, s, ok := strings.Cut(remainder, ".")
	if !ok {
		return header, claims, core.ErrGoogleIdentityContract
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(h)
	if err != nil {
		return header, claims, err
	}
	claimBytes, err := base64.RawURLEncoding.DecodeString(b)
	if err != nil {
		return header, claims, err
	}
	signature, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return header, claims, err
	}
	digest := sha256.Sum256([]byte(h + "." + b))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], signature); err != nil {
		return header, claims, err
	}
	header, err = core.DecodeStrictJSONBytes[googleCloudJWTHeader](headerBytes, core.ExtensibleJSONLimits())
	if err != nil {
		return header, claims, err
	}
	claims, err = core.DecodeStrictJSONStructure[serviceAccountAssertionClaims](claimBytes, core.ExtensibleJSONLimits())
	return header, claims, err
}
func TestGoogleServiceAccountProviderBoundaryLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		mutate     func([]byte) []byte
		status     int
		redirect   bool
		truncate   bool
		cancel     bool
		wantErr    error
		wantCalls  uint64
		wantStatus int
	}{
		{name: "signed_request_uses_exchange_and_exact_target_audience", wantCalls: 1},
		{name: "provider_json_whitespace_spans_many_windows", mutate: func(b []byte) []byte { return append([]byte(strings.Repeat(" ", 128<<10)), b...) }, wantCalls: 1},
		{name: "provider_extension_is_not_a_second_protocol", mutate: func(b []byte) []byte {
			return append(append([]byte{}, b[:len(b)-1]...), []byte(",\"scope\":\"provider-owned\"}")...)
		}, wantCalls: 1},
		{name: "provider_denial_preserves_sdk_status", status: http.StatusForbidden, wantCalls: 1, wantStatus: http.StatusForbidden, wantErr: core.ErrGoogleIdentityContract},
		{name: "redirect_cannot_bypass_exchange", redirect: true, wantCalls: 1, wantErr: core.ErrExchangeRedirect},
		{name: "truncated_declared_body_preserves_native_failure", truncate: true, wantCalls: 1, wantErr: io.ErrUnexpectedEOF},
		{name: "duplicate_token_member_is_refused", mutate: func(b []byte) []byte {
			return append(append([]byte{}, b[:len(b)-1]...), []byte(",\"id_token\":\"other\"}")...)
		}, wantCalls: 1, wantErr: core.ErrJSONContract},
		{name: "malformed_response_is_refused", mutate: func(b []byte) []byte { return b[:len(b)-1] }, wantCalls: 1, wantErr: core.ErrJSONContract},
		{name: "missing_identity_token_cannot_become_access_token", mutate: func([]byte) []byte { return []byte("{\"access_token\":\"opaque\",\"token_type\":\"Bearer\"}") }, wantCalls: 1, wantErr: core.ErrGoogleIdentityContract},
		{name: "empty_response_cannot_invent_token", mutate: func([]byte) []byte { return nil }, wantCalls: 1, wantErr: core.ErrGoogleIdentityContract},
		{name: "cancelled_intent_produces_no_request", cancel: true, wantErr: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			synctest.Test(t, func(t *testing.T) {
				signer := &verifierTestProvider{key: verifierTestKey(t, "testdata/verifier_rsa.pem")}
				signed := signer.sign(t, verifierTestHeader{Algorithm: verifierTestAlgorithm, KeyID: verifierTestKeyID}, verifierClaims(), false)
				response, err := core.MarshalCanonicalJSONDocument(serviceAccountFixtureResponse{IDToken: strings.TrimPrefix(signed, bearerPrefix)})
				if err != nil {
					t.Fatal(err)
				}
				if tc.mutate != nil {
					response = tc.mutate(response)
				}
				request := serviceAccountFixtureRequest(t)
				var calls atomic.Uint64
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls.Add(1)
					if err := r.ParseForm(); err != nil {
						t.Errorf("form error=%v, want nil", err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					header, claims, err := decodeServiceAccountAssertion(r.Form.Get("assertion"), &signer.key.PublicKey)
					if err != nil || header.Algorithm != googleCloudSigningAlgorithmRS256 || header.KeyID != serviceAccountFixtureKeyID || claims.Issuer != serviceAccountFixtureEmail || claims.TargetAudience != request.Audience || claims.Audience != "http://"+r.Host || claims.Expires <= claims.IssuedAt || claims.Subject != "" || claims.Scope != "" || r.Form.Get("grant_type") != serviceAccountGrantType || r.Method != http.MethodPost {
						t.Errorf("SDK assertion header=%v claims=%v method=%v error=%v, want exact signed service-account intent", header, claims, r.Method, err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					if tc.redirect {
						w.Header().Set("Location", "/unexpected")
						w.WriteHeader(http.StatusFound)
						return
					}
					if tc.truncate {
						w.Header().Set("Content-Length", strconv.Itoa(len(response)+1))
					}
					status := tc.status
					if status == 0 {
						status = http.StatusOK
					}
					w.WriteHeader(status)
					if _, err := w.Write(response); err != nil {
						t.Errorf("provider write error=%v, want nil", err)
					}
				}))
				defer server.Close()
				source := serviceAccountFixtureSource(t, dir, server.URL)
				standard, ok := http.DefaultTransport.(*http.Transport)
				if !ok {
					t.Fatalf("transport = %T, want *http.Transport", http.DefaultTransport)
				}
				base := standard.Clone()
				base.Proxy = nil
				defer base.CloseIdleConnections()
				observed := &googleObservedTransport{base: base}
				client, err := exchange.NewClient(&http.Client{Transport: observed})
				if err != nil {
					t.Fatal(err)
				}
				source.client = client
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				if tc.cancel {
					cancel()
				}
				got, err := source.Acquire(ctx, request)
				if !errors.Is(err, tc.wantErr) || calls.Load() != tc.wantCalls || observed.calls.Load() != tc.wantCalls {
					t.Fatalf("acquire error=%v server calls=%d exchange calls=%d, want %v and %d calls", err, calls.Load(), observed.calls.Load(), tc.wantErr, tc.wantCalls)
				}
				if tc.wantErr != nil {
					if got != (Token{}) || !errors.Is(err, core.ErrGoogleIdentityContract) {
						t.Fatalf("refused token=%v error=%v, want zero typed refusal", got, err)
					}
					if tc.wantStatus != 0 {
						var providerError *auth.Error
						if !errors.As(err, &providerError) || providerError.Response.StatusCode != tc.wantStatus {
							t.Fatalf("provider error=%v, want typed status %d", err, tc.wantStatus)
						}
					}
					return
				}
				value, err := got.BearerValue()
				if err != nil || value != signed {
					t.Fatalf("returned token matches=%t error=%v, want exact provider token", value == signed, err)
				}
			})
		})
	}
}
