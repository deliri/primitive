package objectstore_test

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"net/http"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
)

func TestCapabilityExecutionLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive received capability selects every exact provider operation", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name     string
			provider objectstore.Provider
		}{
			{name: "Amazon S3", provider: objectstore.ProviderAmazonS3},
			{name: "Google Cloud Storage", provider: objectstore.ProviderGoogleCloudStorage},
			{name: "Cloudflare Images", provider: objectstore.ProviderCloudflareImages},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				payload := bytes.Repeat([]byte{0x5a}, (32<<10)+1)
				observed := make(chan providerObservation, 1)
				handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
					observed <- observeUpload(request)
					setProviderVersion(writer.Header(), tc.provider)
					writer.WriteHeader(http.StatusOK)
				})
				targetURL, httpClient := providerServer(
					t, tc.provider, objectstore.DirectionUpload, handler,
				)
				ordinary := uploadRequest(t, tc.provider, targetURL, payload)
				capability := receivedUploadCapability(t, tc.provider, ordinary.Target)
				got, gotErr := objectstore.Upload(t.Context(), newObjectstoreClient(t, httpClient), objectstore.UploadCapabilityRequest{
					Capability: capability, Source: ordinary.Source,
					ContentType: ordinary.ContentType, Integrity: ordinary.Integrity, Policy: ordinary.Policy,
				})
				if gotErr != nil {
					t.Fatalf("objectstore.Upload() error = %v, want nil", gotErr)
				}
				if got.Validate() != nil || got.Provider() != tc.provider ||
					got.Direction() != objectstore.DirectionUpload ||
					got.Commitment() != objectstore.CommitmentConfirmed {
					t.Fatalf("objectstore.Upload() transfer = (%v, %v, %v, %v), want exact confirmed %v upload",
						got.Provider(), got.Direction(), got.Commitment(), got.Validate(), tc.provider)
				}
				gotObservation := <-observed
				if !bytes.Equal(gotObservation.body, payload) {
					t.Fatalf("provider body bytes = %d, want exact %d", len(gotObservation.body), len(payload))
				}
			})
		}
	})

	t.Run("negative zero capability refuses before source or transport", func(t *testing.T) {
		t.Parallel()

		payload := []byte{1, 2, 3}
		source := bytes.NewReader(payload)
		handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Errorf("provider transport was invoked, want refusal before transport")
		})
		targetURL, httpClient := providerServer(
			t, objectstore.ProviderGoogleCloudStorage, objectstore.DirectionUpload, handler,
		)
		ordinary := uploadRequest(t, objectstore.ProviderGoogleCloudStorage, targetURL, payload)
		_, gotErr := objectstore.Upload(t.Context(), newObjectstoreClient(t, httpClient), objectstore.UploadCapabilityRequest{
			Source: source, ContentType: ordinary.ContentType,
			Integrity: ordinary.Integrity, Policy: ordinary.Policy,
		})
		if !errors.Is(gotErr, core.ErrObjectStoreContract) {
			t.Fatalf("objectstore.Upload(zero capability) error = %v, want errors.Is %v", gotErr, core.ErrObjectStoreContract)
		}
		if gotRemaining := source.Len(); gotRemaining != len(payload) {
			t.Fatalf("source bytes remaining = %d, want untouched %d", gotRemaining, len(payload))
		}
	})

	t.Run("neutral canceled context refuses a valid capability without effects", func(t *testing.T) {
		t.Parallel()

		payload := []byte{4, 5, 6}
		source := bytes.NewReader(payload)
		handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			t.Errorf("provider transport was invoked, want canceled pre-effect refusal")
		})
		targetURL, httpClient := providerServer(
			t, objectstore.ProviderGoogleCloudStorage, objectstore.DirectionUpload, handler,
		)
		ordinary := uploadRequest(t, objectstore.ProviderGoogleCloudStorage, targetURL, payload)
		capability := receivedUploadCapability(t, objectstore.ProviderGoogleCloudStorage, ordinary.Target)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, gotErr := objectstore.Upload(ctx, newObjectstoreClient(t, httpClient), objectstore.UploadCapabilityRequest{
			Capability: capability, Source: source, ContentType: ordinary.ContentType,
			Integrity: ordinary.Integrity, Policy: ordinary.Policy,
		})
		if !errors.Is(gotErr, context.Canceled) {
			t.Fatalf("objectstore.Upload(canceled) error = %v, want errors.Is %v", gotErr, context.Canceled)
		}
		if gotRemaining := source.Len(); gotRemaining != len(payload) {
			t.Fatalf("source bytes remaining = %d, want untouched %d", gotRemaining, len(payload))
		}
	})
}

