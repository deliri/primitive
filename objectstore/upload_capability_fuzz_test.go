package objectstore

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// FuzzUploadCapabilityAdmitsOnlyTransferableCapabilities drives arbitrary
// documents through the real decode boundary. The oracle is not "did not
// panic": every rejection must carry this package's stable identity and leave
// the receiver unset, and every acceptance must project onto a target the
// transfer entry point would itself admit, while no formatting verb may ever
// render the bearer the document carried.
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
		var capability UploadCapability
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
			if !capability.IsZero() {
				t.Fatalf("receiver after a rejected UnmarshalJSON(%q) is set, want unset", document)
			}
			if _, providerErr := capability.Provider(); providerErr == nil {
				t.Fatalf("rejected capability Provider() error = nil, want a rejection for %q", document)
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
		// An accepted capability must satisfy the same target contract the
		// transfer entry point applies, or decoding would admit a capability
		// that can never be spent.
		if err := target.Validate(); err != nil {
			t.Fatalf("projected target Validate() error = %v, want nil for %q", err, document)
		}
		if rendered := fmt.Sprintf("%v%+v%#v", target.URL, target.URL, target.URL); strings.Count(
			rendered, core.RedactedValueText) != 3 {
			t.Fatalf("formatted signed url = %q, want only redacted text", rendered)
		}
	})
}
