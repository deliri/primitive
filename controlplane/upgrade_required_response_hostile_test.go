package controlplane_test

import (
	"bytes"
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
)

type upgradeRequiredResponseFixture struct {
	canonical []byte
	header    controlplane.ResponseHeader
	base      authenticatedResponseFixture
}

type upgradeRequiredRepresentation struct {
	name      string
	document  []byte
	wantValid bool
}

type upgradeRequiredWireParts struct {
	header      []byte
	attestation []byte
}

func TestUpgradeRequiredResponseAcceptsTenBoundedBodylessRepresentations(t *testing.T) {
	t.Parallel()

	fixture := upgradeRequiredResponseForTest(t, 91)
	parts := upgradeRequiredPartsForTest(t, fixture)
	cases := []upgradeRequiredRepresentation{
		{name: "canonical authority projection", document: fixture.canonical, wantValid: true},
		{name: "one leading space", document: append([]byte{' '}, fixture.canonical...), wantValid: true},
		{name: "one trailing space", document: append(bytes.Clone(fixture.canonical), ' '), wantValid: true},
		{name: "one leading newline", document: append([]byte{'\n'}, fixture.canonical...), wantValid: true},
		{name: "one trailing carriage return", document: append(bytes.Clone(fixture.canonical), '\r'), wantValid: true},
		{name: "mixed outer JSON whitespace", document: append(append([]byte{'\t', '\n'}, fixture.canonical...), '\r', ' '), wantValid: true},
		{name: "attestation before header", document: parts.object(parts.attestation, parts.header), wantValid: true},
		{name: "one byte below shared document ceiling", document: padResponseToSize(t, fixture.canonical, core.JSONDocumentMaximumBytes-1), wantValid: true},
		{name: "exact shared document ceiling", document: padResponseToSize(t, fixture.canonical, core.JSONDocumentMaximumBytes), wantValid: true},
		{name: "four leading JSON whitespace bytes", document: append([]byte{' ', '\t', '\r', '\n'}, fixture.canonical...), wantValid: true},
	}
	if len(cases) != 10 {
		t.Fatalf("upgrade refusal valid inventory = %d, want exactly 10", len(cases))
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			document := decodeUpgradeRequiredResponse(t, tc.document)
			proveUpgradeRequiredResponse(t, fixture, document)
		})
	}
}

func TestUpgradeRequiredIssuanceRejectsEveryIndependentContractFailure(t *testing.T) {
	t.Parallel()

	fixture := upgradeRequiredResponseForTest(t, 96)
	base := controlplane.UpgradeRequiredIssuance{
		Server: fixture.base.server,
		Signer: fixture.base.signer, Header: fixture.header,
		Assessment: upgradeRequiredProtocolAssessment(t, fixture.header),
	}
	cases := []struct {
		want   error
		mutate func(*controlplane.UpgradeRequiredIssuance)
		name   string
	}{
		{name: "zero server capability", mutate: func(value *controlplane.UpgradeRequiredIssuance) { value.Server = controlplane.Server{} }, want: core.ErrControlPlaneContract},
		{name: "nil signer", mutate: func(value *controlplane.UpgradeRequiredIssuance) { value.Signer = nil }, want: core.ErrAttestContract},
		{name: "zero header", mutate: func(value *controlplane.UpgradeRequiredIssuance) { value.Header = controlplane.ResponseHeader{} }, want: core.ErrControlPlaneResponseHeader},
		{name: "zero revision", mutate: func(value *controlplane.UpgradeRequiredIssuance) { value.Header.Revision = controlwire.RevisionUnknown }, want: core.ErrControlPlaneResponseHeader},
		{name: "future revision", mutate: func(value *controlplane.UpgradeRequiredIssuance) {
			value.Header.Revision = controlwire.Revision(math.MaxUint8)
		}, want: core.ErrControlPlaneResponseHeader},
		{name: "zero family", mutate: func(value *controlplane.UpgradeRequiredIssuance) {
			value.Header.Family = controlwire.RouteFamilyUnknown
		}, want: core.ErrControlPlaneResponseHeader},
		{name: "future family", mutate: func(value *controlplane.UpgradeRequiredIssuance) {
			value.Header.Family = controlwire.RouteFamily(math.MaxUint8)
		}, want: core.ErrControlPlaneResponseHeader},
		{name: "active status", mutate: func(value *controlplane.UpgradeRequiredIssuance) {
			value.Header.Status = controlplane.ProductStatusActive
		}, want: core.ErrControlPlaneDecisionConsistency},
		{name: "zero assessment", mutate: func(value *controlplane.UpgradeRequiredIssuance) { value.Assessment = controlwire.ProtocolAssessment{} }, want: core.ErrControlWireProtocolSupport},
		{name: "accepted assessment", mutate: func(value *controlplane.UpgradeRequiredIssuance) {
			value.Assessment.Outcome = controlwire.ProtocolSupportOutcomeAccepted
		}, want: core.ErrControlPlaneResponseBinding},
		{name: "unknown assessment outcome", mutate: func(value *controlplane.UpgradeRequiredIssuance) {
			value.Assessment.Outcome = controlwire.ProtocolSupportOutcomeUnknown
		}, want: core.ErrControlWireProtocolSupport},
		{name: "assessment names another family", mutate: func(value *controlplane.UpgradeRequiredIssuance) {
			value.Assessment.Capability.Family = controlwire.RouteFamilyCheckIns
		}, want: core.ErrControlPlaneResponseBinding},
		{name: "assessment names future revision", mutate: func(value *controlplane.UpgradeRequiredIssuance) {
			value.Assessment.Capability.Revision = controlwire.Revision(math.MaxUint8)
		}, want: core.ErrControlWireProtocolSupport},
	}
	if len(cases) != 13 {
		t.Fatalf("upgrade issuance rejection inventory = %d, want exactly 13", len(cases))
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			issuance := base
			tc.mutate(&issuance)
			validationErr := issuance.Validate()
			projection, issueErr := controlplane.IssueUpgradeRequiredResponse[controlplane.RegistrationDocument](issuance)
			if !errors.Is(validationErr, core.ErrControlPlaneResponseDocument) || !errors.Is(validationErr, tc.want) ||
				!errors.Is(issueErr, core.ErrControlPlaneResponseDocument) || !errors.Is(issueErr, tc.want) || projection.Validate() == nil {
				t.Fatalf("upgrade issuance refusal = (validate %v, issue %v, projection validate %v), want %v/%v and invalid zero", validationErr, issueErr, projection.Validate(), core.ErrControlPlaneResponseDocument, tc.want)
			}
		})
	}
}

