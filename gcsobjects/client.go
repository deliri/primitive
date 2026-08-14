package gcsobjects

import (
	"context"
	"errors"
	"hash/crc32"
	"io"
	"strconv"

	"cloud.google.com/go/storage"
	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
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

// CreateBucket creates one new bucket through the official Cloud Storage SDK.
// Provider conflict is preserved as a typed object-store conflict.
func CreateBucket(
	ctx context.Context,
	client *GCSClient,
	request GCSBucketCreateRequest,
) (GCSBucketProvisioning, error) {
	if err := request.Validate(); err != nil {
		return GCSBucketProvisioning{}, err
	}
	if err := validateGCSCall(ctx, client); err != nil {
		return GCSBucketProvisioning{}, err
	}
	attrs := &storage.BucketAttrs{Location: request.Location.String()}
	if request.Namespace == GCSNamespaceHierarchical {
		attrs.HierarchicalNamespace = &storage.HierarchicalNamespace{Enabled: true}
	}
	if err := client.client.Bucket(request.Bucket.String()).Create(
		ctx, request.Project.String(), attrs,
	); err != nil {
		return GCSBucketProvisioning{}, projectGCSError(err, core.ErrObjectStoreDestination)
	}
	result := GCSBucketProvisioning{request: request, set: true}
	return result, result.Validate()
}

// GCSMediaUpload is create-only ingress for an object a browser or CDN will
// fetch: it carries the content type that makes the bytes render and an
// optional cache-control the edge may honor.
type GCSMediaUpload struct {
	Source       io.Reader
	Bucket       GCSBucket
	Name         GCSObjectName
	ContentType  core.HTTPMediaType
	CacheControl GCSCacheControl
	Integrity    objectstore.Integrity
	CustomTime   temporal.Instant
}

// Validate rejects incomplete or contradictory media ingress. The cache
// directive is optional: a served asset without one lets the edge apply its own
// default, so an absent cache-control is admitted and a present one must be a
// legal field value.
func (r GCSMediaUpload) Validate() error {
	if r.Source == nil {
		return errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreSource)
	}
	for _, err := range []error{
		r.Bucket.Validate(), r.Name.Validate(), validateAuthenticatedGCSIntegrity(r.Integrity),
		r.ContentType.Validate(), r.CustomTime.Validate(),
	} {
		if err != nil {
			return errors.Join(core.ErrObjectStoreContract, err)
		}
	}
	if r.CacheControl.String() != "" {
		if err := r.CacheControl.Validate(); err != nil {
			return errors.Join(core.ErrObjectStoreContract, err)
		}
	}
	return nil
}

// GCSFileUpload is create-only ingress for a stored integrity-bound blob the
// system retrieves and verifies. It carries no content type or cache-control:
// the object is written as application/octet-stream with no cache directive,
// because nothing serves it to a browser.
type GCSFileUpload struct {
	Source     io.Reader
	Bucket     GCSBucket
	Name       GCSObjectName
	Integrity  objectstore.Integrity
	CustomTime temporal.Instant
}

// Validate rejects incomplete or contradictory file ingress.
func (r GCSFileUpload) Validate() error {
	if r.Source == nil {
		return errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreSource)
	}
	for _, err := range []error{
		r.Bucket.Validate(), r.Name.Validate(), validateAuthenticatedGCSIntegrity(r.Integrity),
		r.CustomTime.Validate(),
	} {
		if err != nil {
			return errors.Join(core.ErrObjectStoreContract, err)
		}
	}
	return nil
}

// gcsWrite is the owner-only projection both uploads share at the effect leaf.
type gcsWrite struct {
	Source       io.Reader
	Bucket       GCSBucket
	Name         GCSObjectName
	ContentType  core.HTTPMediaType
	CacheControl GCSCacheControl
	Integrity    objectstore.Integrity
	CustomTime   temporal.Instant
}

// GCSReadRequest is exact authenticated object egress.
type GCSReadRequest struct {
	Destination io.Writer
	Bucket      GCSBucket
	Name        GCSObjectName
	SHA256      core.SHA256Digest
	Maximum     core.ByteCount
}

// GCSUploadObservationRequest names the authority-owned provider object whose
// exact generation must agree with one authenticated client upload result.
type GCSUploadObservationRequest struct {
	Bucket   GCSBucket
	Name     GCSObjectName
	Evidence objectstore.TransferEvidence
}

// Validate rejects any observation that does not name one exact GCS upload
// generation. The generation is provider evidence, not a caller convention.
func (r GCSUploadObservationRequest) Validate() error {
	if err := errors.Join(r.Bucket.Validate(), r.Name.Validate(), r.Evidence.Validate()); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	_, err := gcsGenerationFromEvidence(r.Evidence)
	return err
}

