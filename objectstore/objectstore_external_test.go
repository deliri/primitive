package objectstore_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	testS3UploadSignedHeaders    = "host;if-none-match;x-amz-checksum-crc32c"
	testS3DownloadSignedHeaders  = "host;x-amz-checksum-mode"
	testGCSUploadSignedHeaders   = "host;x-goog-hash;x-goog-if-generation-match"
	testGCSDownloadSignedHeaders = "host"
)

type providerObservation struct {
	method            string
	contentType       string
	checksum          string
	createOnly        string
	checksumMode      string
	multipartFilename string
	multipartField    string
	body              []byte
}

type offWireEnum interface {
	~uint8
	Validate() error
	IsValid() bool
	String() string
	OffWireEnum()
}

type enumExpectation[Enum offWireEnum] struct {
	value    Enum
	wantText string
}

type uploadOperation func(
	context.Context,
	objectstore.Client,
	objectstore.UploadRequest,
) (objectstore.Transfer, error)

type downloadOperation func(
	context.Context,
	objectstore.Client,
	objectstore.DownloadRequest,
) (objectstore.Transfer, error)

func uploadOperationFor(provider objectstore.Provider) uploadOperation {
	switch provider {
	case objectstore.ProviderAmazonS3:
		return objectstore.UploadS3
	case objectstore.ProviderGoogleCloudStorage:
		return objectstore.UploadGCS
	case objectstore.ProviderCloudflareImages:
		return objectstore.UploadCloudflareImages
	default:
		return nil
	}
}

func downloadOperationFor(provider objectstore.Provider) downloadOperation {
	switch provider {
	case objectstore.ProviderAmazonS3:
		return objectstore.DownloadS3
	case objectstore.ProviderGoogleCloudStorage:
		return objectstore.DownloadGCS
	case objectstore.ProviderCloudflareImages:
		return nil
	default:
		return nil
	}
}

func TestOffWireEnumExhaustiveDomains(t *testing.T) {
	t.Parallel()

	exerciseEnumDomain(t, []enumExpectation[objectstore.Provider]{
		{value: objectstore.ProviderAmazonS3, wantText: "amazon_s3"},
		{value: objectstore.ProviderGoogleCloudStorage, wantText: "google_cloud_storage"},
		{value: objectstore.ProviderCloudflareImages, wantText: "cloudflare_images"},
	})
	exerciseEnumDomain(t, []enumExpectation[objectstore.VendorAPI]{
		{value: objectstore.VendorAPIAmazonS3Object, wantText: "amazon_s3_object"},
		{value: objectstore.VendorAPIGoogleCloudStorageXML, wantText: "google_cloud_storage_xml"},
		{value: objectstore.VendorAPICloudflareImagesDirect, wantText: "cloudflare_images_direct"},
	})
	exerciseEnumDomain(t, []enumExpectation[objectstore.DirectionCapability]{
		{value: objectstore.DirectionCapabilityUploadOnly, wantText: "upload_only"},
		{value: objectstore.DirectionCapabilityUploadDownload, wantText: "upload_download"},
	})
	exerciseEnumDomain(t, []enumExpectation[objectstore.UploadEncoding]{
		{value: objectstore.UploadEncodingRawObject, wantText: "raw_object"},
		{value: objectstore.UploadEncodingMultipartFile, wantText: "multipart_file"},
	})
	exerciseEnumDomain(t, []enumExpectation[objectstore.ProviderIntegrity]{
		{value: objectstore.ProviderIntegrityCRC32C, wantText: "crc32c"},
		{value: objectstore.ProviderIntegrityLocalOnly, wantText: "local_only"},
	})
	exerciseEnumDomain(t, []enumExpectation[objectstore.WritePreference]{
		{value: objectstore.WritePreferenceCreateOnly, wantText: "create_only"},
		{value: objectstore.WritePreferenceOneTimeCapability, wantText: "one_time_capability"},
	})
	exerciseEnumDomain(t, []enumExpectation[objectstore.Direction]{
		{value: objectstore.DirectionUpload, wantText: "upload"},
		{value: objectstore.DirectionDownload, wantText: "download"},
	})
	exerciseEnumDomain(t, []enumExpectation[objectstore.Commitment]{
		{value: objectstore.CommitmentNotAttempted, wantText: "not_attempted"},
		{value: objectstore.CommitmentRejected, wantText: "rejected"},
		{value: objectstore.CommitmentConfirmed, wantText: "confirmed"},
		{value: objectstore.CommitmentIndeterminate, wantText: "indeterminate"},
	})
}

func exerciseEnumDomain[Enum offWireEnum](
	t *testing.T,
	valid []enumExpectation[Enum],
) {
	t.Helper()

	for raw := range 256 {
		value := Enum(raw)
		wantText := ""
		wantValid := false
		for _, admitted := range valid {
			if value == admitted.value {
				wantText = admitted.wantText
				wantValid = true
				break
			}
		}
		gotErr := value.Validate()
		if wantValid && gotErr != nil {
			t.Fatalf(
				"%T(%d).Validate() error = %v, want nil",
				value,
				raw,
				gotErr,
			)
		}
		if !wantValid && !errors.Is(gotErr, core.ErrObjectStoreContract) {
			t.Fatalf(
				"%T(%d).Validate() error = %v, want %v",
				value,
				raw,
				gotErr,
				core.ErrObjectStoreContract,
			)
		}
		if gotValid := value.IsValid(); gotValid != wantValid {
			t.Fatalf(
				"%T(%d).IsValid() = %t, want %t",
				value,
				raw,
				gotValid,
				wantValid,
			)
		}
		if gotText := value.String(); gotText != wantText {
			t.Fatalf(
				"%T(%d).String() = %q, want %q",
				value,
				raw,
				gotText,
				wantText,
			)
		}
		value.OffWireEnum()
	}
}

func TestVendorSpecExhaustiveDomain(t *testing.T) {
	t.Parallel()

	wantValid := map[objectstore.Provider]objectstore.VendorSpec{
		objectstore.ProviderAmazonS3: {
			Provider:          objectstore.ProviderAmazonS3,
			API:               objectstore.VendorAPIAmazonS3Object,
			Directions:        objectstore.DirectionCapabilityUploadDownload,
			UploadMethod:      exchange.MethodPut,
			DownloadMethod:    exchange.MethodGet,
			UploadEncoding:    objectstore.UploadEncodingRawObject,
			ProviderIntegrity: objectstore.ProviderIntegrityCRC32C,
			WritePreference:   objectstore.WritePreferenceCreateOnly,
			UploadMaximum: mustByteLength(t,
				objectstore.AmazonS3PutObjectMaximumBytes,
			),
			DownloadMaximum: mustByteLength(t,
				objectstore.AmazonS3ObjectMaximumBytes,
			),
		},
		objectstore.ProviderGoogleCloudStorage: {
			Provider:          objectstore.ProviderGoogleCloudStorage,
			API:               objectstore.VendorAPIGoogleCloudStorageXML,
			Directions:        objectstore.DirectionCapabilityUploadDownload,
			UploadMethod:      exchange.MethodPut,
			DownloadMethod:    exchange.MethodGet,
			UploadEncoding:    objectstore.UploadEncodingRawObject,
			ProviderIntegrity: objectstore.ProviderIntegrityCRC32C,
			WritePreference:   objectstore.WritePreferenceCreateOnly,
			UploadMaximum: mustByteLength(t,
				objectstore.GoogleCloudStorageObjectMaximumBytes,
			),
			DownloadMaximum: mustByteLength(t,
				objectstore.GoogleCloudStorageObjectMaximumBytes,
			),
		},
		objectstore.ProviderCloudflareImages: {
			Provider:          objectstore.ProviderCloudflareImages,
			API:               objectstore.VendorAPICloudflareImagesDirect,
			Directions:        objectstore.DirectionCapabilityUploadOnly,
			UploadMethod:      exchange.MethodPost,
			DownloadMethod:    exchange.MethodUnknown,
			UploadEncoding:    objectstore.UploadEncodingMultipartFile,
			ProviderIntegrity: objectstore.ProviderIntegrityLocalOnly,
			WritePreference:   objectstore.WritePreferenceOneTimeCapability,
			UploadMaximum: mustByteLength(t,
				objectstore.CloudflareImagesUploadMaximumBytes,
			),
			DownloadMaximum: mustByteLength(t, 0),
		},
	}

	for raw := range 256 {
		provider := objectstore.Provider(raw)
		got, gotErr := objectstore.Spec(provider)
		want, wantValidProvider := wantValid[provider]
		if !wantValidProvider {
			if !errors.Is(gotErr, core.ErrObjectStoreContract) ||
				got != (objectstore.VendorSpec{}) {
				t.Fatalf(
					"Spec(Provider(%d)) = (%+v, %v), want (zero, %v)",
					raw,
					got,
					gotErr,
					core.ErrObjectStoreContract,
				)
			}
			continue
		}
		if gotErr != nil || got != want {
			t.Fatalf(
				"Spec(Provider(%d)) = (%+v, %v), want (%+v, nil)",
				raw,
				got,
				gotErr,
				want,
			)
		}
		if gotValidateErr := got.Validate(); gotValidateErr != nil {
			t.Fatalf(
				"Spec(Provider(%d)).Validate() error = %v, want nil",
				raw,
				gotValidateErr,
			)
		}
	}
}

