package controlplane_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
)

// TestGoldenDocumentsRoundTripByteExact is the proof that both ends of the
// exchange agree.
//
// The registration fixtures and the check-in response are bytes a real
// authority produced with its real signing producers. The check-in request is
// what an installation sends, so it is produced by this repository's own issuing
// path from the keys the fixture holds, and
// TestGoldenCheckInRequestIsWhatTheProducersEmit is what keeps it honest.
//
// Decoding one and re-encoding it must return the identical bytes, because the
// signature covers exactly those bytes: a field reordered, a bound widened, or a
// number rendered differently would verify here and fail against a live
// authority, which is the failure that cannot be debugged from a customer's
// machine.
//
// This is the test that would have caught the whole class of drift that copying
// these types into each product was creating.
func TestGoldenDocumentsRoundTripByteExact(t *testing.T) {
	t.Parallel()

	// Each case decodes and re-encodes its own concrete document type. Routing
	// the value through a shared any-typed helper would have thrown away the
	// static type at exactly the point the test exists to pin.
	cases := []struct {
		reencode func([]byte) ([]byte, error)
		name     string
		file     string
	}{
		{
			name: "registration request", file: "registration_request.json",
			reencode: func(data []byte) ([]byte, error) {
				var value controlplane.RegistrationRequest
				if err := value.UnmarshalJSON(data); err != nil {
					return nil, err
				}
				return json.Marshal(value)
			},
		},
		{
			name: "registration response", file: "registration_response.json",
			reencode: func(data []byte) ([]byte, error) {
				var value controlplane.RegistrationDocument
				if err := value.UnmarshalJSON(data); err != nil {
					return nil, err
				}
				return json.Marshal(value)
			},
		},
		{
			name: "check-in request", file: "check_in_request.json",
			reencode: func(data []byte) ([]byte, error) {
				var value controlplane.CheckInRequest
				if err := value.UnmarshalJSON(data); err != nil {
					return nil, err
				}
				return json.Marshal(value)
			},
		},
		{
			name: "check-in response", file: "check_in_response.json",
			reencode: func(data []byte) ([]byte, error) {
				var value controlplane.CheckInResponseDocument
				if err := value.UnmarshalJSON(data); err != nil {
					return nil, err
				}
				return json.Marshal(value)
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			want := readGolden(t, testCase.file)
			got, err := testCase.reencode(want)
			if err != nil {
				t.Fatalf("decode and re-encode of %s error = %v, want nil", testCase.file, err)
			}
			if string(got) != string(want) {
				t.Fatalf("re-encoded %s does not match the committed bytes\ngot  = %s\nwant = %s",
					testCase.file, got, want)
			}
		})
	}
}

// TestGoldenRegistrationResponseCarriesTheFactsItClaims reads the decoded
// document and asserts the facts a caller acts on, so a decoder that produced
// a well-formed but empty document could not pass the byte comparison alone.
func TestGoldenRegistrationResponseCarriesTheFactsItClaims(t *testing.T) {
	t.Parallel()

	var document controlplane.RegistrationDocument
	if err := document.UnmarshalJSON(readGolden(t, "registration_response.json")); err != nil {
		t.Fatalf("UnmarshalJSON() error = %v, want nil", err)
	}
	payload := document.Payload
	if got := payload.Header.Status; got != controlplane.ProductStatusActive {
		t.Errorf("header status = %v, want %v", got, controlplane.ProductStatusActive)
	}
	if got := payload.Header.Status.AdmitsGrant(); !got {
		t.Errorf("golden status %v AdmitsGrant() = %t, want true: the golden carries a grant",
			payload.Header.Status, got)
	}
	if payload.Certificate == nil {
		t.Fatalf("granted registration certificate = %v, want a certificate", payload.Certificate)
	}
	if got := document.Attestation.Domain; got != controlplane.SigningDomainRegistrationV1 {
		t.Errorf("response signing domain = %v, want %v", got, controlplane.SigningDomainRegistrationV1)
	}
	if got := payload.Certificate.Attestation.Domain; got != controlplane.SigningDomainInstallationCertificateV1 {
		t.Errorf("certificate signing domain = %v, want %v",
			got, controlplane.SigningDomainInstallationCertificateV1)
	}
	// The certificate binds to the same installation the header names, which is
	// the fact that stops one machine's certificate being replayed on another.
	if got, want := payload.Certificate.Body.Subject.DeviceID, payload.Header.Installation; got != want {
		t.Errorf("certificate device = %v, want the header installation %v", got, want)
	}
	if got, want := payload.Watermark.Subject, payload.Certificate.Body.Subject; got != want {
		t.Errorf("watermark subject = %v, want the certificate subject %v", got, want)
	}
}

// TestGoldenCheckInRequestIsWhatTheProducersEmit closes the gap a golden alone
// leaves.
//
// A round trip proves the decoder and the encoder agree with each other, which
// they would still do if both drifted together. This proves the committed bytes
// are what the issuing path actually produces from stated typed inputs. The
// authority whose key signed the registration goldens does not sign this one:
// the request is signed by a device key, and the fixture holds both keys, so
// every signature in the file is real and verifiable rather than a recorded
// artifact nobody can check.
func TestGoldenCheckInRequestIsWhatTheProducersEmit(t *testing.T) {
	t.Parallel()

	issued := issueTestCheckIn(t, core.OfferingPeachfuzz, testCheckInWindow())
	got, err := issued.request.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v, want nil", err)
	}
	want := readGolden(t, "check_in_request.json")
	if string(got) != string(want) {
		t.Fatalf("issued check-in does not match the committed bytes\ngot  = %s\nwant = %s", got, want)
	}
}

// TestGoldenCheckInRequestCarriesTheFactsItClaims reads the decoded document and
// asserts the facts an authority acts on, so a decoder that produced a
// well-formed but empty document could not pass the byte comparison alone.
func TestGoldenCheckInRequestCarriesTheFactsItClaims(t *testing.T) {
	t.Parallel()

	var request controlplane.CheckInRequest
	if err := request.UnmarshalJSON(readGolden(t, "check_in_request.json")); err != nil {
		t.Fatalf("UnmarshalJSON() error = %v, want nil", err)
	}
	payload := request.Payload
	if got := payload.Build.Offering(); got != core.OfferingPeachfuzz {
		t.Errorf("golden offering = %v, want %v", got, core.OfferingPeachfuzz)
	}
	// The credential binds to the same installation the payload names, which is
	// the fact that stops one machine's check-in being replayed as another's.
	if got, want := request.Certificate.Body.Subject.DeviceID, payload.Installation; got != want {
		t.Errorf("credential device = %v, want the payload installation %v", got, want)
	}
	if got, want := payload.PreviousWatermark.Subject, request.Certificate.Body.Subject; got != want {
		t.Errorf("watermark subject = %v, want the credential subject %v", got, want)
	}
	if got, want := payload.PreviousWatermark.Generation, payload.LeaseGeneration; got != want {
		t.Errorf("watermark generation = %v, want the claimed lease generation %v", got, want)
	}
	if got := request.Attestation.Domain; got != controlplane.SigningDomainCheckInV1 {
		t.Errorf("request signing domain = %v, want %v", got, controlplane.SigningDomainCheckInV1)
	}
	if got := len(payload.Window.Units); got == 0 {
		t.Errorf("golden window units = %d, want a window that reports work", got)
	}
}

func readGolden(t *testing.T, name string) []byte {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("os.ReadFile(%s) error = %v, want nil", name, err)
	}
	return data
}
