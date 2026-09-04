package controlplane_test

import (
	"crypto/ed25519"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
	"github.com/deliri/primitive/v2026/receipt"
)

const (
	// The fixed facts a check-in fixture is built from. They are spelled here so
	// a failing case names one changed fact rather than a differently built
	// document.
	checkInEntitlementHex   = "20202020202020202020202020202020"
	checkInAccountHex       = "1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e"
	checkInNonceHex         = "2121212121212121212121212121212121212121212121212121212121212121"
	checkInPolicyRevision   = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	checkInRevisionText     = "2026.1"
	checkInIssuedAt         = int64(100)
	checkInAuthoritySeed    = byte(1)
	checkInDeviceSeed       = byte(7)
	checkInOtherDeviceSeed  = byte(9)
	checkInPolicyActivation = uint64(1)
)

// issuedCheckIn is one genuinely signed check-in and the keys that authenticate
// it. Both signatures are real Ed25519 signatures over the exact canonical
// bytes, because verification is the behavior under test and a stand-in would
// prove nothing about it.
type issuedCheckIn struct {
	device      ed25519.PrivateKey
	authority   ed25519.PrivateKey
	subject     lease.Subject
	certificate controlplane.InstallationCertificateDocument
	request     controlplane.CheckInRequest
	trusted     attest.TrustedKeys
}

// issueTestCheckIn builds one complete check-in for the named offering.
//
// The offering is a parameter rather than a constant, which is the whole point
// of the shape: the same types, the same constructors, and the same verification
// path serve every product, and nothing below has an arm per product.
func issueTestCheckIn(
	t testing.TB,
	offering core.Offering,
	window controlplane.UsageWindow,
) issuedCheckIn {
	t.Helper()

	authorityPublic, authority := testSigningKey(t, checkInAuthoritySeed)
	devicePublic, device := testSigningKey(t, checkInDeviceSeed)
	trusted, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{
		Keys: []core.Ed25519PublicKey{authorityPublic},
	})
	if err != nil {
		t.Fatalf("NewTrustedKeys() error = %v, want nil", err)
	}
	client := testControlplaneClient(t, trusted)
	server := testControlplaneServer(t, trusted)

	subject := testSubject(t, offering, devicePublic)
	build := testBuildForOffering(t, offering)
	revision := testRevision(t)
	certificate := issueTestCertificate(
		t, server, subject, build, revision, devicePublic, authority,
	)

	watermark, err := controlplane.NewInitialUsageWatermark(subject)
	if err != nil {
		t.Fatalf("NewInitialUsageWatermark() error = %v, want nil", err)
	}
	payload := controlplane.CheckInPayload{
		Window:            window,
		PreviousWatermark: watermark,
		LeaseGeneration:   watermark.Generation,
		Build:             build,
		Revision:          revision,
		RequestNonce:      testCheckInNonce(t),
		Installation:      subject.DeviceID,
		AppliedPolicy:     testPolicyCursor(t),
	}
	request, err := client.IssueCheckIn(payload, device, certificate)
	if err != nil {
		t.Fatalf("Client.IssueCheckIn() for %v error = %v, want nil", offering, err)
	}
	return issuedCheckIn{
		request: request, certificate: certificate, trusted: trusted,
		device: device, authority: authority, subject: subject,
	}
}

func (i issuedCheckIn) client(t testing.TB) controlplane.Client {
	t.Helper()
	return testControlplaneClient(t, i.trusted)
}

func (i issuedCheckIn) server(t testing.TB) controlplane.Authority {
	t.Helper()
	return testControlplaneServer(t, i.trusted)
}

func testSubject(t testing.TB, offering core.Offering, deviceKey core.Ed25519PublicKey) lease.Subject {
	t.Helper()

	entitlement, err := lease.ParseEntitlementID(checkInEntitlementHex)
	if err != nil {
		t.Fatalf("ParseEntitlementID() error = %v, want nil", err)
	}
	device, err := lease.DeviceIDForPublicKey(deviceKey)
	if err != nil {
		t.Fatalf("DeviceIDForPublicKey() error = %v, want nil", err)
	}
	return lease.Subject{Offering: offering, EntitlementID: entitlement, DeviceID: device}
}