func TestUploadProviderLayerTriad(t *testing.T) {
	t.Parallel()

	payload := bytes.Repeat([]byte("primitive-objectstore-"), 4096)
	cases := []struct {
		name     string
		provider objectstore.Provider
	}{
		{name: "Amazon S3 accepts exact raw create-only bytes", provider: objectstore.ProviderAmazonS3},
		{name: "Google Cloud Storage accepts exact raw create-only bytes", provider: objectstore.ProviderGoogleCloudStorage},
		{name: "Cloudflare Images accepts one exact multipart file", provider: objectstore.ProviderCloudflareImages},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			observed := make(chan providerObservation, 1)
			handler := http.HandlerFunc(func(
				writer http.ResponseWriter,
				request *http.Request,
			) {
				observation := observeUpload(request)
				observed <- observation
				setProviderVersion(writer.Header(), tc.provider)
				writer.WriteHeader(http.StatusOK)
			})
			targetURL, httpClient := providerServer(
				t,
				tc.provider,
				objectstore.DirectionUpload,
				handler,
			)
			client := newObjectstoreClient(t, httpClient)
			request := uploadRequest(t, tc.provider, targetURL, payload)

			got, gotErr := uploadOperationFor(tc.provider)(
				context.Background(),
				client,
				request,
			)
			if gotErr != nil {
				t.Fatalf("Upload() error = %v, want nil", gotErr)
			}
			if gotValidateErr := got.Validate(); gotValidateErr != nil {
				t.Fatalf(
					"Upload() result Validate() error = %v, want nil",
					gotValidateErr,
				)
			}
			if got.Provider() != tc.provider ||
				got.Direction() != objectstore.DirectionUpload ||
				got.Commitment() != objectstore.CommitmentConfirmed ||
				got.Bytes() != mustByteLength(t, uint64(len(payload))) {
				t.Fatalf(
					"Upload() result = provider %v direction %v commitment %v bytes %d, want provider %v upload confirmed bytes %d",
					got.Provider(),
					got.Direction(),
					got.Commitment(),
					got.Bytes().Uint64(),
					tc.provider,
					len(payload),
				)
			}
			verifyTransferEvidence(t, got, request.Integrity)
			verifyProviderVersion(t, got, tc.provider)
			gotObservation := <-observed
			if gotObservation.body == nil ||
				!bytes.Equal(gotObservation.body, payload) {
				t.Fatalf(
					"provider body bytes = %d, want exact non-vacuous %d-byte source",
					len(gotObservation.body),
					len(payload),
				)
			}
			verifyUploadObservation(t, tc.provider, gotObservation)
		})
	}
}

// TestUploadEntryPointsComposeBackToBack proves replication remains ordinary
// caller composition: the caller reopens one source and invokes each exact
// provider operation in sequence. Objectstore owns no fan-out mechanism.
func TestUploadEntryPointsComposeBackToBack(t *testing.T) {
	t.Parallel()

	payload := bytes.Repeat([]byte("explicit-provider-call-"), 1024)
	cases := []struct {
		operation uploadOperation
		name      string
		provider  objectstore.Provider
	}{
		{
			name:      "Amazon S3 direct operation",
			provider:  objectstore.ProviderAmazonS3,
			operation: objectstore.UploadS3,
		},
		{
			name:      "Google Cloud Storage direct operation",
			provider:  objectstore.ProviderGoogleCloudStorage,
			operation: objectstore.UploadGCS,
		},
		{
			name:      "Cloudflare Images direct operation",
			provider:  objectstore.ProviderCloudflareImages,
			operation: objectstore.UploadCloudflareImages,
		},
	}

	for _, tc := range cases {
		observed := make(chan providerObservation, 1)
		handler := http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			observed <- observeUpload(request)
			setProviderVersion(writer.Header(), tc.provider)
			writer.WriteHeader(http.StatusOK)
		})
		targetURL, httpClient := providerServer(
			t, tc.provider, objectstore.DirectionUpload, handler,
		)
		request := uploadRequest(t, tc.provider, targetURL, payload)
		got, gotErr := tc.operation(
			context.Background(),
			newObjectstoreClient(t, httpClient),
			request,
		)
		if gotErr != nil ||
			got.Commitment() != objectstore.CommitmentConfirmed {
			t.Fatalf(
				"%s = (commitment %v, error %v), want confirmed and nil",
				tc.name,
				got.Commitment(),
				gotErr,
			)
		}
		gotObservation := <-observed
		if !bytes.Equal(gotObservation.body, payload) {
			t.Fatalf(
				"%s body bytes = %d, want exact reopened %d-byte source",
				tc.name,
				len(gotObservation.body),
				len(payload),
			)
		}
	}
}

func TestDownloadProviderLayerTriad(t *testing.T) {
	t.Parallel()

	payload := bytes.Repeat([]byte("download-object-"), 2048)
	cases := []struct {
		name     string
		provider objectstore.Provider
	}{
		{name: "Amazon S3 exact object and checksum-mode request", provider: objectstore.ProviderAmazonS3},
		{name: "Google Cloud Storage exact XML API object", provider: objectstore.ProviderGoogleCloudStorage},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			observed := make(chan providerObservation, 1)
			handler := http.HandlerFunc(func(
				writer http.ResponseWriter,
				request *http.Request,
			) {
				observed <- providerObservation{
					method: request.Method,
					checksumMode: request.Header.Get(
						"X-Amz-Checksum-Mode",
					),
				}
				writer.Header().Set(
					core.HTTPHeaderContentType().String(),
					core.HTTPMediaTypeOctetStream().String(),
				)
				setProviderVersion(writer.Header(), tc.provider)
				writer.WriteHeader(http.StatusOK)
				_, _ = writer.Write(payload)
			})
			targetURL, httpClient := providerServer(
				t,
				tc.provider,
				objectstore.DirectionDownload,
				handler,
			)
			client := newObjectstoreClient(t, httpClient)
			var destination bytes.Buffer
			request := downloadRequest(
				t,
				tc.provider,
				targetURL,
				&destination,
				payload,
			)

			got, gotErr := downloadOperationFor(tc.provider)(
				context.Background(),
				client,
				request,
			)
			if gotErr != nil {
				t.Fatalf("Download() error = %v, want nil", gotErr)
			}
			if gotValidateErr := got.Validate(); gotValidateErr != nil {
				t.Fatalf(
					"Download() result Validate() error = %v, want nil",
					gotValidateErr,
				)
			}
			if !bytes.Equal(destination.Bytes(), payload) {
				t.Fatalf(
					"Download() destination bytes = %d, want exact %d-byte object",
					destination.Len(),
					len(payload),
				)
			}
			verifyTransferEvidence(t, got, request.Integrity)
			verifyProviderVersion(t, got, tc.provider)
			gotObservation := <-observed
			if gotObservation.method != http.MethodGet {
				t.Fatalf(
					"provider method = %q, want %q",
					gotObservation.method,
					http.MethodGet,
				)
			}
			if tc.provider == objectstore.ProviderAmazonS3 &&
				gotObservation.checksumMode != "ENABLED" {
				t.Fatalf(
					"S3 checksum mode = %q, want ENABLED",
					gotObservation.checksumMode,
				)
			}
		})
	}
}

func TestZeroLengthStreamLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("empty upload proves the neutral exact stream", func(t *testing.T) {
		t.Parallel()

		handler := http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			body, readErr := io.ReadAll(request.Body)
			if readErr != nil || len(body) != 0 {
				t.Errorf(
					"provider request body = (%d bytes, %v), want (0 bytes, nil)",
					len(body),
					readErr,
				)
			}
			writer.Header().Set("X-Goog-Generation", "1")
			writer.WriteHeader(http.StatusOK)
		})
		targetURL, httpClient := providerServer(
			t,
			objectstore.ProviderGoogleCloudStorage,
			objectstore.DirectionUpload,
			handler,
		)
		request := uploadRequest(
			t,
			objectstore.ProviderGoogleCloudStorage,
			targetURL,
			nil,
		)

		got, gotErr := objectstore.UploadGCS(
			context.Background(),
			newObjectstoreClient(t, httpClient),
			request,
		)
		if gotErr != nil ||
			got.Commitment() != objectstore.CommitmentConfirmed {
			t.Fatalf(
				"Upload() = (commitment %v, error %v), want (confirmed, nil)",
				got.Commitment(),
				gotErr,
			)
		}
		verifyTransferEvidence(t, got, request.Integrity)
	})

	t.Run("declared empty upload rejects one source byte before attempt", func(t *testing.T) {
		t.Parallel()

		request := uploadRequest(
			t,
			objectstore.ProviderGoogleCloudStorage,
			signedProviderURL(
				"https://storage.googleapis.com/bucket/object",
				objectstore.ProviderGoogleCloudStorage,
				objectstore.DirectionUpload,
			),
			nil,
		)
		request.Source = bytes.NewReader([]byte{0x01})
		nativeFailure := errors.New("transport must remain untouched")
		got, gotErr := objectstore.UploadGCS(
			context.Background(),
			newObjectstoreClient(
				t,
				&http.Client{Transport: failureTransport{cause: nativeFailure}},
			),
			request,
		)
		if !errors.Is(gotErr, core.ErrObjectStoreSource) ||
			errors.Is(gotErr, nativeFailure) {
			t.Fatalf(
				"Upload() error = %v, want %v without transport failure",
				gotErr,
				core.ErrObjectStoreSource,
			)
		}
		if got.Commitment() != objectstore.CommitmentNotAttempted {
			t.Fatalf(
				"Upload() commitment = %v, want %v",
				got.Commitment(),
				objectstore.CommitmentNotAttempted,
			)
		}
	})

	t.Run("empty download proves the neutral exact stream", func(t *testing.T) {
		t.Parallel()

		handler := http.HandlerFunc(func(
			writer http.ResponseWriter,
			_ *http.Request,
		) {
			writer.Header().Set(
				core.HTTPHeaderContentType().String(),
				core.HTTPMediaTypeOctetStream().String(),
			)
			writer.Header().Set("X-Goog-Generation", "1")
			writer.WriteHeader(http.StatusOK)
		})
		targetURL, httpClient := providerServer(
			t,
			objectstore.ProviderGoogleCloudStorage,
			objectstore.DirectionDownload,
			handler,
		)
		var destination bytes.Buffer
		request := downloadRequest(
			t,
			objectstore.ProviderGoogleCloudStorage,
			targetURL,
			&destination,
			nil,
		)

		got, gotErr := objectstore.DownloadGCS(
			context.Background(),
			newObjectstoreClient(t, httpClient),
			request,
		)
		if gotErr != nil ||
			got.Commitment() != objectstore.CommitmentConfirmed ||
			destination.Len() != 0 {
			t.Fatalf(
				"Download() = (commitment %v, destination bytes %d, error %v), want (confirmed, 0, nil)",
				got.Commitment(),
				destination.Len(),
				gotErr,
			)
		}
		verifyTransferEvidence(t, got, request.Integrity)
	})
}

func TestSignedHeadersHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	t.Run("one caller-owned signed field is accepted", func(t *testing.T) {
		t.Parallel()

		header := signedHeader(
			t,
			"X-Amz-Server-Side-Encryption",
			"AES256",
		)
		got, gotErr := objectstore.NewSignedHeaders([]objectstore.SignedHeader{
			header,
		})
		if gotErr != nil {
			t.Fatalf("NewSignedHeaders() error = %v, want nil", gotErr)
		}
		if gotValidateErr := got.Validate(); gotValidateErr != nil {
			t.Fatalf(
				"SignedHeaders.Validate() error = %v, want nil",
				gotValidateErr,
			)
		}
	})

	t.Run("zero signed field is rejected", func(t *testing.T) {
		t.Parallel()

		_, gotErr := objectstore.NewSignedHeaders(
			[]objectstore.SignedHeader{{}},
		)
		if !errors.Is(gotErr, core.ErrObjectStoreContract) {
			t.Fatalf(
				"NewSignedHeaders() error = %v, want %v",
				gotErr,
				core.ErrObjectStoreContract,
			)
		}
	})

	t.Run("header injection is rejected", func(t *testing.T) {
		t.Parallel()

		name := parsedHeaderName(t, "X-Signed-Field")
		_, gotErr := objectstore.NewSignedHeader(name, "value\r\nInjected: yes")
		if !errors.Is(gotErr, core.ErrObjectStoreContract) {
			t.Fatalf(
				"NewSignedHeader() error = %v, want %v",
				gotErr,
				core.ErrObjectStoreContract,
			)
		}
	})

	for _, name := range objectstoreOwnedHeaderNames(t) {
		t.Run("package-owned field "+name.String()+" is rejected", func(t *testing.T) {
			t.Parallel()

			_, gotErr := objectstore.NewSignedHeader(name, "value")
			if !errors.Is(gotErr, core.ErrObjectStoreContract) {
				t.Fatalf(
					"NewSignedHeader(%q) error = %v, want %v",
					name.String(),
					gotErr,
					core.ErrObjectStoreContract,
				)
			}
		})
	}

	t.Run("duplicate signed field is rejected", func(t *testing.T) {
		t.Parallel()

		header := signedHeader(t, "X-Signed-Field", "value")
		_, gotErr := objectstore.NewSignedHeaders(
			[]objectstore.SignedHeader{header, header},
		)
		if !errors.Is(gotErr, core.ErrObjectStoreContract) {
			t.Fatalf(
				"NewSignedHeaders() error = %v, want %v",
				gotErr,
				core.ErrObjectStoreContract,
			)
		}
	})

	t.Run("exact signed-field count ceiling is accepted", func(t *testing.T) {
		t.Parallel()

		headers := signedHeaderCount(t, objectstore.SignedHeaderMaximumCount)
		got, gotErr := objectstore.NewSignedHeaders(headers)
		if gotErr != nil {
			t.Fatalf("NewSignedHeaders() error = %v, want nil", gotErr)
		}
		if gotValidateErr := got.Validate(); gotValidateErr != nil {
			t.Fatalf(
				"SignedHeaders.Validate() error = %v, want nil",
				gotValidateErr,
			)
		}
	})

	t.Run("one above signed-field count ceiling is rejected", func(t *testing.T) {
		t.Parallel()

		headers := signedHeaderCount(
			t,
			objectstore.SignedHeaderMaximumCount+1,
		)
		_, gotErr := objectstore.NewSignedHeaders(headers)
		if !errors.Is(gotErr, core.ErrObjectStoreContract) {
			t.Fatalf(
				"NewSignedHeaders() error = %v, want %v",
				gotErr,
				core.ErrObjectStoreContract,
			)
		}
	})

	t.Run("exact aggregate wire-byte ceiling is accepted", func(t *testing.T) {
		t.Parallel()

		headers := signedHeaderWireBoundary(t, 0)
		got, gotErr := objectstore.NewSignedHeaders(headers)
		if gotErr != nil {
			t.Fatalf("NewSignedHeaders() error = %v, want nil", gotErr)
		}
		if gotValidateErr := got.Validate(); gotValidateErr != nil {
			t.Fatalf(
				"SignedHeaders.Validate() error = %v, want nil",
				gotValidateErr,
			)
		}
	})

	t.Run("one above aggregate wire-byte ceiling is rejected", func(t *testing.T) {
		t.Parallel()

		headers := signedHeaderWireBoundary(t, 1)
		_, gotErr := objectstore.NewSignedHeaders(headers)
		if !errors.Is(gotErr, core.ErrObjectStoreContract) {
			t.Fatalf(
				"NewSignedHeaders() error = %v, want %v",
				gotErr,
				core.ErrObjectStoreContract,
			)
		}
	})
}

