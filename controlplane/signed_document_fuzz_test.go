package controlplane_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
	"github.com/deliri/primitive/v2026/receipt"
	"github.com/deliri/primitive/v2026/temporal"
)

type verificationLayerProbe struct {
	positive func() error
	negative func() error
	neutral  func() error
	name     string
}

func TestControlplaneSignedVerificationLayerTriad(t *testing.T) {
	t.Parallel()

	certificate := issueTestCheckIn(t, controlplaneOffering(t, 2), testCheckInWindow())
	registration := issueTestRegistration(t)
	checkIn := issueTestCheckIn(t, controlplaneOffering(t, 1), testCheckInWindow())
	response := issueTestCheckInResponse(t)
	probes := []verificationLayerProbe{
		certificateVerificationProbe(certificate, certificate.client(t)),
		registrationVerificationProbe(registration, registration.client(t)),
		checkInVerificationProbe(checkIn, checkIn.server(t)),
		checkInResponseVerificationProbe(response, response.client(t)),
	}
	for _, probe := range probes {
		t.Run(probe.name, func(t *testing.T) {
			t.Parallel()

			if err := probe.positive(); err != nil {
				t.Errorf("positive verification error = %v, want nil", err)
			}
			if err := probe.negative(); !errors.Is(err, core.ErrControlPlaneContract) {
				t.Errorf("negative verification error = %v, want %v", err, core.ErrControlPlaneContract)
			}
			if err := probe.neutral(); !errors.Is(err, core.ErrControlPlaneContract) {
				t.Errorf("neutral verification error = %v, want %v", err, core.ErrControlPlaneContract)
			}
		})
	}
}

func certificateVerificationProbe(
	issued issuedCheckIn,
	client controlplane.Client,
) verificationLayerProbe {
	return verificationLayerProbe{
		name: "installation certificate authority and device trust",
		positive: func() error {
			proof, err := client.VerifyInstallationCertificate(issued.certificate)
			if err != nil {
				return err
			}
			return proof.Validate()
		},
		negative: func() error {
			_, err := client.VerifyInstallationCertificate(corruptCertificateDigest(issued.certificate))
			return err
		},
		neutral: func() error {
			_, err := (controlplane.Client{}).VerifyInstallationCertificate(
				controlplane.InstallationCertificateDocument{},
			)
			return err
		},
	}
}

func registrationVerificationProbe(
	issued issuedRegistration,
	client controlplane.Client,
) verificationLayerProbe {
	return verificationLayerProbe{
		name: "registration response and nested authorities",
		positive: func() error {
			proof, err := client.VerifyRegistration(issued.verification())
			if err != nil {
				return err
			}
			return proof.Validate()
		},
		negative: func() error {
			request := issued.verification()
			request.Document.Attestation.BodySHA256 = corruptDigest()
			_, err := client.VerifyRegistration(request)
			return err
		},
		neutral: func() error {
			_, err := (controlplane.Client{}).VerifyRegistration(controlplane.RegistrationVerification{})
			return err
		},
	}
}

func checkInVerificationProbe(
	issued issuedCheckIn,
	server controlplane.Server,
) verificationLayerProbe {
	return verificationLayerProbe{
		name: "check-in certificate then device signature",
		positive: func() error {
			proof, err := server.VerifyCheckIn(controlplane.CheckInVerification{
				Request: issued.request,
			})
			if err != nil {
				return err
			}
			return proof.Validate()
		},
		negative: func() error {
			request := issued.request
			request.Attestation.BodySHA256 = corruptDigest()
			_, err := server.VerifyCheckIn(controlplane.CheckInVerification{
				Request: request,
			})
			return err
		},
		neutral: func() error {
			_, err := (controlplane.Server{}).VerifyCheckIn(controlplane.CheckInVerification{})
			return err
		},
	}
}

