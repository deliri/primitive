package objectstore

import (
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// The base URLs declare exactly the headers Objectstore sends itself. The
	// metadata variants add caller-owned fields and are used only when the
	// capability document carries those exact fields.
	capabilityGCSObject = core.SchemeHTTPS + "://storage.googleapis.com/bucket/object"
	capabilityGCSURL    = capabilityGCSObject + "?" + queryGCSSignature + "=signature&" +
		queryGCSSignedHeaders + "=host%3Bx-goog-hash%3Bx-goog-if-generation-match"
	capabilityGCSMetadataRunURL = capabilityGCSObject + "?" + queryGCSSignature + "=signature&" +
		queryGCSSignedHeaders + "=host%3Bx-goog-hash%3Bx-goog-if-generation-match%3Bx-goog-meta-run"
	capabilityGCSMetadataURL = capabilityGCSObject + "?" + queryGCSSignature + "=signature&" +
		queryGCSSignedHeaders + "=host%3Bx-goog-hash%3Bx-goog-if-generation-match%3Bx-goog-meta-run%3Bx-goog-meta-shard"
	capabilityS3URL = core.SchemeHTTPS + "://s3.amazonaws.com/bucket/object" +
		"?" + queryS3Signature + "=signature&" + queryS3SignedHeaders + "=host%3Bif-none-match%3Bx-amz-checksum-crc32c"
	capabilityS3MetadataURL = core.SchemeHTTPS + "://s3.amazonaws.com/bucket/object" +
		"?" + queryS3Signature + "=signature&" + queryS3SignedHeaders + "=host%3Bif-none-match%3Bx-amz-checksum-crc32c%3Bx-amz-meta-run"
	capabilityImagesURL = core.SchemeHTTPS + "://" + cloudflareImagesUploadHost + "/image-id"

	// capabilitySecret is the value a rejection must never disclose. It is
	// placed in the query where a real signature lives.
	capabilitySecret = "TOPSECRETSIGNATUREMATERIAL"
)

func capabilityDocument(t *testing.T, document string) (UploadCapability, error) {
	t.Helper()

	var capability UploadCapability
	err := capability.UnmarshalJSON([]byte(document))
	return capability, err
}

