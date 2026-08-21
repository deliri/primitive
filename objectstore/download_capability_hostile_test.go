package objectstore

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

type downloadCapabilityFixtureRequest struct {
	Provider Provider
	Extent   int
	Expiry   temporal.Instant
}

type downloadCapabilityJSONCase struct {
	name string
	data []byte
}

func TestDownloadCapabilityProjectionLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive provider expiry and exact URL boundaries close one bearer", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name    string
			request downloadCapabilityFixtureRequest
		}{
			{name: "amazon ordinary bearer", request: downloadCapabilityFixtureRequest{Provider: ProviderAmazonS3}},
			{name: "google ordinary bearer", request: downloadCapabilityFixtureRequest{Provider: ProviderGoogleCloudStorage}},
			{name: "amazon pre-epoch expiry", request: downloadCapabilityFixtureRequest{Provider: ProviderAmazonS3, Expiry: temporal.InstantFromNanoseconds(-1)}},
			{name: "google epoch expiry", request: downloadCapabilityFixtureRequest{Provider: ProviderGoogleCloudStorage, Expiry: temporal.InstantFromNanoseconds(0)}},
			{name: "amazon one-nanosecond expiry", request: downloadCapabilityFixtureRequest{Provider: ProviderAmazonS3, Expiry: temporal.InstantFromNanoseconds(1)}},
			{name: "google one below URL maximum", request: downloadCapabilityFixtureRequest{Provider: ProviderGoogleCloudStorage, Extent: CapabilityURLMaximumBytes - 1}},
			{name: "google at URL maximum", request: downloadCapabilityFixtureRequest{Provider: ProviderGoogleCloudStorage, Extent: CapabilityURLMaximumBytes}},
			{name: "amazon one below URL maximum", request: downloadCapabilityFixtureRequest{Provider: ProviderAmazonS3, Extent: CapabilityURLMaximumBytes - 1}},
			{name: "amazon at URL maximum", request: downloadCapabilityFixtureRequest{Provider: ProviderAmazonS3, Extent: CapabilityURLMaximumBytes}},
			{name: "google maximum positive instant", request: downloadCapabilityFixtureRequest{Provider: ProviderGoogleCloudStorage, Expiry: temporal.InstantFromNanoseconds(int64(^uint64(0) >> 1))}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				projection := downloadCapabilityProjectionRequestFixture(t, tc.request)
				encoded, marshalErr := projection.MarshalJSON()
				if marshalErr != nil || len(encoded) > CapabilityJSONMaximumBytes {
					t.Fatalf("DownloadCapabilityProjection.MarshalJSON() = (%d bytes, %v), want bounded document and nil", len(encoded), marshalErr)
				}
				var received DownloadCapability
				decodeErr := received.UnmarshalJSON(encoded)
				if decodeErr != nil || received.Validate() != nil || !sameDownloadCapabilityProjection(received, projection) {
					t.Fatalf("DownloadCapability.UnmarshalJSON() = (%v, %v), want exact projected bearer", received, decodeErr)
				}
			})
		}
	})

	t.Run("negative provider target extent and signed-header mismatches close no bearer", func(t *testing.T) {
		t.Parallel()

		google := downloadTargetFixture(t, downloadCapabilityFixtureRequest{Provider: ProviderGoogleCloudStorage})
		amazon := downloadTargetFixture(t, downloadCapabilityFixtureRequest{Provider: ProviderAmazonS3})
		cloudflare := DownloadTarget{
			URL:       providerSignedURL(t, ProviderCloudflareImages, DirectionDownload),
			ExpiresAt: providerFutureInstant(t),
		}
		oversizeGoogle := downloadTargetFixture(t, downloadCapabilityFixtureRequest{
			Provider: ProviderGoogleCloudStorage, Extent: CapabilityURLMaximumBytes + 1,
		})
		invalidHeaders := google
		invalidHeaders.Headers = SignedHeaders{values: []SignedHeader{{}}}
		cases := []struct {
			name     string
			target   DownloadTarget
			provider Provider
		}{
			{name: "zero provider", provider: ProviderUnknown, target: google},
			{name: "future provider", provider: providerLimit, target: google},
			{name: "zero target", provider: ProviderGoogleCloudStorage},
			{name: "google provider with amazon URL", provider: ProviderGoogleCloudStorage, target: amazon},
			{name: "amazon provider with google URL", provider: ProviderAmazonS3, target: google},
			{name: "cloudflare has no download capability", provider: ProviderCloudflareImages, target: cloudflare},
			{name: "expiry absent", provider: ProviderGoogleCloudStorage, target: DownloadTarget{URL: google.URL, Headers: google.Headers}},
			{name: "URL absent", provider: ProviderGoogleCloudStorage, target: DownloadTarget{ExpiresAt: google.ExpiresAt, Headers: google.Headers}},
			{name: "URL one above maximum", provider: ProviderGoogleCloudStorage, target: oversizeGoogle},
			{name: "invalid signed header set", provider: ProviderGoogleCloudStorage, target: invalidHeaders},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				got, gotErr := NewDownloadCapabilityProjection(tc.provider, tc.target)
				if !errors.Is(gotErr, core.ErrObjectStoreContract) || !got.IsZero() {
					t.Fatalf("NewDownloadCapabilityProjection() = (%v, %v), want zero and errors.Is %v", got, gotErr, core.ErrObjectStoreContract)
				}
			})
		}
	})

	t.Run("neutral zero projection discloses no bearer or commitment", func(t *testing.T) {
		t.Parallel()

		projection := DownloadCapabilityProjection{}
		encoded, marshalErr := projection.MarshalJSON()
		commitment, commitmentErr := projection.Commitment()
		if !errors.Is(marshalErr, core.ErrObjectStoreContract) || encoded != nil ||
			!errors.Is(commitmentErr, core.ErrObjectStoreContract) || commitment != (DownloadCapabilityCommitment{}) {
			t.Fatalf("zero projection disclosure = (%q, %v, %v, %v), want no bearer or commitment", encoded, marshalErr, commitment, commitmentErr)
		}
	})
}

