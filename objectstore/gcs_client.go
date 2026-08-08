package objectstore

import (
	"context"
	"errors"
	"hash/crc32"
	"io"

	"cloud.google.com/go/storage"
	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// GCSClient is an authenticated capability over the official Cloud Storage SDK.
type GCSClient struct{ client *storage.Client }

// NewGCSClient constructs an authenticated production Cloud Storage client.
func NewGCSClient(ctx context.Context, config GCSClientConfig) (*GCSClient, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return nil, errors.Join(core.ErrObjectStoreContract, err)
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	var (
		client *storage.Client
		err    error
	)
	if config.Authentication == GCSAuthenticationServiceAccountFile {
		client, err = storage.NewClient(ctx, option.WithAuthCredentialsFile(
			option.ServiceAccount, config.CredentialFile.String(),
		))
	} else {
		client, err = storage.NewClient(ctx)
	}
	if err != nil {
		return nil, errors.Join(core.ErrObjectStoreContract, err)
	}
	return &GCSClient{client: client}, nil
}

// Validate rejects an unconstructed or closed client capability.
func (c *GCSClient) Validate() error {
	if c == nil || c.client == nil {
		return core.ErrObjectStoreContract
	}
	return nil
}

// Close releases the official SDK client's resources exactly once.
func (c *GCSClient) Close() error {
	if err := c.Validate(); err != nil {
		return err
	}
	err := c.client.Close()
	c.client = nil
	if err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	return nil
}

// GCSWriteRequest is exact create-only authenticated object ingress.
type GCSWriteRequest struct {
	Source       io.Reader
	Bucket       GCSBucket
	Name         GCSObjectName
	ContentType  core.HTTPMediaType
	CacheControl GCSCacheControl
	Integrity    Integrity
	CustomTime   temporal.Instant
}

// Validate rejects incomplete or contradictory write ingress.
func (r GCSWriteRequest) Validate() error {
	if r.Source == nil {
		return errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreSource)
	}
	for _, err := range []error{
		r.Bucket.Validate(), r.Name.Validate(), validateAuthenticatedGCSIntegrity(r.Integrity),
		r.ContentType.Validate(), r.CacheControl.Validate(), r.CustomTime.Validate(),
	} {
		if err != nil {
			return errors.Join(core.ErrObjectStoreContract, err)
		}
	}
	return nil
}

// GCSReadRequest is exact authenticated object egress.
type GCSReadRequest struct {
	Destination io.Writer
	Bucket      GCSBucket
	Name        GCSObjectName
	SHA256      core.SHA256Digest
	Maximum     core.ByteCount
}

// Validate rejects incomplete or contradictory read ingress.
func (r GCSReadRequest) Validate() error {
	if r.Destination == nil {
		return errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreDestination)
	}
	for _, err := range []error{r.Bucket.Validate(), r.Name.Validate(), r.SHA256.Validate()} {
		if err != nil {
			return errors.Join(core.ErrObjectStoreContract, err)
		}
	}
	maximum, err := r.Maximum.Uint64()
	if err != nil || maximum > GoogleCloudStorageObjectMaximumBytes {
		return errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreSize, err)
	}
	return nil
}

func validateAuthenticatedGCSIntegrity(integrity Integrity) error {
	if err := integrity.Validate(); err != nil {
		return err
	}
	if integrity.Length.Uint64() > GoogleCloudStorageObjectMaximumBytes {
		return core.ErrObjectStoreSize
	}
	return nil
}

// GCSDeleteRequest is one bounded, generation-safe destructive prefix sweep.
type GCSDeleteRequest struct {
	Bucket     GCSBucket
	Prefix     GCSObjectPrefix
	MaxObjects core.ByteCount
}

// GCSDeleteObjectRequest is one exact generation-safe object deletion.
type GCSDeleteObjectRequest struct {
	Bucket GCSBucket
	Name   GCSObjectName
}

// Validate rejects an unset bucket or exact object name.
func (r GCSDeleteObjectRequest) Validate() error {
	return errors.Join(r.Bucket.Validate(), r.Name.Validate())
}