func checkInResponseVerificationProbe(
	issued issuedCheckInResponse,
	client controlplane.Client,
) verificationLayerProbe {
	return verificationLayerProbe{
		name: "check-in response and nested lease",
		positive: func() error {
			proof, err := client.VerifyCheckInResponse(issued.verification())
			if err != nil {
				return err
			}
			return proof.Validate()
		},
		negative: func() error {
			request := issued.verification()
			request.Document.Attestation.BodySHA256 = corruptDigest()
			_, err := client.VerifyCheckInResponse(request)
			return err
		},
		neutral: func() error {
			_, err := (controlplane.Client{}).VerifyCheckInResponse(controlplane.CheckInResponseVerification{})
			return err
		},
	}
}

func FuzzInstallationCertificateDocumentDecodeAndVerify(f *testing.F) {
	issued := issueTestCheckIn(f, controlplaneOffering(f, 2), testCheckInWindow())
	client := issued.client(f)
	canonical := mustCertificateJSON(f, issued.certificate)
	f.Add(canonical)
	f.Add(mustCertificateJSON(f, corruptCertificateDigest(issued.certificate)))
	bodyMutation := issued.certificate
	bodyMutation.Body.IssuedAt = testInstant(f, checkInIssuedAt+1)
	f.Add(mustCertificateJSON(f, bodyMutation))
	signerMutation := issued.certificate
	signerMutation.Attestation.Signer, _ = testSigningKey(f, checkInOtherDeviceSeed)
	f.Add(mustCertificateJSON(f, signerMutation))
	lengthMutation := issued.certificate
	lengthMutation.Attestation.BodyLength = incrementBodyLength(f, lengthMutation.Attestation.BodyLength)
	f.Add(mustCertificateJSON(f, lengthMutation))
	signatureMutation := issued.certificate
	signatureMutation.Attestation.Signature = issued.request.Attestation.Signature
	f.Add(mustCertificateJSON(f, signatureMutation))
	f.Add(mutateSigningDomainSeed(f, signingDomainSeedMutation{
		Document: canonical, From: controlplane.SigningDomainInstallationCertificateV1,
		To: controlplane.SigningDomainRegistrationV1,
	}))
	addHostileControlplaneJSONSeeds(f)

	f.Fuzz(func(t *testing.T, data []byte) {
		before := issued.certificate
		candidate := before
		err := candidate.UnmarshalJSON(data)
		if err != nil {
			requireControlplaneDecodeRefusal(t, err)
			requireCertificateProjection(t, candidate, canonical)
			return
		}
		reencoded := mustCertificateJSON(t, candidate)
		requireCertificateStableRoundTrip(t, candidate, reencoded)
		proof, verifyErr := client.VerifyInstallationCertificate(candidate)
		if bytes.Equal(reencoded, canonical) {
			if verifyErr != nil {
				t.Fatalf("authentic certificate verification error = %v, want nil", verifyErr)
			}
			if err := proof.Validate(); err != nil {
				t.Fatalf("authentic certificate proof Validate() error = %v, want nil", err)
			}
			return
		}
		requireControlplaneVerificationRefusal(t, verifyErr)
		if proof != (controlplane.VerifiedInstallationCertificate{}) {
			t.Fatalf("refused certificate proof = %v, want zero", proof)
		}
	})
}

