package runworkspace

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/projectstandards"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/temporal"
)

type VerifiedSource struct {
	Coordinate  projectstandards.SourceCoordinate
	Checkout    core.RelativePath
	Files       uint32
	Directories uint32
}

func (s VerifiedSource) Validate() error {
	if s.Files+s.Directories == 0 {
		return core.ErrPrimitiveContract
	}
	return errors.Join(s.Coordinate.Validate(), s.Checkout.Validate())
}

func (m Manager) AcquireSourceArchive(ctx context.Context, unit Unit, grant runnercontrol.SourceGrant, document runnercontrol.SourceArchiveDocument, trusted attest.TrustedKeys, observedAt temporal.Instant, source io.Reader) (verified VerifiedSource, err error) {
	if err := m.authorizeSourceArchive(unit, grant, document, trusted, observedAt, source); err != nil {
		return VerifiedSource{}, err
	}
	checkout, err := m.prepareCheckout(ctx, unit)
	if err != nil {
		return VerifiedSource{}, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, filestore.RemoveTree(ctx, filestore.TreeRemovalRequest{Location: filestore.Location{Root: m.root, Path: checkout}}))
		}
	}()
	extraction, err := newSourceExtraction(checkout, document, source)
	if err != nil {
		return VerifiedSource{}, err
	}
	if err := extraction.extract(ctx, m); err != nil {
		return VerifiedSource{}, err
	}
	if err := extraction.complete(ctx, m); err != nil {
		return VerifiedSource{}, err
	}
	verified = VerifiedSource{Coordinate: projectstandards.SourceCoordinate{Repository: document.Manifest.Repository, Commit: document.Manifest.Commit, Tree: document.Manifest.Tree}, Checkout: checkout, Files: extraction.fileCount, Directories: extraction.directoryCount}
	return verified, verified.Validate()
}

func (m Manager) authorizeSourceArchive(unit Unit, grant runnercontrol.SourceGrant, document runnercontrol.SourceArchiveDocument, trusted attest.TrustedKeys, observedAt temporal.Instant, source io.Reader) error {
	if err := errors.Join(m.Validate(), unit.Validate(), grant.Validate(), document.Validate(), observedAt.Validate()); err != nil || unit.RootIdentity != m.rootIdentity || core.ReaderIsNil(source) {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	if err := validateSourceAuthorization(grant, document.Manifest, observedAt); err != nil {
		return err
	}
	return runnercontrol.VerifySourceArchive(document, trusted)
}

func (m Manager) prepareCheckout(ctx context.Context, unit Unit) (core.RelativePath, error) {
	if err := unit.Validate(); err != nil {
		return core.RelativePath{}, err
	}
	if err := filestore.EnsureDirectory(ctx, filestore.DirectoryRequest{Location: filestore.Location{Root: m.root, Path: unit.Checkout}, Mode: workspaceDirectoryMode}); err != nil {
		return core.RelativePath{}, err
	}
	return unit.Checkout, nil
}

type sourceExtraction struct {
	checkout       core.RelativePath
	document       runnercontrol.SourceArchiveDocument
	exact          *objectstore.ExactReader
	archive        io.Reader
	reader         *tar.Reader
	archiveHash    hash.Hash
	treeHash       hash.Hash
	fileCount      uint32
	directoryCount uint32
	previous       string
	entryCount     uint32
}

func newSourceExtraction(checkout core.RelativePath, document runnercontrol.SourceArchiveDocument, source io.Reader) (*sourceExtraction, error) {
	exact, err := objectstore.NewExactReader(source, document.Manifest.ArchiveBytes)
	if err != nil {
		return nil, err
	}
	archiveHash := sha256.New()
	archive := io.TeeReader(exact, archiveHash)
	return &sourceExtraction{
		checkout: checkout, document: document, exact: exact, archive: archive,
		reader: tar.NewReader(archive), archiveHash: archiveHash, treeHash: sha256.New(),
		directoryCount: 1,
	}, nil
}

func (e *sourceExtraction) extract(ctx context.Context, manager Manager) error {
	for {
		header, err := e.reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return errors.Join(core.ErrPrimitiveContract, err)
		}
		if err := e.consume(ctx, manager, header); err != nil {
			return err
		}
	}
}

