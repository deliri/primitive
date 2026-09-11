package distributionauth

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/distribution"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/release"
	"github.com/deliri/primitive/v2026/temporal"
	"testing"
)

// This harness proves the outer signed response socket. Inner publication-grant
// authentication is separately exercised by TestPublicationResponseLayerTriad.
// Its oracle compares the authenticated bytes with the real issuer's typed body;
// it never computes an expected signature or duplicates the verifier.
type responseSocket[B core.Validatable, P interface {
	*B
	core.Validatable
	json.Unmarshaler
}] struct {
	canonical       []byte
	sibling         []byte
	body            []byte
	header          controlplane.ResponseHeader
	wantBinding     controlplane.ResponseExpectation
	siblingExpected controlplane.ResponseExpectation
	verify          func(controlplane.ResponseDocument[B, P], controlplane.ResponseExpectation) (controlplane.VerifiedResponse[B, P], error)
	compare         func(testing.TB, B, []byte)
	dispose         func(testing.TB, B)
}

func responseSocketFixture[I core.ValidatedJSONMarshaler, B core.Validatable, P interface {
	*B
	core.Validatable
	json.Unmarshaler
}](
	t testing.TB, fixture publicationAuthFixture, family controlwire.RouteFamily, body I,
	issue func(controlplane.ResponseIssuance[I]) (controlplane.ResponseProjection[I], error),
	verify func(controlplane.ResponseVerification[B, P]) (controlplane.VerifiedResponse[B, P], error),
	compare func(testing.TB, B, []byte), dispose func(testing.TB, B),
) responseSocket[B, P] {
	t.Helper()
	request := fixture.document.Request.Payload
	cert := fixture.installation.Certificate.Body
	activation, err := controlwire.NewPolicyActivation(1)
	if err != nil {
		t.Fatalf("NewPolicyActivation() error = %v, want nil", err)
	}
	header := controlplane.ResponseHeader{ProviderTime: cert.IssuedAt, RequestNonce: request.Nonce, Account: cert.Account, Installation: cert.Subject.DeviceID, Revision: request.Revision, Family: family, Status: controlplane.ProductStatusActive, Offering: request.Build.Offering(), Policy: controlwire.PolicyCursor{Revision: controlwire.PolicyRevisionID{1}, Activation: activation}}
	wantBinding := controlplane.ResponseExpectation{RequestNonce: header.RequestNonce, Account: header.Account, Installation: header.Installation, Revision: header.Revision, Family: family, Offering: header.Offering}
	issuance := controlplane.ResponseIssuance[I]{Server: distributionAuthServer(t, fixture.authority), Signer: fixture.installation.AuthorityPrivate, Header: header, Body: body, Assessment: acceptedDistributionResponseAssessment(t, header)}
	projection, err := issue(issuance)
	if err != nil {
		t.Fatalf("issue response family %v error = %v, want nil", family, err)
	}
	canonical, err := projection.MarshalJSON()
	if err != nil || len(canonical) == 0 {
		t.Fatalf("response projection = (%d bytes, %v), want nonempty and nil", len(canonical), err)
	}
	encodedBody, err := body.MarshalJSON()
	if err != nil || len(encodedBody) == 0 {
		t.Fatalf("typed body = (%d bytes, %v), want nonempty and nil", len(encodedBody), err)
	}
	siblingFamily := controlwire.RouteFamilyReleasePublications
	if family == siblingFamily {
		siblingFamily = controlwire.RouteFamilyUpdateChecks
	}
	issuance.Header.Family = siblingFamily
	issuance.Assessment = acceptedDistributionResponseAssessment(t, issuance.Header)
	refused, err := issue(issuance)
	if !errors.Is(err, core.ErrControlPlaneResponseBinding) || refused.Validate() == nil {
		t.Fatalf("issue sibling family = (%v, %v), want no projection and typed binding refusal", refused.Validate(), err)
	}
	siblingProjection, err := controlplane.IssueResponse(issuance)
	if err != nil {
		t.Fatalf("IssueResponse(authentic sibling) error = %v, want nil", err)
	}
	sibling, err := siblingProjection.MarshalJSON()
	if err != nil {
		t.Fatalf("sibling MarshalJSON() error = %v, want nil", err)
	}
	siblingExpected := wantBinding
	siblingExpected.Family = siblingFamily
	client := distributionAuthClient(t, fixture.authority)
	return responseSocket[B, P]{canonical: canonical, sibling: sibling, body: encodedBody, header: header, wantBinding: wantBinding, siblingExpected: siblingExpected, compare: compare, dispose: dispose, verify: func(d controlplane.ResponseDocument[B, P], e controlplane.ResponseExpectation) (controlplane.VerifiedResponse[B, P], error) {
		return verify(controlplane.ResponseVerification[B, P]{Document: d, Expected: e, Client: client})
	}}
}