func FuzzInstallationCertificateBodyDecodeAndVerify(f *testing.F) {
	issued := issueTestCheckIn(f, controlplaneOffering(f, 2), testCheckInWindow())
	client := issued.client(f)
	canonical := mustCertificateBodyJSON(f, issued.certificate.Body)
	f.Add(canonical)
	foreign := issued.certificate.Body
	foreign.IssuedAt = testInstant(f, checkInIssuedAt+1)
	f.Add(mustCertificateBodyJSON(f, foreign))
	foreign = issued.certificate.Body
	foreign.Account = foreignAccount(f)
	f.Add(mustCertificateBodyJSON(f, foreign))
	foreign = issued.certificate.Body
	foreign.Build = alternateBuild(f, foreign.Build)
	f.Add(mustCertificateBodyJSON(f, foreign))
	addHostileControlplaneJSONSeeds(f)

	f.Fuzz(func(t *testing.T, data []byte) {
		before := issued.certificate.Body
		candidate := before
		err := candidate.UnmarshalJSON(data)
		if err != nil {
			requireControlplaneDecodeRefusal(t, err)
			requireCertificateBodyProjection(t, candidate, canonical)
			return
		}
		reencoded := mustCertificateBodyJSON(t, candidate)
		requireCertificateBodyStableRoundTrip(t, candidate, reencoded)
		document := issued.certificate
		document.Body = candidate
		proof, verifyErr := client.VerifyInstallationCertificate(document)
		if bytes.Equal(reencoded, canonical) {
			if verifyErr != nil {
				t.Fatalf("authentic certificate body verification error = %v, want nil", verifyErr)
			}
			if err := proof.Validate(); err != nil {
				t.Fatalf("authentic certificate body proof Validate() error = %v, want nil", err)
			}
			return
		}
		requireControlplaneVerificationRefusal(t, verifyErr)
		if proof != (controlplane.VerifiedInstallationCertificate{}) {
			t.Fatalf("refused certificate body proof = %v, want zero", proof)
		}
	})
}

func FuzzRegistrationDocumentDecodeAndVerify(f *testing.F) {
	issued := issueTestRegistration(f)
	client := issued.client(f)
	canonical := mustRegistrationJSON(f, issued.document)
	f.Add(canonical)
	corrupt := issued.document
	corrupt.Attestation.BodySHA256 = corruptDigest()
	f.Add(mustRegistrationJSON(f, corrupt))
	corrupt = issued.document
	corrupt.Attestation.Signer, _ = testSigningKey(f, checkInOtherDeviceSeed)
	f.Add(mustRegistrationJSON(f, corrupt))
	corrupt = issued.document
	corrupt.Attestation.BodyLength = incrementBodyLength(f, corrupt.Attestation.BodyLength)
	f.Add(mustRegistrationJSON(f, corrupt))
	corrupt = issued.document
	corrupt.Attestation.Signature = corrupt.Payload.Certificate.Attestation.Signature
	f.Add(mustRegistrationJSON(f, corrupt))
	corrupt = issued.document
	corrupt.Payload.Header.RequestNonce = otherRequestNonce(f)
	f.Add(mustRegistrationJSON(f, corrupt))
	corrupt = cloneRegistrationDocument(issued.document)
	corrupt.Payload.Header.Account = foreignAccount(f)
	corrupt.Payload.Certificate.Body.Account = corrupt.Payload.Header.Account
	f.Add(mustRegistrationJSON(f, corrupt))
	corrupt = cloneRegistrationDocument(issued.document)
	corrupt.Payload.Certificate.Body.Build = alternateBuild(f, corrupt.Payload.Certificate.Body.Build)
	f.Add(mustRegistrationJSON(f, corrupt))
	f.Add(mutateSigningDomainSeed(f, signingDomainSeedMutation{
		Document: canonical, From: controlplane.SigningDomainRegistrationV1,
		To: controlplane.SigningDomainCheckInResponseV1,
	}))
	addHostileControlplaneJSONSeeds(f)

	f.Fuzz(func(t *testing.T, data []byte) {
		candidate := issued.document
		err := candidate.UnmarshalJSON(data)
		if err != nil {
			requireControlplaneDecodeRefusal(t, err)
			requireRegistrationProjection(t, candidate, canonical)
			return
		}
		reencoded := mustRegistrationJSON(t, candidate)
		requireRegistrationStableRoundTrip(t, candidate, reencoded)
		request := issued.verification()
		request.Document = candidate
		proof, verifyErr := client.VerifyRegistration(request)
		if bytes.Equal(reencoded, canonical) {
			if verifyErr != nil {
				t.Fatalf("authentic registration verification error = %v, want nil", verifyErr)
			}
			if err := proof.Validate(); err != nil {
				t.Fatalf("authentic registration proof Validate() error = %v, want nil", err)
			}
			return
		}
		requireControlplaneVerificationRefusal(t, verifyErr)
		if proof != (controlplane.VerifiedRegistration{}) {
			t.Fatalf("refused registration proof = %v, want zero", proof)
		}
	})
}

