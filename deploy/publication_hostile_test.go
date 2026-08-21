package deploy_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	json "encoding/json/v2"
	"errors"
	"hash/crc32"
	"io"
	"net/http"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/deploy"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/release"
)

// TestReleaseGCSRefusesBeforeAnyObjectIsWritten proves every precondition is
// closed before the first external request, so a rejected deployment can never
// leave a partially published release behind.
func TestReleaseGCSRefusesBeforeAnyObjectIsWritten(t *testing.T) {
	t.Parallel()

	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	var absent context.Context

	cases := []struct {
		wantErr     error
		makeContext func(*testing.T) context.Context
		makeClient  func(*testing.T, http.RoundTripper) objectstore.Client
		makePlan    func(*testing.T) deploy.ReleasePlan
		name        string
	}{
		{
			name:        "nil context is refused",
			makeContext: func(*testing.T) context.Context { return absent },
			makeClient:  deployObjectstoreClient,
			makePlan:    func(t *testing.T) deploy.ReleasePlan { return newDeployFixture(t).plan },
			wantErr:     core.ErrNilContext,
		},
		{
			name:        "cancelled context is refused",
			makeContext: func(*testing.T) context.Context { return cancelled },
			makeClient:  deployObjectstoreClient,
			makePlan:    func(t *testing.T) deploy.ReleasePlan { return newDeployFixture(t).plan },
			wantErr:     context.Canceled,
		},
		{
			name:        "unset objectstore client is refused",
			makeContext: func(t *testing.T) context.Context { return t.Context() },
			makeClient: func(*testing.T, http.RoundTripper) objectstore.Client {
				return objectstore.Client{}
			},
			makePlan: func(t *testing.T) deploy.ReleasePlan { return newDeployFixture(t).plan },
			wantErr:  core.ErrDeployContract,
		},
		{
			name:        "unset release plan is refused",
			makeContext: func(t *testing.T) context.Context { return t.Context() },
			makeClient:  deployObjectstoreClient,
			makePlan:    func(*testing.T) deploy.ReleasePlan { return deploy.ReleasePlan{} },
			wantErr:     core.ErrDeployContract,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			transport := &recordingTransport{failAt: -1}
			receipts, gotErr := deploy.ReleaseGCS(
				tc.makeContext(t), tc.makeClient(t, transport), tc.makePlan(t),
			)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("deploy.ReleaseGCS() error = %v, want %v", gotErr, tc.wantErr)
			}
			if transport.requests != 0 {
				t.Fatalf("deploy.ReleaseGCS() issued %d requests, want 0 before rejection", transport.requests)
			}
			if receipts.Count() != 0 {
				t.Fatalf("deploy.ReleaseGCS() receipts = %d, want 0 before rejection", receipts.Count())
			}
			if _, ok := receipts.At(0); ok {
				t.Fatal("Receipts.At(0) ok = true, want false for an empty prefix")
			}
		})
	}
}

