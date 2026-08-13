package deploy_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"hash/crc32"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/deploy"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/release"
	"github.com/deliri/primitive/v2026/temporal"
)

type deployFixture struct {
	payloads [release.PublicationObjectCount][]byte
	plan     deploy.ReleasePlan
}

// loopbackProvider is the real half of the provider proof: an actual TLS
// server the publication reaches over a real client and server HTTP exchange.
// Transport dialing is redirected so the signed capabilities keep their
// vendor-controlled production hosts, the same boundary rule the objectstore
// suite established. The recordingTransport injector below survives only for
// narrow failure and rejection shaping, where fabricating a transport loss or
// proving zero requests is the point; the claimed capability itself must
// cross a real exchange.
type loopbackProvider struct {
	contentTypes []string
	mu           sync.Mutex
}

func (p *loopbackProvider) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if _, err := io.Copy(io.Discard, request.Body); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	p.mu.Lock()
	p.contentTypes = append(p.contentTypes, request.Header.Get("Content-Type"))
	generation := len(p.contentTypes)
	p.mu.Unlock()
	writer.Header().Set("x-goog-generation", strconv.Itoa(generation))
	writer.WriteHeader(http.StatusOK)
}

func (p *loopbackProvider) recorded() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.contentTypes...)
}

func deployLoopbackClient(t *testing.T, handler http.Handler) objectstore.Client {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	serverAddress := strings.TrimPrefix(server.URL, "https://")
	transport := server.Client().Transport.(*http.Transport).Clone()
	transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	transport.TLSClientConfig.ServerName = "example.com"
	dialer := &net.Dialer{}
	transport.DialContext = func(ctx context.Context, network string, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, serverAddress)
	}
	t.Cleanup(transport.CloseIdleConnections)
	return deployObjectstoreClient(t, transport)
}

type recordingTransport struct {
	contentTypes [release.PublicationObjectCount]string
	failAt       int
	requests     int
}

func (t *recordingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	index := t.requests
	t.requests++
	if index == t.failAt {
		return nil, errors.New("injected transport loss")
	}
	if index >= len(t.contentTypes) {
		return nil, errors.New("unexpected upload count")
	}
	t.contentTypes[index] = request.Header.Get("Content-Type")
	if _, err := io.Copy(io.Discard, request.Body); err != nil {
		return nil, err
	}
	if err := request.Body.Close(); err != nil {
		return nil, err
	}
	headers := make(http.Header)
	headers.Set("x-goog-generation", strconv.Itoa(index+1))
	return &http.Response{
		StatusCode: http.StatusOK, Header: headers, Body: http.NoBody,
		ContentLength: 0, Request: request,
	}, nil
}

func TestReleaseGCSUploadsExactManifestOrderAndReturnsReceipts(t *testing.T) {
	t.Parallel()

	fixture := newDeployFixture(t)
	provider := &loopbackProvider{}
	client := deployLoopbackClient(t, provider)
	receipts, err := deploy.ReleaseGCS(context.Background(), client, fixture.plan)
	if err != nil {
		t.Fatalf("deploy.ReleaseGCS() error = %v", err)
	}
	served := provider.recorded()
	if receipts.Count() != release.PublicationObjectCount || len(served) != release.PublicationObjectCount {
		t.Fatalf("deployed counts = receipts %d requests %d, want %d", receipts.Count(), len(served), release.PublicationObjectCount)
	}
	wantContentTypes := [...]string{
		"application/octet-stream", "application/octet-stream", "application/octet-stream", "application/octet-stream",
		"application/json", "application/json", "application/zip", "text/markdown; charset=utf-8",
	}
	for index := range release.PublicationObjectCount {
		receipt, ok := receipts.At(index)
		if !ok {
			t.Fatalf("Receipts.At(%d) ok = false", index)
		}
		if receipt.Role() != release.PublicationRole(index+1) {
			t.Fatalf("Receipts.At(%d).Role() = %v, want %v", index, receipt.Role(), release.PublicationRole(index+1))
		}
		if receipt.Transfer().Commitment() != objectstore.CommitmentConfirmed {
			t.Fatalf("Receipts.At(%d) commitment = %v, want confirmed", index, receipt.Transfer().Commitment())
		}
		version, ok := receipt.Transfer().Version()
		if !ok || version.String() != strconv.Itoa(index+1) {
			t.Fatalf("Receipts.At(%d) version = (%q, %t)", index, version.String(), ok)
		}
		if served[index] != wantContentTypes[index] {
			t.Fatalf("request %d content type = %q, want %q", index, served[index], wantContentTypes[index])
		}
	}
}

