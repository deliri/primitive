package controlplane_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
)

type verificationLayerProbe struct {
	name     string
	positive func() error
	negative func() error
	neutral  func() error
}

func TestControlplaneSignedVerificationLayerTriad(t *testing.T) {
	t.Parallel()

	certificate := issueTestCheckIn(t, core.OfferingWitness, testCheckInWindow())
	registration := issueTestRegistration(t)
	checkIn := issueTestCheckIn(t, core.OfferingBug, testCheckInWindow())
	response := issueTestCheckInResponse(t)
	probes := []verificationLayerProbe{
		certificateVerificationProbe(certificate),
		registrationVerificationProbe(registration),
		checkInVerificationProbe(checkIn),
		checkInResponseVerificationProbe(response),
	}
	for _, probe := range probes {
		probe := probe
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

func certificateVerificationProbe(issued issuedCheckIn) verificationLayerProbe {
	return verificationLayerProbe{
		name: "installation certificate authority and device trust",
		positive: func() error {
			proof, err := controlplane.VerifyInstallationCertificate(issued.certificate, issued.trusted)
			if err != nil {
				return err
			}
			return proof.Validate()
		},
		negative: func() error {
			_, err := controlplane.VerifyInstallationCertificate(
				corruptCertificateDigest(issued.certificate), issued.trusted,
			)
			return err
		},
		neutral: func() error {
			_, err := controlplane.VerifyInstallationCertificate(
				controlplane.InstallationCertificateDocument{}, attest.TrustedKeys{},
			)
			return err
		},
	}
}

func registrationVerificationProbe(issued issuedRegistration) verificationLayerProbe {
	return verificationLayerProbe{
		name: "registration response and nested authorities",
		positive: func() error {
			proof, err := controlplane.VerifyRegistration(issued.verification())
			if err != nil {
				return err
			}
			return proof.Validate()
		},
		negative: func() error {
			request := issued.verification()
			request.Document.Attestation.BodySHA256 = corruptDigest()
			_, err := controlplane.VerifyRegistration(request)
			return err
		},
		neutral: func() error {
			_, err := controlplane.VerifyRegistration(controlplane.RegistrationVerification{})
			return err
		},
	}
}

func checkInVerificationProbe(issued issuedCheckIn) verificationLayerProbe {
	return verificationLayerProbe{
		name: "check-in certificate then device signature",
		positive: func() error {
			proof, err := controlplane.VerifyCheckIn(controlplane.CheckInVerification{
				Request: issued.request, TrustedKeys: issued.trusted,
			})
			if err != nil {
				return err
			}
			return proof.Validate()
		},
		negative: func() error {
			request := issued.request
			request.Attestation.BodySHA256 = corruptDigest()
			_, err := controlplane.VerifyCheckIn(controlplane.CheckInVerification{
				Request: request, TrustedKeys: issued.trusted,
			})
			return err
		},
		neutral: func() error {
			_, err := controlplane.VerifyCheckIn(controlplane.CheckInVerification{})
			return err
		},
	}
}

func checkInResponseVerificationProbe(issued issuedCheckInResponse) verificationLayerProbe {
	return verificationLayerProbe{
		name: "check-in response and nested lease",
		positive: func() error {
			proof, err := controlplane.VerifyCheckInResponse(issued.verification())
			if err != nil {
				return err
			}
			return proof.Validate()
		},
		negative: func() error {
			request := issued.verification()
			request.Document.Attestation.BodySHA256 = corruptDigest()
			_, err := controlplane.VerifyCheckInResponse(request)
			return err
		},
		neutral: func() error {
			_, err := controlplane.VerifyCheckInResponse(controlplane.CheckInResponseVerification{})
			return err
		},
	}
}

func FuzzInstallationCertificateDocumentDecodeAndVerify(f *testing.F) {
	issued := issueTestCheckIn(f, core.OfferingWitness, testCheckInWindow())
	canonical := mustCertificateJSON(f, issued.certificate)
	f.Add(canonical)
	f.Add(mustCertificateJSON(f, corruptCertificateDigest(issued.certificate)))
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
		proof, verifyErr := controlplane.VerifyInstallationCertificate(candidate, issued.trusted)
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
	})
}

func FuzzInstallationCertificateBodyDecodeAndVerify(f *testing.F) {
	issued := issueTestCheckIn(f, core.OfferingWitness, testCheckInWindow())
	canonical := mustCertificateBodyJSON(f, issued.certificate.Body)
	f.Add(canonical)
	foreign := issued.certificate.Body
	foreign.IssuedAt = testInstant(f, checkInIssuedAt+1)
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
		proof, verifyErr := controlplane.VerifyInstallationCertificate(document, issued.trusted)
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
	})
}

func FuzzRegistrationDocumentDecodeAndVerify(f *testing.F) {
	issued := issueTestRegistration(f)
	canonical := mustRegistrationJSON(f, issued.document)
	f.Add(canonical)
	corrupt := issued.document
	corrupt.Attestation.BodySHA256 = corruptDigest()
	f.Add(mustRegistrationJSON(f, corrupt))
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
		proof, verifyErr := controlplane.VerifyRegistration(request)
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
	})
}

func FuzzCheckInRequestDecodeAndVerify(f *testing.F) {
	issued := issueTestCheckIn(f, core.OfferingWitness, testCheckInWindow())
	canonical := mustCheckInJSON(f, issued.request)
	f.Add(canonical)
	corrupt := issued.request
	corrupt.Attestation.BodySHA256 = corruptDigest()
	f.Add(mustCheckInJSON(f, corrupt))
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
		proof, verifyErr := controlplane.VerifyCheckIn(controlplane.CheckInVerification{
			Request: candidate, TrustedKeys: issued.trusted,
		})
		if bytes.Equal(reencoded, canonical) {
			if verifyErr != nil {
				t.Fatalf("authentic check-in verification error = %v, want nil", verifyErr)
			}
			if err := proof.Validate(); err != nil {
				t.Fatalf("authentic check-in proof Validate() error = %v, want nil", err)
			}
			return
		}
		requireControlplaneVerificationRefusal(t, verifyErr)
	})
}

func FuzzCheckInResponseDocumentDecodeAndVerify(f *testing.F) {
	issued := issueTestCheckInResponse(f)
	canonical := mustCheckInResponseJSON(f, issued.document)
	f.Add(canonical)
	corrupt := issued.document
	corrupt.Attestation.BodySHA256 = corruptDigest()
	f.Add(mustCheckInResponseJSON(f, corrupt))
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
		proof, verifyErr := controlplane.VerifyCheckInResponse(request)
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
	})
}

func corruptDigest() core.SHA256Digest {
	var value [32]byte
	for index := range value {
		value[index] = 0xa5
	}
	return core.NewSHA256Digest(value)
}

func corruptCertificateDigest(document controlplane.InstallationCertificateDocument) controlplane.InstallationCertificateDocument {
	document.Attestation.BodySHA256 = corruptDigest()
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
