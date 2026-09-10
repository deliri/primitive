package tailnet

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"net/http"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/googleidentity"
	tailscale "tailscale.com/client/tailscale/v2"
)

type suppliedIdentity struct{ token googleidentity.Token }

func (s suppliedIdentity) Identity(context.Context, googleidentity.Audience) (googleidentity.Token, error) {
	return s.token, nil
}

func TestEarlySDKFilesystemFailureReturnsError(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		nested bool
	}{
		{"state path is a regular file", false},
		{"state parent is a regular file", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			path, err := core.ParseAbsolutePath(filepath.Join(directory, "state-file"))
			if err != nil {
				t.Fatalf("state path = %v, want nil", err)
			}
			location, err := filestore.OpenParent(t.Context(), path)
			if err != nil {
				t.Fatalf("OpenParent() = %v, want nil", err)
			}
			t.Cleanup(func() {
				if err := location.Root.Close(); err != nil {
					t.Errorf("root close = %v, want nil", err)
				}
			})
			temporary, err := core.ParseRelativePath("state.tmp")
			if err != nil {
				t.Fatalf("temporary path = %v, want nil", err)
			}
			if _, err := filestore.Write(t.Context(), filestore.WriteRequest{Location: location, Temporary: temporary, Mode: 0600, Install: filestore.InstallCreate, Source: bytes.NewReader([]byte{'x'})}); err != nil {
				t.Fatalf("fixture Write() = %v, want nil", err)
			}
			configuration := fixtureConfiguration(t)
			configuration.StateDirectory = path
			if tc.nested {
				configuration.StateDirectory, err = core.ParseAbsolutePath(filepath.Join(path.String(), "child"))
				if err != nil {
					t.Fatalf("nested state path = %v, want nil", err)
				}
			}
			key := tailscale.Key{Key: fixtureAuthKey}
			key.Capabilities.Devices.Create.Ephemeral = true
			key.Capabilities.Devices.Create.Tags = []string{configuration.Tag.String()}
			keyBody, err := core.MarshalCanonicalJSONDocument(key)
			if err != nil {
				t.Fatalf("key encoding = %v, want nil", err)
			}
			tokenBody, err := core.MarshalCanonicalJSONDocument(fixtureAccessResponse{AccessToken: fixtureAccessToken, TokenType: exchange.BearerAuthorizationScheme, ExpiresIn: 60})
			if err != nil {
				t.Fatalf("token encoding = %v, want nil", err)
			}
			client, err := NewClient(configuration, suppliedIdentity{token: fixtureIdentityToken(t)})
			if err != nil {
				t.Fatalf("NewClient() = %v, want nil", err)
			}
			t.Cleanup(func() {
				if err := client.Close(); err != nil {
					t.Errorf("client Close() = %v, want nil", err)
				}
			})
			client.enrollment = localEnrollmentClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body := keyBody
				if r.URL.Path == tokenExchangePath {
					body = tokenBody
				}
				w.Header().Set("Content-Type", "application/json")
				if _, err := w.Write(body); err != nil {
					t.Errorf("provider Write() = %v, want nil", err)
				}
			}))
			got, err := client.acquire(t.Context())
			pathErr, hasPath := errors.AsType[*fs.PathError](err)
			if got != nil || client.server != nil || !errors.Is(err, core.ErrTailnetEnrollment) || !errors.Is(err, syscall.ENOTDIR) || !hasPath {
				t.Fatalf("acquire() = (%p,%v,pathError=%t), want no server and typed ENOTDIR", got, err, hasPath)
			}
			if pathErr.Path != configuration.StateDirectory.String() && pathErr.Path != path.String() {
				t.Fatalf("failed path = %q, want authored state path or its file parent", pathErr.Path)
			}
		})
	}
}
