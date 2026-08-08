package objectstore_test

import (
	"bytes"
	"errors"
	"hash/crc32"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestAuthenticatedGCSRequestValidationExhaustsCompositionFailures(t *testing.T) {
	t.Parallel()

	write, read, deleteRequest, deleteObjectRequest := authenticatedGCSRequestFixtures(t)
	tests := []struct {
		wantErr error
		run     func() error
		name    string
	}{
		{name: "complete create-only request crosses ingress", run: write.Validate},
		{name: "complete exact-read request crosses ingress", run: read.Validate},
		{name: "complete destructive-prefix request crosses ingress", run: deleteRequest.Validate},
		{name: "complete exact-object delete request crosses ingress", run: deleteObjectRequest.Validate},
		{name: "zero-byte create remains a real object operation", run: func() error {
			value := write
			value.Integrity = authenticatedGCSIntegrityAtLength(t, 0)
			return value.Validate()
		}},
		{name: "one-byte read ceiling remains admitted", run: func() error {
			value := read
			value.Maximum = authenticatedGCSMaximum(t, 1)
			return value.Validate()
		}},
		{name: "one-object destructive bound remains admitted", run: func() error {
			value := deleteRequest
			value.MaxObjects, _ = core.NewByteCount(1)
			return value.Validate()
		}},
		{name: "maximum destructive bound remains admitted", run: func() error {
			value := deleteRequest
			value.MaxObjects, _ = core.NewByteCount(objectstore.GCSDeleteMaximumObjects)
			return value.Validate()
		}},
		{name: "maximum GCS create extent remains admitted", run: func() error {
			value := write
			value.Integrity = authenticatedGCSIntegrityAtLength(t, objectstore.GoogleCloudStorageObjectMaximumBytes)
			return value.Validate()
		}},
		{name: "maximum GCS read ceiling remains admitted", run: func() error {
			value := read
			value.Maximum = authenticatedGCSMaximum(t, objectstore.GoogleCloudStorageObjectMaximumBytes)
			return value.Validate()
		}},
		{name: "application media metadata composes with binary source", run: write.Validate},
		{name: "create request refuses an unset bucket", run: func() error { value := write; value.Bucket = objectstore.GCSBucket{}; return value.Validate() }, wantErr: core.ErrObjectStoreContract},
		{name: "create request refuses an unset object name", run: func() error { value := write; value.Name = objectstore.GCSObjectName{}; return value.Validate() }, wantErr: core.ErrObjectStoreContract},
		{name: "create request refuses a nil source", run: func() error { value := write; value.Source = nil; return value.Validate() }, wantErr: core.ErrObjectStoreSource},
		{name: "create request refuses unset integrity", run: func() error { value := write; value.Integrity = objectstore.Integrity{}; return value.Validate() }, wantErr: core.ErrObjectStoreContract},
		{name: "create request refuses unset content type", run: func() error { value := write; value.ContentType = core.HTTPMediaType{}; return value.Validate() }, wantErr: core.ErrObjectStoreContract},
		{name: "create request refuses unset cache policy", run: func() error {
			value := write
			value.CacheControl = objectstore.GCSCacheControl{}
			return value.Validate()
		}, wantErr: core.ErrObjectStoreContract},
		{name: "create request refuses unset custom time", run: func() error { value := write; value.CustomTime = temporal.Instant{}; return value.Validate() }, wantErr: core.ErrObjectStoreContract},
		{name: "read request refuses an unset bucket", run: func() error { value := read; value.Bucket = objectstore.GCSBucket{}; return value.Validate() }, wantErr: core.ErrObjectStoreContract},
		{name: "read request refuses an unset object name", run: func() error { value := read; value.Name = objectstore.GCSObjectName{}; return value.Validate() }, wantErr: core.ErrObjectStoreContract},
		{name: "read request refuses a nil destination", run: func() error { value := read; value.Destination = nil; return value.Validate() }, wantErr: core.ErrObjectStoreDestination},
		{name: "read request refuses unset expected digest", run: func() error { value := read; value.SHA256 = core.SHA256Digest{}; return value.Validate() }, wantErr: core.ErrObjectStoreContract},
		{name: "read request refuses unset byte ceiling", run: func() error { value := read; value.Maximum = core.ByteCount{}; return value.Validate() }, wantErr: core.ErrObjectStoreContract},
		{name: "delete request refuses an unset bucket", run: func() error { value := deleteRequest; value.Bucket = objectstore.GCSBucket{}; return value.Validate() }, wantErr: core.ErrObjectStoreContract},
		{name: "delete request refuses an unset prefix", run: func() error {
			value := deleteRequest
			value.Prefix = objectstore.GCSObjectPrefix{}
			return value.Validate()
		}, wantErr: core.ErrObjectStoreContract},
		{name: "delete request refuses an unset bound", run: func() error { value := deleteRequest; value.MaxObjects = core.ByteCount{}; return value.Validate() }, wantErr: core.ErrObjectStoreContract},
		{name: "exact delete request refuses an unset bucket", run: func() error {
			value := deleteObjectRequest
			value.Bucket = objectstore.GCSBucket{}
			return value.Validate()
		}, wantErr: core.ErrObjectStoreContract},
		{name: "exact delete request refuses an unset object name", run: func() error {
			value := deleteObjectRequest
			value.Name = objectstore.GCSObjectName{}
			return value.Validate()
		}, wantErr: core.ErrObjectStoreContract},
		{name: "destructive bound one above ceiling is rejected", run: func() error {
			value := deleteRequest
			value.MaxObjects, _ = core.NewByteCount(objectstore.GCSDeleteMaximumObjects + 1)
			return value.Validate()
		}, wantErr: core.ErrObjectStoreContract},
		{name: "create extent one above GCS ceiling is rejected", run: func() error {
			value := write
			value.Integrity = authenticatedGCSIntegrityAtLength(t, objectstore.GoogleCloudStorageObjectMaximumBytes+1)
			return value.Validate()
		}, wantErr: core.ErrObjectStoreSize},
		{name: "read ceiling one above GCS maximum is rejected", run: func() error {
			value := read
			value.Maximum = authenticatedGCSMaximum(t, objectstore.GoogleCloudStorageObjectMaximumBytes+1)
			return value.Validate()
		}, wantErr: core.ErrObjectStoreSize},
		{name: "create extent one below GCS ceiling remains admitted", run: func() error {
			value := write
			value.Integrity = authenticatedGCSIntegrityAtLength(t, objectstore.GoogleCloudStorageObjectMaximumBytes-1)
			return value.Validate()
		}},
		{name: "read ceiling one below GCS maximum remains admitted", run: func() error {
			value := read
			value.Maximum = authenticatedGCSMaximum(t, objectstore.GoogleCloudStorageObjectMaximumBytes-1)
			return value.Validate()
		}},
		{name: "destructive bound one below ceiling remains admitted", run: func() error {
			value := deleteRequest
			value.MaxObjects, _ = core.NewByteCount(objectstore.GCSDeleteMaximumObjects - 1)
			return value.Validate()
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			gotErr := test.run()
			if !errors.Is(gotErr, test.wantErr) {
				t.Fatalf("authenticated GCS request validation error = %v, want errors.Is(..., %v)", gotErr, test.wantErr)
			}
		})
	}
}

func authenticatedGCSRequestFixtures(t *testing.T) (objectstore.GCSWriteRequest, objectstore.GCSReadRequest, objectstore.GCSDeleteRequest, objectstore.GCSDeleteObjectRequest) {
	t.Helper()
	bucket, gotBucketErr := objectstore.ParseGCSBucket("primitive-object-tests")
	if gotBucketErr != nil {
		t.Fatalf("ParseGCSBucket() error = %v, want nil", gotBucketErr)
	}
	name, gotNameErr := objectstore.ParseGCSObjectName("users/01/profile/photo.webp")
	if gotNameErr != nil {
		t.Fatalf("ParseGCSObjectName() error = %v, want nil", gotNameErr)
	}
	prefix, gotPrefixErr := objectstore.ParseGCSObjectPrefix("users/01/profile/")
	if gotPrefixErr != nil {
		t.Fatalf("ParseGCSObjectPrefix() error = %v, want nil", gotPrefixErr)
	}
	mediaType, gotMediaErr := core.ParseHTTPMediaType("image/webp")
	if gotMediaErr != nil {
		t.Fatalf("ParseHTTPMediaType() error = %v, want nil", gotMediaErr)
	}
	cacheControl, gotCacheErr := objectstore.ParseGCSCacheControl("private, no-store")
	if gotCacheErr != nil {
		t.Fatalf("ParseGCSCacheControl() error = %v, want nil", gotCacheErr)
	}
	maximum, gotMaximumErr := core.NewByteCount(2)
	if gotMaximumErr != nil {
		t.Fatalf("NewByteCount() error = %v, want nil", gotMaximumErr)
	}
	integrity := authenticatedGCSIntegrityAtLength(t, 7)
	write := objectstore.GCSWriteRequest{
		Bucket: bucket, Name: name, Source: bytes.NewReader([]byte("payload")),
		Integrity: integrity, ContentType: mediaType, CacheControl: cacheControl,
		CustomTime: temporal.InstantFromNanoseconds(1_786_183_200_000_000_000),
	}
	read := objectstore.GCSReadRequest{
		Bucket: bucket, Name: name, Destination: &bytes.Buffer{},
		SHA256: integrity.SHA256, Maximum: authenticatedGCSMaximum(t, 7),
	}
	deleteRequest := objectstore.GCSDeleteRequest{Bucket: bucket, Prefix: prefix, MaxObjects: maximum}
	deleteObjectRequest := objectstore.GCSDeleteObjectRequest{Bucket: bucket, Name: name}
	return write, read, deleteRequest, deleteObjectRequest
}

func authenticatedGCSMaximum(t *testing.T, value uint64) core.ByteCount {
	t.Helper()
	maximum, gotMaximumErr := core.NewByteCount(value)
	if gotMaximumErr != nil {
		t.Fatalf("NewByteCount(%d) error = %v, want nil", value, gotMaximumErr)
	}
	return maximum
}

func authenticatedGCSIntegrityAtLength(t *testing.T, lengthValue uint64) objectstore.Integrity {
	t.Helper()
	length, gotLengthErr := core.NewByteLength(lengthValue)
	if gotLengthErr != nil {
		t.Fatalf("NewByteLength(%d) error = %v, want nil", lengthValue, gotLengthErr)
	}
	data := []byte("independent integrity fixture")
	return objectstore.Integrity{
		SHA256: core.SHA256Of(data), Length: length,
		CRC32C: core.NewCRC32C(crc32.Checksum(data, crc32.MakeTable(crc32.Castagnoli))),
	}
}
