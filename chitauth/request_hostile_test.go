package chitauth

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/chit"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlplanetest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/receipt"
)

type queryFixtureRequest struct {
	offering      core.Offering
	authorityByte byte
	deviceByte    byte
	nonceByte     byte
	selection     chit.Selection
}

type queryFixture struct {
	device   ed25519.PrivateKey
	document RequestDocument
	trusted  attest.TrustedKeys
	payload  chit.QueryPayload
}

func TestCredentialedChitQueryVerificationLayerTriadAuthenticatesAllAndSpecificForEveryOffering(t *testing.T) {
	t.Parallel()

	admitted := 0
	for value := 0; value <= 255; value++ {
		offering := core.Offering(value)
		if !offering.IsValid() {
			continue
		}
		admitted++
		selections := []struct {
			name      string
			selection chit.Selection
		}{
			{name: "all", selection: chit.All()},
			{name: "specific", selection: querySpecificSelection(t)},
		}
		for _, selection := range selections {
			t.Run(offering.String()+"/"+selection.name, func(t *testing.T) {
				t.Parallel()

				fixture := newQueryFixture(t, queryFixtureRequest{
					offering: offering, selection: selection.selection,
					authorityByte: byte(value) + 0x20,
					deviceByte:    byte(value) + 0x40, nonceByte: byte(value) + 1,
				})
				verified, err := Verify(Verification{
					Document: fixture.document, TrustedKeys: fixture.trusted,
				})
				if err != nil {
					t.Fatalf("chitauth.Verify(%v, %s) error = %v, want nil", offering, selection.name, err)
				}
				payload, err := verified.Payload()
				if err != nil || payload != fixture.payload {
					t.Fatalf("Verified.Payload(%v, %s) = (%+v, %v), want exact %+v and nil",
						offering, selection.name, payload, err, fixture.payload)
				}
			})
		}
	}
	if admitted < 3 {
		t.Fatalf("admitted offerings = %d, want at least the shipped set", admitted)
	}
	boundaries := []struct {
		name    string
		request queryFixtureRequest
	}{
		{name: "minimum authority maximum device and minimum nonce", request: queryFixtureRequest{
			authorityByte: 1, deviceByte: 255, nonceByte: 1, selection: chit.All(),
		}},
		{name: "maximum authority minimum device and maximum nonce", request: queryFixtureRequest{
			authorityByte: 255, deviceByte: 1, nonceByte: 255, selection: querySpecificSelection(t),
		}},
		{name: "authority one below midpoint and device at midpoint", request: queryFixtureRequest{
			authorityByte: 127, deviceByte: 128, nonceByte: 127, selection: chit.All(),
		}},
		{name: "authority at midpoint and device one below midpoint", request: queryFixtureRequest{
			authorityByte: 128, deviceByte: 127, nonceByte: 128, selection: querySpecificSelection(t),
		}},
	}
	for _, tc := range boundaries {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fixture := newQueryFixture(t, tc.request)
			verified, err := Verify(Verification{Document: fixture.document, TrustedKeys: fixture.trusted})
			if err != nil {
				t.Fatalf("Verify(%s) error = %v, want nil", tc.name, err)
			}
			payload, err := verified.Payload()
			if err != nil || payload != fixture.payload {
				t.Fatalf("Verified.Payload(%s) = (%v, %v), want exact %v and nil",
					tc.name, payload, err, fixture.payload)
			}
		})
	}
}

