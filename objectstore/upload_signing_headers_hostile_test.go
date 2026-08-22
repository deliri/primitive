package objectstore

import (
	"errors"
	"hash/crc32"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestUploadSigningHeadersLayerTriadOwnsExactProviderRequestFields(t *testing.T) {
	t.Parallel()

	payload := []byte("primitive upload signing projection")
	integrity := providerIntegrity(t, payload)
	emptyHeaders, err := NewSignedHeaders(nil)
	if err != nil {
		t.Fatalf("NewSignedHeaders(nil) error = %v, want nil", err)
	}

	t.Run("positive GCS projection carries exact create-only and CRC32C fields", func(t *testing.T) {
		t.Parallel()

		got, gotErr := NewUploadSigningHeaders(
			ProviderGoogleCloudStorage,
			emptyHeaders,
			integrity,
		)
		if gotErr != nil {
			t.Fatalf("NewUploadSigningHeaders(GCS) error = %v, want nil", gotErr)
		}
		wantChecksum, checksumErr := integrity.CRC32C.Base64()
		if checksumErr != nil {
			t.Fatalf("Integrity.CRC32C.Base64() error = %v, want nil", checksumErr)
		}
		want := []providerHeader{
			{name: headerGCSGenerationMatch, value: headerZeroValue},
			{name: headerGCSHash, value: headerGCSChecksumPrefix + wantChecksum},
		}
		if err := signingHeadersMatch(got, want); err != nil {
			t.Fatalf("NewUploadSigningHeaders(GCS) mismatch = %v", err)
		}
	})

	t.Run("negative every non-raw or future provider value is refused", func(t *testing.T) {
		t.Parallel()

		for raw := range math.MaxUint8 + 1 {
			provider := Provider(raw)
			if provider == ProviderAmazonS3 || provider == ProviderGoogleCloudStorage {
				continue
			}
			got, gotErr := NewUploadSigningHeaders(provider, emptyHeaders, integrity)
			if !errors.Is(gotErr, core.ErrObjectStoreContract) || len(got.Values) != 0 {
				t.Fatalf(
					"NewUploadSigningHeaders(Provider(%d)) = (%v, %v), want zero and errors.Is(..., %v)",
					raw,
					got,
					gotErr,
					core.ErrObjectStoreContract,
				)
			}
		}
	})

	t.Run("neutral empty GCS object retains exact fields without caller metadata", func(t *testing.T) {
		t.Parallel()

		emptyIntegrity := Integrity{
			Length: core.ByteLength{},
			SHA256: core.SHA256Of(nil),
			CRC32C: core.NewCRC32C(crc32.Checksum(nil, crc32.MakeTable(crc32.Castagnoli))),
		}
		length, lengthErr := core.NewByteLength(0)
		if lengthErr != nil {
			t.Fatalf("core.NewByteLength(0) error = %v, want nil", lengthErr)
		}
		emptyIntegrity.Length = length
		got, gotErr := NewUploadSigningHeaders(
			ProviderGoogleCloudStorage,
			emptyHeaders,
			emptyIntegrity,
		)
		if gotErr != nil || len(got.Values) != 2 {
			t.Fatalf("NewUploadSigningHeaders(empty GCS) = (%v, %v), want two exact fields and nil", got, gotErr)
		}
	})
}

func signingHeadersMatch(got exchange.Headers, want []providerHeader) error {
	if len(got.Values) != len(want) {
		return core.ErrObjectStoreContract
	}
	for index, header := range got.Values {
		if len(header.Values) != 1 {
			return core.ErrObjectStoreContract
		}
		value, err := header.Values[0].Value()
		if err != nil || header.Name.String() != want[index].name || value != want[index].value {
			return core.ErrObjectStoreContract
		}
	}
	return nil
}
