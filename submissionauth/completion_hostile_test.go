package submissionauth

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"encoding/json/jsontext"
	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/submission"
)

const (
	authCompletionObjectPath = "/custody/proof.json"
	authCompletionQuery      = "?X-Goog-Signature=fixture&X-Goog-SignedHeaders=" +
		"host%3Bx-goog-hash%3Bx-goog-if-generation-match"
	authCompletionFutureHorizonSeconds = 10 * 365 * 24 * 60 * 60
)

var authCompletionContent = []byte(`{"proof":"source-free"}`)

type authCompletionFixtureRequest struct {
	offering      core.Offering
	generation    int64
	authorityByte byte
	deviceByte    byte
	nonceByte     byte
}

type authCompletionFixture struct {
	grant                submission.GrantDocument
	grantProjection      submission.GrantProjection
	credentialed         CompletionDocument
	completionDocument   submission.CompletionDocument
	completionProjection submission.CompletionProjection
	request              authFixture
	verifiedRequest      Verified
}

func TestCredentialedCompletionProjectionLayerTriadBindsWithoutSenderSideDecode(t *testing.T) {
	t.Parallel()

	base := newAuthCompletionFixture(t, authCompletionFixtureRequest{})
	projection, err := AssembleCompletionProjection(CompletionProjectionAssembly{
		Completion: base.completionProjection, Certificate: base.request.certificate,
	})
	if err != nil {
		t.Fatalf("AssembleCompletionProjection() error = %v, want nil", err)
	}
	encoded, err := projection.MarshalJSON()
	if err != nil {
		t.Fatalf("CompletionProjection.MarshalJSON() error = %v, want nil", err)
	}
	var got CompletionDocument
	if err := got.UnmarshalJSON(encoded); err != nil || got != base.credentialed {
		t.Fatalf("credentialed projection receive = (%v, %v), want exact %v and nil", got, err, base.credentialed)
	}
	other := newAuthCompletionFixture(t, authCompletionFixtureRequest{
		authorityByte: 0x61, deviceByte: 0x62, nonceByte: 0x63, offering: submissionAuthOffering(t, 1),
	})
	if got, err := AssembleCompletionProjection(CompletionProjectionAssembly{
		Completion: base.completionProjection, Certificate: other.request.certificate,
	}); !errors.Is(err, core.ErrControlPlaneResponseBinding) || got != (CompletionProjection{}) {
		t.Fatalf("AssembleCompletionProjection(other certificate) = (%v, %v), want zero and errors.Is %v",
			got, err, core.ErrControlPlaneResponseBinding)
	}
	if got, err := AssembleCompletionProjection(CompletionProjectionAssembly{}); !errors.Is(err, core.ErrControlPlaneContract) || got != (CompletionProjection{}) {
		t.Fatalf("AssembleCompletionProjection(zero) = (%v, %v), want zero and errors.Is %v",
			got, err, core.ErrControlPlaneContract)
	}
}