func TestReleaseGCSStopsOnceAndPreservesConfirmedPrefix(t *testing.T) {
	t.Parallel()

	fixture := newDeployFixture(t)
	transport := &recordingTransport{failAt: 3}
	client := deployObjectstoreClient(t, transport)
	receipts, err := deploy.ReleaseGCS(context.Background(), client, fixture.plan)
	if receipts.Count() != 3 || transport.requests != 4 {
		t.Fatalf("failed deployment counts = receipts %d requests %d, want 3 and 4", receipts.Count(), transport.requests)
	}
	var uploadError *deploy.UploadError
	if !errors.As(err, &uploadError) || !errors.Is(err, core.ErrDeployContract) ||
		uploadError.Role != release.PublicationRoleLinuxARM64 ||
		uploadError.Transfer.Commitment() != objectstore.CommitmentIndeterminate {
		t.Fatalf("deploy.ReleaseGCS() error = %#v, want indeterminate Linux ARM64 UploadError", err)
	}
}

func TestPrepareReleaseRejectsCrossObjectCapabilityAndIntegrityReuse(t *testing.T) {
	t.Parallel()

	fixture := newDeployFixtureRequest(t)
	fixture.Items[1] = fixture.Items[0]
	if _, err := deploy.PrepareRelease(fixture); !errors.Is(err, core.ErrDeployContract) {
		t.Fatalf("deploy.PrepareRelease(duplicate capability) error = %v, want %v", err, core.ErrDeployContract)
	}

	fixture = newDeployFixtureRequest(t)
	wrong, err := deploy.NewUploadItem(deploy.UploadItemRequest{
		Source: bytes.NewReader([]byte("wrong")), Capability: fixtureCapability(t, 41),
		Commitment: fixtureCommitment(t, fixtureCapability(t, 41)),
		Integrity:  fixtureReleaseIntegrity(t, []byte("wrong")), Role: release.PublicationRoleWindowsAMD64,
	})
	if err != nil {
		t.Fatalf("deploy.NewUploadItem(wrong integrity setup) error = %v", err)
	}
	fixture.Items[0] = wrong
	if _, err := deploy.PrepareRelease(fixture); !errors.Is(err, core.ErrDeployContract) {
		t.Fatalf("deploy.PrepareRelease(wrong manifest integrity) error = %v, want %v", err, core.ErrDeployContract)
	}
}

func TestObjectRoleExhaustsEveryUint8Value(t *testing.T) {
	t.Parallel()

	for raw := range 256 {
		role := release.PublicationRole(raw)
		wantValid := raw >= int(release.PublicationRoleWindowsAMD64) && raw <= int(release.PublicationRoleReleaseNotes)
		if got := role.IsValid(); got != wantValid {
			t.Fatalf("release.PublicationRole(%d).IsValid() = %t, want %t", raw, got, wantValid)
		}
		if !wantValid && !errors.Is(role.Validate(), core.ErrReleaseContract) {
			t.Fatalf("release.PublicationRole(%d).Validate() error = %v, want %v", raw, role.Validate(), core.ErrReleaseContract)
		}
	}
}

