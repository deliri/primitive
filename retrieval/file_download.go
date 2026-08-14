package retrieval

import (
	"context"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/objectstore"
)

// FileDownloadRequest is one fully typed local activation and remote transfer
// policy for an authenticated retrieval grant.
type FileDownloadRequest struct {
	Client     objectstore.Client
	Observer   objectstore.ProgressObserver
	Activation filestore.ActivationRequest
	Policy     objectstore.Policy
}

// Validate closes all caller-owned download effects. Grant-owned facts are
// closed by VerifiedGrant.DownloadFile before any temporary is created.
func (r FileDownloadRequest) Validate() error {
	if err := errors.Join(r.Client.Validate(), r.Activation.Validate(), r.Policy.Validate()); err != nil {
		return contractError(err)
	}
	if r.Observer != nil {
		if err := r.Observer.Validate(); err != nil {
			return contractError(err)
		}
	}
	return nil
}

// DownloadFile streams one authenticated object into an exclusive temporary,
// verifies the provider transfer and exact stage, then atomically activates
// the customer target. A nonzero recovery request is returned only when the
// completed stage still requires caller-owned resolution.
func (v VerifiedGrant) DownloadFile(
	ctx context.Context,
	request FileDownloadRequest,
) (filestore.CommitRequest, objectstore.Transfer, error) {
	if err := v.validateFileDownload(request); err != nil {
		return filestore.CommitRequest{}, objectstore.Transfer{}, err
	}
	destination, err := filestore.OpenStageDestination(ctx, request.Activation.StageDestination())
	if err != nil {
		return filestore.CommitRequest{}, objectstore.Transfer{}, err
	}
	file, err := destination.File()
	if err != nil {
		return filestore.CommitRequest{}, objectstore.Transfer{}, errors.Join(err, filestore.AbandonStageDestination(destination))
	}
	call, err := v.DownloadCall(DownloadCallRequest{
		Destination: file, Policy: request.Policy, Observer: request.Observer,
	})
	if err != nil {
		return filestore.CommitRequest{}, objectstore.Transfer{}, errors.Join(err, filestore.AbandonStageDestination(destination))
	}
	transfer, err := objectstore.Download(ctx, request.Client, call)
	if err != nil {
		return filestore.CommitRequest{}, transfer, errors.Join(err, filestore.AbandonStageDestination(destination))
	}
	staged, err := filestore.FinishStageDestination(ctx, destination)
	if err != nil {
		return filestore.CommitRequest{}, transfer, err
	}
	commit, err := request.Activation.CommitRequest(staged)
	if err != nil {
		return filestore.CommitRequest{}, transfer, errors.Join(err, filestore.Discard(ctx, staged))
	}
	return finishFileDownload(ctx, commit, transfer)
}

func finishFileDownload(
	ctx context.Context,
	commit filestore.CommitRequest,
	transfer objectstore.Transfer,
) (filestore.CommitRequest, objectstore.Transfer, error) {
	err := filestore.Commit(ctx, commit)
	if err == nil {
		return filestore.CommitRequest{}, transfer, nil
	}
	if errors.Is(err, core.ErrFilestoreActivationIndeterminate) ||
		errors.Is(err, core.ErrFilestoreCleanup) {
		return commit, transfer, err
	}
	if cleanupErr := filestore.Discard(ctx, commit.Staged); cleanupErr != nil {
		return commit, transfer, errors.Join(err, cleanupErr)
	}
	return filestore.CommitRequest{}, transfer, err
}

func (v VerifiedGrant) validateFileDownload(request FileDownloadRequest) error {
	if err := errors.Join(v.Validate(), request.Validate()); err != nil {
		return contractError(err)
	}
	payload, err := v.Payload()
	if err != nil {
		return contractError(err)
	}
	evidence := payload.Entry.Evidence.Payload
	if evidence.Body.Extent != request.Activation.ExpectedBytes {
		return bindingError(errors.New("retrieval activation extent differs from authenticated entry"))
	}
	_, err = v.DownloadCall(DownloadCallRequest{
		Destination: io.Discard, Policy: request.Policy, Observer: request.Observer,
	})
	return err
}

var _ core.Validatable = FileDownloadRequest{}
