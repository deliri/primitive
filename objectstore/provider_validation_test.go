package objectstore

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"hash/crc32"
	"io"
	"net/url"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestUploadProviderValidationHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		mutate   func(*UploadRequest) error
		wantErr  error
		name     string
		provider Provider
	}{
		{name: "S3 complete capability is accepted", provider: ProviderAmazonS3},
		{name: "GCS complete capability is accepted", provider: ProviderGoogleCloudStorage},
		{name: "Cloudflare complete capability is accepted", provider: ProviderCloudflareImages},
		{name: "unknown provider is rejected", provider: ProviderUnknown, wantErr: core.ErrObjectStoreContract},
		{name: "future provider is rejected", provider: providerLimit, wantErr: core.ErrObjectStoreContract},
		{name: "S3 missing signature is rejected", provider: ProviderAmazonS3, mutate: setUploadURL("https://s3.amazonaws.com/bucket/object?X-Amz-SignedHeaders=" + url.QueryEscape(s3UploadSignedHeaderFixture())), wantErr: core.ErrObjectStoreContract},
		{name: "S3 missing signed-header declaration is rejected", provider: ProviderAmazonS3, mutate: setUploadURL("https://s3.amazonaws.com/bucket/object?X-Amz-Signature=sig"), wantErr: core.ErrObjectStoreContract},
		{name: "S3 duplicate signature is rejected", provider: ProviderAmazonS3, mutate: appendUploadQuery("&X-Amz-Signature=second"), wantErr: core.ErrObjectStoreContract},
		{name: "S3 duplicate signed-header declaration is rejected", provider: ProviderAmazonS3, mutate: appendUploadQuery("&X-Amz-SignedHeaders=" + url.QueryEscape(s3UploadSignedHeaderFixture())), wantErr: core.ErrObjectStoreContract},
		{name: "S3-shaped capability on a foreign host is rejected", provider: ProviderAmazonS3, mutate: setUploadURL("https://attacker.example/object?X-Amz-Signature=sig&X-Amz-SignedHeaders=" + url.QueryEscape(s3UploadSignedHeaderFixture())), wantErr: core.ErrObjectStoreContract},
		{name: "S3 unsigned checksum field is rejected", provider: ProviderAmazonS3, mutate: setUploadURL("https://s3.amazonaws.com/bucket/object?X-Amz-Signature=sig&X-Amz-SignedHeaders=" + url.QueryEscape("host;if-none-match")), wantErr: core.ErrObjectStoreContract},
		{name: "S3 unsigned create-only field is rejected", provider: ProviderAmazonS3, mutate: setUploadURL("https://s3.amazonaws.com/bucket/object?X-Amz-Signature=sig&X-Amz-SignedHeaders=" + url.QueryEscape("host;x-amz-checksum-crc32c")), wantErr: core.ErrObjectStoreContract},
		{name: "GCS missing signature is rejected", provider: ProviderGoogleCloudStorage, mutate: setUploadURL("https://storage.googleapis.com/bucket/object?X-Goog-SignedHeaders=" + url.QueryEscape(gcsUploadSignedHeaderFixture())), wantErr: core.ErrObjectStoreContract},
		{name: "GCS missing signed-header declaration is rejected", provider: ProviderGoogleCloudStorage, mutate: setUploadURL("https://storage.googleapis.com/bucket/object?X-Goog-Signature=sig"), wantErr: core.ErrObjectStoreContract},
		{name: "GCS unsigned checksum field is rejected", provider: ProviderGoogleCloudStorage, mutate: setUploadURL("https://storage.googleapis.com/bucket/object?X-Goog-Signature=sig&X-Goog-SignedHeaders=" + url.QueryEscape("host;x-goog-if-generation-match")), wantErr: core.ErrObjectStoreContract},
		{name: "GCS unsigned create-only field is rejected", provider: ProviderGoogleCloudStorage, mutate: setUploadURL("https://storage.googleapis.com/bucket/object?X-Goog-Signature=sig&X-Goog-SignedHeaders=" + url.QueryEscape("host;x-goog-hash")), wantErr: core.ErrObjectStoreContract},
		{name: "GCS declared caller field omitted from request is rejected", provider: ProviderGoogleCloudStorage, mutate: setUploadURL("https://storage.googleapis.com/bucket/object?X-Goog-Signature=sig&X-Goog-SignedHeaders=" + url.QueryEscape("host;x-goog-hash;x-goog-if-generation-match;x-goog-meta-missing")), wantErr: core.ErrObjectStoreContract},
		{name: "GCS-shaped capability on a foreign host is rejected", provider: ProviderGoogleCloudStorage, mutate: setUploadURL("https://attacker.example/object?X-Goog-Signature=sig&X-Goog-SignedHeaders=" + url.QueryEscape(gcsUploadSignedHeaderFixture())), wantErr: core.ErrObjectStoreContract},
		{name: "GCS capability on a nonstandard port is rejected", provider: ProviderGoogleCloudStorage, mutate: setUploadURL("https://storage.googleapis.com:8443/bucket/object?X-Goog-Signature=sig&X-Goog-SignedHeaders=" + url.QueryEscape(gcsUploadSignedHeaderFixture())), wantErr: core.ErrObjectStoreContract},
		{name: "Cloudflare alternate host is rejected", provider: ProviderCloudflareImages, mutate: setUploadURL("https://example.com/image-id"), wantErr: core.ErrObjectStoreContract},
		{name: "Cloudflare nonstandard port is rejected", provider: ProviderCloudflareImages, mutate: setUploadURL("https://upload.imagedelivery.net:8443/image-id"), wantErr: core.ErrObjectStoreContract},
		{name: "S3 one below PutObject maximum is accepted", provider: ProviderAmazonS3, mutate: setUploadLength(AmazonS3PutObjectMaximumBytes - 1)},
		{name: "S3 exact PutObject maximum is accepted", provider: ProviderAmazonS3, mutate: setUploadLength(AmazonS3PutObjectMaximumBytes)},
		{name: "S3 one above PutObject maximum is rejected", provider: ProviderAmazonS3, mutate: setUploadLength(AmazonS3PutObjectMaximumBytes + 1), wantErr: core.ErrObjectStoreSize},
		{name: "GCS one below object maximum is accepted", provider: ProviderGoogleCloudStorage, mutate: setUploadLength(GoogleCloudStorageObjectMaximumBytes - 1)},
		{name: "GCS exact object maximum is accepted", provider: ProviderGoogleCloudStorage, mutate: setUploadLength(GoogleCloudStorageObjectMaximumBytes)},
		{name: "GCS one above object maximum is rejected", provider: ProviderGoogleCloudStorage, mutate: setUploadLength(GoogleCloudStorageObjectMaximumBytes + 1), wantErr: core.ErrObjectStoreSize},
		{name: "Cloudflare one below image maximum is accepted", provider: ProviderCloudflareImages, mutate: setUploadLength(CloudflareImagesUploadMaximumBytes - 1)},
		{name: "Cloudflare exact image maximum is accepted", provider: ProviderCloudflareImages, mutate: setUploadLength(CloudflareImagesUploadMaximumBytes)},
		{name: "Cloudflare one above image maximum is rejected", provider: ProviderCloudflareImages, mutate: setUploadLength(CloudflareImagesUploadMaximumBytes + 1), wantErr: core.ErrObjectStoreSize},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := providerUploadRequest(t, tc.provider)
			if tc.mutate != nil {
				if err := tc.mutate(&request); err != nil {
					t.Fatalf("upload mutation setup error = %v, want nil", err)
				}
			}
			gotErr := request.validateFor(tc.provider)
			if tc.wantErr == nil {
				if gotErr != nil {
					t.Fatalf("UploadRequest.validateFor() error = %v, want nil", gotErr)
				}
				return
			}
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf(
					"UploadRequest.validateFor() error = %v, want %v",
					gotErr,
					tc.wantErr,
				)
			}
		})
	}
}

func TestDownloadProviderValidationHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		mutate   func(*DownloadRequest) error
		wantErr  error
		name     string
		provider Provider
	}{
		{name: "S3 complete capability is accepted", provider: ProviderAmazonS3},
		{name: "GCS complete capability is accepted", provider: ProviderGoogleCloudStorage},
		{name: "Cloudflare upload-only capability is rejected", provider: ProviderCloudflareImages, wantErr: core.ErrObjectStoreContract},
		{name: "unknown provider is rejected", provider: ProviderUnknown, wantErr: core.ErrObjectStoreContract},
		{name: "future provider is rejected", provider: providerLimit, wantErr: core.ErrObjectStoreContract},
		{name: "S3 missing signed checksum mode is rejected", provider: ProviderAmazonS3, mutate: setDownloadURL("https://s3.amazonaws.com/bucket/object?X-Amz-Signature=sig&X-Amz-SignedHeaders=host"), wantErr: core.ErrObjectStoreContract},
		{name: "S3 missing signature is rejected", provider: ProviderAmazonS3, mutate: setDownloadURL("https://s3.amazonaws.com/bucket/object?X-Amz-SignedHeaders=" + url.QueryEscape(s3DownloadSignedHeaderFixture())), wantErr: core.ErrObjectStoreContract},
		{name: "S3 duplicate signature is rejected", provider: ProviderAmazonS3, mutate: appendDownloadQuery("&X-Amz-Signature=second"), wantErr: core.ErrObjectStoreContract},
		{name: "S3-shaped capability on a foreign host is rejected", provider: ProviderAmazonS3, mutate: setDownloadURL("https://attacker.example/object?X-Amz-Signature=sig&X-Amz-SignedHeaders=" + url.QueryEscape(s3DownloadSignedHeaderFixture())), wantErr: core.ErrObjectStoreContract},
		{name: "GCS missing signature is rejected", provider: ProviderGoogleCloudStorage, mutate: setDownloadURL("https://storage.googleapis.com/bucket/object?X-Goog-SignedHeaders=" + url.QueryEscape(gcsDownloadSignedHeaderFixture())), wantErr: core.ErrObjectStoreContract},
		{name: "GCS duplicate signed-header declaration is rejected", provider: ProviderGoogleCloudStorage, mutate: appendDownloadQuery("&X-Goog-SignedHeaders=" + url.QueryEscape(gcsDownloadSignedHeaderFixture())), wantErr: core.ErrObjectStoreContract},
		{name: "GCS declared caller field omitted from request is rejected", provider: ProviderGoogleCloudStorage, mutate: setDownloadURL("https://storage.googleapis.com/bucket/object?X-Goog-Signature=sig&X-Goog-SignedHeaders=" + url.QueryEscape("host;x-goog-meta-missing")), wantErr: core.ErrObjectStoreContract},
		{name: "GCS-shaped capability on a foreign host is rejected", provider: ProviderGoogleCloudStorage, mutate: setDownloadURL("https://attacker.example/object?X-Goog-Signature=sig&X-Goog-SignedHeaders=" + url.QueryEscape(gcsDownloadSignedHeaderFixture())), wantErr: core.ErrObjectStoreContract},
		{name: "S3 one below object maximum is accepted", provider: ProviderAmazonS3, mutate: setDownloadLength(AmazonS3ObjectMaximumBytes - 1)},
		{name: "S3 exact object maximum is accepted", provider: ProviderAmazonS3, mutate: setDownloadLength(AmazonS3ObjectMaximumBytes)},
		{name: "S3 one above object maximum is rejected", provider: ProviderAmazonS3, mutate: setDownloadLength(AmazonS3ObjectMaximumBytes + 1), wantErr: core.ErrObjectStoreSize},
		{name: "GCS one below object maximum is accepted", provider: ProviderGoogleCloudStorage, mutate: setDownloadLength(GoogleCloudStorageObjectMaximumBytes - 1)},
		{name: "GCS exact object maximum is accepted", provider: ProviderGoogleCloudStorage, mutate: setDownloadLength(GoogleCloudStorageObjectMaximumBytes)},
		{name: "GCS one above object maximum is rejected", provider: ProviderGoogleCloudStorage, mutate: setDownloadLength(GoogleCloudStorageObjectMaximumBytes + 1), wantErr: core.ErrObjectStoreSize},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := providerDownloadRequest(t, tc.provider)
			if tc.mutate != nil {
				if err := tc.mutate(&request); err != nil {
					t.Fatalf("download mutation setup error = %v, want nil", err)
				}
			}
			gotErr := request.validateFor(tc.provider)
			if tc.wantErr == nil {
				if gotErr != nil {
					t.Fatalf("DownloadRequest.validateFor() error = %v, want nil", gotErr)
				}
				return
			}
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf(
					"DownloadRequest.validateFor() error = %v, want %v",
					gotErr,
					tc.wantErr,
				)
			}
		})
	}
}

