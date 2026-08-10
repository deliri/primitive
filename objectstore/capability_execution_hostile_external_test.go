package objectstore_test

import (
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/temporal"
)

type capabilityExecutionMutation uint8

const (
	capabilityExecutionUploadZeroRequest capabilityExecutionMutation = iota
	capabilityExecutionUploadZeroCapability
	capabilityExecutionUploadNilSource
	capabilityExecutionUploadZeroContentType
	capabilityExecutionUploadZeroIntegrity
	capabilityExecutionUploadZeroSHA256
	capabilityExecutionUploadZeroCRC32C
	capabilityExecutionUploadZeroPolicy
	capabilityExecutionUploadZeroOperationTimeout
	capabilityExecutionUploadZeroAttemptTimeout
	capabilityExecutionUploadZeroErrorBodyLimit
	capabilityExecutionUploadAboveProviderMaximum
	capabilityExecutionDownloadZeroRequest
	capabilityExecutionDownloadZeroCapability
	capabilityExecutionDownloadNilDestination
	capabilityExecutionDownloadZeroContentType
	capabilityExecutionDownloadZeroIntegrity
	capabilityExecutionDownloadZeroSHA256
	capabilityExecutionDownloadZeroCRC32C
	capabilityExecutionDownloadZeroPolicy
	capabilityExecutionDownloadZeroOperationTimeout
	capabilityExecutionDownloadZeroAttemptTimeout
	capabilityExecutionDownloadZeroErrorBodyLimit
	capabilityExecutionDownloadAboveProviderMaximum
)

type capabilityExecutionCase struct {
	wantErr  error
	name     string
	mutation capabilityExecutionMutation
}