func TestProviderVersionProjectionLayerTriad(t *testing.T) {
	t.Parallel()

	payload := []byte("provider-version-boundary")
	cases := []struct {
		wantErr        error
		name           string
		headerName     string
		wantValue      string
		headerValues   []string
		provider       objectstore.Provider
		wantCommitment objectstore.Commitment
		wantPresent    bool
	}{
		{
			name:           "S3 absent version remains neutral",
			provider:       objectstore.ProviderAmazonS3,
			wantCommitment: objectstore.CommitmentConfirmed,
		},
		{
			name:           "GCS absent generation leaves acceptance indeterminate",
			provider:       objectstore.ProviderGoogleCloudStorage,
			wantErr:        core.ErrObjectStoreContract,
			wantCommitment: objectstore.CommitmentIndeterminate,
		},
		{
			name:           "Cloudflare has no provider version surface",
			provider:       objectstore.ProviderCloudflareImages,
			wantCommitment: objectstore.CommitmentConfirmed,
		},
		{
			name:           "S3 unversioned null identity is preserved",
			headerName:     "X-Amz-Version-Id",
			headerValues:   []string{"null"},
			provider:       objectstore.ProviderAmazonS3,
			wantValue:      "null",
			wantCommitment: objectstore.CommitmentConfirmed,
			wantPresent:    true,
		},
		{
			name:           "S3 one-byte identity is preserved",
			headerName:     "X-Amz-Version-Id",
			headerValues:   []string{"v"},
			provider:       objectstore.ProviderAmazonS3,
			wantValue:      "v",
			wantCommitment: objectstore.CommitmentConfirmed,
			wantPresent:    true,
		},
		{
			name:           "S3 one below byte ceiling is preserved",
			headerName:     "X-Amz-Version-Id",
			headerValues:   []string{strings.Repeat("v", objectstore.AmazonS3VersionIDMaximumBytes-1)},
			provider:       objectstore.ProviderAmazonS3,
			wantValue:      strings.Repeat("v", objectstore.AmazonS3VersionIDMaximumBytes-1),
			wantCommitment: objectstore.CommitmentConfirmed,
			wantPresent:    true,
		},
		{
			name:           "S3 exact byte ceiling is preserved",
			headerName:     "X-Amz-Version-Id",
			headerValues:   []string{strings.Repeat("v", objectstore.AmazonS3VersionIDMaximumBytes)},
			provider:       objectstore.ProviderAmazonS3,
			wantValue:      strings.Repeat("v", objectstore.AmazonS3VersionIDMaximumBytes),
			wantCommitment: objectstore.CommitmentConfirmed,
			wantPresent:    true,
		},
		{
			name:           "S3 UTF-8 identity at byte ceiling is preserved",
			headerName:     "X-Amz-Version-Id",
			headerValues:   []string{strings.Repeat("é", objectstore.AmazonS3VersionIDMaximumBytes/2)},
			provider:       objectstore.ProviderAmazonS3,
			wantValue:      strings.Repeat("é", objectstore.AmazonS3VersionIDMaximumBytes/2),
			wantCommitment: objectstore.CommitmentConfirmed,
			wantPresent:    true,
		},
		{
			name:           "GCS minimum positive generation is preserved",
			headerName:     "X-Goog-Generation",
			headerValues:   []string{"1"},
			provider:       objectstore.ProviderGoogleCloudStorage,
			wantValue:      "1",
			wantCommitment: objectstore.CommitmentConfirmed,
			wantPresent:    true,
		},
		{
			name:           "GCS ordinary generation is preserved",
			headerName:     "X-Goog-Generation",
			headerValues:   []string{"42"},
			provider:       objectstore.ProviderGoogleCloudStorage,
			wantValue:      "42",
			wantCommitment: objectstore.CommitmentConfirmed,
			wantPresent:    true,
		},
		{
			name:           "GCS SDK int64 ceiling generation is preserved",
			headerName:     "X-Goog-Generation",
			headerValues:   []string{"9223372036854775807"},
			provider:       objectstore.ProviderGoogleCloudStorage,
			wantValue:      "9223372036854775807",
			wantCommitment: objectstore.CommitmentConfirmed,
			wantPresent:    true,
		},
		{
			name:           "S3 present empty identity is rejected",
			headerName:     "X-Amz-Version-Id",
			headerValues:   []string{""},
			provider:       objectstore.ProviderAmazonS3,
			wantErr:        core.ErrObjectStoreContract,
			wantCommitment: objectstore.CommitmentIndeterminate,
		},
		{
			name:           "S3 one above byte ceiling is rejected",
			headerName:     "X-Amz-Version-Id",
			headerValues:   []string{strings.Repeat("v", objectstore.AmazonS3VersionIDMaximumBytes+1)},
			provider:       objectstore.ProviderAmazonS3,
			wantErr:        core.ErrObjectStoreContract,
			wantCommitment: objectstore.CommitmentIndeterminate,
		},
		{
			name:           "S3 malformed UTF-8 identity is rejected",
			headerName:     "X-Amz-Version-Id",
			headerValues:   []string{string([]byte{0xff})},
			provider:       objectstore.ProviderAmazonS3,
			wantErr:        core.ErrObjectStoreContract,
			wantCommitment: objectstore.CommitmentIndeterminate,
		},
		{
			name:           "S3 repeated identity is rejected",
			headerName:     "X-Amz-Version-Id",
			headerValues:   []string{"first", "second"},
			provider:       objectstore.ProviderAmazonS3,
			wantErr:        core.ErrObjectStoreContract,
			wantCommitment: objectstore.CommitmentIndeterminate,
		},
		{
			name:           "GCS present empty generation is rejected",
			headerName:     "X-Goog-Generation",
			headerValues:   []string{""},
			provider:       objectstore.ProviderGoogleCloudStorage,
			wantErr:        core.ErrObjectStoreContract,
			wantCommitment: objectstore.CommitmentIndeterminate,
		},
		{
			name:           "GCS zero generation is rejected",
			headerName:     "X-Goog-Generation",
			headerValues:   []string{"0"},
			provider:       objectstore.ProviderGoogleCloudStorage,
			wantErr:        core.ErrObjectStoreContract,
			wantCommitment: objectstore.CommitmentIndeterminate,
		},
		{
			name:           "GCS signed generation is rejected",
			headerName:     "X-Goog-Generation",
			headerValues:   []string{"-1"},
			provider:       objectstore.ProviderGoogleCloudStorage,
			wantErr:        core.ErrObjectStoreContract,
			wantCommitment: objectstore.CommitmentIndeterminate,
		},
		{
			name:           "GCS explicit plus generation is rejected",
			headerName:     "X-Goog-Generation",
			headerValues:   []string{"+1"},
			provider:       objectstore.ProviderGoogleCloudStorage,
			wantErr:        core.ErrObjectStoreContract,
			wantCommitment: objectstore.CommitmentIndeterminate,
		},
		{
			name:           "GCS leading-zero generation is rejected",
			headerName:     "X-Goog-Generation",
			headerValues:   []string{"01"},
			provider:       objectstore.ProviderGoogleCloudStorage,
			wantErr:        core.ErrObjectStoreContract,
			wantCommitment: objectstore.CommitmentIndeterminate,
		},
		{
			name:           "GCS surrounding whitespace is rejected",
			headerName:     "X-Goog-Generation",
			headerValues:   []string{" 1 "},
			provider:       objectstore.ProviderGoogleCloudStorage,
			wantErr:        core.ErrObjectStoreContract,
			wantCommitment: objectstore.CommitmentIndeterminate,
		},
		{
			name:           "GCS one above SDK int64 ceiling is rejected",
			headerName:     "X-Goog-Generation",
			headerValues:   []string{"9223372036854775808"},
			provider:       objectstore.ProviderGoogleCloudStorage,
			wantErr:        core.ErrObjectStoreContract,
			wantCommitment: objectstore.CommitmentIndeterminate,
		},
		{
			name:           "GCS non-decimal generation is rejected",
			headerName:     "X-Goog-Generation",
			headerValues:   []string{"generation"},
			provider:       objectstore.ProviderGoogleCloudStorage,
			wantErr:        core.ErrObjectStoreContract,
			wantCommitment: objectstore.CommitmentIndeterminate,
		},
		{
			name:           "GCS repeated generation is rejected",
			headerName:     "X-Goog-Generation",
			headerValues:   []string{"41", "42"},
			provider:       objectstore.ProviderGoogleCloudStorage,
			wantErr:        core.ErrObjectStoreContract,
			wantCommitment: objectstore.CommitmentIndeterminate,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			transport := providerVersionTransport{
				headerName: tc.headerName, headerValues: tc.headerValues,
			}
			client := newObjectstoreClient(
				t,
				&http.Client{Transport: transport},
			)
			request := uploadRequest(
				t,
				tc.provider,
				signedProviderURL(
					providerEndpoint(tc.provider),
					tc.provider,
					objectstore.DirectionUpload,
				),
				payload,
			)

			got, gotErr := uploadOperationFor(tc.provider)(
				context.Background(),
				client,
				request,
			)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf(
					"Upload() error = %v, want %v",
					gotErr,
					tc.wantErr,
				)
			}
			if got.Commitment() != tc.wantCommitment {
				t.Fatalf(
					"Upload() commitment = %v, want %v",
					got.Commitment(),
					tc.wantCommitment,
				)
			}
			version, gotPresent := got.Version()
			if gotPresent != tc.wantPresent {
				t.Fatalf(
					"Transfer.Version() presence = %t, want %t",
					gotPresent,
					tc.wantPresent,
				)
			}
			if !tc.wantPresent {
				if !version.IsZero() {
					t.Fatalf(
						"Transfer.Version() absent value zero = %t, want true",
						version.IsZero(),
					)
				}
				return
			}
			if gotValidateErr := version.Validate(); gotValidateErr != nil {
				t.Fatalf(
					"Transfer.Version().Validate() error = %v, want nil",
					gotValidateErr,
				)
			}
			if version.Provider() != tc.provider ||
				version.String() != tc.wantValue {
				t.Fatalf(
					"Transfer.Version() = provider %v value %q, want provider %v value %q",
					version.Provider(),
					version.String(),
					tc.provider,
					tc.wantValue,
				)
			}
		})
	}
}

func TestUploadRequestHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	payload := []byte("boundary-payload")
	baseURL := "https://example.com/object?X-Amz-Signature=sig&X-Amz-SignedHeaders=" +
		url.QueryEscape(testS3UploadSignedHeaders)
	base := uploadRequest(
		t,
		objectstore.ProviderAmazonS3,
		baseURL,
		payload,
	)
	nilSource := base
	nilSource.Source = nil
	unsetURL := base
	unsetURL.Target.URL = objectstore.SignedURL{}
	unsetExpiry := base
	unsetExpiry.Target.ExpiresAt = temporal.Instant{}
	unsetSHA := base
	unsetSHA.Integrity.SHA256 = core.SHA256Digest{}
	unsetCRC := base
	unsetCRC.Integrity.CRC32C = core.CRC32C{}
	unsetMediaType := base
	unsetMediaType.ContentType = core.HTTPMediaType{}
	zeroOperationTimeout := base
	zeroOperationTimeout.Policy.OperationTimeout = temporal.Duration{}
	zeroAttemptTimeout := base
	zeroAttemptTimeout.Policy.AttemptTimeout = temporal.Duration{}
	attemptExceedsOperation := base
	attemptExceedsOperation.Policy.OperationTimeout = durationSeconds(t, 1)
	attemptExceedsOperation.Policy.AttemptTimeout = durationSeconds(t, 2)
	zeroErrorLimit := base
	zeroErrorLimit.Policy.ErrorBodyLimit = core.ByteCount{}
	cases := []struct {
		name    string
		wantErr error
		request objectstore.UploadRequest
	}{
		{name: "complete S3 request is accepted", request: base},
		{name: "nil source is rejected", request: nilSource, wantErr: core.ErrObjectStoreSource},
		{name: "unset signed URL is rejected", request: unsetURL, wantErr: core.ErrObjectStoreContract},
		{name: "unset expiry is rejected", request: unsetExpiry, wantErr: core.ErrObjectStoreContract},
		{name: "unset SHA-256 is rejected", request: unsetSHA, wantErr: core.ErrObjectStoreContract},
		{name: "unset CRC32C is rejected", request: unsetCRC, wantErr: core.ErrObjectStoreContract},
		{name: "unset media type is rejected", request: unsetMediaType, wantErr: core.ErrObjectStoreContract},
		{name: "zero operation timeout is rejected", request: zeroOperationTimeout, wantErr: core.ErrObjectStoreContract},
		{name: "zero attempt timeout is rejected", request: zeroAttemptTimeout, wantErr: core.ErrObjectStoreContract},
		{name: "attempt timeout beyond operation is rejected", request: attemptExceedsOperation, wantErr: core.ErrObjectStoreContract},
		{name: "zero error body limit is rejected", request: zeroErrorLimit, wantErr: core.ErrObjectStoreContract},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.request.Validate()
			if tc.wantErr == nil {
				if gotErr != nil {
					t.Fatalf(
						"UploadRequest.Validate() error = %v, want nil",
						gotErr,
					)
				}
				return
			}
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf(
					"UploadRequest.Validate() error = %v, want %v",
					gotErr,
					tc.wantErr,
				)
			}
		})
	}
}

func TestDownloadRequestHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	payload := []byte("download-boundary-payload")
	baseURL := signedProviderURL(
		"https://storage.googleapis.com/bucket/object",
		objectstore.ProviderGoogleCloudStorage,
		objectstore.DirectionDownload,
	)
	base := downloadRequest(
		t,
		objectstore.ProviderGoogleCloudStorage,
		baseURL,
		io.Discard,
		payload,
	)
	nilDestination := base
	nilDestination.Destination = nil
	unsetURL := base
	unsetURL.Target.URL = objectstore.SignedURL{}
	unsetExpiry := base
	unsetExpiry.Target.ExpiresAt = temporal.Instant{}
	unsetSHA := base
	unsetSHA.Integrity.SHA256 = core.SHA256Digest{}
	unsetCRC := base
	unsetCRC.Integrity.CRC32C = core.CRC32C{}
	unsetMediaType := base
	unsetMediaType.ContentType = core.HTTPMediaType{}
	unsetPolicy := base
	unsetPolicy.Policy = objectstore.Policy{}
	cases := []struct {
		name    string
		wantErr error
		request objectstore.DownloadRequest
	}{
		{name: "complete GCS request is accepted", request: base},
		{name: "nil destination is rejected", request: nilDestination, wantErr: core.ErrObjectStoreDestination},
		{name: "unset signed URL is rejected", request: unsetURL, wantErr: core.ErrObjectStoreContract},
		{name: "unset expiry is rejected", request: unsetExpiry, wantErr: core.ErrObjectStoreContract},
		{name: "unset SHA-256 is rejected", request: unsetSHA, wantErr: core.ErrObjectStoreContract},
		{name: "unset CRC32C is rejected", request: unsetCRC, wantErr: core.ErrObjectStoreContract},
		{name: "unset media type is rejected", request: unsetMediaType, wantErr: core.ErrObjectStoreContract},
		{name: "unset policy is rejected", request: unsetPolicy, wantErr: core.ErrObjectStoreContract},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.request.Validate()
			if tc.wantErr == nil {
				if gotErr != nil {
					t.Fatalf(
						"DownloadRequest.Validate() error = %v, want nil",
						gotErr,
					)
				}
				return
			}
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf(
					"DownloadRequest.Validate() error = %v, want %v",
					gotErr,
					tc.wantErr,
				)
			}
		})
	}
}

func TestUploadFailureCommitmentAndExactExtent(t *testing.T) {
	t.Parallel()

	payload := bytes.Repeat([]byte("extent-"), 1024)
	cases := []struct {
		wantErr        error
		name           string
		source         []byte
		status         int
		wantCommitment objectstore.Commitment
	}{
		{name: "one byte short source is rejected before commitment", source: payload[:len(payload)-1], status: http.StatusOK, wantErr: core.ErrObjectStoreSource, wantCommitment: objectstore.CommitmentRejected},
		{name: "one byte long source withholds the final source chunk", source: append(bytes.Clone(payload), 0x7f), status: http.StatusOK, wantErr: core.ErrObjectStoreSource, wantCommitment: objectstore.CommitmentRejected},
		{name: "provider create-only conflict is rejected", source: payload, status: http.StatusPreconditionFailed, wantErr: core.ErrObjectStoreConflict, wantCommitment: objectstore.CommitmentRejected},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			handler := http.HandlerFunc(func(
				writer http.ResponseWriter,
				request *http.Request,
			) {
				_, _ = io.Copy(io.Discard, request.Body)
				writer.WriteHeader(tc.status)
			})
			targetURL, httpClient := providerServer(
				t,
				objectstore.ProviderAmazonS3,
				objectstore.DirectionUpload,
				handler,
			)
			client := newObjectstoreClient(t, httpClient)
			request := uploadRequest(
				t,
				objectstore.ProviderAmazonS3,
				targetURL,
				payload,
			)
			request.Source = bytes.NewReader(tc.source)

			got, gotErr := objectstore.UploadS3(
				context.Background(),
				client,
				request,
			)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf(
					"Upload() error = %v, want %v",
					gotErr,
					tc.wantErr,
				)
			}
			if got.Commitment() != tc.wantCommitment {
				t.Fatalf(
					"Upload() commitment = %v, want %v",
					got.Commitment(),
					tc.wantCommitment,
				)
			}
		})
	}
}

func TestDownloadFailureLayerTriad(t *testing.T) {
	t.Parallel()

	payload := bytes.Repeat([]byte("download-boundary-"), 256)
	destinationFailure := errors.New("destination refused bytes")
	cases := []struct {
		wantErr         error
		destination     func() io.Writer
		name            string
		contentRange    string
		responseBody    []byte
		generations     []string
		status          int
		wantDestination int
	}{
		{
			name:         "absent object leaves destination untouched",
			responseBody: []byte("provider error"),
			status:       http.StatusNotFound,
			destination:  func() io.Writer { return &bytes.Buffer{} },
			wantErr:      core.ErrObjectStoreAbsent,
		},
		{
			name:         "absent status wins over malformed repeated generation",
			responseBody: []byte("provider error"),
			status:       http.StatusNotFound,
			generations:  []string{"1", "2"},
			destination:  func() io.Writer { return &bytes.Buffer{} },
			wantErr:      core.ErrObjectStoreAbsent,
		},
		{
			name:            "one byte short response is an integrity failure",
			responseBody:    payload[:len(payload)-1],
			status:          http.StatusOK,
			destination:     func() io.Writer { return &bytes.Buffer{} },
			wantErr:         core.ErrObjectStoreIntegrity,
			wantDestination: len(payload) - 1,
		},
		{
			name:            "one byte long response is rejected before destination overflow",
			responseBody:    append(bytes.Clone(payload), 0x7f),
			status:          http.StatusOK,
			destination:     func() io.Writer { return &bytes.Buffer{} },
			wantErr:         core.ErrObjectStoreIntegrity,
			wantDestination: len(payload),
		},
		{
			name:            "successful body with repeated generation is rejected",
			responseBody:    payload,
			status:          http.StatusOK,
			generations:     []string{"1", "2"},
			destination:     func() io.Writer { return &bytes.Buffer{} },
			wantErr:         core.ErrObjectStoreContract,
			wantDestination: len(payload),
		},
		{
			name:            "partial-content metadata cannot masquerade as a whole object",
			responseBody:    payload,
			status:          http.StatusOK,
			contentRange:    "bytes 0-4095/8192",
			destination:     func() io.Writer { return &bytes.Buffer{} },
			wantErr:         core.ErrObjectStoreIntegrity,
			wantDestination: len(payload),
		},
		{
			name:         "destination error retains its native identity",
			responseBody: payload,
			status:       http.StatusOK,
			destination: func() io.Writer {
				return failingDestination{cause: destinationFailure}
			},
			wantErr: destinationFailure,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			handler := http.HandlerFunc(func(
				writer http.ResponseWriter,
				_ *http.Request,
			) {
				writer.Header().Set(
					core.HTTPHeaderContentType().String(),
					core.HTTPMediaTypeOctetStream().String(),
				)
				if tc.contentRange != "" {
					writer.Header().Set("Content-Range", tc.contentRange)
				}
				for _, generation := range tc.generations {
					writer.Header().Add("X-Goog-Generation", generation)
				}
				writer.WriteHeader(tc.status)
				_, _ = writer.Write(tc.responseBody)
			})
			targetURL, httpClient := providerServer(
				t,
				objectstore.ProviderGoogleCloudStorage,
				objectstore.DirectionDownload,
				handler,
			)
			client := newObjectstoreClient(t, httpClient)
			destination := tc.destination()
			request := downloadRequest(
				t,
				objectstore.ProviderGoogleCloudStorage,
				targetURL,
				destination,
				payload,
			)

			got, gotErr := objectstore.DownloadGCS(
				context.Background(),
				client,
				request,
			)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf(
					"Download() error = %v, want %v",
					gotErr,
					tc.wantErr,
				)
			}
			if got.Commitment() != objectstore.CommitmentRejected {
				t.Fatalf(
					"Download() commitment = %v, want %v",
					got.Commitment(),
					objectstore.CommitmentRejected,
				)
			}
			buffer, ok := destination.(*bytes.Buffer)
			if ok && buffer.Len() != tc.wantDestination {
				t.Fatalf(
					"Download() destination bytes = %d, want %d",
					buffer.Len(),
					tc.wantDestination,
				)
			}
		})
	}
}