func FuzzCheckInRequestDecodeAndVerify(f *testing.F) {
	issued := issueTestCheckIn(f, controlplaneOffering(f, 2), testCheckInWindow())
	server := issued.server(f)
	canonical := mustCheckInJSON(f, issued.request)
	f.Add(canonical)
	corrupt := issued.request
	corrupt.Attestation.BodySHA256 = corruptDigest()
	f.Add(mustCheckInJSON(f, corrupt))
	corrupt = issued.request
	corrupt.Attestation.Signer, _ = testSigningKey(f, checkInOtherDeviceSeed)
	f.Add(mustCheckInJSON(f, corrupt))
	corrupt = issued.request
	corrupt.Attestation.BodyLength = incrementBodyLength(f, corrupt.Attestation.BodyLength)
	f.Add(mustCheckInJSON(f, corrupt))
	corrupt = issued.request
	corrupt.Attestation.Signature = corrupt.Certificate.Attestation.Signature
	f.Add(mustCheckInJSON(f, corrupt))
	corrupt = issued.request
	corrupt.Payload.RequestNonce = otherRequestNonce(f)
	f.Add(mustCheckInJSON(f, corrupt))
	corrupt = issued.request
	corrupt.Payload.Build = alternateBuild(f, corrupt.Payload.Build)
	corrupt.Certificate.Body.Build = corrupt.Payload.Build
	f.Add(mustCheckInJSON(f, corrupt))
	f.Add(mutateSigningDomainSeed(f, signingDomainSeedMutation{
		Document: canonical, From: controlplane.SigningDomainCheckInV1,
		To: controlplane.SigningDomainRegistrationV1,
	}))
	addHostileControlplaneJSONSeeds(f)

	f.Fuzz(func(t *testing.T, data []byte) {
		candidate := issued.request
		err := candidate.UnmarshalJSON(data)
		if err != nil {
			requireControlplaneDecodeRefusal(t, err)
			requireCheckInProjection(t, candidate, canonical)
			return
		}
		reencoded := mustCheckInJSON(t, candidate)
		requireCheckInStableRoundTrip(t, candidate, reencoded)
		proof, verifyErr := server.VerifyCheckIn(controlplane.CheckInVerification{
			Request: candidate,
		})
		if bytes.Equal(reencoded, canonical) {
			if verifyErr != nil {
				t.Fatalf("authentic check-in verification error = %v, want nil", verifyErr)
			}
			if err := proof.Validate(); err != nil {
				t.Fatalf("authentic check-in proof Validate() error = %v, want nil", err)
			}
			commit, commitErr := server.CommitCheckIn(controlplane.CheckInCommitRequest{
				CheckIn: proof, Current: candidate.Payload.PreviousWatermark,
				RequiredPolicy: candidate.Payload.AppliedPolicy,
			})
			if commitErr != nil {
				t.Fatalf("authentic CommitCheckIn() error = %v, want nil", commitErr)
			}
			watermark, watermarkErr := commit.Watermark()
			disposition, dispositionErr := commit.Disposition()
			previousGeneration, previousErr := candidate.Payload.PreviousWatermark.Generation.Uint64()
			nextGeneration, nextErr := watermark.Generation.Uint64()
			if errors.Join(watermarkErr, dispositionErr, previousErr, nextErr) != nil ||
				disposition != controlplane.UsageDispositionAccepted ||
				nextGeneration != previousGeneration+1 ||
				watermark.Subject != candidate.Payload.PreviousWatermark.Subject ||
				watermark == candidate.Payload.PreviousWatermark {
				t.Fatalf("authentic usage commit = (%v, %v, %v/%v, %v), want accepted one-generation advance for the exact subject",
					watermark, disposition, previousGeneration, nextGeneration,
					errors.Join(watermarkErr, dispositionErr, previousErr, nextErr))
			}
			replayed, replayErr := server.CommitCheckIn(controlplane.CheckInCommitRequest{
				CheckIn: proof, Current: watermark,
				RequiredPolicy: candidate.Payload.AppliedPolicy,
			})
			if replayErr != nil {
				t.Fatalf("authentic replay CommitCheckIn() error = %v, want nil", replayErr)
			}
			gotReplay, replayDispositionErr := replayed.Disposition()
			gotWatermark, replayWatermarkErr := replayed.Watermark()
			if errors.Join(replayDispositionErr, replayWatermarkErr) != nil ||
				gotReplay != controlplane.UsageDispositionReplay || gotWatermark != watermark {
				t.Fatalf("authentic exact replay = (%v, %v, %v), want replay and unchanged watermark",
					gotReplay, gotWatermark, errors.Join(replayDispositionErr, replayWatermarkErr))
			}
			return
		}
		requireControlplaneVerificationRefusal(t, verifyErr)
		requireZeroVerifiedCheckIn(t, proof)
	})
}