func newDeployFixture(t *testing.T) deployFixture {
	t.Helper()
	request := newDeployFixtureRequest(t)
	plan, err := deploy.PrepareRelease(request)
	if err != nil {
		t.Fatalf("deploy.PrepareRelease() error = %v", err)
	}
	var payloads [release.PublicationObjectCount][]byte
	for index := range release.PublicationObjectCount {
		payloads[index] = fixturePayload(index)
	}
	payloads[release.TargetCount], err = json.Marshal(request.Manifest.Document())
	if err != nil {
		t.Fatalf("json.Marshal(ManifestDocument) error = %v", err)
	}
	return deployFixture{plan: plan, payloads: payloads}
}

func newDeployFixtureRequest(t *testing.T) deploy.ReleasePlanRequest {
	t.Helper()
	manifest := fixtureVerifiedManifest(t)
	manifestBytes, err := json.Marshal(manifest.Document())
	if err != nil {
		t.Fatalf("json.Marshal(ManifestDocument) error = %v", err)
	}
	var items [release.PublicationObjectCount]deploy.UploadItem
	for index := range release.PublicationObjectCount {
		payload := fixturePayload(index)
		if index == release.TargetCount {
			payload = manifestBytes
		}
		capability := fixtureCapability(t, index)
		item, err := deploy.NewUploadItem(deploy.UploadItemRequest{
			Source: bytes.NewReader(payload), Capability: capability,
			Commitment: fixtureCommitment(t, capability), Integrity: fixtureReleaseIntegrity(t, payload),
			Role: release.PublicationRole(index + 1),
		})
		if err != nil {
			t.Fatalf("deploy.NewUploadItem(%d) error = %v", index, err)
		}
		items[index] = item
	}
	return deploy.ReleasePlanRequest{Manifest: manifest, Items: items, Policy: fixturePolicy(t)}
}

func fixtureVerifiedManifest(t *testing.T) release.VerifiedManifest {
	t.Helper()
	version := core.NewReleaseVersion(2026, 0, 11)
	commit, _ := core.ParseBuildCommit("b5c32d95d212b0a1a8cef4126e4d11ff288079ef")
	targets := release.Targets()
	var artifacts [release.TargetCount]release.Artifact
	for index := range release.TargetCount {
		platform, _ := targets.At(index)
		build, _ := core.NewBuildIdentity(core.BuildIdentityRequest{
			Offering: core.OfferingBug, Version: version, Commit: commit, Platform: platform,
		})
		payload := fixturePayload(index)
		integrity := fixtureIntegrity(t, payload)
		extent, _ := core.NewByteCount(integrity.Length.Uint64())
		artifact, err := release.NewArtifact(release.ArtifactRequest{
			Build: build, Extent: extent, SHA256: integrity.SHA256, CRC32C: integrity.CRC32C,
		})
		if err != nil {
			t.Fatalf("release.NewArtifact(%d) error = %v", index, err)
		}
		artifacts[index] = artifact
	}
	artifactSet, err := release.NewArtifactSet(release.ArtifactSetRequest{Artifacts: artifacts})
	if err != nil {
		t.Fatalf("release.NewArtifactSet() error = %v", err)
	}
	var metadata [release.MetadataAssetCount]release.MetadataAsset
	for index := range release.MetadataAssetCount {
		payload := fixturePayload(index + release.TargetCount + 1)
		integrity := fixtureIntegrity(t, payload)
		extent, _ := core.NewByteCount(integrity.Length.Uint64())
		asset, err := release.NewMetadataAsset(release.MetadataAssetRequest{
			Kind: release.MetadataKind(index + 1), Extent: extent,
			SHA256: integrity.SHA256, CRC32C: integrity.CRC32C,
		})
		if err != nil {
			t.Fatalf("release.NewMetadataAsset(%d) error = %v", index, err)
		}
		metadata[index] = asset
	}
	metadataSet, err := release.NewMetadataSet(release.MetadataSetRequest{Assets: metadata})
	if err != nil {
		t.Fatalf("release.NewMetadataSet() error = %v", err)
	}
	provenance := fixtureProvenance(t)
	fact, err := release.NewManifestFact(release.ManifestFactRequest{
		Revision: release.Revision2026V1, Offering: core.OfferingBug,
		Version: version, Commit: commit, CreatedAt: temporal.InstantFromNanoseconds(1_000),
		Artifacts: artifactSet, Provenance: provenance, Metadata: metadataSet,
	})
	if err != nil {
		t.Fatalf("release.NewManifestFact() error = %v", err)
	}
	key := fixtureSigningKey()
	document, err := release.IssueManifest(release.IssueManifestRequest{Signer: key, Fact: fact})
	if err != nil {
		t.Fatalf("release.IssueManifest() error = %v", err)
	}
	public, _ := core.NewEd25519PublicKey(key.Public().(ed25519.PublicKey))
	trusted, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{Keys: []core.Ed25519PublicKey{public}})
	if err != nil {
		t.Fatalf("attest.NewTrustedKeys() error = %v", err)
	}
	verified, err := release.VerifyManifest(release.VerifyManifestRequest{
		Document: document, TrustedKeys: trusted, ExpectedOffering: core.OfferingBug,
	})
	if err != nil {
		t.Fatalf("release.VerifyManifest() error = %v", err)
	}
	return verified
}

