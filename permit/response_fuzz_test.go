package permit

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlplanetest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
)

// These fixtures exercise the shared wire contract and real signing mechanics.
// They do not select API policy or claim server transaction/replay coverage.
func signedResponseFixtures(t testing.TB) (RegistrationResponse, CheckInResponse, VerifyRequest) {
	t.Helper()
	r, signer := permitFixture(t)
	installation, err := controlplanetest.IssueInstallation(controlplanetest.InstallationRequest{Offering: r.Subject.Offering, AuthoritySeed: [32]byte{17}, DeviceSeed: [32]byte{23}})
	if err != nil {
		t.Fatalf("IssueInstallation = %v, want nil", err)
	}
	defer clear(installation.AuthorityPrivate)
	defer clear(installation.DevicePrivate)
	authority, err := controlplane.NewAuthority(controlplane.AuthorityConfiguration{TrustedAuthorityKeys: r.TrustedKeys})
	if err != nil {
		t.Fatalf("NewAuthority = %v, want nil", err)
	}
	watermark, err := controlplane.NewInitialUsageWatermark(r.Subject)
	if err != nil {
		t.Fatalf("NewInitialUsageWatermark = %v, want nil", err)
	}
	// The certificate's issue time is a fixed fixture fact, not wall time.
	terms := r.Document.Terms
	terms.NotBefore = installation.Certificate.Body.IssuedAt
	terms.ExpiresAt, err = terms.NotBefore.Add(terms.RetryAfter)
	if err != nil {
		t.Fatalf("Add fixture extent = %v, want nil", err)
	}
	terms.ContactAt = terms.ExpiresAt
	terms.Generation = watermark.Generation
	r.Document, err = Sign(terms, signer)
	if err != nil {
		t.Fatalf("Sign permission = %v, want nil", err)
	}
	r.EffectiveAt, r.MinimumGeneration = terms.NotBefore, terms.Generation
	leaseEnd, err := terms.ExpiresAt.Add(terms.RetryAfter)
	if err != nil {
		t.Fatalf("Add lease extent = %v, want nil", err)
	}
	continuityEnd, err := leaseEnd.Add(terms.RetryAfter)
	if err != nil {
		t.Fatalf("Add continuity extent = %v, want nil", err)
	}
	decision, err := lease.NewGrantDecision(lease.GrantDecisionRequest{Header: lease.Header{Revision: lease.RevisionV1, Subject: r.Subject, Generation: terms.Generation, IssuedAt: terms.NotBefore}, Grant: lease.Grant{NotBefore: terms.NotBefore, ContactAfter: terms.ExpiresAt, NotAfter: leaseEnd, GoodUntil: continuityEnd}})
	if err != nil {
		t.Fatalf("NewGrantDecision = %v, want nil", err)
	}
	envelope, err := attest.Sign(attest.SignRequest[lease.Domain]{Body: decision, Signer: signer})
	if err != nil {
		t.Fatalf("Sign lease = %v, want nil", err)
	}
	header := controlplane.ResponseHeader{ProviderTime: terms.NotBefore, RequestNonce: r.RequestNonce, Account: installation.Certificate.Body.Account, Installation: r.Subject.DeviceID, Revision: controlwire.Revision2026V1, Family: controlwire.RouteFamilyRegistrations, Status: controlplane.ProductStatusActive, Offering: r.Subject.Offering, Policy: controlwire.PolicyCursor{Revision: controlwire.PolicyRevisionID{7}, Activation: 1}}
	document := lease.Document{Decision: decision, Attestation: envelope}
	registration, err := authority.IssueRegistration(controlplane.RegistrationPayload{Certificate: &installation.Certificate, Header: header, Watermark: watermark, Lease: document, Entitlement: r.Subject.EntitlementID}, signer)
	if err != nil {
		t.Fatalf("IssueRegistration = %v, want nil", err)
	}
	header.Family = controlwire.RouteFamilyCheckIns
	checkIn, err := authority.IssueCheckInResponse(controlplane.CheckInResponsePayload{Header: header, Watermark: watermark, Lease: document, Disposition: controlplane.UsageDispositionReplay}, signer)
	if err != nil {
		t.Fatalf("IssueCheckInResponse = %v, want nil", err)
	}
	return RegistrationResponse{Registration: registration, Permission: r.Document}, CheckInResponse{CheckIn: checkIn, Permission: r.Document}, r
}