func FuzzCheckInResponseDocumentDecodeAndVerify(f *testing.F) {
	issued := issueTestCheckInResponse(f)
	client := issued.client(f)
	canonical := mustCheckInResponseJSON(f, issued.document)
	f.Add(canonical)
	corrupt := issued.document
	corrupt.Attestation.BodySHA256 = corruptDigest()
	f.Add(mustCheckInResponseJSON(f, corrupt))
	corrupt = issued.document
	corrupt.Attestation.Signer, _ = testSigningKey(f, checkInOtherDeviceSeed)
	f.Add(mustCheckInResponseJSON(f, corrupt))
	corrupt = issued.document
	corrupt.Attestation.BodyLength = incrementBodyLength(f, corrupt.Attestation.BodyLength)
	f.Add(mustCheckInResponseJSON(f, corrupt))
	corrupt = issued.document
	corrupt.Attestation.Signature = corrupt.Payload.Lease.Attestation.Signature
	f.Add(mustCheckInResponseJSON(f, corrupt))
	corrupt = issued.document
	corrupt.Payload.Header.RequestNonce = otherRequestNonce(f)
	f.Add(mustCheckInResponseJSON(f, corrupt))
	corrupt = issued.document
	corrupt.Payload.Header.Account = foreignAccount(f)
	f.Add(mustCheckInResponseJSON(f, corrupt))
	f.Add(mutateSigningDomainSeed(f, signingDomainSeedMutation{
		Document: canonical, From: controlplane.SigningDomainCheckInResponseV1,
		To: controlplane.SigningDomainCheckInV1,
	}))
	addHostileControlplaneJSONSeeds(f)

	f.Fuzz(func(t *testing.T, data []byte) {
		candidate := issued.document
		err := candidate.UnmarshalJSON(data)
		if err != nil {
			requireControlplaneDecodeRefusal(t, err)
			requireCheckInResponseProjection(t, candidate, canonical)
			return
		}
		reencoded := mustCheckInResponseJSON(t, candidate)
		requireCheckInResponseStableRoundTrip(t, candidate, reencoded)
		request := issued.verification()
		request.Document = candidate
		proof, verifyErr := client.VerifyCheckInResponse(request)
		if bytes.Equal(reencoded, canonical) {
			if verifyErr != nil {
				t.Fatalf("authentic check-in response verification error = %v, want nil", verifyErr)
			}
			if err := proof.Validate(); err != nil {
				t.Fatalf("authentic check-in response proof Validate() error = %v, want nil", err)
			}
			return
		}
		requireControlplaneVerificationRefusal(t, verifyErr)
		if proof != (controlplane.VerifiedCheckInResponse{}) {
			t.Fatalf("refused check-in response proof = %v, want zero", proof)
		}
	})
}

