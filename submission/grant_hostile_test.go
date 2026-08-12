package submission

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"hash/crc32"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/chit"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	testGrantIssuedAt          = int64(1_800_000_000_000_000_000)
	testGrantExpiresAt         = testGrantIssuedAt + 5_000_000_000
	testGrantRetainUntil       = testGrantIssuedAt + 31_536_000_000_000_000
	testCapabilityObjectPrefix = core.SchemeHTTPS + "://" + core.GoogleCloudStorageHost + "/custody/"
	testCapabilityQuery        = "?X-Goog-Signature=fixture&X-Goog-SignedHeaders=" +
		"host%3Bx-goog-hash%3Bx-goog-if-generation-match"
)

type grantFixtureRequest struct {
	objectName        string
	content           []byte
	expiresAt         int64
	offering          core.Offering
	requestNonceByte  byte
	authorityByte     byte
	authorizationByte byte
}

type grantFixture struct {
	request    RequestPayload
	projection GrantProjection
	document   GrantDocument
	trusted    attest.TrustedKeys
	payload    GrantPayload
}

func newGrantFixture(t testing.TB, request grantFixtureRequest) grantFixture {
	t.Helper()

	if request.content == nil {
		request.content = []byte(`{"proof":"source-free"}`)
	}
	if request.objectName == "" {
		request.objectName = "proof.json"
	}
	if request.offering == core.OfferingUnknown {
		request.offering = core.OfferingWitness
	}
	if request.requestNonceByte == 0 {
		request.requestNonceByte = 0x31
	}
	if request.authorityByte == 0 {
		request.authorityByte = 0x41
	}
	if request.authorizationByte == 0 {
		request.authorizationByte = 0x51
	}
	if request.expiresAt == 0 {
		request.expiresAt = testGrantExpiresAt
	}

	requestPayload := testRequestPayload(t, request)
	capability := testCapabilityProjection(t, request.objectName, request.expiresAt)
	commitment, err := capability.Commitment()
	if err != nil {
		t.Fatalf("UploadCapabilityProjection.Commitment() error = %v, want nil", err)
	}
	requestCommitment, err := CommitRequest(requestPayload)
	if err != nil {
		t.Fatalf("CommitRequest() error = %v, want nil", err)
	}
	authorization := testAuthorizationNonce(t, request.authorizationByte)
	payload := GrantPayload{
		Request: requestCommitment, Authorization: authorization, Capability: commitment,
		IssuedAt:    temporal.InstantFromNanoseconds(testGrantIssuedAt),
		ExpiresAt:   temporal.InstantFromNanoseconds(request.expiresAt),
		RetainUntil: temporal.InstantFromNanoseconds(testGrantRetainUntil),
	}
	public, signer := testSigningKey(t, request.authorityByte)
	projection, err := IssueGrant(GrantIssuance{
		Payload: payload, Capability: capability, Signer: signer,
	})
	if err != nil {
		t.Fatalf("IssueGrant() error = %v, want nil", err)
	}
	encoded, err := projection.MarshalJSON()
	if err != nil {
		t.Fatalf("GrantProjection.MarshalJSON() error = %v, want nil", err)
	}
	var document GrantDocument
	if err := json.Unmarshal(encoded, &document); err != nil {
		t.Fatalf("json.Unmarshal(GrantProjection) error = %v, want nil", err)
	}
	trusted, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{
		Keys: []core.Ed25519PublicKey{public},
	})
	if err != nil {
		t.Fatalf("attest.NewTrustedKeys() error = %v, want nil", err)
	}
	return grantFixture{
		request: requestPayload, payload: payload, projection: projection,
		document: document, trusted: trusted,
	}
}

func testRequestPayload(t testing.TB, request grantFixtureRequest) RequestPayload {
	t.Helper()
	if request.content == nil {
		request.content = []byte(`{"proof":"source-free"}`)
	}
	if request.offering == core.OfferingUnknown {
		request.offering = core.OfferingWitness
	}
	if request.requestNonceByte == 0 {
		request.requestNonceByte = 0x31
	}

	commit, err := core.ParseBuildCommit(strings.Repeat("a", 40))
	if err != nil {
		t.Fatalf("core.ParseBuildCommit() error = %v, want nil", err)
	}
	build, err := core.NewBuildIdentity(core.BuildIdentityRequest{
		Version: core.NewReleaseVersion(2026, 0, 53), Commit: commit,
		Platform: core.Platform{
			OperatingSystem: core.OperatingSystemDarwin,
			Architecture:    core.CPUArchitectureARM64,
		},
		Offering: request.offering,
	})
	if err != nil {
		t.Fatalf("core.NewBuildIdentity() error = %v, want nil", err)
	}
	rawNonce := [controlwire.NonceBytes]byte{}
	for index := range rawNonce {
		rawNonce[index] = request.requestNonceByte
	}
	nonce, err := controlwire.NewRequestNonce(rawNonce)
	if err != nil {
		t.Fatalf("controlwire.NewRequestNonce() error = %v, want nil", err)
	}
	return RequestPayload{
		Declaration: testDeclaration(t, request.content), Manifest: testManifestIntent(t), Build: build,
		Revision: controlwire.Revision2026V1, Nonce: nonce,
	}
}

