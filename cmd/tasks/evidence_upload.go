package main

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/gcsobjects"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/temporal"
)

type taskEvidenceUploadPlan struct {
	Source      core.AbsolutePath
	Bucket      gcsobjects.GCSBucket
	Name        gcsobjects.GCSObjectName
	ContentType core.HTTPMediaType
	Integrity   objectstore.Integrity
	CustomTime  temporal.Instant
}

type taskEvidenceUploadRequest struct {
	WorkingDirectory core.AbsolutePath
	Configuration    configurationDocument
	Input            appendEvidenceInput
}

func (r taskEvidenceUploadRequest) Validate() error {
	return errors.Join(r.WorkingDirectory.Validate(), r.Configuration.Validate(), r.Input.Validate())
}

type taskEvidencePreparationRequest struct {
	WorkingDirectory core.AbsolutePath
	Configuration    configurationDocument
	Input            appendEvidenceInput
	Instant          temporal.Instant
}

func (r taskEvidencePreparationRequest) Validate() error {
	if err := errors.Join(
		r.WorkingDirectory.Validate(), r.Configuration.Validate(), r.Input.Validate(), r.Instant.Validate(),
	); err != nil {
		return err
	}
	if r.Configuration.EvidenceStorage == nil {
		return commandError(evidenceStorageRequiredErrorText, nil)
	}
	return nil
}

type taskEvidenceObjectNameRequest struct {
	Prefix gcsobjects.GCSObjectPrefix
	Source core.AbsolutePath
	Input  appendEvidenceInput
	Digest core.SHA256Digest
}

func (r taskEvidenceObjectNameRequest) Validate() error {
	return errors.Join(r.Prefix.Validate(), r.Input.Validate(), r.Source.Validate(), r.Digest.Validate())
}

func (p taskEvidenceUploadPlan) Validate() error {
	return errors.Join(
		p.Source.Validate(), p.Bucket.Validate(), p.Name.Validate(),
		p.ContentType.Validate(), p.Integrity.Validate(), p.CustomTime.Validate(),
	)
}

type taskEvidenceUploadReceipt struct {
	Location core.HTTPEndpoint
	Digest   core.SHA256Digest
}

func (r taskEvidenceUploadReceipt) Validate() error {
	return errors.Join(r.Location.Validate(), r.Digest.Validate())
}

type taskEvidenceSource struct {
	File *os.File
	Root *os.Root
}

func (s taskEvidenceSource) Validate() error {
	if s.File == nil || s.Root == nil {
		return errors.Join(core.ErrFilestoreContract, core.ErrFilestoreSource)
	}
	return nil
}

func (s taskEvidenceSource) Close() error {
	if err := s.Validate(); err != nil {
		return err
	}
	return errors.Join(s.File.Close(), s.Root.Close())
}

func uploadTaskEvidence(
	ctx context.Context,
	request taskEvidenceUploadRequest,
) (taskEvidenceUploadReceipt, error) {
	if ctx == nil {
		return taskEvidenceUploadReceipt{}, commandError(evidenceUploadInputErrorText, core.ErrNilContext)
	}
	if err := request.Validate(); err != nil {
		return taskEvidenceUploadReceipt{}, commandError(evidenceUploadInputErrorText, err)
	}
	observation, err := temporal.Observe()
	if err != nil {
		return taskEvidenceUploadReceipt{}, commandError("evidence upload time observation failed", err)
	}
	instant, err := observation.Instant()
	if err != nil {
		return taskEvidenceUploadReceipt{}, commandError("evidence upload time projection failed", err)
	}
	plan, err := prepareTaskEvidenceUpload(ctx, taskEvidencePreparationRequest{
		WorkingDirectory: request.WorkingDirectory,
		Configuration:    request.Configuration,
		Input:            request.Input,
		Instant:          instant,
	})
	if err != nil {
		return taskEvidenceUploadReceipt{}, err
	}
	client, err := gcsobjects.NewGCSClient(ctx, gcsobjects.GCSClientConfig{
		Authentication: gcsobjects.GCSAuthenticationApplicationDefault,
	})
	if err != nil {
		return taskEvidenceUploadReceipt{}, commandError("evidence storage client construction failed", err)
	}
	metadata, uploadErr := executeTaskEvidenceUpload(ctx, client, plan)
	if err := errors.Join(uploadErr, client.Close()); err != nil {
		return taskEvidenceUploadReceipt{}, commandError("evidence storage upload failed", err)
	}
	location, err := metadata.Address()
	if err != nil {
		return taskEvidenceUploadReceipt{}, commandError("evidence storage address projection failed", err)
	}
	receipt := taskEvidenceUploadReceipt{Location: location, Digest: plan.Integrity.SHA256}
	if err := receipt.Validate(); err != nil {
		return taskEvidenceUploadReceipt{}, commandError("evidence upload receipt is invalid", err)
	}
	return receipt, nil
}

