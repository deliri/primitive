package objectstore

import (
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// TestUploadCapabilityProjectionRoundTripsEveryPublishedProvider proves the
// issuer projection and the existing receiver share one exact capability
// document without making the receiver capable of emitting its bearer.
func TestUploadCapabilityProjectionRoundTripsEveryPublishedProvider(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		rawURL     string
		wantMethod string
		provider   Provider
	}{
		{
			name:       "amazon s3 whole-object signed put",
			provider:   ProviderAmazonS3,
			rawURL:     capabilityS3URL,
			wantMethod: UploadMethodTokenSignedPut,
		},
		{
			name:       "google cloud storage whole-object signed put",
			provider:   ProviderGoogleCloudStorage,
			rawURL:     capabilityGCSURL,
			wantMethod: UploadMethodTokenSignedPut,
		},
		{
			name:       "cloudflare images multipart post",
			provider:   ProviderCloudflareImages,
			rawURL:     capabilityImagesURL,
			wantMethod: UploadMethodTokenMultipartPost,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			signedURL, gotURLErr := ParseSignedURL(tc.rawURL)
			if gotURLErr != nil {
				t.Fatalf("ParseSignedURL() error = %v, want nil", gotURLErr)
			}
			headers, gotHeadersErr := NewSignedHeaders(nil)
			if gotHeadersErr != nil {
				t.Fatalf("NewSignedHeaders(nil) error = %v, want nil", gotHeadersErr)
			}
			target := UploadTarget{
				Headers:   headers,
				URL:       signedURL,
				ExpiresAt: temporal.InstantFromNanoseconds(1893456000000000000),
			}
			projection, gotProjectionErr := NewUploadCapabilityProjection(tc.provider, target)
			if gotProjectionErr != nil {
				t.Fatalf("NewUploadCapabilityProjection() error = %v, want nil", gotProjectionErr)
			}
			if gotValidateErr := projection.Validate(); gotValidateErr != nil {
				t.Fatalf("UploadCapabilityProjection.Validate() error = %v, want nil", gotValidateErr)
			}

			encoded, gotMarshalErr := projection.MarshalJSON()
			if gotMarshalErr != nil {
				t.Fatalf("UploadCapabilityProjection.MarshalJSON() error = %v, want nil", gotMarshalErr)
			}
			if len(encoded) > UploadCapabilityJSONMaximumBytes {
				t.Fatalf("encoded extent = %d, want at most %d",
					len(encoded), UploadCapabilityJSONMaximumBytes)
			}
			encodedURL, gotURLMarshalErr := core.MarshalCanonicalJSONString(tc.rawURL)
			if gotURLMarshalErr != nil {
				t.Fatalf("core.MarshalCanonicalJSONString(raw URL) error = %v, want nil", gotURLMarshalErr)
			}
			wantDocument := `{"provider":"` + tc.provider.String() +
				`","method":"` + tc.wantMethod + `","url":` + string(encodedURL) +
				`,"expires_at":1893456000000000000,"headers":[]}`
			if string(encoded) != wantDocument {
				t.Fatalf("UploadCapabilityProjection.MarshalJSON() = %q, want %q", encoded, wantDocument)
			}
			if tc.provider != ProviderCloudflareImages && !strings.Contains(string(encoded), `\u0026`) {
				t.Fatalf("UploadCapabilityProjection.MarshalJSON() = %q, want canonical query-separator escape", encoded)
			}

			var received UploadCapability
			if gotDecodeErr := json.Unmarshal(encoded, &received); gotDecodeErr != nil {
				t.Fatalf("json.Unmarshal() error = %v, want nil", gotDecodeErr)
			}
			gotProvider, gotProviderErr := received.Provider()
			if gotProviderErr != nil {
				t.Fatalf("received.Provider() error = %v, want nil", gotProviderErr)
			}
			if gotProvider != tc.provider {
				t.Fatalf("received.Provider() = %v, want %v", gotProvider, tc.provider)
			}
			gotTarget, gotTargetErr := received.Target()
			if gotTargetErr != nil {
				t.Fatalf("received.Target() error = %v, want nil", gotTargetErr)
			}
			if gotTarget.URL.value.String() != tc.rawURL {
				t.Fatalf("received target URL = %q, want %q",
					gotTarget.URL.value.String(), tc.rawURL)
			}
			gotExpiry, gotExpiryErr := gotTarget.ExpiresAt.Nanoseconds()
			if gotExpiryErr != nil {
				t.Fatalf("received target expiry error = %v, want nil", gotExpiryErr)
			}
			if gotExpiry != 1893456000000000000 {
				t.Fatalf("received target expiry = %d, want %d",
					gotExpiry, int64(1893456000000000000))
			}
			if len(gotTarget.Headers.values) != 0 {
				t.Fatalf("received target header count = %d, want 0",
					len(gotTarget.Headers.values))
			}
		})
	}
}