func testManifestIntent(t testing.TB) ManifestIntent {
	t.Helper()
	upload, err := ParseUploadID("00000000-0004-7000-8000-000000000004")
	if err != nil {
		t.Fatalf("ParseUploadID() error = %v, want nil", err)
	}
	collection, err := chit.ParseCollectionID("00000000-0005-7000-8000-000000000005")
	if err != nil {
		t.Fatalf("chit.ParseCollectionID() error = %v, want nil", err)
	}
	name, err := chit.ParseEntryName("proof.json")
	if err != nil {
		t.Fatalf("chit.ParseEntryName() error = %v, want nil", err)
	}
	sequence, err := chit.NewEntrySequence(1)
	if err != nil {
		t.Fatalf("chit.NewEntrySequence() error = %v, want nil", err)
	}
	objects, err := chit.NewObjectCount(1)
	if err != nil {
		t.Fatalf("chit.NewObjectCount() error = %v, want nil", err)
	}
	return ManifestIntent{
		Upload: upload, Collection: collection, Name: name, Sequence: sequence, Objects: objects,
	}
}

func testDeclaration(t testing.TB, content []byte) Declaration {
	t.Helper()

	contentType, err := core.ParseHTTPMediaType("application/json")
	if err != nil {
		t.Fatalf("core.ParseHTTPMediaType() error = %v, want nil", err)
	}
	extent, err := core.NewByteLength(uint64(len(content)))
	if err != nil {
		t.Fatalf("core.NewByteLength() error = %v, want nil", err)
	}
	return Declaration{
		ContentType: contentType, Extent: extent, SHA256: core.SHA256Of(content),
		CRC32C: core.NewCRC32C(crc32.Checksum(content, crc32.MakeTable(crc32.Castagnoli))),
	}
}

func testBuildIdentity(t testing.TB, request core.BuildIdentityRequest) core.BuildIdentity {
	t.Helper()

	identity, err := core.NewBuildIdentity(request)
	if err != nil {
		t.Fatalf("core.NewBuildIdentity() error = %v, want nil", err)
	}
	return identity
}

func testByteLength(t testing.TB, value uint64) core.ByteLength {
	t.Helper()

	extent, err := core.NewByteLength(value)
	if err != nil {
		t.Fatalf("core.NewByteLength(%d) error = %v, want nil", value, err)
	}
	return extent
}

func testCapabilityProjection(
	t testing.TB,
	objectName string,
	expiresAt int64,
) objectstore.UploadCapabilityProjection {
	t.Helper()

	signedURL, err := objectstore.ParseSignedURL(
		testCapabilityObjectPrefix + objectName + testCapabilityQuery,
	)
	if err != nil {
		t.Fatalf("objectstore.ParseSignedURL() error = %v, want nil", err)
	}
	headers, err := objectstore.NewSignedHeaders(nil)
	if err != nil {
		t.Fatalf("objectstore.NewSignedHeaders(nil) error = %v, want nil", err)
	}
	projection, err := objectstore.NewUploadCapabilityProjection(
		objectstore.ProviderGoogleCloudStorage,
		objectstore.UploadTarget{
			Headers: headers, URL: signedURL,
			ExpiresAt: temporal.InstantFromNanoseconds(expiresAt),
		},
	)
	if err != nil {
		t.Fatalf("objectstore.NewUploadCapabilityProjection() error = %v, want nil", err)
	}
	return projection
}

func testAuthorizationNonce(t testing.TB, value byte) controlwire.AuthorityNonce {
	t.Helper()

	raw := [controlwire.NonceBytes]byte{}
	for index := range raw {
		raw[index] = value
	}
	nonce, err := controlwire.NewAuthorityNonce(raw)
	if err != nil {
		t.Fatalf("NewAuthorizationNonce() error = %v, want nil", err)
	}
	return nonce
}

func testSigningKey(t testing.TB, value byte) (core.Ed25519PublicKey, ed25519.PrivateKey) {
	t.Helper()

	seed := make([]byte, ed25519.SeedSize)
	for index := range seed {
		seed[index] = value
	}
	private := ed25519.NewKeyFromSeed(seed)
	public, err := core.NewEd25519PublicKey(private.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatalf("core.NewEd25519PublicKey() error = %v, want nil", err)
	}
	return public, private
}

// TestGrantVerificationLayerTriadAuthenticatesOneExactCurrentUploadAgreement is the positive proof
// that the authority signature, request commitment, bearer commitment, upload
// expiry, and retention promise survive the issue/receive boundary together.
func TestGrantVerificationLayerTriadAuthenticatesOneExactCurrentUploadAgreement(t *testing.T) {
	t.Parallel()

	fixture := newGrantFixture(t, grantFixtureRequest{})
	verified, err := VerifyGrant(GrantExpectation{
		Document: fixture.document, Request: fixture.request,
		ObservedAt:  temporal.InstantFromNanoseconds(testGrantIssuedAt),
		TrustedKeys: fixture.trusted,
	})
	if err != nil {
		t.Fatalf("VerifyGrant() error = %v, want nil", err)
	}
	payload, err := verified.Payload()
	if err != nil {
		t.Fatalf("VerifiedGrant.Payload() error = %v, want nil", err)
	}
	if payload != fixture.payload {
		t.Fatalf("VerifiedGrant.Payload() = %+v, want the exact issued payload %+v", payload, fixture.payload)
	}
	capability, err := verified.Capability()
	if err != nil {
		t.Fatalf("VerifiedGrant.Capability() error = %v, want nil", err)
	}
	provider, err := capability.Provider()
	if err != nil || provider != objectstore.ProviderGoogleCloudStorage {
		t.Fatalf("verified capability provider = (%v, %v), want (%v, nil)",
			provider, err, objectstore.ProviderGoogleCloudStorage)
	}
}