func FuzzRegistrationResponseSemanticClosure(f *testing.F) {
	seed, _, verification := signedResponseFixtures(f)
	canonical, err := seed.MarshalJSON()
	if err != nil {
		f.Fatalf("MarshalJSON seed = %v, want nil", err)
	}
	f.Add(canonical, uint8(0))
	f.Add([]byte{}, uint8(0))
	f.Add([]byte("null"), uint8(0))
	f.Add(canonical, uint8(1))
	f.Fuzz(func(t *testing.T, data []byte, mutation uint8) {
		got := seed
		if err := got.UnmarshalJSON(data); err != nil {
			if (!errors.Is(err, core.ErrPermitContract) && !errors.Is(err, core.ErrPermitBinding)) || got != seed {
				t.Fatalf("UnmarshalJSON refusal = %v/%v, want preserved/typed rejection", got, err)
			}
			return
		}
		// Reach structurally valid but unauthenticated input on every odd selector.
		if mutation%2 == 1 {
			before := got.Permission.Terms
			var err error
			if before.Actions == (Actions{}) {
				got.Permission.Terms.Actions, err = NewActions(permitAction(t, "operation-b"))
			} else {
				got.Permission.Terms.Actions = Actions{}
			}
			if err != nil {
				t.Fatalf("NewActions = %v, want nil", err)
			}
			if got.Permission.Terms == before {
				t.Fatal("skill mutation changed = false, want true")
			}
		}
		encoded, err := got.MarshalJSON()
		if err != nil || len(encoded) > ResponseMaximumBytes {
			t.Fatalf("MarshalJSON accepted = %d/%v, want bounded/nil", len(encoded), err)
		}
		var roundTrip RegistrationResponse
		if err := roundTrip.UnmarshalJSON(encoded); err != nil {
			t.Fatalf("canonical decode = %v, want nil", err)
		}
		// Registration contains a certificate pointer; compare its value explicitly.
		gotCertificate, roundCertificate := got.Registration.Payload.Certificate, roundTrip.Registration.Payload.Certificate
		if gotCertificate == nil || roundCertificate == nil || *gotCertificate != *roundCertificate {
			t.Fatalf("certificate = %v, want %v", roundCertificate, gotCertificate)
		}
		roundTrip.Registration.Payload.Certificate = gotCertificate
		if roundTrip != got {
			t.Fatalf("round trip = %+v, want %+v", roundTrip, got)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("canonical second encoding equal/error = %t/%v, want true/nil", bytes.Equal(second, encoded), err)
		}
		client, err := controlplane.NewClient(controlplane.ClientConfiguration{TrustedAuthorityKeys: verification.TrustedKeys})
		if err != nil {
			t.Fatalf("NewClient = %v, want nil", err)
		}
		h := seed.Registration.Payload.Header
		proof, err := client.VerifyRegistration(controlplane.RegistrationVerification{Document: got.Registration, Build: verification.Build, DeviceKey: seed.Registration.Payload.Certificate.Body.DeviceKey, Expected: controlplane.ResponseExpectation{RequestNonce: h.RequestNonce, Account: h.Account, Installation: h.Installation, Revision: h.Revision, Family: h.Family, Offering: h.Offering}})
		gotRegistration := got.Registration
		gotRegistration.Payload.Certificate = seed.Registration.Payload.Certificate
		wantAuthentic := gotRegistration == seed.Registration && *gotCertificate == *seed.Registration.Payload.Certificate
		if err != nil {
			if wantAuthentic || (!errors.Is(err, core.ErrControlPlaneRegistration) && !errors.Is(err, core.ErrControlPlaneResponseHeader)) || proof != (controlplane.VerifiedRegistration{}) {
				t.Fatalf("VerifyRegistration = %v/%v, want foreign bytes refused with zero proof", proof, err)
			}
		} else if !wantAuthentic {
			t.Fatal("authenticated registration equals signed seed = false, want true")
		}
		permissionResponseOracle(t, got.Permission, verification)
	})
}

