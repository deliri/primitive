package cloudidentity

import (
	"testing"

	"github.com/deliri/primitive/v2026/temporal"
)

func TestGoogleCloudPrincipalIdentityFollowsDocumentedOIDCIdentity(t *testing.T) {
	t.Parallel()

	baseline := GoogleCloudVerifiedIdentity{
		Audience:      "https://api.example.invalid",
		Issuer:        "https://accounts.google.com",
		Subject:       "112233445566778899001",
		Email:         "runner-control@example-project.iam.gserviceaccount.com",
		EmailVerified: true,
		IssuedAt:      temporal.InstantFromNanoseconds(100),
		Expires:       temporal.InstantFromNanoseconds(200),
	}
	want, err := baseline.PrincipalIdentity()
	if err != nil {
		t.Fatalf("GoogleCloudVerifiedIdentity.PrincipalIdentity(baseline) error = %v, want nil", err)
	}
	cases := []struct {
		name     string
		mutate   func(*GoogleCloudVerifiedIdentity)
		wantSame bool
	}{
		{name: "renewed token preserves principal identity", mutate: renewGoogleIdentityToken, wantSame: true},
		{name: "another audience preserves the same principal", mutate: changeGoogleIdentityAudience, wantSame: true},
		{name: "changed email address preserves the same principal", mutate: changeGoogleIdentityEmail, wantSame: true},
		{name: "another subject changes workload identity", mutate: changeGoogleIdentitySubject},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			identity := baseline
			tc.mutate(&identity)
			got, gotErr := identity.PrincipalIdentity()
			if gotErr != nil {
				t.Fatalf("GoogleCloudVerifiedIdentity.PrincipalIdentity() error = %v, want nil", gotErr)
			}
			if gotSame := got == want; gotSame != tc.wantSame {
				t.Fatalf("credential identity equal baseline = %t, want %t", gotSame, tc.wantSame)
			}
		})
	}
}

func renewGoogleIdentityToken(identity *GoogleCloudVerifiedIdentity) {
	identity.IssuedAt = temporal.InstantFromNanoseconds(300)
	identity.Expires = temporal.InstantFromNanoseconds(400)
}

func changeGoogleIdentityAudience(identity *GoogleCloudVerifiedIdentity) {
	identity.Audience = "https://runner.example.invalid"
}

func changeGoogleIdentitySubject(identity *GoogleCloudVerifiedIdentity) {
	identity.Subject = "998877665544332211000"
}

func changeGoogleIdentityEmail(identity *GoogleCloudVerifiedIdentity) {
	identity.Email = "another-control@example-project.iam.gserviceaccount.com"
}
