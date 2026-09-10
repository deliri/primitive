package exchange_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

type socketCertificateFixture struct {
	root   *x509.Certificate
	leaf   *x509.Certificate
	server tls.Certificate
	client tls.Certificate
	roots  *x509.CertPool
	now    temporal.Instant
}

// Go owns certificate encoding, signing, parsing and verification. Fixed Ed25519
// keys and Temporal instants make the fixture independent of wall-clock time.
func socketCertificates(t testing.TB) socketCertificateFixture {
	t.Helper()
	before, err := temporal.ParseRFC3339("2025-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	after, err := temporal.ParseRFC3339("2030-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	now, err := temporal.ParseRFC3339("2026-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	start, err := before.Time()
	if err != nil {
		t.Fatal(err)
	}
	end, err := after.Time()
	if err != nil {
		t.Fatal(err)
	}
	rootKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{1}, ed25519.SeedSize))
	clientKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{2}, ed25519.SeedSize))
	serverKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{3}, ed25519.SeedSize))
	rootTemplate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "socket-fixture-root"}, NotBefore: start, NotAfter: end, IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	root := socketSignedCertificate(t, rootTemplate, rootTemplate, rootKey, rootKey)
	clientTemplate := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "socket-fixture-client"}, NotBefore: start, NotAfter: end, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
	leaf := socketSignedCertificate(t, clientTemplate, root, clientKey, rootKey)
	serverTemplate := &x509.Certificate{SerialNumber: big.NewInt(3), NotBefore: start, NotAfter: end, IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1)}, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	server := socketSignedCertificate(t, serverTemplate, root, serverKey, rootKey)
	roots := x509.NewCertPool()
	roots.AddCert(root)
	return socketCertificateFixture{root: root, leaf: leaf, roots: roots, now: now,
		client: tls.Certificate{Certificate: [][]byte{leaf.Raw}, PrivateKey: clientKey, Leaf: leaf},
		server: tls.Certificate{Certificate: [][]byte{server.Raw}, PrivateKey: serverKey, Leaf: server}}
}

func socketSignedCertificate(t testing.TB, template, parent *x509.Certificate, key, signer ed25519.PrivateKey) *x509.Certificate {
	t.Helper()
	der, err := x509.CreateCertificate(rand.Reader, template, parent, key.Public(), signer)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return certificate
}

