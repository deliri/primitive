package objectstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// FuzzUploadCapabilityAdmitsOnlyTransferableCapabilities drives arbitrary
// documents through the real decode boundary. The oracle is not "did not
// panic": every rejection must carry this package's stable identity and leave
// a populated receiver unchanged. Every acceptance must cross the nominal
// issuer projection, decode again with every typed target fact preserved, and
// produce byte-stable canonical output. No formatting verb may ever render the
// bearer the document carried.
func FuzzUploadCapabilityAdmitsOnlyTransferableCapabilities(f *testing.F) {
	f.Add(`{"provider":"google_cloud_storage","method":"signed_put","url":"` +
		capabilityGCSURL + `","expires_at":1893456000000000000}`)
	f.Add(`{"provider":"amazon_s3","method":"signed_put","url":"` +
		capabilityS3URL + `","expires_at":1893456000000000000}`)
	f.Add(`{"provider":"cloudflare_images","method":"multipart_post","url":"` +
		capabilityImagesURL + `","expires_at":1893456000000000000}`)
	f.Add(`{"provider":"google_cloud_storage","method":"signed_put","url":"` +
		capabilityGCSURL + `","expires_at":1893456000000000000,` +
		`"headers":[{"name":"X-Goog-Meta-Run","value":"run-41"}]}`)
	f.Add(`{"provider":"google_cloud_storage","method":"signed_put","url":"` +
		capabilityGCSMetadataRunURL + `","expires_at":1893456000000000000,` +
		`"headers":[{"name":"X-Goog-Meta-Run","value":"run-41"}]}`)
	f.Add(`{"provider":"google_cloud_storage","method":"signed_put","url":"` +
		capabilityGCSMetadataRunURL + `","expires_at":1893456000000000000,` +
		`"headers":[{"name":"X-Goog-Meta-Run","value":"run-<&>-41"}]}`)
	f.Add(`{"provider":"google_cloud_storage","method":"multipart_post","url":"` +
		capabilityGCSURL + `","expires_at":1893456000000000000}`)
	f.Add(`{"provider":"azure_blob","method":"signed_put","url":"` +
		capabilityGCSURL + `","expires_at":1893456000000000000}`)
	f.Add(`{"provider":"cloudflare_images","method":"multipart_post","url":"` +
		capabilityImagesURL + `","expires_at":1893456000000000000,` +
		`"headers":[{"name":"X-Meta-Run","value":"v"}]}`)
	f.Add(`{"provider":"google_cloud_storage","method":"signed_put","url":"` +
		capabilityGCSURL + `","expires_at":"1893456000000000000"}`)
	f.Add(`{}`)
	f.Add(`null`)
	f.Add(``)

	f.Fuzz(func(t *testing.T, document string) {
		capability, fixtureErr := capabilityDocument(t,
			`{"provider":"google_cloud_storage","method":"signed_put","url":"`+
				capabilityGCSURL+`","expires_at":1893456000000000000}`)
		if fixtureErr != nil {
			t.Fatalf("valid receiver fixture decode error = %v, want nil", fixtureErr)
		}
		err := capability.UnmarshalJSON([]byte(document))

		if rendered := fmt.Sprintf("%v|%+v|%#v|%s|%q", capability, capability,
			capability, capability, capability); strings.Count(
			rendered, core.RedactedValueText) != 5 {
			t.Fatalf("formatted capability = %q, want only redacted text", rendered)
		}

		if err != nil {
			if !errors.Is(err, core.ErrObjectStoreContract) {
				t.Fatalf("UnmarshalJSON(%q) error = %v, want %v",
					document, err, core.ErrObjectStoreContract)
			}
			provider, providerErr := capability.Provider()
			if providerErr != nil || provider != ProviderGoogleCloudStorage {
				t.Fatalf("receiver Provider() after rejected UnmarshalJSON(%q) = (%v, %v), want (%v, nil)",
					document, provider, providerErr, ProviderGoogleCloudStorage)
			}
			target, targetErr := capability.Target()
			if targetErr != nil {
				t.Fatalf("receiver Target() after rejected UnmarshalJSON(%q) error = %v, want nil",
					document, targetErr)
			}
			if target.URL.value.String() != capabilityGCSURL {
				t.Fatalf("receiver URL after rejected UnmarshalJSON(%q) = %q, want %q",
					document, target.URL.value.String(), capabilityGCSURL)
			}
			return
		}

		if capability.IsZero() {
			t.Fatalf("accepted UnmarshalJSON(%q) left the receiver unset", document)
		}
		if validateErr := capability.Validate(); validateErr != nil {
			t.Fatalf("accepted capability Validate() error = %v, want nil for %q",
				validateErr, document)
		}
		provider, providerErr := capability.Provider()
		if providerErr != nil || provider.Validate() != nil {
			t.Fatalf("accepted capability Provider() = (%v, %v), want a closed provider for %q",
				provider, providerErr, document)
		}
		target, targetErr := capability.Target()
		if targetErr != nil {
			t.Fatalf("accepted capability Target() error = %v, want nil for %q", targetErr, document)
		}
		projection, projectionErr := NewUploadCapabilityProjection(provider, target)
		if projectionErr != nil {
			t.Fatalf("NewUploadCapabilityProjection() error = %v, want nil for accepted %q",
				projectionErr, document)
		}
		direct, directErr := projection.MarshalJSON()
		if directErr != nil {
			t.Fatalf("UploadCapabilityProjection.MarshalJSON() error = %v, want nil for accepted %q",
				directErr, document)
		}
		canonical, canonicalErr := json.Marshal(projection)
		if canonicalErr != nil {
			t.Fatalf("json.Marshal(UploadCapabilityProjection) error = %v, want nil for accepted %q",
				canonicalErr, document)
		}
		if string(canonical) != string(direct) {
			t.Fatalf("json.Marshal(UploadCapabilityProjection) = %q, want direct fixed point %q for accepted %q",
				canonical, direct, document)
		}
		if len(canonical) > UploadCapabilityJSONMaximumBytes {
			t.Fatalf("canonical extent = %d, want at most %d for accepted %q",
				len(canonical), UploadCapabilityJSONMaximumBytes, document)
		}

		var roundTrip UploadCapability
		if roundTripErr := roundTrip.UnmarshalJSON(canonical); roundTripErr != nil {
			t.Fatalf("canonical UploadCapability.UnmarshalJSON() error = %v, want nil for accepted %q",
				roundTripErr, document)
		}
		roundTripProvider, roundTripProviderErr := roundTrip.Provider()
		if roundTripProviderErr != nil || roundTripProvider != provider {
			t.Fatalf("canonical Provider() = (%v, %v), want (%v, nil) for accepted %q",
				roundTripProvider, roundTripProviderErr, provider, document)
		}
		roundTripTarget, roundTripTargetErr := roundTrip.Target()
		if roundTripTargetErr != nil {
			t.Fatalf("canonical Target() error = %v, want nil for accepted %q",
				roundTripTargetErr, document)
		}
		if roundTripTarget.URL.value.String() != target.URL.value.String() {
			t.Fatalf("canonical target URL = %q, want %q for accepted %q",
				roundTripTarget.URL.value.String(), target.URL.value.String(), document)
		}
		expiresAt, expiresAtErr := target.ExpiresAt.Nanoseconds()
		if expiresAtErr != nil {
			t.Fatalf("accepted target expiry error = %v, want nil for %q", expiresAtErr, document)
		}
		roundTripExpiresAt, roundTripExpiresAtErr := roundTripTarget.ExpiresAt.Nanoseconds()
		if roundTripExpiresAtErr != nil || roundTripExpiresAt != expiresAt {
			t.Fatalf("canonical target expiry = (%d, %v), want (%d, nil) for accepted %q",
				roundTripExpiresAt, roundTripExpiresAtErr, expiresAt, document)
		}
		if len(roundTripTarget.Headers.values) != len(target.Headers.values) {
			t.Fatalf("canonical header count = %d, want %d for accepted %q",
				len(roundTripTarget.Headers.values), len(target.Headers.values), document)
		}
		for index, header := range target.Headers.values {
			if roundTripTarget.Headers.values[index] != header {
				t.Fatalf("canonical header %d = %+v, want %+v for accepted %q",
					index, roundTripTarget.Headers.values[index], header, document)
			}
		}
		secondProjection, secondProjectionErr := NewUploadCapabilityProjection(
			roundTripProvider, roundTripTarget,
		)
		if secondProjectionErr != nil {
			t.Fatalf("canonical NewUploadCapabilityProjection() error = %v, want nil for accepted %q",
				secondProjectionErr, document)
		}
		secondCanonical, secondCanonicalErr := secondProjection.MarshalJSON()
		if secondCanonicalErr != nil {
			t.Fatalf("canonical second MarshalJSON() error = %v, want nil for accepted %q",
				secondCanonicalErr, document)
		}
		if string(secondCanonical) != string(canonical) {
			t.Fatalf("canonical second MarshalJSON() = %q, want %q for accepted %q",
				secondCanonical, canonical, document)
		}
		if rendered := fmt.Sprintf("%v%+v%#v", target.URL, target.URL, target.URL); strings.Count(
			rendered, core.RedactedValueText) != 3 {
			t.Fatalf("formatted signed url = %q, want only redacted text", rendered)
		}
	})
}