// TestUploadCapabilityProjectionPreservesSignedHeadersExactly proves the
// projection carries the issuer-signed header name, value, and order rather
// than reconstructing them from a second convention.
func TestUploadCapabilityProjectionPreservesSignedHeadersExactly(t *testing.T) {
	t.Parallel()

	signedURL, gotURLErr := ParseSignedURL(capabilityGCSMetadataRunURL)
	if gotURLErr != nil {
		t.Fatalf("ParseSignedURL() error = %v, want nil", gotURLErr)
	}
	headerName, gotNameErr := core.ParseHTTPHeaderName("X-Goog-Meta-Run")
	if gotNameErr != nil {
		t.Fatalf("ParseHTTPHeaderName() error = %v, want nil", gotNameErr)
	}
	const headerValue = "run-<&>-41"
	header, gotHeaderErr := NewSignedHeader(headerName, headerValue)
	if gotHeaderErr != nil {
		t.Fatalf("NewSignedHeader() error = %v, want nil", gotHeaderErr)
	}
	headers, gotHeadersErr := NewSignedHeaders([]SignedHeader{header})
	if gotHeadersErr != nil {
		t.Fatalf("NewSignedHeaders() error = %v, want nil", gotHeadersErr)
	}
	target := UploadTarget{
		Headers:   headers,
		URL:       signedURL,
		ExpiresAt: temporal.InstantFromNanoseconds(1893456000000000000),
	}
	projection, gotProjectionErr := NewUploadCapabilityProjection(
		ProviderGoogleCloudStorage, target,
	)
	if gotProjectionErr != nil {
		t.Fatalf("NewUploadCapabilityProjection() error = %v, want nil", gotProjectionErr)
	}
	encoded, gotMarshalErr := projection.MarshalJSON()
	if gotMarshalErr != nil {
		t.Fatalf("UploadCapabilityProjection.MarshalJSON() error = %v, want nil", gotMarshalErr)
	}
	encodedURL, gotURLMarshalErr := core.MarshalCanonicalJSONString(capabilityGCSMetadataRunURL)
	if gotURLMarshalErr != nil {
		t.Fatalf("core.MarshalCanonicalJSONString(raw URL) error = %v, want nil", gotURLMarshalErr)
	}
	encodedHeaderValue, gotHeaderValueMarshalErr := core.MarshalCanonicalJSONString(headerValue)
	if gotHeaderValueMarshalErr != nil {
		t.Fatalf("core.MarshalCanonicalJSONString(header value) error = %v, want nil",
			gotHeaderValueMarshalErr)
	}
	wantDocument := `{"provider":"google_cloud_storage","method":"signed_put","url":` +
		string(encodedURL) + `,"expires_at":1893456000000000000,` +
		`"headers":[{"name":"X-Goog-Meta-Run","value":` + string(encodedHeaderValue) + `}]}`
	if string(encoded) != wantDocument {
		t.Fatalf("MarshalJSON() = %q, want %q", encoded, wantDocument)
	}

	var received UploadCapability
	if gotDecodeErr := received.UnmarshalJSON(encoded); gotDecodeErr != nil {
		t.Fatalf("UploadCapability.UnmarshalJSON() error = %v, want nil", gotDecodeErr)
	}
	receivedTarget, gotTargetErr := received.Target()
	if gotTargetErr != nil {
		t.Fatalf("received.Target() error = %v, want nil", gotTargetErr)
	}
	if len(receivedTarget.Headers.values) != 1 {
		t.Fatalf("received header count = %d, want 1", len(receivedTarget.Headers.values))
	}
	if receivedTarget.Headers.values[0].name != headerName {
		t.Fatalf("received header name = %v, want %v",
			receivedTarget.Headers.values[0].name, headerName)
	}
	if receivedTarget.Headers.values[0].value != headerValue {
		t.Fatalf("received header value = %q, want %q",
			receivedTarget.Headers.values[0].value, headerValue)
	}
}