func TestSocketVerifiedCertificateObservationLayerTriad(t *testing.T) {
	t.Parallel()
	fixture := socketCertificates(t)
	leafDigest := core.NewSHA256Digest(sha256.Sum256(fixture.leaf.Raw))
	rootDigest := core.NewSHA256Digest(sha256.Sum256(fixture.root.Raw))
	if leafDigest == rootDigest {
		t.Fatalf("leaf/root digest=%v/%v, want distinct", leafDigest, rootDigest)
	}
	// Exhaust the presence states at each ownership level, then pin ordering and
	// neutrality of the fields that cannot confer verified identity. This is an
	// observation of a Go-owned state, not a second certificate verifier.
	cases := []struct {
		name     string
		state    *tls.ConnectionState
		zeroCall bool
		want     core.SHA256Digest
		wantErr  error
	}{
		{name: "unbound call cannot mint identity", zeroCall: true, wantErr: core.ErrExchangeContract},
		{name: "plaintext cannot mint identity", wantErr: core.ErrExchangeContract},
		{name: "TLS alone cannot mint identity", state: &tls.ConnectionState{}, wantErr: core.ErrExchangeContract},
		{name: "peer presentation is not verification", state: &tls.ConnectionState{PeerCertificates: []*x509.Certificate{fixture.leaf}}, wantErr: core.ErrExchangeContract},
		{name: "empty first chain cannot fall through to valid second chain", state: &tls.ConnectionState{VerifiedChains: [][]*x509.Certificate{nil, {fixture.leaf, fixture.root}}}, wantErr: core.ErrExchangeContract},
		{name: "nil first leaf cannot fall through to issuer", state: &tls.ConnectionState{VerifiedChains: [][]*x509.Certificate{{nil, fixture.root}}}, wantErr: core.ErrExchangeContract},
		{name: "parsed fields without DER cannot mint identity", state: &tls.ConnectionState{VerifiedChains: [][]*x509.Certificate{{{Subject: fixture.leaf.Subject, PublicKey: fixture.leaf.PublicKey}}}}, wantErr: core.ErrExchangeContract},
		{name: "verified leaf survives absent optional peer list", state: &tls.ConnectionState{VerifiedChains: [][]*x509.Certificate{{fixture.leaf, fixture.root}}}, want: leafDigest},
		{name: "conflicting presented peer cannot replace verified leaf", state: &tls.ConnectionState{PeerCertificates: []*x509.Certificate{fixture.root}, VerifiedChains: [][]*x509.Certificate{{fixture.leaf, fixture.root}}}, want: leafDigest},
		{name: "alternative chain cannot replace selected chain", state: &tls.ConnectionState{VerifiedChains: [][]*x509.Certificate{{fixture.leaf, fixture.root}, {fixture.root}}}, want: leafDigest},
		{name: "issuer tail is neutral to leaf identity", state: &tls.ConnectionState{VerifiedChains: [][]*x509.Certificate{{fixture.leaf}}}, want: leafDigest},
		{name: "chain order determines exact selected identity", state: &tls.ConnectionState{VerifiedChains: [][]*x509.Certificate{{fixture.root}, {fixture.leaf, fixture.root}}}, want: rootDigest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			request.TLS = tc.state
			writer := httptest.NewRecorder()
			call, err := exchange.NewSocketServerCall(writer, request)
			if err != nil {
				t.Fatal(err)
			}
			if tc.zeroCall {
				call = exchange.SocketServerCall{}
			}
			got, err := call.VerifiedClientCertificateDigest()
			if got != tc.want || !errors.Is(err, tc.wantErr) {
				t.Fatalf("verified identity = (%v,%v), want (%v,%v)", got, err, tc.want, tc.wantErr)
			}
			if tc.wantErr == nil && got.Validate() != nil {
				t.Fatalf("admitted identity validation=%v, want nil", got.Validate())
			}
			if request.TLS != tc.state || writer.Body.Len() != 0 || len(writer.Header()) != 0 || writer.Flushed {
				t.Fatalf("TLS preserved/body/headers/flush=%t/%d/%d/%t, want true/0/0/false", request.TLS == tc.state, writer.Body.Len(), len(writer.Header()), writer.Flushed)
			}
		})
	}
}