func (s responseSocket[B, P]) evidence(t testing.TB, proof controlplane.VerifiedResponse[B, P]) {
	t.Helper()
	header, err := proof.Header()
	if err != nil || header != s.header {
		t.Fatalf("authenticated header = (%v, %v), want exact signed header", header, err)
	}
	body, err := proof.Body()
	if err != nil {
		t.Fatalf("authenticated Body() error = %v, want nil", err)
	}
	defer s.dispose(t, body)
	s.compare(t, body, s.body)
}

func (s responseSocket[B, P]) triad(t *testing.T) {
	t.Helper()
	cases := []struct {
		name        string
		data        []byte
		wantBinding controlplane.ResponseExpectation
		wantErr     error
	}{
		{name: "signed body remains bound to exact request", data: s.canonical, wantBinding: s.wantBinding},
		{name: "authentic sibling family cannot cross socket", data: s.sibling, wantBinding: s.siblingExpected, wantErr: core.ErrControlPlaneResponseBinding},
		{name: "absent response cannot yield proof", wantErr: core.ErrControlPlaneResponseDocument},
	}
	wrongNonce := s.wantBinding
	wrongNonce.RequestNonce = distributionAuthNonce(t, 0xe7)
	if wrongNonce.RequestNonce == s.wantBinding.RequestNonce {
		t.Fatal("nonce mutation = baseline, want changed binding")
	}
	cases = append(cases, struct {
		name        string
		data        []byte
		wantBinding controlplane.ResponseExpectation
		wantErr     error
	}{"signed response for another nonce", s.canonical, wrongNonce, core.ErrControlPlaneResponseBinding})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var document controlplane.ResponseDocument[B, P]
			if len(tc.data) > 0 {
				if err := document.UnmarshalJSON(tc.data); err != nil {
					t.Fatalf("UnmarshalJSON(typed signed seed) error = %v, want nil", err)
				}
			}
			proof, err := s.verify(document, tc.wantBinding)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || proof.Validate() == nil {
					t.Fatalf("verify refusal = (%v, %v), want zero proof and %v", proof.Validate(), err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("verify signed response error = %v, want nil", err)
			}
			s.evidence(t, proof)
		})
	}
}

func (s responseSocket[B, P]) fuzz(f *testing.F) {
	f.Helper()
	f.Add(s.canonical)
	f.Add(s.sibling)
	f.Add([]byte{})
	f.Add(append(bytes.Clone(s.canonical), 0))
	f.Fuzz(func(t *testing.T, data []byte) {
		var document controlplane.ResponseDocument[B, P]
		if err := document.UnmarshalJSON(s.canonical); err != nil {
			t.Fatalf("seed decode error = %v, want nil", err)
		}
		err := document.UnmarshalJSON(data)
		if err != nil {
			if !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrControlPlaneResponseDocument) {
				t.Fatalf("decode refusal = %v, want typed JSON/response error", err)
			}
			proof, verifyErr := s.verify(document, s.wantBinding)
			if verifyErr != nil {
				t.Fatalf("refused decode changed receiver: verify error = %v, want nil", verifyErr)
			}
			s.evidence(t, proof)
			return
		}
		if err := document.Validate(); err != nil {
			t.Fatalf("admitted response Validate error = %v, want nil", err)
		}
		proof, err := s.verify(document, s.wantBinding)
		if err != nil {
			if (!errors.Is(err, core.ErrAttestVerification) && !errors.Is(err, core.ErrControlPlaneResponseBinding) && !errors.Is(err, core.ErrControlPlaneResponseDocument)) || proof.Validate() == nil {
				t.Fatalf("verify refusal = (%v, %v), want typed refusal and zero proof", proof.Validate(), err)
			}
			return
		}
		s.evidence(t, proof)
	})
}