// TestUploadCapabilityAdmitsOnlyPublishedVendorShapes pressures both sides of
// the received-capability contract: the vendor token, the method the vendor
// publishes, the signed URL grammar, and the signed-header lattice.
func TestUploadCapabilityAdmitsOnlyPublishedVendorShapes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		document string
		wantErr  bool
	}{
		{
			name: "google cloud storage signed put with no headers member",
			document: `{"provider":"google_cloud_storage","method":"signed_put",` +
				`"url":"` + capabilityGCSURL + `","expires_at":1893456000000000000}`,
		},
		{
			name: "google cloud storage with an empty header set",
			document: `{"provider":"google_cloud_storage","method":"signed_put",` +
				`"url":"` + capabilityGCSURL + `","expires_at":1893456000000000000,"headers":[]}`,
		},
		{
			name: "google cloud storage with one signed metadata field",
			document: `{"provider":"google_cloud_storage","method":"signed_put",` +
				`"url":"` + capabilityGCSMetadataRunURL + `","expires_at":1893456000000000000,` +
				`"headers":[{"name":"X-Goog-Meta-Run","value":"run-41"}]}`,
		},
		{
			name: "google cloud storage with two signed metadata fields",
			document: `{"provider":"google_cloud_storage","method":"signed_put",` +
				`"url":"` + capabilityGCSMetadataURL + `","expires_at":1893456000000000000,` +
				`"headers":[{"name":"X-Goog-Meta-Run","value":"run-41"},` +
				`{"name":"X-Goog-Meta-Shard","value":"7"}]}`,
		},
		{
			name: "google cloud storage header name in lowercase is canonicalized",
			document: `{"provider":"google_cloud_storage","method":"signed_put",` +
				`"url":"` + capabilityGCSMetadataRunURL + `","expires_at":1893456000000000000,` +
				`"headers":[{"name":"x-goog-meta-run","value":"run-41"}]}`,
		},
		{
			name: "amazon s3 signed put with no headers",
			document: `{"provider":"amazon_s3","method":"signed_put",` +
				`"url":"` + capabilityS3URL + `","expires_at":1893456000000000000}`,
		},
		{
			name: "amazon s3 with one signed metadata field",
			document: `{"provider":"amazon_s3","method":"signed_put",` +
				`"url":"` + capabilityS3MetadataURL + `","expires_at":1893456000000000000,` +
				`"headers":[{"name":"X-Amz-Meta-Run","value":"run-41"}]}`,
		},
		{
			name: "cloudflare images multipart post with no headers member",
			document: `{"provider":"cloudflare_images","method":"multipart_post",` +
				`"url":"` + capabilityImagesURL + `","expires_at":1893456000000000000}`,
		},
		{
			name: "cloudflare images with an empty header set",
			document: `{"provider":"cloudflare_images","method":"multipart_post",` +
				`"url":"` + capabilityImagesURL + `","expires_at":1893456000000000000,"headers":[]}`,
		},
		{
			name: "an already expired capability is still a well formed capability",
			document: `{"provider":"google_cloud_storage","method":"signed_put",` +
				`"url":"` + capabilityGCSURL + `","expires_at":1}`,
		},
		{
			name: "a pre-epoch expiry is admitted because expiry policy is the transfer's",
			document: `{"provider":"google_cloud_storage","method":"signed_put",` +
				`"url":"` + capabilityGCSURL + `","expires_at":-1000000000}`,
		},
		{
			name: "member order on the wire does not change admission",
			document: `{"expires_at":1893456000000000000,"url":"` + capabilityGCSURL + `",` +
				`"method":"signed_put","provider":"google_cloud_storage"}`,
		},

		{
			name: "absent provider member is rejected",
			document: `{"method":"signed_put","url":"` + capabilityGCSURL +
				`","expires_at":1893456000000000000}`,
			wantErr: true,
		},
		{
			name: "absent method member is rejected",
			document: `{"provider":"google_cloud_storage","url":"` + capabilityGCSURL +
				`","expires_at":1893456000000000000}`,
			wantErr: true,
		},
		{
			name: "absent url member is rejected",
			document: `{"provider":"google_cloud_storage","method":"signed_put",` +
				`"expires_at":1893456000000000000}`,
			wantErr: true,
		},
		{
			name: "absent expiry member is rejected",
			document: `{"provider":"google_cloud_storage","method":"signed_put",` +
				`"url":"` + capabilityGCSURL + `"}`,
			wantErr: true,
		},
		{
			name: "null provider member is rejected",
			document: `{"provider":null,"method":"signed_put","url":"` + capabilityGCSURL +
				`","expires_at":1893456000000000000}`,
			wantErr: true,
		},
		{
			name: "null expiry member is rejected",
			document: `{"provider":"google_cloud_storage","method":"signed_put",` +
				`"url":"` + capabilityGCSURL + `","expires_at":null}`,
			wantErr: true,
		},
		{
			name: "unknown provider token is rejected",
			document: `{"provider":"azure_blob","method":"signed_put","url":"` + capabilityGCSURL +
				`","expires_at":1893456000000000000}`,
			wantErr: true,
		},
		{
			name: "provider token in the wrong case is rejected",
			document: `{"provider":"Google_Cloud_Storage","method":"signed_put","url":"` +
				capabilityGCSURL + `","expires_at":1893456000000000000}`,
			wantErr: true,
		},
		{
			name: "empty provider token is rejected rather than matching the zero provider",
			document: `{"provider":"","method":"signed_put","url":"` + capabilityGCSURL +
				`","expires_at":1893456000000000000}`,
			wantErr: true,
		},
		{
			name: "unknown method token is rejected",
			document: `{"provider":"google_cloud_storage","method":"resumable_post","url":"` +
				capabilityGCSURL + `","expires_at":1893456000000000000}`,
			wantErr: true,
		},

		{
			name: "a method the named vendor does not publish is rejected",
			document: `{"provider":"google_cloud_storage","method":"multipart_post","url":"` +
				capabilityGCSURL + `","expires_at":1893456000000000000}`,
			wantErr: true,
		},
		{
			name: "cloudflare images cannot claim a signed put",
			document: `{"provider":"cloudflare_images","method":"signed_put","url":"` +
				capabilityImagesURL + `","expires_at":1893456000000000000}`,
			wantErr: true,
		},
		{
			name: "amazon s3 cannot claim a multipart post",
			document: `{"provider":"amazon_s3","method":"multipart_post","url":"` +
				capabilityS3URL + `","expires_at":1893456000000000000}`,
			wantErr: true,
		},
		{
			name: "a plaintext url is rejected",
			document: `{"provider":"google_cloud_storage","method":"signed_put",` +
				`"url":"http://storage.googleapis.com/bucket/object?` + queryGCSSignature +
				`=s&` + queryGCSSignedHeaders + `=host",` +
				`"expires_at":1893456000000000000}`,
			wantErr: true,
		},
		{
			name: "a root-only url path is rejected",
			document: `{"provider":"google_cloud_storage","method":"signed_put",` +
				`"url":"` + core.SchemeHTTPS + `://storage.googleapis.com/?` + queryGCSSignature +
				`=s&` + queryGCSSignedHeaders + `=host",` +
				`"expires_at":1893456000000000000}`,
			wantErr: true,
		},
		{
			name: "an empty url is rejected",
			document: `{"provider":"google_cloud_storage","method":"signed_put","url":"",` +
				`"expires_at":1893456000000000000}`,
			wantErr: true,
		},
		{
			name: "a google url with no signature query is rejected",
			document: `{"provider":"google_cloud_storage","method":"signed_put",` +
				`"url":"` + capabilityGCSObject + `?` + queryGCSSignedHeaders + `=host",` +
				`"expires_at":1893456000000000000}`,
			wantErr: true,
		},
		{
			name: "a google url with no signed-header declaration is rejected",
			document: `{"provider":"google_cloud_storage","method":"signed_put",` +
				`"url":"` + capabilityGCSObject + `?` + queryGCSSignature + `=s",` +
				`"expires_at":1893456000000000000}`,
			wantErr: true,
		},
		{
			name: "an amazon capability presented as a google one is rejected",
			document: `{"provider":"google_cloud_storage","method":"signed_put","url":"` +
				capabilityS3URL + `","expires_at":1893456000000000000}`,
			wantErr: true,
		},
		{
			name: "a cloudflare capability on a foreign host is rejected",
			document: `{"provider":"cloudflare_images","method":"multipart_post",` +
				`"url":"` + core.SchemeHTTPS + `://upload.example.com/image-id","expires_at":1893456000000000000}`,
			wantErr: true,
		},
		{
			name: "cloudflare images cannot carry any signed header",
			document: `{"provider":"cloudflare_images","method":"multipart_post","url":"` +
				capabilityImagesURL + `","expires_at":1893456000000000000,` +
				`"headers":[{"name":"X-Meta-Run","value":"run-41"}]}`,
			wantErr: true,
		},
		{
			name: "a header the capability did not sign is rejected",
			document: `{"provider":"google_cloud_storage","method":"signed_put","url":"` +
				capabilityGCSURL + `","expires_at":1893456000000000000,` +
				`"headers":[{"name":"X-Goog-Meta-Unsigned","value":"v"}]}`,
			wantErr: true,
		},
		{
			name: "a field this package sets itself is rejected",
			document: `{"provider":"google_cloud_storage","method":"signed_put","url":"` +
				capabilityGCSURL + `","expires_at":1893456000000000000,` +
				`"headers":[{"name":"Content-Type","value":"application/octet-stream"}]}`,
			wantErr: true,
		},
		{
			name: "a duplicate header differing only by case is rejected",
			document: `{"provider":"google_cloud_storage","method":"signed_put","url":"` +
				capabilityGCSURL + `","expires_at":1893456000000000000,` +
				`"headers":[{"name":"X-Goog-Meta-Run","value":"a"},` +
				`{"name":"x-goog-meta-run","value":"b"}]}`,
			wantErr: true,
		},
		{
			name: "an exactly duplicated header is rejected",
			document: `{"provider":"google_cloud_storage","method":"signed_put","url":"` +
				capabilityGCSURL + `","expires_at":1893456000000000000,` +
				`"headers":[{"name":"X-Goog-Meta-Run","value":"a"},` +
				`{"name":"X-Goog-Meta-Run","value":"a"}]}`,
			wantErr: true,
		},
		{
			name: "a header with an absent name is rejected",
			document: `{"provider":"google_cloud_storage","method":"signed_put","url":"` +
				capabilityGCSURL + `","expires_at":1893456000000000000,` +
				`"headers":[{"value":"run-41"}]}`,
			wantErr: true,
		},
		{
			name: "a header with an absent value is rejected",
			document: `{"provider":"google_cloud_storage","method":"signed_put","url":"` +
				capabilityGCSURL + `","expires_at":1893456000000000000,` +
				`"headers":[{"name":"X-Goog-Meta-Run"}]}`,
			wantErr: true,
		},
		{
			name: "a header with an empty but present value is admitted",
			document: `{"provider":"google_cloud_storage","method":"signed_put","url":"` +
				capabilityGCSMetadataRunURL + `","expires_at":1893456000000000000,` +
				`"headers":[{"name":"X-Goog-Meta-Run","value":""}]}`,
		},
		{
			name: "a header name outside http token syntax is rejected",
			document: `{"provider":"google_cloud_storage","method":"signed_put","url":"` +
				capabilityGCSURL + `","expires_at":1893456000000000000,` +
				`"headers":[{"name":"X Goog Meta Run","value":"v"}]}`,
			wantErr: true,
		},
		{
			name: "a header value carrying field injection is rejected",
			document: `{"provider":"google_cloud_storage","method":"signed_put","url":"` +
				capabilityGCSURL + `","expires_at":1893456000000000000,` +
				`"headers":[{"name":"X-Goog-Meta-Run","value":"a\r\nX-Injected: b"}]}`,
			wantErr: true,
		},
		{
			name: "an expiry encoded as a json string is rejected",
			document: `{"provider":"google_cloud_storage","method":"signed_put","url":"` +
				capabilityGCSURL + `","expires_at":"1893456000000000000"}`,
			wantErr: true,
		},
		{
			name: "a non-canonical expiry encoding is rejected",
			document: `{"provider":"google_cloud_storage","method":"signed_put","url":"` +
				capabilityGCSURL + `","expires_at":+1893456000000000000}`,
			wantErr: true,
		},
		{
			name: "an unknown member is rejected",
			document: `{"provider":"google_cloud_storage","method":"signed_put","url":"` +
				capabilityGCSURL + `","expires_at":1893456000000000000,"bucket":"b"}`,
			wantErr: true,
		},
		{
			name: "a duplicated member is rejected",
			document: `{"provider":"google_cloud_storage","provider":"amazon_s3",` +
				`"method":"signed_put","url":"` + capabilityGCSURL +
				`","expires_at":1893456000000000000}`,
			wantErr: true,
		},
		{
			name: "a member duplicated only by case folding is rejected",
			document: `{"provider":"google_cloud_storage","Provider":"amazon_s3",` +
				`"method":"signed_put","url":"` + capabilityGCSURL +
				`","expires_at":1893456000000000000}`,
			wantErr: true,
		},
		{
			name:     "a null document is rejected",
			document: `null`,
			wantErr:  true,
		},
		{
			name:     "an array document is rejected",
			document: `[]`,
			wantErr:  true,
		},
		{
			name:     "an empty object is rejected",
			document: `{}`,
			wantErr:  true,
		},
		{
			name: "trailing data after the document is rejected",
			document: `{"provider":"google_cloud_storage","method":"signed_put","url":"` +
				capabilityGCSURL + `","expires_at":1893456000000000000}{}`,
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := capabilityDocument(t, tc.document)
			if tc.wantErr {
				if !errors.Is(gotErr, core.ErrObjectStoreContract) {
					t.Fatalf("decode error = %v, want %v", gotErr, core.ErrObjectStoreContract)
				}
				if !got.IsZero() {
					t.Fatalf("receiver after a rejected decode is set, want the zero capability")
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("decode error = %v, want nil", gotErr)
			}
			if got.IsZero() {
				t.Fatalf("accepted capability IsZero() = true, want false")
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("accepted capability Validate() error = %v, want nil", err)
			}
			if _, err := got.Provider(); err != nil {
				t.Fatalf("accepted capability Provider() error = %v, want nil", err)
			}
			target, targetErr := got.Target()
			if targetErr != nil {
				t.Fatalf("accepted capability Target() error = %v, want nil", targetErr)
			}
			if err := target.Validate(); err != nil {
				t.Fatalf("projected UploadTarget.Validate() error = %v, want nil", err)
			}
		})
	}
}