func providerUploadRequest(t *testing.T, provider Provider) UploadRequest {
	t.Helper()

	payload := []byte("provider-validation")
	return UploadRequest{
		Source:      bytes.NewReader(payload),
		ContentType: core.HTTPMediaTypeOctetStream(),
		Target:      providerUploadTarget(t, provider),
		Integrity:   providerIntegrity(t, payload),
		Policy:      providerPolicy(t),
	}
}

func providerDownloadRequest(t *testing.T, provider Provider) DownloadRequest {
	t.Helper()

	payload := []byte("provider-validation")
	return DownloadRequest{
		Destination: io.Discard,
		ContentType: core.HTTPMediaTypeOctetStream(),
		Target: DownloadTarget{
			URL:       providerSignedURL(t, provider, DirectionDownload),
			ExpiresAt: providerFutureInstant(t),
		},
		Integrity: providerIntegrity(t, payload),
		Policy:    providerPolicy(t),
	}
}

func providerUploadTarget(t testing.TB, provider Provider) UploadTarget {
	t.Helper()

	return UploadTarget{
		URL:       providerSignedURL(t, provider, DirectionUpload),
		ExpiresAt: providerFutureInstant(t),
	}
}

func providerSignedURL(
	t testing.TB,
	provider Provider,
	direction Direction,
) SignedURL {
	t.Helper()

	value := "https://example.com/object"
	switch provider {
	case ProviderAmazonS3:
		signed := s3UploadSignedHeaderFixture()
		if direction == DirectionDownload {
			signed = s3DownloadSignedHeaderFixture()
		}
		value = "https://s3.amazonaws.com/bucket/object?X-Amz-Signature=sig&X-Amz-SignedHeaders=" +
			url.QueryEscape(signed)
	case ProviderGoogleCloudStorage:
		signed := gcsUploadSignedHeaderFixture()
		if direction == DirectionDownload {
			signed = gcsDownloadSignedHeaderFixture()
		}
		value = "https://storage.googleapis.com/bucket/object?X-Goog-Signature=sig&X-Goog-SignedHeaders=" +
			url.QueryEscape(signed)
	case ProviderCloudflareImages:
		value = "https://upload.imagedelivery.net/image-id"
	case ProviderUnknown, providerLimit:
	}
	parsed, err := ParseSignedURL(value)
	if err != nil {
		t.Fatalf("ParseSignedURL() setup error = %v, want nil", err)
	}
	return parsed
}