func TestUpgradeRequiredResponseRejectsTwentyFiveHostileRepresentationsWithoutMutation(t *testing.T) {
	t.Parallel()

	fixture := upgradeRequiredResponseForTest(t, 92)
	foreign := upgradeRequiredResponseForTest(t, 93)
	foreignHeader := foreign.header
	foreignHeader.RequestNonce = otherRequestNonce(t)
	foreign = upgradeRequiredResponseWithHeader(t, foreign.base, foreignHeader)
	parts := upgradeRequiredPartsForTest(t, fixture)
	foreignParts := upgradeRequiredPartsForTest(t, foreign)
	cases := []upgradeRequiredRepresentation{
		{name: "nil input", document: nil},
		{name: "empty input", document: []byte{}},
		{name: "whitespace input", document: []byte{' ', '\n'}},
		{name: "null input", document: []byte("null")},
		{name: "array input", document: []byte("[]")},
		{name: "boolean input", document: []byte("true")},
		{name: "numeric input", document: []byte("0")},
		{name: "string input", document: []byte(`"upgrade"`)},
		{name: "empty object", document: []byte("{}")},
		{name: "opening object only", document: []byte{'{'}},
		{name: "first byte truncation", document: bytes.Clone(fixture.canonical[:1])},
		{name: "midpoint truncation", document: bytes.Clone(fixture.canonical[:len(fixture.canonical)/2])},
		{name: "one byte truncation", document: bytes.Clone(fixture.canonical[:len(fixture.canonical)-1])},
		{name: "trailing scalar", document: append(bytes.Clone(fixture.canonical), '0')},
		{name: "one above shared document ceiling", document: padResponseToSize(t, fixture.canonical, core.JSONDocumentMaximumBytes+1)},
		{name: "unknown member", document: parts.object(parts.header, parts.attestation, []byte(`"future":true`))},
		{name: "duplicate header", document: parts.object(parts.header, parts.header, parts.attestation)},
		{name: "duplicate attestation", document: parts.object(parts.header, parts.attestation, parts.attestation)},
		{name: "missing header", document: parts.object(parts.attestation)},
		{name: "missing attestation", document: parts.object(parts.header)},
		{name: "null body is forbidden", document: parts.object(parts.header, []byte(`"body":null`), parts.attestation)},
		{name: "empty object body is forbidden", document: parts.object(parts.header, []byte(`"body":{}`), parts.attestation)},
		{name: "valid product body is forbidden", document: parts.object(parts.header, responsePartsForTest(t, fixture.base).body, parts.attestation)},
		{name: "foreign signed header with local attestation", document: parts.object(foreignParts.header, parts.attestation)},
		{name: "foreign valid attestation with local header", document: parts.object(parts.header, foreignParts.attestation)},
	}
	if len(cases) != 25 {
		t.Fatalf("upgrade refusal hostile inventory = %d, want exactly 25", len(cases))
	}
	preserved := decodeUpgradeRequiredResponse(t, fixture.canonical)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			candidate := preserved
			err := candidate.UnmarshalJSON(tc.document)
			if !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrControlPlaneResponseDocument) {
				t.Fatalf("ResponseDocument.UnmarshalJSON(hostile refusal) error = %v, want %v/%v", err, core.ErrJSONContract, core.ErrControlPlaneResponseDocument)
			}
			if candidate.Validate() != nil {
				t.Fatalf("rejected refusal changed populated receiver")
			}
			var zero controlplane.ResponseDocument[controlplane.RegistrationDocument, *controlplane.RegistrationDocument]
			zeroErr := zero.UnmarshalJSON(tc.document)
			if !errors.Is(zeroErr, core.ErrJSONContract) || !errors.Is(zeroErr, core.ErrControlPlaneResponseDocument) ||
				!errors.Is(zero.Validate(), core.ErrControlPlaneResponseDocument) {
				t.Fatalf("fresh rejected refusal = (decode %v, validate %v), want typed refusal and invalid zero", zeroErr, zero.Validate())
			}
		})
	}
}