// TestUploadCapabilityProviderProjectionIsExact proves the wire token selects
// exactly the vendor it names, so a capability cannot be spent at the wrong
// destination.
func TestUploadCapabilityProviderProjectionIsExact(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		document  string
		wantToken string
		want      Provider
	}{
		{
			name: "amazon s3 token selects amazon s3",
			document: `{"provider":"amazon_s3","method":"signed_put","url":"` +
				capabilityS3URL + `","expires_at":1893456000000000000}`,
			wantToken: "amazon_s3",
			want:      ProviderAmazonS3,
		},
		{
			name: "google cloud storage token selects google cloud storage",
			document: `{"provider":"google_cloud_storage","method":"signed_put","url":"` +
				capabilityGCSURL + `","expires_at":1893456000000000000}`,
			wantToken: "google_cloud_storage",
			want:      ProviderGoogleCloudStorage,
		},
		{
			name: "cloudflare images token selects cloudflare images",
			document: `{"provider":"cloudflare_images","method":"multipart_post","url":"` +
				capabilityImagesURL + `","expires_at":1893456000000000000}`,
			wantToken: "cloudflare_images",
			want:      ProviderCloudflareImages,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			capability, err := capabilityDocument(t, tc.document)
			if err != nil {
				t.Fatalf("decode error = %v, want nil", err)
			}
			got, gotErr := capability.Provider()
			if gotErr != nil {
				t.Fatalf("Provider() error = %v, want nil", gotErr)
			}
			if got != tc.want {
				t.Fatalf("Provider() = %v, want %v", got, tc.want)
			}
			if got.String() != tc.wantToken {
				t.Fatalf("Provider().String() = %q, want %q", got.String(), tc.wantToken)
			}
			if !strings.Contains(tc.document, `"provider":"`+tc.wantToken+`"`) {
				t.Fatalf("document = %s, want it to carry provider token %q", tc.document, tc.wantToken)
			}
		})
	}
}