func TestCapabilityExecutionHostileRequestBoundaryMatrix(t *testing.T) {
	t.Parallel()

	cases := []capabilityExecutionCase{
		{name: "upload zero request", mutation: capabilityExecutionUploadZeroRequest, wantErr: core.ErrObjectStoreContract},
		{name: "upload zero capability", mutation: capabilityExecutionUploadZeroCapability, wantErr: core.ErrObjectStoreContract},
		{name: "upload nil source", mutation: capabilityExecutionUploadNilSource, wantErr: core.ErrObjectStoreSource},
		{name: "upload zero content type", mutation: capabilityExecutionUploadZeroContentType, wantErr: core.ErrObjectStoreContract},
		{name: "upload zero integrity", mutation: capabilityExecutionUploadZeroIntegrity, wantErr: core.ErrObjectStoreContract},
		{name: "upload zero SHA-256", mutation: capabilityExecutionUploadZeroSHA256, wantErr: core.ErrObjectStoreContract},
		{name: "upload zero CRC32C", mutation: capabilityExecutionUploadZeroCRC32C, wantErr: core.ErrObjectStoreContract},
		{name: "upload zero policy", mutation: capabilityExecutionUploadZeroPolicy, wantErr: core.ErrObjectStoreContract},
		{name: "upload zero operation timeout", mutation: capabilityExecutionUploadZeroOperationTimeout, wantErr: core.ErrObjectStoreContract},
		{name: "upload zero attempt timeout", mutation: capabilityExecutionUploadZeroAttemptTimeout, wantErr: core.ErrObjectStoreContract},
		{name: "upload zero error body limit", mutation: capabilityExecutionUploadZeroErrorBodyLimit, wantErr: core.ErrObjectStoreContract},
		{name: "upload one byte above provider maximum", mutation: capabilityExecutionUploadAboveProviderMaximum, wantErr: core.ErrObjectStoreSize},
		{name: "download zero request", mutation: capabilityExecutionDownloadZeroRequest, wantErr: core.ErrObjectStoreContract},
		{name: "download zero capability", mutation: capabilityExecutionDownloadZeroCapability, wantErr: core.ErrObjectStoreContract},
		{name: "download nil destination", mutation: capabilityExecutionDownloadNilDestination, wantErr: core.ErrObjectStoreDestination},
		{name: "download zero content type", mutation: capabilityExecutionDownloadZeroContentType, wantErr: core.ErrObjectStoreContract},
		{name: "download zero integrity", mutation: capabilityExecutionDownloadZeroIntegrity, wantErr: core.ErrObjectStoreContract},
		{name: "download zero SHA-256", mutation: capabilityExecutionDownloadZeroSHA256, wantErr: core.ErrObjectStoreContract},
		{name: "download zero CRC32C", mutation: capabilityExecutionDownloadZeroCRC32C, wantErr: core.ErrObjectStoreContract},
		{name: "download zero policy", mutation: capabilityExecutionDownloadZeroPolicy, wantErr: core.ErrObjectStoreContract},
		{name: "download zero operation timeout", mutation: capabilityExecutionDownloadZeroOperationTimeout, wantErr: core.ErrObjectStoreContract},
		{name: "download zero attempt timeout", mutation: capabilityExecutionDownloadZeroAttemptTimeout, wantErr: core.ErrObjectStoreContract},
		{name: "download zero error body limit", mutation: capabilityExecutionDownloadZeroErrorBodyLimit, wantErr: core.ErrObjectStoreContract},
		{name: "download one byte above provider maximum", mutation: capabilityExecutionDownloadAboveProviderMaximum, wantErr: core.ErrObjectStoreSize},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			payload := []byte{1}
			handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Errorf("provider transport was invoked by request validation")
			})
			uploadURL, _ := providerServer(
				t, objectstore.ProviderGoogleCloudStorage, objectstore.DirectionUpload, handler,
			)
			downloadURL, _ := providerServer(
				t, objectstore.ProviderGoogleCloudStorage, objectstore.DirectionDownload, handler,
			)
			uploadOrdinary := uploadRequest(t, objectstore.ProviderGoogleCloudStorage, uploadURL, payload)
			downloadOrdinary := downloadRequest(t, objectstore.ProviderGoogleCloudStorage, downloadURL, io.Discard, payload)
			upload := objectstore.UploadCapabilityRequest{
				Capability: receivedUploadCapability(t, objectstore.ProviderGoogleCloudStorage, uploadOrdinary.Target),
				Source:     uploadOrdinary.Source, ContentType: uploadOrdinary.ContentType,
				Integrity: uploadOrdinary.Integrity, Policy: uploadOrdinary.Policy,
			}
			download := objectstore.DownloadCapabilityRequest{
				Capability:  receivedDownloadCapability(t, objectstore.ProviderGoogleCloudStorage, downloadOrdinary.Target),
				Destination: io.Discard, ContentType: downloadOrdinary.ContentType,
				Integrity: downloadOrdinary.Integrity, Policy: downloadOrdinary.Policy,
			}
			var gotErr error
			switch tc.mutation {
			case capabilityExecutionUploadZeroRequest:
				gotErr = (objectstore.UploadCapabilityRequest{}).Validate()
			case capabilityExecutionUploadZeroCapability:
				upload.Capability = objectstore.UploadCapability{}
				gotErr = upload.Validate()
			case capabilityExecutionUploadNilSource:
				upload.Source = nil
				gotErr = upload.Validate()
			case capabilityExecutionUploadZeroContentType:
				upload.ContentType = core.HTTPMediaType{}
				gotErr = upload.Validate()
			case capabilityExecutionUploadZeroIntegrity:
				upload.Integrity = objectstore.Integrity{}
				gotErr = upload.Validate()
			case capabilityExecutionUploadZeroSHA256:
				upload.Integrity.SHA256 = core.SHA256Digest{}
				gotErr = upload.Validate()
			case capabilityExecutionUploadZeroCRC32C:
				upload.Integrity.CRC32C = core.CRC32C{}
				gotErr = upload.Validate()
			case capabilityExecutionUploadZeroPolicy:
				upload.Policy = objectstore.Policy{}
				gotErr = upload.Validate()
			case capabilityExecutionUploadZeroOperationTimeout:
				upload.Policy.OperationTimeout = temporal.Duration{}
				gotErr = upload.Validate()
			case capabilityExecutionUploadZeroAttemptTimeout:
				upload.Policy.AttemptTimeout = temporal.Duration{}
				gotErr = upload.Validate()
			case capabilityExecutionUploadZeroErrorBodyLimit:
				upload.Policy.ErrorBodyLimit = core.ByteCount{}
				gotErr = upload.Validate()
			case capabilityExecutionUploadAboveProviderMaximum:
				upload.Integrity.Length = mustByteLength(t, objectstore.GoogleCloudStorageObjectMaximumBytes+1)
				gotErr = upload.Validate()
			case capabilityExecutionDownloadZeroRequest:
				gotErr = (objectstore.DownloadCapabilityRequest{}).Validate()
			case capabilityExecutionDownloadZeroCapability:
				download.Capability = objectstore.DownloadCapability{}
				gotErr = download.Validate()
			case capabilityExecutionDownloadNilDestination:
				download.Destination = nil
				gotErr = download.Validate()
			case capabilityExecutionDownloadZeroContentType:
				download.ContentType = core.HTTPMediaType{}
				gotErr = download.Validate()
			case capabilityExecutionDownloadZeroIntegrity:
				download.Integrity = objectstore.Integrity{}
				gotErr = download.Validate()
			case capabilityExecutionDownloadZeroSHA256:
				download.Integrity.SHA256 = core.SHA256Digest{}
				gotErr = download.Validate()
			case capabilityExecutionDownloadZeroCRC32C:
				download.Integrity.CRC32C = core.CRC32C{}
				gotErr = download.Validate()
			case capabilityExecutionDownloadZeroPolicy:
				download.Policy = objectstore.Policy{}
				gotErr = download.Validate()
			case capabilityExecutionDownloadZeroOperationTimeout:
				download.Policy.OperationTimeout = temporal.Duration{}
				gotErr = download.Validate()
			case capabilityExecutionDownloadZeroAttemptTimeout:
				download.Policy.AttemptTimeout = temporal.Duration{}
				gotErr = download.Validate()
			case capabilityExecutionDownloadZeroErrorBodyLimit:
				download.Policy.ErrorBodyLimit = core.ByteCount{}
				gotErr = download.Validate()
			case capabilityExecutionDownloadAboveProviderMaximum:
				download.Integrity.Length = mustByteLength(t, objectstore.GoogleCloudStorageObjectMaximumBytes+1)
				gotErr = download.Validate()
			default:
				t.Fatalf("capability execution mutation = %d, want a published test mutation", tc.mutation)
			}
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("capability execution request Validate() error = %v, want errors.Is %v", gotErr, tc.wantErr)
			}
		})
	}
}