func FuzzCheckInResponseSemanticClosure(f *testing.F) {
	_, seed, verification := signedResponseFixtures(f)
	canonical, err := seed.MarshalJSON()
	if err != nil {
		f.Fatalf("MarshalJSON seed = %v, want nil", err)
	}
	f.Add(canonical, uint8(0))
	f.Add([]byte{}, uint8(0))
	f.Add([]byte("null"), uint8(0))
	f.Add(canonical, uint8(1))
	f.Fuzz(func(t *testing.T, data []byte, mutation uint8) {
		got := seed
		if err := got.UnmarshalJSON(data); err != nil {
			if (!errors.Is(err, core.ErrPermitContract) && !errors.Is(err, core.ErrPermitBinding)) || got != seed {
				t.Fatalf("UnmarshalJSON refusal = %v/%v, want preserved/typed rejection", got, err)
			}
			return
		}
		if mutation%2 == 1 {
			before := got.Permission.Terms
			var err error
			if before.Actions == (Actions{}) {
				got.Permission.Terms.Actions, err = NewActions(permitAction(t, "operation-b"))
			} else {
				got.Permission.Terms.Actions = Actions{}
			}
			if err != nil {
				t.Fatalf("NewActions = %v, want nil", err)
			}
			if got.Permission.Terms == before {
				t.Fatal("skill mutation changed = false, want true")
			}
		}
		encoded, err := got.MarshalJSON()
		if err != nil || len(encoded) > ResponseMaximumBytes {
			t.Fatalf("MarshalJSON accepted = %d/%v, want bounded/nil", len(encoded), err)
		}
		var roundTrip CheckInResponse
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
			t.Fatalf("round trip = %+v/%v, want %+v/nil", roundTrip, err, got)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("canonical second encoding equal/error = %t/%v, want true/nil", bytes.Equal(second, encoded), err)
		}
		// Authenticate the entire mechanical response independently of its permission.
		proof, err := attest.Verify(attest.VerifyRequest[controlplane.SigningDomain]{Body: got.CheckIn.Payload, Envelope: got.CheckIn.Attestation, TrustedKeys: verification.TrustedKeys})
		if err != nil {
			if got.CheckIn == seed.CheckIn || !errors.Is(err, core.ErrAttestVerification) || proof != (attest.Verified[controlplane.SigningDomain]{}) {
				t.Fatalf("response attestation = %v/%v, want foreign bytes refused with zero proof", proof, err)
			}
		} else if got.CheckIn != seed.CheckIn {
			t.Fatal("authenticated check-in equals signed seed = false, want true")
		}
		permissionResponseOracle(t, got.Permission, verification)
	})
}

func permissionResponseOracle(t *testing.T, got Document, request VerifyRequest) {
	t.Helper()
	want := request.Document
	request.Document = got
	proof, err := Verify(request)
	if got != want {
		if (!errors.Is(err, core.ErrPermitAuthentication) && !errors.Is(err, core.ErrPermitBinding) && !errors.Is(err, core.ErrPermitContract)) || proof != (Verified{}) {
			t.Fatalf("permission verification = %v/%v, want changed signed agreement refused with zero proof", proof, err)
		}
	} else if err != nil || proof.Allows(permitAction(t, "operation-a"), request.EffectiveAt) != nil {
		t.Fatalf("permission verification = %v, want signed seed executable", err)
	}
}