// TestUploadCapabilityProjectionRejectsInvalidConstruction pressures the
// constructor at its exact ownership boundaries. Every rejection returns the
// neutral zero projection, so no invalid bearer document can be emitted later.
func TestUploadCapabilityProjectionRejectsInvalidConstruction(t *testing.T) {
	t.Parallel()

	signedURL, gotURLErr := ParseSignedURL(capabilityGCSURL)
	if gotURLErr != nil {
		t.Fatalf("ParseSignedURL() error = %v, want nil", gotURLErr)
	}
	headers, gotHeadersErr := NewSignedHeaders(nil)
	if gotHeadersErr != nil {
		t.Fatalf("NewSignedHeaders(nil) error = %v, want nil", gotHeadersErr)
	}
	validTarget := UploadTarget{
		Headers:   headers,
		URL:       signedURL,
		ExpiresAt: temporal.InstantFromNanoseconds(1893456000000000000),
	}
	headerName, gotNameErr := core.ParseHTTPHeaderName("X-Goog-Meta-Run")
	if gotNameErr != nil {
		t.Fatalf("ParseHTTPHeaderName() error = %v, want nil", gotNameErr)
	}
	invalidUTF8Header, gotHeaderErr := NewSignedHeader(headerName, string([]byte{0xff}))
	if gotHeaderErr != nil {
		t.Fatalf("NewSignedHeader(invalid UTF-8 HTTP bytes) error = %v, want nil", gotHeaderErr)
	}
	invalidUTF8Headers, gotInvalidHeadersErr := NewSignedHeaders([]SignedHeader{invalidUTF8Header})
	if gotInvalidHeadersErr != nil {
		t.Fatalf("NewSignedHeaders(invalid UTF-8 HTTP bytes) error = %v, want nil", gotInvalidHeadersErr)
	}
	invalidUTF8Target := validTarget
	invalidUTF8Target.Headers = invalidUTF8Headers

	cases := []struct {
		name     string
		target   UploadTarget
		provider Provider
	}{
		{name: "zero provider is rejected", provider: ProviderUnknown, target: validTarget},
		{name: "future provider is rejected", provider: providerLimit, target: validTarget},
		{name: "zero target is rejected", provider: ProviderGoogleCloudStorage},
		{
			name:     "provider and signed URL vendor mismatch is rejected",
			provider: ProviderAmazonS3,
			target:   validTarget,
		},
		{
			name:     "signed header bytes that JSON cannot preserve are rejected",
			provider: ProviderGoogleCloudStorage,
			target:   invalidUTF8Target,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := NewUploadCapabilityProjection(tc.provider, tc.target)
			if !errors.Is(gotErr, core.ErrObjectStoreContract) {
				t.Fatalf("NewUploadCapabilityProjection() error = %v, want %v",
					gotErr, core.ErrObjectStoreContract)
			}
			if !got.IsZero() {
				t.Fatalf("rejected projection IsZero() = false, want true")
			}
		})
	}
}

// TestUploadCapabilityProjectionBoundsItsEmittedURL proves the exact URL
// boundary and its adjacent hostile value before JSON is allocated.
func TestUploadCapabilityProjectionBoundsItsEmittedURL(t *testing.T) {
	t.Parallel()

	base := capabilityGCSObject + "?" + queryGCSSignedHeaders +
		"=host%3Bx-goog-hash%3Bx-goog-if-generation-match&" +
		queryGCSSignature + "="
	headers, gotHeadersErr := NewSignedHeaders(nil)
	if gotHeadersErr != nil {
		t.Fatalf("NewSignedHeaders(nil) error = %v, want nil", gotHeadersErr)
	}
	cases := []struct {
		name       string
		extent     int
		wantReject bool
	}{
		{
			name:   "url one byte below the bound is admitted",
			extent: UploadCapabilityURLMaximumBytes - 1,
		},
		{name: "url exactly at the bound is admitted", extent: UploadCapabilityURLMaximumBytes},
		{
			name:       "url one byte above the bound is rejected",
			extent:     UploadCapabilityURLMaximumBytes + 1,
			wantReject: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rawURL := base + strings.Repeat("a", tc.extent-len(base))
			signedURL, gotURLErr := ParseSignedURL(rawURL)
			if gotURLErr != nil {
				t.Fatalf("ParseSignedURL() error = %v, want nil", gotURLErr)
			}
			target := UploadTarget{
				Headers:   headers,
				URL:       signedURL,
				ExpiresAt: temporal.InstantFromNanoseconds(1893456000000000000),
			}
			got, gotProjectionErr := NewUploadCapabilityProjection(
				ProviderGoogleCloudStorage, target,
			)
			if tc.wantReject {
				if !errors.Is(gotProjectionErr, core.ErrObjectStoreContract) {
					t.Fatalf("NewUploadCapabilityProjection() error = %v, want %v",
						gotProjectionErr, core.ErrObjectStoreContract)
				}
				if !got.IsZero() {
					t.Fatalf("rejected projection IsZero() = false, want true")
				}
				return
			}
			if gotProjectionErr != nil {
				t.Fatalf("NewUploadCapabilityProjection() error = %v, want nil",
					gotProjectionErr)
			}
			encoded, gotMarshalErr := got.MarshalJSON()
			if gotMarshalErr != nil {
				t.Fatalf("UploadCapabilityProjection.MarshalJSON() error = %v, want nil",
					gotMarshalErr)
			}
			if len(encoded) > UploadCapabilityJSONMaximumBytes {
				t.Fatalf("encoded extent = %d, want at most %d",
					len(encoded), UploadCapabilityJSONMaximumBytes)
			}
		})
	}
}