func retainedResponseBody[B core.Validatable](t testing.TB, body B) {
	t.Helper()
	if err := body.Validate(); err != nil {
		t.Errorf("retained body Validate error = %v, want nil", err)
	}
}
func destroyResponseMaterial(t testing.TB, body release.MaterialResponse) {
	t.Helper()
	if err := body.Destroy(); err != nil {
		t.Errorf("MaterialResponse.Destroy error = %v, want nil", err)
	}
}

func socketLatest(t testing.TB, fixture publicationAuthFixture) release.LatestDocument {
	t.Helper()
	generation, err := release.NewGeneration(1)
	if err != nil {
		t.Fatalf("NewGeneration error = %v, want nil", err)
	}
	issuedAt, err := fixture.installation.Certificate.Body.IssuedAt.Nanoseconds()
	if err != nil {
		t.Fatalf("IssuedAt.Nanoseconds error = %v, want nil", err)
	}
	latest, err := release.IssueLatest(release.IssueLatestRequest{Key: fixture.installation.AuthorityPrivate, Manifest: fixture.release.verified, IssuedAt: fixture.installation.Certificate.Body.IssuedAt, ValidFrom: fixture.installation.Certificate.Body.IssuedAt, ValidUntil: temporal.InstantFromNanoseconds(issuedAt + 1_000_000_000), Generation: generation})
	if err != nil {
		t.Fatalf("IssueLatest error = %v, want nil", err)
	}
	return latest
}
func socketMaterial(t testing.TB, fixture publicationAuthFixture) release.MaterialResponse {
	t.Helper()
	build := fixture.installation.Build
	request, err := release.NewMaterialRequest(release.MaterialRequestInput{Version: build.Version(), Commit: build.Commit(), Offering: build.Offering(), Nonce: fixture.document.Request.Payload.Nonce})
	if err != nil {
		t.Fatalf("NewMaterialRequest error = %v, want nil", err)
	}
	seed, err := release.NewReleaseSigningSeed(distributionAuthSeed(0x63))
	if err != nil {
		t.Fatalf("NewReleaseSigningSeed error = %v, want nil", err)
	}
	body := release.MaterialResponse{Request: request, ReleaseSigningSeed: seed, ServerPublicKey: fixture.installation.AuthorityPublic}
	t.Cleanup(func() { destroyResponseMaterial(t, body) })
	return body
}
func socketUpdate(t testing.TB, fixture publicationAuthFixture) distribution.UpdateResponseDocument {
	t.Helper()
	request := distribution.UpdateRequestPayload{Build: fixture.installation.Build, Nonce: fixture.document.Request.Payload.Nonce, Revision: fixture.document.Request.Payload.Revision}
	commitment, err := distribution.CommitRequest(request)
	if err != nil {
		t.Fatalf("CommitRequest error = %v, want nil", err)
	}
	body, err := distribution.IssueUpdateResponse(distribution.UpdateResponseIssuance{Signer: fixture.installation.AuthorityPrivate, Payload: distribution.UpdateResponsePayload{Installed: fixture.release.document, Latest: socketLatest(t, fixture), Request: commitment, IssuedAt: fixture.installation.Certificate.Body.IssuedAt, ExpiresAt: fixture.grant.Payload.ExpiresAt}})
	if err != nil {
		t.Fatalf("IssueUpdateResponse(inner) error = %v, want nil", err)
	}
	return body
}
func socketUpgrade(t testing.TB, fixture publicationAuthFixture) distribution.UpgradeGrantProjection {
	t.Helper()
	request := newDistributionAuthFixture(t, distributionAuthFixtureRequest{}).upgrade.Request.Payload
	commitment, err := distribution.CommitRequest(request)
	if err != nil {
		t.Fatalf("CommitRequest error = %v, want nil", err)
	}
	signedURL, err := objectstore.ParseSignedURL("https://storage.googleapis.com/bucket/object?X-Goog-Signature=signature&X-Goog-SignedHeaders=host")
	if err != nil {
		t.Fatalf("ParseSignedURL error = %v, want nil", err)
	}
	headers, err := objectstore.NewSignedHeaders(nil)
	if err != nil {
		t.Fatalf("NewSignedHeaders error = %v, want nil", err)
	}
	capability, err := objectstore.NewDownloadCapabilityProjection(objectstore.ProviderGoogleCloudStorage, objectstore.DownloadTarget{URL: signedURL, Headers: headers, ExpiresAt: temporal.InstantFromNanoseconds(2_051_222_400_000_000_000)})
	if err != nil {
		t.Fatalf("NewDownloadCapabilityProjection error = %v, want nil", err)
	}
	capabilityCommitment, err := capability.Commitment()
	if err != nil {
		t.Fatalf("Capability.Commitment error = %v, want nil", err)
	}
	body, err := distribution.IssueUpgradeGrant(distribution.UpgradeGrantIssuance{Signer: fixture.installation.AuthorityPrivate, Capability: capability, Payload: distribution.UpgradeGrantPayload{Request: commitment, Authorization: publicationAuthAuthorityNonce(t, 0x72), Capability: capabilityCommitment, IssuedAt: fixture.installation.Certificate.Body.IssuedAt, ExpiresAt: fixture.grant.Payload.ExpiresAt}})
	if err != nil {
		t.Fatalf("IssueUpgradeGrant error = %v, want nil", err)
	}
	return body
}