// TestGrantVerificationLayerTriadRefusesEveryNearMissOfItsExactRequest changes one valid request
// fact at a time. Every near miss still validates on its own, so only the
// signed request commitment can explain the refusal.
func TestGrantVerificationLayerTriadRefusesEveryNearMissOfItsExactRequest(t *testing.T) {
	t.Parallel()

	fixture := newGrantFixture(t, grantFixtureRequest{})
	otherMediaType, err := core.ParseHTTPMediaType("application/octet-stream")
	if err != nil {
		t.Fatalf("core.ParseHTTPMediaType() error = %v, want nil", err)
	}
	otherUpload, err := ParseUploadID("00000000-0006-7000-8000-000000000006")
	if err != nil {
		t.Fatalf("ParseUploadID() error = %v, want nil", err)
	}
	otherCollection, err := chit.ParseCollectionID("00000000-0007-7000-8000-000000000007")
	if err != nil {
		t.Fatalf("chit.ParseCollectionID() error = %v, want nil", err)
	}
	otherName, err := chit.ParseEntryName("other.json")
	if err != nil {
		t.Fatalf("chit.ParseEntryName() error = %v, want nil", err)
	}
	otherObjectCount, err := chit.NewObjectCount(2)
	if err != nil {
		t.Fatalf("chit.NewObjectCount() error = %v, want nil", err)
	}
	otherCommit, err := core.ParseBuildCommit(strings.Repeat("b", 40))
	if err != nil {
		t.Fatalf("core.ParseBuildCommit() error = %v, want nil", err)
	}
	baseBuild := fixture.request.Build
	otherVersion := testBuildIdentity(t, core.BuildIdentityRequest{
		Version: core.NewReleaseVersion(2026, 0, 54), Commit: baseBuild.Commit(),
		Platform: baseBuild.Platform(), Offering: baseBuild.Offering(),
	})
	otherBuildCommit := testBuildIdentity(t, core.BuildIdentityRequest{
		Version: baseBuild.Version(), Commit: otherCommit,
		Platform: baseBuild.Platform(), Offering: baseBuild.Offering(),
	})
	otherOperatingSystem := testBuildIdentity(t, core.BuildIdentityRequest{
		Version: baseBuild.Version(), Commit: baseBuild.Commit(),
		Platform: core.Platform{
			OperatingSystem: core.OperatingSystemLinux,
			Architecture:    baseBuild.Platform().Architecture,
		},
		Offering: baseBuild.Offering(),
	})
	otherArchitecture := testBuildIdentity(t, core.BuildIdentityRequest{
		Version: baseBuild.Version(), Commit: baseBuild.Commit(),
		Platform: core.Platform{
			OperatingSystem: baseBuild.Platform().OperatingSystem,
			Architecture:    core.CPUArchitectureAMD64,
		},
		Offering: baseBuild.Offering(),
	})
	otherOffering := testBuildIdentity(t, core.BuildIdentityRequest{
		Version: baseBuild.Version(), Commit: baseBuild.Commit(),
		Platform: baseBuild.Platform(), Offering: core.OfferingBug,
	})
	otherNonce := testRequestPayload(t, grantFixtureRequest{requestNonceByte: 0x32}).Nonce
	shorterExtent := testByteLength(t, fixture.request.Declaration.Extent.Uint64()-1)
	longerExtent := testByteLength(t, fixture.request.Declaration.Extent.Uint64()+1)
	cases := []struct {
		name   string
		mutate func(RequestPayload) RequestPayload
	}{
		{
			name: "content media type",
			mutate: func(value RequestPayload) RequestPayload {
				value.Declaration.ContentType = otherMediaType
				return value
			},
		},
		{
			name: "content extent one byte below",
			mutate: func(value RequestPayload) RequestPayload {
				value.Declaration.Extent = shorterExtent
				return value
			},
		},
		{
			name: "content extent one byte above",
			mutate: func(value RequestPayload) RequestPayload {
				value.Declaration.Extent = longerExtent
				return value
			},
		},
		{
			name: "content SHA-256",
			mutate: func(value RequestPayload) RequestPayload {
				value.Declaration.SHA256 = core.SHA256Of([]byte("other exact content"))
				return value
			},
		},
		{
			name: "content CRC32C",
			mutate: func(value RequestPayload) RequestPayload {
				value.Declaration.CRC32C = core.NewCRC32C(1)
				return value
			},
		},
		{
			name: "manifest upload identity",
			mutate: func(value RequestPayload) RequestPayload {
				value.Manifest.Upload = otherUpload
				return value
			},
		},
		{
			name: "manifest collection identity",
			mutate: func(value RequestPayload) RequestPayload {
				value.Manifest.Collection = otherCollection
				return value
			},
		},
		{
			name: "manifest entry name",
			mutate: func(value RequestPayload) RequestPayload {
				value.Manifest.Name = otherName
				return value
			},
		},
		{
			name: "manifest object count",
			mutate: func(value RequestPayload) RequestPayload {
				value.Manifest.Objects = otherObjectCount
				return value
			},
		},
		{
			name: "build release version",
			mutate: func(value RequestPayload) RequestPayload {
				value.Build = otherVersion
				return value
			},
		},
		{
			name: "build commit",
			mutate: func(value RequestPayload) RequestPayload {
				value.Build = otherBuildCommit
				return value
			},
		},
		{
			name: "build operating system",
			mutate: func(value RequestPayload) RequestPayload {
				value.Build = otherOperatingSystem
				return value
			},
		},
		{
			name: "build architecture",
			mutate: func(value RequestPayload) RequestPayload {
				value.Build = otherArchitecture
				return value
			},
		},
		{
			name: "build offering",
			mutate: func(value RequestPayload) RequestPayload {
				value.Build = otherOffering
				return value
			},
		},
		{
			name: "request nonce",
			mutate: func(value RequestPayload) RequestPayload {
				value.Nonce = otherNonce
				return value
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := tc.mutate(fixture.request)
			if err := request.Validate(); err != nil {
				t.Fatalf("near-miss RequestPayload.Validate() error = %v, want nil", err)
			}
			commitment, err := CommitRequest(request)
			if err != nil {
				t.Fatalf("CommitRequest(near miss) error = %v, want nil", err)
			}
			if commitment == fixture.payload.Request {
				t.Fatalf("CommitRequest(near miss) = %v, want distinct from issued %v",
					commitment, fixture.payload.Request)
			}
			verified, err := VerifyGrant(GrantExpectation{
				Document: fixture.document, Request: request,
				ObservedAt:  temporal.InstantFromNanoseconds(testGrantIssuedAt),
				TrustedKeys: fixture.trusted,
			})
			if !errors.Is(err, core.ErrControlPlaneResponseBinding) {
				t.Fatalf("VerifyGrant(near miss) error = %v, want errors.Is %v",
					err, core.ErrControlPlaneResponseBinding)
			}
			if err := verified.Validate(); !errors.Is(err, core.ErrControlPlaneContract) {
				t.Fatalf("rejected VerifiedGrant.Validate() error = %v, want errors.Is %v",
					err, core.ErrControlPlaneContract)
			}
			payload, payloadErr := verified.Payload()
			if payload != (GrantPayload{}) || !errors.Is(payloadErr, core.ErrControlPlaneContract) {
				t.Fatalf("rejected VerifiedGrant.Payload() = (%v, %v), want zero and errors.Is %v",
					payload, payloadErr, core.ErrControlPlaneContract)
			}
			capability, capabilityErr := verified.Capability()
			if !capability.IsZero() || !errors.Is(capabilityErr, core.ErrControlPlaneContract) {
				t.Fatalf("rejected VerifiedGrant.Capability() zero = %t, error = %v, want true and errors.Is %v",
					capability.IsZero(), capabilityErr, core.ErrControlPlaneContract)
			}
		})
	}
}

