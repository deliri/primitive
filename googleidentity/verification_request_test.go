package googleidentity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
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
			request := GoogleCloudVerificationRequest{Authorization: tc.authorization}
			if err := request.Validate(); !errors.Is(err, tc.want) {
				t.Fatalf("request.Validate = %v, want %v", err, tc.want)
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
			request := GoogleCloudVerificationRequest{Authorization: p.sign(t, verifierTestHeader{Algorithm: verifierTestAlgorithm, KeyID: verifierTestKeyID}, verifierClaims(), false)}
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