func materialResponseSocket(t testing.TB) responseSocket[release.MaterialResponse, *release.MaterialResponse] {
	t.Helper()
	fixture := newPublicationAuthFixture(t, publicationAuthFixtureRequest{})
	return responseSocketFixture[release.MaterialResponse, release.MaterialResponse, *release.MaterialResponse](t, fixture, controlwire.RouteFamilyReleaseMaterials, socketMaterial(t, fixture),
		func(i controlplane.ResponseIssuance[release.MaterialResponse]) (controlplane.ResponseProjection[release.MaterialResponse], error) {
			return IssueMaterialResponse(MaterialResponseIssuance{Signer: i.Signer, Header: i.Header, Body: i.Body, Server: i.Server, Assessment: i.Assessment})
		},
		func(v controlplane.ResponseVerification[release.MaterialResponse, *release.MaterialResponse]) (controlplane.VerifiedResponse[release.MaterialResponse, *release.MaterialResponse], error) {
			return VerifyMaterialResponse(MaterialResponseVerification{Document: v.Document, Expected: v.Expected, Client: v.Client})
		}, compareSocketBody[release.MaterialResponse], destroyResponseMaterial)
}
func TestMaterialResponseSocketLayerTriad(t *testing.T) {
	t.Parallel()
	materialResponseSocket(t).triad(t)
}
func FuzzMaterialResponseSocketSemanticAuthentication(f *testing.F) {
	materialResponseSocket(f).fuzz(f)
}