func TestCredentialedChitQueryVerificationLayerTriadRefusesAccountDeviceAuthorityAndBuildSubstitution(t *testing.T) {
	t.Parallel()

	base := newQueryFixture(t, queryFixtureRequest{})
	otherDevice := newQueryFixture(t, queryFixtureRequest{authorityByte: 0x51, deviceByte: 0x52, nonceByte: 0x53})
	otherOffering := newQueryFixture(t, queryFixtureRequest{
		offering: core.OfferingBug, authorityByte: 0x61, deviceByte: 0x62, nonceByte: 0x63,
	})

	wrongDeviceAssembly, err := Assemble(RequestAssembly{
		Request: otherDevice.document.Request, Certificate: base.document.Certificate,
	})
	if err != nil {
		t.Fatalf("Assemble(same-build other device) error = %v, want nil before signature verification", err)
	}
	tamperedNonce := base.document
	tamperedNonce.Request.Payload.Nonce = queryNonce(t, 0x72)
	tamperedSelection := base.document
	tamperedSelection.Request.Payload.Query.Selection = querySpecificSelection(t)
	tamperedLimit := base.document
	minimumLimit, err := core.NewCatalogPageLimit(1)
	if err != nil {
		t.Fatalf("core.NewCatalogPageLimit(minimum) error = %v, want nil", err)
	}
	tamperedLimit.Request.Payload.Query.Limit = minimumLimit
	tamperedOfferingIdentity := base.document
	tamperedOfferingIdentity.Request.Payload.Query.Scope.Offering = queryOffering(t, 0x73)
	tamperedSigner := base.document
	tamperedSigner.Request.Attestation.Signer = otherDevice.document.Certificate.Body.DeviceKey
	tamperedLength := base.document
	bodyLength, err := tamperedLength.Request.Attestation.BodyLength.Uint64()
	if err != nil {
		t.Fatalf("request attestation BodyLength.Uint64() error = %v, want nil", err)
	}
	tamperedLength.Request.Attestation.BodyLength, err = core.NewByteCount(bodyLength + 1)
	if err != nil {
		t.Fatalf("core.NewByteCount(tampered length) error = %v, want nil", err)
	}
	tamperedDigest := base.document
	tamperedDigest.Request.Attestation.BodySHA256 = core.SHA256Of([]byte("other credentialed chit query"))
	cases := []struct {
		want     error
		name     string
		document RequestDocument
		trusted  attest.TrustedKeys
	}{
		{name: "zero document", trusted: base.trusted, want: core.ErrControlPlaneContract},
		{name: "zero trust", document: base.document, want: core.ErrControlPlaneContract},
		{name: "same-build other device", document: wrongDeviceAssembly, trusted: base.trusted, want: core.ErrAttestVerification},
		{name: "other authority", document: base.document, trusted: otherDevice.trusted, want: core.ErrAttestVerification},
		{name: "signed request nonce changed after issue", document: tamperedNonce, trusted: base.trusted, want: core.ErrAttestVerification},
		{name: "signed selection changed after issue", document: tamperedSelection, trusted: base.trusted, want: core.ErrAttestVerification},
		{name: "signed page limit changed after issue", document: tamperedLimit, trusted: base.trusted, want: core.ErrAttestVerification},
		{name: "signed offering identity changed after issue", document: tamperedOfferingIdentity, trusted: base.trusted, want: core.ErrAttestVerification},
		{name: "request signer changed after issue", document: tamperedSigner, trusted: base.trusted, want: core.ErrAttestVerification},
		{name: "signed body length changed after issue", document: tamperedLength, trusted: base.trusted, want: core.ErrAttestVerification},
		{name: "signed body digest changed after issue", document: tamperedDigest, trusted: base.trusted, want: core.ErrAttestVerification},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := Verify(Verification{Document: tc.document, TrustedKeys: tc.trusted})
			if !errors.Is(err, tc.want) || got != (Verified{}) {
				t.Fatalf("Verify(%s) = (%v, %v), want zero and errors.Is %v", tc.name, got, err, tc.want)
			}
		})
	}
	if document, err := Assemble(RequestAssembly{
		Request: otherOffering.document.Request, Certificate: base.document.Certificate,
	}); !errors.Is(err, core.ErrControlPlaneResponseBinding) || document != (RequestDocument{}) {
		t.Fatalf("Assemble(other build) = (%+v, %v), want zero and errors.Is %v",
			document, err, core.ErrControlPlaneResponseBinding)
	}
	wrongAccountRequest := queryDocumentWithAccount(t, base, queryAccount(t, 0x71))
	if document, err := Assemble(RequestAssembly{
		Request: wrongAccountRequest, Certificate: base.document.Certificate,
	}); !errors.Is(err, core.ErrControlPlaneResponseBinding) || document != (RequestDocument{}) {
		t.Fatalf("Assemble(other account) = (%+v, %v), want zero and errors.Is %v",
			document, err, core.ErrControlPlaneResponseBinding)
	}
}

func TestCredentialedChitQueryVerificationLayerTriadZeroValuesNeverAcquireProof(t *testing.T) {
	t.Parallel()

	verified, err := Verify(Verification{})
	if !errors.Is(err, core.ErrControlPlaneContract) || verified != (Verified{}) {
		t.Fatalf("Verify(zero) = (%v, %v), want zero and errors.Is %v",
			verified, err, core.ErrControlPlaneContract)
	}
	payload, err := (Verified{}).Payload()
	if !errors.Is(err, core.ErrControlPlaneContract) || payload != (chit.QueryPayload{}) {
		t.Fatalf("Verified{}.Payload() = (%v, %v), want zero and errors.Is %v",
			payload, err, core.ErrControlPlaneContract)
	}
}

