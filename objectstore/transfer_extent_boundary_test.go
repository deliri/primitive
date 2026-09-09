package objectstore

import (
	json "encoding/json/v2"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestTransferEvidenceCannotExceedProviderExtent(t *testing.T) {
	t.Parallel()
	for _, operation := range []struct {
		name      string
		provider  Provider
		direction Direction
		version   string
	}{
		{name: "S3 upload", provider: ProviderAmazonS3, direction: DirectionUpload},
		{name: "S3 download", provider: ProviderAmazonS3, direction: DirectionDownload},
		{name: "GCS upload", provider: ProviderGoogleCloudStorage, direction: DirectionUpload, version: "1"},
		{name: "GCS download", provider: ProviderGoogleCloudStorage, direction: DirectionDownload},
		{name: "Cloudflare upload", provider: ProviderCloudflareImages, direction: DirectionUpload},
	} {
		t.Run(operation.name, func(t *testing.T) {
			t.Parallel()
			spec, err := Spec(operation.provider)
			if err != nil {
				t.Fatal(err)
			}
			maximum := spec.UploadMaximum.Uint64()
			if operation.direction == DirectionDownload {
				maximum = spec.DownloadMaximum.Uint64()
			}
			for _, tc := range []struct {
				name   string
				length uint64
				want   error
			}{
				{name: "empty transfer remains a real object"},
				{name: "one below provider ceiling remains representable", length: maximum - 1},
				{name: "exact provider ceiling remains representable", length: maximum},
				{name: "one byte over the provider ceiling cannot become evidence", length: maximum + 1, want: core.ErrObjectStoreSize},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					baseline := sealedTransferEvidenceFixture(t, transferEvidenceFixtureRequest{Provider: operation.provider, Direction: operation.direction, Version: operation.version})
					preserved := transferEvidenceFromFixture(t, transferEvidenceFixtureRequest{Provider: operation.provider, Direction: operation.direction, Version: operation.version})
					length, err := core.NewByteLength(tc.length)
					if err != nil {
						t.Fatal(err)
					}
					candidate := baseline
					candidate.bytes = length
					if err := candidate.Validate(); !errors.Is(err, tc.want) {
						t.Errorf("Transfer.Validate = %v, want %v", err, tc.want)
					}
					projection, err := candidate.Evidence()
					if !errors.Is(err, tc.want) || (tc.want != nil && projection != (TransferEvidenceProjection{})) {
						t.Errorf("projection = (%v, %v), want exact admission or zero/%v", projection, err, tc.want)
					}
					wire := transferEvidenceWireFrom(preserved)
					wire.Bytes = &length
					encoded, err := json.Marshal(wire)
					if err != nil {
						t.Fatal(err)
					}
					got := preserved
					err = got.UnmarshalJSON(encoded)
					if !errors.Is(err, tc.want) {
						t.Errorf("ingress error = %v, want %v", err, tc.want)
					}
					want := preserved
					if tc.want == nil {
						want.bytes = length
					}
					if got != want {
						t.Errorf("received evidence = %v, want exact preserved facts %v", got, want)
					}
				})
			}
		})
	}
}