func publicationResponseSocket(t testing.TB) responseSocket[distribution.PublicationGrantDocument, *distribution.PublicationGrantDocument] {
	t.Helper()
	fixture := newPublicationAuthFixture(t, publicationAuthFixtureRequest{})
	return responseSocketFixture[distribution.PublicationGrantProjection, distribution.PublicationGrantDocument, *distribution.PublicationGrantDocument](t, fixture, controlwire.RouteFamilyReleasePublications, fixture.grantProjection,
		func(i controlplane.ResponseIssuance[distribution.PublicationGrantProjection]) (controlplane.ResponseProjection[distribution.PublicationGrantProjection], error) {
			return IssuePublicationResponse(PublicationResponseIssuance{Signer: i.Signer, Header: i.Header, Body: i.Body, Server: i.Server, Assessment: i.Assessment})
		},
		func(v controlplane.ResponseVerification[distribution.PublicationGrantDocument, *distribution.PublicationGrantDocument]) (controlplane.VerifiedResponse[distribution.PublicationGrantDocument, *distribution.PublicationGrantDocument], error) {
			return VerifyPublicationResponse(PublicationResponseVerification{Document: v.Document, Expected: v.Expected, Client: v.Client})
		}, comparePublicationSocketBody, retainedResponseBody[distribution.PublicationGrantDocument])
}
func TestPublicationResponseSocketLayerTriad(t *testing.T) {
	t.Parallel()
	publicationResponseSocket(t).triad(t)
}
func FuzzPublicationResponseSocketSemanticAuthentication(f *testing.F) {
	publicationResponseSocket(f).fuzz(f)
}

func publicationcompletionResponseSocket(t testing.TB) responseSocket[release.LatestDocument, *release.LatestDocument] {
	t.Helper()
	fixture := newPublicationAuthFixture(t, publicationAuthFixtureRequest{})
	return responseSocketFixture[release.LatestDocument, release.LatestDocument, *release.LatestDocument](t, fixture, controlwire.RouteFamilyReleasePublicationCompletions, socketLatest(t, fixture),
		func(i controlplane.ResponseIssuance[release.LatestDocument]) (controlplane.ResponseProjection[release.LatestDocument], error) {
			return IssuePublicationCompletionResponse(PublicationCompletionResponseIssuance{Signer: i.Signer, Header: i.Header, Body: i.Body, Server: i.Server, Assessment: i.Assessment})
		},
		func(v controlplane.ResponseVerification[release.LatestDocument, *release.LatestDocument]) (controlplane.VerifiedResponse[release.LatestDocument, *release.LatestDocument], error) {
			return VerifyPublicationCompletionResponse(PublicationCompletionResponseVerification{Document: v.Document, Expected: v.Expected, Client: v.Client})
		}, compareSocketBody[release.LatestDocument], retainedResponseBody[release.LatestDocument])
}
func TestPublicationCompletionResponseSocketLayerTriad(t *testing.T) {
	t.Parallel()
	publicationcompletionResponseSocket(t).triad(t)
}
func FuzzPublicationCompletionResponseSocketSemanticAuthentication(f *testing.F) {
	publicationcompletionResponseSocket(f).fuzz(f)
}

func updateResponseSocket(t testing.TB) responseSocket[distribution.UpdateResponseDocument, *distribution.UpdateResponseDocument] {
	t.Helper()
	fixture := newPublicationAuthFixture(t, publicationAuthFixtureRequest{})
	return responseSocketFixture[distribution.UpdateResponseDocument, distribution.UpdateResponseDocument, *distribution.UpdateResponseDocument](t, fixture, controlwire.RouteFamilyUpdateChecks, socketUpdate(t, fixture),
		func(i controlplane.ResponseIssuance[distribution.UpdateResponseDocument]) (controlplane.ResponseProjection[distribution.UpdateResponseDocument], error) {
			return IssueUpdateResponse(UpdateResponseIssuance{Signer: i.Signer, Header: i.Header, Body: i.Body, Server: i.Server, Assessment: i.Assessment})
		},
		func(v controlplane.ResponseVerification[distribution.UpdateResponseDocument, *distribution.UpdateResponseDocument]) (controlplane.VerifiedResponse[distribution.UpdateResponseDocument, *distribution.UpdateResponseDocument], error) {
			return VerifyUpdateResponse(UpdateResponseVerification{Document: v.Document, Expected: v.Expected, Client: v.Client})
		}, compareSocketBody[distribution.UpdateResponseDocument], retainedResponseBody[distribution.UpdateResponseDocument])
}
func TestUpdateResponseSocketLayerTriad(t *testing.T)             { t.Parallel(); updateResponseSocket(t).triad(t) }
func FuzzUpdateResponseSocketSemanticAuthentication(f *testing.F) { updateResponseSocket(f).fuzz(f) }