func TestDownloadCapabilityDocumentJSONLayerTriad(t *testing.T) {
	t.Parallel()

	google := downloadCapabilityProjectionFixture(t, ProviderGoogleCloudStorage)
	googleJSON, gotErr := google.MarshalJSON()
	if gotErr != nil {
		t.Fatalf("DownloadCapabilityProjection.MarshalJSON(GCS) error = %v, want nil", gotErr)
	}
	amazon := downloadCapabilityProjectionFixture(t, ProviderAmazonS3)
	amazonJSON, gotErr := amazon.MarshalJSON()
	if gotErr != nil {
		t.Fatalf("DownloadCapabilityProjection.MarshalJSON(S3) error = %v, want nil", gotErr)
	}
	var before DownloadCapability
	if gotErr := before.UnmarshalJSON(googleJSON); gotErr != nil {
		t.Fatalf("DownloadCapability.UnmarshalJSON(setup) error = %v, want nil", gotErr)
	}

	t.Run("positive provider order whitespace and exact document boundaries preserve bearer", func(t *testing.T) {
		t.Parallel()

		cases := []downloadCapabilityJSONCase{
			{name: "canonical google bearer", data: googleJSON},
			{name: "canonical amazon bearer", data: amazonJSON},
			{name: "leading whitespace", data: append([]byte(" \n\t"), googleJSON...)},
			{name: "trailing whitespace", data: append(append([]byte(nil), googleJSON...), ' ', '\n', '\t')},
			{name: "both-side whitespace", data: append(append([]byte(" \n"), googleJSON...), '\n', ' ')},
			{name: "members reordered", data: marshalReorderedDownloadCapability(t, googleJSON)},
			{name: "one below document ceiling", data: downloadCapabilityPadJSON(googleJSON, CapabilityJSONMaximumBytes-1)},
			{name: "at document ceiling", data: downloadCapabilityPadJSON(googleJSON, CapabilityJSONMaximumBytes)},
			{name: "one trailing carriage return", data: append(append([]byte(nil), googleJSON...), '\r')},
			{name: "four whitespace forms on both sides", data: append(append([]byte("\t\r\n "), googleJSON...), " \n\r\t"...)},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				var got DownloadCapability
				decodeErr := got.UnmarshalJSON(tc.data)
				if decodeErr != nil || got.Validate() != nil {
					t.Fatalf("DownloadCapability.UnmarshalJSON() = (%v, %v), want valid bearer and nil", got, decodeErr)
				}
			})
		}
	})

	t.Run("negative malformed missing duplicate type-wrong and oversized documents reject transactionally", func(t *testing.T) {
		t.Parallel()

		for _, tc := range downloadCapabilityHostileJSONCases(googleJSON) {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				got := before
				decodeErr := got.UnmarshalJSON(tc.data)
				if !errors.Is(decodeErr, core.ErrObjectStoreContract) || !sameDownloadCapability(got, before) {
					t.Fatalf("DownloadCapability.UnmarshalJSON() = (%v, %v), want preserved receiver and errors.Is %v", got, decodeErr, core.ErrObjectStoreContract)
				}
			})
		}
	})

	t.Run("neutral rejected input discloses no bearer", func(t *testing.T) {
		t.Parallel()

		var got DownloadCapability
		decodeErr := got.UnmarshalJSON(nil)
		if !errors.Is(decodeErr, core.ErrObjectStoreContract) || !got.IsZero() {
			t.Fatalf("zero DownloadCapability.UnmarshalJSON(nil) = (%v, %v), want zero and errors.Is %v", got, decodeErr, core.ErrObjectStoreContract)
		}
	})
}

