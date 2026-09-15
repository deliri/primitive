package permit

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
	"github.com/deliri/primitive/v2026/temporal"
)

func buildTransferFixture(t testing.TB) (BuildTransferIssuance, BuildTransferVerification) {
	t.Helper()
	registration, _, request := signedResponseFixtures(t)
	_, signer := permitFixture(t)
	authority, err := controlplane.NewAuthority(controlplane.AuthorityConfiguration{TrustedAuthorityKeys: request.TrustedKeys})
	if err != nil {
		t.Fatalf("NewAuthority = %v, want nil", err)
	}
	certificate, err := authority.VerifyInstallationCertificate(*registration.Registration.Payload.Certificate)
	if err != nil {
		t.Fatalf("VerifyInstallationCertificate = %v, want nil", err)
	}
	terms := request.Document.Terms
	terms.Reporting = reportSchedule(t)
	request.Document, err = Sign(terms, signer)
	if err != nil {
		t.Fatalf("Sign scheduled permission = %v, want nil", err)
	}
	permission, err := Verify(request)
	if err != nil {
		t.Fatalf("Verify prior permission = %v, want nil", err)
	}
	build, err := core.NewBuildIdentity(core.BuildIdentityRequest{Offering: request.Build.Offering(), Version: core.NewReleaseVersion(2026, 1, 99), Commit: request.Build.Commit(), Platform: request.Build.Platform()})
	if err != nil || build == request.Build {
		t.Fatalf("candidate build = %v/%v, want changed valid build", build, err)
	}
	issuance := BuildTransferIssuance{Certificate: certificate, Permission: permission, Build: build, Authority: authority, Signer: signer}
	document, err := IssueBuildTransfer(issuance)
	if err != nil {
		t.Fatalf("IssueBuildTransfer = %v, want nil", err)
	}
	verification := BuildTransferVerification{Document: document, PreviousCertificate: certificate, PreviousPermission: permission, TrustedKeys: request.TrustedKeys, Build: build, EffectiveAt: request.EffectiveAt}
	return issuance, verification
}