// TestUploadCapabilityNeverDisclosesItsBearer is the disclosure ratchet. A
// capability carries a credential, so no formatting verb and no rejection
// diagnostic may render it.
func TestUploadCapabilityNeverDisclosesItsBearer(t *testing.T) {
	t.Parallel()

	document := `{"provider":"google_cloud_storage","method":"signed_put",` +
		`"url":"` + capabilityGCSObject + `?` + queryGCSSignature + `=` +
		capabilitySecret + `&` + queryGCSSignedHeaders +
		`=host%3Bx-goog-hash%3Bx-goog-if-generation-match","expires_at":1893456000000000000}`

	capability, err := capabilityDocument(t, document)
	if err != nil {
		t.Fatalf("decode error = %v, want nil", err)
	}

	t.Run("no formatting verb renders the bearer", func(t *testing.T) {
		t.Parallel()

		for _, format := range []string{"%v", "%+v", "%#v", "%s", "%q", "%d", "%x"} {
			rendered := fmt.Sprintf(format, capability)
			if strings.Contains(rendered, capabilitySecret) {
				t.Fatalf("Sprintf(%q) = %q, want no bearer material", format, rendered)
			}
			if rendered != core.RedactedValueText {
				t.Fatalf("Sprintf(%q) = %q, want %q", format, rendered, core.RedactedValueText)
			}
		}
	})

	t.Run("the projected target does not render the bearer", func(t *testing.T) {
		t.Parallel()

		target, targetErr := capability.Target()
		if targetErr != nil {
			t.Fatalf("Target() error = %v, want nil", targetErr)
		}
		rendered := fmt.Sprintf("%v %+v %#v", target, target.URL, target.URL)
		if strings.Contains(rendered, capabilitySecret) {
			t.Fatalf("formatted target = %q, want no bearer material", rendered)
		}
	})

	t.Run("a rejection diagnostic does not render the bearer", func(t *testing.T) {
		t.Parallel()

		rejected := `{"provider":"google_cloud_storage","method":"multipart_post",` +
			`"url":"` + capabilityGCSObject + `?` + queryGCSSignature + `=` +
			capabilitySecret + `&` + queryGCSSignedHeaders + `=host","expires_at":1893456000000000000}`
		_, rejectErr := capabilityDocument(t, rejected)
		// The load-bearing rejection proof is the typed identity. The rendering
		// check below is the second tier: it proves what an operator or a log
		// line would see, which is the only place a bearer can leak.
		if !errors.Is(rejectErr, core.ErrObjectStoreContract) {
			t.Fatalf("decode error = %v, want %v", rejectErr, core.ErrObjectStoreContract)
		}
		rendered := fmt.Sprintf("%v|%+v", rejectErr, rejectErr)
		if strings.Contains(rendered, capabilitySecret) {
			t.Fatalf("rendered rejection = %q, want no bearer material", rendered)
		}
	})
}