func TestSignedURLFormattingIsAlwaysRedacted(t *testing.T) {
	t.Parallel()

	const secret = "capability-secret"
	value := signedURL(
		t,
		"https://example.com/object?X-Amz-Signature="+secret+
			"&X-Amz-SignedHeaders="+url.QueryEscape(testS3UploadSignedHeaders),
	)
	for _, format := range []string{"%v", "%+v", "%s", "%q", "%#v"} {
		got := fmt.Sprintf(format, value)
		if got != core.RedactedValueText || strings.Contains(got, secret) {
			t.Fatalf(
				"fmt.Sprintf(%q, SignedURL) = %q, want %q without bearer",
				format,
				got,
				core.RedactedValueText,
			)
		}
	}
}

func TestUploadAttemptBoundaryTable(t *testing.T) {
	t.Parallel()

	payload := []byte("attempt-boundary")
	nativeFailure := errors.New("native transport refusal")
	targetURL := signedProviderURL(
		"https://s3.amazonaws.com/bucket/object",
		objectstore.ProviderAmazonS3,
		objectstore.DirectionUpload,
	)
	base := uploadRequest(
		t,
		objectstore.ProviderAmazonS3,
		targetURL,
		payload,
	)
	expired := base
	expired.Target.ExpiresAt = instantAt(
		t,
		time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC),
	)
	cancelledContext, cancel := context.WithCancel(context.Background())
	cancel()

	cases := []struct {
		ctx            context.Context
		transport      http.RoundTripper
		wantErr        error
		name           string
		request        objectstore.UploadRequest
		wantCommitment objectstore.Commitment
		wantSafeURL    bool
	}{
		{
			name:           "expired capability spends no attempt",
			ctx:            context.Background(),
			request:        expired,
			transport:      failureTransport{cause: nativeFailure},
			wantErr:        core.ErrObjectStoreExpired,
			wantCommitment: objectstore.CommitmentNotAttempted,
		},
		{
			name:           "pre-cancelled context spends no attempt",
			ctx:            cancelledContext,
			request:        base,
			transport:      failureTransport{cause: nativeFailure},
			wantErr:        context.Canceled,
			wantCommitment: objectstore.CommitmentNotAttempted,
		},
		{
			name:           "attempted transport loss is indeterminate and keeps native identity",
			ctx:            context.Background(),
			request:        base,
			transport:      failureTransport{cause: nativeFailure},
			wantErr:        nativeFailure,
			wantCommitment: objectstore.CommitmentIndeterminate,
			wantSafeURL:    true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client := newObjectstoreClient(
				t,
				&http.Client{Transport: tc.transport},
			)
			got, gotErr := objectstore.UploadS3(tc.ctx, client, tc.request)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf(
					"Upload() error = %v, want %v",
					gotErr,
					tc.wantErr,
				)
			}
			if got.Commitment() != tc.wantCommitment {
				t.Fatalf(
					"Upload() commitment = %v, want %v",
					got.Commitment(),
					tc.wantCommitment,
				)
			}
			urlFailure, gotURLFailure := errors.AsType[*url.Error](gotErr)
			if gotURLFailure != tc.wantSafeURL {
				t.Fatalf(
					"Upload() URL error present = %t, want %t",
					gotURLFailure,
					tc.wantSafeURL,
				)
			}
			if gotURLFailure &&
				(urlFailure.URL != core.RedactedValueText ||
					urlFailure.Op != "Put" ||
					!errors.Is(urlFailure, nativeFailure)) {
				t.Fatalf(
					"Upload() URL error = op %q URL %q cause %v, want Put, %q, and native identity",
					urlFailure.Op,
					urlFailure.URL,
					urlFailure.Err,
					core.RedactedValueText,
				)
			}
		})
	}
}

func BenchmarkUploadStreaming1KiB(b *testing.B) {
	b.ReportAllocs()
	benchmarkUploadStreaming(b, 1024)
}

func BenchmarkUploadStreaming10MiB(b *testing.B) {
	b.ReportAllocs()
	benchmarkUploadStreaming(b, 10*1024*1024)
}

func benchmarkUploadStreaming(b *testing.B, size int) {
	payload := bytes.Repeat([]byte("o"), size)
	httpClient := &http.Client{Transport: benchmarkDrainTransport{}}
	client := newObjectstoreClient(b, httpClient)
	request := uploadRequest(
		b,
		objectstore.ProviderGoogleCloudStorage,
		signedProviderURL(
			"https://storage.googleapis.com/bucket/object",
			objectstore.ProviderGoogleCloudStorage,
			objectstore.DirectionUpload,
		),
		payload,
	)
	b.ReportAllocs()
	b.SetBytes(int64(size))
	b.ResetTimer()

	for b.Loop() {
		request.Source = bytes.NewReader(payload)
		got, gotErr := objectstore.UploadGCS(
			context.Background(),
			client,
			request,
		)
		if gotErr != nil ||
			got.Commitment() != objectstore.CommitmentConfirmed {
			b.Fatalf(
				"Upload() = (commitment %v, %v), want (confirmed, nil)",
				got.Commitment(),
				gotErr,
			)
		}
	}
}

type benchmarkDrainTransport struct{}

func (benchmarkDrainTransport) RoundTrip(
	request *http.Request,
) (*http.Response, error) {
	_, copyErr := io.Copy(io.Discard, request.Body)
	closeErr := request.Body.Close()
	if err := errors.Join(copyErr, closeErr); err != nil {
		return nil, err
	}
	headers := make(http.Header)
	headers.Set("X-Goog-Generation", "42")
	return &http.Response{
		StatusCode:    http.StatusOK,
		Header:        headers,
		Body:          http.NoBody,
		ContentLength: 0,
		Request:       request,
	}, nil
}

type providerVersionTransport struct {
	headerName   string
	headerValues []string
}

func (t providerVersionTransport) RoundTrip(
	request *http.Request,
) (*http.Response, error) {
	_, copyErr := io.Copy(io.Discard, request.Body)
	closeErr := request.Body.Close()
	if err := errors.Join(copyErr, closeErr); err != nil {
		return nil, err
	}
	headers := make(http.Header)
	if t.headerName != "" {
		headers[http.CanonicalHeaderKey(t.headerName)] = append(
			[]string(nil),
			t.headerValues...,
		)
	}
	return &http.Response{
		StatusCode:    http.StatusOK,
		Header:        headers,
		Body:          http.NoBody,
		ContentLength: 0,
		Request:       request,
	}, nil
}

type failingDestination struct {
	cause error
}

func (w failingDestination) Write(_ []byte) (int, error) {
	return 0, w.cause
}

type failureTransport struct {
	cause error
}

func (t failureTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, t.cause
}

func providerServer(
	tb testing.TB,
	provider objectstore.Provider,
	direction objectstore.Direction,
	handler http.Handler,
) (string, *http.Client) {
	tb.Helper()

	server := httptest.NewTLSServer(handler)
	tb.Cleanup(server.Close)
	serverAddress := strings.TrimPrefix(server.URL, "https://")
	transport := server.Client().Transport.(*http.Transport).Clone()
	transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	transport.TLSClientConfig.ServerName = "example.com"
	dialer := &net.Dialer{}
	transport.DialContext = func(
		ctx context.Context,
		network string,
		_ string,
	) (net.Conn, error) {
		return dialer.DialContext(ctx, network, serverAddress)
	}
	client := &http.Client{Transport: transport}
	tb.Cleanup(transport.CloseIdleConnections)
	return signedProviderURL(providerEndpoint(provider), provider, direction), client
}

func signedProviderURL(
	endpoint string,
	provider objectstore.Provider,
	direction objectstore.Direction,
) string {
	switch provider {
	case objectstore.ProviderAmazonS3:
		signed := testS3UploadSignedHeaders
		if direction == objectstore.DirectionDownload {
			signed = testS3DownloadSignedHeaders
		}
		return endpoint +
			"?X-Amz-Signature=signature&X-Amz-SignedHeaders=" +
			url.QueryEscape(signed)
	case objectstore.ProviderGoogleCloudStorage:
		signed := testGCSUploadSignedHeaders
		if direction == objectstore.DirectionDownload {
			signed = testGCSDownloadSignedHeaders
		}
		return endpoint +
			"?X-Goog-Signature=signature&X-Goog-SignedHeaders=" +
			url.QueryEscape(signed)
	case objectstore.ProviderCloudflareImages:
		return endpoint
	default:
		return endpoint
	}
}