// testBuildForOffering takes the golden build and changes only its offering, so
// a check-in for another product differs from the golden in exactly that field.
func testBuildForOffering(t testing.TB, offering core.Offering) core.BuildIdentity {
	t.Helper()

	original := testBuildIdentity(t, 1, 0, 0)
	build, err := core.NewBuildIdentity(core.BuildIdentityRequest{
		Offering: offering, Version: core.NewReleaseVersion(1, 0, 0),
		Commit: original.Commit(), Platform: original.Platform(),
	})
	if err != nil {
		t.Fatalf("NewBuildIdentity(%v) error = %v, want nil", offering, err)
	}
	return build
}

func issueTestCertificate(
	t testing.TB,
	server controlplane.Authority,
	subject lease.Subject,
	build core.BuildIdentity,
	revision controlwire.Revision,
	deviceKey core.Ed25519PublicKey,
	authority ed25519.PrivateKey,
) controlplane.InstallationCertificateDocument {
	t.Helper()

	account, err := receipt.ParsePrincipalIdentity(checkInAccountHex)
	if err != nil {
		t.Fatalf("ParsePrincipalIdentity() error = %v, want nil", err)
	}
	body := controlplane.InstallationCertificateBody{
		IssuedAt: testInstant(t, checkInIssuedAt), Build: build, Revision: revision,
		Subject: subject, DeviceKey: deviceKey, Account: account,
	}
	certificate, err := server.IssueInstallationCertificate(body, authority)
	if err != nil {
		t.Fatalf("Authority.IssueInstallationCertificate() error = %v, want nil", err)
	}
	return certificate
}

func testRevision(t testing.TB) controlwire.Revision {
	t.Helper()

	revision, err := controlwire.ParseRevision(checkInRevisionText)
	if err != nil {
		t.Fatalf("ParseRevision(%s) error = %v, want nil", checkInRevisionText, err)
	}
	return revision
}

func testCheckInNonce(t testing.TB) controlwire.RequestNonce {
	t.Helper()

	nonce, err := controlwire.ParseRequestNonce(checkInNonceHex)
	if err != nil {
		t.Fatalf("ParseRequestNonce() error = %v, want nil", err)
	}
	return nonce
}

func testPolicyCursor(t testing.TB) controlwire.PolicyCursor {
	t.Helper()

	revision, err := controlwire.ParsePolicyRevisionID(checkInPolicyRevision)
	if err != nil {
		t.Fatalf("ParsePolicyRevisionID() error = %v, want nil", err)
	}
	activation, err := controlwire.NewPolicyActivation(checkInPolicyActivation)
	if err != nil {
		t.Fatalf("NewPolicyActivation() error = %v, want nil", err)
	}
	return controlwire.PolicyCursor{Revision: revision, Activation: activation}
}

// testCheckInWindow is the window a fixture reports unless a case needs another:
// two units of one class, both ending the same way.
func testCheckInWindow() controlplane.UsageWindow {
	return testWindow(unitsOf(1, 2), outcomesOf(1, 2))
}

