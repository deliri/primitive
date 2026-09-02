package runworkspace

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/temporal"
)

type SourceDownloadRequest struct {
	Client      objectstore.Client
	ContentType core.HTTPMediaType
	Grant       runnercontrol.SourceGrant
	Capability  objectstore.DownloadCapability
	Unit        Unit
	Document    runnercontrol.SourceArchiveDocument
	TrustedKeys attest.TrustedKeys
	Integrity   objectstore.Integrity
	Policy      objectstore.Policy
	ObservedAt  temporal.Instant
}

func (r SourceDownloadRequest) Validate() error {
	if err := errors.Join(r.Unit.Validate(), r.Grant.Validate(), r.Document.Validate(), r.TrustedKeys.Validate(), r.ObservedAt.Validate(), r.Client.Validate(), r.Capability.Validate(), r.Integrity.Validate(), r.ContentType.Validate(), r.Policy.Validate()); err != nil {
		return err
	}
	manifest := r.Document.Manifest
	if r.Integrity.SHA256 != manifest.ArchiveDigest || r.Integrity.Length != manifest.ArchiveBytes {
		return core.ErrPrimitiveContract
	}
	return validateSourceAuthorization(r.Grant, manifest, r.ObservedAt)
}

func (m Manager) AcquireSource(ctx context.Context, request SourceDownloadRequest) (VerifiedSource, error) {
	if err := errors.Join(m.Validate(), request.Validate()); err != nil || request.Unit.RootIdentity != m.rootIdentity {
		return VerifiedSource{}, errors.Join(core.ErrPrimitiveContract, err)
	}
	archivePath, err := joinLiteral(request.Unit.Root, ".source-archive.download")
	if err != nil {
		return VerifiedSource{}, err
	}
	staged, err := m.downloadSourceArchive(ctx, archivePath, request)
	if err != nil {
		return VerifiedSource{}, err
	}
	return m.extractDownloadedSource(ctx, archivePath, staged, request)
}

func (m Manager) downloadSourceArchive(ctx context.Context, archivePath core.RelativePath, request SourceDownloadRequest) (filestore.StagedFile, error) {
	destination, err := filestore.OpenStageDestination(ctx, filestore.StageDestinationRequest{
		Temporary:     filestore.Location{Root: m.root, Path: archivePath},
		ExpectedBytes: request.Integrity.Length,
		Mode:          0o600,
	})
	if err != nil {
		return filestore.StagedFile{}, err
	}
	file, err := destination.File()
	if err != nil {
		return filestore.StagedFile{}, errors.Join(err, filestore.AbandonStageDestination(destination))
	}
	transfer, err := objectstore.Download(ctx, request.Client, objectstore.DownloadCapabilityRequest{
		Destination: file,
		ContentType: request.ContentType,
		Capability:  request.Capability,
		Integrity:   request.Integrity,
		Policy:      request.Policy,
	})
	if err != nil {
		return filestore.StagedFile{}, errors.Join(err, filestore.AbandonStageDestination(destination))
	}
	if err := validateSourceTransfer(transfer, request); err != nil {
		return filestore.StagedFile{}, errors.Join(err, filestore.AbandonStageDestination(destination))
	}
	staged, err := filestore.FinishStageDestination(ctx, destination)
	if err != nil {
		return filestore.StagedFile{}, err
	}
	return staged, nil
}

func (m Manager) extractDownloadedSource(ctx context.Context, archivePath core.RelativePath, staged filestore.StagedFile, request SourceDownloadRequest) (VerifiedSource, error) {
	reader, err := filestore.OpenStagedRead(ctx, staged)
	if err != nil {
		return VerifiedSource{}, errors.Join(err, removeDownloadedArchive(ctx, m, archivePath))
	}
	verified, acquireErr := m.AcquireSourceArchive(ctx, SourceArchiveAcquisitionRequest{
		Unit: request.Unit, Grant: request.Grant, Document: request.Document,
		Trusted: request.TrustedKeys, ObservedAt: request.ObservedAt, Source: reader,
	})
	closeErr := reader.Close()
	removeErr := removeDownloadedArchive(ctx, m, archivePath)
	if err := errors.Join(acquireErr, closeErr, removeErr); err != nil {
		return VerifiedSource{}, err
	}
	return verified, nil
}

func validateSourceTransfer(transfer objectstore.Transfer, request SourceDownloadRequest) error {
	if err := transfer.Validate(); err != nil {
		return err
	}
	if transfer.Direction() != objectstore.DirectionDownload || transfer.SHA256() != request.Integrity.SHA256 || transfer.Bytes() != request.Integrity.Length {
		return core.ErrPrimitiveContract
	}
	return nil
}

func removeDownloadedArchive(ctx context.Context, manager Manager, path core.RelativePath) error {
	return filestore.Remove(ctx, filestore.RemovalRequest{Location: filestore.Location{Root: manager.root, Path: path}})
}

var _ core.Validatable = SourceDownloadRequest{}