// TestUploadCapabilityIsDecodeOnly is the structural ratchet behind the
// boundary. Emitting a capability is issuing one, which this package excludes,
// so the type must not acquire a marshaler or a string accessor by accident.
func TestUploadCapabilityIsDecodeOnly(t *testing.T) {
	t.Parallel()

	capability := any(UploadCapability{})
	if _, ok := capability.(json.Marshaler); ok {
		t.Fatalf("UploadCapability implements json.Marshaler, want no marshaler")
	}
	if _, ok := capability.(fmt.Stringer); ok {
		t.Fatalf("UploadCapability implements fmt.Stringer, want no string accessor")
	}
	if _, ok := capability.(encoding.TextMarshaler); ok {
		t.Fatalf("UploadCapability implements encoding.TextMarshaler, want no text accessor")
	}
	if _, ok := any(&UploadCapability{}).(json.Unmarshaler); !ok {
		t.Fatalf("*UploadCapability does not implement json.Unmarshaler, want a decoder")
	}
}

// TestUploadCapabilityZeroValueRefusesEveryProjection proves the neutral case:
// an undecoded capability produces no provider, no target, and no silent zero.
func TestUploadCapabilityZeroValueRefusesEveryProjection(t *testing.T) {
	t.Parallel()

	var capability UploadCapability
	if !capability.IsZero() {
		t.Fatalf("zero UploadCapability.IsZero() = false, want true")
	}
	if err := capability.Validate(); !errors.Is(err, core.ErrObjectStoreContract) {
		t.Fatalf("zero UploadCapability.Validate() error = %v, want %v",
			err, core.ErrObjectStoreContract)
	}
	provider, providerErr := capability.Provider()
	if !errors.Is(providerErr, core.ErrObjectStoreContract) ||
		provider != ProviderUnknown {
		t.Fatalf("zero UploadCapability.Provider() = (%v, %v), want (%v, %v)",
			provider, providerErr, ProviderUnknown, core.ErrObjectStoreContract)
	}
	target, targetErr := capability.Target()
	if !errors.Is(targetErr, core.ErrObjectStoreContract) {
		t.Fatalf("zero UploadCapability.Target() error = %v, want %v",
			targetErr, core.ErrObjectStoreContract)
	}
	if target.Validate() == nil {
		t.Fatalf("zero UploadCapability.Target() returned a valid target, want the zero target")
	}

	var absent *UploadCapability
	if err := absent.UnmarshalJSON([]byte(`{}`)); !errors.Is(err, core.ErrObjectStoreContract) {
		t.Fatalf("(*UploadCapability)(nil).UnmarshalJSON() error = %v, want %v",
			err, core.ErrObjectStoreContract)
	}
}

