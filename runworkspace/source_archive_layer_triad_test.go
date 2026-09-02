package runworkspace

import (
	"archive/tar"
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	primitiveid "github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/standard"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestSourceArchiveEffectLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive signed archive becomes a read-only verified checkout bound to its source grant", func(t *testing.T) {
		t.Parallel()
		manager, unit := sourceWorkspaceFixture(t)
		defer closeSourceManager(t, manager)
		defer cleanupSourceUnit(t, manager, unit)
		archive, document, grant, trusted := sourceArchiveFixture(t, unit, []byte("package subject\n"))

		got, gotErr := manager.AcquireSourceArchive(t.Context(), SourceArchiveAcquisitionRequest{Unit: unit, Grant: grant, Document: document, Trusted: trusted, ObservedAt: temporal.InstantFromNanoseconds(10), Source: bytes.NewReader(archive)})
		if gotErr != nil || got.Coordinate != grant.Source || got.Files != 1 || got.Directories != 2 {
			t.Fatalf("Manager.AcquireSourceArchive() = (%+v, %v), want exact coordinate, 1 file, 2 directories, nil", got, gotErr)
		}
		filePath, pathErr := core.ParseRelativePath(got.Checkout.String() + "/pkg/main.go")
		file, openErr := filestore.OpenRead(t.Context(), filestore.ReadHandleRequest{Location: filestore.Location{Root: manager.root, Path: filePath}})
		if err := errors.Join(pathErr, openErr); err != nil {
			t.Fatalf("filestore.OpenRead(verified source) error = %v, want nil", err)
		}
		info, statErr := file.Stat()
		closeErr := file.Close()
		if statErr != nil || closeErr != nil || info.Mode().Perm() != 0o400 {
			t.Fatalf("verified source mode/stat/close = (%#o, %v, %v), want (0400, nil, nil)", info.Mode().Perm(), statErr, closeErr)
		}
	})

	t.Run("negative tampered archive is refused and its partial checkout is removed", func(t *testing.T) {
		t.Parallel()
		manager, unit := sourceWorkspaceFixture(t)
		defer closeSourceManager(t, manager)
		defer cleanupSourceUnit(t, manager, unit)
		archive, document, grant, trusted := sourceArchiveFixture(t, unit, []byte("package subject\n"))
		archive[len(archive)/2] ^= 1

		got, gotErr := manager.AcquireSourceArchive(t.Context(), SourceArchiveAcquisitionRequest{Unit: unit, Grant: grant, Document: document, Trusted: trusted, ObservedAt: temporal.InstantFromNanoseconds(10), Source: bytes.NewReader(archive)})
		if got != (VerifiedSource{}) || !errors.Is(gotErr, core.ErrPrimitiveContract) {
			t.Fatalf("Manager.AcquireSourceArchive(tampered) = (%+v, %v), want zero and errors.Is(..., %v)", got, gotErr, core.ErrPrimitiveContract)
		}
		checkout, pathErr := joinLiteral(unit.Root, "checkout")
		if pathErr != nil {
			t.Fatalf("joinLiteral(checkout) setup error = %v, want nil", pathErr)
		}
		_, openErr := filestore.OpenRead(t.Context(), filestore.ReadHandleRequest{Location: filestore.Location{Root: manager.root, Path: checkout}})
		if !errors.Is(openErr, core.ErrFilestoreSource) {
			t.Fatalf("filestore.OpenRead(removed checkout) error = %v, want errors.Is(..., %v)", openErr, core.ErrFilestoreSource)
		}
	})

	t.Run("neutral expired grant refuses before archive or checkout effects", func(t *testing.T) {
		t.Parallel()
		manager, unit := sourceWorkspaceFixture(t)
		defer closeSourceManager(t, manager)
		defer cleanupSourceUnit(t, manager, unit)
		archive, document, grant, trusted := sourceArchiveFixture(t, unit, []byte("package subject\n"))

		got, gotErr := manager.AcquireSourceArchive(t.Context(), SourceArchiveAcquisitionRequest{Unit: unit, Grant: grant, Document: document, Trusted: trusted, ObservedAt: grant.ExpiresAt, Source: bytes.NewReader(archive)})
		if got != (VerifiedSource{}) || !errors.Is(gotErr, core.ErrPrimitiveContract) {
			t.Fatalf("Manager.AcquireSourceArchive(expired grant) = (%+v, %v), want zero and errors.Is(..., %v)", got, gotErr, core.ErrPrimitiveContract)
		}
		observation, observeErr := manager.Observe(t.Context(), temporal.InstantFromNanoseconds(101), Residue{})
		if observeErr != nil || observation.Entries != 1 {
			t.Fatalf("Manager.Observe(after pre-effect refusal) = (%+v, %v), want only the scheduling unit and nil", observation, observeErr)
		}
	})
}

