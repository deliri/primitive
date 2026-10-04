package cloudflare

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/temporal"
)

const webhookTestSeconds int64 = 1791000000

func testScratch(t *testing.T, directory string) *os.File {
	t.Helper()
	path, err := core.ParseAbsolutePath(directory)
	if err != nil {
		t.Fatal(err)
	}
	root, err := filestore.OpenRoot(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	relative, err := core.ParseRelativePath("webhook-scratch")
	if err != nil {
		t.Fatal(err)
	}
	location := filestore.Location{Root: root, Path: relative}
	created, err := filestore.OpenScratch(t.Context(), filestore.ScratchRequest{Location: location, Mode: 0o600})
	if err != nil {
		t.Fatal(err)
	}
	if err := created.Close(); err != nil {
		t.Fatal(err)
	}
	file, err := filestore.OpenUpdate(t.Context(), filestore.UpdateHandleRequest{Location: location})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	})
	return file
}

func streamTestSignature(t *testing.T, secret []byte, seconds int64, body []byte) string {
	t.Helper()
	stamp := strconv.FormatInt(seconds, 10)
	mac := hmac.New(sha256.New, secret)
	if _, err := io.WriteString(mac, stamp+"."); err != nil {
		t.Fatal(err)
	}
	if _, err := mac.Write(body); err != nil {
		t.Fatal(err)
	}
	return "time=" + stamp + ",sig1=" + hex.EncodeToString(mac.Sum(nil))
}

func testWebhookRequest(t *testing.T, body []byte, destination io.Writer, name, value string) WebhookReceiveRequest {
	t.Helper()
	endpoint, err := core.ParseHTTPEndpoint("https://app.example.invalid/media/webhook")
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, endpoint.String(), bytes.NewReader(body))
	request.Header.Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
	request.Header.Set(name, value)
	call, err := exchange.NewSocketServerCall(httptest.NewRecorder(), request)
	if err != nil {
		t.Fatal(err)
	}
	stamp, err := temporal.InstantFromUnixSeconds(webhookTestSeconds)
	if err != nil {
		t.Fatal(err)
	}
	maximum, err := core.NewByteCount(1024 * 1024)
	if err != nil {
		t.Fatal(err)
	}
	return WebhookReceiveRequest{BodyMaximum: maximum, Call: call, Endpoint: endpoint, Destination: destination, ObservedAt: stamp}
}

func TestStreamWebhookAuthenticationLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		wantErr  error
		mutation func(string) string
		name     string
		body     []byte
		age      int64
	}{
		{name: "authenticated body is emitted byte for byte", body: []byte("{\"uid\":\"one\"}\n")},
		{name: "empty authenticated body emits no fabricated data"},
		{name: "reordered signature fields preserve authenticated identity", body: []byte("reordered"), mutation: func(s string) string { first, second, _ := strings.Cut(s, ","); return second + "," + first }},
		{name: "signature bit mutation exposes no body", body: []byte("secret"), mutation: func(s string) string { return s[:len(s)-1] + "x" }, wantErr: core.ErrCloudflareVerification},
		{name: "one before freshness floor refuses body", age: -61, body: []byte("old"), wantErr: core.ErrCloudflareVerification},
		{name: "exact freshness floor retains body", age: -60, body: []byte("floor")},
		{name: "one after freshness floor retains body", age: -59, body: []byte("newer")},
		{name: "exact future tolerance retains body", age: 2, body: []byte("future floor")},
		{name: "one beyond future tolerance refuses body", age: 3, body: []byte("future"), wantErr: core.ErrCloudflareVerification},
		{name: "far future timestamp refuses body", age: 1000000000, body: []byte("distant"), wantErr: core.ErrCloudflareVerification},
		{name: "duplicate signature member refuses ambiguity", mutation: func(s string) string { return s + ",sig1=" + strings.Repeat("0", 64) }, wantErr: core.ErrCloudflareAuthentication},
		{name: "unknown signature version refuses authentication", mutation: func(s string) string { return strings.Replace(s, "sig1=", "sig2=", 1) }, wantErr: core.ErrCloudflareVerification},
		{name: "negative timestamp refuses authentication", mutation: func(s string) string { return strings.Replace(s, "time=", "time=-", 1) }, wantErr: core.ErrCloudflareVerification},
		{name: "truncated digest refuses authentication", mutation: func(s string) string { return s[:len(s)-1] }, wantErr: core.ErrCloudflareVerification},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			secret, err := ParseStreamWebhookSecret([]byte("stream-test-key"))
			if err != nil {
				t.Fatal(err)
			}
			receiver, err := NewStreamWebhookReceiver(secret)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := receiver.Close(); err != nil {
					t.Error(err)
				}
			}()
			if err := secret.Close(); err != nil {
				t.Fatal(err)
			}
			value := streamTestSignature(t, []byte("stream-test-key"), webhookTestSeconds+tc.age, tc.body)
			if tc.mutation != nil {
				before := value
				value = tc.mutation(value)
				if before == value {
					t.Fatal("mutation = unchanged, want changed signature")
				}
			}
			var destination bytes.Buffer
			request := testWebhookRequest(t, tc.body, &destination, core.CloudflareStreamSignatureHeader, value)
			maximumAge, err := temporal.DurationFromSeconds(60)
			if err != nil {
				t.Fatal(err)
			}
			future, err := temporal.DurationFromSeconds(2)
			if err != nil {
				t.Fatal(err)
			}
			scratch := testScratch(t, directory)
			// Reusing scratch with a longer previous body must not append stale bytes.
			if _, err := io.WriteString(scratch, strings.Repeat("stale", 100)); err != nil {
				t.Fatal(err)
			}
			got, gotErr := receiver.Receive(StreamWebhookReceiveRequest{WebhookReceiveRequest: request, Scratch: scratch, MaximumAge: maximumAge, FutureTolerance: future})
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Receive() error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (InboundObservation{}) || destination.Len() != 0 {
					t.Fatalf("refusal = (%+v, %q), want zero observation and no output", got, destination.Bytes())
				}
				return
			}
			if !bytes.Equal(destination.Bytes(), tc.body) || got.Bytes.Uint64() != uint64(len(tc.body)) || got.ObservedAt != request.ObservedAt || got.Validate() != nil {
				t.Fatalf("Receive() = (%+v, %q), want exact %q and timestamp", got, destination.Bytes(), tc.body)
			}
		})
	}
}

func TestImagesWebhookAuthenticationLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr error
		name    string
		header  string
		body    string
	}{
		{name: "matching notification secret streams exact body", header: "images-secret", body: "{\"text\":\"uploaded\"}"},
		{name: "matching secret and empty body preserve empty observation", header: "images-secret"},
		{name: "bearer spelling is not notification authentication", header: "Bearer images-secret", body: "forged", wantErr: core.ErrCloudflareVerification},
		{name: "different secret emits no body", header: "other-secret", body: "forged", wantErr: core.ErrCloudflareVerification},
		{name: "missing secret emits no body", wantErr: core.ErrCloudflareAuthentication},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			secret, err := ParseNotificationSecret([]byte("images-secret"))
			if err != nil {
				t.Fatal(err)
			}
			receiver, err := NewImagesWebhookReceiver(secret)
			if err != nil {
				t.Fatal(err)
			}
			if err := secret.Close(); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := receiver.Close(); err != nil {
					t.Error(err)
				}
			}()
			var destination bytes.Buffer
			request := testWebhookRequest(t, []byte(tc.body), &destination, core.CloudflareNotificationAuthenticationHeader, tc.header)
			got, gotErr := receiver.Receive(request)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Receive() error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (InboundObservation{}) || destination.Len() != 0 {
					t.Fatalf("refusal = (%+v,%q), want zero", got, destination.Bytes())
				}
			} else if destination.String() != tc.body || got.Bytes.Uint64() != uint64(len(tc.body)) {
				t.Fatalf("Receive() = (%+v,%q), want %q", got, destination.Bytes(), tc.body)
			}
		})
	}
}