func FuzzUpgradeRequiredResponseSemanticClosure(f *testing.F) {
	fixture := upgradeRequiredResponseForTest(f, 94)
	foreign := upgradeRequiredResponseForTest(f, 95)
	foreignHeader := foreign.header
	foreignHeader.RequestNonce = otherRequestNonce(f)
	foreign = upgradeRequiredResponseWithHeader(f, foreign.base, foreignHeader)
	parts := upgradeRequiredPartsForTest(f, fixture)
	foreignParts := upgradeRequiredPartsForTest(f, foreign)
	for _, data := range [][]byte{
		fixture.canonical,
		parts.object(parts.attestation, parts.header),
		foreign.canonical,
		parts.object(foreignParts.header, parts.attestation),
		parts.object(parts.header, foreignParts.attestation),
		parts.object(parts.header, []byte(`"body":null`), parts.attestation),
		nil, {}, []byte("null"), []byte("{}"), []byte{'{'},
		padResponseToSize(f, fixture.canonical, core.JSONDocumentMaximumBytes+1),
	} {
		f.Add(data)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		preserved := decodeUpgradeRequiredResponse(t, fixture.canonical)
		candidate := preserved
		decodeErr := candidate.UnmarshalJSON(data)
		if decodeErr != nil {
			if !errors.Is(decodeErr, core.ErrJSONContract) || !errors.Is(decodeErr, core.ErrControlPlaneResponseDocument) || candidate.Validate() != nil {
				t.Fatalf("rejected refusal = (decode %v, validate %v), want typed refusal and preserved receiver", decodeErr, candidate.Validate())
			}
			return
		}
		if candidate.Validate() != nil {
			t.Fatalf("accepted refusal Validate() error = %v, want nil", candidate.Validate())
		}
		verified, verifyErr := controlplane.VerifyResponse(controlplane.ResponseVerification[controlplane.RegistrationDocument, *controlplane.RegistrationDocument]{
			Client: fixture.base.client, Document: candidate, Expected: fixture.base.expected,
		})
		if verifyErr != nil {
			bindingRefusal := errors.Is(verifyErr, core.ErrControlPlaneResponseBinding)
			authenticationRefusal := errors.Is(verifyErr, core.ErrControlPlaneResponseDocument) &&
				errors.Is(verifyErr, core.ErrAttestVerification)
			if (!bindingRefusal && !authenticationRefusal) || verified.Validate() == nil {
				t.Fatalf("untrusted accepted refusal = (%v, %v), want invalid zero and typed binding or authentication refusal", verified, verifyErr)
			}
			return
		}
		header, headerErr := verified.Header()
		body, bodyErr := verified.Body()
		if headerErr != nil || header != fixture.header || !errors.Is(bodyErr, core.ErrControlPlaneUpgradeRequired) || body != (controlplane.RegistrationDocument{}) {
			t.Fatalf("trusted refusal facts = (header %+v/%v, body %+v/%v), want exact header, zero body, and %v", header, headerErr, body, bodyErr, core.ErrControlPlaneUpgradeRequired)
		}
		projection, issueErr := controlplane.IssueUpgradeRequiredResponse[controlplane.RegistrationDocument](controlplane.UpgradeRequiredIssuance{
			Server: fixture.base.server,
			Signer: fixture.base.signer, Header: header, Assessment: upgradeRequiredProtocolAssessment(t, header),
		})
		canonical, marshalErr := projection.MarshalJSON()
		if issueErr != nil || marshalErr != nil || !bytes.Equal(canonical, fixture.canonical) {
			t.Fatalf("trusted refusal canonical closure = (%d bytes, issue %v, marshal %v), want exact %d bytes", len(canonical), issueErr, marshalErr, len(fixture.canonical))
		}
	})
}

