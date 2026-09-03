package github

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	json "encoding/json/v2"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const fixedRSAPrivateKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEA8LjMP9WwXygYqR7HoO5cgeiSbh6/VbAKxcs585h5cZBHd0S8
9k5Gkdi9f0v0fo9rbXXP8nwPBCoOg381jeTBoXabR9AxACkm4kgxFWShqnYdakQO
w8rjkONn0Dm+V4g2fi4eZ1DxZnhz4njE/LcCPIhTprV/f4/aQBV7H4oWrqlCZ1xg
GYiah50m+iQ/GKloZgG4apubMx/d9p5nL0IqUBmc70SvHpNeLAyN0Bdq8uVCxzgR
zaybGEUZRP2S8/n9WCev0pyRHlLlOBZzWfwXr3Scl+tAzNmhu+KgkqPsfyFA1vvb
3IiJKmDipvtGegqcLI3pijaPRv9gXRLvyJrlRwIDAQABAoIBAEZvTBRBimHNcanK
f87u79JzIqVmCcYgxIYreMF2E9LOzJpxWnkXXj6+lHPy3Y9Kl7xnhHkHI72sMKL5
Tco+7Qk5kyXoHO5XHDGJvhLsZwFhninB0DAp5Xw3jeC3hKJIEOnKxMqmPHwnMoFJ
pRns0pKzsQZOhQfmJ44ouuX3mbtw6dE2LY1kKmheBv0xwEdR1pMbByWSbXRH22DV
355TD+GI245YQHIEqZOX5SjMDxwSTfQ1VfFetOnY5W98gwTnoZ2dU3P3N0CB0xvo
gnTdFJ0XkeYYdWeatrM+opDr4GvDr/lSCokv5wKzs1LKy+OCNU1TbLmdod5aNVom
FnqGoIECgYEA/yPKV1wyg1MZzKFFfC/O6x8lC8XOt06mbrnuTbY/NU66oBfcif7a
+YbZaX5FTwdQqa1mbuQRAO6nxB7WiKj24XHb+G/NLuGT8Bdg2A+usbd0eU3Mods5
ILfY7rk7K1QdKkGLPEUTv4t+0hKB0j0/iEOP4kVBu7TcQtDqiNU/69kCgYEA8YiQ
PCtsoM9eFpfD/P6cyXdT8Krd8BW+wJwLMakJLOhDXYGYb1HZIf+jzlYTlCzsXFn2
PXg7E+hIOPLcx9gqpRFZsWNO94ww3XLKWQCdzJMgFaZ4ZKFPySIVlCorwPWPoEoD
GgxerWP//sO0iDtSz1nDeXpuiDk5mc4B8hpgRh8CgYAMbzgbTdkAYXpuaKW0Sbgx
6VCq5DcQ4/pkhxdAHlOyS2X5C3CqIQuXAaVy6L6D/X1G57aITQEvJHJ0snQOMP3n
Ot9XmktLr57AIsOLhCglbSV2C/6fHMoJ+CvQZqKll/Hb71nT1CIEQc4qetBs6KNC
BtjqVCnB9iyN7RShGpOE8QKBgQCpEQB1PagyADVJ9z2277prw108Tz4++dmmFRQ4
1KuZhZLx9u7urQoiJEFTAyl9RNzF4Cre6DPiQWucgVNNh+CB3t07r9nsqXLi76D4
H9hVBH8m6HnJZqjkjzkvlz09OiYo+uWk7BexoxfkCrVpzqyue5S6iZqpO/U31d3C
y/er3QKBgDuFL+pVlClSQeqOO89s0vvOn0QXpQJjknuAj8DvsioO2O635uMojD4U
wGIALp90B3rxF/CM52tQljuOUza4vSoG2wBNNkqTBlfkzP77Slp1OXVRb13ou/Tl
Cf1H4fZfzVDbDuhxWVwY2pUmGi1CXkhOTlYwUm9o4gfXmCtYBbPG
-----END RSA PRIVATE KEY-----` // #nosec G101 -- deterministic non-production fixture.

func TestGitHubContentsTransportLayerTriad(t *testing.T) {
	t.Parallel()

	tests := []struct {
		wantErr     error
		name        string
		wantContent string
		response    contentsWire
	}{
		{
			name:        "positive exact file crosses public socket",
			response:    contentsWire{Path: "source/main.go", Size: 12, Type: "file", Encoding: "base64", Content: base64.StdEncoding.EncodeToString([]byte("package main"))},
			wantContent: "package main",
		},
		{
			name:     "negative mismatched path releases no file",
			response: contentsWire{Path: "other.go", Size: 12, Type: "file", Encoding: "base64", Content: base64.StdEncoding.EncodeToString([]byte("package main"))},
			wantErr:  core.ErrGitHubResponse,
		},
		{
			name:        "neutral empty file remains exact empty evidence",
			response:    contentsWire{Path: "source/main.go", Size: 0, Type: "file", Encoding: "base64", Content: ""},
			wantContent: "",
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, incoming *http.Request) {
				if incoming.URL.Path != "/repos/owner/repository/contents/source/main.go" || incoming.URL.Query().Get("ref") != parsedCommit(t).String() ||
					incoming.Header.Get(headerAPIVersion) != core.GitHubAPIVersion || incoming.Header.Get(headerUserAgent) != "primitive-test" || incoming.Header.Get("Authorization") != "" {
					t.Errorf("GitHub file request = %s?%s headers=%v, want exact public provider agreement", incoming.URL.Path, incoming.URL.RawQuery, incoming.Header)
				}
				writeJSON(t, writer, testCase.response, http.StatusOK)
			}))
			defer server.Close()

			client := clientFixture(t, server.URL)
			maximum := byteCountFixture(t, core.GitHubContentsInlineMaximumBytes)
			got, gotErr := client.ReadFile(context.Background(), FileRequest{
				Repository: parsedRepository(t, "owner/repository"), Commit: parsedCommit(t),
				Path: parsedPath(t, "source/main.go"), MaximumBytes: maximum,
			})
			if !errors.Is(gotErr, testCase.wantErr) {
				t.Fatalf("Client.ReadFile() error = %v, want %v", gotErr, testCase.wantErr)
			}
			if string(got.Content) != testCase.wantContent {
				t.Fatalf("Client.ReadFile().Content = %q, want %q", got.Content, testCase.wantContent)
			}
		})
	}
}

func TestGitHubTagAndHeadTransportLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive tag continuation and head retain exact commits", func(t *testing.T) {
		t.Parallel()

		commit := parsedCommit(t)
		var server *httptest.Server
		server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, incoming *http.Request) {
			switch incoming.URL.Path {
			case "/repos/owner/repository/tags":
				// GitHub emits its numeric repository resource path in real Link
				// headers even when the request used the owner/name path.
				next := server.URL + "/repositories/1315532978/tags?per_page=100&page=2"
				writer.Header().Set(headerLink, "<"+next+`>; rel="next"`)
				writeJSON(t, writer, []tagWire{{Name: "anything-product-owned", Commit: tagCommitWire{SHA: commit.String()}}}, http.StatusOK)
			case "/repos/owner/repository/commits":
				writeJSON(t, writer, []headWire{{SHA: commit.String()}}, http.StatusOK)
			default:
				t.Errorf("GitHub request path = %q, want tags or commits", incoming.URL.Path)
				writer.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		client := clientFixture(t, server.URL)
		repository := parsedRepository(t, "owner/repository")
		page, pageErr := client.ReadTagPage(context.Background(), TagPageRequest{Repository: repository, Page: 1})
		head, headErr := client.ReadHead(context.Background(), HeadRequest{Repository: repository})
		if pageErr != nil || headErr != nil || page.NextPage != 2 || len(page.Tags) != 1 || page.Tags[0].Commit != commit || head.Commit != commit {
			t.Fatalf("tag/head observations = (%+v, %+v) errors=(%v, %v), want exact commit and next page 2", page, head, pageErr, headErr)
		}
	})

	t.Run("negative foreign continuation is a typed binding refusal", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set(headerLink, `<https://example.invalid/repos/owner/repository/tags?page=2&per_page=100>; rel="next"`)
			writeJSON(t, writer, []tagWire{}, http.StatusOK)
		}))
		defer server.Close()

		client := clientFixture(t, server.URL)
		got, gotErr := client.ReadTagPage(context.Background(), TagPageRequest{Repository: parsedRepository(t, "owner/repository"), Page: 1})
		if !errors.Is(gotErr, core.ErrGitHubBinding) || got.Tags != nil {
			t.Fatalf("Client.ReadTagPage(foreign next) = (%+v, %v), want zero and %v", got, gotErr, core.ErrGitHubBinding)
		}
	})

	t.Run("neutral empty tag page has no invented continuation", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writeJSON(t, writer, []tagWire{}, http.StatusOK)
		}))
		defer server.Close()

		client := clientFixture(t, server.URL)
		got, gotErr := client.ReadTagPage(context.Background(), TagPageRequest{Repository: parsedRepository(t, "owner/repository"), Page: 1})
		if gotErr != nil || len(got.Tags) != 0 || got.NextPage != 0 {
			t.Fatalf("Client.ReadTagPage(empty) = (%+v, %v), want empty page without continuation", got, gotErr)
		}
	})
}