// Direct mechanical ratchet: the API still owns publication selection,
// revocation, transaction/replay, and retention. This is not HTTP evidence.
func TestBuildTransferPermissionLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		mutate  func(*testing.T, *BuildTransferVerification)
		wantErr error
	}{
		{"changed build retains every other signed fact", func(*testing.T, *BuildTransferVerification) {}, nil},
		{"expiry one nanosecond below remains valid", func(t *testing.T, r *BuildTransferVerification) {
			r.EffectiveAt = shiftTransferInstant(t, r.Document.Permission.Terms.ExpiresAt, -1)
		}, nil},
		{"exact expiry cannot become a renewal", func(t *testing.T, r *BuildTransferVerification) {
			r.EffectiveAt = r.Document.Permission.Terms.ExpiresAt
		}, core.ErrPermitValidity},
		{"one nanosecond after expiry stays expired", func(t *testing.T, r *BuildTransferVerification) {
			r.EffectiveAt = shiftTransferInstant(t, r.Document.Permission.Terms.ExpiresAt, +1)
		}, core.ErrPermitValidity},
		{"before original activation cannot execute", func(t *testing.T, r *BuildTransferVerification) {
			r.EffectiveAt = shiftTransferInstant(t, r.Document.Permission.Terms.NotBefore, -1)
		}, core.ErrPermitValidity},
		{"same build cannot claim a transfer", func(t *testing.T, r *BuildTransferVerification) {
			r.Build = r.Document.Certificate.Body.Build
			r.Document.Permission.Terms.Build, _ = priorBuild(r.PreviousPermission)
		}, core.ErrPermitBinding},
		{"extra action cannot ride a build change", func(t *testing.T, r *BuildTransferVerification) {
			r.Document.Permission.Terms.Actions, _ = NewActions(permitAction(t, "operation-a"), permitAction(t, "operation-b"))
		}, core.ErrPermitBinding},
		{"removed action is not an exact transfer", func(t *testing.T, r *BuildTransferVerification) { r.Document.Permission.Terms.Actions = Actions{} }, core.ErrPermitBinding},
		{"expiry extension cannot ride a build change", func(t *testing.T, r *BuildTransferVerification) {
			r.Document.Permission.Terms.ExpiresAt = shiftTransferInstant(t, r.Document.Permission.Terms.ExpiresAt, +1)
		}, core.ErrPermitBinding},
		{"activation shift cannot reset elapsed permission", func(t *testing.T, r *BuildTransferVerification) {
			r.Document.Permission.Terms.NotBefore = shiftTransferInstant(t, r.Document.Permission.Terms.NotBefore, -1)
		}, core.ErrPermitBinding},
		{"contact shift cannot postpone renewal", func(t *testing.T, r *BuildTransferVerification) {
			r.Document.Permission.Terms.ContactAt = shiftTransferInstant(t, r.Document.Permission.Terms.ContactAt, +1)
		}, core.ErrPermitBinding},
		{"retry delay cannot be replaced", func(t *testing.T, r *BuildTransferVerification) {
			r.Document.Permission.Terms.RetryAfter, _ = temporal.DurationFromMilliseconds(2000)
		}, core.ErrPermitBinding},
		{"generation advance cannot manufacture usage", func(t *testing.T, r *BuildTransferVerification) {
			r.Document.Permission.Terms.Generation, _ = lease.NewGeneration(2)
		}, core.ErrPermitBinding},
		{"nonce substitution cannot replace the prior response", func(t *testing.T, r *BuildTransferVerification) {
			r.Document.Permission.Terms.RequestNonce, _ = controlwire.NewRequestNonce([32]byte{99})
		}, core.ErrPermitBinding},
		{"reporting removal cannot disable the registered phase", func(t *testing.T, r *BuildTransferVerification) {
			r.Document.Permission.Terms.Reporting = ReportSchedule{}
		}, core.ErrPermitBinding},
		{"reporting opening cannot move with upgrade time", func(t *testing.T, r *BuildTransferVerification) {
			r.Document.Permission.Terms.Reporting.NextReportAt = shiftTransferInstant(t, r.Document.Permission.Terms.Reporting.NextReportAt, 1)
		}, core.ErrPermitBinding},
		{"certificate date cannot reenroll the key", func(t *testing.T, r *BuildTransferVerification) {
			r.Document.Certificate.Body.IssuedAt = shiftTransferInstant(t, r.Document.Certificate.Body.IssuedAt, +1)
		}, core.ErrPermitBinding},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			issuance, r := buildTransferFixture(t)
			before := r.Document
			tc.mutate(t, &r)
			// Re-sign altered facts so the binding oracle, rather than a stale
			// signature, must reject authority-authentic incompatible terms.
			var err error
			r.Document.Permission, err = Sign(r.Document.Permission.Terms, issuance.Signer)
			if err != nil {
				t.Fatalf("Sign mutated permission = %v, want nil", err)
			}
			r.Document.Certificate, err = issuance.Authority.IssueInstallationCertificate(r.Document.Certificate.Body, issuance.Signer)
			if err != nil {
				t.Fatalf("Sign mutated certificate = %v, want nil", err)
			}
			got, err := VerifyBuildTransfer(r)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("VerifyBuildTransfer = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (VerifiedBuildTransfer{}) {
					t.Fatalf("refused proof = %v, want zero", got)
				}
				if r.Document == before && r.EffectiveAt == before.Permission.Terms.NotBefore {
					t.Fatal("mutation changed = false, want true")
				}
				return
			}
			permission, err := got.Permission()
			if err != nil {
				t.Fatalf("Permission = %v, want nil", err)
			}
			terms, err := permission.Terms()
			if err != nil || terms != before.Permission.Terms {
				t.Fatalf("terms = %+v/%v, want %+v/nil", terms, err, before.Permission.Terms)
			}
			if err := permission.Allows(permitAction(t, "operation-a"), r.EffectiveAt); err != nil {
				t.Fatalf("transferred action = %v, want nil", err)
			}
		})
	}
	t.Run("empty rights remain empty after transfer", func(t *testing.T) {
		t.Parallel()
		issuance, r := buildTransferFixture(t)
		terms, err := issuance.Permission.Terms()
		if err != nil {
			t.Fatalf("Terms = %v, want nil", err)
		}
		terms.Actions = Actions{}
		terms.Reporting = ReportSchedule{}
		document, err := Sign(terms, issuance.Signer)
		if err != nil {
			t.Fatalf("Sign empty rights = %v, want nil", err)
		}
		issuance.Permission, err = Verify(VerifyRequest{Document: document, RequestNonce: terms.RequestNonce, TrustedKeys: r.TrustedKeys, Subject: terms.Subject, Build: terms.Build, EffectiveAt: r.EffectiveAt, MinimumGeneration: terms.Generation})
		if err != nil {
			t.Fatalf("Verify empty rights = %v, want nil", err)
		}
		r.PreviousPermission = issuance.Permission
		r.Document, err = IssueBuildTransfer(issuance)
		if err != nil {
			t.Fatalf("IssueBuildTransfer = %v, want nil", err)
		}
		got, err := VerifyBuildTransfer(r)
		if err != nil {
			t.Fatalf("VerifyBuildTransfer = %v, want nil", err)
		}
		permission, err := got.Permission()
		if err != nil {
			t.Fatalf("Permission = %v, want nil", err)
		}
		if err := permission.Allows(permitAction(t, "operation-a"), r.EffectiveAt); !errors.Is(err, core.ErrPermitAction) {
			t.Fatalf("empty rights action = %v, want ErrPermitAction", err)
		}
		want := terms
		want.Build = issuance.Build
		gotTerms, err := permission.Terms()
		if err != nil || gotTerms != want {
			t.Fatalf("empty rights terms = %+v/%v, want %+v/nil", gotTerms, err, want)
		}
	})
}