func FuzzStreamWebhookRawBodyAuthentication(f *testing.F) {
	seed, err := core.MarshalCanonicalJSONDocument(streamDirectUploadWire{UID: strings.Repeat("a", core.CloudflareIdentityCharacters), UploadURL: "https://" + core.CloudflareStreamUploadHost + "/upload"})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed, false)
	f.Add([]byte{}, true)
	f.Fuzz(func(t *testing.T, body []byte, mutate bool) {
		if len(body) >= 1024*1024 {
			return
		}
		directory := t.TempDir()
		secret, err := ParseStreamWebhookSecret([]byte("stream-fuzz-key"))
		if err != nil {
			t.Fatal(err)
		}
		receiver, err := NewStreamWebhookReceiver(secret)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := receiver.Close(); err != nil {
				t.Error(err)
			}
		}()
		if err := secret.Close(); err != nil {
			t.Fatal(err)
		}
		signature := streamTestSignature(t, []byte("stream-fuzz-key"), webhookTestSeconds, body)
		input := body
		if mutate {
			input = bytes.Clone(body)
			input = append(input, 0)
			if bytes.Equal(input, body) {
				t.Fatalf("mutated body bytes = %d, want changed input from %d bytes", len(input), len(body))
			}
		}
		var destination bytes.Buffer
		request := testWebhookRequest(t, input, &destination, core.CloudflareStreamSignatureHeader, signature)
		got, gotErr := receiver.Receive(StreamWebhookReceiveRequest{WebhookReceiveRequest: request, Scratch: testScratch(t, directory)})
		if mutate {
			if !errors.Is(gotErr, core.ErrCloudflareVerification) || got != (InboundObservation{}) || destination.Len() != 0 {
				t.Fatalf("mutated body = (%+v,%v,%d), want typed refusal and zero output", got, gotErr, destination.Len())
			}
			return
		}
		if gotErr != nil || !bytes.Equal(destination.Bytes(), body) || got.Bytes.Uint64() != uint64(len(body)) {
			t.Fatalf("authenticated body = (%+v,%v), want exact source bytes", got, gotErr)
		}
	})
}

func TestStreamWebhookCallerBudgetBoundsScratchLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr error
		name    string
		size    int
	}{
		{name: "one below caller budget remains authenticated", size: 31},
		{name: "exact caller budget remains authenticated", size: 32},
		{name: "one above caller budget cannot publish authenticated output", size: 33, wantErr: core.ErrCloudflareContract},
		{name: "many copy windows cannot bypass total extent", size: 3 * exchange.TransferBufferBytes, wantErr: core.ErrCloudflareContract},
		{name: "empty body preserves zero byte authentication", size: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			scratch := testScratch(t, directory)
			key := []byte("opaque signing key")
			secret, err := ParseStreamWebhookSecret(key)
			if err != nil {
				t.Fatal(err)
			}
			receiver, err := NewStreamWebhookReceiver(secret)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := receiver.Close(); err != nil {
					t.Error(err)
				}
			}()
			if err := secret.Close(); err != nil {
				t.Fatal(err)
			}
			body := bytes.Repeat([]byte{'x'}, tc.size)
			var destination bytes.Buffer
			request := testWebhookRequest(t, body, &destination, core.CloudflareStreamSignatureHeader, streamTestSignature(t, key, webhookTestSeconds, body))
			maximum, err := core.NewByteCount(32)
			if err != nil {
				t.Fatal(err)
			}
			request.BodyMaximum = maximum
			got, gotErr := receiver.Receive(StreamWebhookReceiveRequest{WebhookReceiveRequest: request, Scratch: scratch})
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Receive error=%v, want %v", gotErr, tc.wantErr)
			}
			extent, err := scratch.Seek(0, io.SeekEnd)
			if err != nil {
				t.Fatal(err)
			}
			if extent > 32 {
				t.Fatalf("scratch extent=%d, want at most caller budget 32", extent)
			}
			if gotErr != nil {
				if got != (InboundObservation{}) || destination.Len() != 0 {
					t.Fatalf("refusal=(%+v,%d output bytes), want zero proof and output", got, destination.Len())
				}
				return
			}
			if !bytes.Equal(destination.Bytes(), body) || got.Bytes.Uint64() != uint64(tc.size) {
				t.Fatalf("authenticated bytes=%d/%d, want exact body=%d", destination.Len(), got.Bytes.Uint64(), tc.size)
			}
		})
	}
}