func upgradeResponseSocket(t testing.TB) responseSocket[distribution.UpgradeGrantDocument, *distribution.UpgradeGrantDocument] {
	t.Helper()
	fixture := newPublicationAuthFixture(t, publicationAuthFixtureRequest{})
	return responseSocketFixture[distribution.UpgradeGrantProjection, distribution.UpgradeGrantDocument, *distribution.UpgradeGrantDocument](t, fixture, controlwire.RouteFamilyUpgrades, socketUpgrade(t, fixture),
		func(i controlplane.ResponseIssuance[distribution.UpgradeGrantProjection]) (controlplane.ResponseProjection[distribution.UpgradeGrantProjection], error) {
			return IssueUpgradeResponse(UpgradeResponseIssuance{Signer: i.Signer, Header: i.Header, Body: i.Body, Server: i.Server, Assessment: i.Assessment})
		},
		func(v controlplane.ResponseVerification[distribution.UpgradeGrantDocument, *distribution.UpgradeGrantDocument]) (controlplane.VerifiedResponse[distribution.UpgradeGrantDocument, *distribution.UpgradeGrantDocument], error) {
			return VerifyUpgradeResponse(UpgradeResponseVerification{Document: v.Document, Expected: v.Expected, Client: v.Client})
		}, compareUpgradeSocketBody, retainedResponseBody[distribution.UpgradeGrantDocument])
}
func TestUpgradeResponseSocketLayerTriad(t *testing.T) {
	t.Parallel()
	upgradeResponseSocket(t).triad(t)
}
func FuzzUpgradeResponseSocketSemanticAuthentication(f *testing.F) { upgradeResponseSocket(f).fuzz(f) }

func compareSocketBody[B core.ValidatedJSONMarshaler](t testing.TB, body B, want []byte) {
	t.Helper()
	got, err := body.MarshalJSON()
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("body projection = (%d bytes, %v), want exact signed bytes", len(got), err)
	}
}
func comparePublicationSocketBody(t testing.TB, got distribution.PublicationGrantDocument, data []byte) {
	t.Helper()
	var want distribution.PublicationGrantDocument
	if err := want.UnmarshalJSON(data); err != nil {
		t.Fatalf("signed seed grant decode error = %v, want nil", err)
	}
	if got.Payload != want.Payload || got.Attestation != want.Attestation {
		t.Fatalf("grant payload = %v, want %v; attestation = %v, want %v", got.Payload, want.Payload, got.Attestation, want.Attestation)
	}
	for i := range got.Capabilities {
		a, ae := got.Capabilities[i].Commitment()
		b, be := want.Capabilities[i].Commitment()
		if ae != nil || be != nil || a != b {
			t.Fatalf("capability %d commitment = (%v, %v), want %v with nil error %v", i, a, ae, b, be)
		}
	}
}
func compareUpgradeSocketBody(t testing.TB, got distribution.UpgradeGrantDocument, data []byte) {
	t.Helper()
	var want distribution.UpgradeGrantDocument
	if err := want.UnmarshalJSON(data); err != nil {
		t.Fatalf("signed seed grant decode error = %v, want nil", err)
	}
	a, ae := got.Capability.Commitment()
	b, be := want.Capability.Commitment()
	if got.Payload != want.Payload || got.Attestation != want.Attestation || ae != nil || be != nil || a != b {
		t.Fatalf("grant facts and capability = (%v, %v), want exact signed facts and nil errors", ae, be)
	}
}