func priorBuild(v Verified) (core.BuildIdentity, error) {
	terms, err := v.Terms()
	return terms.Build, err
}

func FuzzBuildTransferSemanticClosure(f *testing.F) {
	_, seed := buildTransferFixture(f)
	canonical, err := seed.Document.MarshalJSON()
	if err != nil {
		f.Fatalf("MarshalJSON seed = %v, want nil", err)
	}
	f.Add(canonical, uint8(0))
	f.Add(canonical, uint8(1))
	f.Add(canonical, uint8(2))
	f.Add([]byte{}, uint8(0))
	f.Add([]byte("null"), uint8(0))
	f.Fuzz(func(t *testing.T, data []byte, mutation uint8) {
		got := seed.Document
		if err := got.UnmarshalJSON(data); err != nil {
			if (!errors.Is(err, core.ErrPermitContract) && !errors.Is(err, core.ErrPermitBinding)) || got != seed.Document {
				t.Fatalf("decode refusal = %v/%v, want preserved typed refusal", got, err)
			}
			return
		}
		encoded, err := got.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON = %v, want nil", err)
		}
		var roundTrip BuildTransfer
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
			t.Fatalf("round trip = %v/%v, want %v/nil", roundTrip, err, got)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(encoded, second) {
			t.Fatalf("canonical equality = %t/%v, want true/nil", bytes.Equal(encoded, second), err)
		}
		// Authentic-looking foreign signatures reach the real verifier after
		// valid decoding. They cannot borrow the trusted authority identity.
		foreign := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{91}, ed25519.SeedSize))
		defer clear(foreign)
		switch mutation % 3 {
		case 0:
		case 1:
			got.Permission, err = Sign(got.Permission.Terms, foreign)
		case 2:
			authority, newErr := controlplane.NewAuthority(controlplane.AuthorityConfiguration{TrustedAuthorityKeys: seed.TrustedKeys})
			if newErr != nil {
				t.Fatalf("NewAuthority = %v, want nil", newErr)
			}
			got.Certificate, err = authority.IssueInstallationCertificate(got.Certificate.Body, foreign)
		}
		if err != nil {
			t.Fatalf("sign foreign mutation = %v, want nil", err)
		}
		r := seed
		r.Document = got
		proof, err := VerifyBuildTransfer(r)
		if err != nil {
			if proof != (VerifiedBuildTransfer{}) || (!errors.Is(err, core.ErrPermitBinding) && !errors.Is(err, core.ErrPermitContract) && !errors.Is(err, core.ErrPermitAuthentication)) {
				t.Fatalf("verify refusal = %v/%v, want zero/typed refusal", proof, err)
			}
			if got == seed.Document {
				t.Fatalf("canonical signed seed refused = %v, want nil", err)
			}
			return
		}
		if got != seed.Document || mutation%3 != 0 {
			t.Fatalf("verified foreign document = %v, want exact signed seed", got)
		}
		if err := proof.Validate(); err != nil {
			t.Fatalf("verified proof = %v, want nil", err)
		}
	})
}

func shiftTransferInstant(t testing.TB, instant temporal.Instant, delta int64) temporal.Instant {
	t.Helper()
	nanos, err := instant.Nanoseconds()
	if err != nil {
		t.Fatalf("Instant.Nanoseconds = %v, want nil", err)
	}
	got := temporal.InstantFromNanoseconds(nanos + delta)
	if err := got.Validate(); err != nil {
		t.Fatalf("shifted instant = %v, want nil", err)
	}
	return got
}