func corruptDigest() core.SHA256Digest {
	var value [32]byte
	for index := range value {
		value[index] = 0xa5
	}
	return core.NewSHA256Digest(value)
}

type signingDomainSeedMutation struct {
	Document []byte
	From     controlplane.SigningDomain
	To       controlplane.SigningDomain
}

func mutateSigningDomainSeed(t testing.TB, request signingDomainSeedMutation) []byte {
	t.Helper()
	from, fromErr := request.From.MarshalJSON()
	to, toErr := request.To.MarshalJSON()
	if fromErr != nil || toErr != nil {
		t.Fatalf("SigningDomain seed projections error = %v, want nil", errors.Join(fromErr, toErr))
	}
	if count := bytes.Count(request.Document, from); count != 1 {
		t.Fatalf("signed document source domain occurrences = %d, want 1", count)
	}
	return bytes.Replace(request.Document, from, to, 1)
}

func incrementBodyLength(t testing.TB, current core.ByteCount) core.ByteCount {
	t.Helper()
	value, err := current.Uint64()
	if err != nil {
		t.Fatalf("attestation BodyLength.Uint64() error = %v, want nil", err)
	}
	incremented, err := core.NewByteCount(value + 1)
	if err != nil {
		t.Fatalf("NewByteCount(body length + 1) error = %v, want nil", err)
	}
	return incremented
}

func alternateBuild(t testing.TB, current core.BuildIdentity) core.BuildIdentity {
	t.Helper()
	version := core.NewReleaseVersion(9, 9, 9)
	comparison, err := version.Compare(current.Version())
	if err != nil {
		t.Fatalf("alternate ReleaseVersion.Compare(current) error = %v, want nil", err)
	}
	if comparison == core.ComparisonEqual {
		version = core.NewReleaseVersion(9, 9, 8)
	}
	alternate, err := core.NewBuildIdentity(core.BuildIdentityRequest{
		Offering: current.Offering(), Version: version,
		Commit: current.Commit(), Platform: current.Platform(),
	})
	if err != nil {
		t.Fatalf("NewBuildIdentity(alternate version) error = %v, want nil", err)
	}
	return alternate
}

func foreignAccount(t testing.TB) receipt.AccountIdentity {
	t.Helper()
	account, err := receipt.ParseAccountIdentity(checkInResponseOtherAccountHex)
	if err != nil {
		t.Fatalf("ParseAccountIdentity(foreign) error = %v, want nil", err)
	}
	return account
}

func corruptCertificateDigest(document controlplane.InstallationCertificateDocument) controlplane.InstallationCertificateDocument {
	document.Attestation.BodySHA256 = corruptDigest()
	return document
}

func cloneRegistrationDocument(document controlplane.RegistrationDocument) controlplane.RegistrationDocument {
	if document.Payload.Certificate == nil {
		return document
	}
	certificate := *document.Payload.Certificate
	document.Payload.Certificate = &certificate
	return document
}

func addHostileControlplaneJSONSeeds(f *testing.F) {
	f.Helper()
	for _, seed := range [][]byte{
		nil,
		{},
		[]byte("null"),
		[]byte("{}"),
		[]byte("[]"),
		[]byte("{\"unknown\":true}"),
		[]byte("{\"payload\":null}"),
		bytes.Repeat([]byte{' '}, controlplane.CheckInRequestJSONMaximumBytes+1),
	} {
		f.Add(seed)
	}
}

func requireControlplaneDecodeRefusal(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrControlPlaneContract) {
		t.Fatalf("UnmarshalJSON() error = %v, want %v and %v", err, core.ErrJSONContract, core.ErrControlPlaneContract)
	}
}

func requireControlplaneVerificationRefusal(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, core.ErrControlPlaneContract) {
		t.Fatalf("verification error = %v, want %v", err, core.ErrControlPlaneContract)
	}
}