func TestGitHubAppAuthenticationLayerTriad(t *testing.T) {
	t.Parallel()

	fixedNow := time.Date(2026, time.September, 3, 10, 0, 0, 0, time.UTC)
	observation, err := temporal.NewObservation(fixedNow)
	if err != nil {
		t.Fatalf("temporal.NewObservation(fixed) error = %v, want nil", err)
	}
	var tokenCalls atomic.Uint32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, incoming *http.Request) {
		switch incoming.URL.Path {
		case "/app/installations/2/access_tokens":
			tokenCalls.Add(1)
			if err := verifyJWTFixture(incoming.Header.Get("Authorization"), fixedNow); err != nil {
				t.Errorf("installation JWT verification error = %v, want nil", err)
			}
			writeJSON(t, writer, struct {
				Token     string `json:"token"`
				ExpiresAt string `json:"expires_at"`
			}{Token: "ghs_fixed", ExpiresAt: fixedNow.Add(time.Hour).Format(time.RFC3339)}, http.StatusCreated)
		case "/repos/owner/repository/commits":
			if incoming.Header.Get("Authorization") != "Bearer ghs_fixed" {
				t.Errorf("repository Authorization = %q, want installation bearer", incoming.Header.Get("Authorization"))
			}
			writeJSON(t, writer, []headWire{{SHA: parsedCommit(t).String()}}, http.StatusOK)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	transport, err := exchange.NewStandardClient()
	if err != nil {
		t.Fatalf("exchange.NewStandardClient() error = %v, want nil", err)
	}
	userAgent, err := ParseUserAgent("primitive-test")
	if err != nil {
		t.Fatalf("ParseUserAgent() error = %v, want nil", err)
	}
	app, err := NewAppID(1)
	if err != nil {
		t.Fatalf("NewAppID(1) error = %v, want nil", err)
	}
	installation, err := NewInstallationID(2)
	if err != nil {
		t.Fatalf("NewInstallationID(2) error = %v, want nil", err)
	}
	credential, err := NewAppCredential(app, installation, []byte(fixedRSAPrivateKey))
	if err != nil {
		t.Fatalf("NewAppCredential() error = %v, want nil", err)
	}
	authority, err := core.ParseHTTPEndpoint(server.URL)
	if err != nil {
		t.Fatalf("core.ParseHTTPEndpoint(server) error = %v, want nil", err)
	}
	client, err := newClient(clientConstruction{
		client: transport, authority: authority, userAgent: userAgent, credential: credential,
		observe: func() (temporal.Observation, error) { return observation, nil },
	})
	if err != nil {
		t.Fatalf("newClient(app fixture) error = %v, want nil", err)
	}
	if err := credential.Close(); err != nil {
		t.Fatalf("original AppCredential.Close() error = %v, want nil after client-owned copy", err)
	}
	request := HeadRequest{Repository: parsedRepository(t, "owner/repository")}
	first, firstErr := client.ReadHead(context.Background(), request)
	second, secondErr := client.ReadHead(context.Background(), request)
	if firstErr != nil || secondErr != nil || first != second || tokenCalls.Load() != 1 {
		t.Fatalf("two authenticated heads = (%+v/%v, %+v/%v, token calls %d), want equal, nil, and one token", first, firstErr, second, secondErr, tokenCalls.Load())
	}
	if err := client.Close(); err != nil || client.Validate() == nil {
		t.Fatalf("Client.Close()/Validate() = (%v, %v), want nil then invalid", err, client.Validate())
	}

	t.Run("negative malformed private key cannot create authentication capability", func(t *testing.T) {
		t.Parallel()

		app, appErr := NewAppID(1)
		installation, installationErr := NewInstallationID(2)
		got, gotErr := NewAppCredential(app, installation, []byte("not a PEM key"))
		if appErr != nil || installationErr != nil || !errors.Is(gotErr, core.ErrGitHubAuthentication) || got.state != nil {
			t.Fatalf("NewAppCredential(malformed) = (%v, %v), identity errors=(%v,%v), want zero and %v", got, gotErr, appErr, installationErr, core.ErrGitHubAuthentication)
		}
	})

	t.Run("neutral public client emits no authentication fact", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, incoming *http.Request) {
			if got := incoming.Header.Get("Authorization"); got != "" {
				t.Errorf("public Authorization = %q, want absent", got)
			}
			writeJSON(t, writer, []headWire{{SHA: parsedCommit(t).String()}}, http.StatusOK)
		}))
		defer server.Close()

		client := clientFixture(t, server.URL)
		got, gotErr := client.ReadHead(context.Background(), HeadRequest{Repository: parsedRepository(t, "owner/repository")})
		if gotErr != nil || got.Commit != parsedCommit(t) {
			t.Fatalf("public Client.ReadHead() = (%v, %v), want exact commit and nil", got, gotErr)
		}
	})
}

