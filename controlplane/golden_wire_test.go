package controlplane_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
)

// TestGoldenDocumentsRoundTripByteExact is the proof that both ends of the
// exchange agree.
//
// The fixtures are bytes a real authority produced with its real signing
// producers. Decoding one and re-encoding it must return the identical bytes,
// because the signature covers exactly those bytes: a field reordered, a bound
// widened, or a number rendered differently would verify here and fail against
// a live authority, which is the failure that cannot be debugged from a
// customer's machine.
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
				t.Fatalf("re-encoded %s does not match the authority's bytes\ngot  = %s\nwant = %s",
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

func readGolden(t *testing.T, name string) []byte {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("os.ReadFile(%s) error = %v, want nil", name, err)
	}
	return data
}
