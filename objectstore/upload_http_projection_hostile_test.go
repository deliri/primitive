package objectstore

import (
	json "encoding/json/v2"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestUploadHTTPProjectionCarriesOneCompleteBrowserSpendableRequest(t *testing.T) {
	t.Parallel()

	const metadataValue = "run-41"
	rawURL := capabilityGCSObject + "?" + queryGCSSignature + "=signature&" +
		queryGCSSignedHeaders + "=content-type%3Bhost%3Bx-goog-hash%3B" +
		"x-goog-if-generation-match%3Bx-goog-meta-run"
	signedURL, gotURLErr := ParseSignedURL(rawURL)
	if gotURLErr != nil {
		t.Fatalf("ParseSignedURL() error = %v, want nil", gotURLErr)
	}
	metadataName, gotNameErr := core.ParseHTTPHeaderName("X-Goog-Meta-Run")
	if gotNameErr != nil {
		t.Fatalf("core.ParseHTTPHeaderName() error = %v, want nil", gotNameErr)
	}
	metadata, gotHeaderErr := NewSignedHeader(metadataName, metadataValue)
	if gotHeaderErr != nil {
		t.Fatalf("NewSignedHeader() error = %v, want nil", gotHeaderErr)
	}
	headers, gotHeadersErr := NewSignedHeaders([]SignedHeader{metadata})
	if gotHeadersErr != nil {
		t.Fatalf("NewSignedHeaders() error = %v, want nil", gotHeadersErr)
	}
	capability, gotCapabilityErr := NewUploadCapabilityProjection(
		ProviderGoogleCloudStorage,
		UploadTarget{
			URL:       signedURL,
			Headers:   headers,
			ExpiresAt: temporal.InstantFromNanoseconds(1_893_456_000_000_000_000),
		},
	)
	if gotCapabilityErr != nil {
		t.Fatalf("NewUploadCapabilityProjection() error = %v, want nil", gotCapabilityErr)
	}
	payload := []byte("browser-upload")
	integrity := providerIntegrity(t, payload)
	projection, gotProjectionErr := NewUploadHTTPProjection(
		capability,
		integrity,
		core.HTTPMediaTypeOctetStream(),
	)
	if gotProjectionErr != nil {
		t.Fatalf("NewUploadHTTPProjection() error = %v, want nil", gotProjectionErr)
	}
	wantCommitment, gotCapabilityCommitmentErr := capability.Commitment()
	if gotCapabilityCommitmentErr != nil {
		t.Fatalf("UploadCapabilityProjection.Commitment() error = %v, want nil", gotCapabilityCommitmentErr)
	}
	gotCommitment, gotProjectionCommitmentErr := projection.Commitment()
	if gotProjectionCommitmentErr != nil {
		t.Fatalf("UploadHTTPProjection.Commitment() error = %v, want nil", gotProjectionCommitmentErr)
	}
	if gotCommitment != wantCommitment {
		t.Fatalf("UploadHTTPProjection.Commitment() = %v, want underlying capability commitment %v", gotCommitment, wantCommitment)
	}
	encoded, gotMarshalErr := projection.MarshalJSON()
	if gotMarshalErr != nil {
		t.Fatalf("UploadHTTPProjection.MarshalJSON() error = %v, want nil", gotMarshalErr)
	}
	var wire uploadHTTPProjectionWire
	if gotDecodeErr := json.Unmarshal(encoded, &wire); gotDecodeErr != nil {
		t.Fatalf("json.Unmarshal(UploadHTTPProjection) error = %v, want nil", gotDecodeErr)
	}
	if wire.Provider == nil || *wire.Provider != ProviderGoogleCloudStorage.String() ||
		wire.Method == nil || *wire.Method != exchange.MethodPut ||
		wire.URL == nil || *wire.URL != rawURL ||
		wire.ExpiresAt == nil ||
		wire.Bytes == nil || *wire.Bytes != integrity.Length ||
		wire.SHA256 == nil || *wire.SHA256 != integrity.SHA256 ||
		wire.CRC32C == nil || *wire.CRC32C != integrity.CRC32C ||
		wire.ContentType == nil || *wire.ContentType != core.HTTPMediaTypeOctetStream() ||
		wire.ResponseVersionHeader == nil || *wire.ResponseVersionHeader != headerGCSGeneration {
		t.Fatalf("UploadHTTPProjection wire = %+v, want exact GCS request and response evidence facts", wire)
	}
	wantHeaders := []uploadCapabilityHeaderWire{
		wireHeader(t, core.HTTPHeaderContentType(), core.HTTPMediaTypeOctetStream().String()),
		wireHeader(t, mustHeaderName(t, headerGCSHash), headerGCSChecksumPrefix+mustCRC32CBase64(t, integrity.CRC32C)),
		wireHeader(t, mustHeaderName(t, headerGCSGenerationMatch), headerZeroValue),
		wireHeader(t, metadataName, metadataValue),
	}
	if !slices.EqualFunc(wire.Headers, wantHeaders, sameUploadCapabilityHeaderWire) {
		t.Fatalf("UploadHTTPProjection headers = %+v, want complete signed browser fields %+v", wire.Headers, wantHeaders)
	}
}

func TestUploadHTTPProjectionCarriesEveryRawProviderField(t *testing.T) {
	t.Parallel()

	payload := []byte("provider-browser-upload")
	integrity := providerIntegrity(t, payload)
	cases := []struct {
		name                string
		wantResponseHeader  string
		wantProviderHeaders []uploadCapabilityHeaderWire
		provider            Provider
	}{
		{
			name:               "amazon s3 raw put carries create-only checksum and version fields",
			provider:           ProviderAmazonS3,
			wantResponseHeader: headerS3Version,
			wantProviderHeaders: []uploadCapabilityHeaderWire{
				wireHeader(t, mustHeaderName(t, headerIfNoneMatch), headerCreateOnlyValue),
				wireHeader(t, mustHeaderName(t, headerS3ChecksumCRC32C), mustCRC32CBase64(t, integrity.CRC32C)),
			},
		},
		{
			name:               "google cloud storage raw put carries create-only checksum and generation fields",
			provider:           ProviderGoogleCloudStorage,
			wantResponseHeader: headerGCSGeneration,
			wantProviderHeaders: []uploadCapabilityHeaderWire{
				wireHeader(t, mustHeaderName(t, headerGCSHash), headerGCSChecksumPrefix+mustCRC32CBase64(t, integrity.CRC32C)),
				wireHeader(t, mustHeaderName(t, headerGCSGenerationMatch), headerZeroValue),
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			capability, gotCapabilityErr := NewUploadCapabilityProjection(
				tc.provider,
				providerUploadTarget(t, tc.provider),
			)
			if gotCapabilityErr != nil {
				t.Fatalf("NewUploadCapabilityProjection() error = %v, want nil", gotCapabilityErr)
			}
			projection, gotProjectionErr := NewUploadHTTPProjection(
				capability,
				integrity,
				core.HTTPMediaTypeOctetStream(),
			)
			if gotProjectionErr != nil {
				t.Fatalf("NewUploadHTTPProjection() error = %v, want nil", gotProjectionErr)
			}
			encoded, gotMarshalErr := json.Marshal(projection)
			if gotMarshalErr != nil {
				t.Fatalf("json.Marshal(UploadHTTPProjection) error = %v, want nil", gotMarshalErr)
			}
			var wire uploadHTTPProjectionWire
			if gotDecodeErr := json.Unmarshal(encoded, &wire); gotDecodeErr != nil {
				t.Fatalf("json.Unmarshal(UploadHTTPProjection) error = %v, want nil", gotDecodeErr)
			}
			wantHeaders := append([]uploadCapabilityHeaderWire{
				wireHeader(t, core.HTTPHeaderContentType(), core.HTTPMediaTypeOctetStream().String()),
			}, tc.wantProviderHeaders...)
			slices.SortFunc(wantHeaders, compareUploadHTTPHeaderWire)
			if !slices.EqualFunc(wire.Headers, wantHeaders, sameUploadCapabilityHeaderWire) {
				t.Fatalf("UploadHTTPProjection headers = %+v, want %+v", wire.Headers, wantHeaders)
			}
			if wire.ResponseVersionHeader == nil || *wire.ResponseVersionHeader != tc.wantResponseHeader {
				t.Fatalf("response version header = %v, want %q", wire.ResponseVersionHeader, tc.wantResponseHeader)
			}
		})
	}
}

func TestUploadHTTPProjectionRejectsEveryIncompleteOrMultipartContract(t *testing.T) {
	t.Parallel()

	validCapability, gotCapabilityErr := NewUploadCapabilityProjection(
		ProviderGoogleCloudStorage,
		providerUploadTarget(t, ProviderGoogleCloudStorage),
	)
	if gotCapabilityErr != nil {
		t.Fatalf("NewUploadCapabilityProjection() error = %v, want nil", gotCapabilityErr)
	}
	cloudflareCapability, gotCloudflareErr := NewUploadCapabilityProjection(
		ProviderCloudflareImages,
		providerUploadTarget(t, ProviderCloudflareImages),
	)
	if gotCloudflareErr != nil {
		t.Fatalf("NewUploadCapabilityProjection(Cloudflare) error = %v, want nil", gotCloudflareErr)
	}
	validIntegrity := providerIntegrity(t, []byte("browser-upload"))
	validContentType := core.HTTPMediaTypeOctetStream()

	unsetCapability := validCapability
	unsetCapability.set = false
	unknownProvider := validCapability
	unknownProvider.provider = ProviderUnknown
	futureProvider := validCapability
	futureProvider.provider = providerLimit
	zeroTarget := validCapability
	zeroTarget.target = UploadTarget{}
	zeroURL := validCapability
	zeroURL.target.URL = SignedURL{}
	zeroExpiry := validCapability
	zeroExpiry.target.ExpiresAt = temporal.Instant{}
	invalidHeaders := validCapability
	invalidHeaders.target.Headers = SignedHeaders{values: []SignedHeader{{}}}
	zeroSHA256 := validIntegrity
	zeroSHA256.SHA256 = core.SHA256Digest{}
	zeroCRC32C := validIntegrity
	zeroCRC32C.CRC32C = core.CRC32C{}

	cases := []struct {
		name        string
		contentType core.HTTPMediaType
		capability  UploadCapabilityProjection
		integrity   Integrity
	}{
		{name: "zero capability is rejected", integrity: validIntegrity, contentType: validContentType},
		{name: "explicitly unset capability is rejected", capability: unsetCapability, integrity: validIntegrity, contentType: validContentType},
		{name: "unknown provider is rejected", capability: unknownProvider, integrity: validIntegrity, contentType: validContentType},
		{name: "future provider is rejected", capability: futureProvider, integrity: validIntegrity, contentType: validContentType},
		{name: "zero target is rejected", capability: zeroTarget, integrity: validIntegrity, contentType: validContentType},
		{name: "zero signed URL is rejected", capability: zeroURL, integrity: validIntegrity, contentType: validContentType},
		{name: "zero expiry is rejected", capability: zeroExpiry, integrity: validIntegrity, contentType: validContentType},
		{name: "invalid caller header set is rejected", capability: invalidHeaders, integrity: validIntegrity, contentType: validContentType},
		{name: "zero integrity is rejected", capability: validCapability, contentType: validContentType},
		{name: "zero sha256 is rejected", capability: validCapability, integrity: zeroSHA256, contentType: validContentType},
		{name: "zero crc32c is rejected", capability: validCapability, integrity: zeroCRC32C, contentType: validContentType},
		{name: "zero media type is rejected", capability: validCapability, integrity: validIntegrity},
		{name: "multipart provider is rejected", capability: cloudflareCapability, integrity: validIntegrity, contentType: validContentType},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := NewUploadHTTPProjection(tc.capability, tc.integrity, tc.contentType)
			if !errors.Is(gotErr, core.ErrObjectStoreContract) &&
				!errors.Is(gotErr, core.ErrObjectStoreSize) {
				t.Fatalf("NewUploadHTTPProjection() error = %v, want typed Objectstore rejection", gotErr)
			}
			if !got.IsZero() {
				t.Fatalf("rejected projection IsZero() = false, want true")
			}
		})
	}
}

