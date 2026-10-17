package cloudflare

import (
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// Native TLS fixtures prove that a non-200 provider envelope survives the
// same single request as its status. Codes are retained data, not an absence
// classifier: this SDK does not infer object deletion from any error code.
func TestImagesNativeRefusalStatusLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		status      int
		code        int64
		success     bool
		malformed   bool
		truncated   bool
		wantRefusal bool
	}{
		{name: "missing image retains exact native refusal", status: http.StatusNotFound, code: 5404, wantRefusal: true},
		{name: "wrong route retains its different native issue", status: http.StatusNotFound, code: 7003, wantRefusal: true},
		{name: "wrong account preserves authentication refusal", status: http.StatusForbidden, code: 10000, wantRefusal: true},
		{name: "throttling preserves retry classification", status: http.StatusTooManyRequests, code: 1001, wantRefusal: true},
		{name: "provider outage preserves availability classification", status: http.StatusServiceUnavailable, code: 1000, wantRefusal: true},
		{name: "future native code is retained without inventing meaning", status: http.StatusBadRequest, code: 987654321, wantRefusal: true},
		{name: "empty typed refusal invents no issue", status: http.StatusNotFound, wantRefusal: true},
		{name: "unexpected status cannot authorize success envelope", status: http.StatusNotFound, success: true},
		{name: "malformed failure body retains only native failure", status: http.StatusNotFound, malformed: true},
		{name: "complete JSON before truncated transport is not a provider receipt", status: http.StatusNotFound, code: 5404, truncated: true},
	} {
		for _, method := range []exchange.Method{exchange.MethodGet, exchange.MethodDelete, exchange.MethodPost} {
			t.Run(tc.name+"/"+method.String(), func(t *testing.T) {
				t.Parallel()
				envelope := apiEnvelope[imageDeleteWire]{Success: new(tc.success), Result: imageDeleteWire{body: []byte(`null`)}}
				if tc.code != 0 {
					envelope.Errors = []APIIssue{{Code: tc.code, Message: "synthetic provider diagnostic"}}
				}
				body, err := core.MarshalCanonicalJSONDocument(envelope)
				if err != nil {
					t.Fatal(err)
				}
				if tc.malformed {
					body = []byte(`{"success":false`)
				}
				var calls atomic.Int64
				client := testExchange(t, func(w http.ResponseWriter, r *http.Request) {
					calls.Add(1)
					path := "/client/v4/accounts/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/images/v1/codex-absence-probe"
					if method == exchange.MethodPost {
						path = "/client/v4/accounts/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/images/v2/direct_upload"
					}
					if r.Method != method.String() || r.Header.Get("Authorization") != "Bearer cloudflare-test-token" || r.URL.Path != path {
						t.Errorf("provider request=%s %s,want exact authenticated image route", r.Method, r.URL.Path)
					}
					w.Header().Set("Content-Type", "application/json")
					if tc.truncated {
						w.Header().Set("Content-Length", fmt.Sprint(len(body)+1))
					}
					w.WriteHeader(tc.status)
					if _, err := w.Write(body); err != nil {
						t.Error(err)
					}
				})
				server, err := NewImagesServer(client, testOptions(t))
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := server.Close(); err != nil {
						t.Error(err)
					}
				})
				id, err := ParseImageID("codex-absence-probe")
				if err != nil {
					t.Fatal(err)
				}
				if method == exchange.MethodGet {
					details, cause := server.Details(t.Context(), id, testPolicy())
					err = cause
					if details.ID != (ImageID{}) || len(details.Variants) != 0 {
						t.Fatalf("refused image details=%+v,want no metadata", details)
					}
				} else if method == exchange.MethodDelete {
					err = server.Delete(t.Context(), id, testPolicy())
				} else {
					upload, cause := server.CreateDirectUpload(t.Context(), ImageDirectUploadRequest{}, testPolicy())
					err = cause
					if upload != (ImageUpload{}) {
						t.Fatalf("refused direct upload=%+v,want no capability", upload)
					}
				}
				status, ok := errors.AsType[exchange.StatusError](err)
				if !ok || calls.Load() != 1 {
					t.Fatalf("provider status=%v, calls=%d, error=%v,want native status and single attempt", ok, calls.Load(), err)
				}
				value, conversionErr := status.Status().Int()
				if conversionErr != nil || value != tc.status {
					t.Fatalf("native status=%d/%v,want %d", value, conversionErr, tc.status)
				}
				refusal, present := errors.AsType[APIRefusal](err)
				if present != tc.wantRefusal {
					t.Fatalf("native typed refusal=%t,error=%v,want %t", present, err, tc.wantRefusal)
				}
				if present {
					if len(refusal.Issues) != len(envelope.Errors) {
						t.Fatalf("native issues=%+v,want %+v", refusal.Issues, envelope.Errors)
					}
					if tc.code != 0 && (refusal.Issues[0].Code != tc.code || refusal.Issues[0].Message != envelope.Errors[0].Message) {
						t.Fatalf("native issue=%+v,want exact code and diagnostic", refusal.Issues[0])
					}
				}
				if tc.status == http.StatusForbidden && !errors.Is(err, core.ErrExchangeAuthentication) || tc.status == http.StatusTooManyRequests && !errors.Is(err, core.ErrExchangeRateLimited) || tc.status == http.StatusServiceUnavailable && !errors.Is(err, core.ErrExchangeUnavailable) {
					t.Fatalf("provider error=%v,want preserved native failure family", err)
				}
			})
		}
	}
}
