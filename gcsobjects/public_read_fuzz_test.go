package gcsobjects

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"cloud.google.com/go/iam"
	"cloud.google.com/go/storage"
	"github.com/deliri/primitive/v2026/core"
	"google.golang.org/api/option"
	storageapi "google.golang.org/api/storage/v1"
)

// FuzzGCSBucketPublicReadProviderPolicySemanticClosure mutates the real JSON
// response consumed by the official Cloud Storage SDK. Every admitted response
// must produce validated, call-counted policy evidence whose change agrees with
// an independent typed policy projection; every refusal must return zero sealed
// evidence with the stable provider-boundary identity.
func FuzzGCSBucketPublicReadProviderPolicySemanticClosure(f *testing.F) {
	for _, policy := range []storageapi.Policy{
		gcsPolicy("etag-empty"),
		gcsPolicy("etag-public", gcsPublicReadBinding(iam.AllUsers)),
		gcsPolicy("etag-member", gcsPublicReadBinding("serviceAccount:reader@example.test")),
		gcsPolicy("etag-owner", gcsPolicyBinding(iam.Owner, "serviceAccount:owner@example.test")),
		gcsPolicy("etag-repeated", gcsPublicReadBinding("serviceAccount:reader@example.test"), gcsPublicReadBinding(iam.AllUsers)),
		gcsPolicy("etag-conditional", &storageapi.PolicyBindings{
			Role:      string(gcsPublicReadRole),
			Members:   []string{iam.AllUsers},
			Condition: &storageapi.Expr{Expression: "request.time < timestamp('2030-01-01T00:00:00Z')"},
		}),
	} {
		encoded, err := json.Marshal(policy)
		if err != nil {
			f.Fatalf("json.Marshal(seed policy) error = %v, want nil", err)
		}
		f.Add(encoded)
	}
	for _, malformed := range [][]byte{
		nil,
		{},
		[]byte("{"),
		[]byte("null"),
		[]byte("[]"),
		[]byte(`{"bindings":"wrong type"}`),
		[]byte(`{"bindings":[null]}`),
	} {
		f.Add(malformed)
	}
	boundaryPolicy, err := json.Marshal(gcsPolicy("etag-boundary", gcsPublicReadBinding(iam.AllUsers)))
	if err != nil {
		f.Fatalf("json.Marshal(boundary policy) error = %v, want nil", err)
	}
	for _, extent := range []int{
		GCSProviderResponseMaximumBytes - 1,
		GCSProviderResponseMaximumBytes,
		GCSProviderResponseMaximumBytes + 1,
	} {
		f.Add(paddedGCSPolicyResponse(f, boundaryPolicy, extent))
	}

	f.Fuzz(func(t *testing.T, providerBytes []byte) {
		provider := &gcsPublicReadFuzzProvider{t: t, initial: providerBytes}
		client := gcsPublicReadFuzzClient(t, provider)
		bucket := parsedGCSBucket(t, gcsProviderBucketText)
		got, gotErr := GrantGCSBucketPublicRead(context.Background(), client, GCSBucketPublicReadRequest{Bucket: bucket})
		if gotErr != nil {
			if got != (GCSBucketPublicReadGrant{}) ||
				!errors.Is(gotErr, core.ErrObjectStoreContract) ||
				!errors.Is(gotErr, core.ErrObjectStoreDestination) {
				t.Fatalf("GrantGCSBucketPublicRead(rejected %d bytes) = (%v, %v), want zero with contract and destination identities",
					len(providerBytes), got, gotErr)
			}
			if gotGets, gotSets := provider.gets.Load(), provider.sets.Load(); gotGets != 1 || gotSets != 0 {
				t.Fatalf("rejected provider policy calls = (%d GET, %d SET, error %v), want (1, 0, typed provider refusal)", gotGets, gotSets, gotErr)
			}
			if len(providerBytes) > GCSProviderResponseMaximumBytes {
				if !errors.Is(gotErr, core.ErrObjectStoreSize) ||
					!errors.Is(gotErr, core.ErrExchangeResponse) ||
					!errors.Is(gotErr, core.ErrExchangeBodyLimit) {
					t.Fatalf("GrantGCSBucketPublicRead(rejected %d bytes) error = %v, want object-store size and Exchange body-limit identities",
						len(providerBytes), gotErr)
				}
				return
			}
			if len(providerBytes) != 0 && !jsontext.Value(providerBytes).IsValid() &&
				(!errors.Is(gotErr, core.ErrExchangeResponse) || !errors.Is(gotErr, core.ErrJSONContract)) {
				t.Fatalf("GrantGCSBucketPublicRead(rejected invalid JSON) error = %v, want Exchange response and JSON identities", gotErr)
			}
			return
		}
		if len(providerBytes) > GCSProviderResponseMaximumBytes {
			t.Fatalf("GrantGCSBucketPublicRead(accepted %d bytes) = %v, want bounded refusal", len(providerBytes), got)
		}

		if got.Validate() != nil || got.Bucket() != bucket {
			t.Fatalf("GrantGCSBucketPublicRead(accepted %d bytes) = %v, want validated exact-bucket evidence", len(providerBytes), got)
		}
		var independent storageapi.Policy
		independentErr := json.Unmarshal(providerBytes, &independent)
		independentObject := bytes.HasPrefix(bytes.TrimSpace(providerBytes), []byte("{"))
		switch got.Change() {
		case GCSBucketPublicReadUnchanged:
			if independentErr != nil || !independentObject ||
				!gcsStoragePolicyHasUnconditionalRole(independent, iam.AllUsers, gcsPublicReadRole) ||
				provider.sets.Load() != 0 || provider.gets.Load() != 1 {
				t.Fatalf("unchanged policy oracle = (decode %v, object %t, GET %d, SET %d, policy %+v), want public member with one GET and zero SET",
					independentErr, independentObject, provider.gets.Load(), provider.sets.Load(), independent)
			}
		case GCSBucketPublicReadGranted:
			written := provider.writtenPolicy()
			if !gcsStoragePolicyHasUnconditionalRole(written, iam.AllUsers, gcsPublicReadRole) ||
				provider.sets.Load() != 1 || provider.gets.Load() != 2 {
				t.Fatalf("granted policy oracle = (GET %d, SET %d, policy %+v), want confirmed public member with two GET and one SET",
					provider.gets.Load(), provider.sets.Load(), written)
			}
			if independentErr == nil && independentObject && !gcsStoragePolicyContains(independent, written) {
				t.Fatalf("granted policy = %+v, want every independently decoded provider binding from %+v preserved", written, independent)
			}
		default:
			t.Fatalf("GrantGCSBucketPublicRead() change = %v, want a closed published change", got.Change())
		}
	})
}