func TestUploadHTTPProjectionRedactsTheBearerUnderEveryFormattingVerb(t *testing.T) {
	t.Parallel()

	capability, gotCapabilityErr := NewUploadCapabilityProjection(
		ProviderGoogleCloudStorage,
		providerUploadTarget(t, ProviderGoogleCloudStorage),
	)
	if gotCapabilityErr != nil {
		t.Fatalf("NewUploadCapabilityProjection() error = %v, want nil", gotCapabilityErr)
	}
	projection, gotProjectionErr := NewUploadHTTPProjection(
		capability,
		providerIntegrity(t, []byte("browser-upload")),
		core.HTTPMediaTypeOctetStream(),
	)
	if gotProjectionErr != nil {
		t.Fatalf("NewUploadHTTPProjection() error = %v, want nil", gotProjectionErr)
	}
	formats := []string{"%v", "%+v", "%#v", "%s", "%q", "%x", "%X", "%d", "%020v", "%.3v"}
	for _, format := range formats {
		rendered := fmt.Sprintf(format, projection)
		if rendered != core.RedactedValueText {
			t.Fatalf("fmt.Sprintf(%q) = %q, want %q", format, rendered, core.RedactedValueText)
		}
		if strings.Contains(rendered, capabilitySecret) || strings.Contains(rendered, core.SchemeHTTPS) {
			t.Fatalf("fmt.Sprintf(%q) disclosed bearer material", format)
		}
	}
}

func wireHeader(t testing.TB, name core.HTTPHeaderName, value string) uploadCapabilityHeaderWire {
	t.Helper()
	if err := name.Validate(); err != nil {
		t.Fatalf("HTTP header name Validate() error = %v, want nil", err)
	}
	return uploadCapabilityHeaderWire{Name: pointer(name.String()), Value: pointer(value)}
}

func mustHeaderName(t testing.TB, value string) core.HTTPHeaderName {
	t.Helper()
	name, err := core.ParseHTTPHeaderName(value)
	if err != nil {
		t.Fatalf("core.ParseHTTPHeaderName() error = %v, want nil", err)
	}
	return name
}

func mustCRC32CBase64(t testing.TB, value core.CRC32C) string {
	t.Helper()
	encoded, err := value.Base64()
	if err != nil {
		t.Fatalf("CRC32C.Base64() error = %v, want nil", err)
	}
	return encoded
}

func sameUploadCapabilityHeaderWire(left, right uploadCapabilityHeaderWire) bool {
	return left.Name != nil && right.Name != nil && *left.Name == *right.Name &&
		left.Value != nil && right.Value != nil && *left.Value == *right.Value
}

func pointer[T any](value T) *T { return &value }
