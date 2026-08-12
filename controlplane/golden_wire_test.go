package controlplane_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
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

// TestGoldenRegistrationRequestCarriesTheFactsItClaims pins what this fixture
// chose to be, which a round trip cannot.
//
// A golden has invariants and it has choices. Validate already enforces the
// invariants, so restating them here would prove nothing. The choices are the
// coverage: which product this request enrols, and which protocol revision it
// speaks. Regenerated against another offering or another revision, the byte
// comparison would still pass while the fixture quietly stopped covering the
// path it was built for.
//
// Those are the only two, and this asserts both. Everything else a registration
// request carries is an invariant: the token has to be well formed and the
// installation has to be the identity its own device key derives, or
// UnmarshalJSON would have refused the bytes above. Re-checking either one here
// would be an assertion that cannot fail, which is worse than no assertion,
// because it reads as coverage.
func TestGoldenRegistrationRequestCarriesTheFactsItClaims(t *testing.T) {
	t.Parallel()

	var request controlplane.RegistrationRequest
	if err := request.UnmarshalJSON(readGolden(t, "registration_request.json")); err != nil {
		t.Fatalf("UnmarshalJSON() error = %v, want nil", err)
	}
	if got := request.Build.Offering(); got != core.OfferingPeachfuzz {
		t.Errorf("golden offering = %v, want %v", got, core.OfferingPeachfuzz)
	}
	revision, err := controlwire.ParseRevision(checkInRevisionText)
	if err != nil {
		t.Fatalf("ParseRevision(%s) error = %v, want nil", checkInRevisionText, err)
	}
	if got := request.Revision; got != revision {
		t.Errorf("golden protocol revision = %v, want %v", got, revision)
	}
}

// TestGoldenCheckInResponseCarriesTheFactsItClaims pins this fixture's choices
// the same way.
//
// It is an acceptance of a first window under an active product, so it covers
// the path where a watermark advances. A refusal, a stopped status, or a later
// generation would all round trip byte exact and none of them would exercise
// what this fixture exists for.
//
// It does not pair with check_in_request.json. That one is signed by keys this
// repository holds so its signatures can be verified; this one is bytes a real
// authority produced. They are independent fixtures, and nothing here should
// read as one exchange.
func TestGoldenCheckInResponseCarriesTheFactsItClaims(t *testing.T) {
	t.Parallel()

	var document controlplane.CheckInResponseDocument
	if err := document.UnmarshalJSON(readGolden(t, "check_in_response.json")); err != nil {
		t.Fatalf("UnmarshalJSON() error = %v, want nil", err)
	}
	payload := document.Payload
	if got := payload.Disposition; got != controlplane.UsageDispositionAccepted {
		t.Errorf("golden disposition = %v, want %v", got, controlplane.UsageDispositionAccepted)
	}
	if got := payload.Disposition.AdvancesWatermark(); !got {
		t.Errorf("golden disposition %v AdvancesWatermark() = %t, want true: the golden accepts",
			payload.Disposition, got)
	}
	if got := payload.Header.Status; got != controlplane.ProductStatusActive {
		t.Errorf("golden status = %v, want %v", got, controlplane.ProductStatusActive)
	}
	if got := payload.Lease.Decision.Outcome(); got != lease.OutcomeGrant {
		t.Errorf("golden lease outcome = %v, want %v: the golden covers the grant path", got, lease.OutcomeGrant)
	}
	initial, err := lease.NewGeneration(controlplane.UsageWatermarkInitialGeneration)
	if err != nil {
		t.Fatalf("NewGeneration() error = %v, want nil", err)
	}
	if got := payload.Watermark.Generation; got != initial {
		t.Errorf("golden watermark generation = %v, want the first accepted window %v", got, initial)
	}
	if got := document.Attestation.Domain; got != controlplane.SigningDomainCheckInResponseV1 {
		t.Errorf("response signing domain = %v, want %v", got, controlplane.SigningDomainCheckInResponseV1)
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

func readGolden(t testing.TB, name string) []byte {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("os.ReadFile(%s) error = %v, want nil", name, err)
	}
	return data
}