func providerEndpoint(provider objectstore.Provider) string {
	switch provider {
	case objectstore.ProviderAmazonS3:
		return "https://s3.amazonaws.com/bucket/object"
	case objectstore.ProviderGoogleCloudStorage:
		return "https://storage.googleapis.com/bucket/object"
	case objectstore.ProviderCloudflareImages:
		return "https://upload.imagedelivery.net/image-id"
	default:
		return "https://example.com/object"
	}
}

func newObjectstoreClient(
	tb testing.TB,
	httpClient *http.Client,
) objectstore.Client {
	tb.Helper()

	exchangeClient, gotClientErr := exchange.NewClient(httpClient)
	if gotClientErr != nil {
		tb.Fatalf("exchange.NewClient() setup error = %v, want nil", gotClientErr)
	}
	client, gotClientErr := objectstore.NewClient(exchangeClient)
	if gotClientErr != nil {
		tb.Fatalf(
			"objectstore.NewClient() setup error = %v, want nil",
			gotClientErr,
		)
	}
	return client
}

func uploadRequest(
	tb testing.TB,
	_ objectstore.Provider,
	targetURL string,
	payload []byte,
) objectstore.UploadRequest {
	tb.Helper()

	headers, gotHeadersErr := objectstore.NewSignedHeaders(nil)
	if gotHeadersErr != nil {
		tb.Fatalf(
			"NewSignedHeaders() setup error = %v, want nil",
			gotHeadersErr,
		)
	}
	return objectstore.UploadRequest{
		Source: bytes.NewReader(payload),
		Target: objectstore.UploadTarget{
			URL:       signedURL(tb, targetURL),
			Headers:   headers,
			ExpiresAt: futureInstant(tb),
		},
		Integrity:   integrity(tb, payload),
		ContentType: core.HTTPMediaTypeOctetStream(),
		Policy:      operationPolicy(tb),
	}
}

func downloadRequest(
	tb testing.TB,
	_ objectstore.Provider,
	targetURL string,
	destination io.Writer,
	payload []byte,
) objectstore.DownloadRequest {
	tb.Helper()

	headers, gotHeadersErr := objectstore.NewSignedHeaders(nil)
	if gotHeadersErr != nil {
		tb.Fatalf(
			"NewSignedHeaders() setup error = %v, want nil",
			gotHeadersErr,
		)
	}
	return objectstore.DownloadRequest{
		Destination: destination,
		Target: objectstore.DownloadTarget{
			URL:       signedURL(tb, targetURL),
			Headers:   headers,
			ExpiresAt: futureInstant(tb),
		},
		Integrity:   integrity(tb, payload),
		ContentType: core.HTTPMediaTypeOctetStream(),
		Policy:      operationPolicy(tb),
	}
}

func signedURL(tb testing.TB, value string) objectstore.SignedURL {
	tb.Helper()

	parsed, gotErr := objectstore.ParseSignedURL(value)
	if gotErr != nil {
		tb.Fatalf("ParseSignedURL() setup error = %v, want nil", gotErr)
	}
	return parsed
}

func futureInstant(tb testing.TB) temporal.Instant {
	tb.Helper()

	return instantAt(
		tb,
		time.Date(2035, time.January, 1, 0, 0, 0, 0, time.UTC),
	)
}

func instantAt(tb testing.TB, value time.Time) temporal.Instant {
	tb.Helper()

	instant, gotErr := temporal.NewInstant(value)
	if gotErr != nil {
		tb.Fatalf("temporal.NewInstant() setup error = %v, want nil", gotErr)
	}
	return instant
}

func operationPolicy(tb testing.TB) objectstore.Policy {
	tb.Helper()

	errorLimit, gotLimitErr := core.NewByteCount(4096)
	if gotLimitErr != nil {
		tb.Fatalf(
			"core.NewByteCount() setup error = %v, want nil",
			gotLimitErr,
		)
	}
	return objectstore.Policy{
		OperationTimeout: durationSeconds(tb, 10),
		AttemptTimeout:   durationSeconds(tb, 5),
		ErrorBodyLimit:   errorLimit,
	}
}

func durationSeconds(tb testing.TB, seconds uint64) temporal.Duration {
	tb.Helper()

	value, gotErr := temporal.DurationFromSeconds(seconds)
	if gotErr != nil {
		tb.Fatalf(
			"temporal.DurationFromSeconds() setup error = %v, want nil",
			gotErr,
		)
	}
	return value
}

func integrity(testingContext testing.TB, payload []byte) objectstore.Integrity {
	testingContext.Helper()
	sha := sha256.Sum256(payload)
	return objectstore.Integrity{
		SHA256: core.NewSHA256Digest(sha),
		Length: mustByteLength(testingContext, uint64(len(payload))),
		CRC32C: core.NewCRC32C(
			crc32.Checksum(payload, crc32.MakeTable(crc32.Castagnoli)),
		),
	}
}

func parsedHeaderName(
	tb testing.TB,
	value string,
) core.HTTPHeaderName {
	tb.Helper()

	name, gotErr := core.ParseHTTPHeaderName(value)
	if gotErr != nil {
		tb.Fatalf(
			"core.ParseHTTPHeaderName(%q) setup error = %v, want nil",
			value,
			gotErr,
		)
	}
	return name
}

func signedHeader(
	tb testing.TB,
	name string,
	value string,
) objectstore.SignedHeader {
	tb.Helper()

	header, gotErr := objectstore.NewSignedHeader(
		parsedHeaderName(tb, name),
		value,
	)
	if gotErr != nil {
		tb.Fatalf(
			"NewSignedHeader(%q) setup error = %v, want nil",
			name,
			gotErr,
		)
	}
	return header
}

func signedHeaderCount(
	tb testing.TB,
	count int,
) []objectstore.SignedHeader {
	tb.Helper()

	headers := make([]objectstore.SignedHeader, count)
	for index := range headers {
		headers[index] = signedHeader(
			tb,
			fmt.Sprintf("X-Signed-%02d", index),
			"value",
		)
	}
	return headers
}

func signedHeaderWireBoundary(
	tb testing.TB,
	delta int,
) []objectstore.SignedHeader {
	tb.Helper()

	const (
		headerCount     = 4
		headerNameBytes = 3
		wireSyntaxBytes = 4
	)
	lastValueBytes := objectstore.SignedHeaderMaximumBytes -
		headerCount*(headerNameBytes+wireSyntaxBytes) -
		3*exchange.HeaderValueMaximumBytes +
		delta
	return []objectstore.SignedHeader{
		signedHeader(
			tb,
			"X-A",
			strings.Repeat("a", exchange.HeaderValueMaximumBytes),
		),
		signedHeader(
			tb,
			"X-B",
			strings.Repeat("b", exchange.HeaderValueMaximumBytes),
		),
		signedHeader(
			tb,
			"X-C",
			strings.Repeat("c", exchange.HeaderValueMaximumBytes),
		),
		signedHeader(tb, "X-D", strings.Repeat("d", lastValueBytes)),
	}
}

func objectstoreOwnedHeaderNames(tb testing.TB) []core.HTTPHeaderName {
	tb.Helper()

	return []core.HTTPHeaderName{
		core.HTTPHeaderContentType(),
		core.HTTPHeaderContentLength(),
		core.HTTPHeaderAcceptEncoding(),
		core.HTTPHeaderContentEncoding(),
		core.HTTPHeaderAccept(),
		parsedHeaderName(tb, "Authorization"),
		core.HTTPHeaderIdempotencyKey(),
		parsedHeaderName(tb, "Content-Range"),
		parsedHeaderName(tb, "Host"),
		parsedHeaderName(tb, "Range"),
		parsedHeaderName(tb, "If-None-Match"),
		parsedHeaderName(tb, "X-Amz-Checksum-Crc32c"),
		parsedHeaderName(tb, "X-Amz-Checksum-Mode"),
		parsedHeaderName(tb, "X-Goog-Hash"),
		parsedHeaderName(tb, "X-Goog-If-Generation-Match"),
	}
}

func verifyTransferEvidence(
	t *testing.T,
	transfer objectstore.Transfer,
	want objectstore.Integrity,
) {
	t.Helper()

	status, present := transfer.Status()
	statusCode, statusErr := status.Int()
	if !present || statusErr != nil || statusCode != http.StatusOK {
		t.Fatalf(
			"Transfer.Status() = (code %d, present %t, error %v), want (200, true, nil)",
			statusCode,
			present,
			statusErr,
		)
	}
	if transfer.Bytes() != want.Length ||
		transfer.SHA256() != want.SHA256 ||
		transfer.CRC32C() != want.CRC32C {
		t.Fatalf(
			"Transfer evidence = bytes %d SHA-256 %v CRC32C %v, want bytes %d SHA-256 %v CRC32C %v",
			transfer.Bytes().Uint64(),
			transfer.SHA256(),
			transfer.CRC32C(),
			want.Length.Uint64(),
			want.SHA256,
			want.CRC32C,
		)
	}
}

