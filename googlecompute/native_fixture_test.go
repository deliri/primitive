package googlecompute

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/googleidentity"
	"github.com/deliri/primitive/v2026/temporal"
	"google.golang.org/api/compute/v1"
)

type metadataWire struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// This is an explicit local HTTP provider double. Both official Compute and
// metadata SDKs use real sockets; only owned destinations are routable.
func nativeClient(t testing.TB, provider http.Handler) (Client, googleidentity.AccessToken, *atomic.Int64) {
	t.Helper()
	var calls atomic.Int64
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer compute-native-fixture" || r.Header.Get("Cookie") != "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		provider.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	metadata := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/computeMetadata/v1/instance/service-accounts/default/token" || r.Header.Get("Metadata-Flavor") != "Google" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Metadata-Flavor", "Google")
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(metadataWire{AccessToken: "compute-native-fixture", TokenType: "Bearer", ExpiresIn: 300}); err != nil {
			return
		}
	}))
	t.Cleanup(metadata.Close)
	httpClient := server.Client()
	transport := httpClient.Transport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.TLSClientConfig.ServerName = server.Certificate().DNSNames[0]
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		switch address {
		case "compute.googleapis.com:443":
			address = strings.TrimPrefix(server.URL, "https://")
		case "metadata.google.internal:80":
			address = strings.TrimPrefix(metadata.URL, "http://")
		default:
			return nil, core.ErrGoogleComputeBinding
		}
		return (&net.Dialer{}).DialContext(ctx, network, address)
	}
	httpClient.Transport = transport
	t.Cleanup(transport.CloseIdleConnections)
	network, err := exchange.NewClient(httpClient)
	if err != nil {
		t.Fatalf("Exchange client error = %v, want nil", err)
	}
	identity, err := googleidentity.NewClient(network)
	if err != nil {
		t.Fatalf("metadata client error = %v, want nil", err)
	}
	t.Cleanup(func() {
		if err := identity.CloseIdleConnections(); err != nil {
			t.Errorf("metadata Close error = %v, want nil", err)
		}
	})
	policy, err := googleidentity.DefaultPolicy()
	if err != nil {
		t.Fatalf("metadata policy error = %v, want nil", err)
	}
	token, err := googleidentity.AcquireGoogleCloudAccessToken(t.Context(), identity, googleidentity.GoogleCloudAccessTokenRequest{Policy: policy})
	if err != nil {
		t.Fatalf("metadata acquisition error = %v, want nil", err)
	}
	client, err := NewClient(t.Context(), network)
	if err != nil {
		t.Fatalf("Compute client error = %v, want nil", err)
	}
	return client, token, &calls
}

func nativeReference(t testing.TB) InstanceReference {
	t.Helper()
	project, pErr := NewResourceName("fixture-project")
	zone, zErr := NewResourceName("northamerica-northeast2-a")
	instance, iErr := NewResourceName("anvil")
	if err := errors.Join(pErr, zErr, iErr); err != nil {
		t.Fatalf("reference error = %v, want nil", err)
	}
	return InstanceReference{Project: project, Zone: zone, Instance: instance}
}
func nativePolicy(t testing.TB) exchange.OperationPolicy {
	t.Helper()
	timeout, err := temporal.DurationFromSeconds(20)
	if err != nil {
		t.Fatalf("timeout error = %v, want nil", err)
	}
	return exchange.OperationPolicy{OperationTimeout: timeout, AttemptTimeout: timeout, Retry: exchange.RetryPolicy{MaximumAttempts: 1}, Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject}}
}
func nativeOperation(t testing.TB, operationType string) *compute.Operation {
	t.Helper()
	r := nativeReference(t)
	return &compute.Operation{Name: "operation-123", TargetLink: "https://compute.googleapis.com" + instancePath(r), Zone: "https://compute.googleapis.com" + zonePath(r), OperationType: operationType, Status: "DONE"}
}
func nativeInstance(t testing.TB, status string) *compute.Instance {
	t.Helper()
	r := nativeReference(t)
	return &compute.Instance{Name: r.Instance.String(), SelfLink: "https://compute.googleapis.com" + instancePath(r), Zone: "https://compute.googleapis.com" + zonePath(r), Status: status}
}