// TestPrepareReleaseRejectsEveryRoleSlotThatDoesNotMatchItsManifestEntry proves
// the plan binds each object to one exact manifest fact and one exact position.
func TestPrepareReleaseRejectsEveryRoleSlotThatDoesNotMatchItsManifestEntry(t *testing.T) {
	t.Parallel()

	cases := []struct {
		mutate func(*deploy.ReleasePlanRequest)
		name   string
	}{
		{
			name:   "unset manifest is rejected",
			mutate: func(r *deploy.ReleasePlanRequest) { r.Manifest = release.VerifiedManifest{} },
		},
		{
			name:   "unset objectstore policy is rejected",
			mutate: func(r *deploy.ReleasePlanRequest) { r.Policy = objectstore.Policy{} },
		},
		{
			name:   "unset first item is rejected",
			mutate: func(r *deploy.ReleasePlanRequest) { r.Items[0] = deploy.UploadItem{} },
		},
		{
			name:   "unset manifest item is rejected",
			mutate: func(r *deploy.ReleasePlanRequest) { r.Items[4] = deploy.UploadItem{} },
		},
		{
			name:   "unset last metadata item is rejected",
			mutate: func(r *deploy.ReleasePlanRequest) { r.Items[7] = deploy.UploadItem{} },
		},
		{
			name: "swapped binary slots are rejected",
			mutate: func(r *deploy.ReleasePlanRequest) {
				r.Items[0], r.Items[1] = r.Items[1], r.Items[0]
			},
		},
		{
			name: "swapped metadata slots are rejected",
			mutate: func(r *deploy.ReleasePlanRequest) {
				r.Items[5], r.Items[6] = r.Items[6], r.Items[5]
			},
		},
		{
			name: "manifest and binary slots exchanged are rejected",
			mutate: func(r *deploy.ReleasePlanRequest) {
				r.Items[3], r.Items[4] = r.Items[4], r.Items[3]
			},
		},
		{
			name: "reversed publication order is rejected",
			mutate: func(r *deploy.ReleasePlanRequest) {
				for low, high := 0, release.PublicationObjectCount-1; low < high; low, high = low+1, high-1 {
					r.Items[low], r.Items[high] = r.Items[high], r.Items[low]
				}
			},
		},
		{
			name:   "duplicated last item is rejected",
			mutate: func(r *deploy.ReleasePlanRequest) { r.Items[6] = r.Items[7] },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := newDeployFixtureRequest(t)
			tc.mutate(&request)
			got, gotErr := deploy.PrepareRelease(request)
			if !errors.Is(gotErr, core.ErrDeployContract) {
				t.Fatalf("deploy.PrepareRelease() error = %v, want %v", gotErr, core.ErrDeployContract)
			}
			if gotErr := got.Validate(); !errors.Is(gotErr, core.ErrDeployContract) {
				t.Fatalf("rejected deploy.ReleasePlan.Validate() error = %v, want %v", gotErr, core.ErrDeployContract)
			}
			if err := request.Validate(); !errors.Is(err, core.ErrDeployContract) {
				t.Fatalf("deploy.ReleasePlanRequest.Validate() error = %v, want %v", err, core.ErrDeployContract)
			}
		})
	}
}

// TestPrepareReleaseRejectsEveryManifestIntegritySubstitution proves the
// manifest source is bound to the complete canonical transfer contract before
// any preceding binary upload can begin.
func TestPrepareReleaseRejectsEveryManifestIntegritySubstitution(t *testing.T) {
	t.Parallel()

	request := newDeployFixtureRequest(t)
	manifestBytes, err := json.Marshal(request.Manifest.Document())
	if err != nil {
		t.Fatalf("json.Marshal(ManifestDocument) error = %v, want nil", err)
	}
	want := fixtureIntegrity(t, manifestBytes)
	other := append(append([]byte(nil), manifestBytes...), 1)
	otherDigest := sha256.Sum256(other)
	otherLength, err := core.NewByteLength(uint64(len(other)))
	if err != nil {
		t.Fatalf("core.NewByteLength(other manifest) error = %v, want nil", err)
	}
	otherCRC := core.NewCRC32C(crc32.Checksum(other, crc32.MakeTable(crc32.Castagnoli)))

	cases := []struct {
		name      string
		integrity release.ArtifactIntegrity
	}{
		{name: "length substitution is rejected", integrity: fixtureReleaseIntegrityFromObjectstore(t, objectstore.Integrity{
			Length: otherLength, SHA256: want.SHA256, CRC32C: want.CRC32C,
		})},
		{name: "sha256 substitution is rejected", integrity: fixtureReleaseIntegrityFromObjectstore(t, objectstore.Integrity{
			Length: want.Length, SHA256: core.NewSHA256Digest(otherDigest), CRC32C: want.CRC32C,
		})},
		{name: "crc32c substitution is rejected", integrity: fixtureReleaseIntegrityFromObjectstore(t, objectstore.Integrity{
			Length: want.Length, SHA256: want.SHA256, CRC32C: otherCRC,
		})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			candidate := request
			capability := fixtureCapability(t, release.TargetCount)
			item, itemErr := deploy.NewUploadItem(deploy.UploadItemRequest{
				Source: bytes.NewReader(manifestBytes), Capability: capability,
				Commitment: fixtureCommitment(t, capability), Integrity: tc.integrity,
				Role: release.PublicationRoleManifest,
			})
			if itemErr != nil {
				t.Fatalf("deploy.NewUploadItem(mutated manifest) error = %v, want nil", itemErr)
			}
			candidate.Items[release.TargetCount] = item
			if _, gotErr := deploy.PrepareRelease(candidate); !errors.Is(gotErr, core.ErrDeployContract) {
				t.Fatalf("deploy.PrepareRelease(mutated manifest) error = %v, want %v", gotErr, core.ErrDeployContract)
			}
		})
	}
}

