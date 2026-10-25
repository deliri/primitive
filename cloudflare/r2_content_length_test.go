package cloudflare

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func r2SignedLength(t testing.TB, bytes uint64) exchange.SignedContentLength {
	t.Helper()
	extent, err := core.NewByteLength(bytes)
	if err != nil {
		t.Fatal(err)
	}
	length, err := exchange.NewSignedContentLength(extent)
	if err != nil {
		t.Fatal(err)
	}
	return length
}

func r2ByteLength(t testing.TB, bytes uint64) core.ByteLength {
	t.Helper()
	extent, err := core.NewByteLength(bytes)
	if err != nil {
		t.Fatal(err)
	}
	return extent
}

// The browser receives only the URL; the issuer's declared extent must be
// inside it. The provider double verifies exactly as SigV4 does: from the
// received framing. Each row sends the body through the real client and then
// replays the same URL with a different body length, which must not verify
// unless the grant carries no signed length.
func TestR2SignedContentLengthPutLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		body          string
		wantHeaders   string
		signed        bool
		wantForeignOK bool
	}{
		{name: "declared extent is signed and the exact body is accepted", signed: true, body: "hello", wantHeaders: "content-length;content-type;host;if-none-match"},
		{name: "one-byte declared extent refuses a larger replay", signed: true, body: "h", wantHeaders: "content-length;content-type;host;if-none-match"},
		{name: "unsigned grant still admits any extent, the gap this field closes", body: "hello", wantHeaders: "content-type;host;if-none-match", wantForeignOK: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := testR2Server(t, R2JurisdictionDefault)
			intent := testR2Intent(t, exchange.MethodPut, "session/original.mp4")
			intent.ContentType = core.HTTPMediaTypeOctetStream()
			intent.Conditions = R2WriteConditions{CreateOnly: true}
			if tc.signed {
				intent.ContentLength = r2SignedLength(t, uint64(len(tc.body)))
			}
			grant, err := server.Presign(t.Context(), intent)
			if err != nil {
				t.Fatalf("Presign() error = %v, want nil", err)
			}
			endpoint, err := grant.Endpoint()
			if err != nil {
				t.Fatal(err)
			}
			signedURL := endpoint.HTTPURL()
			if got := signedURL.Query().Get(core.CloudflareR2QuerySignedHeaders); got != tc.wantHeaders {
				t.Fatalf("signed headers = %q, want %q", got, tc.wantHeaders)
			}
			headers, err := grant.Headers()
			if err != nil {
				t.Fatal(err)
			}
			for _, header := range headers.Values {
				if header.Name == core.HTTPHeaderContentLength() {
					t.Fatal("grant headers list Content-Length, want transport-derived framing only")
				}
			}
			var calls, verified int
			client := testExchange(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if verifyR2TestSignature(r, []byte("r2-secret-key")) {
					verified++
				}
				_, _ = io.Copy(io.Discard, r.Body)
			})
			provider, err := NewR2Client(client)
			if err != nil {
				t.Fatal(err)
			}
			_, err = provider.Put(t.Context(), R2WriteRequest{Grant: grant, Bytes: r2ByteLength(t, uint64(len(tc.body))), Source: strings.NewReader(tc.body)}, testPolicy())
			if err != nil || calls != 1 || verified != 1 {
				t.Fatalf("Put(exact) = (%v, %d calls, %d verified), want nil, one and one", err, calls, verified)
			}
			foreign := tc.body + strings.Repeat("x", 1<<10)
			replay, err := http.NewRequest(http.MethodPut, endpoint.String(), strings.NewReader(foreign))
			if err != nil {
				t.Fatal(err)
			}
			replay.Header.Set("Content-Type", intent.ContentType.String())
			replay.Header.Set("If-None-Match", "*")
			if got := verifyR2TestSignature(replay, []byte("r2-secret-key")); got != tc.wantForeignOK {
				t.Fatalf("larger replay verified = %t, want %t", got, tc.wantForeignOK)
			}
		})
	}
}