// TestCheckInCarriesEveryOfferingThroughOneShape is the obliviousness proof.
//
// One payload type, one request type, one issuing function, and one verifying
// function serve every product, and the only thing that differs between them is
// a field. If a product ever needed its own type, function, or arm, this test is
// where that would show up as a compile error rather than as a design nobody
// re-examined.
func TestCheckInOfferingLayerTriadCarriesRepresentativeOpaqueOfferingsThroughOneShape(t *testing.T) {
	t.Parallel()

	offerings := []core.Offering{
		controlplaneOffering(t, 1),
		controlplaneOffering(t, 127),
		controlplaneOffering(t, 255),
	}
	if len(offerings) < 3 {
		t.Fatalf("admitted offerings = %d, want at least three distinct opaque offerings", len(offerings))
	}
	for _, offering := range offerings {
		t.Run(offering.String(), func(t *testing.T) {
			t.Parallel()

			issued := issueTestCheckIn(t, offering, testCheckInWindow())
			server := issued.server(t)
			route, routeErr := issued.request.ControlRoute()
			if routeErr != nil || route.Offering() != offering ||
				route.Family() != controlwire.RouteFamilyCheckIns ||
				issued.request.ControlNonce() != issued.request.Payload.RequestNonce {
				t.Fatalf("check-in control projection(%v) = (%v, %v, %v), want exact route and signed nonce",
					offering, route, issued.request.ControlNonce(), routeErr)
			}
			verified, err := server.VerifyCheckIn(controlplane.CheckInVerification{
				Request: issued.request,
			})
			if err != nil {
				t.Fatalf("VerifyCheckIn() for %v error = %v, want nil", offering, err)
			}
			request, err := verified.Request()
			if err != nil {
				t.Fatalf("Request() error = %v, want nil", err)
			}
			if got := request.Payload.Build.Offering(); got != offering {
				t.Fatalf("verified offering = %v, want %v", got, offering)
			}
			if got, want := request.Payload.Installation, issued.subject.DeviceID; got != want {
				t.Fatalf("verified installation = %v, want the subject device %v", got, want)
			}
			if got, want := request.Payload.PreviousWatermark.Subject, issued.subject; got != want {
				t.Fatalf("verified watermark subject = %v, want %v", got, want)
			}
			encoded, err := request.MarshalJSON()
			if err != nil {
				t.Fatalf("MarshalJSON() error = %v, want nil", err)
			}
			var decoded controlplane.CheckInRequest
			if err := decoded.UnmarshalJSON(encoded); err != nil {
				t.Fatalf("UnmarshalJSON() error = %v, want nil", err)
			}
			reencoded, err := decoded.MarshalJSON()
			if err != nil {
				t.Fatalf("re-MarshalJSON() error = %v, want nil", err)
			}
			if string(reencoded) != string(encoded) {
				t.Fatalf("re-encoded request = %s, want %s", reencoded, encoded)
			}
		})
	}
}

func TestVerifiedCheckInOwnsUsageSlicesAcrossInputAndAccessorMutation(t *testing.T) {
	t.Parallel()

	issued := issueTestCheckIn(t, controlplaneOffering(t, 61), testCheckInWindow())
	input := issued.request
	wantUnits := issued.request.Payload.Window.Units[0].Count
	wantOutcomes := issued.request.Payload.Window.Outcomes[0].Count
	verified, err := issued.server(t).VerifyCheckIn(controlplane.CheckInVerification{Request: input})
	if err != nil {
		t.Fatalf("VerifyCheckIn() error = %v, want nil", err)
	}
	input.Payload.Window.Units[0].Count++
	input.Payload.Window.Outcomes[0].Count++
	first, err := verified.Request()
	if err != nil {
		t.Fatalf("VerifiedCheckIn.Request(first) error = %v, want nil", err)
	}
	first.Payload.Window.Units[0].Count++
	first.Payload.Window.Outcomes[0].Count++
	second, err := verified.Request()
	if err != nil || second.Payload.Window.Units[0].Count != wantUnits ||
		second.Payload.Window.Outcomes[0].Count != wantOutcomes {
		t.Fatalf("VerifiedCheckIn.Request(after mutation) = (%v, %v), want original authenticated counts", second, err)
	}
}