func upgradeRequiredResponseForTest(t testing.TB, seed byte) upgradeRequiredResponseFixture {
	t.Helper()

	base := authenticatedResponseForTest(t, seed)
	header := base.header
	header.Status = controlplane.ProductStatusUpgradeRequired
	return upgradeRequiredResponseWithHeader(t, base, header)
}

func upgradeRequiredResponseWithHeader(
	t testing.TB,
	base authenticatedResponseFixture,
	header controlplane.ResponseHeader,
) upgradeRequiredResponseFixture {
	t.Helper()

	projection, err := controlplane.IssueUpgradeRequiredResponse[controlplane.RegistrationDocument](controlplane.UpgradeRequiredIssuance{
		Server: base.server,
		Signer: base.signer, Header: header, Assessment: upgradeRequiredProtocolAssessment(t, header),
	})
	if err != nil {
		t.Fatalf("IssueUpgradeRequiredResponse() error = %v, want nil", err)
	}
	canonical, err := projection.MarshalJSON()
	if err != nil {
		t.Fatalf("upgrade refusal MarshalJSON() error = %v, want nil", err)
	}
	base.expected = expectationFor(header)
	return upgradeRequiredResponseFixture{base: base, header: header, canonical: canonical}
}

func decodeUpgradeRequiredResponse(t testing.TB, document []byte) controlplane.ResponseDocument[controlplane.RegistrationDocument, *controlplane.RegistrationDocument] {
	t.Helper()

	var decoded controlplane.ResponseDocument[controlplane.RegistrationDocument, *controlplane.RegistrationDocument]
	if err := decoded.UnmarshalJSON(document); err != nil {
		t.Fatalf("ResponseDocument.UnmarshalJSON(compiler-issued refusal) error = %v, want nil", err)
	}
	return decoded
}

func proveUpgradeRequiredResponse(
	t testing.TB,
	fixture upgradeRequiredResponseFixture,
	document controlplane.ResponseDocument[controlplane.RegistrationDocument, *controlplane.RegistrationDocument],
) {
	t.Helper()

	verified, err := controlplane.VerifyResponse(controlplane.ResponseVerification[controlplane.RegistrationDocument, *controlplane.RegistrationDocument]{
		Client: fixture.base.client, Document: document, Expected: fixture.base.expected,
	})
	if err != nil {
		t.Fatalf("VerifyResponse(authentic refusal) error = %v, want nil", err)
	}
	header, headerErr := verified.Header()
	body, bodyErr := verified.Body()
	if headerErr != nil || header != fixture.header || !errors.Is(bodyErr, core.ErrControlPlaneUpgradeRequired) || body != (controlplane.RegistrationDocument{}) {
		t.Fatalf("verified refusal = (header %+v/%v, body %+v/%v), want exact header, zero body, and %v", header, headerErr, body, bodyErr, core.ErrControlPlaneUpgradeRequired)
	}
}

func upgradeRequiredPartsForTest(t testing.TB, fixture upgradeRequiredResponseFixture) upgradeRequiredWireParts {
	t.Helper()

	headerJSON, err := fixture.header.MarshalJSON()
	if err != nil {
		t.Fatalf("ResponseHeader.MarshalJSON() error = %v, want nil", err)
	}
	members := responseCanonicalMembers(t, fixture.canonical)
	header, headerIndex := responseMemberWithValue(t, members, headerJSON)
	if len(members) != 2 || headerIndex < 0 || headerIndex > 1 {
		t.Fatalf("canonical refusal members = %d with header index %d, want two members and one header",
			len(members), headerIndex)
	}
	return upgradeRequiredWireParts{
		header: header, attestation: bytes.Clone(members[1-headerIndex]),
	}
}

func (p upgradeRequiredWireParts) object(members ...[]byte) []byte {
	total := 2
	for _, member := range members {
		total += len(member)
	}
	if len(members) > 1 {
		total += len(members) - 1
	}
	encoded := make([]byte, 0, total)
	encoded = append(encoded, '{')
	for index, member := range members {
		if index != 0 {
			encoded = append(encoded, ',')
		}
		encoded = append(encoded, member...)
	}
	return append(encoded, '}')
}