func TestUploadCapabilityRejectedDecodePreservesExistingReceiver(t *testing.T) {
	t.Parallel()

	document := `{"provider":"` + ProviderAmazonS3.String() + `","method":"` +
		UploadMethodTokenSignedPut + `","url":"` +
		capabilityS3URL + `","expires_at":1893456000000000000}`
	capability, err := capabilityDocument(t, document)
	if err != nil {
		t.Fatalf("initial decode error = %v, want nil", err)
	}

	hostile := `{"provider":"` + ProviderGoogleCloudStorage.String() + `","method":"` +
		UploadMethodTokenMultipartPost + `","url":"` +
		capabilityGCSURL + `","expires_at":1893456000000000000}`
	if gotErr := capability.UnmarshalJSON([]byte(hostile)); !errors.Is(gotErr, core.ErrObjectStoreContract) {
		t.Fatalf("rejected replacement decode error = %v, want %v",
			gotErr, core.ErrObjectStoreContract)
	}
	provider, providerErr := capability.Provider()
	if providerErr != nil || provider != ProviderAmazonS3 {
		t.Fatalf("Provider() after rejected replacement = (%v, %v), want (%v, nil)",
			provider, providerErr, ProviderAmazonS3)
	}
	if _, targetErr := capability.Target(); targetErr != nil {
		t.Fatalf("Target() after rejected replacement error = %v, want nil", targetErr)
	}
}