func TestCredentialedCompletionProjectionEncodeValidatedJSONHostileBindingMatrix(t *testing.T) {
	t.Parallel()

	type validCase struct {
		name    string
		request authCompletionFixtureRequest
	}
	valid := []validCase{
		{name: "product beta default fixture encodes as receive-only projection"},
		{name: "product alpha nonce 2 encodes as receive-only projection", request: authCompletionFixtureRequest{
			offering: submissionAuthOffering(t, 1), nonceByte: 0x02,
		}},
		{name: "product gamma nonce 3 encodes as receive-only projection", request: authCompletionFixtureRequest{
			offering: submissionAuthOffering(t, 3), nonceByte: 0x03,
		}},
		{name: "product beta nonce 4 device 0x32 encodes as receive-only projection", request: authCompletionFixtureRequest{
			nonceByte: 0x04, deviceByte: 0x32,
		}},
		{name: "product alpha nonce 5 authority 0x22 encodes as receive-only projection", request: authCompletionFixtureRequest{
			offering: submissionAuthOffering(t, 1), nonceByte: 0x05, authorityByte: 0x22,
		}},
		{name: "product gamma nonce 6 encodes as receive-only projection", request: authCompletionFixtureRequest{
			offering: submissionAuthOffering(t, 3), nonceByte: 0x06,
		}},
		{name: "product beta nonce 7 encodes as receive-only projection", request: authCompletionFixtureRequest{
			nonceByte: 0x07,
		}},
		{name: "product alpha nonce 8 encodes as receive-only projection", request: authCompletionFixtureRequest{
			offering: submissionAuthOffering(t, 1), nonceByte: 0x08,
		}},
		{name: "product gamma nonce 9 encodes as receive-only projection", request: authCompletionFixtureRequest{
			offering: submissionAuthOffering(t, 3), nonceByte: 0x09,
		}},
		{name: "product beta nonce 10 encodes as receive-only projection", request: authCompletionFixtureRequest{
			nonceByte: 0x0a,
		}},
	}
	if len(valid) < 10 {
		t.Fatalf("credentialed completion EncodeValidatedJSON valid cases = %d, want at least 10", len(valid))
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fixture := newAuthCompletionFixture(t, tc.request)
			projection := assembleAuthCompletionProjection(t, fixture)
			canonical, err := projection.MarshalJSON()
			if err != nil {
				t.Fatalf("CompletionProjection.MarshalJSON() error = %v, want nil", err)
			}
			got, gotErr := core.EncodeValidatedJSON(projection, core.DefaultStrictJSONLimits())
			if gotErr != nil || !bytes.Equal(got, canonical) {
				t.Fatalf("EncodeValidatedJSON(CompletionProjection) = (%d bytes, %v), want exact %d-byte receive-only projection",
					len(got), gotErr, len(canonical))
			}
			if err := projection.ValidateJSONProjection(got, core.DefaultStrictJSONLimits()); err != nil {
				t.Fatalf("ValidateJSONProjection(exact encoded bytes) error = %v, want nil", err)
			}
		})
	}

	fixture := newAuthCompletionFixture(t, authCompletionFixtureRequest{nonceByte: 0x51})
	projection := assembleAuthCompletionProjection(t, fixture)
	canonical, err := projection.MarshalJSON()
	if err != nil {
		t.Fatalf("CompletionProjection.MarshalJSON() error = %v, want nil", err)
	}
	foreign := newAuthCompletionFixture(t, authCompletionFixtureRequest{
		offering: submissionAuthOffering(t, 1), authorityByte: 0x61, deviceByte: 0x62, nonceByte: 0x63,
	})
	foreignProjection := assembleAuthCompletionProjection(t, foreign)
	foreignEncoded, err := foreignProjection.MarshalJSON()
	if err != nil {
		t.Fatalf("foreign CompletionProjection.MarshalJSON() error = %v, want nil", err)
	}
	if bytes.Equal(foreignEncoded, canonical) {
		t.Fatalf("foreign credentialed completion bytes = %d identical bytes, want a load-bearing difference", len(canonical))
	}
	certificateJSON, err := json.Marshal(fixture.request.certificate)
	if err != nil {
		t.Fatalf("json.Marshal(certificate) error = %v, want nil", err)
	}
	completionJSON, err := json.Marshal(fixture.completionDocument)
	if err != nil {
		t.Fatalf("json.Marshal(completion) error = %v, want nil", err)
	}
	reordered := append(append(append([]byte(`{"completion":`), completionJSON...), `,"certificate":`...), certificateJSON...)
	reordered = append(reordered, '}')
	if bytes.Equal(reordered, canonical) {
		reordered = append(append(append([]byte(`{"certificate":`), certificateJSON...), `,"completion":`...), completionJSON...)
		reordered = append(reordered, '}')
	}
	if bytes.Equal(reordered, canonical) {
		t.Fatalf("certificate-first completion bytes = %d identical bytes, want a genuine member reorder", len(canonical))
	}
	indented := jsontext.Value(bytes.Clone(canonical))
	if err := indented.Indent(jsontext.WithIndent("  ")); err != nil {
		t.Fatalf("json.Indent(credentialed completion) error = %v, want nil", err)
	}
	unknown := append(bytes.Clone(canonical[:len(canonical)-1]), []byte(`,"future":true}`)...)
	duplicateCompletion := append(bytes.Clone(canonical[:len(canonical)-1]), []byte(`,"completion":null}`)...)
	duplicateCertificate := append(bytes.Clone(canonical[:len(canonical)-1]), []byte(`,"certificate":null}`)...)
	mutated := bytes.Clone(canonical)
	mutated[len(mutated)/2] ^= 0x01
	type rejectCase struct {
		name string
		data []byte
	}
	reject := []rejectCase{
		{name: "empty input"},
		{name: "whitespace only", data: []byte(" \t\r\n")},
		{name: "null root", data: []byte(`null`)},
		{name: "boolean root", data: []byte(`true`)},
		{name: "scalar root", data: []byte(`1`)},
		{name: "string root", data: []byte(`"completion"`)},
		{name: "array root", data: []byte(`[]`)},
		{name: "empty object", data: []byte(`{}`)},
		{name: "unknown member", data: unknown},
		{name: "duplicate completion", data: duplicateCompletion},
	}
	if len(reject) < 10 {
		t.Fatalf("credentialed completion ValidateJSONProjection reject cases = %d, want at least 10", len(reject))
	}
	for _, tc := range reject {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := projection.ValidateJSONProjection(tc.data, core.DefaultStrictJSONLimits()); !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("ValidateJSONProjection(%s) error = %v, want errors.Is %v", tc.name, err, core.ErrJSONContract)
			}
		})
	}
	boundary := []rejectCase{
		{name: "duplicate certificate", data: duplicateCertificate},
		{name: "missing completion", data: []byte(`{"certificate":null}`)},
		{name: "missing certificate", data: []byte(`{"completion":null}`)},
		{name: "completion wrong type", data: []byte(`{"completion":true,"certificate":null}`)},
		{name: "certificate wrong type", data: []byte(`{"completion":null,"certificate":true}`)},
		{name: "truncated opening", data: []byte(`{`)},
		{name: "truncated array", data: []byte(`[`)},
		{name: "truncated canonical", data: canonical[:len(canonical)-1]},
		{name: "half truncated canonical", data: canonical[:len(canonical)/2]},
		{name: "two documents", data: append(bytes.Clone(canonical), canonical...)},
		{name: "trailing scalar", data: append(bytes.Clone(canonical), []byte(` 0`)...)},
		{name: "leading space", data: append([]byte(" "), canonical...)},
		{name: "trailing newline", data: append(bytes.Clone(canonical), '\n')},
		{name: "carriage return framing", data: append(append([]byte("\r"), canonical...), '\r')},
		{name: "mixed outer whitespace", data: append(append([]byte("\t\r\n"), canonical...), ' ', '\t')},
		{name: "reordered members", data: reordered},
		{name: "indented", data: []byte(indented)},
		{name: "foreign authentic projection", data: foreignEncoded},
		{name: "mutated interior byte", data: mutated},
		{name: "one above ceiling", data: authLeftPadJSON(canonical, CompletionDocumentJSONMaximumBytes+1)},
	}
	if len(boundary) < 20 {
		t.Fatalf("credentialed completion ValidateJSONProjection boundary cases = %d, want at least 20", len(boundary))
	}
	for _, tc := range boundary {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := projection.ValidateJSONProjection(tc.data, core.DefaultStrictJSONLimits()); !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("ValidateJSONProjection(%s) error = %v, want errors.Is %v", tc.name, err, core.ErrJSONContract)
			}
		})
	}

	t.Run("zero projection is refused before any wire bytes exist", func(t *testing.T) {
		t.Parallel()

		if got, gotErr := core.EncodeValidatedJSON(CompletionProjection{}, core.DefaultStrictJSONLimits()); got != nil ||
			!errors.Is(gotErr, core.ErrJSONContract) || !errors.Is(gotErr, core.ErrControlPlaneContract) {
			t.Fatalf("EncodeValidatedJSON(zero) = (%d bytes, %v), want nil and %v/%v",
				len(got), gotErr, core.ErrJSONContract, core.ErrControlPlaneContract)
		}
	})
	t.Run("one-byte-short document limit refuses an otherwise authentic projection", func(t *testing.T) {
		t.Parallel()

		maximum, err := core.NewByteCount(uint64(len(canonical) - 1))
		if err != nil {
			t.Fatalf("NewByteCount(one below projection length) error = %v, want nil", err)
		}
		limits := core.DefaultStrictJSONLimits()
		limits.DocumentMaximumBytes = maximum
		if got, gotErr := core.EncodeValidatedJSON(projection, limits); got != nil || !errors.Is(gotErr, core.ErrJSONContract) {
			t.Fatalf("EncodeValidatedJSON(one-byte-short limit) = (%d bytes, %v), want nil and %v",
				len(got), gotErr, core.ErrJSONContract)
		}
	})
}