// Validate rejects unbounded or whole-bucket destructive ingress.
func (r GCSDeleteRequest) Validate() error {
	if err := r.Bucket.Validate(); err != nil {
		return err
	}
	if err := r.Prefix.Validate(); err != nil {
		return err
	}
	maximum, err := r.MaxObjects.Uint64()
	if err != nil || maximum > GCSDeleteMaximumObjects {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	return nil
}

// CreateGCSObject streams one exact object through the official SDK under a
// generation-zero precondition. Existing objects are conflicts, never writes.
func CreateGCSObject(ctx context.Context, client *GCSClient, request GCSWriteRequest) (GCSObjectMetadata, error) {
	if err := validateGCSCall(ctx, client); err != nil {
		return GCSObjectMetadata{}, err
	}
	if err := request.Validate(); err != nil {
		return GCSObjectMetadata{}, err
	}
	object := client.client.Bucket(request.Bucket.String()).Object(request.Name.String()).If(
		storage.Conditions{DoesNotExist: true},
	)
	writeContext, cancel := context.WithCancel(ctx)
	defer cancel()
	writer := object.NewWriter(writeContext)
	writer.ContentType = request.ContentType.String()
	writer.CacheControl = request.CacheControl.String()
	customTime, err := request.CustomTime.Time()
	if err != nil {
		return GCSObjectMetadata{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	writer.CustomTime = customTime
	checksum, err := request.Integrity.CRC32C.Uint32()
	if err != nil {
		return GCSObjectMetadata{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	writer.CRC32C = checksum
	writer.SendCRC32C = true
	if err := streamGCSWrite(writer, cancel, request); err != nil {
		return GCSObjectMetadata{}, err
	}
	if err := writer.Close(); err != nil {
		return GCSObjectMetadata{}, projectGCSError(err, core.ErrObjectStoreDestination)
	}
	return metadataFromGCSAttrs(writer.Attrs())
}

func streamGCSWrite(writer *storage.Writer, cancel context.CancelFunc, request GCSWriteRequest) error {
	length, err := request.Integrity.Length.Int64()
	if err != nil {
		return errors.Join(core.ErrObjectStoreSize, err)
	}
	exact := newExactReader(request.Source, length)
	digest := core.NewDigestWriter()
	if length == 0 {
		err = exact.proveEmpty()
	} else {
		_, err = io.Copy(writer, io.TeeReader(exact, digest))
	}
	if err != nil {
		cancel()
		_ = writer.Close()
		if exact.failure != nil {
			return exact.failure
		}
		return projectGCSError(err, core.ErrObjectStoreDestination)
	}
	actualDigest, actualLength, err := digest.Seal()
	if err != nil || actualDigest != request.Integrity.SHA256 || actualLength != request.Integrity.Length {
		cancel()
		_ = writer.Close()
		return errors.Join(core.ErrObjectStoreSource, core.ErrObjectStoreIntegrity, err)
	}
	return nil
}

// ReadGCSObject streams one exact generation into a caller-owned destination.
func ReadGCSObject(ctx context.Context, client *GCSClient, request GCSReadRequest) (GCSObjectMetadata, error) {
	if err := validateGCSCall(ctx, client); err != nil {
		return GCSObjectMetadata{}, err
	}
	if err := request.Validate(); err != nil {
		return GCSObjectMetadata{}, err
	}
	object := client.client.Bucket(request.Bucket.String()).Object(request.Name.String())
	reader, err := object.NewReader(ctx)
	if err != nil {
		return GCSObjectMetadata{}, projectGCSError(err, core.ErrObjectStoreSource)
	}
	metadata, err := metadataFromReader(ctx, object, reader)
	if err != nil {
		_ = reader.Close()
		return GCSObjectMetadata{}, err
	}
	integrity, err := readIntegrityFromMetadata(request, metadata)
	if err != nil {
		_ = reader.Close()
		return GCSObjectMetadata{}, err
	}
	if err := streamGCSRead(reader, request.Destination, integrity); err != nil {
		return GCSObjectMetadata{}, errors.Join(err, reader.Close())
	}
	if err := reader.Close(); err != nil {
		return GCSObjectMetadata{}, projectGCSError(err, core.ErrObjectStoreSource)
	}
	return metadata, nil
}

func metadataFromReader(ctx context.Context, object *storage.ObjectHandle, reader *storage.Reader) (GCSObjectMetadata, error) {
	attrs, err := object.Generation(reader.Attrs.Generation).Attrs(ctx)
	if err != nil {
		return GCSObjectMetadata{}, projectGCSError(err, core.ErrObjectStoreSource)
	}
	return metadataFromGCSAttrs(attrs)
}

func readIntegrityFromMetadata(request GCSReadRequest, metadata GCSObjectMetadata) (Integrity, error) {
	maximum, err := request.Maximum.Uint64()
	if err != nil {
		return Integrity{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	if metadata.Length().Uint64() > maximum {
		return Integrity{}, core.ErrObjectStoreSize
	}
	return Integrity{SHA256: request.SHA256, Length: metadata.Length(), CRC32C: metadata.CRC32C()}, nil
}

func streamGCSRead(reader *storage.Reader, destinationWriter io.Writer, integrity Integrity) error {
	length, err := integrity.Length.Int64()
	if err != nil {
		return errors.Join(core.ErrObjectStoreSize, err)
	}
	exact := newExactReader(reader, length)
	digest := core.NewDigestWriter()
	checksum := crc32.New(crc32.MakeTable(crc32.Castagnoli))
	destination := io.MultiWriter(destinationWriter, digest, checksum)
	if length == 0 {
		err = exact.proveEmpty()
	} else {
		_, err = io.Copy(destination, exact)
	}
	if err != nil {
		if exact.failure != nil {
			return exact.failure
		}
		return errors.Join(core.ErrObjectStoreDestination, err)
	}
	actualDigest, actualLength, err := digest.Seal()
	if err != nil || actualDigest != integrity.SHA256 ||
		actualLength != integrity.Length ||
		core.NewCRC32C(checksum.Sum32()) != integrity.CRC32C {
		return errors.Join(core.ErrObjectStoreIntegrity, err)
	}
	return nil
}

// DeleteGCSObject permanently deletes one exact current generation and proves
// that no current object remains at the name. Soft-delete buckets are refused.
func DeleteGCSObject(ctx context.Context, client *GCSClient, request GCSDeleteObjectRequest) (GCSDeleteObjectResult, error) {
	if err := validateGCSCall(ctx, client); err != nil {
		return GCSDeleteObjectResult{}, err
	}
	if err := request.Validate(); err != nil {
		return GCSDeleteObjectResult{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	bucket := client.client.Bucket(request.Bucket.String())
	if err := requirePermanentGCSDeletion(ctx, bucket); err != nil {
		return GCSDeleteObjectResult{}, err
	}
	object := bucket.Object(request.Name.String())
	attrs, err := object.Attrs(ctx)
	if err != nil {
		return GCSDeleteObjectResult{}, projectGCSError(err, core.ErrObjectStoreDestination)
	}
	generation, err := NewGCSGeneration(attrs.Generation)
	if err != nil {
		return GCSDeleteObjectResult{}, err
	}
	value, err := generation.Int64()
	if err != nil {
		return GCSDeleteObjectResult{}, err
	}
	if err := object.Generation(value).If(storage.Conditions{GenerationMatch: value}).Delete(ctx); err != nil &&
		!errors.Is(err, storage.ErrObjectNotExist) {
		return GCSDeleteObjectResult{}, projectGCSError(err, core.ErrObjectStoreDestination)
	}
	if err := confirmGCSObjectAbsent(ctx, object); err != nil {
		return GCSDeleteObjectResult{}, err
	}
	return GCSDeleteObjectResult{name: request.Name, generation: generation}, nil
}

func confirmGCSObjectAbsent(ctx context.Context, object *storage.ObjectHandle) error {
	_, err := object.Attrs(ctx)
	if errors.Is(err, storage.ErrObjectNotExist) {
		return nil
	}
	if err != nil {
		return projectGCSError(err, core.ErrObjectStoreDestination)
	}
	return errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreConflict)
}

// DeleteGCSObjects permanently deletes every live object under one bounded
// prefix, using each listed generation as a delete precondition, then proves
// the prefix absent. Buckets with soft deletion enabled are refused.
func DeleteGCSObjects(ctx context.Context, client *GCSClient, request GCSDeleteRequest) (GCSDeleteResult, error) {
	if err := validateGCSCall(ctx, client); err != nil {
		return GCSDeleteResult{}, err
	}
	if err := request.Validate(); err != nil {
		return GCSDeleteResult{}, err
	}
	bucket := client.client.Bucket(request.Bucket.String())
	if err := requirePermanentGCSDeletion(ctx, bucket); err != nil {
		return GCSDeleteResult{}, err
	}
	maximum, _ := request.MaxObjects.Uint64()
	deleted, err := deleteGCSPrefix(ctx, bucket, request.Prefix, maximum)
	if err != nil {
		return GCSDeleteResult{}, err
	}
	if err := confirmGCSPrefixAbsent(ctx, bucket, request.Prefix); err != nil {
		return GCSDeleteResult{}, err
	}
	count, err := core.NewByteLength(deleted)
	if err != nil {
		return GCSDeleteResult{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	return GCSDeleteResult{prefix: request.Prefix, deleted: count}, nil
}

func requirePermanentGCSDeletion(ctx context.Context, bucket *storage.BucketHandle) error {
	attrs, err := bucket.Attrs(ctx)
	if err != nil {
		return projectGCSError(err, core.ErrObjectStoreDestination)
	}
	if attrs.SoftDeletePolicy != nil && attrs.SoftDeletePolicy.RetentionDuration > 0 {
		return errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreConflict)
	}
	return nil
}

func deleteGCSPrefix(ctx context.Context, bucket *storage.BucketHandle, prefix GCSObjectPrefix, maximum uint64) (uint64, error) {
	objects := bucket.Objects(ctx, &storage.Query{Prefix: prefix.String(), Projection: storage.ProjectionNoACL})
	var deleted uint64
	for {
		attrs, err := objects.Next()
		if errors.Is(err, iterator.Done) {
			return deleted, nil
		}
		if err != nil {
			return deleted, projectGCSError(err, core.ErrObjectStoreDestination)
		}
		if deleted == maximum {
			return deleted, errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreSize)
		}
		object := bucket.Object(attrs.Name).Generation(attrs.Generation).If(
			storage.Conditions{GenerationMatch: attrs.Generation},
		)
		if err := object.Delete(ctx); err != nil && !errors.Is(err, storage.ErrObjectNotExist) {
			return deleted, projectGCSError(err, core.ErrObjectStoreDestination)
		}
		deleted++
	}
}

func confirmGCSPrefixAbsent(ctx context.Context, bucket *storage.BucketHandle, prefix GCSObjectPrefix) error {
	objects := bucket.Objects(ctx, &storage.Query{Prefix: prefix.String(), Projection: storage.ProjectionNoACL})
	_, err := objects.Next()
	if errors.Is(err, iterator.Done) {
		return nil
	}
	if err != nil {
		return projectGCSError(err, core.ErrObjectStoreDestination)
	}
	return errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreConflict)
}

func validateGCSCall(ctx context.Context, client *GCSClient) error {
	if err := contextstate.Validate(ctx); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	return client.Validate()
}

func metadataFromGCSAttrs(attrs *storage.ObjectAttrs) (GCSObjectMetadata, error) {
	if attrs == nil {
		return GCSObjectMetadata{}, core.ErrObjectStoreContract
	}
	identity, err := gcsIdentityFromAttrs(attrs)
	if err != nil {
		return GCSObjectMetadata{}, err
	}
	properties, err := gcsPropertiesFromAttrs(attrs)
	if err != nil {
		return GCSObjectMetadata{}, err
	}
	times, err := gcsTimesFromAttrs(attrs)
	if err != nil {
		return GCSObjectMetadata{}, err
	}
	metadata := GCSObjectMetadata{
		bucket: identity.bucket, name: identity.name, generation: identity.generation,
		length: properties.length, crc32c: core.NewCRC32C(attrs.CRC32C),
		contentType: properties.contentType, cacheControl: properties.cacheControl,
		createdAt: times.createdAt, updatedAt: times.updatedAt, customTime: times.customTime,
	}
	if err := metadata.Validate(); err != nil {
		return GCSObjectMetadata{}, err
	}
	return metadata, nil
}

type gcsObjectIdentity struct {
	bucket     GCSBucket
	name       GCSObjectName
	generation GCSGeneration
}

func gcsIdentityFromAttrs(attrs *storage.ObjectAttrs) (gcsObjectIdentity, error) {
	bucket, err := ParseGCSBucket(attrs.Bucket)
	if err != nil {
		return gcsObjectIdentity{}, err
	}
	name, err := ParseGCSObjectName(attrs.Name)
	if err != nil {
		return gcsObjectIdentity{}, err
	}
	generation, err := NewGCSGeneration(attrs.Generation)
	if err != nil {
		return gcsObjectIdentity{}, err
	}
	return gcsObjectIdentity{bucket: bucket, name: name, generation: generation}, nil
}

type gcsObjectProperties struct {
	contentType  core.HTTPMediaType
	cacheControl GCSCacheControl
	length       core.ByteLength
}

func gcsPropertiesFromAttrs(attrs *storage.ObjectAttrs) (gcsObjectProperties, error) {
	lengthValue, err := core.CheckedUint64FromInt64(attrs.Size)
	if err != nil {
		return gcsObjectProperties{}, errors.Join(core.ErrObjectStoreSize, err)
	}
	length, err := core.NewByteLength(lengthValue)
	if err != nil {
		return gcsObjectProperties{}, errors.Join(core.ErrObjectStoreSize, err)
	}
	contentType, err := core.ParseHTTPMediaType(attrs.ContentType)
	if err != nil {
		return gcsObjectProperties{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	cacheControl, err := ParseGCSCacheControl(attrs.CacheControl)
	if err != nil {
		return gcsObjectProperties{}, err
	}
	return gcsObjectProperties{length: length, contentType: contentType, cacheControl: cacheControl}, nil
}

type gcsObjectTimes struct {
	createdAt  temporal.Instant
	updatedAt  temporal.Instant
	customTime temporal.Instant
}

func gcsTimesFromAttrs(attrs *storage.ObjectAttrs) (gcsObjectTimes, error) {
	createdAt, err := temporal.NewInstant(attrs.Created)
	if err != nil {
		return gcsObjectTimes{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	updatedAt, err := temporal.NewInstant(attrs.Updated)
	if err != nil {
		return gcsObjectTimes{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	customTime, err := temporal.NewInstant(attrs.CustomTime)
	if err != nil {
		return gcsObjectTimes{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	return gcsObjectTimes{createdAt: createdAt, updatedAt: updatedAt, customTime: customTime}, nil
}

func projectGCSError(cause error, operation core.ErrorIdentity) error {
	projected := []error{core.ErrObjectStoreContract, operation}
	if errors.Is(cause, storage.ErrObjectNotExist) {
		projected = append(projected, core.ErrObjectStoreAbsent)
	}
	provider, hasProvider := errors.AsType[*googleapi.Error](cause)
	if hasProvider {
		switch provider.Code {
		case 404:
			projected = append(projected, core.ErrObjectStoreAbsent)
		case 409, 412:
			projected = append(projected, core.ErrObjectStoreConflict)
		}
	}
	if errors.Is(cause, context.Canceled) || errors.Is(cause, context.DeadlineExceeded) {
		projected = append(projected, cause)
	}
	return errors.Join(projected...)
}