// TestUploadCapabilityBoundsItsReceivedExtents proves both sides of the URL and
// signed-header bounds this package already owns.
func TestUploadCapabilityBoundsItsReceivedExtents(t *testing.T) {
	t.Parallel()

	base := capabilityGCSObject + "?" + queryGCSSignedHeaders +
		"=host%3Bx-goog-hash%3Bx-goog-if-generation-match&" +
		queryGCSSignature + "="

	cases := []struct {
		name       string
		signature  int
		wantReject bool
	}{
		{name: "a url far below the bound is admitted", signature: 64},
		{
			name:      "a url one byte below the bound is admitted",
			signature: CapabilityURLMaximumBytes - len(base) - 1,
		},
		{
			name:      "a url exactly at the bound is admitted",
			signature: CapabilityURLMaximumBytes - len(base),
		},
		{
			name:       "a url one byte above the bound is rejected",
			signature:  CapabilityURLMaximumBytes - len(base) + 1,
			wantReject: true,
		},
		{
			name:       "a url far above the bound is rejected",
			signature:  CapabilityURLMaximumBytes,
			wantReject: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			document := `{"provider":"google_cloud_storage","method":"signed_put","url":"` +
				base + strings.Repeat("a", tc.signature) +
				`","expires_at":1893456000000000000}`
			got, gotErr := capabilityDocument(t, document)
			if tc.wantReject {
				if !errors.Is(gotErr, core.ErrObjectStoreContract) {
					t.Fatalf("decode error = %v, want %v", gotErr, core.ErrObjectStoreContract)
				}
				if !got.IsZero() {
					t.Fatalf("receiver after a rejected decode is set, want the zero capability")
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("decode error = %v, want nil", gotErr)
			}
		})
	}

	t.Run("a header set above the owned count bound is rejected", func(t *testing.T) {
		t.Parallel()

		signed := make([]string, 0, SignedHeaderMaximumCount+4)
		signed = append(signed, "host", "x-goog-hash", "x-goog-if-generation-match")
		wire := make([]string, 0, SignedHeaderMaximumCount+1)
		for index := 1; index <= SignedHeaderMaximumCount+1; index++ {
			name := fmt.Sprintf("x-goog-meta-f%02d", index)
			signed = append(signed, name)
			wire = append(wire, fmt.Sprintf(`{"name":%q,"value":"v"}`, name))
		}
		document := `{"provider":"google_cloud_storage","method":"signed_put",` +
			`"url":"` + capabilityGCSObject + `?` + queryGCSSignature + `=s&` +
			queryGCSSignedHeaders + `=` + strings.Join(signed, "%3B") + `",` +
			`"expires_at":1893456000000000000,"headers":[` + strings.Join(wire, ",") + `]}`
		got, gotErr := capabilityDocument(t, document)
		if !errors.Is(gotErr, core.ErrObjectStoreContract) {
			t.Fatalf("decode error = %v, want %v", gotErr, core.ErrObjectStoreContract)
		}
		if !got.IsZero() {
			t.Fatalf("receiver after a rejected decode is set, want the zero capability")
		}
	})

	t.Run("insignificant whitespace above the document bound is rejected", func(t *testing.T) {
		t.Parallel()

		// This case isolates the document bound from every sub-bound. The URL
		// and the header set both stay well inside their own limits, so only
		// the complete received extent can reject it.
		padding := strings.Repeat(" ", CapabilityJSONMaximumBytes)
		document := `{` + padding + `"provider":"google_cloud_storage","method":"signed_put",` +
			`"url":"` + capabilityGCSURL + `","expires_at":1893456000000000000}`
		got, gotErr := capabilityDocument(t, document)
		if !errors.Is(gotErr, core.ErrObjectStoreContract) {
			t.Fatalf("decode error = %v, want %v", gotErr, core.ErrObjectStoreContract)
		}
		if !got.IsZero() {
			t.Fatalf("receiver after a rejected decode is set, want the zero capability")
		}
	})

	t.Run("insignificant whitespace below the document bound is admitted", func(t *testing.T) {
		t.Parallel()

		base := `{"provider":"google_cloud_storage","method":"signed_put",` +
			`"url":"` + capabilityGCSURL + `","expires_at":1893456000000000000}`
		padding := strings.Repeat(" ", CapabilityJSONMaximumBytes-len(base))
		if _, err := capabilityDocument(t, `{`+padding+base[1:]); err != nil {
			t.Fatalf("decode of a document exactly at the bound error = %v, want nil", err)
		}
	})

	t.Run("an oversized header value is rejected", func(t *testing.T) {
		t.Parallel()

		document := `{"provider":"google_cloud_storage","method":"signed_put","url":"` +
			capabilityGCSMetadataRunURL + `","expires_at":1893456000000000000,"headers":[{"name":"X-Goog-Meta-Run",` +
			`"value":"` + strings.Repeat("v", CapabilityJSONMaximumBytes) + `"}]}`
		got, gotErr := capabilityDocument(t, document)
		if !errors.Is(gotErr, core.ErrObjectStoreContract) {
			t.Fatalf("decode error = %v, want %v", gotErr, core.ErrObjectStoreContract)
		}
		if !got.IsZero() {
			t.Fatalf("receiver after a rejected decode is set, want the zero capability")
		}
	})
}