// TestUploadItemRejectsEveryUnboundCapabilityOrSource closes the per-object
// boundary: the source, the grant, and the separately authenticated commitment
// must all be present and must name the same capability.
func TestUploadItemRejectsEveryUnboundCapabilityOrSource(t *testing.T) {
	t.Parallel()

	capability := fixtureCapability(t, 0)
	other := fixtureCapability(t, 99)

	cases := []struct {
		name    string
		request deploy.UploadItemRequest
	}{
		{
			name: "nil source is rejected",
			request: deploy.UploadItemRequest{
				Capability: capability, Commitment: fixtureCommitment(t, capability),
				Integrity: fixtureReleaseIntegrity(t, fixturePayload(0)), Role: release.PublicationRoleWindowsAMD64,
			},
		},
		{
			name: "unset capability is rejected",
			request: deploy.UploadItemRequest{
				Source: sourceForTest(0), Commitment: fixtureCommitment(t, capability),
				Integrity: fixtureReleaseIntegrity(t, fixturePayload(0)), Role: release.PublicationRoleWindowsAMD64,
			},
		},
		{
			name: "unset commitment is rejected",
			request: deploy.UploadItemRequest{
				Source: sourceForTest(0), Capability: capability,
				Integrity: fixtureReleaseIntegrity(t, fixturePayload(0)), Role: release.PublicationRoleWindowsAMD64,
			},
		},
		{
			name: "commitment from another capability is rejected",
			request: deploy.UploadItemRequest{
				Source: sourceForTest(0), Capability: capability, Commitment: fixtureCommitment(t, other),
				Integrity: fixtureReleaseIntegrity(t, fixturePayload(0)), Role: release.PublicationRoleWindowsAMD64,
			},
		},
		{
			name: "unset integrity is rejected",
			request: deploy.UploadItemRequest{
				Source: sourceForTest(0), Capability: capability,
				Commitment: fixtureCommitment(t, capability), Role: release.PublicationRoleWindowsAMD64,
			},
		},
		{
			name: "unknown role is rejected",
			request: deploy.UploadItemRequest{
				Source: sourceForTest(0), Capability: capability,
				Commitment: fixtureCommitment(t, capability),
				Integrity:  fixtureReleaseIntegrity(t, fixturePayload(0)), Role: release.PublicationRoleUnknown,
			},
		},
		{
			name: "future role is rejected",
			request: deploy.UploadItemRequest{
				Source: sourceForTest(0), Capability: capability,
				Commitment: fixtureCommitment(t, capability),
				Integrity:  fixtureReleaseIntegrity(t, fixturePayload(0)),
				Role:       release.PublicationRoleReleaseNotes + 1,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := deploy.NewUploadItem(tc.request)
			if !errors.Is(gotErr, core.ErrDeployContract) {
				t.Fatalf("deploy.NewUploadItem() error = %v, want %v", gotErr, core.ErrDeployContract)
			}
			if gotErr := got.Validate(); !errors.Is(gotErr, core.ErrDeployContract) {
				t.Fatalf("rejected deploy.UploadItem.Validate() error = %v, want %v", gotErr, core.ErrDeployContract)
			}
			if gotErr := tc.request.Validate(); !errors.Is(gotErr, core.ErrDeployContract) {
				t.Fatalf("deploy.UploadItemRequest.Validate() error = %v, want %v", gotErr, core.ErrDeployContract)
			}
		})
	}
}