func FuzzSourceArchiveSemanticBoundary(f *testing.F) {
	seedArchive := sourceTarFixture(f, []byte("package subject\n"))
	f.Add(seedArchive)
	f.Add([]byte{})
	f.Add([]byte("not-a-tar"))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			return
		}
		manager, unit := sourceWorkspaceFixture(t)
		defer closeSourceManager(t, manager)
		defer cleanupSourceUnit(t, manager, unit)
		_, seedDocument, seedGrant, trusted := sourceArchiveFixture(t, unit, []byte("package subject\n"))
		manifest := seedDocument.Manifest
		manifest.ArchiveDigest = core.SHA256Of(data)
		manifest.ArchiveBytes = sourceByteLength(t, uint64(len(data)))
		key, _ := sourceSignerFixture(t)
		document, issueErr := runnercontrol.IssueSourceArchive(manifest, key)
		if issueErr != nil {
			t.Fatalf("IssueSourceArchive(fuzz input) error = %v, want nil", issueErr)
		}

		got, gotErr := manager.AcquireSourceArchive(t.Context(), SourceArchiveAcquisitionRequest{Unit: unit, Grant: seedGrant, Document: document, Trusted: trusted, ObservedAt: temporal.InstantFromNanoseconds(10), Source: bytes.NewReader(data)})
		if bytes.Equal(data, seedArchive) {
			if gotErr != nil || got.Files != 1 || got.Coordinate != seedGrant.Source {
				t.Fatalf("AcquireSourceArchive(canonical seed) = (%+v, %v), want exact verified source and nil", got, gotErr)
			}
			return
		}
		if gotErr == nil {
			if err := got.Validate(); err != nil || got.Coordinate != seedGrant.Source {
				t.Fatalf("AcquireSourceArchive(accepted fuzz input) = (%+v, nil), want valid grant-bound source; validation error %v", got, err)
			}
			return
		}
		if got != (VerifiedSource{}) {
			t.Fatalf("AcquireSourceArchive(rejected fuzz input) = (%+v, %v), want zero source beside rejection", got, gotErr)
		}
	})
}

func sourceWorkspaceFixture(t testing.TB) (Manager, Unit) {
	t.Helper()
	rootPath, rootErr := core.ParseAbsolutePath(t.TempDir())
	manager, managerErr := Open(t.Context(), Configuration{RunParent: rootPath})
	uuid, uuidErr := primitiveid.ParseUUIDv7("01890f2e-7b00-7000-8000-000000000001")
	unit, unitErr := manager.CreateUnit(t.Context(), runnercontrol.SchedulingUnitIdentity{Kind: runnercontrol.SchedulingUnitRunPlan, Identity: uuid})
	if err := errors.Join(rootErr, managerErr, uuidErr, unitErr); err != nil {
		t.Fatalf("source workspace fixture error = %v, want nil", err)
	}
	return manager, unit
}

func closeSourceManager(t testing.TB, manager Manager) {
	t.Helper()
	if err := manager.Close(); err != nil {
		t.Errorf("Manager.Close() source fixture error = %v, want nil", err)
	}
}

func cleanupSourceUnit(t testing.TB, manager Manager, unit Unit) {
	t.Helper()
	if err := manager.CleanupUnit(t.Context(), unit); err != nil {
		t.Errorf("Manager.CleanupUnit() source fixture error = %v, want nil", err)
	}
}

