package googleidentity

import (
	"context"
	"errors"
	"github.com/deliri/primitive/v2026/temporal"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"testing/synctest"

	"github.com/deliri/primitive/v2026/core"
)

// Each case serves an actual TLS response to the official SDK. The signed
// input remains fixed, so the certificate response alone determines admission.
const verifierFormerCertificateCutoffBytes = 256 << 10

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
		{name: "certificate below former byte extent is admitted", response: paddedVerifierCertificate(t, verifierFormerCertificateCutoffBytes-1)},
		{name: "certificate at former byte extent is admitted", response: paddedVerifierCertificate(t, verifierFormerCertificateCutoffBytes)},
		{name: "certificate beyond former cutoff preserves signed identity", response: paddedVerifierCertificate(t, verifierFormerCertificateCutoffBytes+1)},
		{name: "extreme declaration reads actual bytes and preserves truncation", response: func(w http.ResponseWriter, _ *http.Request, _ []byte) {
			w.Header().Set("Content-Length", strconv.FormatInt(1<<62, 10))
			w.WriteHeader(http.StatusOK)
		}, wantErr: io.ErrUnexpectedEOF},
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

func paddedVerifierCertificate(t testing.TB, size int) func(http.ResponseWriter, *http.Request, []byte) {
	return func(w http.ResponseWriter, _ *http.Request, b []byte) {
		w.Header().Set("Content-Length", strconv.Itoa(size))
		// Only JSON whitespace changes; the real authority keys stay identical.
		writeVerifierCertificate(t, w, append(b, strings.Repeat(" ", size-len(b))...))
	}
}

func TestGoogleCloudVerifierCancellationWaitsForCertificateReadExit(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		preCancel  bool
		cancelRead bool
		wantCalls  uint64
		wantErr    error
	}{
		{name: "complete_certificate_preserves_exact_identity", wantCalls: 1},
		{name: "cancelled_body_read_joins_provider_exit", cancelRead: true, wantCalls: 1, wantErr: context.Canceled},
		{name: "cancelled_before_ingress_performs_no_certificate_request", preCancel: true, wantErr: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			duration, err := temporal.DurationFromSeconds(10)
			if err != nil {
				t.Fatal(err)
			}
			watchdog, stop, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: t.Context(), Duration: duration})
			if err != nil {
				t.Fatal(err)
			}
			defer stop()
			started, exited, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
			p := newVerifierTestProvider(t, func(w http.ResponseWriter, r *http.Request, body []byte) {
				defer close(exited)
				w.Header().Set("Content-Length", strconv.Itoa(len(body)))
				w.WriteHeader(http.StatusOK)
				if err := http.NewResponseController(w).Flush(); err != nil {
					t.Errorf("flush: %v", err)
				}
				close(started)
				select {
				case <-r.Context().Done():
					return
				case <-release:
					writeVerifierCertificate(t, w, body)
				}
			})
			bearer := p.sign(t, verifierTestHeader{Algorithm: verifierTestAlgorithm, KeyID: verifierTestKeyID}, verifierClaims(), false)
			verifier := p.verifier(t, verifierTestAudience)
			ctx, cancel := context.WithCancel(watchdog)
			defer cancel()
			if tc.preCancel {
				cancel()
			}
			type result struct {
				identity GoogleCloudVerifiedIdentity
				err      error
			}
			done := make(chan result, 1)
			go func() { identity, err := verifier.Verify(ctx, bearer); done <- result{identity, err} }()
			if !tc.preCancel {
				select {
				case <-started:
				case <-watchdog.Done():
					t.Fatal("certificate read started=false, want true")
				}
				if tc.cancelRead {
					cancel()
				} else {
					close(release)
				}
			}
			var got result
			select {
			case got = <-done:
			case <-watchdog.Done():
				t.Fatal("verification joined=false, want true")
			}
			if !tc.preCancel {
				select {
				case <-exited:
				case <-watchdog.Done():
					t.Fatal("provider joined=false, want true")
				}
			}
			want := GoogleCloudVerifiedIdentity{}
			if tc.wantErr == nil {
				want = verifierClaims().identity(t)
			}
			if !errors.Is(got.err, tc.wantErr) || got.identity != want || p.calls.Load() != tc.wantCalls {
				t.Fatalf("identity=%v error=%v calls=%d, want %v, %v, %d", got.identity, got.err, p.calls.Load(), want, tc.wantErr, tc.wantCalls)
			}
			if tc.wantErr != nil && !errors.Is(got.err, core.ErrGoogleIdentityContract) {
				t.Fatalf("error=%v, want identity boundary", got.err)
			}
		})
	}
}
