package chitauth

import (
	"bytes"
	"crypto/ed25519"
	json "encoding/json/v2"
	"errors"
	"testing"

	"encoding/json/jsontext"
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
	payload  chit.QueryPayload
	document RequestDocument
	client   controlplane.Client
	server   controlplane.Server
}

func TestCredentialedChitQueryVerificationLayerTriadAuthenticatesOnlyTheBoundDeviceRequest(t *testing.T) {
	t.Parallel()

	t.Run("positive ten signed identity and selection boundaries expose the exact query", func(t *testing.T) {
		t.Parallel()
		proveCredentialedChitQueryVerificationAdmissions(t)
	})
	t.Run("negative signed substitutions and foreign authority facts expose no proof", func(t *testing.T) {
		t.Parallel()
		proveCredentialedChitQueryVerificationRejections(t)
	})
	t.Run("neutral zero contracts expose neither query nor proof", func(t *testing.T) {
		t.Parallel()
		proveCredentialedChitQueryVerificationNeutralState(t)
	})
}

func proveCredentialedChitQueryVerificationAdmissions(t *testing.T) {
	t.Helper()

	cases := []struct {
		name          string
		request       queryFixtureRequest
		wantOffering  core.Offering
		wantSelection chit.Selection
	}{
		{name: "minimum opaque offering with all selection", request: queryFixtureRequest{
			offering: chitAuthOffering(t, 1), authorityByte: 0x21, deviceByte: 0x41,
			nonceByte: 1, selection: chit.All(),
		}, wantOffering: chitAuthOffering(t, 1), wantSelection: chit.All()},
		{name: "minimum opaque offering with specific selection", request: queryFixtureRequest{
			offering: chitAuthOffering(t, 1), authorityByte: 0x22, deviceByte: 0x42,
			nonceByte: 2, selection: querySpecificSelection(t),
		}, wantOffering: chitAuthOffering(t, 1), wantSelection: querySpecificSelection(t)},
		{name: "midpoint opaque offering with all selection", request: queryFixtureRequest{
			offering: chitAuthOffering(t, 127), authorityByte: 0x23, deviceByte: 0x43,
			nonceByte: 3, selection: chit.All(),
		}, wantOffering: chitAuthOffering(t, 127), wantSelection: chit.All()},
		{name: "midpoint opaque offering with specific selection", request: queryFixtureRequest{
			offering: chitAuthOffering(t, 127), authorityByte: 0x24, deviceByte: 0x44,
			nonceByte: 4, selection: querySpecificSelection(t),
		}, wantOffering: chitAuthOffering(t, 127), wantSelection: querySpecificSelection(t)},
		{name: "maximum opaque offering with all selection", request: queryFixtureRequest{
			offering: chitAuthOffering(t, 255), authorityByte: 0x25, deviceByte: 0x45,
			nonceByte: 5, selection: chit.All(),
		}, wantOffering: chitAuthOffering(t, 255), wantSelection: chit.All()},
		{name: "maximum opaque offering with specific selection", request: queryFixtureRequest{
			offering: chitAuthOffering(t, 255), authorityByte: 0x26, deviceByte: 0x46,
			nonceByte: 6, selection: querySpecificSelection(t),
		}, wantOffering: chitAuthOffering(t, 255), wantSelection: querySpecificSelection(t)},
		{name: "minimum authority maximum device and minimum nonce", request: queryFixtureRequest{
			offering: chitAuthOffering(t, 2), authorityByte: 1, deviceByte: 255,
			nonceByte: 1, selection: chit.All(),
		}, wantOffering: chitAuthOffering(t, 2), wantSelection: chit.All()},
		{name: "maximum authority minimum device and maximum nonce", request: queryFixtureRequest{
			offering: chitAuthOffering(t, 2), authorityByte: 255, deviceByte: 1,
			nonceByte: 255, selection: querySpecificSelection(t),
		}, wantOffering: chitAuthOffering(t, 2), wantSelection: querySpecificSelection(t)},
		{name: "authority one below midpoint and device at midpoint", request: queryFixtureRequest{
			offering: chitAuthOffering(t, 2), authorityByte: 127, deviceByte: 128,
			nonceByte: 127, selection: chit.All(),
		}, wantOffering: chitAuthOffering(t, 2), wantSelection: chit.All()},
		{name: "authority at midpoint and device one below midpoint", request: queryFixtureRequest{
			offering: chitAuthOffering(t, 2), authorityByte: 128, deviceByte: 127,
			nonceByte: 128, selection: querySpecificSelection(t),
		}, wantOffering: chitAuthOffering(t, 2), wantSelection: querySpecificSelection(t)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fixture := newQueryFixture(t, tc.request)
			route, routeErr := fixture.document.ControlRoute()
			wantRoute, wantRouteErr := controlwire.NewRouteContract(
				tc.wantOffering, controlwire.RouteFamilyChits,
			)
			if routeErr != nil || wantRouteErr != nil || route != wantRoute ||
				fixture.document.ControlNonce() != fixture.payload.Nonce ||
				fixture.payload.Query.Selection != tc.wantSelection {
				t.Fatalf("RequestDocument control projection = (%v, %v, %v), want (%v, %v, nil)",
					route, fixture.document.ControlNonce(), routeErr, wantRoute, fixture.payload.Nonce)
			}
			verified, err := Verify(Verification{Server: fixture.server, Document: fixture.document})
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

func proveCredentialedChitQueryVerificationRejections(t *testing.T) {
	t.Helper()

	baseRequest := standardQueryFixtureRequest(t)
	base := newQueryFixture(t, baseRequest)
	otherDeviceRequest := standardQueryFixtureRequest(t)
	otherDeviceRequest.authorityByte = 0x51
	otherDeviceRequest.deviceByte = 0x52
	otherDeviceRequest.nonceByte = 0x53
	otherDevice := newQueryFixture(t, otherDeviceRequest)
	otherOfferingRequest := standardQueryFixtureRequest(t)
	otherOfferingRequest.offering = chitAuthOffering(t, 1)
	otherOfferingRequest.authorityByte = 0x61
	otherOfferingRequest.deviceByte = 0x62
	otherOfferingRequest.nonceByte = 0x63
	otherOffering := newQueryFixture(t, otherOfferingRequest)

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
	tamperedPartition := base.document
	tamperedPartition.Request.Payload.Query.Partition = queryPartition(t, 0x73)
	tamperedLimit := base.document
	minimumLimit, err := core.NewCatalogPageLimit(1)
	if err != nil {
		t.Fatalf("core.NewCatalogPageLimit(minimum) error = %v, want nil", err)
	}
	tamperedLimit.Request.Payload.Query.Limit = minimumLimit
	tamperedOfferingIdentity := base.document
	tamperedOfferingIdentity.Request.Payload.Query.Scope.Offering = queryOffering(t, chitAuthOffering(t, 1))
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
		server   controlplane.Server
	}{
		{name: "zero document", server: base.server, want: core.ErrControlPlaneContract},
		{name: "zero server capability", document: base.document, want: core.ErrControlPlaneContract},
		{name: "same-build other device", document: wrongDeviceAssembly, server: base.server, want: core.ErrAttestVerification},
		{name: "other authority", document: base.document, server: otherDevice.server, want: core.ErrAttestVerification},
		{name: "signed request nonce changed after issue", document: tamperedNonce, server: base.server, want: core.ErrAttestVerification},
		{name: "signed selection changed after issue", document: tamperedSelection, server: base.server, want: core.ErrAttestVerification},
		{name: "signed partition changed after issue", document: tamperedPartition, server: base.server, want: core.ErrAttestVerification},
		{name: "signed page limit changed after issue", document: tamperedLimit, server: base.server, want: core.ErrAttestVerification},
		{name: "signed offering identity changed after issue", document: tamperedOfferingIdentity, server: base.server, want: core.ErrControlPlaneResponseBinding},
		{name: "request signer changed after issue", document: tamperedSigner, server: base.server, want: core.ErrAttestVerification},
		{name: "signed body length changed after issue", document: tamperedLength, server: base.server, want: core.ErrAttestVerification},
		{name: "signed body digest changed after issue", document: tamperedDigest, server: base.server, want: core.ErrAttestVerification},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := Verify(Verification{Server: tc.server, Document: tc.document})
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
	wrongOfferingRequest := queryDocumentWithOffering(t, base, queryOffering(t, chitAuthOffering(t, 1)))
	if document, err := Assemble(RequestAssembly{
		Request: wrongOfferingRequest, Certificate: base.document.Certificate,
	}); !errors.Is(err, core.ErrControlPlaneResponseBinding) || document != (RequestDocument{}) {
		t.Fatalf("Assemble(other offering identity) = (%+v, %v), want zero and errors.Is %v",
			document, err, core.ErrControlPlaneResponseBinding)
	}
}

func proveCredentialedChitQueryVerificationNeutralState(t *testing.T) {
	t.Helper()

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

func TestCredentialedChitQueryJSONEnforcesTenValidTenRejectAndTwentyBoundaryCases(t *testing.T) {
	t.Parallel()

	request := standardQueryFixtureRequest(t)
	request.selection = querySpecificSelection(t)
	fixture := newQueryFixture(t, request)
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
	indented := jsontext.Value(bytes.Clone(encoded))
	if err := indented.Indent(jsontext.WithIndent("  ")); err != nil {
		t.Fatalf("json.Indent(request) error = %v, want nil", err)
	}
	unknown := append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"future":true}`)...)
	duplicateRequest := append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"request":null}`)...)
	duplicateCertificate := append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"certificate":null}`)...)
	cases := []struct {
		name         string
		data         []byte
		receiver     RequestDocument
		wantDocument RequestDocument
		wantErr      error
	}{
		{name: "valid canonical production projection", data: encoded, wantDocument: fixture.document},
		{name: "valid reordered typed members", data: reordered, wantDocument: fixture.document},
		{name: "valid indented production projection", data: []byte(indented), wantDocument: fixture.document},
		{name: "valid leading space", data: append([]byte(" "), encoded...), wantDocument: fixture.document},
		{name: "valid trailing newline", data: append(bytes.Clone(encoded), '\n'), wantDocument: fixture.document},
		{name: "valid carriage return framing", data: append(append([]byte("\r"), encoded...), '\r'), wantDocument: fixture.document},
		{name: "valid mixed whitespace", data: append(append([]byte("\t\r\n"), encoded...), ' ', '\t'), wantDocument: fixture.document},
		{name: "valid one byte below ceiling", data: queryJSONAtLength(t, encoded, RequestDocumentJSONMaximumBytes-1), wantDocument: fixture.document},
		{name: "valid exactly at ceiling", data: queryJSONAtLength(t, encoded, RequestDocumentJSONMaximumBytes), wantDocument: fixture.document},
		{name: "valid midpoint extent", data: queryJSONAtLength(t, encoded, RequestDocumentJSONMaximumBytes/2), wantDocument: fixture.document},

		{name: "reject boolean document", data: []byte(`true`), receiver: fixture.document, wantDocument: fixture.document, wantErr: core.ErrJSONContract},
		{name: "reject empty object", data: []byte(`{}`), receiver: fixture.document, wantDocument: fixture.document, wantErr: core.ErrJSONContract},
		{name: "reject unknown member", data: unknown, receiver: fixture.document, wantDocument: fixture.document, wantErr: core.ErrJSONContract},
		{name: "reject duplicate request", data: duplicateRequest, receiver: fixture.document, wantDocument: fixture.document, wantErr: core.ErrJSONContract},
		{name: "reject duplicate certificate", data: duplicateCertificate, receiver: fixture.document, wantDocument: fixture.document, wantErr: core.ErrJSONContract},
		{name: "reject request with wrong type", data: []byte(`{"request":true,"certificate":null}`), receiver: fixture.document, wantDocument: fixture.document, wantErr: core.ErrJSONContract},
		{name: "reject certificate with wrong type", data: []byte(`{"request":null,"certificate":true}`), receiver: fixture.document, wantDocument: fixture.document, wantErr: core.ErrJSONContract},
		{name: "reject missing request", data: []byte(`{"certificate":null}`), receiver: fixture.document, wantDocument: fixture.document, wantErr: core.ErrJSONContract},
		{name: "reject missing certificate", data: []byte(`{"request":null}`), receiver: fixture.document, wantDocument: fixture.document, wantErr: core.ErrJSONContract},
		{name: "reject structurally complete zero members", data: []byte(`{"request":null,"certificate":null}`), receiver: fixture.document, wantDocument: fixture.document, wantErr: core.ErrJSONContract},

		{name: "boundary empty input", wantErr: core.ErrJSONContract},
		{name: "boundary whitespace only", data: []byte(" \t\r\n"), wantErr: core.ErrJSONContract},
		{name: "boundary null", data: []byte(`null`), wantErr: core.ErrJSONContract},
		{name: "boundary scalar", data: []byte(`1`), wantErr: core.ErrJSONContract},
		{name: "boundary string", data: []byte(`"request"`), wantErr: core.ErrJSONContract},
		{name: "boundary array", data: []byte(`[]`), wantErr: core.ErrJSONContract},
		{name: "boundary open object", data: []byte(`{`), wantErr: core.ErrJSONContract},
		{name: "boundary open array", data: []byte(`[`), wantErr: core.ErrJSONContract},
		{name: "boundary one byte truncated", data: encoded[:len(encoded)-1], wantErr: core.ErrJSONContract},
		{name: "boundary half truncated", data: encoded[:len(encoded)/2], wantErr: core.ErrJSONContract},
		{name: "boundary two concatenated documents", data: append(bytes.Clone(encoded), encoded...), wantErr: core.ErrJSONContract},
		{name: "boundary trailing scalar", data: append(bytes.Clone(encoded), []byte(` 0`)...), wantErr: core.ErrJSONContract},
		{name: "boundary one byte above ceiling", data: queryJSONAtLength(t, encoded, RequestDocumentJSONMaximumBytes+1), wantErr: core.ErrJSONContract},
		{name: "boundary leading byte order mark", data: append([]byte{0xef, 0xbb, 0xbf}, encoded...), wantErr: core.ErrJSONContract},
		{name: "boundary trailing comma", data: append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,}`)...), wantErr: core.ErrJSONContract},
		{name: "boundary prefixed token", data: append([]byte(`0 `), encoded...), wantErr: core.ErrJSONContract},
		{name: "boundary suffixed object", data: append(bytes.Clone(encoded), []byte(` {}`)...), wantErr: core.ErrJSONContract},
		{name: "boundary invalid utf8", data: []byte{0xff}, wantErr: core.ErrJSONContract},
		{name: "boundary embedded null byte", data: append(bytes.Clone(encoded[:len(encoded)/2]), append([]byte{0}, encoded[len(encoded)/2:]...)...), wantErr: core.ErrJSONContract},
		{name: "boundary missing value after member", data: []byte(`{"request":`), wantErr: core.ErrJSONContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			receiver := tc.receiver
			gotErr := receiver.UnmarshalJSON(tc.data)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("RequestDocument.UnmarshalJSON(%s) error = %v, want errors.Is %v", tc.name, gotErr, tc.wantErr)
			}
			if receiver != tc.wantDocument {
				t.Fatalf("RequestDocument.UnmarshalJSON(%s) receiver = %+v, want %+v",
					tc.name, receiver, tc.wantDocument)
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

	if err := errors.Join(request.offering.Validate(), request.selection.Validate()); err != nil {
		t.Fatalf("queryFixtureRequest typed fields validation error = %v, want nil", err)
	}
	installation, err := controlplanetest.IssueInstallation(controlplanetest.InstallationRequest{
		AuthoritySeed: querySeed(request.authorityByte), DeviceSeed: querySeed(request.deviceByte),
		Offering: request.offering,
	})
	if err != nil {
		t.Fatalf("controlplanetest.IssueInstallation() error = %v, want nil", err)
	}
	scope, err := installation.Certificate.Body.Scope()
	if err != nil {
		t.Fatalf("InstallationCertificateBody.Scope() error = %v, want nil", err)
	}
	query, err := chit.NewQuery(chit.QueryRequest{
		Scope:     scope,
		Partition: queryPartition(t, request.nonceByte),
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
	client, err := controlplane.NewClient(controlplane.ClientConfiguration{TrustedAuthorityKeys: trusted})
	if err != nil {
		t.Fatalf("controlplane.NewClient() error = %v, want nil", err)
	}
	server, err := controlplane.NewServer(controlplane.ServerConfiguration{TrustedAuthorityKeys: trusted})
	if err != nil {
		t.Fatalf("controlplane.NewServer() error = %v, want nil", err)
	}
	return queryFixture{
		document: document, payload: payload, device: installation.DevicePrivate,
		client: client, server: server,
	}
}

func standardQueryFixtureRequest(t testing.TB) queryFixtureRequest {
	t.Helper()

	return queryFixtureRequest{
		offering: chitAuthOffering(t, 2), authorityByte: 0x21, deviceByte: 0x31,
		nonceByte: 0x41, selection: chit.All(),
	}
}

func queryPartition(t testing.TB, marker byte) chit.Partition {
	t.Helper()
	raw := [core.SHA256DigestBytes]byte{}
	for index := range raw {
		raw[index] = marker
	}
	partition, err := chit.NewPartition(core.NewSHA256Digest(raw))
	if err != nil {
		t.Fatalf("chit.NewPartition(marker %d) error = %v, want nil", marker, err)
	}
	return partition
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

func queryDocumentWithOffering(t testing.TB, fixture queryFixture, offering core.Offering) chit.QueryDocument {
	t.Helper()

	payload := fixture.payload
	payload.Query.Scope.Offering = offering
	document, err := chit.IssueQuery(chit.QueryIssuance{Signer: fixture.device, Payload: payload})
	if err != nil {
		t.Fatalf("chit.IssueQuery(other offering identity) error = %v, want nil", err)
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

	raw := [core.SHA256DigestBytes]byte{}
	for index := range raw {
		raw[index] = marker
	}
	nonce, err := controlwire.NewRequestNonce(raw)
	if err != nil {
		t.Fatalf("controlwire.NewRequestNonce() error = %v, want nil", err)
	}
	return nonce
}

func queryOffering(t testing.TB, offering core.Offering) core.Offering {
	t.Helper()
	if err := offering.Validate(); err != nil {
		t.Fatalf("Offering.Validate() error = %v, want nil", err)
	}
	return offering
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

func queryJSONAtLength(t testing.TB, encoded []byte, length int) []byte {
	t.Helper()

	if length < len(encoded) {
		t.Fatalf("requested JSON boundary length = %d, want at least canonical length %d", length, len(encoded))
	}
	padded := make([]byte, length)
	for index := 0; index < length-len(encoded); index++ {
		padded[index] = ' '
	}
	copy(padded[length-len(encoded):], encoded)
	if len(padded) != length {
		t.Fatalf("padded JSON length = %d, want %d", len(padded), length)
	}
	return padded
}
