package googleidentity

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/deliri/primitive/v2026/core"
)

// Each case serves an actual TLS response to the official SDK. The signed
// input remains fixed, so the certificate response alone determines admission.
func TestGoogleCloudVerifierCertificateLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		response func(http.ResponseWriter, *http.Request, []byte)
		wantErr  error
	}{
		{name: "trusted certificate yields exact signed identity"},
		{name: "empty certificate response yields no proof", response: func(w http.ResponseWriter, _ *http.Request, _ []byte) { w.WriteHeader(http.StatusNoContent) }, wantErr: core.ErrGoogleIdentityContract},
		{name: "empty key set cannot admit a signed principal", response: func(w http.ResponseWriter, _ *http.Request, _ []byte) {
			body, err := core.MarshalCanonicalJSONDocument(verifierTestKeySet{Keys: []verifierTestJWK{}})
			if err != nil {
				t.Errorf("empty key set encoding error = %v, want nil", err)
				return
			}
			writeVerifierCertificate(t, w, body)
		}, wantErr: core.ErrGoogleIdentityContract},
		{name: "provider denial preserves refusal", response: func(w http.ResponseWriter, _ *http.Request, b []byte) {
			w.WriteHeader(http.StatusForbidden)
			writeVerifierCertificate(t, w, b)
		}, wantErr: core.ErrGoogleIdentityContract},
		{name: "redirect never fetches a second authority", response: func(w http.ResponseWriter, r *http.Request, _ []byte) {
			w.Header().Set("Location", "/other-authority")
			w.WriteHeader(http.StatusFound)
		}, wantErr: core.ErrExchangeRedirect},
		{name: "truncated declared certificate response preserves read failure", response: func(w http.ResponseWriter, _ *http.Request, b []byte) {
			w.Header().Set("Content-Length", strconv.Itoa(len(b)+1))
			writeVerifierCertificate(t, w, b)
		}, wantErr: io.ErrUnexpectedEOF},
		{name: "malformed certificate JSON preserves syntax refusal", response: func(w http.ResponseWriter, _ *http.Request, b []byte) { writeVerifierCertificate(t, w, b[:len(b)-1]) }, wantErr: core.ErrJSONContract},
		{name: "certificate one below byte ceiling is admitted", response: paddedVerifierCertificate(GoogleCloudIdentityCertificateMaximumBytes - 1)},
		{name: "certificate at byte ceiling is admitted", response: paddedVerifierCertificate(GoogleCloudIdentityCertificateMaximumBytes)},
		{name: "certificate one above byte ceiling yields no partial proof", response: paddedVerifierCertificate(GoogleCloudIdentityCertificateMaximumBytes + 1), wantErr: core.ErrExchangeBodyLimit},
		{name: "extreme declared certificate size is refused before allocation", response: func(w http.ResponseWriter, _ *http.Request, _ []byte) {
			w.Header().Set("Content-Length", strconv.FormatInt(1<<62, 10))
			w.WriteHeader(http.StatusOK)
		}, wantErr: core.ErrExchangeBodyLimit},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				p := newVerifierTestProvider(t, tc.response)
				claims := verifierClaims()
				bearer := p.sign(t, verifierTestHeader{Algorithm: verifierTestAlgorithm, KeyID: verifierTestKeyID}, claims, false)
				got, err := p.verifier(t, verifierTestAudience).Verify(t.Context(), bearer)
				if calls := p.calls.Load(); calls != 1 {
					t.Fatalf("certificate requests = %d, want 1", calls)
				}
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("Verify() error = %v, want %v", err, tc.wantErr)
				}
				if tc.wantErr != nil {
					if !errors.Is(err, core.ErrGoogleIdentityContract) || got != (GoogleCloudVerifiedIdentity{}) {
						t.Fatalf("refused verification = (%+v, %v), want zero and typed boundary refusal", got, err)
					}
					return
				}
				if want := claims.identity(t); got != want {
					t.Fatalf("verified identity = %+v, want %+v", got, want)
				}
			})
		})
	}
}

func writeVerifierCertificate(t testing.TB, w http.ResponseWriter, body []byte) {
	t.Helper()
	if _, err := w.Write(body); err != nil {
		t.Errorf("certificate Write() error = %v, want nil", err)
	}
}

func paddedVerifierCertificate(size int) func(http.ResponseWriter, *http.Request, []byte) {
	return func(w http.ResponseWriter, _ *http.Request, b []byte) {
		w.Header().Set("Content-Length", strconv.Itoa(size))
		// Only JSON whitespace changes; the real authority keys stay identical.
		_, _ = w.Write(append(b, strings.Repeat(" ", size-len(b))...))
	}
}

func TestGoogleCloudVerifierCancellationWaitsForCertificateReadExit(t *testing.T) {
	t.Parallel()
	started, exited := make(chan struct{}), make(chan struct{})
	p := newVerifierTestProvider(t, func(w http.ResponseWriter, r *http.Request, _ []byte) {
		defer close(exited)
		w.Header().Set("Content-Length", "128")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		close(started)
		<-r.Context().Done()
	})
	bearer := p.sign(t, verifierTestHeader{Algorithm: verifierTestAlgorithm, KeyID: verifierTestKeyID}, verifierClaims(), false)
	v := p.verifier(t, verifierTestAudience)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	type result struct {
		identity GoogleCloudVerifiedIdentity
		err      error
	}
	done := make(chan result, 1)
	go func() { identity, err := v.Verify(ctx, bearer); done <- result{identity, err} }()
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("certificate read started = false, want true")
	}
	cancel()
	select {
	case got := <-done:
		if got.identity != (GoogleCloudVerifiedIdentity{}) || !errors.Is(got.err, context.Canceled) || !errors.Is(got.err, core.ErrGoogleIdentityContract) {
			t.Fatalf("cancelled Verify() = (%+v, %v), want zero and preserved cancellation", got.identity, got.err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("verifier exited = false, want true")
	}
	select {
	case <-exited:
	case <-time.After(10 * time.Second):
		t.Fatal("certificate handler exited = false, want true")
	}
	if calls := p.calls.Load(); calls != 1 {
		t.Fatalf("certificate requests = %d, want 1", calls)
	}
}
