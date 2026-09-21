package accesspermit

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
	"github.com/deliri/primitive/v2026/temporal"
)

func responseWindow() controlplane.UsageWindow {
	return controlplane.UsageWindow{Bounds: temporal.IntervalBounds{Start: temporal.InstantFromNanoseconds(90), End: temporal.InstantFromNanoseconds(100)}, Freshness: temporal.InstantFromNanoseconds(100)}
}

func responseExpectation(header controlplane.ResponseHeader) controlplane.ResponseExpectation {
	return controlplane.ResponseExpectation{Offering: header.Offering, Account: header.Account, Installation: header.Installation, RequestNonce: header.RequestNonce, Family: header.Family, Revision: header.Revision}
}

// Drives real Primitive issuers; no hand-authored valid protocol JSON.
func responseFixture(t testing.TB) (RegistrationResponse, CheckInResponse, attest.TrustedKeys) {
	t.Helper()
	permit, keys, key := permitFixture(t)
	binding := permit.Terms.Binding
	authority, err := controlplane.NewAuthority(controlplane.AuthorityConfiguration{TrustedAuthorityKeys: keys})
	if err != nil {
		t.Fatalf("NewAuthority() error = %v, want nil", err)
	}
	at := temporal.InstantFromNanoseconds(100)
	watermark, err := controlplane.NewInitialUsageWatermark(binding.Subject)
	if err != nil {
		t.Fatalf("NewInitialUsageWatermark() error = %v, want nil", err)
	}
	header := controlplane.ResponseHeader{Offering: binding.Subject.Offering, Account: binding.Account, Installation: binding.Subject.DeviceID, RequestNonce: binding.RequestNonce, Revision: controlwire.Revision2026V1, Family: controlwire.RouteFamilyRegistrations, ProviderTime: at, Policy: controlwire.PolicyCursor{Revision: controlwire.PolicyRevisionID([16]byte{1}), Activation: 1}, Status: controlplane.ProductStatusActive}
	decision, err := lease.NewGrantDecision(lease.GrantDecisionRequest{Header: lease.Header{Subject: binding.Subject, Generation: watermark.Generation, IssuedAt: at, Revision: lease.RevisionV1}, Grant: lease.Grant{NotBefore: at, ContactAfter: temporal.InstantFromNanoseconds(200), NotAfter: temporal.InstantFromNanoseconds(400), GoodUntil: temporal.InstantFromNanoseconds(500)}})
	if err != nil {
		t.Fatalf("NewGrantDecision() error = %v, want nil", err)
	}
	envelope, err := attest.Sign(attest.SignRequest[lease.Domain]{Body: decision, Signer: key})
	if err != nil {
		t.Fatalf("Sign(lease) error = %v, want nil", err)
	}
	signedLease := lease.Document{Decision: decision, Attestation: envelope}
	commit, err := core.ParseBuildCommit("0123456789abcdef0123456789abcdef01234567")
	if err != nil {
		t.Fatalf("ParseBuildCommit() error = %v, want nil", err)
	}
	build, err := core.NewBuildIdentity(core.BuildIdentityRequest{Offering: binding.Subject.Offering, Commit: commit, Version: core.NewReleaseVersion(2026, 1, 1), Platform: core.Platform{OperatingSystem: core.OperatingSystemLinux, Architecture: core.CPUArchitectureAMD64}})
	if err != nil {
		t.Fatalf("NewBuildIdentity() error = %v, want nil", err)
	}
	public, err := core.NewEd25519PublicKey(key.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatalf("NewEd25519PublicKey() error = %v, want nil", err)
	}
	certificate, err := authority.IssueInstallationCertificate(controlplane.InstallationCertificateBody{Subject: binding.Subject, Account: binding.Account, Build: build, DeviceKey: public, IssuedAt: at, Revision: controlwire.Revision2026V1}, key)
	if err != nil {
		t.Fatalf("IssueInstallationCertificate() error = %v, want nil", err)
	}
	registration, err := authority.IssueRegistration(controlplane.RegistrationPayload{Header: header, Watermark: watermark, Lease: signedLease, Certificate: &certificate, Entitlement: binding.Subject.EntitlementID}, key)
	if err != nil {
		t.Fatalf("IssueRegistration() error = %v, want nil", err)
	}
	registrationPermit := permit
	header.Family = controlwire.RouteFamilyCheckIns
	watermark, err = controlplane.AdvanceUsageWatermark(watermark, responseWindow())
	if err != nil {
		t.Fatalf("AdvanceUsageWatermark() error = %v, want nil", err)
	}
	leaseHeader, err := decision.Header()
	if err != nil {
		t.Fatalf("Header() error = %v, want nil", err)
	}
	grant, err := decision.Grant()
	if err != nil {
		t.Fatalf("Grant() error = %v, want nil", err)
	}
	leaseHeader.Generation = watermark.Generation
	decision, err = lease.NewGrantDecision(lease.GrantDecisionRequest{Header: leaseHeader, Grant: grant})
	if err != nil {
		t.Fatalf("NewGrantDecision(check-in) error = %v, want nil", err)
	}
	envelope, err = attest.Sign(attest.SignRequest[lease.Domain]{Body: decision, Signer: key})
	if err != nil {
		t.Fatalf("Sign(check-in lease) error = %v, want nil", err)
	}
	signedLease = lease.Document{Decision: decision, Attestation: envelope}
	permit.Terms.Binding.Family = header.Family
	permit.Terms.Binding.Generation = watermark.Generation
	checkInPermit, err := Issue(permit.Terms, key)
	if err != nil {
		t.Fatalf("Issue(check-in permit) error = %v, want nil", err)
	}
	checkIn, err := authority.IssueCheckInResponse(controlplane.CheckInResponsePayload{Header: header, Watermark: watermark, Lease: signedLease, Disposition: controlplane.UsageDispositionAccepted}, key)
	if err != nil {
		t.Fatalf("IssueCheckInResponse() error = %v, want nil", err)
	}
	client, err := controlplane.NewClient(controlplane.ClientConfiguration{TrustedAuthorityKeys: keys})
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}
	if _, err := client.VerifyCheckInResponse(controlplane.CheckInResponseVerification{Document: checkIn, Expected: responseExpectation(header), PreviousWatermark: registration.Payload.Watermark, Window: responseWindow()}); err != nil {
		t.Fatalf("VerifyCheckInResponse(seed) error = %v, want nil", err)
	}
	return RegistrationResponse{Registration: registration, Permit: registrationPermit}, CheckInResponse{CheckIn: checkIn, Permit: checkInPermit}, keys
}