func assembleAuthCompletionProjection(t testing.TB, fixture authCompletionFixture) CompletionProjection {
	t.Helper()

	projection, err := AssembleCompletionProjection(CompletionProjectionAssembly{
		Completion: fixture.completionProjection, Certificate: fixture.request.certificate,
	})
	if err != nil {
		t.Fatalf("AssembleCompletionProjection() error = %v, want nil", err)
	}
	return projection
}

// TestCredentialedCompletionLayerTriadClosesRepresentativeOpaqueOfferings proves that the
// authority certificate, original request, authority grant, real provider
// transfer, and device completion all survive the complete authentication
// path for every admitted product without a product-specific branch.
func TestCredentialedCompletionLayerTriadClosesRepresentativeOpaqueOfferings(t *testing.T) {
	t.Parallel()

	admitted := 0
	for value, offering := range []core.Offering{
		submissionAuthOffering(t, 1),
		submissionAuthOffering(t, 127),
		submissionAuthOffering(t, 255),
	} {
		admitted++
		t.Run(offering.String(), func(t *testing.T) {
			t.Parallel()

			fixture := newAuthCompletionFixture(t, authCompletionFixtureRequest{
				offering: offering, authorityByte: byte(value) + 0x20,
				deviceByte: byte(value) + 0x40, nonceByte: byte(value) + 1,
			})
			route, routeErr := fixture.credentialed.ControlRoute()
			if routeErr != nil || route.Offering() != offering ||
				route.Family() != controlwire.RouteFamilySubmissionCompletions ||
				fixture.credentialed.ControlNonce() != fixture.completionDocument.Payload.Nonce {
				t.Fatalf("completion control projection(%v) = (%v, %v, %v), want exact route and signed nonce",
					offering, route, fixture.credentialed.ControlNonce(), routeErr)
			}
			verified, err := VerifyCompletion(CompletionVerification{
				Document: fixture.credentialed, Request: fixture.verifiedRequest,
				Grant: fixture.grant, GrantKeys: fixture.request.trusted,
				Server: submissionAuthServer(t, fixture.request.trusted),
			})
			if err != nil {
				t.Fatalf("submissionauth.VerifyCompletion(%v) error = %v, want nil", offering, err)
			}
			payload, err := verified.Payload()
			if err != nil {
				t.Fatalf("VerifiedCompletion.Payload(%v) error = %v, want nil", offering, err)
			}
			if payload != fixture.completionDocument.Payload {
				t.Fatalf("VerifiedCompletion.Payload(%v) = %+v, want exact %+v",
					offering, payload, fixture.completionDocument.Payload)
			}
		})
	}
	if admitted < 3 {
		t.Fatalf("admitted offerings = %d, want at least the shipped set", admitted)
	}
}