func TestDownloadCapabilityAndProjectionRedactEveryFormattingPath(t *testing.T) {
	t.Parallel()

	projection := downloadCapabilityProjectionFixture(t, ProviderGoogleCloudStorage)
	encoded, err := projection.MarshalJSON()
	if err != nil {
		t.Fatalf("DownloadCapabilityProjection.MarshalJSON() error = %v, want nil", err)
	}
	var received DownloadCapability
	if err := received.UnmarshalJSON(encoded); err != nil {
		t.Fatalf("DownloadCapability.UnmarshalJSON() error = %v, want nil", err)
	}
	verbs := []string{"%v", "%+v", "%#v", "%s", "%q", "%x", "%X"}
	for _, verb := range verbs {
		if got := fmt.Sprintf(verb, projection); got != core.RedactedValueText {
			t.Fatalf("fmt.Sprintf(%q, projection) = %q, want %q", verb, got, core.RedactedValueText)
		}
		if got := fmt.Sprintf(verb, received); got != core.RedactedValueText {
			t.Fatalf("fmt.Sprintf(%q, received) = %q, want %q", verb, got, core.RedactedValueText)
		}
	}
}

func downloadCapabilityProjectionFixture(t testing.TB, provider Provider) DownloadCapabilityProjection {
	t.Helper()
	return downloadCapabilityProjectionRequestFixture(t, downloadCapabilityFixtureRequest{Provider: provider})
}

func downloadCapabilityProjectionRequestFixture(
	t testing.TB,
	request downloadCapabilityFixtureRequest,
) DownloadCapabilityProjection {
	t.Helper()

	target := downloadTargetFixture(t, request)
	projection, gotErr := NewDownloadCapabilityProjection(request.Provider, target)
	if gotErr != nil {
		t.Fatalf("NewDownloadCapabilityProjection(%v) error = %v, want nil", request.Provider, gotErr)
	}
	return projection
}

func downloadTargetFixture(t testing.TB, request downloadCapabilityFixtureRequest) DownloadTarget {
	t.Helper()

	if request.Provider == ProviderUnknown {
		request.Provider = ProviderGoogleCloudStorage
	}
	if request.Expiry == (temporal.Instant{}) {
		request.Expiry = providerFutureInstant(t)
	}
	signedURL := providerSignedURL(t, request.Provider, DirectionDownload)
	if request.Extent > 0 {
		base := signedURL.value.String() + "&padding="
		rawURL := base + strings.Repeat("a", request.Extent-len(base))
		var gotErr error
		signedURL, gotErr = ParseSignedURL(rawURL)
		if gotErr != nil {
			t.Fatalf("ParseSignedURL(%d-byte fixture) error = %v, want nil", request.Extent, gotErr)
		}
	}
	headers, gotErr := NewSignedHeaders(nil)
	if gotErr != nil {
		t.Fatalf("NewSignedHeaders(nil) error = %v, want nil", gotErr)
	}
	return DownloadTarget{URL: signedURL, Headers: headers, ExpiresAt: request.Expiry}
}