func prepareTaskEvidenceUpload(
	ctx context.Context,
	request taskEvidencePreparationRequest,
) (taskEvidenceUploadPlan, error) {
	if ctx == nil {
		return taskEvidenceUploadPlan{}, commandError(evidenceUploadPreparationInputErrorText, core.ErrNilContext)
	}
	if err := request.Validate(); err != nil {
		return taskEvidenceUploadPlan{}, commandError(evidenceUploadPreparationInputErrorText, err)
	}
	sourcePath, err := request.Input.sourcePath(request.WorkingDirectory)
	if err != nil {
		return taskEvidenceUploadPlan{}, err
	}
	source, err := openTaskEvidenceSource(ctx, sourcePath)
	if err != nil {
		return taskEvidenceUploadPlan{}, commandError("evidence source cannot be opened", err)
	}
	inspection, inspectErr := objectstore.Inspect(ctx, objectstore.InspectionRequest{
		Source: source.File, MaximumBytes: request.Configuration.EvidenceStorage.MaximumBytes,
	})
	if err := errors.Join(inspectErr, source.Close()); err != nil {
		return taskEvidenceUploadPlan{}, commandError("evidence source inspection failed", err)
	}
	bucket, prefix, err := request.Configuration.EvidenceStorage.destination()
	if err != nil {
		return taskEvidenceUploadPlan{}, err
	}
	name, err := taskEvidenceObjectName(taskEvidenceObjectNameRequest{
		Prefix: prefix, Input: request.Input, Source: sourcePath, Digest: inspection.Integrity.SHA256,
	})
	if err != nil {
		return taskEvidenceUploadPlan{}, commandError("evidence object name is invalid", err)
	}
	plan := taskEvidenceUploadPlan{
		Source: sourcePath, Bucket: bucket, Name: name, ContentType: request.Input.ContentType,
		Integrity: inspection.Integrity, CustomTime: request.Instant,
	}
	if err := plan.Validate(); err != nil {
		return taskEvidenceUploadPlan{}, commandError("evidence upload plan is invalid", err)
	}
	return plan, nil
}

func openTaskEvidenceSource(ctx context.Context, path core.AbsolutePath) (taskEvidenceSource, error) {
	location, err := filestore.OpenParent(ctx, path)
	if err != nil {
		return taskEvidenceSource{}, err
	}
	file, err := filestore.OpenRead(ctx, filestore.ReadHandleRequest{Location: location})
	if err != nil {
		return taskEvidenceSource{}, errors.Join(err, location.Root.Close())
	}
	source := taskEvidenceSource{File: file, Root: location.Root}
	if err := source.Validate(); err != nil {
		return taskEvidenceSource{}, errors.Join(err, file.Close(), location.Root.Close())
	}
	return source, nil
}