func TestCredentialedChitQueryJSONLayerTriadIsStrictBoundedAndPreserving(t *testing.T) {
	t.Parallel()

	fixture := newQueryFixture(t, queryFixtureRequest{selection: querySpecificSelection(t)})
	encoded, err := fixture.document.MarshalJSON()
	if err != nil {
		t.Fatalf("RequestDocument.MarshalJSON() error = %v, want nil", err)
	}
	reordered, err := json.Marshal(struct {
		Request     chit.QueryDocument                           `json:"request"`
		Certificate controlplane.InstallationCertificateDocument `json:"certificate"`
	}{Request: fixture.document.Request, Certificate: fixture.document.Certificate})
	if err != nil {
		t.Fatalf("json.Marshal(reordered request) error = %v, want nil", err)
	}
	var indented bytes.Buffer
	if err := json.Indent(&indented, encoded, "", "  "); err != nil {
		t.Fatalf("json.Indent(request) error = %v, want nil", err)
	}
	unknown := append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"future":true}`)...)
	duplicateRequest := append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"request":null}`)...)
	duplicateCertificate := append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"certificate":null}`)...)
	valid := []struct {
		name string
		data []byte
	}{
		{name: "canonical", data: encoded},
		{name: "reordered", data: reordered},
		{name: "indented", data: indented.Bytes()},
		{name: "leading space", data: append([]byte(" "), encoded...)},
		{name: "trailing newline", data: append(bytes.Clone(encoded), '\n')},
		{name: "carriage return framing", data: append(append([]byte("\r"), encoded...), '\r')},
		{name: "mixed whitespace", data: append(append([]byte("\t\r\n"), encoded...), ' ', '\t')},
		{name: "one below ceiling", data: queryPadJSON(encoded, RequestDocumentJSONMaximumBytes-1)},
		{name: "exact ceiling", data: queryPadJSON(encoded, RequestDocumentJSONMaximumBytes)},
		{name: "half ceiling", data: queryPadJSON(encoded, RequestDocumentJSONMaximumBytes/2)},
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var receiver RequestDocument
			if err := receiver.UnmarshalJSON(tc.data); err != nil {
				t.Fatalf("RequestDocument.UnmarshalJSON(%s) error = %v, want nil", tc.name, err)
			}
			if receiver != fixture.document {
				t.Fatalf("RequestDocument.UnmarshalJSON(%s) = %+v, want exact %+v", tc.name, receiver, fixture.document)
			}
		})
	}
	invalid := []struct {
		name string
		data []byte
	}{
		{name: "empty"},
		{name: "whitespace only", data: []byte(" \t\r\n")},
		{name: "null", data: []byte(`null`)},
		{name: "boolean", data: []byte(`true`)},
		{name: "scalar", data: []byte(`1`)},
		{name: "string", data: []byte(`"request"`)},
		{name: "array", data: []byte(`[]`)},
		{name: "empty object", data: []byte(`{}`)},
		{name: "unknown member", data: unknown},
		{name: "duplicate request", data: duplicateRequest},
		{name: "duplicate certificate", data: duplicateCertificate},
		{name: "request wrong type", data: []byte(`{"request":true,"certificate":null}`)},
		{name: "certificate wrong type", data: []byte(`{"request":null,"certificate":true}`)},
		{name: "missing request", data: []byte(`{"certificate":null}`)},
		{name: "missing certificate", data: []byte(`{"request":null}`)},
		{name: "open object", data: []byte(`{`)},
		{name: "open array", data: []byte(`[`)},
		{name: "truncated", data: encoded[:len(encoded)-1]},
		{name: "half truncated", data: encoded[:len(encoded)/2]},
		{name: "two documents", data: append(bytes.Clone(encoded), encoded...)},
		{name: "trailing scalar", data: append(bytes.Clone(encoded), []byte(` 0`)...)},
		{name: "one above ceiling", data: queryPadJSON(encoded, RequestDocumentJSONMaximumBytes+1)},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			receiver := fixture.document
			if err := receiver.UnmarshalJSON(tc.data); !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("RequestDocument.UnmarshalJSON(%s) error = %v, want errors.Is %v", tc.name, err, core.ErrJSONContract)
			}
			if receiver != fixture.document {
				t.Fatalf("RequestDocument.UnmarshalJSON(%s) mutated receiver = %+v, want preserved %+v",
					tc.name, receiver, fixture.document)
			}
		})
	}
	var receiver *RequestDocument
	if err := receiver.UnmarshalJSON(encoded); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil RequestDocument.UnmarshalJSON() error = %v, want errors.Is %v", err, core.ErrJSONContract)
	}
}

func newQueryFixture(t testing.TB, request queryFixtureRequest) queryFixture {
	t.Helper()

	if request.offering == core.OfferingUnknown {
		request.offering = core.OfferingWitness
	}
	if request.authorityByte == 0 {
		request.authorityByte = 0x21
	}
	if request.deviceByte == 0 {
		request.deviceByte = 0x31
	}
	if request.nonceByte == 0 {
		request.nonceByte = 0x41
	}
	if request.selection == (chit.Selection{}) {
		request.selection = chit.All()
	}
	installation, err := controlplanetest.IssueInstallation(controlplanetest.InstallationRequest{
		AuthoritySeed: querySeed(request.authorityByte), DeviceSeed: querySeed(request.deviceByte),
		Offering: request.offering,
	})
	if err != nil {
		t.Fatalf("controlplanetest.IssueInstallation() error = %v, want nil", err)
	}
	query, err := chit.NewQuery(chit.QueryRequest{
		Scope: receipt.Scope{
			Account: installation.Certificate.Body.Account, Offering: queryOffering(t, byte(request.offering)+1),
		},
		Selection: request.selection, Position: chit.Start(), PageSize: core.CatalogPageMaximumEntries,
	})
	if err != nil {
		t.Fatalf("chit.NewQuery() error = %v, want nil", err)
	}
	payload := chit.QueryPayload{
		Query: query, Build: installation.Build, Nonce: queryNonce(t, request.nonceByte),
		Revision: controlwire.Revision2026V1,
	}
	signed, err := chit.IssueQuery(chit.QueryIssuance{Signer: installation.DevicePrivate, Payload: payload})
	if err != nil {
		t.Fatalf("chit.IssueQuery() error = %v, want nil", err)
	}
	document, err := Assemble(RequestAssembly{Request: signed, Certificate: installation.Certificate})
	if err != nil {
		t.Fatalf("chitauth.Assemble() error = %v, want nil", err)
	}
	trusted, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{
		Keys: []core.Ed25519PublicKey{installation.AuthorityPublic},
	})
	if err != nil {
		t.Fatalf("attest.NewTrustedKeys() error = %v, want nil", err)
	}
	return queryFixture{
		document: document, payload: payload, trusted: trusted, device: installation.DevicePrivate,
	}
}

func queryDocumentWithAccount(t testing.TB, fixture queryFixture, account receipt.AccountIdentity) chit.QueryDocument {
	t.Helper()

	payload := fixture.payload
	payload.Query.Scope.Account = account
	document, err := chit.IssueQuery(chit.QueryIssuance{Signer: fixture.device, Payload: payload})
	if err != nil {
		t.Fatalf("chit.IssueQuery(other account) error = %v, want nil", err)
	}
	return document
}

func querySpecificSelection(t testing.TB) chit.Selection {
	t.Helper()

	identity, err := chit.ParseChitID("00000000-0007-7000-8000-000000000007")
	if err != nil {
		t.Fatalf("chit.ParseChitID() error = %v, want nil", err)
	}
	selection, err := chit.Specific(identity)
	if err != nil {
		t.Fatalf("chit.Specific() error = %v, want nil", err)
	}
	return selection
}

func querySeed(marker byte) [ed25519.SeedSize]byte {
	seed := [ed25519.SeedSize]byte{}
	for index := range seed {
		seed[index] = marker
	}
	return seed
}

func queryNonce(t testing.TB, marker byte) controlwire.RequestNonce {
	t.Helper()

	raw := [controlwire.NonceBytes]byte{}
	for index := range raw {
		raw[index] = marker
	}
	nonce, err := controlwire.NewRequestNonce(raw)
	if err != nil {
		t.Fatalf("controlwire.NewRequestNonce() error = %v, want nil", err)
	}
	return nonce
}

func queryOffering(t testing.TB, marker byte) receipt.OfferingIdentity {
	t.Helper()

	raw := [receipt.LifecycleIdentityBytes]byte{}
	for index := range raw {
		raw[index] = marker
	}
	identity, err := receipt.NewOfferingIdentity(raw)
	if err != nil {
		t.Fatalf("receipt.NewOfferingIdentity() error = %v, want nil", err)
	}
	return identity
}

func queryAccount(t testing.TB, marker byte) receipt.AccountIdentity {
	t.Helper()

	raw := [receipt.LifecycleIdentityBytes]byte{}
	for index := range raw {
		raw[index] = marker
	}
	identity, err := receipt.NewAccountIdentity(raw)
	if err != nil {
		t.Fatalf("receipt.NewAccountIdentity() error = %v, want nil", err)
	}
	return identity
}

func queryPadJSON(encoded []byte, length int) []byte {
	if length < len(encoded) {
		return nil
	}
	padded := make([]byte, length)
	for index := 0; index < length-len(encoded); index++ {
		padded[index] = ' '
	}
	copy(padded[length-len(encoded):], encoded)
	return padded
}