func providerFutureInstant(t testing.TB) temporal.Instant {
	t.Helper()

	value, err := temporal.NewInstant(
		time.Date(2035, time.January, 1, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("temporal.NewInstant() setup error = %v, want nil", err)
	}
	return value
}

func providerPolicy(t *testing.T) Policy {
	t.Helper()

	operation, err := temporal.DurationFromSeconds(10)
	if err != nil {
		t.Fatalf("temporal.DurationFromSeconds(10) error = %v, want nil", err)
	}
	attempt, err := temporal.DurationFromSeconds(5)
	if err != nil {
		t.Fatalf("temporal.DurationFromSeconds(5) error = %v, want nil", err)
	}
	errorLimit, err := core.NewByteCount(4096)
	if err != nil {
		t.Fatalf("core.NewByteCount() error = %v, want nil", err)
	}
	return Policy{
		OperationTimeout: operation,
		AttemptTimeout:   attempt,
		ErrorBodyLimit:   errorLimit,
	}
}

func providerIntegrity(t *testing.T, payload []byte) Integrity {
	t.Helper()
	sha := sha256.Sum256(payload)
	return Integrity{
		SHA256: core.NewSHA256Digest(sha),
		Length: mustByteLength(t, uint64(len(payload))),
		CRC32C: core.NewCRC32C(
			crc32.Checksum(payload, crc32.MakeTable(crc32.Castagnoli)),
		),
	}
}

func setUploadURL(value string) func(*UploadRequest) error {
	return func(request *UploadRequest) error {
		parsed, err := ParseSignedURL(value)
		if err != nil {
			return err
		}
		request.Target.URL = parsed
		return nil
	}
}

func appendUploadQuery(suffix string) func(*UploadRequest) error {
	return func(request *UploadRequest) error {
		value := request.Target.URL.value.String() + suffix
		parsed, err := ParseSignedURL(value)
		if err != nil {
			return err
		}
		request.Target.URL = parsed
		return nil
	}
}

func setUploadLength(value uint64) func(*UploadRequest) error {
	return func(request *UploadRequest) error {
		length, err := core.NewByteLength(value)
		request.Integrity.Length = length
		return err
	}
}

func setDownloadURL(value string) func(*DownloadRequest) error {
	return func(request *DownloadRequest) error {
		parsed, err := ParseSignedURL(value)
		if err != nil {
			return err
		}
		request.Target.URL = parsed
		return nil
	}
}

func appendDownloadQuery(suffix string) func(*DownloadRequest) error {
	return func(request *DownloadRequest) error {
		value := request.Target.URL.value.String() + suffix
		parsed, err := ParseSignedURL(value)
		if err != nil {
			return err
		}
		request.Target.URL = parsed
		return nil
	}
}

func setDownloadLength(value uint64) func(*DownloadRequest) error {
	return func(request *DownloadRequest) error {
		length, err := core.NewByteLength(value)
		request.Integrity.Length = length
		return err
	}
}

func s3UploadSignedHeaderFixture() string {
	return "host;if-none-match;x-amz-checksum-crc32c"
}

func s3DownloadSignedHeaderFixture() string { return "host;x-amz-checksum-mode" }

func gcsUploadSignedHeaderFixture() string {
	return "host;x-goog-hash;x-goog-if-generation-match"
}

func gcsDownloadSignedHeaderFixture() string { return "host" }