func taskEvidenceObjectName(request taskEvidenceObjectNameRequest) (gcsobjects.GCSObjectName, error) {
	if err := request.Validate(); err != nil {
		return gcsobjects.GCSObjectName{}, err
	}
	taskSegment, err := gcsobjects.ParseGCSObjectSegment(request.Input.TaskID.String())
	if err != nil {
		return gcsobjects.GCSObjectName{}, err
	}
	digestText, err := request.Digest.Hex()
	if err != nil {
		return gcsobjects.GCSObjectName{}, err
	}
	digestSegment, err := gcsobjects.ParseGCSObjectSegment(digestText)
	if err != nil {
		return gcsobjects.GCSObjectName{}, err
	}
	base, err := request.Source.Base()
	if err != nil {
		return gcsobjects.GCSObjectName{}, err
	}
	leaf, err := gcsobjects.ParseGCSObjectSegment(base.String())
	if err != nil {
		return gcsobjects.GCSObjectName{}, err
	}
	taskPrefix, err := gcsobjects.ComposeGCSChildPrefix(gcsobjects.GCSChildPrefixRequest{
		Parent: request.Prefix, Segment: taskSegment,
	})
	if err != nil {
		return gcsobjects.GCSObjectName{}, err
	}
	digestPrefix, err := gcsobjects.ComposeGCSChildPrefix(gcsobjects.GCSChildPrefixRequest{
		Parent: taskPrefix, Segment: digestSegment,
	})
	if err != nil {
		return gcsobjects.GCSObjectName{}, err
	}
	return gcsobjects.ComposeGCSObjectName(gcsobjects.GCSObjectInPrefixRequest{
		Prefix: digestPrefix, Leaf: leaf,
	})
}

func executeTaskEvidenceUpload(
	ctx context.Context,
	client *gcsobjects.GCSClient,
	plan taskEvidenceUploadPlan,
) (gcsobjects.GCSObjectMetadata, error) {
	if err := plan.Validate(); err != nil {
		return gcsobjects.GCSObjectMetadata{}, err
	}
	source, err := openTaskEvidenceSource(ctx, plan.Source)
	if err != nil {
		return gcsobjects.GCSObjectMetadata{}, err
	}
	metadata, uploadErr := gcsobjects.UploadMedia(ctx, client, gcsobjects.GCSMediaUpload{
		Source: source.File, Bucket: plan.Bucket, Name: plan.Name,
		ContentType: plan.ContentType, Integrity: plan.Integrity, CustomTime: plan.CustomTime,
	})
	closeErr := source.Close()
	if uploadErr == nil {
		return metadata, closeErr
	}
	if closeErr != nil || !errors.Is(uploadErr, core.ErrObjectStoreConflict) {
		return gcsobjects.GCSObjectMetadata{}, errors.Join(uploadErr, closeErr)
	}
	return verifyExistingTaskEvidence(ctx, client, plan)
}

func verifyExistingTaskEvidence(
	ctx context.Context,
	client *gcsobjects.GCSClient,
	plan taskEvidenceUploadPlan,
) (gcsobjects.GCSObjectMetadata, error) {
	maximum, err := core.NewByteCount(plan.Integrity.Length.Uint64())
	if err != nil {
		return gcsobjects.GCSObjectMetadata{}, err
	}
	metadata, err := gcsobjects.ReadGCSObject(ctx, client, gcsobjects.GCSReadRequest{
		Destination: io.Discard, Bucket: plan.Bucket, Name: plan.Name,
		SHA256: plan.Integrity.SHA256, Maximum: maximum,
	})
	if err != nil {
		return gcsobjects.GCSObjectMetadata{}, err
	}
	if metadata.Length() != plan.Integrity.Length || metadata.CRC32C() != plan.Integrity.CRC32C ||
		metadata.ContentType() != plan.ContentType {
		return gcsobjects.GCSObjectMetadata{}, errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreIntegrity)
	}
	return metadata, nil
}

var (
	_ core.Validatable = taskEvidenceUploadPlan{}
	_ core.Validatable = taskEvidenceUploadRequest{}
	_ core.Validatable = taskEvidencePreparationRequest{}
	_ core.Validatable = taskEvidenceObjectNameRequest{}
	_ core.Validatable = taskEvidenceUploadReceipt{}
	_ core.Validatable = taskEvidenceSource{}
)
