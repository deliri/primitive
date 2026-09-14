package gcsobjects

import (
	"context"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/testserial"
	"google.golang.org/api/googleapi"
	iamcredentials "google.golang.org/api/iamcredentials/v1"
)

func TestGCSClientShutdownLayerTriadClosesRealProviderConnections(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardProcessEnvironment, Scope: core.TestIsolationScopePackageProcess})
	directory := t.TempDir()
	var opened atomic.Int64
	var requests atomic.Int64
	closed := make(chan struct{}, 32)
	server := httptest.NewUnstartedServer(gcsCredentialTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		writeGoogleAPIError(w, http.StatusNotFound)
	})))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		switch state {
		case http.StateNew:
			opened.Add(1)
		case http.StateClosed:
			closed <- struct{}{}
		}
	}
	server.Start()
	t.Cleanup(server.Close)
	t.Setenv("STORAGE_EMULATOR_HOST", server.URL)
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", gcsLocalCredentialFile(t, directory, server.URL))
	client, err := NewGCSClient(t.Context(), GCSClientConfig{Authentication: GCSAuthenticationApplicationDefault})
	if err != nil {
		t.Fatalf("NewGCSClient() error = %v, want nil", err)
	}
	t.Cleanup(func() {
		if client.Validate() == nil {
			if err := client.Close(); err != nil {
				t.Errorf("cleanup Close() error = %v, want nil", err)
			}
		}
	})
	bucket, err := ParseGCSBucket(gcsProviderBucketText)
	if err != nil {
		t.Fatalf("ParseGCSBucket() error = %v, want nil", err)
	}
	name, err := ParseGCSObjectName(gcsProviderObjectText)
	if err != nil {
		t.Fatalf("ParseGCSObjectName() error = %v, want nil", err)
	}
	got, err := LookupGCSObject(t.Context(), client, GCSObjectLookupRequest{Bucket: bucket, Name: name})
	if !errors.Is(err, core.ErrObjectStoreAbsent) || got != (GCSObjectMetadata{}) || requests.Load() != 1 {
		t.Fatalf("LookupGCSObject() metadata/error/requests = %v/%v/%d, want zero/%v/1", got, err, requests.Load(), core.ErrObjectStoreAbsent)
	}
	wantClosed := opened.Load()
	if wantClosed == 0 {
		t.Fatal("opened provider connections = 0, want nonzero before close")
	}
	if err := client.Close(); err != nil {
		t.Fatalf("Close() error = %v, want nil", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	for count := int64(0); count < wantClosed; count++ {
		select {
		case <-closed:
		case <-ctx.Done():
			t.Fatalf("closed provider connections = %d, want %d after client close", count, wantClosed)
		}
	}
	for _, candidate := range []*GCSClient{nil, {}, client} {
		if err := candidate.Close(); !errors.Is(err, core.ErrObjectStoreContract) {
			t.Fatalf("Close(unconstructed or closed) error = %v, want %v", err, core.ErrObjectStoreContract)
		}
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("requests after repeated close = %d, want 1", got)
	}
}

// The public constructor and Close own the real SDK transport. The SDK call
// below is an explicit direct signing-leaf probe, not upload-capability proof.
func TestGCSCapabilityIssuerShutdownLayerTriadClosesSigningConnections(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardProcessEnvironment, Scope: core.TestIsolationScopePackageProcess})
	directory := t.TempDir()
	var opened atomic.Int64
	var requests atomic.Int64
	closed := make(chan struct{}, 32)
	server := httptest.NewUnstartedServer(gcsCredentialTestHandler(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		writeGoogleAPIError(w, http.StatusNotFound)
	})))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		switch state {
		case http.StateNew:
			opened.Add(1)
		case http.StateClosed:
			closed <- struct{}{}
		}
	}
	server.Start()
	t.Cleanup(server.Close)
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", gcsLocalCredentialFile(t, directory, server.URL))
	issuer, err := NewGCSCapabilityIssuer(t.Context(), GCSClientConfig{Authentication: GCSAuthenticationApplicationDefault})
	if err != nil {
		t.Fatalf("NewGCSCapabilityIssuer() error = %v, want nil", err)
	}
	t.Cleanup(func() {
		if issuer.Validate() == nil {
			if err := issuer.Close(); err != nil {
				t.Errorf("cleanup Close() error = %v, want nil", err)
			}
		}
	})
	issuer.service.BasePath = server.URL + "/"
	response, err := issuer.service.Projects.ServiceAccounts.SignBlob(
		gcsSignBlobResourcePrefix+"example-project@appspot.gserviceaccount.com",
		&iamcredentials.SignBlobRequest{Payload: base64.StdEncoding.EncodeToString([]byte("owned signing request"))},
	).Context(t.Context()).Do()
	var refusal *googleapi.Error
	if !errors.As(err, &refusal) || refusal.Code != http.StatusNotFound || response != nil || requests.Load() != 1 {
		t.Fatalf("SignBlob() response/error/requests = %v/%v/%d, want nil/typed 404/1", response, err, requests.Load())
	}
	wantClosed := opened.Load()
	if wantClosed == 0 {
		t.Fatal("opened signing connections = 0, want nonzero before close")
	}
	if err := issuer.Close(); err != nil {
		t.Fatalf("Close() error = %v, want nil", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	for count := int64(0); count < wantClosed; count++ {
		select {
		case <-closed:
		case <-ctx.Done():
			t.Fatalf("closed signing connections = %d, want %d after issuer close", count, wantClosed)
		}
	}
	for _, candidate := range []*GCSCapabilityIssuer{nil, {}, issuer} {
		if err := candidate.Close(); !errors.Is(err, core.ErrObjectStoreContract) {
			t.Fatalf("Close(unconstructed or closed issuer) error = %v, want %v", err, core.ErrObjectStoreContract)
		}
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("requests after repeated issuer close = %d, want 1", got)
	}
}