// TestCredentialedCompletionRefusesCrossInstallationAndAgreementSubstitution
// attacks every independently valid seam. In particular, equal build
// identities do not permit one installation's request to be completed under
// another installation's certificate.
func TestCredentialedCompletionLayerTriadRefusesCrossInstallationAndAgreementSubstitution(t *testing.T) {
	t.Parallel()

	base := newAuthCompletionFixture(t, authCompletionFixtureRequest{})
	other := newAuthCompletionFixture(t, authCompletionFixtureRequest{
		authorityByte: 0x61, deviceByte: 0x62, nonceByte: 0x63,
	})
	otherCompletionSigner := base.credentialed
	otherCompletionSigner.Completion.Attestation.Signer = other.request.certificate.Body.DeviceKey
	otherNonceCompletion := base.credentialed
	otherNonceCompletion.Completion.Payload.Nonce = other.request.request.Payload.Nonce
	otherNonceAttestation, err := attest.Sign(attest.SignRequest[submission.SigningDomain]{
		Body: otherNonceCompletion.Completion.Payload, Signer: base.request.device,
	})
	if err != nil {
		t.Fatalf("attest.Sign(completion with other nonce) error = %v, want nil", err)
	}
	otherNonceCompletion.Completion.Attestation = otherNonceAttestation
	cases := []struct {
		want   error
		mutate func(*CompletionVerification)
		name   string
	}{
		{name: "completion absent", mutate: func(value *CompletionVerification) {
			value.Document = CompletionDocument{}
		}, want: core.ErrControlPlaneContract},
		{name: "original verified request absent", mutate: func(value *CompletionVerification) {
			value.Request = Verified{}
		}, want: core.ErrControlPlaneContract},
		{name: "grant absent", mutate: func(value *CompletionVerification) {
			value.Grant = submission.GrantDocument{}
		}, want: core.ErrControlPlaneContract},
		{name: "authority trust absent", mutate: func(value *CompletionVerification) {
			value.Server = controlplane.Server{}
		}, want: core.ErrControlPlaneContract},
		{name: "other installation certificate", mutate: func(value *CompletionVerification) {
			value.Document.Certificate = other.request.certificate
		}, want: core.ErrControlPlaneResponseBinding},
		{name: "other installation request", mutate: func(value *CompletionVerification) {
			value.Request = other.verifiedRequest
		}, want: core.ErrControlPlaneResponseBinding},
		{name: "other request grant", mutate: func(value *CompletionVerification) {
			value.Grant = other.grant
		}, want: core.ErrControlPlaneResponseBinding},
		{name: "other device completion with current certificate", mutate: func(value *CompletionVerification) {
			value.Document.Completion = other.completionDocument
		}, want: core.ErrControlPlaneResponseBinding},
		{name: "validly signed completion names another request nonce", mutate: func(value *CompletionVerification) {
			value.Document = otherNonceCompletion
		}, want: core.ErrControlPlaneResponseBinding},
		{name: "other authority trust", mutate: func(value *CompletionVerification) {
			value.Server = submissionAuthServer(t, other.request.trusted)
		}, want: core.ErrAttestVerification},
		{name: "other device named by completion envelope", mutate: func(value *CompletionVerification) {
			value.Document = otherCompletionSigner
		}, want: core.ErrAttestVerification},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			verification := CompletionVerification{
				Document: base.credentialed, Request: base.verifiedRequest,
				Grant: base.grant, GrantKeys: base.request.trusted,
				Server: submissionAuthServer(t, base.request.trusted),
			}
			tc.mutate(&verification)
			got, err := VerifyCompletion(verification)
			if !errors.Is(err, tc.want) || got != (VerifiedCompletion{}) {
				t.Fatalf("VerifyCompletion(%s) = (%v, %v), want zero and errors.Is %v",
					tc.name, got, err, tc.want)
			}
		})
	}
}