func FuzzSocketVerifiedClientCertificateIdentity(f *testing.F) {
	fixture := socketCertificates(f)
	now, err := fixture.now.Time()
	if err != nil {
		f.Fatal(err)
	}
	options := x509.VerifyOptions{Roots: fixture.roots, CurrentTime: now, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
	if _, err := fixture.leaf.Verify(options); err != nil {
		f.Fatal(err)
	}
	f.Add(fixture.leaf.Raw, false)
	f.Add(fixture.leaf.Raw, true)
	f.Add(fixture.server.Leaf.Raw, false)
	f.Add(fixture.root.Raw, false)
	f.Add([]byte{}, false)
	for _, at := range []int{0, len(fixture.leaf.Raw) / 2, len(fixture.leaf.Raw) - 1} {
		mutated := bytes.Clone(fixture.leaf.Raw)
		mutated[at] ^= 1
		f.Add(mutated, false)
	}
	f.Fuzz(func(t *testing.T, der []byte, discardVerification bool) {
		// Bound the independent ASN.1/cryptographic oracle. Exchange observes
		// already-verified Go state; it does not admit or allocate DER itself.
		if len(der) > 4*len(fixture.leaf.Raw) {
			return
		}
		certificate, parseErr := x509.ParseCertificate(der)
		state := &tls.ConnectionState{}
		if parseErr == nil {
			state.PeerCertificates = []*x509.Certificate{certificate}
			chains, verifyErr := certificate.Verify(options)
			if verifyErr == nil && !discardVerification {
				state.VerifiedChains = chains
			}
		}
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
		request.TLS = state
		writer := httptest.NewRecorder()
		call, err := exchange.NewSocketServerCall(writer, request)
		if err != nil {
			t.Fatal(err)
		}
		got, err := call.VerifiedClientCertificateDigest()
		if len(state.VerifiedChains) == 0 {
			if !errors.Is(err, core.ErrExchangeContract) || got != (core.SHA256Digest{}) {
				t.Fatalf("unverified peer yielded (%v,%v)", got, err)
			}
		} else {
			// Only these two fixture certificates can authenticate for this trust
			// root and usage. Any other accepted signed document needs scrutiny.
			if !bytes.Equal(der, fixture.leaf.Raw) && !bytes.Equal(der, fixture.root.Raw) {
				t.Fatalf("accepted DER=%x, want original leaf or root DER", der)
			}
			want := sha256.Sum256(der)
			actual, projectionErr := got.Bytes()
			if err != nil || projectionErr != nil || got.Validate() != nil || actual != want {
				t.Fatalf("verified digest = (%x,%v,%v), want %x", actual, err, projectionErr, want)
			}
			if !bytes.Equal(certificate.Raw, der) {
				t.Fatalf("verified DER=%x, want %x", certificate.Raw, der)
			}
		}
		if writer.Body.Len() != 0 || len(writer.Header()) != 0 || writer.Flushed {
			t.Fatalf("response body/headers/flush=%d/%d/%t, want 0/0/false", writer.Body.Len(), len(writer.Header()), writer.Flushed)
		}
	})
}

func TestSocketGoMutualTLSLayerTriad(t *testing.T) {
	t.Parallel()
	fixture := socketCertificates(t)
	now, err := fixture.now.Time()
	if err != nil {
		t.Fatal(err)
	}
	corrupt := fixture.client
	corrupt.Certificate = [][]byte{bytes.Clone(fixture.leaf.Raw)}
	corrupt.Certificate[0][len(corrupt.Certificate[0])-1] ^= 1
	corrupt.Leaf = nil
	wrongKey := fixture.client
	wrongKey.PrivateKey = fixture.server.PrivateKey
	cases := []struct {
		name             string
		mode             tls.ClientAuthType
		certificate      *tls.Certificate
		roots            *x509.CertPool
		wantCalls        int64
		wantIdentity     core.SHA256Digest
		wantIdentityErr  error
		wantTransportErr error
	}{
		{name: "Go verifies client before identity observation", mode: tls.RequireAndVerifyClientCert, certificate: &fixture.client, roots: fixture.roots, wantCalls: 1, wantIdentity: core.NewSHA256Digest(sha256.Sum256(fixture.leaf.Raw))},
		{name: "required client absence stops before handler", mode: tls.RequireAndVerifyClientCert, roots: fixture.roots, wantTransportErr: core.ErrExchangeTransport},
		{name: "untrusted issuer stops before handler", mode: tls.RequireAndVerifyClientCert, certificate: &fixture.client, roots: x509.NewCertPool(), wantTransportErr: core.ErrExchangeTransport},
		{name: "server usage cannot authenticate a client", mode: tls.RequireAndVerifyClientCert, certificate: &fixture.server, roots: fixture.roots, wantTransportErr: core.ErrExchangeTransport},
		{name: "signature mutation cannot authenticate unchanged subject", mode: tls.RequireAndVerifyClientCert, certificate: &corrupt, roots: fixture.roots, wantTransportErr: core.ErrExchangeTransport},
		{name: "private key must prove certificate possession", mode: tls.RequireAndVerifyClientCert, certificate: &wrongKey, roots: fixture.roots, wantTransportErr: core.ErrExchangeTransport},
		{name: "requesting without verification cannot confer identity", mode: tls.RequireAnyClientCert, certificate: &fixture.client, roots: fixture.roots, wantCalls: 1, wantIdentityErr: core.ErrExchangeContract},
		{name: "optional verified client retains identity", mode: tls.VerifyClientCertIfGiven, certificate: &fixture.client, roots: fixture.roots, wantCalls: 1, wantIdentity: core.NewSHA256Digest(sha256.Sum256(fixture.leaf.Raw))},
		{name: "optional absent client remains anonymous", mode: tls.VerifyClientCertIfGiven, roots: fixture.roots, wantCalls: 1, wantIdentityErr: core.ErrExchangeContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			type observation struct {
				digest                core.SHA256Digest
				identityErr, writeErr error
			}
			observed := make(chan observation, 1)
			var calls atomic.Int64
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if calls.Add(1) != 1 {
					return
				}
				call, callErr := exchange.NewSocketServerCall(writer, request)
				if callErr != nil {
					observed <- observation{identityErr: callErr}
					return
				}
				digest, identityErr := call.VerifiedClientCertificateDigest()
				writeErr := exchange.WriteNoBody(exchange.NoBodyWriteCall{Call: call, Response: exchange.ServerNoBodyResponse{Status: core.HTTPStatusOK()}})
				observed <- observation{digest: digest, identityErr: identityErr, writeErr: writeErr}
			}))
			server.Config.ErrorLog = log.New(io.Discard, "", 0)
			server.TLS = &tls.Config{Certificates: []tls.Certificate{fixture.server}, ClientAuth: tc.mode, ClientCAs: tc.roots, Time: func() time.Time { return now }}
			server.StartTLS()
			defer server.Close()
			transport := &http.Transport{Proxy: nil, TLSClientConfig: &tls.Config{RootCAs: fixture.roots, Time: func() time.Time { return now }, GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
				if tc.certificate == nil {
					return &tls.Certificate{}, nil
				}
				return tc.certificate, nil
			}}}
			defer transport.CloseIdleConnections()
			client := mustExchangeClient(t, &http.Client{Transport: transport})
			got, err := exchange.SendNoBodyBounded(exchange.NoBodyBoundedCall{Context: t.Context(), Client: client,
				Request: exchange.NoBodyBoundedRequest{Target: mustEndpoint(t, server.URL), Semantics: exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySingleAttempt}, ExpectedStatus: core.HTTPStatusOK()},
				Policy:  exchange.NoBodyBoundedPolicy{Operation: singleAttemptOperationPolicy(t)}})
			if !errors.Is(err, tc.wantTransportErr) || calls.Load() != tc.wantCalls {
				t.Fatalf("TLS transport = (%v,%d handler calls), want (%v,%d)", err, calls.Load(), tc.wantTransportErr, tc.wantCalls)
			}
			if tc.wantTransportErr != nil {
				var native *url.Error
				if !errors.As(err, &native) || len(got.Body) != 0 {
					t.Fatalf("TLS refusal lost Go cause or published response: (%+v,%v)", got, err)
				}
				select {
				case fact := <-observed:
					t.Fatalf("unauthenticated handler fact escaped: %+v", fact)
				default:
				}
				return
			}
			// The completed HTTP response synchronizes with the handler's buffered
			// publication. No additional goroutine or unbounded channel wait exists.
			select {
			case fact := <-observed:
				if fact.digest != tc.wantIdentity || !errors.Is(fact.identityErr, tc.wantIdentityErr) || fact.writeErr != nil {
					t.Fatalf("Go-admitted identity = (%+v,%v,%v), want (%+v,%v,nil)", fact.digest, fact.identityErr, fact.writeErr, tc.wantIdentity, tc.wantIdentityErr)
				}
			default:
				t.Fatalf("completed HTTP operation has no handler observation; owned completion channel=%p", observed)
			}
			if len(got.Body) != 0 || got.Metadata.Status != core.HTTPStatusOK() {
				t.Fatalf("TLS response = %+v, want exact empty successful response", got)
			}
		})
	}
}