// TestUploadCapabilityDecodesThroughTheRealOuterDecoders keeps the hostile
// tables honest. Those tables drive UnmarshalJSON directly so the identity they
// assert is this package's, which means the type must separately be proved to
// work as a member of a real document under both the standard library and
// Core's strict decoder.
func TestUploadCapabilityDecodesThroughTheRealOuterDecoders(t *testing.T) {
	t.Parallel()

	document := `{"capability":{"provider":"google_cloud_storage","method":"signed_put",` +
		`"url":"` + capabilityGCSURL + `","expires_at":1893456000000000000}}`

	t.Run("as a member decoded by encoding/json", func(t *testing.T) {
		t.Parallel()

		var carrier capabilityCarrier
		if err := json.Unmarshal([]byte(document), &carrier); err != nil {
			t.Fatalf("json.Unmarshal() error = %v, want nil", err)
		}
		requireCapabilityGoogleCloudStorage(t, carrier.Capability)
	})

	t.Run("as a member decoded by the strict json contract", func(t *testing.T) {
		t.Parallel()

		carrier, err := core.DecodeStrictJSON[capabilityCarrier](
			[]byte(document), core.DefaultStrictJSONLimits(),
		)
		if err != nil {
			t.Fatalf("DecodeStrictJSON() error = %v, want nil", err)
		}
		requireCapabilityGoogleCloudStorage(t, carrier.Capability)
	})

	t.Run("a rejected member fails the whole strict document", func(t *testing.T) {
		t.Parallel()

		hostile := `{"capability":{"provider":"google_cloud_storage","method":"multipart_post",` +
			`"url":"` + capabilityGCSURL + `","expires_at":1893456000000000000}}`
		got, err := core.DecodeStrictJSON[capabilityCarrier](
			[]byte(hostile), core.DefaultStrictJSONLimits(),
		)
		if !errors.Is(err, core.ErrObjectStoreContract) {
			t.Fatalf("DecodeStrictJSON() error = %v, want %v", err, core.ErrObjectStoreContract)
		}
		if !got.Capability.IsZero() {
			t.Fatalf("carrier after a rejected decode carries a capability, want the zero value")
		}
	})
}

// capabilityCarrier is the smallest real document that embeds a capability, the
// shape every consumer protocol wraps it in.
type capabilityCarrier struct {
	Capability UploadCapability `json:"capability"`
}

// Validate delegates to the capability, so the carrier cannot pass while the
// member it exists to carry is unset.
func (c capabilityCarrier) Validate() error { return c.Capability.Validate() }

func requireCapabilityGoogleCloudStorage(t *testing.T, got UploadCapability) {
	t.Helper()

	provider, err := got.Provider()
	if err != nil {
		t.Fatalf("Provider() error = %v, want nil", err)
	}
	if provider != ProviderGoogleCloudStorage {
		t.Fatalf("Provider() = %v, want %v", provider, ProviderGoogleCloudStorage)
	}
	if _, err := got.Target(); err != nil {
		t.Fatalf("Target() error = %v, want nil", err)
	}
}
