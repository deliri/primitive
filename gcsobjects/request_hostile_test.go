package gcsobjects_test

import (
	"bytes"
	"errors"
	"hash/crc32"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/gcsobjects"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestAuthenticatedGCSRequestValidationExhaustsCompositionFailures(t *testing.T) {
	t.Parallel()

	media, file, read, deleteRequest, deleteObjectRequest := authenticatedGCSRequestFixtures(t)
	tests := []struct {
		wantErr error
		run     func() error
		name    string
	}{
		{name: "complete served-media create crosses ingress", run: media.Validate},
		{name: "complete stored-file create crosses ingress", run: file.Validate},
		{name: "complete exact-read request crosses ingress", run: read.Validate},
		{name: "complete destructive-prefix request crosses ingress", run: deleteRequest.Validate},
		{name: "complete exact-object delete request crosses ingress", run: deleteObjectRequest.Validate},
		{name: "zero-byte served media remains a real object operation", run: func() error {
			value := media
			value.Integrity = authenticatedGCSIntegrityAtLength(t, 0)
			return value.Validate()
		}},
		{name: "zero-byte stored file remains a real object operation", run: func() error {
			value := file
			value.Integrity = authenticatedGCSIntegrityAtLength(t, 0)
			return value.Validate()
		}},
		{name: "one-byte read ceiling remains admitted", run: func() error {
			value := read
			setAuthenticatedGCSReadLength(t, &value, 1)
			return value.Validate()
		}},
		{name: "one-object destructive bound remains admitted", run: func() error {
			value := deleteRequest
			value.MaxObjects, _ = core.NewByteCount(1)
			return value.Validate()
		}},
		{name: "maximum destructive bound remains admitted", run: func() error {
			value := deleteRequest
			value.MaxObjects, _ = core.NewByteCount(gcsobjects.GCSDeleteMaximumObjects)
			return value.Validate()
		}},
		{name: "maximum GCS media extent remains admitted", run: func() error {
			value := media
			value.Integrity = authenticatedGCSIntegrityAtLength(t, objectstore.GoogleCloudStorageObjectMaximumBytes)
			return value.Validate()
		}},
		{name: "maximum GCS file extent remains admitted", run: func() error {
			value := file
			value.Integrity = authenticatedGCSIntegrityAtLength(t, objectstore.GoogleCloudStorageObjectMaximumBytes)
			return value.Validate()
		}},
		{name: "maximum GCS read ceiling remains admitted", run: func() error {
			value := read
			setAuthenticatedGCSReadLength(t, &value, objectstore.GoogleCloudStorageObjectMaximumBytes)
			return value.Validate()
		}},
		{name: "media create refuses an unset bucket", run: func() error { value := media; value.Bucket = gcsobjects.GCSBucket{}; return value.Validate() }, wantErr: core.ErrObjectStoreContract},
		{name: "media create refuses an unset object name", run: func() error { value := media; value.Name = gcsobjects.GCSObjectName{}; return value.Validate() }, wantErr: core.ErrObjectStoreContract},
		{name: "media create refuses a nil source", run: func() error { value := media; value.Source = nil; return value.Validate() }, wantErr: core.ErrObjectStoreSource},
		{name: "media create refuses unset integrity", run: func() error { value := media; value.Integrity = objectstore.Integrity{}; return value.Validate() }, wantErr: core.ErrObjectStoreContract},
		{name: "media create refuses unset content type", run: func() error { value := media; value.ContentType = core.HTTPMediaType{}; return value.Validate() }, wantErr: core.ErrObjectStoreContract},
		{name: "media create admits an absent cache policy", run: func() error {
			value := media
			value.CacheControl = gcsobjects.GCSCacheControl{}
			return value.Validate()
		}},
		{name: "media create refuses unset custom time", run: func() error { value := media; value.CustomTime = temporal.Instant{}; return value.Validate() }, wantErr: core.ErrObjectStoreContract},
		{name: "file create refuses an unset bucket", run: func() error { value := file; value.Bucket = gcsobjects.GCSBucket{}; return value.Validate() }, wantErr: core.ErrObjectStoreContract},
		{name: "file create refuses an unset object name", run: func() error { value := file; value.Name = gcsobjects.GCSObjectName{}; return value.Validate() }, wantErr: core.ErrObjectStoreContract},
		{name: "file create refuses a nil source", run: func() error { value := file; value.Source = nil; return value.Validate() }, wantErr: core.ErrObjectStoreSource},
		{name: "file create refuses unset integrity", run: func() error { value := file; value.Integrity = objectstore.Integrity{}; return value.Validate() }, wantErr: core.ErrObjectStoreContract},
		{name: "file create refuses unset custom time", run: func() error { value := file; value.CustomTime = temporal.Instant{}; return value.Validate() }, wantErr: core.ErrObjectStoreContract},
		{name: "read request refuses an unset bucket", run: func() error { value := read; value.Bucket = gcsobjects.GCSBucket{}; return value.Validate() }, wantErr: core.ErrObjectStoreContract},
		{name: "read request refuses an unset object name", run: func() error { value := read; value.Name = gcsobjects.GCSObjectName{}; return value.Validate() }, wantErr: core.ErrObjectStoreContract},
		{name: "read request refuses an unset destination", run: func() error {
			value := read
			value.Destination = filestore.StageDestinationRequest{}
			return value.Validate()
		}, wantErr: core.ErrObjectStoreContract},
		{name: "read request refuses unset expected digest", run: func() error { value := read; value.Integrity.SHA256 = core.SHA256Digest{}; return value.Validate() }, wantErr: core.ErrObjectStoreContract},
		{name: "read request refuses unset exact extent", run: func() error { value := read; value.Integrity.Length = core.ByteLength{}; return value.Validate() }, wantErr: core.ErrObjectStoreContract},
		{name: "delete request refuses an unset bucket", run: func() error { value := deleteRequest; value.Bucket = gcsobjects.GCSBucket{}; return value.Validate() }, wantErr: core.ErrObjectStoreContract},
		{name: "delete request refuses an unset prefix", run: func() error {
			value := deleteRequest
			value.Prefix = gcsobjects.GCSObjectPrefix{}
			return value.Validate()
		}, wantErr: core.ErrObjectStoreContract},
		{name: "delete request refuses an unset bound", run: func() error { value := deleteRequest; value.MaxObjects = core.ByteCount{}; return value.Validate() }, wantErr: core.ErrObjectStoreContract},
		{name: "exact delete request refuses an unset bucket", run: func() error {
			value := deleteObjectRequest
			value.Bucket = gcsobjects.GCSBucket{}
			return value.Validate()
		}, wantErr: core.ErrObjectStoreContract},
		{name: "exact delete request refuses an unset object name", run: func() error {
			value := deleteObjectRequest
			value.Name = gcsobjects.GCSObjectName{}
			return value.Validate()
		}, wantErr: core.ErrObjectStoreContract},
		{name: "destructive bound one above ceiling is rejected", run: func() error {
			value := deleteRequest
			value.MaxObjects, _ = core.NewByteCount(gcsobjects.GCSDeleteMaximumObjects + 1)
			return value.Validate()
		}, wantErr: core.ErrObjectStoreContract},
		{name: "media extent one above GCS ceiling is rejected", run: func() error {
			value := media
			value.Integrity = authenticatedGCSIntegrityAtLength(t, objectstore.GoogleCloudStorageObjectMaximumBytes+1)
			return value.Validate()
		}, wantErr: core.ErrObjectStoreSize},
		{name: "file extent one above GCS ceiling is rejected", run: func() error {
			value := file
			value.Integrity = authenticatedGCSIntegrityAtLength(t, objectstore.GoogleCloudStorageObjectMaximumBytes+1)
			return value.Validate()
		}, wantErr: core.ErrObjectStoreSize},
		{name: "read ceiling one above GCS maximum is rejected", run: func() error {
			value := read
			setAuthenticatedGCSReadLength(t, &value, objectstore.GoogleCloudStorageObjectMaximumBytes+1)
			return value.Validate()
		}, wantErr: core.ErrObjectStoreSize},
		{name: "media extent one below GCS ceiling remains admitted", run: func() error {
			value := media
			value.Integrity = authenticatedGCSIntegrityAtLength(t, objectstore.GoogleCloudStorageObjectMaximumBytes-1)
			return value.Validate()
		}},
		{name: "read ceiling one below GCS maximum remains admitted", run: func() error {
			value := read
			setAuthenticatedGCSReadLength(t, &value, objectstore.GoogleCloudStorageObjectMaximumBytes-1)
			return value.Validate()
		}},
		{name: "destructive bound one below ceiling remains admitted", run: func() error {
			value := deleteRequest
			value.MaxObjects, _ = core.NewByteCount(gcsobjects.GCSDeleteMaximumObjects - 1)
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

func authenticatedGCSRequestFixtures(t *testing.T) (
	gcsobjects.GCSMediaUpload,
	gcsobjects.GCSFileUpload,
	gcsobjects.GCSReadRequest,
	gcsobjects.GCSDeleteRequest,
	gcsobjects.GCSDeleteObjectRequest,
) {
	t.Helper()
	bucket, gotBucketErr := gcsobjects.ParseGCSBucket("primitive-object-tests")
	if gotBucketErr != nil {
		t.Fatalf("ParseGCSBucket() error = %v, want nil", gotBucketErr)
	}
	name, gotNameErr := gcsobjects.ParseGCSObjectName("users/01/profile/photo.webp")
	if gotNameErr != nil {
		t.Fatalf("ParseGCSObjectName() error = %v, want nil", gotNameErr)
	}
	prefix, gotPrefixErr := gcsobjects.ParseGCSObjectPrefix("users/01/profile/")
	if gotPrefixErr != nil {
		t.Fatalf("ParseGCSObjectPrefix() error = %v, want nil", gotPrefixErr)
	}
	mediaType, gotMediaErr := core.ParseHTTPMediaType("image/webp")
	if gotMediaErr != nil {
		t.Fatalf("ParseHTTPMediaType() error = %v, want nil", gotMediaErr)
	}
	cacheControl, gotCacheErr := gcsobjects.ParseGCSCacheControl("private, no-store")
	if gotCacheErr != nil {
		t.Fatalf("ParseGCSCacheControl() error = %v, want nil", gotCacheErr)
	}
	maximum, gotMaximumErr := core.NewByteCount(2)
	if gotMaximumErr != nil {
		t.Fatalf("NewByteCount() error = %v, want nil", gotMaximumErr)
	}
	integrity := authenticatedGCSIntegrityAtLength(t, 7)
	customTime := temporal.InstantFromNanoseconds(1_786_183_200_000_000_000)
	media := gcsobjects.GCSMediaUpload{
		Bucket: bucket, Name: name, Source: bytes.NewReader([]byte("payload")),
		Integrity: integrity, ContentType: mediaType, CacheControl: cacheControl,
		CustomTime: customTime,
	}
	file := gcsobjects.GCSFileUpload{
		Bucket: bucket, Name: name, Source: bytes.NewReader([]byte("payload")),
		Integrity: integrity, CustomTime: customTime,
	}
	read := liveGCSReadRequest(t, bucket, name, integrity)
	deleteRequest := gcsobjects.GCSDeleteRequest{Bucket: bucket, Prefix: prefix, MaxObjects: maximum}
	deleteObjectRequest := gcsobjects.GCSDeleteObjectRequest{Bucket: bucket, Name: name}
	return media, file, read, deleteRequest, deleteObjectRequest
}

func setAuthenticatedGCSReadLength(t *testing.T, request *gcsobjects.GCSReadRequest, value uint64) {
	t.Helper()
	length, err := core.NewByteLength(value)
	if err != nil {
		t.Fatalf("core.NewByteLength(%d) error = %v, want nil", value, err)
	}
	request.Integrity.Length = length
	request.Destination.ExpectedBytes = length
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