func TestGitHubExportedClientConstructorsOwnProductionAuthority(t *testing.T) {
	t.Parallel()

	transport, err := exchange.NewStandardClient()
	if err != nil {
		t.Fatalf("exchange.NewStandardClient() error = %v, want nil", err)
	}
	userAgent, err := ParseUserAgent("primitive-test")
	if err != nil {
		t.Fatalf("ParseUserAgent() error = %v, want nil", err)
	}
	public, err := NewClient(transport, userAgent)
	if err != nil || public.Validate() != nil {
		t.Fatalf("NewClient() = (%v, %v), want production-authority client and nil", public, err)
	}
	if err := public.Close(); err != nil {
		t.Fatalf("public Client.Close() error = %v, want nil", err)
	}

	app, appErr := NewAppID(1)
	installation, installationErr := NewInstallationID(2)
	credential, credentialErr := NewAppCredential(app, installation, []byte(fixedRSAPrivateKey))
	if err := errors.Join(appErr, installationErr, credentialErr); err != nil {
		t.Fatalf("App credential fixture error = %v, want nil", err)
	}
	appClient, err := NewAppClient(transport, userAgent, credential)
	if err != nil || appClient.Validate() != nil {
		t.Fatalf("NewAppClient() = (%v, %v), want production-authority client and nil", appClient, err)
	}
	if err := credential.Close(); err != nil || appClient.Validate() != nil {
		t.Fatalf("credential ownership after NewAppClient() = (close:%v validate:%v), want independent copy", err, appClient.Validate())
	}
	if err := appClient.Close(); err != nil {
		t.Fatalf("App Client.Close() error = %v, want nil", err)
	}
}