func (e *sourceExtraction) consume(ctx context.Context, manager Manager, header *tar.Header) error {
	if e.entryCount >= e.document.Manifest.EntryMaximum {
		return core.ErrPrimitiveContract
	}
	e.entryCount++
	path, err := archiveEntryPath(e.checkout, header.Name, e.document.Manifest.DepthMaximum)
	if err != nil || e.previous >= path.String() {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	e.previous = path.String()
	switch header.Typeflag {
	case tar.TypeDir:
		return e.consumeDirectory(ctx, manager, path, header)
	case tar.TypeReg, tar.TypeRegA:
		return e.consumeFile(ctx, manager, path, header)
	default:
		return core.ErrPrimitiveContract
	}
}

func (e *sourceExtraction) consumeDirectory(ctx context.Context, manager Manager, path core.RelativePath, header *tar.Header) error {
	if header.Size != 0 {
		return core.ErrPrimitiveContract
	}
	if err := filestore.EnsureDirectory(ctx, filestore.DirectoryRequest{Location: filestore.Location{Root: manager.root, Path: path}, Mode: workspaceDirectoryMode}); err != nil {
		return err
	}
	e.directoryCount++
	return writeTreeEntry(e.treeHash, path, 0o500, 0, core.SHA256Of(nil))
}

func (e *sourceExtraction) consumeFile(ctx context.Context, manager Manager, path core.RelativePath, header *tar.Header) error {
	maximum, err := e.document.Manifest.FileMaximumBytes.Uint64()
	if err != nil || header.Size < 0 || uint64(header.Size) > maximum {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	fileHash := sha256.New()
	if err := e.writeFile(ctx, manager, path, header, fileHash); err != nil {
		return err
	}
	mode := archiveFileMode(header)
	if err := writeTreeEntry(e.treeHash, path, mode, uint64(header.Size), digestFromHash(fileHash)); err != nil {
		return err
	}
	e.fileCount++
	return filestore.SetPermissions(ctx, filestore.PermissionRequest{Location: filestore.Location{Root: manager.root, Path: path}, Mode: mode})
}

func (e *sourceExtraction) writeFile(ctx context.Context, manager Manager, path core.RelativePath, header *tar.Header, fileHash hash.Hash) error {
	temporary, err := archiveTemporary(path, e.entryCount)
	if err != nil {
		return err
	}
	writeMaximum := uint64(header.Size)
	if writeMaximum == 0 {
		writeMaximum = 1
	}
	limit, err := core.NewByteCount(writeMaximum)
	if err != nil {
		return err
	}
	_, err = filestore.Write(ctx, filestore.WriteRequest{Source: io.TeeReader(e.reader, fileHash), Location: filestore.Location{Root: manager.root, Path: path}, Temporary: temporary, Mode: 0o600, Install: filestore.InstallCreate, MaximumBytes: limit})
	return err
}

func archiveFileMode(header *tar.Header) fs.FileMode {
	if header.FileInfo().Mode()&0o111 != 0 {
		return 0o500
	}
	return 0o400
}

func (e *sourceExtraction) complete(ctx context.Context, manager Manager) error {
	if err := drainArchiveTail(e.archive); err != nil {
		return err
	}
	if err := e.exact.ProveEmpty(); err != nil {
		return err
	}
	if digestFromHash(e.archiveHash) != e.document.Manifest.ArchiveDigest || digestFromHash(e.treeHash) != e.document.Manifest.Tree {
		return core.ErrPrimitiveContract
	}
	return sealCheckout(ctx, manager.root, e.checkout, e.document.Manifest.EntryMaximum)
}

func validateSourceAuthorization(grant runnercontrol.SourceGrant, manifest runnercontrol.SourceArchiveManifest, observedAt temporal.Instant) error {
	want := projectstandards.SourceCoordinate{Repository: manifest.Repository, Commit: manifest.Commit, Tree: manifest.Tree}
	if grant.Source != want {
		return core.ErrPrimitiveContract
	}
	grantComparison, grantErr := observedAt.Compare(grant.ExpiresAt)
	documentComparison, documentErr := observedAt.Compare(manifest.ExpiresAt)
	issuedComparison, issuedErr := observedAt.Compare(manifest.IssuedAt)
	if grantErr != nil || documentErr != nil || issuedErr != nil || grantComparison != core.ComparisonLess || documentComparison != core.ComparisonLess || issuedComparison == core.ComparisonLess {
		return errors.Join(core.ErrPrimitiveContract, grantErr, documentErr, issuedErr)
	}
	return nil
}

func archiveEntryPath(checkout core.RelativePath, name string, depthMaximum uint16) (core.RelativePath, error) {
	if name == "" || strings.HasPrefix(name, "./") || strings.HasPrefix(name, "/") {
		return core.RelativePath{}, core.ErrPrimitiveContract
	}
	trimmed := strings.TrimSuffix(name, "/")
	depth := strings.Count(trimmed, "/") + 1
	if depth > int(depthMaximum) {
		return core.RelativePath{}, core.ErrPrimitiveContract
	}
	relative, err := core.ParseRelativePath(trimmed)
	if err != nil || relative.String() == "." {
		return core.RelativePath{}, errors.Join(core.ErrPrimitiveContract, err)
	}
	componentPath, err := core.ParseRelativePath(filepath.Join(checkout.String(), relative.String()))
	if err != nil {
		return core.RelativePath{}, err
	}
	return componentPath, nil
}
func archiveTemporary(path core.RelativePath, index uint32) (core.RelativePath, error) {
	parent, err := core.ParseRelativePath(filepath.Dir(path.String()))
	if err != nil {
		return core.RelativePath{}, err
	}
	component, err := core.ParsePathComponent(fmt.Sprintf(".anvil-stage-%08x", index))
	if err != nil {
		return core.RelativePath{}, err
	}
	return parent.Join(component)
}
func writeTreeEntry(destination hash.Hash, path core.RelativePath, mode fs.FileMode, size uint64, digest core.SHA256Digest) error {
	digestHex, err := digest.Hex()
	if err != nil {
		return err
	}
	fields := [][]byte{[]byte(path.String()), []byte(mode.String()), []byte(digestHex)}
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], size)
	fields = append(fields, length[:])
	for _, field := range fields {
		var frame [8]byte
		binary.BigEndian.PutUint64(frame[:], uint64(len(field)))
		if _, err := destination.Write(frame[:]); err != nil {
			return err
		}
		if _, err := destination.Write(field); err != nil {
			return err
		}
	}
	return nil
}
func digestFromHash(source hash.Hash) core.SHA256Digest {
	var digest [core.SHA256DigestBytes]byte
	copy(digest[:], source.Sum(nil))
	return core.NewSHA256Digest(digest)
}
func drainArchiveTail(source io.Reader) error {
	buffer := make([]byte, 32*1024)
	for {
		count, err := source.Read(buffer)
		for _, value := range buffer[:count] {
			if value != 0 {
				return core.ErrPrimitiveContract
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if count == 0 {
			return core.ErrPrimitiveContract
		}
	}
}
func sealCheckout(ctx context.Context, root *os.Root, checkout core.RelativePath, entryMaximum uint32) error {
	maximum, err := filestore.NewDirectoryEntryMaximum(entryMaximum)
	if err != nil {
		return err
	}
	err = filestore.Walk(ctx, filestore.WalkRequest{
		Location: filestore.Location{Root: root, Path: checkout}, Order: filestore.WalkOrderLexical, DirectoryEntryMaximum: maximum,
		Visit: func(entry filestore.WalkEntry) (filestore.WalkDirective, error) {
			if !entry.Entry.IsDir() {
				return filestore.WalkContinue, nil
			}
			permissionErr := filestore.SetPermissions(ctx, filestore.PermissionRequest{Location: filestore.Location{Root: root, Path: entry.Path}, Mode: 0o500})
			return filestore.WalkContinue, permissionErr
		},
	})
	if err != nil {
		return err
	}
	return filestore.SetPermissions(ctx, filestore.PermissionRequest{Location: filestore.Location{Root: root, Path: checkout}, Mode: 0o500})
}

var _ core.Validatable = VerifiedSource{}