func TestResponsePermitBindingLayerTriad(t *testing.T) {
	t.Parallel()
	registration, checkIn, keys := responseFixture(t)
	for _, tc := range []struct {
		want     error
		mutation func(*Document)
		name     string
	}{
		{name: "exact registration and permit share authenticated identity"},
		{name: "foreign nonce cannot accompany authentic registration", mutation: func(d *Document) {
			d.Terms.Binding.RequestNonce, _ = controlwire.NewRequestNonce([core.SHA256DigestBytes]byte{42})
		}, want: core.ErrAccessPermitBinding},
		{name: "absent permit cannot inherit lease authority", mutation: func(d *Document) { *d = Document{} }, want: core.ErrAccessPermitContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r, c := registration, checkIn
			if tc.mutation != nil {
				tc.mutation(&r.Permit)
				tc.mutation(&c.Permit)
			}
			if err := r.Validate(); !errors.Is(err, tc.want) {
				t.Fatalf("registration Validate() = %v, want %v", err, tc.want)
			}
			if err := c.Validate(); !errors.Is(err, tc.want) {
				t.Fatalf("check-in Validate() = %v, want %v", err, tc.want)
			}
			if tc.want != nil {
				return
			}
			client, err := controlplane.NewClient(controlplane.ClientConfiguration{TrustedAuthorityKeys: keys})
			if err != nil {
				t.Fatalf("NewClient() error = %v, want nil", err)
			}
			header := r.Registration.Payload.Header
			verified, err := client.VerifyRegistration(controlplane.RegistrationVerification{Document: r.Registration, DeviceKey: r.Registration.Payload.Certificate.Body.DeviceKey, Build: r.Registration.Payload.Certificate.Body.Build, Expected: controlplane.ResponseExpectation{Offering: header.Offering, Account: header.Account, Installation: header.Installation, RequestNonce: header.RequestNonce, Family: header.Family, Revision: header.Revision}})
			if err != nil {
				t.Fatalf("VerifyRegistration() error = %v, want nil", err)
			}
			payload, err := verified.Payload()
			if err != nil {
				t.Fatalf("Payload() error = %v, want nil", err)
			}
			binding, err := ResponseBinding(payload.Header, payload.Watermark.Subject, payload.Watermark.Generation)
			if err != nil {
				t.Fatalf("ResponseBinding() error = %v, want nil", err)
			}
			proof, err := Verify(r.Permit, binding, keys)
			if err != nil {
				t.Fatalf("Verify(permit) error = %v, want nil", err)
			}
			// The independent permit expires while the genuine lease is current.
			if err := proof.Allows(temporal.InstantFromNanoseconds(300)); !errors.Is(err, core.ErrAccessPermitDenied) {
				t.Fatalf("Allows(exact permit expiry) = %v, want denial despite current lease", err)
			}
		})
	}
}