func verifyJWTFixture(authorization string, now time.Time) error {
	encoded := strings.TrimPrefix(authorization, "Bearer ")
	parts := strings.Split(encoded, ".")
	if len(parts) != 3 || encoded == authorization {
		return errors.New("authorization is not a three-part bearer JWT")
	}
	claimsPayload, claimsErr := base64.RawURLEncoding.DecodeString(parts[1])
	signature, signatureErr := base64.RawURLEncoding.DecodeString(parts[2])
	if err := errors.Join(claimsErr, signatureErr); err != nil {
		return err
	}
	var claims jwtClaims
	if err := json.Unmarshal(claimsPayload, &claims, json.RejectUnknownMembers(true)); err != nil {
		return err
	}
	if claims.Issuer != 1 || int64(claims.IssuedAtUnixSeconds) != now.Add(-time.Minute).Unix() || int64(claims.ExpiresAtUnixSeconds) != now.Add(9*time.Minute).Unix() {
		return errors.New("JWT claims do not bind exact app and fixed time window")
	}
	block, _ := pem.Decode([]byte(fixedRSAPrivateKey))
	if block == nil {
		return errors.New("fixed RSA verifier key is malformed")
	}
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return err
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	return rsa.VerifyPKCS1v15(&privateKey.PublicKey, crypto.SHA256, digest[:], signature)
}

func clientFixture(t testing.TB, authorityText string) Client {
	t.Helper()
	transport, err := exchange.NewStandardClient()
	if err != nil {
		t.Fatalf("exchange.NewStandardClient() error = %v, want nil", err)
	}
	userAgent, err := ParseUserAgent("primitive-test")
	if err != nil {
		t.Fatalf("ParseUserAgent() error = %v, want nil", err)
	}
	authority, err := core.ParseHTTPEndpoint(authorityText)
	if err != nil {
		t.Fatalf("core.ParseHTTPEndpoint(%q) error = %v, want nil", authorityText, err)
	}
	client, err := newClient(clientConstruction{client: transport, authority: authority, userAgent: userAgent, observe: temporal.Observe})
	if err != nil {
		t.Fatalf("newClient(public fixture) error = %v, want nil", err)
	}
	return client
}

func byteCountFixture(t testing.TB, value uint64) core.ByteCount {
	t.Helper()
	got, err := core.NewByteCount(value)
	if err != nil {
		t.Fatalf("core.NewByteCount(%d) error = %v, want nil", value, err)
	}
	return got
}

func writeJSON[Value any](t testing.TB, writer http.ResponseWriter, value Value, status int) {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal(response fixture) error = %v, want nil", err)
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	if _, err := writer.Write(payload); err != nil {
		t.Fatalf("http.ResponseWriter.Write() error = %v, want nil", err)
	}
}
