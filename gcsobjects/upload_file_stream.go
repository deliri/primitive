package gcsobjects

import (
	"bufio"
	"context"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/temporal"
	"google.golang.org/api/googleapi"
	storageapi "google.golang.org/api/storage/v1"
)

const gcsFileStreamBufferBytes = 32 << 10

// GCSFileStreamUpload borrows a source whose final extent is not known yet.
// EOF means the producer has finished, not merely caught up with a live file.
// The source must cooperate with cancellation; its lifetime stays with its owner.
type GCSFileStreamUpload struct {
	Source     io.Reader
	Bucket     GCSBucket
	Name       GCSObjectName
	CustomTime temporal.Instant
}

func (r GCSFileStreamUpload) Validate() error {
	if core.ReaderIsNil(r.Source) {
		return errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreSource)
	}
	if err := errors.Join(r.Bucket.Validate(), r.Name.Validate(), r.CustomTime.Validate()); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	return nil
}

// GCSFileStreamResult binds the locally observed stream to one acknowledged
// provider generation. It is released only after close and checksum agreement.
type GCSFileStreamResult struct {
	metadata  GCSObjectMetadata
	integrity objectstore.Integrity
	set       bool
}

func (r GCSFileStreamResult) Validate() error {
	if !r.set {
		return core.ErrObjectStoreContract
	}
	if err := errors.Join(r.metadata.Validate(), r.integrity.Validate()); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	if r.metadata.Length() != r.integrity.Length || r.metadata.CRC32C() != r.integrity.CRC32C || r.metadata.ContentType() != core.HTTPMediaTypeOctetStream() {
		return core.ErrObjectStoreIntegrity
	}
	return nil
}

func (r GCSFileStreamResult) Metadata() (GCSObjectMetadata, error) {
	if err := r.Validate(); err != nil {
		return GCSObjectMetadata{}, err
	}
	return r.metadata, nil
}

func (r GCSFileStreamResult) Integrity() (objectstore.Integrity, error) {
	if err := r.Validate(); err != nil {
		return objectstore.Integrity{}, err
	}
	return r.integrity, nil
}

// gcsStreamDestination applies the provider's existing object-size ceiling
// before its writer receives an excess byte. It is not a product flow quota.
type gcsStreamDestination struct {
	destination io.Writer
	remaining   uint64
}

func (w *gcsStreamDestination) Write(data []byte) (int, error) {
	if uint64(len(data)) > w.remaining {
		return 0, core.ErrObjectStoreSize
	}
	n, err := w.destination.Write(data)
	if n < 0 || n > len(data) {
		return 0, errors.Join(core.ErrObjectStoreDestination, err)
	}
	w.remaining -= uint64(n)
	if n != len(data) && err == nil {
		err = io.ErrShortWrite
	}
	if err != nil {
		err = errors.Join(core.ErrObjectStoreDestination, err)
	}
	return n, err
}

// UploadFileStream performs one create-only stored-object upload without a
// preliminary source pass. Objectstore observes the SHA-256, CRC32C and extent
// while the official SDK streams bytes. A private object may exist before its
// returned metadata is verified; callers must publish no receipt or delete a
// source until this function returns a validated result. Failure returns zero.
// The SDK's zero chunk size selects its direct streaming request and disables
// its buffered retry. A failed attempt leaves source retention/retry to the
// caller. Fixed read/inspection buffers bound memory independently of length.
func UploadFileStream(ctx context.Context, client *GCSClient, request GCSFileStreamUpload) (GCSFileStreamResult, error) {
	if err := errors.Join(validateGCSCall(ctx, client), request.Validate()); err != nil {
		return GCSFileStreamResult{}, err
	}
	input := bufio.NewReaderSize(request.Source, gcsFileStreamBufferBytes)
	_, firstErr := input.Peek(1)
	if firstErr != nil && firstErr != io.EOF {
		return GCSFileStreamResult{}, errors.Join(core.ErrObjectStoreSource, firstErr)
	}
	customTime, err := request.CustomTime.RFC3339Nano()
	if err != nil {
		return GCSFileStreamResult{}, err
	}
	writeContext, cancel := context.WithCancel(ctx)
	defer cancel()
	if client.objects == nil {
		return GCSFileStreamResult{}, core.ErrObjectStoreContract
	}
	var integrity objectstore.Integrity
	var response *storageapi.Object
	transfer, transferErr := filestore.TransferPipe(writeContext, filestore.PipeTransferRequest{
		Produce: func(ctx context.Context, destination io.Writer) error {
			completed := false
			defer func() {
				// TransferPipe joins a panicking producer but closes its writer
				// gracefully. Abort HTTP before that close can seal a prefix.
				if !completed {
					cancel()
				}
			}()
			var err error
			integrity, err = writeGCSFileStream(ctx, input, destination, firstErr == io.EOF)
			if err != nil {
				cancel()
				return err
			}
			completed = true
			return nil
		},
		Consume: func(ctx context.Context, source io.Reader) error {
			var err error
			response, err = client.objects.Insert(request.Bucket.String(), &storageapi.Object{
				Name: request.Name.String(), ContentType: core.HTTPMediaTypeOctetStream().String(), CustomTime: customTime,
			}).Name(request.Name.String()).IfGenerationMatch(0).Projection("full").
				Media(source, googleapi.ChunkSize(0), googleapi.ContentType(core.HTTPMediaTypeOctetStream().String())).Context(ctx).Do()
			if err != nil {
				return projectGCSError(err, core.ErrObjectStoreDestination)
			}
			return nil
		},
	})
	if err := errors.Join(transferErr, transfer.ProducerError, transfer.ConsumerError, transfer.CleanupError); err != nil {
		return GCSFileStreamResult{}, err
	}
	metadata, err := uploadedObjectMetadata(response, false)
	if err != nil {
		return GCSFileStreamResult{}, err
	}
	if metadata.Bucket() != request.Bucket || metadata.Name() != request.Name {
		return GCSFileStreamResult{}, core.ErrObjectStoreIntegrity
	}
	if metadata.CRC32C() != integrity.CRC32C {
		return GCSFileStreamResult{}, errors.Join(core.ErrObjectStoreDestination, core.ErrObjectStoreIntegrity)
	}
	result := GCSFileStreamResult{metadata: metadata, integrity: integrity, set: true}
	if err := result.Validate(); err != nil {
		return GCSFileStreamResult{}, err
	}
	return result, nil
}

func writeGCSFileStream(ctx context.Context, source io.Reader, destination io.Writer, empty bool) (objectstore.Integrity, error) {
	if empty {
		return objectstore.Integrity{SHA256: core.SHA256Of(nil), CRC32C: core.NewCRC32C(0)}, nil
	}
	spec, err := objectstore.Spec(objectstore.ProviderGoogleCloudStorage)
	if err != nil {
		return objectstore.Integrity{}, err
	}
	// One additional inspection byte distinguishes EOF exactly at the native
	// ceiling. The destination refuses that byte before passing it to the SDK.
	maximum, err := core.NewByteCount(spec.UploadMaximum.Uint64() + 1)
	if err != nil {
		return objectstore.Integrity{}, err
	}
	limited := &gcsStreamDestination{destination: destination, remaining: spec.UploadMaximum.Uint64()}
	observed, err := objectstore.Inspect(ctx, objectstore.InspectionRequest{Source: io.TeeReader(source, limited), MaximumBytes: maximum})
	if err != nil {
		return objectstore.Integrity{}, err
	}
	return observed.Integrity, nil
}