func TestCredentialedCompletionLayerTriadZeroValuesNeverAcquireCustodyProof(t *testing.T) {
	t.Parallel()

	verified, err := VerifyCompletion(CompletionVerification{})
	if !errors.Is(err, core.ErrControlPlaneContract) || verified != (VerifiedCompletion{}) {
		t.Fatalf("VerifyCompletion(zero) = (%v, %v), want zero and errors.Is %v",
			verified, err, core.ErrControlPlaneContract)
	}
	payload, err := (VerifiedCompletion{}).Payload()
	if !errors.Is(err, core.ErrControlPlaneContract) || payload != (submission.CompletionPayload{}) {
		t.Fatalf("VerifiedCompletion{}.Payload() = (%v, %v), want zero and errors.Is %v",
			payload, err, core.ErrControlPlaneContract)
	}
}

// TestCredentialedCompletionJSONIsStrictBoundedAndPreserving attacks the
// public receiver at framing, shape, duplication, truncation, and exact byte
// ceilings while proving every refusal preserves the previous valid value.
func TestCredentialedCompletionJSONLayerTriadIsStrictBoundedAndPreserving(t *testing.T) {
	t.Parallel()

	fixture := newAuthCompletionFixture(t, authCompletionFixtureRequest{})
	encoded, err := fixture.credentialed.MarshalJSON()
	if err != nil {
		t.Fatalf("CompletionDocument.MarshalJSON() error = %v, want nil", err)
	}
	reordered, err := json.Marshal(struct {
		Certificate controlplane.InstallationCertificateDocument `json:"certificate"`
		Completion  submission.CompletionDocument                `json:"completion"`
	}{Certificate: fixture.request.certificate, Completion: fixture.completionDocument})
	if err != nil {
		t.Fatalf("json.Marshal(reordered completion) error = %v, want nil", err)
	}
	indented := jsontext.Value(bytes.Clone(encoded))
	if err := indented.Indent(jsontext.WithIndent("  ")); err != nil {
		t.Fatalf("json.Indent(credentialed completion) error = %v, want nil", err)
	}
	unknown := append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"future":true}`)...)
	duplicateCompletion := append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"completion":null}`)...)
	duplicateCertificate := append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"certificate":null}`)...)
	valid := []struct {
		name string
		data []byte
	}{
		{name: "canonical", data: encoded},
		{name: "reordered members", data: reordered},
		{name: "indented", data: []byte(indented)},
		{name: "leading space", data: append([]byte(" "), encoded...)},
		{name: "trailing newline", data: append(bytes.Clone(encoded), '\n')},
		{name: "carriage return framing", data: append(append([]byte("\r"), encoded...), '\r')},
		{name: "mixed outer whitespace", data: append(append([]byte("\t\r\n"), encoded...), ' ', '\t')},
		{name: "half ceiling", data: authLeftPadJSON(encoded, CompletionDocumentJSONMaximumBytes/2)},
		{name: "one below ceiling", data: authLeftPadJSON(encoded, CompletionDocumentJSONMaximumBytes-1)},
		{name: "exact ceiling", data: authLeftPadJSON(encoded, CompletionDocumentJSONMaximumBytes)},
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var receiver CompletionDocument
			if err := receiver.UnmarshalJSON(tc.data); err != nil {
				t.Fatalf("CompletionDocument.UnmarshalJSON(%s) error = %v, want nil", tc.name, err)
			}
			if receiver != fixture.credentialed {
				t.Fatalf("CompletionDocument.UnmarshalJSON(%s) = %+v, want exact %+v",
					tc.name, receiver, fixture.credentialed)
			}
		})
	}
	invalid := []struct {
		name string
		data []byte
	}{
		{name: "empty input"},
		{name: "whitespace only", data: []byte(" \t\r\n")},
		{name: "null root", data: []byte(`null`)},
		{name: "boolean root", data: []byte(`true`)},
		{name: "scalar root", data: []byte(`1`)},
		{name: "string root", data: []byte(`"completion"`)},
		{name: "array root", data: []byte(`[]`)},
		{name: "empty object", data: []byte(`{}`)},
		{name: "unknown member", data: unknown},
		{name: "duplicate completion", data: duplicateCompletion},
		{name: "duplicate certificate", data: duplicateCertificate},
		{name: "missing completion", data: []byte(`{"certificate":null}`)},
		{name: "missing certificate", data: []byte(`{"completion":null}`)},
		{name: "completion wrong type", data: []byte(`{"completion":true,"certificate":null}`)},
		{name: "certificate wrong type", data: []byte(`{"completion":null,"certificate":true}`)},
		{name: "truncated opening", data: []byte(`{`)},
		{name: "truncated array", data: []byte(`[`)},
		{name: "truncated canonical", data: encoded[:len(encoded)-1]},
		{name: "half truncated canonical", data: encoded[:len(encoded)/2]},
		{name: "two documents", data: append(bytes.Clone(encoded), encoded...)},
		{name: "trailing scalar", data: append(bytes.Clone(encoded), []byte(` 0`)...)},
		{name: "one above ceiling", data: authLeftPadJSON(encoded, CompletionDocumentJSONMaximumBytes+1)},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			receiver := fixture.credentialed
			if err := receiver.UnmarshalJSON(tc.data); !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("CompletionDocument.UnmarshalJSON(%s) error = %v, want errors.Is %v",
					tc.name, err, core.ErrJSONContract)
			}
			if receiver != fixture.credentialed {
				t.Fatalf("CompletionDocument.UnmarshalJSON(%s) mutated receiver = %+v, want preserved %+v",
					tc.name, receiver, fixture.credentialed)
			}
		})
	}
	var receiver *CompletionDocument
	if err := receiver.UnmarshalJSON(encoded); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil CompletionDocument.UnmarshalJSON() error = %v, want errors.Is %v", err, core.ErrJSONContract)
	}
}

func newAuthCompletionFixture(t testing.TB, request authCompletionFixtureRequest) authCompletionFixture {
	t.Helper()

	base := newAuthFixture(t, authFixtureRequest{
		offering: request.offering, authorityByte: request.authorityByte,
		deviceByte: request.deviceByte, nonceByte: request.nonceByte,
	})
	if request.generation == 0 {
		request.generation = 7
	}
	verifiedRequest, err := Verify(Verification{Document: base.document, Server: submissionAuthServer(t, base.trusted)})
	if err != nil {
		t.Fatalf("submissionauth.Verify() error = %v, want nil", err)
	}
	operationPolicy, err := controlwire.ControlExchangeOperationPolicy()
	if err != nil {
		t.Fatalf("controlwire.ControlExchangeOperationPolicy() error = %v, want nil", err)
	}
	advance, err := operationPolicy.OperationTimeout.Multiply(
		authCompletionFutureHorizonSeconds / controlwire.ExchangeOperationTimeoutSeconds,
	)
	if err != nil {
		t.Fatalf("control exchange timeout horizon multiplication error = %v, want nil", err)
	}
	issuedAt, err := base.certificate.Body.IssuedAt.Add(advance)
	if err != nil {
		t.Fatalf("certificate issued-at plus fixture horizon error = %v, want nil", err)
	}
	expiresAt, err := issuedAt.Add(operationPolicy.OperationTimeout)
	if err != nil {
		t.Fatalf("certificate issued-at plus operation timeout error = %v, want nil", err)
	}
	retainUntil, err := expiresAt.Add(operationPolicy.OperationTimeout)
	if err != nil {
		t.Fatalf("grant expiry plus retention interval error = %v, want nil", err)
	}
	address := core.SchemeHTTPS + "://" + core.GoogleCloudStorageHost +
		authCompletionObjectPath + authCompletionQuery
	signedURL, err := objectstore.ParseSignedURL(address)
	if err != nil {
		t.Fatalf("objectstore.ParseSignedURL() error = %v, want nil", err)
	}
	headers, err := objectstore.NewSignedHeaders(nil)
	if err != nil {
		t.Fatalf("objectstore.NewSignedHeaders() error = %v, want nil", err)
	}
	capability, err := objectstore.NewUploadCapabilityProjection(
		objectstore.ProviderGoogleCloudStorage,
		objectstore.UploadTarget{URL: signedURL, Headers: headers, ExpiresAt: expiresAt},
	)
	if err != nil {
		t.Fatalf("objectstore.NewUploadCapabilityProjection() error = %v, want nil", err)
	}
	commitment, err := capability.Commitment()
	if err != nil {
		t.Fatalf("UploadCapabilityProjection.Commitment() error = %v, want nil", err)
	}
	requestCommitment, err := submission.CommitRequest(base.request.Payload)
	if err != nil {
		t.Fatalf("submission.CommitRequest() error = %v, want nil", err)
	}
	grantPayload := submission.GrantPayload{
		Request: requestCommitment, Authorization: authAuthorityNonce(t, 0x71),
		Capability: commitment, IssuedAt: issuedAt,
		ExpiresAt: expiresAt, RetainUntil: retainUntil,
	}
	grantProjection, err := submission.IssueGrant(submission.GrantIssuance{
		Signer: base.authority, Capability: capability, Payload: grantPayload,
	})
	if err != nil {
		t.Fatalf("submission.IssueGrant() error = %v, want nil", err)
	}
	grant := receiveAuthGrant(t, grantProjection)
	verifiedGrant, err := submission.VerifyGrant(submission.GrantExpectation{
		Request: base.request.Payload, Document: grant, TrustedKeys: base.trusted,
		ObservedAt: issuedAt,
	})
	if err != nil {
		t.Fatalf("submission.VerifyGrant() error = %v, want nil", err)
	}
	transfer := authCompletionUpload(t, authCompletionUploadRequest{
		grant: verifiedGrant, request: base.request.Payload, generation: request.generation,
	})
	projection, err := submission.IssueCompletion(submission.CompletionIssuance{
		Signer: base.device, Request: base.request.Payload, Grant: verifiedGrant, Transfer: transfer,
	})
	if err != nil {
		t.Fatalf("submission.IssueCompletion() error = %v, want nil", err)
	}
	completion := receiveAuthCompletion(t, projection)
	credentialed, err := AssembleCompletion(CompletionAssembly{
		Completion: completion, Certificate: base.certificate,
	})
	if err != nil {
		t.Fatalf("submissionauth.AssembleCompletion() error = %v, want nil", err)
	}
	return authCompletionFixture{
		request: base, verifiedRequest: verifiedRequest, grant: grant, grantProjection: grantProjection,
		credentialed: credentialed, completionDocument: completion, completionProjection: projection,
	}
}

func authAuthorityNonce(t testing.TB, marker byte) controlwire.AuthorityNonce {
	t.Helper()

	raw := [core.SHA256DigestBytes]byte{}
	for index := range raw {
		raw[index] = marker
	}
	nonce, err := controlwire.NewAuthorityNonce(raw)
	if err != nil {
		t.Fatalf("controlwire.NewAuthorityNonce() error = %v, want nil", err)
	}
	return nonce
}

func receiveAuthGrant(t testing.TB, projection submission.GrantProjection) submission.GrantDocument {
	t.Helper()

	encoded, err := projection.MarshalJSON()
	if err != nil {
		t.Fatalf("GrantProjection.MarshalJSON() error = %v, want nil", err)
	}
	var document submission.GrantDocument
	if err := json.Unmarshal(encoded, &document); err != nil {
		t.Fatalf("json.Unmarshal(GrantDocument) error = %v, want nil", err)
	}
	return document
}

func receiveAuthCompletion(
	t testing.TB,
	projection submission.CompletionProjection,
) submission.CompletionDocument {
	t.Helper()

	encoded, err := projection.MarshalJSON()
	if err != nil {
		t.Fatalf("CompletionProjection.MarshalJSON() error = %v, want nil", err)
	}
	var document submission.CompletionDocument
	if err := json.Unmarshal(encoded, &document); err != nil {
		t.Fatalf("json.Unmarshal(CompletionDocument) error = %v, want nil", err)
	}
	return document
}

type authCompletionUploadRequest struct {
	request    submission.RequestPayload
	grant      submission.VerifiedGrant
	generation int64
}

func authCompletionUpload(t testing.TB, request authCompletionUploadRequest) objectstore.Transfer {
	t.Helper()

	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, incoming *http.Request) {
		got, err := io.ReadAll(io.LimitReader(incoming.Body, int64(len(authCompletionContent))+1))
		if err != nil {
			t.Errorf("provider body read error = %v, want nil", err)
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		if !bytes.Equal(got, authCompletionContent) {
			t.Errorf("provider body = %q, want exact fixture", got)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		writer.Header().Set("x-goog-generation", strconv.FormatInt(request.generation, 10))
		writer.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	transport := server.Client().Transport.(*http.Transport).Clone()
	transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	transport.TLSClientConfig.ServerName = "example.com"
	serverAddress := server.Listener.Addr().String()
	dialer := &net.Dialer{}
	transport.DialContext = func(ctx context.Context, network string, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, serverAddress)
	}
	t.Cleanup(transport.CloseIdleConnections)
	exchangeClient, err := exchange.NewClient(&http.Client{Transport: transport})
	if err != nil {
		t.Fatalf("exchange.NewClient() error = %v, want nil", err)
	}
	store, err := objectstore.NewClient(exchangeClient)
	if err != nil {
		t.Fatalf("objectstore.NewClient() error = %v, want nil", err)
	}
	capability, err := request.grant.Capability()
	if err != nil {
		t.Fatalf("VerifiedGrant.Capability() error = %v, want nil", err)
	}
	transfer, err := objectstore.Upload(context.Background(), store, objectstore.UploadCapabilityRequest{
		Source: bytes.NewReader(authCompletionContent), ContentType: request.request.Declaration.ContentType,
		Capability: capability, Integrity: request.request.Declaration.Integrity(),
		Policy: authCompletionPolicy(t),
	})
	if err != nil {
		t.Fatalf("objectstore.Upload() error = %v, want nil", err)
	}
	return transfer
}

func authCompletionPolicy(t testing.TB) objectstore.Policy {
	t.Helper()

	operation, err := controlwire.ControlExchangeOperationPolicy()
	if err != nil {
		t.Fatalf("controlwire.ControlExchangeOperationPolicy() error = %v, want nil", err)
	}
	errorLimit, err := core.NewByteCount(4 << 10)
	if err != nil {
		t.Fatalf("core.NewByteCount(error limit) error = %v, want nil", err)
	}
	policy := objectstore.Policy{
		OperationTimeout: operation.OperationTimeout,
		AttemptTimeout:   operation.AttemptTimeout, ErrorBodyLimit: errorLimit,
	}
	if err := policy.Validate(); err != nil {
		t.Fatalf("objectstore.Policy.Validate() error = %v, want nil", err)
	}
	return policy
}