func FuzzImagesWebhookExactSecretAndRoute(f *testing.F) {
	body, err := core.MarshalCanonicalJSONDocument(imageDirectUploadWire{ID: "draft", UploadURL: "https://" + core.CloudflareImagesUploadHost + "/one-use"})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(string(body), "notification key", false)
	f.Add("", "wrong secret", true)
	f.Fuzz(func(t *testing.T, body, header string, foreignRoute bool) {
		if len(body) > 1024*1024 {
			return
		}
		secret, err := ParseNotificationSecret([]byte("notification key"))
		if err != nil {
			t.Fatal(err)
		}
		receiver, err := NewImagesWebhookReceiver(secret)
		if err != nil {
			t.Fatal(err)
		}
		if err := secret.Close(); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := receiver.Close(); err != nil {
				t.Error(err)
			}
		}()
		var destination bytes.Buffer
		request := testWebhookRequest(t, []byte(body), &destination, core.CloudflareNotificationAuthenticationHeader, header)
		if foreignRoute {
			endpoint, err := core.ParseHTTPEndpoint("https://app.example.invalid/foreign")
			if err != nil {
				t.Fatal(err)
			}
			request.Endpoint = endpoint
		}
		got, gotErr := receiver.Receive(request)
		wantAccepted := header == "notification key" && !foreignRoute
		if wantAccepted {
			if gotErr != nil || destination.String() != body || got.Bytes.Uint64() != uint64(len(body)) {
				t.Fatalf("accepted notification error/bytes=%v/%d, want nil/%d and exact body", gotErr, got.Bytes.Uint64(), len(body))
			}
			return
		}
		if gotErr == nil || !errors.Is(gotErr, core.ErrCloudflareContract) || got != (InboundObservation{}) || destination.Len() != 0 {
			t.Fatalf("notification refusal=(%+v,%v,%d bytes), want typed zero with no output", got, gotErr, destination.Len())
		}
	})
}

func FuzzStreamWebhookSignatureRepresentation(f *testing.F) {
	body, err := core.MarshalCanonicalJSONDocument(streamDirectUploadWire{UID: strings.Repeat("b", core.CloudflareIdentityCharacters), UploadURL: "https://" + core.CloudflareStreamUploadHost + "/one-use"})
	if err != nil {
		f.Fatal(err)
	}
	// Cloudflare is the signature producer. This independent standard-library
	// signer is the provider fixture, not a second production signer in the SDK.
	stamp := strconv.FormatInt(webhookTestSeconds, 10)
	mac := hmac.New(sha256.New, []byte("stream-fuzz-key"))
	if _, err := io.WriteString(mac, stamp+"."); err != nil {
		f.Fatal(err)
	}
	if _, err := mac.Write(body); err != nil {
		f.Fatal(err)
	}
	signature := "time=" + stamp + ",sig1=" + hex.EncodeToString(mac.Sum(nil))
	f.Add(signature)
	first, second, _ := strings.Cut(signature, ",")
	reordered := second + "," + first
	f.Add(reordered)
	f.Add("")
	f.Add(signature + ",time=0")
	f.Add("time=-1,sig1=" + strings.Repeat("0", 64))
	f.Fuzz(func(t *testing.T, header string) {
		directory := t.TempDir()
		secret, err := ParseStreamWebhookSecret([]byte("stream-fuzz-key"))
		if err != nil {
			t.Fatal(err)
		}
		receiver, err := NewStreamWebhookReceiver(secret)
		if err != nil {
			t.Fatal(err)
		}
		if err := secret.Close(); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := receiver.Close(); err != nil {
				t.Error(err)
			}
		}()
		var destination bytes.Buffer
		request := testWebhookRequest(t, body, &destination, core.CloudflareStreamSignatureHeader, header)
		got, gotErr := receiver.Receive(StreamWebhookReceiveRequest{WebhookReceiveRequest: request, Scratch: testScratch(t, directory)})
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrCloudflareContract) || got != (InboundObservation{}) || destination.Len() != 0 {
				t.Fatalf("signature refusal=(%+v,%v,%d), want typed zero and no output", got, gotErr, destination.Len())
			}
			return
		}
		// With a zero-width freshness interval only the exact pinned timestamp is
		// valid. Hex case may differ; the authenticated digest must be identical.
		if (!strings.EqualFold(header, signature) && !strings.EqualFold(header, reordered)) || !bytes.Equal(destination.Bytes(), body) || got.Bytes.Uint64() != uint64(len(body)) {
			t.Fatalf("authenticated body bytes = %d, header matches seed = %v, want %d and true", got.Bytes.Uint64(), strings.EqualFold(header, signature) || strings.EqualFold(header, reordered), len(body))
		}
	})
}
