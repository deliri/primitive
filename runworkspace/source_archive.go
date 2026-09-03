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
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/runprotocol"
	"github.com/deliri/primitive/v2026/temporal"
)

type VerifiedSource struct {
	Checkout    core.RelativePath
	Coordinate  runprotocol.SourceCoordinate
	Files       uint32
	Directories uint32
}

type SourceArchiveAcquisitionRequest struct {
	Source     io.Reader
	Grant      runnercontrol.SourceGrant
	Unit       Unit
	Document   runnercontrol.SourceArchiveDocument
	Trusted    attest.TrustedKeys
	ObservedAt temporal.Instant
}

func (r SourceArchiveAcquisitionRequest) Validate() error {
	if err := errors.Join(r.Unit.Validate(), r.Grant.Validate(), r.Document.Validate(), r.Trusted.Validate(), r.ObservedAt.Validate()); err != nil || core.ReaderIsNil(r.Source) {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	return validateSourceAuthorization(r.Grant, r.Document.Manifest, r.ObservedAt)
}

func (s VerifiedSource) Validate() error {
	if s.Files+s.Directories == 0 {
		return core.ErrPrimitiveContract
	}
	return errors.Join(s.Coordinate.Validate(), s.Checkout.Validate())
}

func (m Manager) AcquireSourceArchive(ctx context.Context, request SourceArchiveAcquisitionRequest) (verified VerifiedSource, err error) {
	if err := m.authorizeSourceArchive(request); err != nil {
		return VerifiedSource{}, err
	}
	checkout, err := m.prepareCheckout(ctx, request.Unit)
	if err != nil {
		return VerifiedSource{}, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, filestore.RemoveTree(ctx, filestore.TreeRemovalRequest{Location: filestore.Location{Root: m.root, Path: checkout}}))
		}
	}()
	extraction, err := newSourceExtraction(checkout, request.Document, request.Source)
	if err != nil {
		return VerifiedSource{}, err
	}
	if err := extraction.extract(ctx, m); err != nil {
		return VerifiedSource{}, err
	}
	if err := extraction.complete(ctx, m); err != nil {
		return VerifiedSource{}, err
	}
	manifest := request.Document.Manifest
	verified = VerifiedSource{Coordinate: runprotocol.SourceCoordinate{Repository: manifest.Repository, Commit: manifest.Commit, Tree: manifest.Tree}, Checkout: checkout, Files: extraction.fileCount, Directories: extraction.directoryCount}
	return verified, verified.Validate()
}

func (m Manager) authorizeSourceArchive(request SourceArchiveAcquisitionRequest) error {
	if err := errors.Join(m.Validate(), request.Validate()); err != nil || request.Unit.RootIdentity != m.rootIdentity {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	return runnercontrol.VerifySourceArchive(runnercontrol.SourceArchiveVerification{
		Document: request.Document, TrustedKeys: request.Trusted, ObservedAt: request.ObservedAt,
	})
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
	archive        io.Reader
	archiveHash    hash.Hash
	treeHash       hash.Hash
	exact          *objectstore.ExactReader
	reader         *tar.Reader
	checkout       core.RelativePath
	previous       string
	document       runnercontrol.SourceArchiveDocument
	fileCount      uint32
	directoryCount uint32
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
	case tar.TypeReg:
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
	return writeTreeEntry(treeEntry{destination: e.treeHash, path: path, mode: 0o500, digest: core.SHA256Of(nil)})
}

func (e *sourceExtraction) consumeFile(ctx context.Context, manager Manager, path core.RelativePath, header *tar.Header) error {
	maximum, err := e.document.Manifest.FileMaximumBytes.Uint64()
	if err != nil || header.Size < 0 || uint64(header.Size) > maximum {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	fileHash := sha256.New()
	if err := e.writeFile(ctx, sourceFileWrite{manager: manager, path: path, header: header, fileHash: fileHash}); err != nil {
		return err
	}
	mode := archiveFileMode(header)
	if err := writeTreeEntry(treeEntry{destination: e.treeHash, path: path, mode: mode, size: uint64(header.Size), digest: digestFromHash(fileHash)}); err != nil {
		return err
	}
	e.fileCount++
	return filestore.SetPermissions(ctx, filestore.PermissionRequest{Location: filestore.Location{Root: manager.root, Path: path}, Mode: mode})
}

type sourceFileWrite struct {
	fileHash hash.Hash
	header   *tar.Header
	path     core.RelativePath
	manager  Manager
}

func (e *sourceExtraction) writeFile(ctx context.Context, request sourceFileWrite) error {
	temporary, err := archiveTemporary(request.path, e.entryCount)
	if err != nil {
		return err
	}
	writeMaximum, err := core.CheckedUint64FromInt64(request.header.Size)
	if err != nil {
		return err
	}
	if writeMaximum == 0 {
		writeMaximum = 1
	}
	limit, err := core.NewByteCount(writeMaximum)
	if err != nil {
		return err
	}
	_, err = filestore.Write(ctx, filestore.WriteRequest{Source: io.TeeReader(e.reader, request.fileHash), Location: filestore.Location{Root: request.manager.root, Path: request.path}, Temporary: temporary, Mode: 0o600, Install: filestore.InstallCreate, MaximumBytes: limit})
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
	want := runprotocol.SourceCoordinate{Repository: manifest.Repository, Commit: manifest.Commit, Tree: manifest.Tree}
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
	if name == "" || strings.HasPrefix(name, "./") || strings.HasPrefix(name, "/") || strings.ContainsRune(name, '\\') {
		return core.RelativePath{}, core.ErrPrimitiveContract
	}
	trimmed := strings.TrimSuffix(name, "/")
	depth := strings.Count(trimmed, "/") + 1
	if depth > int(depthMaximum) {
		return core.RelativePath{}, core.ErrPrimitiveContract
	}
	relative, err := core.ParseRelativePath(filepath.FromSlash(trimmed))
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
	component, err := core.ParsePathComponent(fmt.Sprintf(".primitive-stage-%08x", index))
	if err != nil {
		return core.RelativePath{}, err
	}
	return parent.Join(component)
}

type treeEntry struct {
	destination hash.Hash
	path        core.RelativePath
	size        uint64
	mode        fs.FileMode
	digest      core.SHA256Digest
}

func writeTreeEntry(entry treeEntry) error {
	digestHex, err := entry.digest.Hex()
	if err != nil {
		return err
	}
	fields := [][]byte{[]byte(entry.path.String()), []byte(entry.mode.String()), []byte(digestHex)}
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], entry.size)
	fields = append(fields, length[:])
	for _, field := range fields {
		var frame [8]byte
		binary.BigEndian.PutUint64(frame[:], uint64(len(field)))
		if _, err := entry.destination.Write(frame[:]); err != nil {
			return err
		}
		if _, err := entry.destination.Write(field); err != nil {
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