func gcsGenerationFromEvidence(evidence objectstore.TransferEvidence) (GCSGeneration, error) {
	if evidence.Provider() != objectstore.ProviderGoogleCloudStorage ||
		evidence.Direction() != objectstore.DirectionUpload {
		return GCSGeneration{}, core.ErrObjectStoreContract
	}
	version, present := evidence.Version()
	if !present || version.Provider() != objectstore.ProviderGoogleCloudStorage {
		return GCSGeneration{}, core.ErrObjectStoreContract
	}
	value, err := strconv.ParseInt(version.String(), 10, 64)
	if err != nil {
		return GCSGeneration{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	return NewGCSGeneration(value)
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
	if err != nil || maximum > objectstore.GoogleCloudStorageObjectMaximumBytes {
		return errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreSize, err)
	}
	return nil
}

func validateAuthenticatedGCSIntegrity(integrity objectstore.Integrity) error {
	if err := integrity.Validate(); err != nil {
		return err
	}
	if integrity.Length.Uint64() > objectstore.GoogleCloudStorageObjectMaximumBytes {
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

// UploadMedia streams one create-only object a browser or CDN will fetch,
// stamping the content type that makes it render and any cache-control the
// caller declared.
func UploadMedia(ctx context.Context, client *GCSClient, request GCSMediaUpload) (GCSObjectMetadata, error) {
	if err := request.Validate(); err != nil {
		return GCSObjectMetadata{}, err
	}
	return createGCSObject(ctx, client, gcsWrite(request))
}

// UploadFile streams one create-only integrity-bound blob the system stores
// and later retrieves, written as application/octet-stream with no cache
// directive because nothing serves it to a browser.
func UploadFile(ctx context.Context, client *GCSClient, request GCSFileUpload) (GCSObjectMetadata, error) {
	if err := request.Validate(); err != nil {
		return GCSObjectMetadata{}, err
	}
	return createGCSObject(ctx, client, gcsWrite{
		Source:      request.Source,
		Bucket:      request.Bucket,
		Name:        request.Name,
		ContentType: core.HTTPMediaTypeOctetStream(),
		Integrity:   request.Integrity,
		CustomTime:  request.CustomTime,
	})
}

// createGCSObject streams one exact object through the official SDK under a
// generation-zero precondition. Existing objects are conflicts, never writes.
func createGCSObject(ctx context.Context, client *GCSClient, request gcsWrite) (GCSObjectMetadata, error) {
	if err := validateGCSCall(ctx, client); err != nil {
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

func streamGCSWrite(writer *storage.Writer, cancel context.CancelFunc, request gcsWrite) error {
	exact, err := objectstore.NewExactReader(request.Source, request.Integrity.Length)
	if err != nil {
		return err
	}
	digest := core.NewDigestWriter()
	if request.Integrity.Length.Uint64() == 0 {
		err = exact.ProveEmpty()
	} else {
		_, err = io.Copy(writer, io.TeeReader(exact, digest))
	}
	if err != nil {
		cancel()
		_ = writer.Close()
		if exact.Failure() != nil {
			return exact.Failure()
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

// ObserveGCSUpload reads only the official provider metadata for the exact
// generation authenticated by a client completion. It downloads no object
// bytes and releases proof only when provider identity, extent, and CRC32C all
// agree with that completion.
func ObserveGCSUpload(
	ctx context.Context,
	client *GCSClient,
	request GCSUploadObservationRequest,
) (VerifiedGCSUpload, error) {
	if err := validateGCSCall(ctx, client); err != nil {
		return VerifiedGCSUpload{}, err
	}
	if err := request.Validate(); err != nil {
		return VerifiedGCSUpload{}, err
	}
	generation, err := gcsGenerationFromEvidence(request.Evidence)
	if err != nil {
		return VerifiedGCSUpload{}, err
	}
	value, err := generation.Int64()
	if err != nil {
		return VerifiedGCSUpload{}, err
	}
	attrs, err := client.client.Bucket(request.Bucket.String()).
		Object(request.Name.String()).Generation(value).Attrs(ctx)
	if err != nil {
		return VerifiedGCSUpload{}, projectGCSError(err, core.ErrObjectStoreSource)
	}
	metadata, err := metadataFromGCSAttrs(attrs)
	if err != nil {
		return VerifiedGCSUpload{}, errors.Join(core.ErrObjectStoreSource, err)
	}
	if metadata.Bucket() != request.Bucket || metadata.Name() != request.Name {
		return VerifiedGCSUpload{}, errors.Join(core.ErrObjectStoreIntegrity, core.ErrObjectStoreSource)
	}
	return verifiedGCSUpload(metadata, request.Evidence)
}

func verifiedGCSUpload(
	metadata GCSObjectMetadata,
	evidence objectstore.TransferEvidence,
) (VerifiedGCSUpload, error) {
	version, present := evidence.Version()
	if !present {
		return VerifiedGCSUpload{}, core.ErrObjectStoreContract
	}
	observation, err := objectstore.VerifyProviderUpload(objectstore.ProviderUploadObservationRequest{
		Evidence: evidence, Version: version,
		Bytes: metadata.Length(), CRC32C: metadata.CRC32C(),
		ContentType: metadata.ContentType(), OccurredAt: metadata.CreatedAt(),
	})
	if err != nil {
		return VerifiedGCSUpload{}, err
	}
	verified := VerifiedGCSUpload{metadata: metadata, observation: observation, set: true}
	if err := verified.Validate(); err != nil {
		return VerifiedGCSUpload{}, err
	}
	return verified, nil
}

func metadataFromReader(ctx context.Context, object *storage.ObjectHandle, reader *storage.Reader) (GCSObjectMetadata, error) {
	attrs, err := object.Generation(reader.Attrs.Generation).Attrs(ctx)
	if err != nil {
		return GCSObjectMetadata{}, projectGCSError(err, core.ErrObjectStoreSource)
	}
	return metadataFromGCSAttrs(attrs)
}

func readIntegrityFromMetadata(request GCSReadRequest, metadata GCSObjectMetadata) (objectstore.Integrity, error) {
	maximum, err := request.Maximum.Uint64()
	if err != nil {
		return objectstore.Integrity{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	if metadata.Length().Uint64() > maximum {
		return objectstore.Integrity{}, core.ErrObjectStoreSize
	}
	return objectstore.Integrity{SHA256: request.SHA256, Length: metadata.Length(), CRC32C: metadata.CRC32C()}, nil
}

func streamGCSRead(reader *storage.Reader, destinationWriter io.Writer, integrity objectstore.Integrity) error {
	exact, err := objectstore.NewExactReader(reader, integrity.Length)
	if err != nil {
		return err
	}
	digest := core.NewDigestWriter()
	checksum := crc32.New(crc32.MakeTable(crc32.Castagnoli))
	destination := io.MultiWriter(destinationWriter, digest, checksum)
	if integrity.Length.Uint64() == 0 {
		err = exact.ProveEmpty()
	} else {
		_, err = io.Copy(destination, exact)
	}
	if err != nil {
		if exact.Failure() != nil {
			return exact.Failure()
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
	generation, value, err := currentGCSGeneration(ctx, object)
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

// currentGCSGeneration observes the exact live generation a delete
// precondition binds to. Absence surfaces from the attribute read itself;
// attributes carrying no object is a contract violation rather than an
// absence.
func currentGCSGeneration(ctx context.Context, object *storage.ObjectHandle) (GCSGeneration, int64, error) {
	attrs, err := object.Attrs(ctx)
	if err != nil {
		return GCSGeneration{}, 0, projectGCSError(err, core.ErrObjectStoreDestination)
	}
	if attrs == nil {
		return GCSGeneration{}, 0, errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreDestination)
	}
	generation, err := NewGCSGeneration(attrs.Generation)
	if err != nil {
		return GCSGeneration{}, 0, err
	}
	value, err := generation.Int64()
	if err != nil {
		return GCSGeneration{}, 0, err
	}
	return generation, value, nil
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
	cacheControl, err := optionalGCSCacheControl(attrs.CacheControl)
	if err != nil {
		return gcsObjectProperties{}, err
	}
	return gcsObjectProperties{length: length, contentType: contentType, cacheControl: cacheControl}, nil
}

// optionalGCSCacheControl reads a possibly-absent provider cache directive. A
// stored file is written with no Cache-Control, so an empty field is the absent
// value rather than a malformed one; a present value must still be a legal HTTP
// field value.
func optionalGCSCacheControl(value string) (GCSCacheControl, error) {
	if value == "" {
		return GCSCacheControl{}, nil
	}
	return ParseGCSCacheControl(value)
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
	var customTime temporal.Instant
	if !attrs.CustomTime.IsZero() {
		customTime, err = temporal.NewInstant(attrs.CustomTime)
		if err != nil {
			return gcsObjectTimes{}, errors.Join(core.ErrObjectStoreContract, err)
		}
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
