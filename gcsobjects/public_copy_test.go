package gcsobjects

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

type publicCopyFixtureTransport struct {
	base   http.RoundTripper
	target *url.URL
}

func (r publicCopyFixtureTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	address := *request.URL
	address.Scheme, address.Host = r.target.Scheme, r.target.Host
	clone.URL = &address
	return r.base.RoundTrip(clone)
}

func publicCopyFixtureClient(t testing.TB, handler http.Handler) exchange.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	client, err := exchange.NewClient(&http.Client{Transport: publicCopyFixtureTransport{base: http.DefaultTransport, target: target}})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func publicCopyFixtureRequest(t testing.TB, data []byte) GCSPublicCopyRequest {
	t.Helper()
	mediaType, err := core.ParseHTTPMediaType("application/pdf")
	if err != nil {
		t.Fatal(err)
	}
	deadline, err := temporal.DurationFromSeconds(2)
	if err != nil {
		t.Fatal(err)
	}
	return GCSPublicCopyRequest{Bucket: parsedGCSBucket(t, gcsProviderBucketText), Name: parsedGCSObjectName(t, gcsProviderObjectText), ContentType: mediaType, Integrity: gcsProviderIntegrity(t, data, data), Policy: exchange.StreamPolicy{OperationTimeout: deadline, AttemptTimeout: deadline, Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject}}}
}

func TestGCSPublicCopyLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		body          string
		wantBody      string
		contentType   string
		status        int
		chunked       bool
		wrongCRC      bool
		duplicateType bool
		wantErr       bool
	}{
		{name: "anonymous exact PDF is verified", body: "public pdf", wantBody: "public pdf"},
		{name: "empty media has canonical empty integrity"},
		{name: "one byte is an exact object", body: "x", wantBody: "x"},
		{name: "one byte below the copy window remains complete", body: strings.Repeat("a", (32<<10)-1), wantBody: strings.Repeat("a", (32<<10)-1)},
		{name: "exact copy window remains complete", body: strings.Repeat("b", 32<<10), wantBody: strings.Repeat("b", 32<<10)},
		{name: "one byte beyond the copy window continues", body: strings.Repeat("c", (32<<10)+1), wantBody: strings.Repeat("c", (32<<10)+1)},
		{name: "many copy windows and final partial window continue", body: strings.Repeat("d", (96<<10)+7), wantBody: strings.Repeat("d", (96<<10)+7)},
		{name: "chunked delivery has the same exact proof", body: "chunked pdf", wantBody: "chunked pdf", chunked: true},
		{name: "short delivery refuses exact extent", body: "pdf", wantBody: "pdfx", wantErr: true},
		{name: "long delivery refuses exact extent", body: "pdfx", wantBody: "pdf", wantErr: true},
		{name: "same length different bytes refuses digest", body: "pdX", wantBody: "pdf", wantErr: true},
		{name: "correct digest with foreign checksum refuses", body: "pdf", wantBody: "pdf", wrongCRC: true, wantErr: true},
		{name: "private object cannot be certified public", body: "private", wantBody: "private", status: http.StatusForbidden, wantErr: true},
		{name: "missing object remains unavailable", status: http.StatusNotFound, wantErr: true},
		{name: "redirect is not followed with another credential context", status: http.StatusFound, wantErr: true},
		{name: "provider failure does not retry", status: http.StatusServiceUnavailable, wantErr: true},
		{name: "successful HTML is not a PDF", body: "pdf", wantBody: "pdf", contentType: "text/html", wantErr: true},
		{name: "duplicate media type is not accepted as one declaration", body: "pdf", wantBody: "pdf", duplicateType: true, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int64
			client := publicCopyFixtureClient(t, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				calls.Add(1)
				if request.Header.Get("Authorization") != "" || request.Header.Get("Cookie") != "" || request.Method != http.MethodGet || request.URL.Path != "/"+gcsProviderBucketText+"/"+gcsProviderObjectText {
					t.Errorf("public request = %s %s with auth=%t/cookie=%t, want exact anonymous object GET", request.Method, request.URL.Path, request.Header.Get("Authorization") != "", request.Header.Get("Cookie") != "")
					writer.WriteHeader(400)
					return
				}
				mediaType := tc.contentType
				if mediaType == "" {
					mediaType = "application/pdf"
				}
				writer.Header().Set("Content-Type", mediaType)
				if tc.duplicateType {
					writer.Header().Add("Content-Type", "application/pdf")
				}
				if !tc.chunked {
					writer.Header().Set("Content-Length", strconv.Itoa(len(tc.body)))
				}
				if tc.status == http.StatusFound {
					writer.Header().Set("Location", request.URL.String()+"?redirected=1")
				}
				status := tc.status
				if status == 0 {
					status = http.StatusOK
				}
				writer.WriteHeader(status)
				if tc.chunked {
					writer.(http.Flusher).Flush()
				}
				_, _ = writer.Write([]byte(tc.body))
			}))
			request := publicCopyFixtureRequest(t, []byte(tc.wantBody))
			if tc.wrongCRC {
				request.Integrity.CRC32C = core.NewCRC32C(1)
			}
			got, gotErr := VerifyPublicGCSObject(t.Context(), client, request)
			if tc.wantErr {
				if !errors.Is(gotErr, core.ErrObjectStoreIntegrity) || got != (core.HTTPEndpoint{}) {
					t.Fatalf("VerifyPublicGCSObject = (%v,%v), want zero and typed integrity refusal", got, gotErr)
				}
			} else {
				want, err := ObjectAddress(request.Bucket, request.Name)
				if err != nil {
					t.Fatal(err)
				}
				if gotErr != nil || got != want {
					t.Fatalf("VerifyPublicGCSObject = (%v,%v), want exact verified address %v", got, gotErr, want)
				}
			}
			if gotCalls := calls.Load(); gotCalls != 1 {
				t.Fatalf("public GET calls = %d, want one with no redirect or retry", gotCalls)
			}
		})
	}
}