func sameDownloadCapabilityProjection(got DownloadCapability, want DownloadCapabilityProjection) bool {
	gotCommitment, gotErr := got.Commitment()
	wantCommitment, wantErr := want.Commitment()
	return gotErr == nil && wantErr == nil && gotCommitment == wantCommitment
}

func sameDownloadCapability(got, want DownloadCapability) bool {
	gotCommitment, gotErr := got.Commitment()
	wantCommitment, wantErr := want.Commitment()
	return gotErr == nil && wantErr == nil && gotCommitment == wantCommitment
}

func marshalReorderedDownloadCapability(t *testing.T, canonical []byte) []byte {
	t.Helper()

	var wire uploadCapabilityWire
	if gotErr := json.Unmarshal(canonical, &wire); gotErr != nil {
		t.Fatalf("json.Unmarshal(download wire) error = %v, want nil", gotErr)
	}
	encoded, gotErr := core.MarshalCanonicalJSONDocument(struct {
		ExpiresAt *temporal.NumericInstant     `json:"expires_at"`
		URL       *string                      `json:"url"`
		Method    *string                      `json:"method"`
		Provider  *string                      `json:"provider"`
		Headers   []uploadCapabilityHeaderWire `json:"headers"`
	}{Headers: wire.Headers, ExpiresAt: wire.ExpiresAt, URL: wire.URL, Method: wire.Method, Provider: wire.Provider})
	if gotErr != nil {
		t.Fatalf("core.MarshalCanonicalJSONDocument(reordered download) error = %v, want nil", gotErr)
	}
	return encoded
}

func downloadCapabilityPadJSON(document []byte, wantBytes int) []byte {
	if len(document) >= wantBytes {
		return append([]byte(nil), document...)
	}
	return append(append([]byte(nil), document...), bytes.Repeat([]byte{' '}, wantBytes-len(document))...)
}

func downloadCapabilityHostileJSONCases(canonical []byte) []downloadCapabilityJSONCase {
	return []downloadCapabilityJSONCase{
		{name: "empty document", data: nil},
		{name: "whitespace-only document", data: []byte(" \n\t")},
		{name: "null document", data: []byte("null")},
		{name: "array instead of structure", data: []byte("[]")},
		{name: "string instead of structure", data: []byte(`"download"`)},
		{name: "number instead of structure", data: []byte("1")},
		{name: "boolean instead of structure", data: []byte("true")},
		{name: "truncated opening brace", data: []byte("{")},
		{name: "truncated inside bearer", data: canonical[:len(canonical)/2]},
		{name: "truncated before final brace", data: canonical[:len(canonical)-1]},
		{name: "trailing object", data: append(append([]byte(nil), canonical...), '{', '}')},
		{name: "two concatenated documents", data: append(append([]byte(nil), canonical...), canonical...)},
		{name: "unknown top-level member", data: bytes.Replace(canonical, []byte(`{"provider"`), []byte(`{"unknown":1,"provider"`), 1)},
		{name: "duplicate provider member", data: bytes.Replace(canonical, []byte(`{"provider":`), []byte(`{"provider":null,"provider":`), 1)},
		{name: "missing every member", data: []byte("{}")},
		{name: "missing provider", data: bytes.Replace(canonical, canonical[:bytes.Index(canonical, []byte(`,"method"`))], []byte("{"), 1)},
		{name: "provider has wrong scalar type", data: bytes.Replace(canonical, []byte(`"provider":"google_cloud_storage"`), []byte(`"provider":1`), 1)},
		{name: "method has wrong scalar type", data: bytes.Replace(canonical, []byte(`"method":"signed_get"`), []byte(`"method":1`), 1)},
		{name: "URL has wrong scalar type", data: bytes.Replace(canonical, []byte(`"url":"`), []byte(`"url":1,"discarded":"`), 1)},
		{name: "expiry has wrong scalar type", data: bytes.Replace(canonical, []byte(`"expires_at":`), []byte(`"expires_at":"invalid","discarded":`), 1)},
		{name: "upload method substituted", data: bytes.Replace(canonical, []byte(DownloadMethodTokenSignedGet), []byte(UploadMethodTokenSignedPut), 1)},
		{name: "one above document ceiling", data: downloadCapabilityPadJSON(canonical, CapabilityJSONMaximumBytes+1)},
	}
}