func fixtureProvenance(t *testing.T) release.BuildProvenance {
	t.Helper()
	goDigest := sha256.Sum256([]byte("go tool"))
	garbleDigest := sha256.Sum256([]byte("garble tool"))
	wire := struct {
		GarbleDerivation       string            `json:"garble_derivation"`
		GarbleModule           string            `json:"garble_module"`
		GarbleVersion          string            `json:"garble_version"`
		GarbleRevision         string            `json:"garble_revision"`
		GarbleModuleSum        string            `json:"garble_module_sum"`
		GarbleLiterals         string            `json:"garble_literals"`
		GarbleDiagnostics      string            `json:"garble_diagnostics"`
		GoToolchain            string            `json:"go_toolchain"`
		MainPackage            string            `json:"main_package"`
		ModuleMode             string            `json:"module_mode"`
		LinkerAssignments      []struct{}        `json:"linker_assignments"`
		GoExecutableSHA256     core.SHA256Digest `json:"go_executable_sha256"`
		GarbleExecutableSHA256 core.SHA256Digest `json:"garble_executable_sha256"`
	}{
		GoToolchain: "go1.26.5", GoExecutableSHA256: core.NewSHA256Digest(goDigest),
		GarbleModule: "mvdan.cc/garble", GarbleVersion: "v0.16.1-0.20260621195108-ffa2daf72f03",
		GarbleRevision:         "ffa2daf72f036d7ff72f6a3c8243997f06fa7b4e",
		GarbleModuleSum:        "h1:3/JEpDf12w/71XWzIrnLazgTQD6UWElzrRQWo4oJ7s0=",
		GarbleExecutableSHA256: core.NewSHA256Digest(garbleDigest),
		GarbleLiterals:         "obfuscate", GarbleDiagnostics: "preserve", GarbleDerivation: "one",
		MainPackage: "github.com/offGridSoft/bug/cmd/bug", ModuleMode: "vendor",
		LinkerAssignments: []struct{}{},
	}
	encoded, err := json.Marshal(wire)
	if err != nil {
		t.Fatalf("json.Marshal(provenance fixture) error = %v", err)
	}
	var provenance release.BuildProvenance
	if err := json.Unmarshal(encoded, &provenance); err != nil {
		t.Fatalf("json.Unmarshal(BuildProvenance) error = %v", err)
	}
	return provenance
}