// TestGrantRefusesCapabilitySubstitutionAndExpiryDrift proves the separately
// transported bearer is usable only when every byte and its expiry agree with
// the authority-signed commitment.
func TestGrantRefusesCapabilitySubstitutionAndExpiryDrift(t *testing.T) {
	t.Parallel()

	fixture := newGrantFixture(t, grantFixtureRequest{})
	substitute := newGrantFixture(t, grantFixtureRequest{objectName: "other.json"})
	tampered := fixture.document
	tampered.Capability = substitute.document.Capability
	if err := tampered.Validate(); !errors.Is(err, core.ErrControlPlaneResponseBinding) {
		t.Fatalf("GrantDocument.Validate(capability substitution) error = %v, want errors.Is %v",
			err, core.ErrControlPlaneResponseBinding)
	}

	driftedCapability := testCapabilityProjection(t, "proof.json", testGrantExpiresAt+1)
	driftedCommitment, err := driftedCapability.Commitment()
	if err != nil {
		t.Fatalf("drifted UploadCapabilityProjection.Commitment() error = %v, want nil", err)
	}
	driftedPayload := fixture.payload
	driftedPayload.Capability = driftedCommitment
	projection, err := IssueGrant(GrantIssuance{
		Payload: driftedPayload, Capability: driftedCapability,
		Signer: testGrantSigner(t),
	})
	if !errors.Is(err, core.ErrControlPlaneResponseBinding) || !projection.Capability.IsZero() {
		t.Fatalf("IssueGrant(expiry drift) capability zero = %t, error = %v, want true and errors.Is %v",
			projection.Capability.IsZero(), err, core.ErrControlPlaneResponseBinding)
	}
}

// TestGrantRefusesEveryAuthorityAndSignedPayloadSubstitution proves a valid
// Ed25519 signature from another authority and a valid field substituted after
// signing are both non-authoritative.
func TestGrantRefusesEveryAuthorityAndSignedPayloadSubstitution(t *testing.T) {
	t.Parallel()

	fixture := newGrantFixture(t, grantFixtureRequest{})
	otherPublic, _ := testSigningKey(t, 0x42)
	otherTrust, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{
		Keys: []core.Ed25519PublicKey{otherPublic},
	})
	if err != nil {
		t.Fatalf("attest.NewTrustedKeys(other authority) error = %v, want nil", err)
	}
	if verified, err := VerifyGrant(GrantExpectation{
		Document: fixture.document, Request: fixture.request,
		ObservedAt:  temporal.InstantFromNanoseconds(testGrantIssuedAt),
		TrustedKeys: otherTrust,
	}); !errors.Is(err, core.ErrAttestVerification) {
		t.Fatalf("VerifyGrant(other authority) = (%v, %v), want zero and errors.Is %v",
			verified, err, core.ErrAttestVerification)
	}

	tampered := fixture.document
	tampered.Payload.Authorization = testAuthorizationNonce(t, 0x52)
	if err := tampered.Validate(); err != nil {
		t.Fatalf("tampered GrantDocument.Validate() error = %v, want nil", err)
	}
	if verified, err := VerifyGrant(GrantExpectation{
		Document: tampered, Request: fixture.request,
		ObservedAt:  temporal.InstantFromNanoseconds(testGrantIssuedAt),
		TrustedKeys: fixture.trusted,
	}); !errors.Is(err, core.ErrAttestVerification) {
		t.Fatalf("VerifyGrant(payload substitution) = (%v, %v), want zero and errors.Is %v",
			verified, err, core.ErrAttestVerification)
	}
}