// TestUploadCapabilityProjectionIsAnEmbeddedJSONFixedPoint proves direct
// canonical output, encoding/json's custom-marshaler path, and a real enclosing
// protocol structure carry identical capability bytes. The fixture combines a
// maximum URL with a large signed header value and forces HTML-sensitive bytes
// through both independently bounded fields.
func TestUploadCapabilityProjectionIsAnEmbeddedJSONFixedPoint(t *testing.T) {
	t.Parallel()

	base := capabilityGCSMetadataRunURL
	rawURL := base + strings.Repeat("&", UploadCapabilityURLMaximumBytes-len(base))
	signedURL, gotURLErr := ParseSignedURL(rawURL)
	if gotURLErr != nil {
		t.Fatalf("ParseSignedURL() error = %v, want nil", gotURLErr)
	}
	headerName, gotNameErr := core.ParseHTTPHeaderName("X-Goog-Meta-Run")
	if gotNameErr != nil {
		t.Fatalf("ParseHTTPHeaderName() error = %v, want nil", gotNameErr)
	}
	headerValue := strings.Repeat("&", 8_000)
	header, gotHeaderErr := NewSignedHeader(headerName, headerValue)
	if gotHeaderErr != nil {
		t.Fatalf("NewSignedHeader() error = %v, want nil", gotHeaderErr)
	}
	headers, gotHeadersErr := NewSignedHeaders([]SignedHeader{header})
	if gotHeadersErr != nil {
		t.Fatalf("NewSignedHeaders() error = %v, want nil", gotHeadersErr)
	}
	projection, gotProjectionErr := NewUploadCapabilityProjection(
		ProviderGoogleCloudStorage,
		UploadTarget{
			Headers:   headers,
			URL:       signedURL,
			ExpiresAt: temporal.InstantFromNanoseconds(1893456000000000000),
		},
	)
	if gotProjectionErr != nil {
		t.Fatalf("NewUploadCapabilityProjection() error = %v, want nil", gotProjectionErr)
	}

	direct, gotDirectErr := projection.MarshalJSON()
	if gotDirectErr != nil {
		t.Fatalf("UploadCapabilityProjection.MarshalJSON() error = %v, want nil", gotDirectErr)
	}
	if len(direct) > UploadCapabilityJSONMaximumBytes {
		t.Fatalf("UploadCapabilityProjection.MarshalJSON() extent = %d, want at most %d",
			len(direct), UploadCapabilityJSONMaximumBytes)
	}
	if !strings.Contains(string(direct), `\u0026`) {
		t.Fatalf("UploadCapabilityProjection.MarshalJSON() did not exercise canonical HTML escaping")
	}

	throughMarshaler, gotMarshalErr := json.Marshal(projection)
	if gotMarshalErr != nil {
		t.Fatalf("json.Marshal(UploadCapabilityProjection) error = %v, want nil", gotMarshalErr)
	}
	if len(throughMarshaler) > UploadCapabilityJSONMaximumBytes {
		t.Fatalf("json.Marshal(UploadCapabilityProjection) extent = %d, want at most %d",
			len(throughMarshaler), UploadCapabilityJSONMaximumBytes)
	}
	if string(throughMarshaler) != string(direct) {
		t.Fatalf("json.Marshal(UploadCapabilityProjection) extent = %d, want direct fixed-point extent %d",
			len(throughMarshaler), len(direct))
	}

	envelope, gotEnvelopeErr := json.Marshal(struct {
		Capability UploadCapabilityProjection `json:"capability"`
	}{Capability: projection})
	if gotEnvelopeErr != nil {
		t.Fatalf("json.Marshal(capability envelope) error = %v, want nil", gotEnvelopeErr)
	}
	wantEnvelope := `{"capability":` + string(direct) + `}`
	if string(envelope) != wantEnvelope {
		t.Fatalf("json.Marshal(capability envelope) extent = %d, want fixed-point extent %d",
			len(envelope), len(wantEnvelope))
	}

	var received struct {
		Capability UploadCapability `json:"capability"`
	}
	if gotDecodeErr := json.Unmarshal(envelope, &received); gotDecodeErr != nil {
		t.Fatalf("json.Unmarshal(capability envelope) error = %v, want nil", gotDecodeErr)
	}
	receivedTarget, gotTargetErr := received.Capability.Target()
	if gotTargetErr != nil {
		t.Fatalf("received.Capability.Target() error = %v, want nil", gotTargetErr)
	}
	if receivedTarget.URL.value.String() != rawURL {
		t.Fatalf("received target URL extent = %d, want %d exact bytes",
			len(receivedTarget.URL.value.String()), len(rawURL))
	}
	if len(receivedTarget.Headers.values) != 1 {
		t.Fatalf("received target header count = %d, want 1", len(receivedTarget.Headers.values))
	}
	if receivedTarget.Headers.values[0].name != headerName ||
		receivedTarget.Headers.values[0].value != headerValue {
		t.Fatalf("received signed header = (%v, %d bytes), want (%v, %d bytes)",
			receivedTarget.Headers.values[0].name,
			len(receivedTarget.Headers.values[0].value),
			headerName,
			len(headerValue))
	}
}

