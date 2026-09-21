package gcsobjects

import (
	"context"
	"encoding/base64"
	json "encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"cloud.google.com/go/auth/credentials"
	"cloud.google.com/go/storage"
	"github.com/deliri/primitive/v2026/core"
	"google.golang.org/api/iamcredentials/v1"
)

// Real credential-file constructor, OAuth exchange and official IAM SDK.
// Local endpoints simulate scope enforcement and provider refusal. The returned
// signature is synthetic and is never spent against Cloud Storage.
func TestGCSCapabilityAuthenticationLayerTriadPreservesSigningScope(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr   error
		name      string
		wantCalls uint64
		refused   bool
		canceled  bool
	}{
		{name: "authenticated signing carries storage and IAM usable scope", wantCalls: 1},
		{name: "provider refusal releases no upload capability", refused: true, wantCalls: 1, wantErr: core.ErrObjectStoreDestination},
		{name: "canceled issuance performs no token or signing requests", canceled: true, wantErr: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			var tokenCalls, signingCalls atomic.Uint64
			var scopesMatch atomic.Bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/token" {
					tokenCalls.Add(1)
					if err := r.ParseForm(); err != nil {
						t.Errorf("token form error = %v, want nil", err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					parts := strings.Split(r.Form.Get("assertion"), ".")
					if len(parts) != 3 {
						t.Errorf("OAuth assertion parts = %d, want 3", len(parts))
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					payload, err := base64.RawURLEncoding.DecodeString(parts[1])
					var claims struct {
						Scope string `json:"scope"`
					}
					if err != nil || json.Unmarshal(payload, &claims) != nil {
						t.Errorf("OAuth assertion decode error = %v, want valid scope claims", err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					scopes := strings.Fields(claims.Scope)
					scopesMatch.Store(slices.Contains(scopes, storage.ScopeFullControl) && slices.Contains(scopes, iamcredentials.CloudPlatformScope))
					w.Header().Set("Content-Type", "application/json")
					response := struct {
						AccessToken string `json:"access_token"`
						TokenType   string `json:"token_type"`
						ExpiresIn   int    `json:"expires_in"`
					}{AccessToken: "synthetic-scoped-token", TokenType: "Bearer", ExpiresIn: 3600}
					if err := json.MarshalWrite(w, response); err != nil {
						t.Errorf("token response error = %v, want nil", err)
					}
					return
				}
				signingCalls.Add(1)
				if !scopesMatch.Load() || tc.refused {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, ":signBlob") || r.Header.Get("Authorization") != "Bearer synthetic-scoped-token" {
					t.Errorf("signing method/path/auth = %s/%s/%t, want POST/signBlob/authenticated", r.Method, r.URL.Path, r.Header.Get("Authorization") == "Bearer synthetic-scoped-token")
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				var request iamcredentials.SignBlobRequest
				if err := json.UnmarshalRead(r.Body, &request); err != nil || request.Payload == "" {
					t.Errorf("signing payload error/nonempty = %v/%t, want nil/true", err, request.Payload != "")
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				if err := json.MarshalWrite(w, &iamcredentials.SignBlobResponse{SignedBlob: base64.StdEncoding.EncodeToString([]byte("synthetic-signature"))}); err != nil {
					t.Errorf("signing response error = %v, want nil", err)
				}
			}))
			t.Cleanup(server.Close)
			document := gcsServiceAccountCredentialDocument{Type: credentials.ServiceAccount, ProjectID: "primitive-provider-proof", PrivateKeyID: "provider-owned-key-id", PrivateKey: string(canonicalGCSCredentialPrivateKey(t)), ClientEmail: "provider-proof@primitive-provider-proof.iam.gserviceaccount.com", TokenURI: server.URL + "/token"}
			encoded, err := json.Marshal(document)
			if err != nil {
				t.Fatalf("credential marshal error = %v, want nil", err)
			}
			filename := filepath.Join(dir, "credential.json")
			if err := os.WriteFile(filename, encoded, 0o600); err != nil {
				t.Fatalf("credential write error = %v, want nil", err)
			}
			absolute, err := core.ParseAbsolutePath(filename)
			if err != nil {
				t.Fatalf("credential path error = %v, want nil", err)
			}
			issuer, err := NewGCSCapabilityIssuer(t.Context(), GCSClientConfig{Authentication: GCSAuthenticationServiceAccountFile, CredentialFile: absolute})
			if err != nil {
				t.Fatalf("NewGCSCapabilityIssuer() error = %v, want nil", err)
			}
			t.Cleanup(func() {
				if err := issuer.Close(); err != nil {
					t.Errorf("issuer.Close() error = %v, want nil", err)
				}
			})
			issuer.service.BasePath = server.URL + "/"
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.canceled {
				cancel()
			}
			got, err := IssueGCSUploadCapability(ctx, issuer, gcsCapabilityRequest(t))
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("IssueGCSUploadCapability() error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if !got.IsZero() {
					t.Error("refused capability unset = false, want true")
				}
			} else if got.IsZero() || got.Validate() != nil {
				t.Error("accepted capability valid = false, want true")
			}
			if tokenCalls.Load() != tc.wantCalls || signingCalls.Load() != tc.wantCalls {
				t.Errorf("token/signing calls = %d/%d, want %d/%d", tokenCalls.Load(), signingCalls.Load(), tc.wantCalls, tc.wantCalls)
			}
			if tc.wantCalls != 0 && !scopesMatch.Load() {
				t.Error("OAuth scope covers storage and IAM = false, want true")
			}
		})
	}
}
