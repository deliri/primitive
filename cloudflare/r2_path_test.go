package cloudflare

import (
	"net/http"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/smithy-go/encoding/httpbinding"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

// An R2 grant's wire path is the AWS S3 serializer's escaping of the exact
// bucket and key bytes, and its signature is the AWS SDK signer's over that
// same path: reserved punctuation and spaces are percent-encoded, Unicode and
// percent bytes stay exact, and repeated slashes and dot segments are never
// cleaned.
func TestR2NativePathEncodingLayerTriad(t *testing.T) {
	t.Parallel()
	server := testR2Server(t, R2JurisdictionDefault)
	bucket, err := ParseR2Bucket("anvil-retention")
	if err != nil {
		t.Fatal(err)
	}
	expires, err := temporal.DurationFromSeconds(60)
	if err != nil {
		t.Fatal(err)
	}
	signedAt := temporal.InstantFromNanoseconds(4_000_000_000_000_000_000)
	for _, row := range []struct {
		layer, name, key string
	}{
		{layer: "positive", name: "unreserved bytes stay literal", key: "Az09-._~"},
		{layer: "negative", name: "reserved punctuation is percent-encoded", key: "a+b:c@d&e=f;$g"},
		{layer: "negative", name: "Unicode, percent, hash and query bytes are exact", key: "東京/%+#?"},
		// Mixed with reserved bytes, so a discarded raw path cannot mask it.
		{layer: "negative", name: "a space is %20, never a form plus", key: "run 1/a+b c"},
		{layer: "neutral", name: "repeated slashes and dot segments are never cleaned", key: "//./a/../"},
	} {
		t.Run(row.layer+"/"+row.name, func(t *testing.T) {
			t.Parallel()
			key, err := ParseR2Key(row.key)
			if err != nil {
				t.Fatalf("key %q: %v", row.key, err)
			}
			grant, err := server.Presign(t.Context(), R2PresignRequest{Bucket: bucket, Key: key, SignedAt: signedAt, Expires: expires, Method: exchange.MethodGet})
			if err != nil {
				t.Fatalf("presign %q: %v", row.key, err)
			}
			endpoint, err := grant.Endpoint()
			if err != nil {
				t.Fatal(err)
			}
			got := endpoint.HTTPURL()
			want := httpbinding.EscapePath("/"+bucket.String()+"/"+row.key, false)
			if got.EscapedPath() != want || got.Path != "/"+bucket.String()+"/"+row.key {
				t.Fatalf("R2 wire path %q decoded %q, want native %q for exact key %q", got.EscapedPath(), got.Path, want, row.key)
			}
			// Re-sign the same target with the AWS SDK over the native path.
			native := got
			query := native.Query()
			signature := query.Get(core.SignatureV4SignatureQuery)
			for _, name := range []string{core.SignatureV4SignatureQuery, core.SignatureV4AlgorithmQuery, core.SignatureV4CredentialQuery, core.SignatureV4DateQuery, core.SignatureV4SignedHeadersQuery} {
				query.Del(name)
			}
			native.RawQuery = query.Encode()
			request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, native.String(), nil)
			if err != nil {
				t.Fatal(err)
			}
			signer := v4.NewSigner(func(options *v4.SignerOptions) {
				options.DisableHeaderHoisting = true
				options.DisableURIPathEscaping = true
			})
			stamp, err := signedAt.Time()
			if err != nil {
				t.Fatal(err)
			}
			signed, _, err := signer.PresignHTTP(t.Context(), aws.Credentials{AccessKeyID: "r2-access-key", SecretAccessKey: "r2-secret-key"}, request, core.CloudflareR2UnsignedPayload, core.CloudflareR2SigningService, core.CloudflareR2SigningRegion, stamp.UTC().Truncate(time.Second))
			if err != nil {
				t.Fatal(err)
			}
			reference, err := core.ParseHTTPEndpoint(signed)
			if err != nil {
				t.Fatal(err)
			}
			referenceURL := reference.HTTPURL()
			if expected := referenceURL.Query().Get(core.SignatureV4SignatureQuery); signature == "" || signature != expected {
				t.Fatalf("R2 signature %q, want the AWS SDK's %q over native path %q", signature, expected, want)
			}
		})
	}
}
