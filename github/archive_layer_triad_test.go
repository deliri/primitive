package github

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestGitHubTarArchiveTransportLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive exact archive streams once and seals its accepted bytes", func(t *testing.T) {
		t.Parallel()

		content := []byte("typed tar archive bytes")
		var archiveCalls atomic.Uint64
		server := archiveServer(t, content, &archiveCalls)
		defer server.Close()

		client := clientFixture(t, server.URL)
		var destination bytes.Buffer
		got, gotErr := client.ReadTarArchive(context.Background(), TarArchiveRequest{
			Destination: &destination, Repository: parsedRepository(t, "owner/repository"),
			Commit: parsedCommit(t), MaximumBytes: byteCountFixture(t, uint64(len(content))),
		})
		if gotErr != nil || got.State != ArchiveTransferComplete || got.Length.Uint64() != uint64(len(content)) ||
			got.SHA256 != core.SHA256Of(content) || !bytes.Equal(destination.Bytes(), content) {
			t.Fatalf("Client.ReadTarArchive(exact) = (%+v, %v, %q), want complete %d-byte digest-bound transfer", got, gotErr, destination.Bytes(), len(content))
		}
		if archiveCalls.Load() != 1 {
			t.Fatalf("archive transfer calls = %d, want 1", archiveCalls.Load())
		}
	})

	t.Run("negative one byte above caller ceiling remains typed partial evidence", func(t *testing.T) {
		t.Parallel()

		content := []byte("ceiling-plus-one")
		maximum := uint64(len(content) - 1)
		var archiveCalls atomic.Uint64
		server := archiveServer(t, content, &archiveCalls)
		defer server.Close()

		client := clientFixture(t, server.URL)
		var destination bytes.Buffer
		got, gotErr := client.ReadTarArchive(context.Background(), TarArchiveRequest{
			Destination: &destination, Repository: parsedRepository(t, "owner/repository"),
			Commit: parsedCommit(t), MaximumBytes: byteCountFixture(t, maximum),
		})
		wantBytes := content[:maximum]
		if !errors.Is(gotErr, core.ErrExchangeBodyLimit) || got.State != ArchiveTransferIncomplete ||
			got.Length.Uint64() != maximum || got.SHA256 != core.SHA256Of(wantBytes) || !bytes.Equal(destination.Bytes(), wantBytes) {
			t.Fatalf("Client.ReadTarArchive(over ceiling) = (%+v, %v, %q), want incomplete %d-byte evidence and %v", got, gotErr, destination.Bytes(), maximum, core.ErrExchangeBodyLimit)
		}
		if archiveCalls.Load() != 1 {
			t.Fatalf("oversized archive transfer calls = %d, want 1", archiveCalls.Load())
		}
	})

	t.Run("neutral empty provider body creates no archive and cannot become completion", func(t *testing.T) {
		t.Parallel()

		var archiveCalls atomic.Uint64
		server := archiveServer(t, nil, &archiveCalls)
		defer server.Close()

		client := clientFixture(t, server.URL)
		var destination bytes.Buffer
		got, gotErr := client.ReadTarArchive(context.Background(), TarArchiveRequest{
			Destination: &destination, Repository: parsedRepository(t, "owner/repository"),
			Commit: parsedCommit(t), MaximumBytes: byteCountFixture(t, 1),
		})
		if !errors.Is(gotErr, core.ErrGitHubResponse) || got.State != ArchiveTransferIncomplete ||
			got.Length.Uint64() != 0 || got.SHA256 != core.SHA256Of(nil) || destination.Len() != 0 {
			t.Fatalf("Client.ReadTarArchive(empty) = (%+v, %v, %d destination bytes), want zero-byte incomplete evidence and %v", got, gotErr, destination.Len(), core.ErrGitHubResponse)
		}
		if archiveCalls.Load() != 1 {
			t.Fatalf("empty archive transfer calls = %d, want 1", archiveCalls.Load())
		}
	})
}