func requireZeroVerifiedCheckIn(t *testing.T, proof controlplane.VerifiedCheckIn) {
	t.Helper()
	request, err := proof.Request()
	if !errors.Is(err, core.ErrControlPlaneContract) {
		t.Fatalf("refused check-in proof Request() error = %v, want %v", err, core.ErrControlPlaneContract)
	}
	if !isZeroCheckInRequest(request) {
		t.Fatalf("refused check-in proof Request() = %+v, want exact zero request", request)
	}
}

func isZeroCheckInRequest(request controlplane.CheckInRequest) bool {
	payload := request.Payload
	return payload.Window.Units == nil && payload.Window.Outcomes == nil &&
		payload.Window.Bounds == (temporal.IntervalBounds{}) && payload.Window.Freshness == (temporal.Instant{}) &&
		payload.PreviousWatermark == (controlplane.UsageWatermark{}) && payload.LeaseGeneration == (lease.Generation{}) &&
		payload.Build == (core.BuildIdentity{}) && payload.Revision == controlwire.Revision(0) &&
		payload.RequestNonce == (controlwire.RequestNonce{}) && payload.Installation == (lease.DeviceID{}) &&
		payload.AppliedPolicy == (controlwire.PolicyCursor{}) &&
		request.Certificate == (controlplane.InstallationCertificateDocument{}) &&
		request.Attestation == (attest.Envelope[controlplane.SigningDomain]{})
}

func mustCertificateJSON(t testing.TB, document controlplane.InstallationCertificateDocument) []byte {
	t.Helper()
	encoded, err := document.MarshalJSON()
	if err != nil {
		t.Fatalf("certificate MarshalJSON() error = %v, want nil", err)
	}
	return encoded
}

func requireCertificateProjection(t *testing.T, document controlplane.InstallationCertificateDocument, want []byte) {
	t.Helper()
	if got := mustCertificateJSON(t, document); !bytes.Equal(got, want) {
		t.Fatalf("rejected certificate mutated receiver projection = %x, want %x", got, want)
	}
}

func requireCertificateStableRoundTrip(t *testing.T, document controlplane.InstallationCertificateDocument, encoded []byte) {
	t.Helper()
	var decoded controlplane.InstallationCertificateDocument
	if err := decoded.UnmarshalJSON(encoded); err != nil {
		t.Fatalf("accepted certificate re-decode error = %v, want nil", err)
	}
	if got := mustCertificateJSON(t, decoded); !bytes.Equal(got, encoded) {
		t.Fatalf("accepted certificate second projection = %x, want %x", got, encoded)
	}
	if err := document.Validate(); err != nil {
		t.Fatalf("accepted certificate Validate() error = %v, want nil", err)
	}
}

func mustCertificateBodyJSON(t testing.TB, body controlplane.InstallationCertificateBody) []byte {
	t.Helper()
	encoded, err := body.MarshalJSON()
	if err != nil {
		t.Fatalf("certificate body MarshalJSON() error = %v, want nil", err)
	}
	return encoded
}

func requireCertificateBodyProjection(t *testing.T, body controlplane.InstallationCertificateBody, want []byte) {
	t.Helper()
	if got := mustCertificateBodyJSON(t, body); !bytes.Equal(got, want) {
		t.Fatalf("rejected certificate body mutated receiver projection = %x, want %x", got, want)
	}
}

func requireCertificateBodyStableRoundTrip(t *testing.T, body controlplane.InstallationCertificateBody, encoded []byte) {
	t.Helper()
	var decoded controlplane.InstallationCertificateBody
	if err := decoded.UnmarshalJSON(encoded); err != nil {
		t.Fatalf("accepted certificate body re-decode error = %v, want nil", err)
	}
	if got := mustCertificateBodyJSON(t, decoded); !bytes.Equal(got, encoded) {
		t.Fatalf("accepted certificate body second projection = %x, want %x", got, encoded)
	}
	if err := body.Validate(); err != nil {
		t.Fatalf("accepted certificate body Validate() error = %v, want nil", err)
	}
}