func observeUpload(request *http.Request) providerObservation {
	observation := providerObservation{
		method: request.Method,
		contentType: request.Header.Get(
			core.HTTPHeaderContentType().String(),
		),
		checksum:   request.Header.Get("X-Amz-Checksum-Crc32c"),
		createOnly: request.Header.Get("If-None-Match"),
	}
	if observation.checksum == "" {
		observation.checksum = request.Header.Get("X-Goog-Hash")
		observation.createOnly = request.Header.Get(
			"X-Goog-If-Generation-Match",
		)
	}
	base, parameters, mediaErr := mime.ParseMediaType(observation.contentType)
	if mediaErr == nil && base == "multipart/form-data" {
		reader := multipart.NewReader(
			request.Body,
			parameters["boundary"],
		)
		part, partErr := reader.NextPart()
		if partErr == nil {
			observation.multipartField = part.FormName()
			observation.multipartFilename = part.FileName()
			observation.body, _ = io.ReadAll(part)
		}
		return observation
	}
	observation.body, _ = io.ReadAll(request.Body)
	return observation
}

func verifyUploadObservation(
	t *testing.T,
	provider objectstore.Provider,
	got providerObservation,
) {
	t.Helper()

	switch provider {
	case objectstore.ProviderAmazonS3:
		if got.method != http.MethodPut ||
			got.createOnly != "*" ||
			got.checksum == "" {
			t.Fatalf(
				"S3 request = method %q create-only %q checksum %q, want PUT, *, and CRC32C",
				got.method,
				got.createOnly,
				got.checksum,
			)
		}
	case objectstore.ProviderGoogleCloudStorage:
		if got.method != http.MethodPut ||
			got.createOnly != "0" ||
			!strings.HasPrefix(got.checksum, "crc32c=") {
			t.Fatalf(
				"GCS request = method %q generation-match %q checksum %q, want PUT, 0, and crc32c",
				got.method,
				got.createOnly,
				got.checksum,
			)
		}
	case objectstore.ProviderCloudflareImages:
		if got.method != http.MethodPost ||
			got.multipartField != "file" ||
			got.multipartFilename != "file" {
			t.Fatalf(
				"Cloudflare request = method %q field %q filename %q, want POST file/file",
				got.method,
				got.multipartField,
				got.multipartFilename,
			)
		}
	default:
		t.Fatalf("provider = %v, want closed supported provider", provider)
	}
}

func setProviderVersion(
	headers http.Header,
	provider objectstore.Provider,
) {
	value := providerVersionValue(provider)
	switch provider {
	case objectstore.ProviderAmazonS3:
		headers.Set("X-Amz-Version-Id", value)
	case objectstore.ProviderGoogleCloudStorage:
		headers.Set("X-Goog-Generation", value)
	case objectstore.ProviderCloudflareImages:
	}
}

func providerVersionValue(provider objectstore.Provider) string {
	switch provider {
	case objectstore.ProviderAmazonS3:
		return "version-42"
	case objectstore.ProviderGoogleCloudStorage:
		return "42"
	case objectstore.ProviderCloudflareImages:
		return ""
	default:
		return ""
	}
}

func verifyProviderVersion(
	t *testing.T,
	transfer objectstore.Transfer,
	provider objectstore.Provider,
) {
	t.Helper()

	version, present := transfer.Version()
	wantValue := providerVersionValue(provider)
	wantPresent := wantValue != ""
	if present != wantPresent {
		t.Fatalf(
			"Transfer.Version() presence = %t, want %t",
			present,
			wantPresent,
		)
	}
	if !wantPresent {
		if !version.IsZero() {
			t.Fatalf(
				"Transfer.Version() absent value zero = %t, want true",
				version.IsZero(),
			)
		}
		return
	}
	if gotValidateErr := version.Validate(); gotValidateErr != nil {
		t.Fatalf(
			"Transfer.Version().Validate() error = %v, want nil",
			gotValidateErr,
		)
	}
	if version.Provider() != provider || version.String() != wantValue {
		t.Fatalf(
			"Transfer.Version() = provider %v value %q, want provider %v value %q",
			version.Provider(),
			version.String(),
			provider,
			wantValue,
		)
	}
}

// TestDownloadProviderChecksumLayerTriad proves the provider's own CRC32C is
// compared against the locally streamed checksum. Before this gate the request
// demanded S3 checksum mode, and required it in the signed-header declaration,
// only to discard the answer.
func TestDownloadProviderChecksumLayerTriad(t *testing.T) {
	t.Parallel()

	payload := bytes.Repeat([]byte("checksum-proof-"), 512)
	matching := providerChecksumBase64(t, payload)
	divergent := providerChecksumBase64(t, append([]byte("x"), payload...))
	cases := []struct {
		wantErr      error
		name         string
		checksum     string
		checksumType string
		provider     objectstore.Provider
	}{
		{
			name:         "positive S3 full-object checksum agrees with the streamed object",
			provider:     objectstore.ProviderAmazonS3,
			checksum:     matching,
			checksumType: "FULL_OBJECT",
		},
		{
			name:     "positive S3 legacy checksum agrees without a type field",
			provider: objectstore.ProviderAmazonS3,
			checksum: matching,
		},
		{
			name:     "positive GCS crc32c component agrees with the streamed object",
			provider: objectstore.ProviderGoogleCloudStorage,
			checksum: matching,
		},
		{
			name:         "negative S3 full-object checksum contradicts the streamed object",
			provider:     objectstore.ProviderAmazonS3,
			checksum:     divergent,
			checksumType: "FULL_OBJECT",
			wantErr:      core.ErrObjectStoreIntegrity,
		},
		{
			name:     "negative GCS crc32c component contradicts the streamed object",
			provider: objectstore.ProviderGoogleCloudStorage,
			checksum: divergent,
			wantErr:  core.ErrObjectStoreIntegrity,
		},
		{
			name:     "negative S3 checksum is not decodable",
			provider: objectstore.ProviderAmazonS3,
			checksum: "not-base64!!",
			wantErr:  core.ErrObjectStoreIntegrity,
		},
		{
			name:         "neutral S3 composite checksum is not compared as whole-object CRC32C",
			provider:     objectstore.ProviderAmazonS3,
			checksum:     divergent,
			checksumType: "COMPOSITE",
		},
		{
			name:         "negative S3 unknown checksum type is rejected",
			provider:     objectstore.ProviderAmazonS3,
			checksum:     matching,
			checksumType: "FUTURE",
			wantErr:      core.ErrObjectStoreIntegrity,
		},
		{
			name:     "neutral S3 object stored without a checksum",
			provider: objectstore.ProviderAmazonS3,
		},
		{
			name:     "neutral GCS response without a crc32c component",
			provider: objectstore.ProviderGoogleCloudStorage,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			handler := http.HandlerFunc(func(
				writer http.ResponseWriter,
				request *http.Request,
			) {
				writer.Header().Set(
					core.HTTPHeaderContentType().String(),
					core.HTTPMediaTypeOctetStream().String(),
				)
				setProviderVersion(writer.Header(), tc.provider)
				setProviderChecksum(writer.Header(), tc.provider, tc.checksum)
				if tc.checksumType != "" {
					writer.Header().Set(
						"X-Amz-Checksum-Type",
						tc.checksumType,
					)
				}
				writer.WriteHeader(http.StatusOK)
				_, _ = writer.Write(payload)
			})
			targetURL, httpClient := providerServer(
				t, tc.provider, objectstore.DirectionDownload, handler,
			)
			client := newObjectstoreClient(t, httpClient)
			var destination bytes.Buffer
			request := downloadRequest(
				t,
				tc.provider,
				targetURL,
				&destination,
				payload,
			)

			got, gotErr := downloadOperationFor(tc.provider)(
				context.Background(),
				client,
				request,
			)
			if tc.wantErr == nil {
				if gotErr != nil || got.Validate() != nil ||
					got.Commitment() != objectstore.CommitmentConfirmed {
					t.Fatalf(
						"Download() commitment/error = (%v, %v), want confirmed and nil",
						got.Commitment(),
						gotErr,
					)
				}
				verifyTransferEvidence(t, got, request.Integrity)
				return
			}
			if !errors.Is(gotErr, tc.wantErr) ||
				got.Commitment() != objectstore.CommitmentRejected {
				t.Fatalf(
					"Download() commitment/error = (%v, %v), want rejected and %v",
					got.Commitment(),
					gotErr,
					tc.wantErr,
				)
			}
			if got.Validate() == nil {
				t.Fatalf(
					"Download() rejected result Validate() = nil, want a refused transfer",
				)
			}
		})
	}
}

func providerChecksumBase64(tb testing.TB, payload []byte) string {
	tb.Helper()

	value := core.NewCRC32C(
		crc32.Checksum(payload, crc32.MakeTable(crc32.Castagnoli)),
	)
	encoded, gotErr := value.Base64()
	if gotErr != nil {
		tb.Fatalf("core.CRC32C.Base64() setup error = %v, want nil", gotErr)
	}
	return encoded
}

func setProviderChecksum(
	headers http.Header,
	provider objectstore.Provider,
	checksum string,
) {
	if checksum == "" {
		return
	}
	switch provider {
	case objectstore.ProviderAmazonS3:
		headers.Set("X-Amz-Checksum-Crc32c", checksum)
	case objectstore.ProviderGoogleCloudStorage:
		headers.Set(
			"X-Goog-Hash",
			"md5=1B2M2Y8AsgTpgAmY7PhCfg==,crc32c="+checksum,
		)
	case objectstore.ProviderCloudflareImages:
	}
}
