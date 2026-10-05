package cloudflare

import (
	"bytes"
	"encoding/json/v2"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

// tlsRedirectTransport is a test-only network seam: the SDK validates and signs
// the production host; this transport dials an actual local TLS provider fixture.
type tlsRedirectTransport struct {
	base   http.RoundTripper
	target *url.URL
}

func (r tlsRedirectTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	copy := request.Clone(request.Context())
	copy.URL.Host = r.target.Host
	copy.Host = request.URL.Host
	return r.base.RoundTrip(copy)
}
func testExchange(t *testing.T, handler http.HandlerFunc) exchange.Client {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	client, err := exchange.NewClient(&http.Client{Transport: tlsRedirectTransport{base: server.Client().Transport, target: target}})
	if err != nil {
		t.Fatal(err)
	}
	return client
}
func testOptions(t *testing.T) ServerOptions {
	t.Helper()
	token, err := ParseAPIToken([]byte("cloudflare-test-token"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := token.Close(); err != nil {
			t.Error(err)
		}
	})
	account, err := ParseAccountID(strings.Repeat("a", core.CloudflareIdentityCharacters))
	if err != nil {
		t.Fatal(err)
	}
	return ServerOptions{Token: token, Account: account, ResponseLimits: core.DefaultStrictJSONLimits()}
}
func testPolicy() exchange.StreamPolicy {
	timeout, err := temporal.DurationFromSeconds(10)
	if err != nil {
		panic(err)
	}
	return exchange.StreamPolicy{OperationTimeout: timeout, Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject}}
}

func TestImagesServerProviderLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr error
		name    string
		id      string
		success bool
	}{
		{name: "draft creation binds exact custom ID", success: true, id: "tenant/photo"},
		{name: "provider refusal cannot expose upload capability", id: "tenant/photo", wantErr: core.ErrCloudflareResponse},
		{name: "empty successful result cannot invent capability", success: true, wantErr: core.ErrCloudflareResponse},
		{name: "different provider ID cannot replace requested subject", success: true, id: "tenant/other", wantErr: core.ErrCloudflareBinding},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int64
			client := testExchange(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.URL.Path != "/client/v4/accounts/"+strings.Repeat("a", core.CloudflareIdentityCharacters)+"/images/v2/direct_upload" || r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer cloudflare-test-token" {
					t.Errorf("provider request=(%s,%s,%s), want account-bound authenticated POST", r.Method, r.Host, r.URL.Path)
				}
				media, parameters, err := mime.ParseMediaType(r.Header.Get(core.HTTPHeaderContentType().String()))
				if err != nil || media != "multipart/form-data" {
					t.Errorf("media=(%s,%v), want multipart", media, err)
					return
				}
				reader := multipart.NewReader(r.Body, parameters["boundary"])
				var id string
				for {
					part, err := reader.NextPart()
					if errors.Is(err, io.EOF) {
						break
					}
					if err != nil {
						t.Error(err)
						return
					}
					data, err := io.ReadAll(part)
					if err != nil {
						t.Error(err)
						return
					}
					if part.FormName() == "id" {
						id = string(data)
					}
				}
				if id != "tenant/photo" {
					t.Errorf("form ID=%q, want tenant/photo", id)
				}
				result := apiEnvelope[imageDirectUploadWire]{Success: &tc.success, Result: imageDirectUploadWire{ID: tc.id, UploadURL: "https://" + core.CloudflareImagesUploadHost + "/one-use"}}
				body, err := core.MarshalCanonicalJSONDocument(result)
				if err != nil {
					t.Error(err)
					return
				}
				w.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
				if _, err := w.Write(body); err != nil {
					t.Error(err)
				}
			})
			server, err := NewImagesServer(client, testOptions(t))
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := server.Close(); err != nil {
					t.Error(err)
				}
			}()
			id, err := ParseImageID("tenant/photo")
			if err != nil {
				t.Fatal(err)
			}
			got, gotErr := server.CreateDirectUpload(t.Context(), ImageDirectUploadRequest{CustomID: id}, testPolicy())
			if !errors.Is(gotErr, tc.wantErr) || calls.Load() != 1 {
				t.Fatalf("CreateDirectUpload=(%v,%d calls), want %v and one call", gotErr, calls.Load(), tc.wantErr)
			}
			if tc.wantErr != nil {
				if got.ID != (ImageID{}) || got.endpoint != (core.HTTPEndpoint{}) {
					t.Fatalf("refused capability=%+v, want zero", got)
				}
				return
			}
			if got.ID != id || got.Validate() != nil {
				t.Fatalf("issued ID=%v, want %v", got.ID, id)
			}
		})
	}
}

func TestImagesClientMultipartSourceLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr  error
		name     string
		body     string
		declared uint64
	}{
		{name: "multipart file and framing reach provider", body: "image-bytes", declared: 11},
		{name: "empty file preserves zero file extent"},
		{name: "source shorter than declaration cannot pass", body: "x", declared: 2, wantErr: io.ErrUnexpectedEOF},
		{name: "source longer than declaration cannot pass", body: "xy", declared: 1, wantErr: core.ErrCloudflareContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var gotFile bytes.Buffer
			client := testExchange(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "" {
					t.Errorf("upload authorization bytes = %d, want 0", len(r.Header.Get("Authorization")))
				}
				_, parameters, err := mime.ParseMediaType(r.Header.Get(core.HTTPHeaderContentType().String()))
				if err != nil {
					t.Error(err)
					return
				}
				reader := multipart.NewReader(r.Body, parameters["boundary"])
				part, err := reader.NextPart()
				if err != nil {
					return
				}
				if part.FormName() != core.CloudflareMultipartFileField || part.FileName() != "image.png" {
					t.Errorf("multipart part=(%q,%q), want file/image.png", part.FormName(), part.FileName())
				}
				if _, err := io.Copy(&gotFile, part); err != nil {
					return
				}
				if _, err := reader.NextPart(); !errors.Is(err, io.EOF) {
					return
				}
				w.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
				if _, err := io.WriteString(w, "{}"); err != nil {
					t.Error(err)
				}
			})
			images, err := NewImagesClient(client)
			if err != nil {
				t.Fatal(err)
			}
			id, err := ParseImageID("draft")
			if err != nil {
				t.Fatal(err)
			}
			upload, err := ParseImageUpload(id, "https://"+core.CloudflareImagesUploadHost+"/one-use")
			if err != nil {
				t.Fatal(err)
			}
			length, err := core.NewByteLength(tc.declared)
			if err != nil {
				t.Fatal(err)
			}
			var response bytes.Buffer
			_, gotErr := images.Upload(t.Context(), upload, MediaUpload{Source: strings.NewReader(tc.body), Response: &response, Filename: "image.png", Bytes: length}, testPolicy())
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Upload error=%v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr == nil && (gotFile.String() != tc.body || response.String() != "{}") {
				t.Fatalf("provider file/response=(%q,%q), want (%q,{})", gotFile.String(), response.String(), tc.body)
			}
		})
	}
}

type responseTransport struct {
	calls *int
	data  []byte
}

func (r responseTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	*r.calls++
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{core.HTTPHeaderContentType().String(): []string{core.HTTPMediaTypeJSON().String()}}, Body: io.NopCloser(bytes.NewReader(r.data)), Request: request}, nil
}

func FuzzImagesDirectUploadResponseSemanticClosure(f *testing.F) {
	accepted := true
	wire := imageDirectUploadWire{ID: "draft", UploadURL: "https://" + core.CloudflareImagesUploadHost + "/upload"}
	if err := wire.Validate(); err != nil {
		f.Fatal(err)
	}
	seed, err := core.MarshalCanonicalJSONDocument(apiEnvelope[imageDirectUploadWire]{Success: &accepted, Result: wire})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	f.Add([]byte{})
	f.Add([]byte(`{"success":true,"success":false}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		calls := 0
		client, err := exchange.NewClient(&http.Client{Transport: responseTransport{data: data, calls: &calls}})
		if err != nil {
			t.Fatal(err)
		}
		server, err := NewImagesServer(client, testOptions(t))
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := server.Close(); err != nil {
				t.Error(err)
			}
		}()
		got, gotErr := server.CreateDirectUpload(t.Context(), ImageDirectUploadRequest{}, testPolicy())
		if calls != 1 {
			t.Fatalf("calls=%d, want one", calls)
		}
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrCloudflareContract) || got.ID != (ImageID{}) || got.endpoint != (core.HTTPEndpoint{}) {
				t.Fatalf("refusal=(%v,%v), want zero and typed Cloudflare error", got.ID, gotErr)
			}
			return
		}
		var independent apiEnvelope[imageDirectUploadWire]
		if err := json.Unmarshal(data, &independent); err != nil || independent.Success == nil || !*independent.Success || len(independent.Errors) != 0 || independent.Result.ID != got.ID.String() || independent.Result.UploadURL != got.endpoint.String() {
			t.Fatalf("accepted response does not retain source identity, want independent wire agreement: %v", err)
		}
		if got.Validate() != nil {
			t.Fatalf("accepted ID=%v, want validated capability", got.ID)
		}
		reencoded, err := core.MarshalCanonicalJSONDocument(apiEnvelope[imageDirectUploadWire]{Success: &accepted, Result: imageDirectUploadWire{ID: got.ID.String(), UploadURL: got.endpoint.String()}})
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := core.DecodeStrictJSONStructure[apiEnvelope[imageDirectUploadWire]](reencoded, core.DefaultStrictJSONLimits())
		if err != nil || decoded.Result.ID != got.ID.String() || decoded.Result.UploadURL != got.endpoint.String() {
			t.Fatalf("canonical result=(%+v,%v), want original capability", decoded.Result, err)
		}
	})
}
