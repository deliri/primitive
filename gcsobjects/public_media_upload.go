package gcsobjects

import (
	"context"
	"errors"
	"io"
	"sync"

	"cloud.google.com/go/storage"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/temporal"
	"google.golang.org/api/googleapi"
	storageapi "google.golang.org/api/storage/v1"
)

// publicMediaSource refuses a false digest before the final source fragment
// reaches the official SDK. ExactReader owns extent and native source errors;
// the SDK owns multipart framing, HTTP cancellation and response closure.
type publicMediaSource struct {
	exact    *objectstore.ExactReader
	digest   *core.DigestWriter
	want     objectstore.Integrity
	mu       sync.Mutex
	length   uint64
	verified bool
	failure  error
}

func (r *publicMediaSource) Read(destination []byte) (int, error) {
	if len(destination) == 0 {
		return 0, nil
	}
	r.mu.Lock()
	if r.failure != nil {
		err := r.failure
		r.mu.Unlock()
		return 0, err
	}
	if r.verified {
		r.mu.Unlock()
		return 0, io.EOF
	}
	r.mu.Unlock()
	n, readErr := r.exact.Read(destination)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failure != nil {
		return 0, r.failure
	}
	if _, err := r.digest.Write(destination[:n]); err != nil {
		r.failure = errors.Join(core.ErrObjectStoreSource, err)
		return 0, r.failure
	}
	r.length += uint64(n)
	if readErr != nil && readErr != io.EOF {
		r.failure = errors.Join(core.ErrObjectStoreSource, readErr)
		return 0, r.failure
	}
	if r.length == r.want.Length.Uint64() && !r.verified {
		if r.length == 0 {
			if err := r.exact.ProveEmpty(); err != nil {
				r.failure = err
				return 0, err
			}
		}
		digest, extent, err := r.digest.Seal()
		if err != nil || digest != r.want.SHA256 || extent != r.want.Length {
			r.failure = errors.Join(core.ErrObjectStoreSource, core.ErrObjectStoreIntegrity, err)
			return 0, r.failure
		}
		r.verified = true
	}
	return n, readErr
}

func (r *publicMediaSource) result() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.failure != nil {
		return r.failure
	}
	if !r.verified {
		return core.ErrObjectStoreSource
	}
	return nil
}

// createPublicGCSMediaObject uses the official raw JSON API because the higher
// Storage Writer projects a malformed ACL in an SDK-owned goroutine and can
// kill the process. The shared authenticated client and endpoint are unchanged.
func createPublicGCSMediaObject(ctx context.Context, client *GCSClient, request gcsWrite) (GCSObjectMetadata, error) {
	if client.objects == nil {
		return GCSObjectMetadata{}, core.ErrObjectStoreContract
	}
	exact, err := objectstore.NewExactReader(request.Source, request.Integrity.Length)
	if err != nil {
		return GCSObjectMetadata{}, err
	}
	source := &publicMediaSource{exact: exact, digest: core.NewDigestWriter(), want: request.Integrity}
	checksum, err := request.Integrity.CRC32C.Base64()
	if err != nil {
		return GCSObjectMetadata{}, err
	}
	customTime, err := request.CustomTime.RFC3339Nano()
	if err != nil {
		return GCSObjectMetadata{}, err
	}
	writeContext, cancel := context.WithCancel(ctx)
	defer cancel()
	response, callErr := client.objects.Insert(request.Bucket.String(), &storageapi.Object{
		Name: request.Name.String(), ContentType: request.ContentType.String(),
		CacheControl: request.CacheControl.String(), CustomTime: customTime, Crc32c: checksum,
	}).Name(request.Name.String()).IfGenerationMatch(0).PredefinedAcl("publicRead").Projection("full").
		Media(source, googleapi.ChunkSize(0), googleapi.ContentType(request.ContentType.String())).Context(writeContext).Do()
	if sourceErr := source.result(); callErr != nil || sourceErr != nil {
		return GCSObjectMetadata{}, errors.Join(sourceErr, projectGCSError(callErr, core.ErrObjectStoreDestination))
	}
	metadata, err := publicMediaMetadata(response)
	if err != nil {
		return GCSObjectMetadata{}, err
	}
	if metadata.Bucket() != request.Bucket || metadata.Name() != request.Name || metadata.ContentType() != request.ContentType || metadata.Length() != request.Integrity.Length || metadata.CRC32C() != request.Integrity.CRC32C || metadata.CustomTime() != request.CustomTime || metadata.CacheControl() != request.CacheControl {
		return GCSObjectMetadata{}, core.ErrObjectStoreIntegrity
	}
	return metadata, nil
}

func publicMediaMetadata(response *storageapi.Object) (GCSObjectMetadata, error) {
	if response == nil {
		return GCSObjectMetadata{}, core.ErrObjectStoreContract
	}
	var acl []storage.ACLRule
	for _, entry := range response.Acl {
		if entry == nil {
			return GCSObjectMetadata{}, core.ErrObjectStoreContract
		}
		acl = append(acl, storage.ACLRule{Entity: storage.ACLEntity(entry.Entity), Role: storage.ACLRole(entry.Role)})
	}
	if !gcsACLHasPublicRead(acl) {
		return GCSObjectMetadata{}, errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreDestination)
	}
	created, createdErr := temporal.ParseRFC3339(response.TimeCreated)
	updated, updatedErr := temporal.ParseRFC3339(response.Updated)
	custom, customErr := temporal.ParseRFC3339(response.CustomTime)
	var checksum core.CRC32C
	crcErr := checksum.UnmarshalText([]byte(response.Crc32c))
	if err := errors.Join(createdErr, updatedErr, customErr, crcErr); err != nil {
		return GCSObjectMetadata{}, errors.Join(core.ErrObjectStoreContract, err)
	}
	createdAt, _ := created.Time()
	updatedAt, _ := updated.Time()
	customAt, _ := custom.Time()
	crc, _ := checksum.Uint32()
	length, err := core.NewByteLength(response.Size)
	if err != nil {
		return GCSObjectMetadata{}, err
	}
	size, err := length.Int64()
	if err != nil {
		return GCSObjectMetadata{}, err
	}
	return metadataFromGCSAttrs(&storage.ObjectAttrs{
		Bucket: response.Bucket, Name: response.Name, Generation: response.Generation,
		Size: size, CRC32C: crc, ContentType: response.ContentType,
		CacheControl: response.CacheControl, Created: createdAt, Updated: updatedAt, CustomTime: customAt,
	})
}