// TestGrantPayloadRefusesEveryInvalidLifetimeMember attacks each signed time
// slot and both strict ordering edges independently.
func TestGrantPayloadRefusesEveryInvalidLifetimeMember(t *testing.T) {
	t.Parallel()

	fixture := newGrantFixture(t, grantFixtureRequest{})
	cases := []struct {
		name    string
		payload GrantPayload
	}{
		{name: "unset request commitment", payload: func() GrantPayload {
			value := fixture.payload
			value.Request = RequestCommitment{}
			return value
		}()},
		{name: "unset authorization nonce", payload: func() GrantPayload {
			value := fixture.payload
			value.Authorization = controlwire.AuthorityNonce{}
			return value
		}()},
		{name: "unset capability commitment", payload: func() GrantPayload {
			value := fixture.payload
			value.Capability = objectstore.UploadCapabilityCommitment{}
			return value
		}()},
		{name: "unset issued instant", payload: func() GrantPayload {
			value := fixture.payload
			value.IssuedAt = temporal.Instant{}
			return value
		}()},
		{name: "unset expiry instant", payload: func() GrantPayload {
			value := fixture.payload
			value.ExpiresAt = temporal.Instant{}
			return value
		}()},
		{name: "unset retention instant", payload: func() GrantPayload {
			value := fixture.payload
			value.RetainUntil = temporal.Instant{}
			return value
		}()},
		{name: "issuance equals expiry", payload: func() GrantPayload {
			value := fixture.payload
			value.ExpiresAt = value.IssuedAt
			return value
		}()},
		{name: "expiry equals retention", payload: func() GrantPayload {
			value := fixture.payload
			value.RetainUntil = value.ExpiresAt
			return value
		}()},
		{name: "expiry precedes issuance", payload: func() GrantPayload {
			value := fixture.payload
			value.ExpiresAt = temporal.InstantFromNanoseconds(testGrantIssuedAt - 1)
			return value
		}()},
		{name: "retention precedes expiry", payload: func() GrantPayload {
			value := fixture.payload
			value.RetainUntil = temporal.InstantFromNanoseconds(testGrantExpiresAt - 1)
			return value
		}()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := tc.payload.Validate(); !errors.Is(err, core.ErrControlPlaneContract) {
				t.Fatalf("GrantPayload.Validate(%s) error = %v, want errors.Is %v",
					tc.name, err, core.ErrControlPlaneContract)
			}
		})
	}
}

// TestAuthorizationNonceClosesEveryByteAndCanonicalTextEdge proves the nonce
// is exactly 32 nonzero bytes, admits a nonzero byte in every position, and
// rejects non-canonical external forms without changing prior authority.
func TestAuthorizationNonceClosesEveryByteAndCanonicalTextEdge(t *testing.T) {
	t.Parallel()

	for position := range controlwire.NonceBytes {
		t.Run("single nonzero byte at position "+strconv.Itoa(position), func(t *testing.T) {
			t.Parallel()

			raw := [controlwire.NonceBytes]byte{}
			raw[position] = 1
			nonce, err := controlwire.NewAuthorityNonce(raw)
			if err != nil {
				t.Fatalf("NewAuthorizationNonce(position %d) error = %v, want nil", position, err)
			}
			parsed, err := controlwire.ParseAuthorityNonce(nonce.String())
			if err != nil || parsed != nonce {
				t.Fatalf("ParseAuthorizationNonce(position %d) = (%v, %v), want (%v, nil)",
					position, parsed, err, nonce)
			}
		})
	}

	zero, err := controlwire.NewAuthorityNonce([controlwire.NonceBytes]byte{})
	if !errors.Is(err, core.ErrControlWireNonce) || zero != (controlwire.AuthorityNonce{}) {
		t.Fatalf("NewAuthorizationNonce(zero) = (%v, %v), want zero and errors.Is %v",
			zero, err, core.ErrControlPlaneContract)
	}
	preserved := testAuthorizationNonce(t, 0xab)
	encoded, err := preserved.MarshalJSON()
	if err != nil {
		t.Fatalf("AuthorizationNonce.MarshalJSON() error = %v, want nil", err)
	}
	var roundTrip controlwire.AuthorityNonce
	if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != preserved {
		t.Fatalf("AuthorizationNonce.UnmarshalJSON(canonical) = (%v, %v), want (%v, nil)",
			roundTrip, err, preserved)
	}
	canonical := preserved.String()
	cases := []struct {
		name string
		data []byte
	}{
		{name: "empty input", data: nil},
		{name: "null value", data: []byte(`null`)},
		{name: "numeric value", data: []byte(`1`)},
		{name: "boolean value", data: []byte(`true`)},
		{name: "array value", data: []byte(`[]`)},
		{name: "object value", data: []byte(`{}`)},
		{name: "empty text", data: []byte(`""`)},
		{name: "one hex digit", data: []byte(`"0"`)},
		{name: "one byte below exact extent", data: []byte(`"` + canonical[:len(canonical)-1] + `"`)},
		{name: "one byte above exact extent", data: []byte(`"` + canonical + `0"`)},
		{name: "all-zero nonce", data: []byte(`"` + strings.Repeat("0", 64) + `"`)},
		{name: "uppercase hex", data: []byte(`"` + strings.ToUpper(canonical) + `"`)},
		{name: "non-hex character", data: []byte(`"` + canonical[:63] + `g"`)},
		{name: "hex prefix", data: []byte(`"0x` + canonical[:62] + `"`)},
		{name: "leading space inside text", data: []byte(`" ` + canonical[:63] + `"`)},
		{name: "trailing second JSON value", data: append(bytes.Clone(encoded), []byte(` null`)...)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			receiver := preserved
			if err := receiver.UnmarshalJSON(tc.data); !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("AuthorizationNonce.UnmarshalJSON(%s) error = %v, want errors.Is %v",
					tc.name, err, core.ErrJSONContract)
			}
			if receiver != preserved {
				t.Fatalf("AuthorizationNonce.UnmarshalJSON(%s) receiver = %v, want preserved %v",
					tc.name, receiver, preserved)
			}
		})
	}
}

