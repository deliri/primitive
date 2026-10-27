package googleidentity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/deliri/primitive/v2026/core"
)

func TestGoogleCloudVerificationRequestGrammarAndRedaction(t *testing.T) {
	t.Parallel()
	p := newVerifierTestProvider(t, nil)
	header := verifierTestHeader{Algorithm: verifierTestAlgorithm, KeyID: verifierTestKeyID}
	claims := verifierClaims()
	canonical := p.sign(t, header, claims, false)
	claims.Subject = strings.Repeat("s", 8192)
	large := p.sign(t, header, claims, false)
	if len(large) <= 8192 {
		t.Fatalf("large signed credential = %d bytes, want above outbound-header quota", len(large))
	}
	for _, tc := range []struct {
		name, authorization string
		want                error
	}{
		{name: "genuine typed signed document admits grammar", authorization: canonical},
		{name: "genuine 8192-byte subject has no inbound quota", authorization: large},
		{name: "foreign signature remains a grammatical request not authenticated proof", authorization: p.sign(t, header, verifierClaims(), true)},
		{name: "absent request has no credential", want: core.ErrGoogleIdentityContract},
		{name: "absent token cannot form a request", authorization: googleCloudIdentityBearerPrefix, want: core.ErrGoogleIdentityContract},
		{name: "missing scheme is not inferred", authorization: strings.TrimPrefix(canonical, googleCloudIdentityBearerPrefix), want: core.ErrGoogleIdentityContract},
		{name: "noncanonical scheme is not repaired", authorization: strings.Replace(canonical, "Bearer ", "bearer ", 1), want: core.ErrGoogleIdentityContract},
		{name: "leading whitespace is not trimmed", authorization: " " + canonical, want: core.ErrGoogleIdentityContract},
		{name: "trailing whitespace is not trimmed", authorization: canonical + " ", want: core.ErrGoogleIdentityContract},
		{name: "line delimiter cannot carry two credentials", authorization: canonical + "\n" + canonical, want: core.ErrGoogleIdentityContract},
		{name: "opaque nonJWT bearer cannot validate a JOSE header", authorization: googleCloudIdentityBearerPrefix + "opaque", want: core.ErrGoogleIdentityContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request, err := NewGoogleCloudVerificationRequest(tc.authorization)
			if !errors.Is(err, tc.want) || err != nil && request != (GoogleCloudVerificationRequest{}) {
				t.Fatalf("NewGoogleCloudVerificationRequest = %v, want %v", err, tc.want)
			}
			if err == nil && (request.Validate() != nil || request.authorization == nil || *request.authorization != tc.authorization) {
				t.Fatal("constructor changed admitted credential or returned invalid nominal input")
			}
			for _, format := range []string{"%v", "%+v", "%#v", "%s", "%q", "%x"} {
				if got := fmt.Sprintf(format, request); got != core.RedactedValueText {
					t.Fatalf("request format %s = %q, want %q", format, got, core.RedactedValueText)
				}
			}
		})
	}
}

type nilVerificationContext struct{ context.Context }

func TestGoogleCloudVerificationRequestContextRefusalZeroFacts(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		context func(*testing.T) context.Context
		want    error
	}{
		{name: "nil context cannot start certificate discovery", context: func(*testing.T) context.Context { return nil }, want: core.ErrNilContext},
		{name: "typed nil context cannot start certificate discovery", context: func(*testing.T) context.Context { var ctx *nilVerificationContext; return ctx }, want: core.ErrContextObservation},
		{name: "cancelled context cannot start certificate discovery", context: func(t *testing.T) context.Context {
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			return ctx
		}, want: context.Canceled},
		{name: "expired context cannot start certificate discovery", context: func(t *testing.T) context.Context {
			ctx, cancel := context.WithDeadline(t.Context(), time.Unix(1, 0))
			t.Cleanup(cancel)
			return ctx
		}, want: context.DeadlineExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := newVerifierTestProvider(t, nil)
			request, parseErr := NewGoogleCloudVerificationRequest(p.sign(t, verifierTestHeader{Algorithm: verifierTestAlgorithm, KeyID: verifierTestKeyID}, verifierClaims(), false))
			if parseErr != nil {
				t.Fatal(parseErr)
			}
			if err := request.Validate(); err != nil {
				t.Fatal(err)
			}
			got, err := p.verifier(t, verifierTestAudience).Verify(tc.context(t), request)
			if !errors.Is(err, core.ErrGoogleIdentityContract) || !errors.Is(err, tc.want) || got != (GoogleCloudVerifiedIdentity{}) || p.calls.Load() != 0 {
				t.Fatalf("context refusal = (%+v,%v,%d certificates), want zero,contract/%v,0", got, err, p.calls.Load(), tc.want)
			}
		})
	}
}

// fmt bypasses Formatter for invalid %p/%w diagnostics. The fallback must not
// gain credential bytes by reflecting the request's underlying representation.
func TestGoogleCloudVerificationRequestInvalidVerbPrivacy(t *testing.T) {
	t.Parallel()
	p := newVerifierTestProvider(t, nil)
	canonical := p.sign(t, verifierTestHeader{Algorithm: verifierTestAlgorithm, KeyID: verifierTestKeyID}, verifierClaims(), false)
	request, err := NewGoogleCloudVerificationRequest(canonical)
	if err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"%p", "%w"} {
		t.Run(format, func(t *testing.T) {
			t.Parallel()
			for _, got := range []string{fmt.Sprintf(format, request), fmt.Sprintf(format, &request)} {
				if strings.Contains(got, canonical) {
					t.Fatalf("invalid-format %s credential disclosed = true, want false", format)
				}
			}
		})
	}
}

// This exercises the opaque constructor through the actual public SDK effect,
// including the signed extent that ruled out the generic outbound HeaderValue.
func TestGoogleCloudVerificationRequestConstructorNativeBinding(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name           string
		large, foreign bool
		want           error
	}{
		{name: "opaque ordinary request retains exact signed principal"},
		{name: "opaque large request preserves genuine 8192-byte subject", large: true},
		{name: "opaque foreign signature cannot become identity", foreign: true, want: core.ErrGoogleIdentityContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				p := newVerifierTestProvider(t, nil)
				claims := verifierClaims()
				if tc.large {
					claims.Subject = strings.Repeat("s", 8192)
				}
				bearer := p.sign(t, verifierTestHeader{Algorithm: verifierTestAlgorithm, KeyID: verifierTestKeyID}, claims, tc.foreign)
				request, err := NewGoogleCloudVerificationRequest(bearer)
				if err != nil || request.Validate() != nil {
					t.Fatalf("typed constructor = %v, want admitted request", err)
				}
				got, err := p.verifier(t, verifierTestAudience).Verify(t.Context(), request)
				want := GoogleCloudVerifiedIdentity{}
				if tc.want == nil {
					want = claims.identity(t)
				}
				if !errors.Is(err, tc.want) || got != want || p.calls.Load() != 1 {
					t.Fatalf("opaque SDK request = (exact identity %t,error %v,%d certificates), want true,%v,1", got == want, err, p.calls.Load(), tc.want)
				}
			})
		})
	}
}
