package cloudidentity

import (
	"context"
	"errors"
	"maps"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"google.golang.org/api/idtoken"
)

func TestGoogleCloudVerifierConstructionLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive validated audience constructs the bounded official verifier", func(t *testing.T) {
		t.Parallel()
		got, gotErr := NewGoogleCloudVerifier(t.Context(), GoogleCloudVerifierConfiguration{
			Audience: mustAudience(t, "https://api.example.invalid"),
		})
		if gotErr != nil || got.Validate() != nil {
			t.Fatalf("NewGoogleCloudVerifier(valid audience) = (%v, %v), want validated verifier and nil", got, gotErr)
		}
	})

	t.Run("negative zero audience creates no verifier capability", func(t *testing.T) {
		t.Parallel()
		got, gotErr := NewGoogleCloudVerifier(t.Context(), GoogleCloudVerifierConfiguration{})
		if got != (GoogleCloudVerifier{}) || !errors.Is(gotErr, core.ErrCloudIdentityContract) {
			t.Fatalf("NewGoogleCloudVerifier(zero audience) = (%v, %v), want zero and errors.Is(..., %v)", got, gotErr, core.ErrCloudIdentityContract)
		}
	})

	t.Run("neutral cancelled construction preserves cancellation without a verifier", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		got, gotErr := NewGoogleCloudVerifier(ctx, GoogleCloudVerifierConfiguration{
			Audience: mustAudience(t, "https://api.example.invalid"),
		})
		if got != (GoogleCloudVerifier{}) || !errors.Is(gotErr, context.Canceled) {
			t.Fatalf("NewGoogleCloudVerifier(cancelled) = (%v, %v), want zero and errors.Is(..., %v)", got, gotErr, context.Canceled)
		}
	})
}

func TestGoogleCloudVerifiedIdentityProjectionMatchesDocumentedClaims(t *testing.T) {
	t.Parallel()

	nominal := documentedGoogleServiceAccountPayload()
	want := documentedGoogleServiceAccountIdentity(t)
	type mutation func(*idtoken.Payload)
	cases := []struct {
		name     string
		mutate   mutation
		want     GoogleCloudVerifiedIdentity
		wantErr  error
		nilInput bool
	}{
		{name: "canonical Google issuer and verified service account email are admitted", want: want},
		{name: "legacy user-token issuer spelling is refused by the service-account contract", mutate: useLegacyIssuerInPayload, wantErr: core.ErrCloudIdentityContract},
		{name: "nil verified payload is refused", nilInput: true, wantErr: core.ErrCloudIdentityContract},
		{name: "foreign issuer is refused after SDK signature verification", mutate: useForeignIssuerInPayload, wantErr: core.ErrCloudIdentityContract},
		{name: "missing audience is refused", mutate: removeAudienceFromPayload, wantErr: core.ErrCloudIdentityContract},
		{name: "missing subject is refused", mutate: removeSubjectFromPayload, wantErr: core.ErrCloudIdentityContract},
		{name: "missing service account email is refused", mutate: removeEmailFromPayload, wantErr: core.ErrCloudIdentityContract},
		{name: "wrong-type service account email is refused", mutate: makeEmailTypeWrongInPayload, wantErr: core.ErrCloudIdentityContract},
		{name: "missing email verification claim is refused", mutate: removeEmailVerificationFromPayload, wantErr: core.ErrCloudIdentityContract},
		{name: "false email verification claim is refused", mutate: makeEmailUnverifiedInPayload, wantErr: core.ErrCloudIdentityContract},
		{name: "wrong-type email verification claim is refused", mutate: makeEmailVerificationTypeWrongInPayload, wantErr: core.ErrCloudIdentityContract},
		{name: "equal issue and expiry instants are refused", mutate: makeGoogleTokenLifetimeEmpty, wantErr: core.ErrCloudIdentityContract},
		{name: "issue instant after expiry is refused", mutate: reverseGoogleTokenLifetime, wantErr: core.ErrCloudIdentityContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			payload := cloneGooglePayload(nominal)
			if tc.mutate != nil {
				tc.mutate(&payload)
			}
			input := &payload
			if tc.nilInput {
				input = nil
			}
			got, gotErr := googleCloudVerifiedIdentity(input)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || got != (GoogleCloudVerifiedIdentity{}) {
					t.Fatalf("googleCloudVerifiedIdentity() = (%+v, %v), want zero and errors.Is(..., %v)", got, gotErr, tc.wantErr)
				}
				return
			}
			if gotErr != nil || got != tc.want {
				t.Fatalf("googleCloudVerifiedIdentity() = (%+v, %v), want (%+v, nil)", got, gotErr, tc.want)
			}
		})
	}
}