func fixtureCapability(t *testing.T, index int) objectstore.UploadCapability {
	t.Helper()
	target := "https://storage.googleapis.com/bucket/object-" + strconv.Itoa(index) +
		"?X-Goog-Signature=signature&X-Goog-SignedHeaders=" +
		url.QueryEscape("host;x-goog-hash;x-goog-if-generation-match")
	document := struct {
		Provider  string `json:"provider"`
		Method    string `json:"method"`
		URL       string `json:"url"`
		ExpiresAt int64  `json:"expires_at"`
	}{
		Provider: "google_cloud_storage", Method: "signed_put", URL: target,
		ExpiresAt: time.Date(2035, time.January, 1, 0, 0, 0, 0, time.UTC).UnixNano(),
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("json.Marshal(capability) error = %v", err)
	}
	var capability objectstore.UploadCapability
	if err := json.Unmarshal(encoded, &capability); err != nil {
		t.Fatalf("json.Unmarshal(UploadCapability) error = %v", err)
	}
	return capability
}

func fixtureCommitment(
	t *testing.T,
	capability objectstore.UploadCapability,
) objectstore.UploadCapabilityCommitment {
	t.Helper()
	commitment, err := capability.Commitment()
	if err != nil {
		t.Fatalf("UploadCapability.Commitment() error = %v", err)
	}
	return commitment
}

func fixtureIntegrity(t *testing.T, payload []byte) objectstore.Integrity {
	t.Helper()
	digest := sha256.Sum256(payload)
	length, err := core.NewByteLength(uint64(len(payload)))
	if err != nil {
		t.Fatalf("core.NewByteLength() error = %v", err)
	}
	return objectstore.Integrity{
		Length: length, SHA256: core.NewSHA256Digest(digest),
		CRC32C: core.NewCRC32C(crc32.Checksum(payload, crc32.MakeTable(crc32.Castagnoli))),
	}
}

func fixtureReleaseIntegrity(t *testing.T, payload []byte) release.ArtifactIntegrity {
	t.Helper()
	return fixtureReleaseIntegrityFromObjectstore(t, fixtureIntegrity(t, payload))
}

func fixtureReleaseIntegrityFromObjectstore(
	t *testing.T,
	integrity objectstore.Integrity,
) release.ArtifactIntegrity {
	t.Helper()
	extent, err := core.NewByteCount(integrity.Length.Uint64())
	if err != nil {
		t.Fatalf("core.NewByteCount(release integrity) error = %v, want nil", err)
	}
	asset, err := release.NewMetadataAsset(release.MetadataAssetRequest{
		Kind: release.MetadataKindDependencies, Extent: extent,
		SHA256: integrity.SHA256, CRC32C: integrity.CRC32C,
	})
	if err != nil {
		t.Fatalf("release.NewMetadataAsset(release integrity) error = %v, want nil", err)
	}
	return asset.Integrity()
}

func fixturePayload(index int) []byte {
	return []byte("release-object-" + strconv.Itoa(index))
}

func fixtureSigningKey() ed25519.PrivateKey {
	seed := make([]byte, ed25519.SeedSize)
	for index := range seed {
		seed[index] = byte(index + 1)
	}
	return ed25519.NewKeyFromSeed(seed)
}

func fixturePolicy(t *testing.T) objectstore.Policy {
	t.Helper()
	operation, _ := temporal.DurationFromSeconds(10)
	attempt, _ := temporal.DurationFromSeconds(5)
	errorLimit, _ := core.NewByteCount(4096)
	return objectstore.Policy{
		OperationTimeout: operation, AttemptTimeout: attempt, ErrorBodyLimit: errorLimit,
	}
}

func deployObjectstoreClient(t *testing.T, transport http.RoundTripper) objectstore.Client {
	t.Helper()
	client, err := objectstore.NewClient(&http.Client{Transport: transport})
	if err != nil {
		t.Fatalf("objectstore.NewClient() error = %v", err)
	}
	return client
}