func TestR2SignedContentLengthPutRefusesForeignExtentBeforeTransfer(t *testing.T) {
	t.Parallel()
	server := testR2Server(t, R2JurisdictionDefault)
	intent := testR2Intent(t, exchange.MethodPut, "session/original.mp4")
	intent.ContentLength = r2SignedLength(t, 5)
	grant, err := server.Presign(t.Context(), intent)
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range []uint64{4, 6} {
		var calls int
		provider, err := NewR2Client(testExchange(t, func(http.ResponseWriter, *http.Request) { calls++ }))
		if err != nil {
			t.Fatal(err)
		}
		_, err = provider.Put(t.Context(), R2WriteRequest{Grant: grant, Bytes: r2ByteLength(t, size), Source: bytes.NewReader(make([]byte, size))}, testPolicy())
		if !errors.Is(err, core.ErrCloudflareBinding) || calls != 0 {
			t.Fatalf("Put(%d bytes against 5 signed) = (%v, %d calls), want binding refusal and none", size, err, calls)
		}
	}
}

func TestR2SignedContentLengthPresignBoundaries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr error
		name    string
		bytes   uint64
		method  exchange.Method
	}{
		{name: "PUT at the single-upload maximum is signed", method: exchange.MethodPut, bytes: core.CloudflareR2SingleUploadMaximumBytes},
		{name: "PUT one byte below the maximum is signed", method: exchange.MethodPut, bytes: core.CloudflareR2SingleUploadMaximumBytes - 1},
		{name: "PUT one byte above the maximum is refused", method: exchange.MethodPut, bytes: core.CloudflareR2SingleUploadMaximumBytes + 1, wantErr: core.ErrCloudflareBinding},
		{name: "PUT of zero bytes signs an empty object", method: exchange.MethodPut, bytes: 0},
		{name: "GET has no body to bound", method: exchange.MethodGet, bytes: 5, wantErr: core.ErrCloudflareBinding},
		{name: "HEAD has no body to bound", method: exchange.MethodHead, bytes: 5, wantErr: core.ErrCloudflareBinding},
		{name: "DELETE has no body to bound", method: exchange.MethodDelete, bytes: 5, wantErr: core.ErrCloudflareBinding},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := testR2Server(t, R2JurisdictionDefault)
			intent := testR2Intent(t, tc.method, "session/original.mp4")
			intent.ContentLength = r2SignedLength(t, tc.bytes)
			got, err := server.Presign(t.Context(), intent)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Presign() error = %v, want %v", err, tc.wantErr)
			}
			if err != nil && got != (R2Grant{}) {
				t.Fatal("refused grant is populated, want zero")
			}
		})
	}
}

// A grant crossing the server/client wall must say whether its extent was
// signed; dropping or inventing the length is a binding refusal.
func TestR2SignedContentLengthGrantCrossingLayerTriad(t *testing.T) {
	t.Parallel()
	server := testR2Server(t, R2JurisdictionDefault)
	signed := testR2Intent(t, exchange.MethodPut, "session/original.mp4")
	signed.ContentLength = r2SignedLength(t, 5)
	unsigned := testR2Intent(t, exchange.MethodPut, "session/original.mp4")
	for _, tc := range []struct {
		wantErr error
		name    string
		intent  R2PresignRequest
		carried exchange.SignedContentLength
	}{
		{name: "signed extent carried intact", intent: signed, carried: signed.ContentLength},
		{name: "unsigned grant carries no extent", intent: unsigned},
		{name: "signed extent stripped in transit is refused", intent: signed, wantErr: core.ErrCloudflareBinding},
		{name: "extent invented for an unsigned grant is refused", intent: unsigned, carried: r2SignedLength(t, 5), wantErr: core.ErrCloudflareBinding},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			grant, err := server.Presign(t.Context(), tc.intent)
			if err != nil {
				t.Fatal(err)
			}
			endpoint, err := grant.Endpoint()
			if err != nil {
				t.Fatal(err)
			}
			got, err := ParseR2Grant(R2GrantInput{Account: server.account, Jurisdiction: R2JurisdictionDefault, Endpoint: endpoint, Method: exchange.MethodPut, ContentLength: tc.carried})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ParseR2Grant() error = %v, want %v", err, tc.wantErr)
			}
			if err != nil {
				if got != (R2Grant{}) {
					t.Fatal("refused grant is populated, want zero")
				}
				return
			}
			if got != grant {
				t.Fatalf("crossed grant = %v, want the issued grant", got)
			}
		})
	}
}

func TestR2SignedContentLengthMultipartPartLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr error
		name    string
		bytes   uint64
		action  R2MultipartAction
	}{
		{name: "part at the provider part maximum is signed", action: R2MultipartPart, bytes: core.CloudflareR2MultipartMaximumPartBytes},
		{name: "part one byte above the part maximum is refused", action: R2MultipartPart, bytes: core.CloudflareR2MultipartMaximumPartBytes + 1, wantErr: core.ErrCloudflareBinding},
		{name: "create has no body to bound", action: R2MultipartCreate, bytes: 5, wantErr: core.ErrCloudflareBinding},
		{name: "complete manifest is not a caller extent", action: R2MultipartComplete, bytes: 5, wantErr: core.ErrCloudflareBinding},
		{name: "abort has no body to bound", action: R2MultipartAbort, bytes: 5, wantErr: core.ErrCloudflareBinding},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := testR2Server(t, R2JurisdictionDefault)
			intent := multipartIntent(t, tc.action)
			intent.ContentLength = r2SignedLength(t, tc.bytes)
			got, err := server.PresignMultipart(t.Context(), intent)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("PresignMultipart() error = %v, want %v", err, tc.wantErr)
			}
			if err != nil && got != (R2MultipartGrant{}) {
				t.Fatal("refused grant is populated, want zero")
			}
		})
	}
}

// One signed part: the real client uploads the exact extent and the provider
// double verifies it; a different length is refused locally before transfer
// and a replayed URL with a larger body does not verify.
func TestR2SignedContentLengthUploadPartBindsExtent(t *testing.T) {
	t.Parallel()
	server := testR2Server(t, R2JurisdictionDefault)
	intent := multipartIntent(t, R2MultipartPart)
	intent.ContentLength = r2SignedLength(t, 5)
	grant, err := server.PresignMultipart(t.Context(), intent)
	if err != nil {
		t.Fatal(err)
	}
	var calls, verified int
	client := testExchange(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if verifyR2TestSignature(r, []byte("r2-secret-key")) {
			verified++
		}
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set(core.CloudflareR2ETagHeader, `"etag"`)
	})
	provider, err := NewR2Client(client)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UploadPart(t.Context(), grant, strings.NewReader("hello"), r2ByteLength(t, 5), testPolicy()); err != nil || calls != 1 || verified != 1 {
		t.Fatalf("UploadPart(exact) = (%v, %d calls, %d verified), want nil, one and one", err, calls, verified)
	}
	if _, err := provider.UploadPart(t.Context(), grant, strings.NewReader("hello!"), r2ByteLength(t, 6), testPolicy()); !errors.Is(err, core.ErrCloudflareBinding) || calls != 1 {
		t.Fatalf("UploadPart(6 against 5 signed) = (%v, %d calls), want binding refusal and no new call", err, calls)
	}
	endpoint, err := grant.Endpoint()
	if err != nil {
		t.Fatal(err)
	}
	replay, err := http.NewRequest(http.MethodPut, endpoint.String(), strings.NewReader("hello, larger"))
	if err != nil {
		t.Fatal(err)
	}
	if verifyR2TestSignature(replay, []byte("r2-secret-key")) {
		t.Fatal("larger part replay verified, want refusal")
	}
}