// TestGrantJSONBoundaryIsStrictBoundedAndPreserving attacks the bearer-bearing
// outer document without comparing error text or exposing the bearer.
func TestGrantJSONBoundaryIsStrictBoundedAndPreserving(t *testing.T) {
	t.Parallel()

	fixture := newGrantFixture(t, grantFixtureRequest{})
	encoded, err := fixture.projection.MarshalJSON()
	if err != nil {
		t.Fatalf("GrantProjection.MarshalJSON() error = %v, want nil", err)
	}
	missingCapability, err := json.Marshal(struct {
		Payload     GrantPayload                   `json:"payload"`
		Attestation attest.Envelope[SigningDomain] `json:"attestation"`
	}{Payload: fixture.payload, Attestation: fixture.document.Attestation})
	if err != nil {
		t.Fatalf("json.Marshal(missing capability fixture) error = %v, want nil", err)
	}
	missingPayload, err := json.Marshal(struct {
		Capability  objectstore.UploadCapabilityProjection `json:"capability"`
		Attestation attest.Envelope[SigningDomain]         `json:"attestation"`
	}{Attestation: fixture.document.Attestation, Capability: fixture.projection.Capability})
	if err != nil {
		t.Fatalf("json.Marshal(missing payload fixture) error = %v, want nil", err)
	}
	missingAttestation, err := json.Marshal(struct {
		Capability objectstore.UploadCapabilityProjection `json:"capability"`
		Payload    GrantPayload                           `json:"payload"`
	}{Payload: fixture.payload, Capability: fixture.projection.Capability})
	if err != nil {
		t.Fatalf("json.Marshal(missing attestation fixture) error = %v, want nil", err)
	}
	reordered, err := json.Marshal(struct {
		Capability  objectstore.UploadCapabilityProjection `json:"capability"`
		Attestation attest.Envelope[SigningDomain]         `json:"attestation"`
		Payload     GrantPayload                           `json:"payload"`
	}{
		Capability:  fixture.projection.Capability,
		Attestation: fixture.document.Attestation,
		Payload:     fixture.payload,
	})
	if err != nil {
		t.Fatalf("json.Marshal(reordered grant fixture) error = %v, want nil", err)
	}
	nullPayload, err := json.Marshal(struct {
		Payload     *GrantPayload                          `json:"payload"`
		Capability  objectstore.UploadCapabilityProjection `json:"capability"`
		Attestation attest.Envelope[SigningDomain]         `json:"attestation"`
	}{Attestation: fixture.document.Attestation, Capability: fixture.projection.Capability})
	if err != nil {
		t.Fatalf("json.Marshal(null grant payload fixture) error = %v, want nil", err)
	}
	nullCapability, err := json.Marshal(struct {
		Capability  *objectstore.UploadCapabilityProjection `json:"capability"`
		Payload     GrantPayload                            `json:"payload"`
		Attestation attest.Envelope[SigningDomain]          `json:"attestation"`
	}{Payload: fixture.payload, Attestation: fixture.document.Attestation})
	if err != nil {
		t.Fatalf("json.Marshal(null capability fixture) error = %v, want nil", err)
	}
	wrongPayloadType, err := json.Marshal(struct {
		Capability  objectstore.UploadCapabilityProjection `json:"capability"`
		Attestation attest.Envelope[SigningDomain]         `json:"attestation"`
		Payload     int                                    `json:"payload"`
	}{Payload: 1, Attestation: fixture.document.Attestation, Capability: fixture.projection.Capability})
	if err != nil {
		t.Fatalf("json.Marshal(wrong grant payload type fixture) error = %v, want nil", err)
	}
	wrongAttestationType, err := json.Marshal(struct {
		Capability  objectstore.UploadCapabilityProjection `json:"capability"`
		Payload     GrantPayload                           `json:"payload"`
		Attestation int                                    `json:"attestation"`
	}{Payload: fixture.payload, Attestation: 1, Capability: fixture.projection.Capability})
	if err != nil {
		t.Fatalf("json.Marshal(wrong grant attestation type fixture) error = %v, want nil", err)
	}
	wrongCapabilityType, err := json.Marshal(struct {
		Payload     GrantPayload                   `json:"payload"`
		Attestation attest.Envelope[SigningDomain] `json:"attestation"`
		Capability  int                            `json:"capability"`
	}{Payload: fixture.payload, Attestation: fixture.document.Attestation, Capability: 1})
	if err != nil {
		t.Fatalf("json.Marshal(wrong capability type fixture) error = %v, want nil", err)
	}
	var indented bytes.Buffer
	if err := json.Indent(&indented, encoded, "", "  "); err != nil {
		t.Fatalf("json.Indent(grant) error = %v, want nil", err)
	}
	unknown := append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"future":true}`)...)
	duplicatePayload := append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"payload":null}`)...)
	duplicateAttestation := append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"attestation":null}`)...)
	duplicateCapability := append(bytes.Clone(encoded[:len(encoded)-1]), []byte(`,"capability":null}`)...)
	originalCommitment, err := fixture.document.Capability.Commitment()
	if err != nil {
		t.Fatalf("original capability Commitment() error = %v, want nil", err)
	}
	validCases := []struct {
		name string
		data []byte
	}{
		{name: "canonical grant", data: encoded},
		{name: "one leading space", data: append([]byte(" "), encoded...)},
		{name: "one trailing space", data: append(bytes.Clone(encoded), ' ')},
		{name: "leading and trailing newlines", data: append(append([]byte("\n"), encoded...), '\n')},
		{name: "mixed legal outer whitespace", data: append(append([]byte("\t\r\n"), encoded...), ' ', '\t')},
		{name: "members in reverse order", data: reordered},
		{name: "indented grant", data: indented.Bytes()},
		{name: "one byte below document ceiling", data: leftPadJSON(encoded, GrantDocumentJSONMaximumBytes-1)},
		{name: "exactly at document ceiling", data: leftPadJSON(encoded, GrantDocumentJSONMaximumBytes)},
		{name: "canonical second decode", data: bytes.Clone(encoded)},
	}
	for _, tc := range validCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var receiver GrantDocument
			if err := receiver.UnmarshalJSON(tc.data); err != nil {
				t.Fatalf("GrantDocument.UnmarshalJSON(%s) error = %v, want nil", tc.name, err)
			}
			gotCommitment, err := receiver.Capability.Commitment()
			if err != nil || gotCommitment != originalCommitment ||
				receiver.Payload != fixture.document.Payload ||
				receiver.Attestation != fixture.document.Attestation {
				t.Fatalf("GrantDocument.UnmarshalJSON(%s) did not preserve the exact grant", tc.name)
			}
		})
	}
	cases := []struct {
		name string
		data []byte
	}{
		{name: "empty input", data: nil},
		{name: "whitespace without a value", data: []byte(" \t\r\n")},
		{name: "null root", data: []byte(`null`)},
		{name: "string root", data: []byte(`"grant"`)},
		{name: "number root", data: []byte(`1`)},
		{name: "boolean root", data: []byte(`true`)},
		{name: "array root", data: []byte(`[]`)},
		{name: "empty object", data: []byte(`{}`)},
		{name: "unknown member", data: unknown},
		{name: "duplicate payload", data: duplicatePayload},
		{name: "duplicate attestation", data: duplicateAttestation},
		{name: "duplicate capability", data: duplicateCapability},
		{name: "missing payload", data: missingPayload},
		{name: "missing attestation", data: missingAttestation},
		{name: "missing capability", data: missingCapability},
		{name: "null payload", data: nullPayload},
		{name: "null attestation", data: append(bytes.Clone(missingAttestation[:len(missingAttestation)-1]), []byte(`,"attestation":null}`)...)},
		{name: "null capability", data: nullCapability},
		{name: "payload has scalar type", data: wrongPayloadType},
		{name: "attestation has scalar type", data: wrongAttestationType},
		{name: "capability has scalar type", data: wrongCapabilityType},
		{name: "truncated after opening brace", data: []byte(`{`)},
		{name: "truncated after payload name", data: []byte(`{"payload":`)},
		{name: "truncated canonical grant", data: encoded[:len(encoded)-1]},
		{name: "second grant trails canonical value", data: append(bytes.Clone(encoded), encoded...)},
		{name: "one byte above document ceiling", data: leftPadJSON(encoded, GrantDocumentJSONMaximumBytes+1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			receiver := fixture.document
			if err := receiver.UnmarshalJSON(tc.data); !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("GrantDocument.UnmarshalJSON(%s) error = %v, want errors.Is %v",
					tc.name, err, core.ErrJSONContract)
			}
			gotCommitment, err := receiver.Capability.Commitment()
			if err != nil || gotCommitment != originalCommitment ||
				receiver.Payload != fixture.document.Payload ||
				receiver.Attestation != fixture.document.Attestation {
				t.Fatalf("GrantDocument.UnmarshalJSON(%s) mutated the preserved receiver", tc.name)
			}
		})
	}
}

// TestIssueGrantReturnsNeutralOnEveryInvalidIngress proves issuance never
// returns a partially usable bearer beside a refusal.
func TestIssueGrantReturnsNeutralOnEveryInvalidIngress(t *testing.T) {
	t.Parallel()

	fixture := newGrantFixture(t, grantFixtureRequest{})
	substitute := testCapabilityProjection(t, "other.json", testGrantExpiresAt)
	cases := []struct {
		name     string
		issuance GrantIssuance
	}{
		{
			name: "zero payload",
			issuance: GrantIssuance{
				Capability: fixture.projection.Capability, Signer: testGrantSigner(t),
			},
		},
		{
			name: "nil signer",
			issuance: GrantIssuance{
				Payload: fixture.payload, Capability: fixture.projection.Capability,
			},
		},
		{
			name: "capability commitment substitution",
			issuance: GrantIssuance{
				Payload: fixture.payload, Capability: substitute, Signer: testGrantSigner(t),
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			projection, err := IssueGrant(tc.issuance)
			if !errors.Is(err, core.ErrControlPlaneContract) || !projection.Capability.IsZero() {
				t.Fatalf("IssueGrant(%s) capability zero = %t, error = %v, want true and errors.Is %v",
					tc.name, projection.Capability.IsZero(), err, core.ErrControlPlaneContract)
			}
		})
	}
}

func testGrantSigner(t *testing.T) ed25519.PrivateKey {
	t.Helper()

	_, signer := testSigningKey(t, 0x41)
	return signer
}

// TestGrantLifetimeClosesBothOneNanosecondBoundaries proves a grant is current
// at issuance and one nanosecond before expiry, but neither before issuance nor
// at expiry.
func TestGrantLifetimeClosesBothOneNanosecondBoundaries(t *testing.T) {
	t.Parallel()

	fixture := newGrantFixture(t, grantFixtureRequest{})
	cases := []struct {
		name       string
		observedAt int64
		wantErr    bool
	}{
		{name: "one before issuance", observedAt: testGrantIssuedAt - 1, wantErr: true},
		{name: "at issuance", observedAt: testGrantIssuedAt},
		{name: "one before expiry", observedAt: testGrantExpiresAt - 1},
		{name: "at expiry", observedAt: testGrantExpiresAt, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			verified, err := VerifyGrant(GrantExpectation{
				Document: fixture.document, Request: fixture.request,
				ObservedAt:  temporal.InstantFromNanoseconds(tc.observedAt),
				TrustedKeys: fixture.trusted,
			})
			if tc.wantErr {
				if !errors.Is(err, core.ErrControlPlaneResponseBinding) {
					t.Fatalf("VerifyGrant(%d) = (%v, %v), want zero and errors.Is %v",
						tc.observedAt, verified, err, core.ErrControlPlaneResponseBinding)
				}
				return
			}
			if err != nil {
				t.Fatalf("VerifyGrant(%d) error = %v, want nil", tc.observedAt, err)
			}
		})
	}
}

// TestSubmissionSchemaLayerTriadZeroValuesRefuseEveryBoundary is the neutral proof: no empty
// declaration, nonce, request commitment, grant, bearer, or verified proof can
// cross a boundary and acquire authority from defaults.
func TestSubmissionSchemaLayerTriadZeroValuesRefuseEveryBoundary(t *testing.T) {
	t.Parallel()

	cases := []struct {
		run     func() error
		wantErr error
		name    string
	}{
		{name: "declaration", wantErr: core.ErrControlPlaneContract, run: func() error { return (Declaration{}).Validate() }},
		{name: "request payload", wantErr: core.ErrControlPlaneContract, run: func() error { return (RequestPayload{}).Validate() }},
		{name: "request commitment", wantErr: core.ErrControlPlaneContract, run: func() error { return (RequestCommitment{}).Validate() }},
		{name: "authorization nonce", wantErr: core.ErrControlWireNonce, run: func() error { return (controlwire.AuthorityNonce{}).Validate() }},
		{name: "grant payload", wantErr: core.ErrControlPlaneContract, run: func() error { return (GrantPayload{}).Validate() }},
		{name: "grant document", wantErr: core.ErrControlPlaneContract, run: func() error { return (GrantDocument{}).Validate() }},
		{name: "grant projection", wantErr: core.ErrControlPlaneContract, run: func() error { return (GrantProjection{}).Validate() }},
		{name: "verified grant", wantErr: core.ErrControlPlaneContract, run: func() error { return (VerifiedGrant{}).Validate() }},
		{name: "nil authorization nonce JSON receiver", run: func() error {
			var receiver *controlwire.AuthorityNonce
			return receiver.UnmarshalJSON([]byte(`"` + strings.Repeat("1", 64) + `"`))
		}, wantErr: core.ErrControlWireNonce},
		{name: "nil grant payload JSON receiver", run: func() error {
			var receiver *GrantPayload
			return receiver.UnmarshalJSON([]byte(`{}`))
		}, wantErr: core.ErrControlPlaneContract},
		{name: "nil grant document JSON receiver", run: func() error {
			var receiver *GrantDocument
			return receiver.UnmarshalJSON([]byte(`{}`))
		}, wantErr: core.ErrControlPlaneContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := tc.run(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("zero %s error = %v, want errors.Is %v", tc.name, err, tc.wantErr)
			}
		})
	}

	zeroDigestJSON := []byte(`"` + strings.Repeat("0", 64) + `"`)
	var requestCommitment RequestCommitment
	if err := json.Unmarshal(zeroDigestJSON, &requestCommitment); !errors.Is(err, core.ErrControlPlaneContract) {
		t.Fatalf("json.Unmarshal(all-zero RequestCommitment) error = %v, want errors.Is %v",
			err, core.ErrControlPlaneContract)
	}
	var authorization controlwire.AuthorityNonce
	if err := json.Unmarshal(zeroDigestJSON, &authorization); !errors.Is(err, core.ErrControlWireNonce) {
		t.Fatalf("json.Unmarshal(all-zero AuthorizationNonce) error = %v, want errors.Is %v",
			err, core.ErrControlWireNonce)
	}
}
