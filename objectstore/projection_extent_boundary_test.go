package objectstore

import (
	json "encoding/json/v2"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestUploadHTTPProjectionExtentLayerTriad(t *testing.T) {
	t.Parallel()
	for _, provider := range []Provider{ProviderAmazonS3, ProviderGoogleCloudStorage} {
		t.Run(provider.String(), func(t *testing.T) {
			t.Parallel()
			spec, err := Spec(provider)
			if err != nil {
				t.Fatal(err)
			}
			for _, tc := range []struct {
				name  string
				bytes uint64
				want  error
			}{
				{name: "empty object remains an exact raw request"},
				{name: "one below provider maximum remains spendable", bytes: spec.UploadMaximum.Uint64() - 1},
				{name: "provider maximum can be rendered without reading the body", bytes: spec.UploadMaximum.Uint64()},
				{name: "one byte beyond provider maximum cannot become a spendable projection", bytes: spec.UploadMaximum.Uint64() + 1, want: core.ErrObjectStoreSize},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					capability, err := NewUploadCapabilityProjection(provider, providerUploadTarget(t, provider))
					if err != nil {
						t.Fatal(err)
					}
					declaration := providerIntegrity(t, nil)
					declaration.Length, err = core.NewByteLength(tc.bytes)
					if err != nil {
						t.Fatal(err)
					}
					got, err := NewUploadHTTPProjection(capability, declaration, core.HTTPMediaTypeOctetStream())
					if !errors.Is(err, tc.want) {
						t.Fatalf("constructor error = %v, want %v", err, tc.want)
					}
					if tc.want != nil {
						if !got.IsZero() {
							t.Fatalf("refused projection IsZero = %t, want true", got.IsZero())
						}
						return
					}
					if err := got.Validate(); err != nil {
						t.Fatal(err)
					}
					encoded, err := got.MarshalJSON()
					if err != nil {
						t.Fatalf("admitted projection cannot encode: %v", err)
					}
					var wire uploadHTTPProjectionWire
					if err := json.Unmarshal(encoded, &wire); err != nil {
						t.Fatal(err)
					}
					if wire.Bytes == nil || *wire.Bytes != declaration.Length || wire.SHA256 == nil || *wire.SHA256 != declaration.SHA256 || wire.CRC32C == nil || *wire.CRC32C != declaration.CRC32C {
						t.Fatalf("projected body = (%v, %v, %v), want exact (%v, %v, %v)", wire.Bytes, wire.SHA256, wire.CRC32C, declaration.Length, declaration.SHA256, declaration.CRC32C)
					}
				})
			}
		})
	}
}
