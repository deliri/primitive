package cloudflare

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

func testR2Server(t *testing.T, jurisdiction R2Jurisdiction) R2Server {
	t.Helper()
	credentials, err := ParseR2Credentials([]byte("r2-access-key"), []byte("r2-secret-key"))
	if err != nil {
		t.Fatal(err)
	}
	account, err := ParseAccountID(strings.Repeat("a", core.CloudflareIdentityCharacters))
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewR2Server(credentials, account, jurisdiction)
	if err != nil {
		t.Fatal(err)
	}
	if err := credentials.Close(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
	})
	return server
}
func testR2Intent(t *testing.T, method exchange.Method, key string) R2PresignRequest {
	t.Helper()
	bucket, err := ParseR2Bucket("media-bucket")
	if err != nil {
		t.Fatal(err)
	}
	object, err := ParseR2Key(key)
	if err != nil {
		t.Fatal(err)
	}
	stamp, err := temporal.InstantFromUnixSeconds(webhookTestSeconds)
	if err != nil {
		t.Fatal(err)
	}
	expires, err := temporal.DurationFromSeconds(60)
	if err != nil {
		t.Fatal(err)
	}
	return R2PresignRequest{Bucket: bucket, Key: object, SignedAt: stamp, Expires: expires, Method: method}
}

// Independent SigV4 oracle: production uses the official SDK signer; this
// provider fixture rebuilds the canonical request from the observed HTTP bytes
// and the explicitly pinned R2 scope, then uses Go's HMAC implementation.
func verifyR2TestSignature(request *http.Request, secret []byte) bool {
	query := request.URL.Query()
	signature := query.Get("X-Amz-Signature")
	query.Del("X-Amz-Signature")
	stamp := query.Get("X-Amz-Date")
	if len(stamp) < 8 {
		return false
	}
	scope := stamp[:8] + "/auto/s3/aws4_request"
	headerNames := strings.Split(query.Get("X-Amz-SignedHeaders"), ";")
	var headers strings.Builder
	for _, name := range headerNames {
		value := request.Header.Get(name)
		if name == "host" {
			value = request.Host
		}
		headers.WriteString(name + ":" + strings.TrimSpace(value) + "\n")
	}
	canonical := request.Method + "\n" + request.URL.EscapedPath() + "\n" + strings.ReplaceAll(query.Encode(), "+", "%20") + "\n" + headers.String() + "\n" + strings.Join(headerNames, ";") + "\nUNSIGNED-PAYLOAD"
	hash := sha256.Sum256([]byte(canonical))
	message := "AWS4-HMAC-SHA256\n" + stamp + "\n" + scope + "\n" + hex.EncodeToString(hash[:])
	key := append([]byte("AWS4"), secret...)
	for _, part := range []string{stamp[:8], "auto", "s3", "aws4_request", message} {
		mac := hmac.New(sha256.New, key)
		_, _ = mac.Write([]byte(part))
		key = mac.Sum(nil)
	}
	return hmac.Equal([]byte(signature), []byte(hex.EncodeToString(key)))
}

func TestR2ServerPresignProviderBindingLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		key          string
		media        core.HTTPMediaType
		jurisdiction R2Jurisdiction
		method       exchange.Method
	}{
		{name: "default jurisdiction GET retains slash and UTF8", key: "folder/café.png", jurisdiction: R2JurisdictionDefault, method: exchange.MethodGet},
		{name: "EU HEAD retains percent literal", key: "percent%25", jurisdiction: R2JurisdictionEU, method: exchange.MethodHead},
		{name: "US PUT binds exact content type", key: "a space+plus", jurisdiction: R2JurisdictionUS, method: exchange.MethodPut, media: core.HTTPMediaTypeJSON()},
		{name: "FedRAMP DELETE retains dot segments as object identity", key: "a/../b", jurisdiction: R2JurisdictionFedRAMP, method: exchange.MethodDelete},
		{name: "GET leading slash remains in object key", key: "/object", jurisdiction: R2JurisdictionDefault, method: exchange.MethodGet},
		{name: "GET repeated slashes remain distinct", key: "a//b", jurisdiction: R2JurisdictionDefault, method: exchange.MethodGet},
		{name: "GET query and fragment characters remain path bytes", key: "a?b#c", jurisdiction: R2JurisdictionDefault, method: exchange.MethodGet},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := testR2Server(t, tc.jurisdiction)
			intent := testR2Intent(t, tc.method, tc.key)
			intent.ContentType = tc.media
			grant, err := server.Presign(t.Context(), intent)
			if err != nil {
				t.Fatal(err)
			}
			u := grant.endpoint.HTTPURL()
			request, err := http.NewRequestWithContext(t.Context(), tc.method.String(), u.String(), nil)
			if err != nil {
				t.Fatal(err)
			}
			if !tc.media.IsZero() {
				request.Header.Set("Content-Type", tc.media.String())
			}
			if u.Path != "/media-bucket/"+tc.key || !verifyR2TestSignature(request, []byte("r2-secret-key")) {
				t.Fatalf("presign path/signature=(%q,%t), want exact key and independent valid signature", u.Path, verifyR2TestSignature(request, []byte("r2-secret-key")))
			}
			input := R2GrantInput{Endpoint: grant.endpoint, Account: server.account, Jurisdiction: tc.jurisdiction, Method: tc.method, ContentType: tc.media}
			parsed, err := ParseR2Grant(input)
			if err != nil || parsed != grant {
				t.Fatalf("ParseR2Grant=(%v,%v), want original grant", parsed, err)
			}
			request.URL.Path += "x"
			if verifyR2TestSignature(request, []byte("r2-secret-key")) {
				t.Fatal("mutated key signature = admitted, want rejection")
			}
		})
	}
	t.Run("unknown method emits no grant", func(t *testing.T) {
		t.Parallel()
		server := testR2Server(t, R2JurisdictionDefault)
		intent := testR2Intent(t, exchange.MethodPost, "key")
		got, err := server.Presign(t.Context(), intent)
		if !errors.Is(err, core.ErrCloudflareBinding) || got != (R2Grant{}) {
			t.Fatalf("Presign=(%v,%v), want zero/binding refusal", got, err)
		}
	})
	t.Run("zero expiry emits no grant", func(t *testing.T) {
		t.Parallel()
		server := testR2Server(t, R2JurisdictionDefault)
		intent := testR2Intent(t, exchange.MethodGet, "key")
		intent.Expires = temporal.Duration{}
		got, err := server.Presign(t.Context(), intent)
		if !errors.Is(err, core.ErrCloudflareContract) || got != (R2Grant{}) {
			t.Fatalf("Presign=(%v,%v), want zero/contract refusal", got, err)
		}
	})
}