func mustRegistrationJSON(t testing.TB, document controlplane.RegistrationDocument) []byte {
	t.Helper()
	encoded, err := document.MarshalJSON()
	if err != nil {
		t.Fatalf("registration MarshalJSON() error = %v, want nil", err)
	}
	return encoded
}

func requireRegistrationProjection(t *testing.T, document controlplane.RegistrationDocument, want []byte) {
	t.Helper()
	if got := mustRegistrationJSON(t, document); !bytes.Equal(got, want) {
		t.Fatalf("rejected registration mutated receiver projection = %x, want %x", got, want)
	}
}

func requireRegistrationStableRoundTrip(t *testing.T, document controlplane.RegistrationDocument, encoded []byte) {
	t.Helper()
	var decoded controlplane.RegistrationDocument
	if err := decoded.UnmarshalJSON(encoded); err != nil {
		t.Fatalf("accepted registration re-decode error = %v, want nil", err)
	}
	if got := mustRegistrationJSON(t, decoded); !bytes.Equal(got, encoded) {
		t.Fatalf("accepted registration second projection = %x, want %x", got, encoded)
	}
	if err := document.Validate(); err != nil {
		t.Fatalf("accepted registration Validate() error = %v, want nil", err)
	}
}

func mustCheckInJSON(t testing.TB, document controlplane.CheckInRequest) []byte {
	t.Helper()
	encoded, err := document.MarshalJSON()
	if err != nil {
		t.Fatalf("check-in MarshalJSON() error = %v, want nil", err)
	}
	return encoded
}

func requireCheckInProjection(t *testing.T, document controlplane.CheckInRequest, want []byte) {
	t.Helper()
	if got := mustCheckInJSON(t, document); !bytes.Equal(got, want) {
		t.Fatalf("rejected check-in mutated receiver projection = %x, want %x", got, want)
	}
}

func requireCheckInStableRoundTrip(t *testing.T, document controlplane.CheckInRequest, encoded []byte) {
	t.Helper()
	var decoded controlplane.CheckInRequest
	if err := decoded.UnmarshalJSON(encoded); err != nil {
		t.Fatalf("accepted check-in re-decode error = %v, want nil", err)
	}
	if got := mustCheckInJSON(t, decoded); !bytes.Equal(got, encoded) {
		t.Fatalf("accepted check-in second projection = %x, want %x", got, encoded)
	}
	if err := document.Validate(); err != nil {
		t.Fatalf("accepted check-in Validate() error = %v, want nil", err)
	}
}

func mustCheckInResponseJSON(t testing.TB, document controlplane.CheckInResponseDocument) []byte {
	t.Helper()
	encoded, err := document.MarshalJSON()
	if err != nil {
		t.Fatalf("check-in response MarshalJSON() error = %v, want nil", err)
	}
	return encoded
}

func requireCheckInResponseProjection(t *testing.T, document controlplane.CheckInResponseDocument, want []byte) {
	t.Helper()
	if got := mustCheckInResponseJSON(t, document); !bytes.Equal(got, want) {
		t.Fatalf("rejected check-in response mutated receiver projection = %x, want %x", got, want)
	}
}

func requireCheckInResponseStableRoundTrip(t *testing.T, document controlplane.CheckInResponseDocument, encoded []byte) {
	t.Helper()
	var decoded controlplane.CheckInResponseDocument
	if err := decoded.UnmarshalJSON(encoded); err != nil {
		t.Fatalf("accepted check-in response re-decode error = %v, want nil", err)
	}
	if got := mustCheckInResponseJSON(t, decoded); !bytes.Equal(got, encoded) {
		t.Fatalf("accepted check-in response second projection = %x, want %x", got, encoded)
	}
	if err := document.Validate(); err != nil {
		t.Fatalf("accepted check-in response Validate() error = %v, want nil", err)
	}
}