func documentedGoogleServiceAccountPayload() idtoken.Payload {
	return idtoken.Payload{
		Issuer: googleCloudIdentityIssuer, Audience: "https://api.example.invalid",
		IssuedAt: 100, Expires: 200, Subject: "112233445566778899001",
		Claims: map[string]any{
			googleCloudIdentityEmailClaim:         "runner-control@example-project.iam.gserviceaccount.com",
			googleCloudIdentityEmailVerifiedClaim: true,
		},
	}
}

func documentedGoogleServiceAccountIdentity(t testing.TB) GoogleCloudVerifiedIdentity {
	t.Helper()
	issuedAt, issuedAtErr := temporal.NewInstant(time.Unix(100, 0).UTC())
	expires, expiresErr := temporal.NewInstant(time.Unix(200, 0).UTC())
	if err := errors.Join(issuedAtErr, expiresErr); err != nil {
		t.Fatalf("documented Google service account instant error = %v, want nil", err)
	}
	return GoogleCloudVerifiedIdentity{
		Audience: "https://api.example.invalid", Issuer: googleCloudIdentityIssuer,
		Subject: "112233445566778899001", Email: "runner-control@example-project.iam.gserviceaccount.com",
		EmailVerified: true, IssuedAt: issuedAt, Expires: expires,
	}
}

func cloneGooglePayload(payload idtoken.Payload) idtoken.Payload {
	clone := payload
	clone.Claims = make(map[string]any, len(payload.Claims))
	maps.Copy(clone.Claims, payload.Claims)
	return clone
}

func useLegacyIssuerInPayload(payload *idtoken.Payload) {
	payload.Issuer = "accounts.google.com"
}
func useForeignIssuerInPayload(payload *idtoken.Payload) {
	payload.Issuer = "https://issuer.example.invalid"
}
func removeAudienceFromPayload(payload *idtoken.Payload) { payload.Audience = "" }
func removeSubjectFromPayload(payload *idtoken.Payload)  { payload.Subject = "" }
func removeEmailFromPayload(payload *idtoken.Payload) {
	delete(payload.Claims, googleCloudIdentityEmailClaim)
}
func makeEmailTypeWrongInPayload(payload *idtoken.Payload) {
	payload.Claims[googleCloudIdentityEmailClaim] = true
}
func removeEmailVerificationFromPayload(payload *idtoken.Payload) {
	delete(payload.Claims, googleCloudIdentityEmailVerifiedClaim)
}
func makeEmailUnverifiedInPayload(payload *idtoken.Payload) {
	payload.Claims[googleCloudIdentityEmailVerifiedClaim] = false
}
func makeEmailVerificationTypeWrongInPayload(payload *idtoken.Payload) {
	payload.Claims[googleCloudIdentityEmailVerifiedClaim] = "true"
}
func makeGoogleTokenLifetimeEmpty(payload *idtoken.Payload) { payload.Expires = payload.IssuedAt }
func reverseGoogleTokenLifetime(payload *idtoken.Payload)   { payload.IssuedAt = payload.Expires + 1 }