// TestUploadErrorKeepsEveryTypedFailureFactReachable proves the failure detail
// routes the deploy identity, the provider cause, the failed role, and the
// commitment evidence without exposing the signed capability. The rendered
// message stays a diagnostic; the typed fields are the contract.
func TestUploadErrorKeepsEveryTypedFailureFactReachable(t *testing.T) {
	t.Parallel()

	cause := errors.New("provider refused the create-only precondition")
	failure := &deploy.UploadError{Role: release.PublicationRoleDocumentation, Cause: cause}

	if !errors.Is(failure, core.ErrDeployContract) {
		t.Fatalf("errors.Is(deploy.UploadError, ErrDeployContract) = false, want true for %v", failure)
	}
	if !errors.Is(failure, core.ErrPrimitiveContract) {
		t.Fatalf("errors.Is(deploy.UploadError, ErrPrimitiveContract) = false, want true for %v", failure)
	}
	if !errors.Is(failure, cause) {
		t.Fatalf("errors.Is(deploy.UploadError, cause) = false, want true for %v", failure)
	}
	var found *deploy.UploadError
	if !errors.As(error(failure), &found) || found.Role != release.PublicationRoleDocumentation {
		t.Fatalf("errors.As(deploy.UploadError).Role = %v, want %v", found.Role, release.PublicationRoleDocumentation)
	}

	withoutCause := &deploy.UploadError{Role: release.PublicationRoleManifest}
	if !errors.Is(withoutCause, core.ErrDeployContract) {
		t.Fatalf("errors.Is(causeless deploy.UploadError, ErrDeployContract) = false, want true")
	}
	if got := withoutCause.Unwrap(); !errors.Is(got, core.ErrDeployContract) {
		t.Fatalf("causeless deploy.UploadError.Unwrap() = %v, want the deploy contract identity", got)
	}

	var nilFailure *deploy.UploadError
	if got, want := nilFailure.Error(), core.ErrDeployContract.Error(); got != want {
		t.Fatalf("nil deploy.UploadError.Error() = %q, want %q", got, want)
	}
	if got := nilFailure.Unwrap(); got != nil {
		t.Fatalf("nil deploy.UploadError.Unwrap() = %v, want nil", got)
	}
}

// TestObjectRoleLabelsNameEveryPublishedObjectExactlyOnce proves the ordered
// publication domain has no duplicate or missing label.
func TestObjectRoleLabelsNameEveryPublishedObjectExactlyOnce(t *testing.T) {
	t.Parallel()

	want := [release.PublicationObjectCount]string{
		"windows_amd64", "darwin_arm64", "linux_amd64", "linux_arm64",
		"manifest", "dependencies", "documentation", "release_notes",
	}
	seen := make(map[string]int, len(want))
	for index, label := range want {
		role := release.PublicationRole(index + 1)
		if got := role.String(); got != label {
			t.Fatalf("release.PublicationRole(%d).String() = %q, want %q", index+1, got, label)
		}
		seen[label]++
	}
	if len(seen) != release.PublicationObjectCount {
		t.Fatalf("deploy object role labels = %d distinct, want %d", len(seen), release.PublicationObjectCount)
	}
	for _, role := range []release.PublicationRole{
		release.PublicationRoleUnknown, release.PublicationRoleReleaseNotes + 1, 255,
	} {
		if got := role.String(); got != core.UnknownEnumDiagnostic {
			t.Fatalf("release.PublicationRole(%d).String() = %q, want %q", role, got, core.UnknownEnumDiagnostic)
		}
	}
	metadataRoles := [release.MetadataAssetCount]release.PublicationRole{
		release.PublicationRoleDependencies, release.PublicationRoleDocumentation, release.PublicationRoleReleaseNotes,
	}
	for index, role := range metadataRoles {
		kind := release.MetadataKind(index + 1)
		if got, want := role.String(), kind.String(); got != want {
			t.Fatalf("release.PublicationRole(%v).String() = %q, want release.MetadataKind(%d).String() = %q",
				role, got, index+1, want)
		}
	}
}

func sourceForTest(index int) io.Reader { return bytes.NewReader(fixturePayload(index)) }