func FuzzRegistrationResponseSemanticBinding(f *testing.F) {
	seed, _, keys := responseFixture(f)
	client, err := controlplane.NewClient(controlplane.ClientConfiguration{TrustedAuthorityKeys: keys})
	if err != nil {
		f.Fatalf("NewClient() error = %v, want nil", err)
	}
	canonical, err := seed.MarshalJSON()
	if err != nil {
		f.Fatalf("MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		got := seed
		err := got.UnmarshalJSON(data)
		if err != nil {
			if !errors.Is(err, core.ErrAccessPermitContract) || got != seed {
				t.Fatalf("refusal = %v, want typed refusal and preserved receiver", err)
			}
			return
		}
		encoded, err := got.MarshalJSON()
		if err != nil || len(encoded) > RegistrationResponseMaximumBytes {
			t.Fatalf("projection = %d/%v, want bounded/nil", len(encoded), err)
		}
		var roundTrip RegistrationResponse
		if err := roundTrip.UnmarshalJSON(encoded); err != nil {
			t.Fatalf("round trip error = %v, want nil", err)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("second projection equality/error = %t/%v, want true/nil", bytes.Equal(second, encoded), err)
		}
		registrationProof, verifyErr := client.VerifyRegistration(controlplane.RegistrationVerification{Document: got.Registration, Expected: responseExpectation(seed.Registration.Payload.Header), Build: seed.Registration.Payload.Certificate.Body.Build, DeviceKey: seed.Registration.Payload.Certificate.Body.DeviceKey})
		gotRegistrationBytes, gotRegistrationErr := got.Registration.MarshalJSON()
		seedRegistrationBytes, seedRegistrationErr := seed.Registration.MarshalJSON()
		if gotRegistrationErr != nil || seedRegistrationErr != nil || bytes.Equal(gotRegistrationBytes, seedRegistrationBytes) != (verifyErr == nil) {
			t.Fatalf("registration seed membership/error = %t/%v, want equivalent authentication", bytes.Equal(gotRegistrationBytes, seedRegistrationBytes), verifyErr)
		}
		if verifyErr == nil {
			gotBytes, encodeErr := got.Registration.MarshalJSON()
			seedBytes, seedErr := seed.Registration.MarshalJSON()
			if encodeErr != nil || seedErr != nil || !bytes.Equal(gotBytes, seedBytes) || registrationProof.Validate() != nil {
				t.Fatal("authenticated registration = different bytes, want exact signed seed")
			}
		} else if !errors.Is(verifyErr, core.ErrControlPlaneContract) || registrationProof != (controlplane.VerifiedRegistration{}) {
			t.Fatalf("registration refusal = %v, want typed error and zero proof", verifyErr)
		}
		proof, err := Verify(got.Permit, seed.Permit.Terms.Binding, keys)
		if (got.Permit == seed.Permit) != (err == nil) {
			t.Fatalf("permit seed membership/error = %t/%v, want equivalent authentication", got.Permit == seed.Permit, err)
		}
		if err == nil {
			if got.Permit != seed.Permit || proof.Validate() != nil {
				t.Fatalf("verified permit = %+v, want exact signed seed", got.Permit)
			}
		} else if proof != (Verified{}) || !errors.Is(err, core.ErrAccessPermitContract) {
			t.Fatalf("refused proof = %+v, want zero", proof)
		}
	})
}