func TestR2ClientRealTLSLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		method     exchange.Method
		wantStatus int
	}{
		{name: "GET streams exact stored body", body: strings.Repeat("stored", 10000), method: exchange.MethodGet, wantStatus: http.StatusOK},
		{name: "empty GET remains successful empty object", method: exchange.MethodGet, wantStatus: http.StatusOK},
		{name: "HEAD performs one metadata operation", method: exchange.MethodHead, wantStatus: http.StatusOK},
		{name: "PUT sends exact signed media", body: "{\"object\":1}", method: exchange.MethodPut, wantStatus: http.StatusOK},
		{name: "DELETE preserves empty provider completion", method: exchange.MethodDelete, wantStatus: http.StatusNoContent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			observed := make(chan string, 1)
			client := testExchange(t, func(w http.ResponseWriter, r *http.Request) {
				if !verifyR2TestSignature(r, []byte("r2-secret-key")) {
					w.WriteHeader(http.StatusForbidden)
					observed <- "signature refused"
					return
				}
				if r.Method != tc.method.String() {
					t.Errorf("method=%s, want %s", r.Method, tc.method)
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				observed <- string(body)
				w.WriteHeader(tc.wantStatus)
				if tc.method == exchange.MethodGet {
					if _, err := io.WriteString(w, tc.body); err != nil {
						t.Error(err)
					}
				}
			})
			server := testR2Server(t, R2JurisdictionDefault)
			intent := testR2Intent(t, tc.method, "object")
			if tc.method == exchange.MethodPut {
				intent.ContentType = core.HTTPMediaTypeJSON()
			}
			grant, err := server.Presign(t.Context(), intent)
			if err != nil {
				t.Fatal(err)
			}
			r2, err := NewR2Client(client)
			if err != nil {
				t.Fatal(err)
			}
			var destination bytes.Buffer
			var result exchange.StreamResponse
			switch tc.method {
			case exchange.MethodGet, exchange.MethodHead:
				result, err = r2.Read(t.Context(), grant, &destination, testPolicy())
			case exchange.MethodPut:
				length, lengthErr := core.NewByteLength(uint64(len(tc.body)))
				if lengthErr != nil {
					t.Fatal(lengthErr)
				}
				result, err = r2.Put(t.Context(), R2WriteRequest{Grant: grant, Source: strings.NewReader(tc.body), Bytes: length}, testPolicy())
			case exchange.MethodDelete:
				result, err = r2.Delete(t.Context(), grant, testPolicy())
			default:
				t.Fatalf("fixture method = %v, want GET, HEAD, PUT or DELETE", tc.method)
			}
			if err != nil {
				t.Fatal(err)
			}
			status, err := result.Metadata.Status.Int()
			if err != nil || status != tc.wantStatus || result.Metadata.Attempts != 1 {
				t.Fatalf("response=(%+v,%v), want status %d and one attempt", result, err, tc.wantStatus)
			}
			gotBody := <-observed
			if tc.method == exchange.MethodPut && gotBody != tc.body {
				t.Fatalf("uploaded=%q, want %q", gotBody, tc.body)
			}
			if tc.method == exchange.MethodGet && destination.String() != tc.body {
				t.Fatalf("download bytes=%d, want %d", destination.Len(), len(tc.body))
			}
		})
	}
}

func FuzzR2GrantSignatureAndAuthority(f *testing.F) {
	f.Add("folder/object", uint32(1), uint8(R2JurisdictionDefault))
	f.Add("space %+#/../x", uint32(core.CloudflareR2PresignMaximumSeconds), uint8(R2JurisdictionEU))
	f.Fuzz(func(t *testing.T, key string, seconds uint32, jurisdiction uint8) {
		object, err := ParseR2Key(key)
		if err != nil {
			if !errors.Is(err, core.ErrCloudflareBinding) || object != (R2Key{}) {
				t.Fatalf("key refusal=(%v,%v), want zero and binding", object, err)
			}
			return
		}
		j := R2Jurisdiction(jurisdiction)
		if j.Validate() != nil {
			return
		}
		server := testR2Server(t, j)
		intent := testR2Intent(t, exchange.MethodGet, key)
		intent.Expires, err = temporal.DurationFromSeconds(uint64(seconds))
		if err != nil {
			t.Fatal(err)
		}
		grant, err := server.Presign(t.Context(), intent)
		if seconds < 1 || seconds > core.CloudflareR2PresignMaximumSeconds {
			if !errors.Is(err, core.ErrCloudflareContract) || grant != (R2Grant{}) {
				t.Fatalf("expiry %d=(%v,%v), want zero/refusal", seconds, grant, err)
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, grant.endpoint.String(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if !verifyR2TestSignature(request, []byte("r2-secret-key")) || request.URL.Path != "/media-bucket/"+key {
			t.Fatalf("signed request path = %q, verified = %v, want %q and true", request.URL.Path, verifyR2TestSignature(request, []byte("r2-secret-key")), "/media-bucket/"+key)
		}
		query := request.URL.Query()
		query.Set("X-Amz-Expires", strconv.FormatUint(uint64(seconds)+1, 10))
		request.URL.RawQuery = query.Encode()
		if verifyR2TestSignature(request, []byte("r2-secret-key")) {
			t.Fatalf("changed expiry verified = %v, want false", verifyR2TestSignature(request, []byte("r2-secret-key")))
		}
	})
}