func TestGCSPublicCopyRefusesInvalidIngressWithoutRequests(t *testing.T) {
	t.Parallel()
	var calls atomic.Int64
	client := publicCopyFixtureClient(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	valid := publicCopyFixtureRequest(t, []byte("pdf"))
	for _, missing := range []string{"bucket", "name", "type", "integrity", "operation deadline", "attempt deadline", "redirect policy"} {
		request := valid
		switch missing {
		case "bucket":
			request.Bucket = GCSBucket{}
		case "name":
			request.Name = GCSObjectName{}
		case "type":
			request.ContentType = core.HTTPMediaType{}
		case "integrity":
			request.Integrity.SHA256 = core.SHA256Digest{}
		case "operation deadline":
			request.Policy.OperationTimeout = temporal.Duration{}
		case "attempt deadline":
			request.Policy.AttemptTimeout = temporal.Duration{}
		case "redirect policy":
			request.Policy.Redirect.Mode = exchange.RedirectSameOrigin
		}
		got, gotErr := VerifyPublicGCSObject(t.Context(), client, request)
		if !errors.Is(gotErr, core.ErrObjectStoreContract) || got != (core.HTTPEndpoint{}) {
			t.Fatalf("unset %s = (%v,%v), want typed ingress refusal and zero", missing, got, gotErr)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	got, gotErr := VerifyPublicGCSObject(ctx, client, valid)
	if !errors.Is(gotErr, context.Canceled) || got != (core.HTTPEndpoint{}) {
		t.Fatalf("cancelled = (%v,%v), want preserved cancellation and zero", got, gotErr)
	}
	if gotCalls := calls.Load(); gotCalls != 0 {
		t.Fatalf("invalid ingress requests = %d, want zero", gotCalls)
	}
}