func TestGitHubTarArchiveAuthenticationStopsAtTheDocumentedAPIBoundary(t *testing.T) {
	t.Parallel()

	fixedNow := time.Date(2026, time.September, 3, 10, 0, 0, 0, time.UTC)
	observation, err := temporal.NewObservation(fixedNow)
	if err != nil {
		t.Fatalf("temporal.NewObservation(fixed) error = %v, want nil", err)
	}
	content := []byte("private archive bytes")
	var tokenCalls atomic.Uint64
	var archiveCalls atomic.Uint64
	var server *httptest.Server
	archiveAPIPath := "/repos/owner/repository/tarball/" + parsedCommit(t).String()
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, incoming *http.Request) {
		switch incoming.URL.Path {
		case "/app/installations/2/access_tokens":
			tokenCalls.Add(1)
			if verifyErr := verifyJWTFixture(incoming.Header.Get("Authorization"), fixedNow); verifyErr != nil {
				t.Errorf("installation JWT verification error = %v, want nil", verifyErr)
			}
			writeJSON(t, writer, struct {
				Token     string `json:"token"`
				ExpiresAt string `json:"expires_at"`
			}{Token: "ghs_fixed", ExpiresAt: fixedNow.Add(time.Hour).Format(time.RFC3339)}, http.StatusCreated)
		case archiveAPIPath:
			if got := incoming.Header.Get("Authorization"); got != "Bearer ghs_fixed" {
				t.Errorf("archive API Authorization = %q, want installation bearer", got)
			}
			writer.Header().Set(headerLocation, server.URL+"/temporary-private-archive")
			writer.WriteHeader(http.StatusFound)
		case "/temporary-private-archive":
			archiveCalls.Add(1)
			if got := incoming.Header.Get("Authorization"); got != "" {
				t.Errorf("temporary archive Authorization = %q, want absent", got)
			}
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write(content)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := authenticatedClientFixture(t, server.URL, observation)
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			t.Errorf("authenticated Client.Close() error = %v, want nil", closeErr)
		}
	}()
	var destination bytes.Buffer
	got, gotErr := client.ReadTarArchive(context.Background(), TarArchiveRequest{
		Destination: &destination, Repository: parsedRepository(t, "owner/repository"),
		Commit: parsedCommit(t), MaximumBytes: byteCountFixture(t, uint64(len(content))),
	})
	if gotErr != nil || got.State != ArchiveTransferComplete || !bytes.Equal(destination.Bytes(), content) || tokenCalls.Load() != 1 || archiveCalls.Load() != 1 {
		t.Fatalf("authenticated ReadTarArchive() = (%+v, %v, %q, token calls %d, archive calls %d), want complete private transfer with one call per boundary", got, gotErr, destination.Bytes(), tokenCalls.Load(), archiveCalls.Load())
	}
}

func archiveServer(t testing.TB, content []byte, archiveCalls *atomic.Uint64) *httptest.Server {
	t.Helper()
	var server *httptest.Server
	archiveAPIPath := "/repos/owner/repository/tarball/" + parsedCommit(t).String()
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, incoming *http.Request) {
		switch incoming.URL.Path {
		case archiveAPIPath:
			if incoming.Header.Get(headerAPIVersion) != core.GitHubAPIVersion || incoming.Header.Get(headerUserAgent) != "primitive-test" {
				t.Errorf("archive API headers = %v, want version and caller identity", incoming.Header)
			}
			writer.Header().Set(headerLocation, server.URL+"/archive")
			writer.WriteHeader(http.StatusFound)
		case "/archive":
			archiveCalls.Add(1)
			if got := incoming.Header.Get("Authorization"); got != "" {
				t.Errorf("archive download Authorization = %q, want absent", got)
			}
			writer.WriteHeader(http.StatusOK)
			if flusher, ok := writer.(http.Flusher); ok {
				flusher.Flush()
			}
			_, _ = writer.Write(content)
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	return server
}

func authenticatedClientFixture(t testing.TB, authorityText string, observation temporal.Observation) Client {
	t.Helper()
	transport, err := exchange.NewStandardClient()
	if err != nil {
		t.Fatalf("exchange.NewStandardClient() error = %v, want nil", err)
	}
	userAgent, err := ParseUserAgent("primitive-test")
	if err != nil {
		t.Fatalf("ParseUserAgent() error = %v, want nil", err)
	}
	app, appErr := NewAppID(1)
	installation, installationErr := NewInstallationID(2)
	credential, credentialErr := NewAppCredential(app, installation, []byte(fixedRSAPrivateKey))
	if err := errors.Join(appErr, installationErr, credentialErr); err != nil {
		t.Fatalf("App credential fixture error = %v, want nil", err)
	}
	defer func() {
		if closeErr := credential.Close(); closeErr != nil {
			t.Errorf("fixture AppCredential.Close() error = %v, want nil", closeErr)
		}
	}()
	authority, err := core.ParseHTTPEndpoint(authorityText)
	if err != nil {
		t.Fatalf("core.ParseHTTPEndpoint(%q) error = %v, want nil", authorityText, err)
	}
	client, err := newClient(clientConstruction{
		client: transport, authority: authority, userAgent: userAgent, credential: credential,
		observe: func() (temporal.Observation, error) { return observation, nil },
	})
	if err != nil {
		t.Fatalf("newClient(authenticated fixture) error = %v, want nil", err)
	}
	return client
}