func gcsPublicReadFuzzClient(t testing.TB, handler http.Handler) *GCSClient {
	t.Helper()

	listener, listenErr := net.Listen("tcp", "127.0.0.1:0")
	if listenErr != nil {
		t.Fatalf("net.Listen(loopback provider) error = %v, want nil", listenErr)
	}
	server := &http.Server{Handler: handler}
	served := make(chan error, 1)
	go func() {
		served <- server.Serve(gcsPublicReadLingerListener{Listener: listener})
	}()
	t.Cleanup(func() {
		closeErr := server.Close()
		select {
		case serveErr := <-served:
			if closeErr != nil || (!errors.Is(serveErr, http.ErrServerClosed) && !errors.Is(serveErr, net.ErrClosed)) {
				t.Errorf("loopback provider shutdown = (%v, %v), want nil and closed-server identity", closeErr, serveErr)
			}
		case <-time.After(10 * time.Second):
			t.Errorf("loopback provider shutdown completed = false, want true before timeout")
		}
	})

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			var dialer net.Dialer
			connection, dialErr := dialer.DialContext(ctx, network, address)
			if dialErr != nil {
				return nil, dialErr
			}
			if tcpConnection, ok := connection.(*net.TCPConn); ok {
				if lingerErr := tcpConnection.SetLinger(0); lingerErr != nil {
					_ = connection.Close()
					return nil, lingerErr
				}
			}
			return connection, nil
		},
	}
	client, clientErr := storage.NewClient(
		context.Background(),
		option.WithEndpoint("http://"+listener.Addr().String()+"/storage/v1/"),
		option.WithoutAuthentication(),
		option.WithHTTPClient(boundedGCSTestHTTPClient(t, &http.Client{Transport: transport})),
	)
	if clientErr != nil {
		t.Fatalf("storage.NewClient(loopback provider) error = %v, want nil", clientErr)
	}
	t.Cleanup(func() {
		if closeErr := client.Close(); closeErr != nil {
			t.Errorf("storage.Client.Close() error = %v, want nil", closeErr)
		}
		transport.CloseIdleConnections()
	})
	return &GCSClient{client: client}
}

type gcsPublicReadLingerListener struct {
	net.Listener
}

func (l gcsPublicReadLingerListener) Accept() (net.Conn, error) {
	connection, acceptErr := l.Listener.Accept()
	if acceptErr != nil {
		return nil, acceptErr
	}
	if tcpConnection, ok := connection.(*net.TCPConn); ok {
		if lingerErr := tcpConnection.SetLinger(0); lingerErr != nil {
			_ = connection.Close()
			return nil, lingerErr
		}
	}
	return connection, nil
}

type gcsPublicReadFuzzProvider struct {
	t       testing.TB
	written storageapi.Policy
	initial []byte
	gets    atomic.Int64
	sets    atomic.Int64
	mu      sync.Mutex
}

func (p *gcsPublicReadFuzzProvider) ServeHTTP(writer http.ResponseWriter, incoming *http.Request) {
	if incoming.URL.Path != "/storage/v1/b/"+gcsProviderBucketText+"/iam" {
		p.t.Errorf("provider path = %q, want exact bucket IAM path", incoming.URL.Path)
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	switch incoming.Method {
	case http.MethodGet:
		p.gets.Add(1)
		if p.sets.Load() == 0 {
			writer.Header().Set("Content-Type", "application/json")
			writer.Header().Set("Content-Length", strconv.Itoa(len(p.initial)))
			if len(p.initial) > GCSProviderResponseMaximumBytes {
				return
			}
			if _, err := writer.Write(p.initial); err != nil {
				p.t.Errorf("provider fuzz policy response error = %v, want nil", err)
			}
			return
		}
		writeGCSPolicy(p.t, writer, p.writtenPolicy())
	case http.MethodPut:
		p.sets.Add(1)
		var policy storageapi.Policy
		if err := json.UnmarshalRead(incoming.Body, &policy); err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		p.mu.Lock()
		p.written = policy
		p.mu.Unlock()
		writeGCSPolicy(p.t, writer, policy)
	default:
		writer.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (p *gcsPublicReadFuzzProvider) writtenPolicy() storageapi.Policy {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.written
}