// TestUploadCapabilityProjectionIsEncodeOnlyAndRedacted is the structural and
// disclosure ratchet for the issuer side of the nominal boundary.
func TestUploadCapabilityProjectionIsEncodeOnlyAndRedacted(t *testing.T) {
	t.Parallel()

	projection := any(UploadCapabilityProjection{})
	if _, ok := projection.(json.Marshaler); !ok {
		t.Fatalf("UploadCapabilityProjection does not implement json.Marshaler, want an encoder")
	}
	if _, ok := projection.(json.Unmarshaler); ok {
		t.Fatalf("UploadCapabilityProjection implements json.Unmarshaler, want no decoder")
	}
	if _, ok := any(&UploadCapabilityProjection{}).(json.Unmarshaler); ok {
		t.Fatalf("*UploadCapabilityProjection implements json.Unmarshaler, want no decoder")
	}
	if _, ok := projection.(fmt.Stringer); ok {
		t.Fatalf("UploadCapabilityProjection implements fmt.Stringer, want no string accessor")
	}
	if _, ok := projection.(encoding.TextMarshaler); ok {
		t.Fatalf("UploadCapabilityProjection implements encoding.TextMarshaler, want no text accessor")
	}

	rendered := fmt.Sprintf("%v|%+v|%#v|%s|%q", projection, projection, projection, projection, projection)
	if strings.Contains(rendered, capabilityGCSURL) {
		t.Fatalf("formatted projection disclosed its signed URL, want redaction")
	}
	wantRendered := strings.Join([]string{
		core.RedactedValueText,
		core.RedactedValueText,
		core.RedactedValueText,
		core.RedactedValueText,
		core.RedactedValueText,
	}, "|")
	if rendered != wantRendered {
		t.Fatalf("formatted projection = %q, want %q", rendered, wantRendered)
	}
}

// TestUploadCapabilityProjectionZeroValueRefusesOutput proves the neutral
// value cannot emit a syntactically plausible bearer document.
func TestUploadCapabilityProjectionZeroValueRefusesOutput(t *testing.T) {
	t.Parallel()

	var projection UploadCapabilityProjection
	if !projection.IsZero() {
		t.Fatalf("zero UploadCapabilityProjection.IsZero() = false, want true")
	}
	if gotValidateErr := projection.Validate(); !errors.Is(gotValidateErr, core.ErrObjectStoreContract) {
		t.Fatalf("zero UploadCapabilityProjection.Validate() error = %v, want %v",
			gotValidateErr, core.ErrObjectStoreContract)
	}
	encoded, gotMarshalErr := projection.MarshalJSON()
	if !errors.Is(gotMarshalErr, core.ErrObjectStoreContract) {
		t.Fatalf("zero UploadCapabilityProjection.MarshalJSON() error = %v, want %v",
			gotMarshalErr, core.ErrObjectStoreContract)
	}
	if encoded != nil {
		t.Fatalf("zero UploadCapabilityProjection.MarshalJSON() bytes = %q, want nil", encoded)
	}
}