func FuzzCheckInResponseSemanticBinding(f *testing.F) {
	registration, seed, keys := responseFixture(f)
	client, err := controlplane.NewClient(controlplane.ClientConfiguration{TrustedAuthorityKeys: keys})
	if err != nil {
		f.Fatalf("NewClient() error = %v, want nil", err)
	}
	canonical, err := seed.MarshalJSON()
	if err != nil {
		f.Fatalf("MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		got := seed
		err := got.UnmarshalJSON(data)
		if err != nil {
			if !errors.Is(err, core.ErrAccessPermitContract) || got != seed {
				t.Fatalf("refusal = %v, want typed refusal and preserved receiver", err)
			}
			return
		}
		encoded, err := got.MarshalJSON()
		if err != nil || len(encoded) > CheckInResponseMaximumBytes {
			t.Fatalf("projection = %d/%v, want bounded/nil", len(encoded), err)
		}
		var roundTrip CheckInResponse
		if err := roundTrip.UnmarshalJSON(encoded); err != nil {
			t.Fatalf("round trip error = %v, want nil", err)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("second projection equality/error = %t/%v, want true/nil", bytes.Equal(second, encoded), err)
		}
		responseProof, verifyErr := client.VerifyCheckInResponse(controlplane.CheckInResponseVerification{Document: got.CheckIn, Expected: responseExpectation(seed.CheckIn.Payload.Header), PreviousWatermark: registration.Registration.Payload.Watermark, Window: responseWindow()})
		if (got.CheckIn == seed.CheckIn) != (verifyErr == nil) {
			t.Fatalf("check-in seed membership/error = %t/%v, want equivalent authentication", got.CheckIn == seed.CheckIn, verifyErr)
		}
		if verifyErr == nil {
			if got.CheckIn != seed.CheckIn || responseProof.Validate() != nil {
				t.Fatal("authenticated check-in = different facts, want exact signed seed")
			}
		} else if !errors.Is(verifyErr, core.ErrControlPlaneContract) || responseProof != (controlplane.VerifiedCheckInResponse{}) {
			t.Fatalf("check-in refusal = %v, want typed error and zero proof", verifyErr)
		}
		proof, err := Verify(got.Permit, seed.Permit.Terms.Binding, keys)
		if (got.Permit == seed.Permit) != (err == nil) {
			t.Fatalf("permit seed membership/error = %t/%v, want equivalent authentication", got.Permit == seed.Permit, err)
		}
		if err == nil {
			if got.Permit != seed.Permit || proof.Validate() != nil {
				t.Fatalf("verified permit = %+v, want exact signed seed", got.Permit)
			}
		} else if proof != (Verified{}) || !errors.Is(err, core.ErrAccessPermitContract) {
			t.Fatalf("refused proof = %+v, want zero", proof)
		}
	})
}