func sourceArchiveFixture(t testing.TB, unit Unit, content []byte) ([]byte, runnercontrol.SourceArchiveDocument, runnercontrol.SourceGrant, attest.TrustedKeys) {
	t.Helper()
	archive := sourceTarFixture(t, content)
	repository, repositoryErr := standard.NewRepositoryIdentity("github.com/example/project")
	commit, commitErr := core.ParseBuildCommit("0123456789abcdef0123456789abcdef01234567")
	checkout, checkoutErr := joinLiteral(unit.Root, "checkout")
	directoryPath, directoryErr := archiveEntryPath(checkout, "pkg/", 8)
	filePath, fileErr := archiveEntryPath(checkout, "pkg/main.go", 8)
	tree := sha256.New()
	treeDirectoryErr := writeTreeEntry(treeEntry{destination: tree, path: directoryPath, mode: 0o500, digest: core.SHA256Of(nil)})
	treeFileErr := writeTreeEntry(treeEntry{destination: tree, path: filePath, mode: 0o400, size: uint64(len(content)), digest: core.SHA256Of(content)})
	archiveBytes := sourceByteLength(t, uint64(len(archive)))
	fileMaximum, maximumErr := core.NewByteCount(1 << 20)
	if err := errors.Join(repositoryErr, commitErr, checkoutErr, directoryErr, fileErr, treeDirectoryErr, treeFileErr, maximumErr); err != nil {
		t.Fatalf("source archive manifest fixture error = %v, want nil", err)
	}
	coordinate := standard.SourceCoordinate{Repository: repository, Commit: commit, Tree: digestFromHash(tree)}
	manifest := runnercontrol.SourceArchiveManifest{SchemaVersion: runnercontrol.SchemaVersion, Repository: repository, Commit: commit, Tree: coordinate.Tree, ArchiveDigest: core.SHA256Of(archive), ArchiveBytes: archiveBytes, EntryMaximum: 4, DepthMaximum: 8, FileMaximumBytes: fileMaximum, IssuedAt: temporal.InstantFromNanoseconds(1), ExpiresAt: temporal.InstantFromNanoseconds(100)}
	key, trusted := sourceSignerFixture(t)
	document, documentErr := runnercontrol.IssueSourceArchive(manifest, key)
	authority, authorityErr := core.ParseHTTPEndpoint("https://source.example.invalid/archive")
	credential, credentialErr := standard.NewIdentifier("source-read-once")
	grant, grantErr := runnercontrol.NewSourceGrant(runnercontrol.SourceGrant{RepositoryGrant: core.SHA256Of([]byte("repository-grant")), Source: coordinate, Authority: authority, Credential: credential, ExpiresAt: temporal.InstantFromNanoseconds(100)})
	if err := errors.Join(documentErr, authorityErr, credentialErr, grantErr); err != nil {
		t.Fatalf("source archive authorization fixture error = %v, want nil", err)
	}
	return archive, document, grant, trusted
}

func sourceTarFixture(t testing.TB, content []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	w := tar.NewWriter(&buffer)
	directoryErr := w.WriteHeader(&tar.Header{Name: "pkg/", Typeflag: tar.TypeDir, Mode: 0o755})
	fileErr := w.WriteHeader(&tar.Header{Name: "pkg/main.go", Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(content))})
	_, writeErr := w.Write(content)
	closeErr := w.Close()
	if err := errors.Join(directoryErr, fileErr, writeErr, closeErr); err != nil {
		t.Fatalf("tar source fixture error = %v, want nil", err)
	}
	return buffer.Bytes()
}

func sourceSignerFixture(t testing.TB) (ed25519.PrivateKey, attest.TrustedKeys) {
	t.Helper()
	seed := sha256.Sum256([]byte("primitive-runworkspace-source-archive-test"))
	privateKey := ed25519.NewKeyFromSeed(seed[:])
	publicKey, publicErr := core.NewEd25519PublicKey(privateKey.Public().(ed25519.PublicKey))
	trusted, trustedErr := attest.NewTrustedKeys(attest.TrustedKeysRequest{Keys: []core.Ed25519PublicKey{publicKey}})
	if err := errors.Join(publicErr, trustedErr); err != nil {
		t.Fatalf("source signer fixture error = %v, want nil", err)
	}
	return privateKey, trusted
}

func sourceByteLength(t testing.TB, value uint64) core.ByteLength {
	t.Helper()
	got, err := core.NewByteLength(value)
	if err != nil {
		t.Fatalf("core.NewByteLength(%d) source fixture error = %v, want nil", value, err)
	}
	return got
}