// TestCheckInRefusesEveryTamperedFact walks the facts a signature is supposed to
// bind and proves each one is actually bound.
//
// A check-in is the document that moves money, so the failures that matter are
// the ones where a well-formed request is accepted on behalf of the wrong
// installation, the wrong entitlement, or the wrong window.
func TestCheckInRefusesEveryTamperedFact(t *testing.T) {
	t.Parallel()

	// The two mutations are separate fields rather than one that returns both,
	// so a case that wants the empty trust set can say so. A single return value
	// would have to treat the zero trust set as "no override", and the case
	// proving an empty trust set is refused would silently test nothing.
	cases := []struct {
		mutate  func(*testing.T, *controlplane.CheckInRequest, issuedCheckIn)
		trusted func(*testing.T) attest.TrustedKeys
		name    string
	}{
		{
			name: "a window swapped after signing no longer matches the signature",
			mutate: func(_ *testing.T, request *controlplane.CheckInRequest, _ issuedCheckIn) {
				request.Payload.Window = testWindow(unitsOf(1, 9), outcomesOf(1, 9))
			},
		},
		{
			name: "a nonce swapped after signing no longer matches the signature",
			mutate: func(t *testing.T, request *controlplane.CheckInRequest, _ issuedCheckIn) {
				request.Payload.RequestNonce = otherRequestNonce(t)
			},
		},
		{
			name: "a stripped attestation authenticates nothing",
			mutate: func(_ *testing.T, request *controlplane.CheckInRequest, _ issuedCheckIn) {
				request.Attestation = attest.Envelope[controlplane.SigningDomain]{}
			},
		},
		{
			name: "a credential this authority did not sign is refused",
			mutate: func(t *testing.T, request *controlplane.CheckInRequest, issued issuedCheckIn) {
				_, impostor := testSigningKey(t, checkInOtherDeviceSeed)
				forged, err := issued.server(t).IssueInstallationCertificate(
					issued.certificate.Body,
					impostor,
				)
				if err != nil {
					t.Fatalf("IssueInstallationCertificate() error = %v, want nil", err)
				}
				request.Certificate = forged
			},
		},
		{
			name: "an installation naming another device than its credential is refused",
			mutate: func(t *testing.T, request *controlplane.CheckInRequest, _ issuedCheckIn) {
				_, other := testDeviceKey(t, checkInOtherDeviceSeed)
				request.Payload.Installation = other
			},
		},
		{
			name: "a generation that disagrees with the watermark it continues is refused",
			mutate: func(t *testing.T, request *controlplane.CheckInRequest, _ issuedCheckIn) {
				generation, err := lease.NewGeneration(2)
				if err != nil {
					t.Fatalf("NewGeneration() error = %v, want nil", err)
				}
				request.Payload.LeaseGeneration = generation
			},
		},
		{
			name: "an empty trust set admits no authority at all",
			trusted: func(_ *testing.T) attest.TrustedKeys {
				return attest.TrustedKeys{}
			},
		},
		{
			name: "a trust set holding only some other key is refused",
			trusted: func(t *testing.T) attest.TrustedKeys {
				stranger, _ := testSigningKey(t, checkInOtherDeviceSeed)
				keys, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{
					Keys: []core.Ed25519PublicKey{stranger},
				})
				if err != nil {
					t.Fatalf("NewTrustedKeys() error = %v, want nil", err)
				}
				return keys
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			issued := issueTestCheckIn(t, controlplaneOffering(t, 3), testCheckInWindow())
			server := issued.server(t)
			if _, err := server.VerifyCheckIn(controlplane.CheckInVerification{
				Request: issued.request,
			}); err != nil {
				t.Fatalf("the untampered fixture must verify first: VerifyCheckIn() error = %v", err)
			}

			request := issued.request
			if testCase.mutate != nil {
				testCase.mutate(t, &request, issued)
			}
			trusted := issued.trusted
			if testCase.trusted != nil {
				trusted = testCase.trusted(t)
			}
			server, serverErr := controlplane.NewAuthority(controlplane.AuthorityConfiguration{
				TrustedAuthorityKeys: trusted,
			})
			verified, err := server.VerifyCheckIn(controlplane.CheckInVerification{
				Request: request,
			})
			err = errors.Join(serverErr, err)
			if !errors.Is(err, core.ErrControlPlaneCheckIn) {
				t.Fatalf("VerifyCheckIn() = (%v, %v), want errors.Is %v", verified, err, core.ErrControlPlaneCheckIn)
			}
			if validateErr := verified.Validate(); !errors.Is(validateErr, core.ErrControlPlaneCheckIn) {
				t.Fatalf("refused VerifiedCheckIn.Validate() error = %v, want errors.Is %v", validateErr, core.ErrControlPlaneCheckIn)
			}
		})
	}
}

// TestVerifiedCheckInCannotBeManufactured proves the sealed type is sealed.
//
// A zero VerifiedCheckIn is what a caller gets by declaring the type instead of
// verifying, and it must not be able to present itself as proof of anything.
func TestVerifiedCheckInCannotBeManufactured(t *testing.T) {
	t.Parallel()

	var unverified controlplane.VerifiedCheckIn
	if err := unverified.Validate(); !errors.Is(err, core.ErrControlPlaneCheckIn) {
		t.Fatalf("zero VerifiedCheckIn Validate() error = %v, want errors.Is %v", err, core.ErrControlPlaneCheckIn)
	}
	request, err := unverified.Request()
	if !errors.Is(err, core.ErrControlPlaneCheckIn) {
		t.Fatalf("Request() error = %v, want the check-in contract identity", err)
	}
	if validateErr := request.Validate(); !errors.Is(validateErr, core.ErrControlPlaneCheckIn) {
		t.Fatalf("refused Request().Validate() error = %v, want errors.Is %v", validateErr, core.ErrControlPlaneCheckIn)
	}
}