func TestDownloadCapabilityExecutionSelectsEveryDownloadProvider(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		provider objectstore.Provider
	}{
		{name: "Amazon S3", provider: objectstore.ProviderAmazonS3},
		{name: "Google Cloud Storage", provider: objectstore.ProviderGoogleCloudStorage},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			payload := bytes.Repeat([]byte{0xa5}, (32<<10)-1)
			handler := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeOctetStream().String())
				setProviderVersion(writer.Header(), tc.provider)
				writer.WriteHeader(http.StatusOK)
				_, _ = writer.Write(payload)
			})
			targetURL, httpClient := providerServer(
				t, tc.provider, objectstore.DirectionDownload, handler,
			)
			var destination bytes.Buffer
			ordinary := downloadRequest(t, tc.provider, targetURL, &destination, payload)
			capability := receivedDownloadCapability(t, tc.provider, ordinary.Target)
			got, gotErr := objectstore.Download(t.Context(), newObjectstoreClient(t, httpClient), objectstore.DownloadCapabilityRequest{
				Capability: capability, Destination: &destination,
				ContentType: ordinary.ContentType, Integrity: ordinary.Integrity, Policy: ordinary.Policy,
			})
			if gotErr != nil {
				t.Fatalf("objectstore.Download() error = %v, want nil", gotErr)
			}
			if got.Validate() != nil || got.Provider() != tc.provider ||
				got.Direction() != objectstore.DirectionDownload ||
				got.Commitment() != objectstore.CommitmentConfirmed {
				t.Fatalf("objectstore.Download() transfer = (%v, %v, %v, %v), want exact confirmed %v download",
					got.Provider(), got.Direction(), got.Commitment(), got.Validate(), tc.provider)
			}
			if !bytes.Equal(destination.Bytes(), payload) {
				t.Fatalf("download destination bytes = %d, want exact %d", destination.Len(), len(payload))
			}
		})
	}
}

func receivedUploadCapability(
	t *testing.T,
	provider objectstore.Provider,
	target objectstore.UploadTarget,
) objectstore.UploadCapability {
	t.Helper()

	projection, err := objectstore.NewUploadCapabilityProjection(provider, target)
	if err != nil {
		t.Fatalf("NewUploadCapabilityProjection() error = %v, want nil", err)
	}
	encoded, err := projection.MarshalJSON()
	if err != nil {
		t.Fatalf("UploadCapabilityProjection.MarshalJSON() error = %v, want nil", err)
	}
	var capability objectstore.UploadCapability
	if err := json.Unmarshal(encoded, &capability); err != nil {
		t.Fatalf("json.Unmarshal(UploadCapability) error = %v, want nil", err)
	}
	return capability
}

func receivedDownloadCapability(
	t *testing.T,
	provider objectstore.Provider,
	target objectstore.DownloadTarget,
) objectstore.DownloadCapability {
	t.Helper()

	projection, err := objectstore.NewDownloadCapabilityProjection(provider, target)
	if err != nil {
		t.Fatalf("NewDownloadCapabilityProjection() error = %v, want nil", err)
	}
	encoded, err := projection.MarshalJSON()
	if err != nil {
		t.Fatalf("DownloadCapabilityProjection.MarshalJSON() error = %v, want nil", err)
	}
	var capability objectstore.DownloadCapability
	if err := json.Unmarshal(encoded, &capability); err != nil {
		t.Fatalf("json.Unmarshal(DownloadCapability) error = %v, want nil", err)
	}
	return capability
}
