package gcsobjects

import (
	"context"
	"errors"
	"hash/crc32"
	"io"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/objectstore"
)

// GCSPublicCopyRequest verifies anonymous delivery of one exact media object.
// The caller owns its content agreement and finite, single-attempt deadline.
type GCSPublicCopyRequest struct {
	Bucket      GCSBucket
	Name        GCSObjectName
	ContentType core.HTTPMediaType
	Integrity   objectstore.Integrity
	Policy      exchange.StreamPolicy
}

func (r GCSPublicCopyRequest) Validate() error {
	if err := errors.Join(r.Bucket.Validate(), r.Name.Validate(), r.ContentType.Validate(), r.Integrity.Validate(), r.Policy.Validate()); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	if _, err := r.Integrity.Length.Int64(); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	if r.Policy.OperationTimeout.IsZero() || r.Policy.AttemptTimeout.IsZero() || r.Policy.Redirect.Mode != exchange.RedirectReject {
		return core.ErrObjectStoreContract
	}
	return nil
}

// VerifyPublicGCSObject returns an address only after a GET delivers the exact
// content type, extent, SHA-256 and CRC32C. The request supplies no authorization
// or cookies. The caller must supply an anonymous client: an opaque caller-owned
// transport can inject credentials, which this operation cannot inspect.
// No redirect, retry, bucket listing or authenticated SDK object read occurs.
// Filestore owns the joined pipe and fixed copy window; no body is retained.
func VerifyPublicGCSObject(ctx context.Context, client exchange.Client, request GCSPublicCopyRequest) (core.HTTPEndpoint, error) {
	if err := errors.Join(contextstate.Validate(ctx), client.Validate(), request.Validate()); err != nil {
		return core.HTTPEndpoint{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	address, err := ObjectAddress(request.Bucket, request.Name)
	if err != nil {
		return core.HTTPEndpoint{}, err
	}
	checksum := crc32.New(crc32.MakeTable(crc32.Castagnoli))
	expected := filestore.ContentIndexEntry{Digest: request.Integrity.SHA256, Extent: request.Integrity.Length}
	result, transferErr := filestore.TransferPipe(ctx, filestore.PipeTransferRequest{
		Produce: func(ctx context.Context, destination io.Writer) error {
			_, err := exchange.Download(exchange.DownloadCall{
				Context: ctx, Client: client, Policy: request.Policy,
				Request: exchange.DownloadRequest{
					Target: address, Destination: destination,
					Semantics:      exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySingleAttempt},
					ExpectedStatus: core.HTTPStatusOK(), ExpectedResponseContentType: request.ContentType,
				},
			})
			return err
		},
		Consume: func(ctx context.Context, source io.Reader) error {
			_, err := filestore.CopyContent(ctx, filestore.CopyContentRequest{Source: source, Destination: checksum, Expected: &expected})
			return err
		},
	})
	if err := errors.Join(transferErr, result.ProducerError, result.ConsumerError, result.CleanupError); err != nil {
		return core.HTTPEndpoint{}, errors.Join(core.ErrObjectStoreSource, core.ErrObjectStoreIntegrity, err)
	}
	if core.NewCRC32C(checksum.Sum32()) != request.Integrity.CRC32C {
		return core.HTTPEndpoint{}, core.ErrObjectStoreIntegrity
	}
	return address, nil
}
