package cloudflare

import (
	"errors"
	"net/http"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func multipartIntent(t testing.TB, action R2MultipartAction) R2MultipartPresignRequest {
	t.Helper()
	base := testR2Intent(t, exchange.MethodPut, "video/owner/large file.mp4")
	result := R2MultipartPresignRequest{Bucket: base.Bucket, Key: base.Key, SignedAt: base.SignedAt, Expires: base.Expires, Action: action}
	if action == R2MultipartCreate {
		result.ContentType = core.HTTPMediaTypeOctetStream()
	} else {
		id, err := ParseR2UploadID("opaque+/upload==")
		if err != nil {
			t.Fatal(err)
		}
		result.UploadID = id
	}
	if action == R2MultipartPart {
		result.PartNumber = 1
	}
	return result
}

func TestR2MultipartSignatureBindsOperationAndCoordinate(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		action R2MultipartAction
		method string
		part   uint16
	}{
		{"create one upload", R2MultipartCreate, "POST", 0},
		{"upload first part", R2MultipartPart, "PUT", 1},
		{"upload final permitted part", R2MultipartPart, "PUT", 10000},
		{"complete exact upload", R2MultipartComplete, "POST", 0},
		{"abort exact upload", R2MultipartAbort, "DELETE", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := testR2Server(t, R2JurisdictionDefault)
			intent := multipartIntent(t, tc.action)
			intent.PartNumber = tc.part
			grant, err := server.PresignMultipart(t.Context(), intent)
			if err != nil {
				t.Fatal(err)
			}
			endpoint, err := grant.Endpoint()
			if err != nil {
				t.Fatal(err)
			}
			request, err := http.NewRequest(tc.method, endpoint.String(), nil)
			if err != nil {
				t.Fatal(err)
			}
			if tc.action == R2MultipartCreate {
				request.Header.Set("Content-Type", intent.ContentType.String())
			}
			if !verifyR2TestSignature(request, []byte("r2-secret-key")) {
				t.Fatal("signature valid=false, want true")
			}
			query := request.URL.Query()
			if tc.action == R2MultipartCreate {
				if !query.Has("uploads") || query.Has("uploadId") {
					t.Fatal("create query does not bind uploads")
				}
			} else if query.Get("uploadId") != intent.UploadID.String() {
				t.Fatal("upload identity changed")
			}
			if tc.part != 0 && query.Get("partNumber") != strconv.Itoa(int(tc.part)) {
				t.Fatal("part identity changed")
			}
			query.Set("uploadId", "crossed-upload")
			request.URL.RawQuery = query.Encode()
			if verifyR2TestSignature(request, []byte("r2-secret-key")) {
				t.Fatal("crossed signature valid=true, want false")
			}
		})
	}
}

func TestR2MultipartRejectsContradictoryGrants(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		mutate func(*R2MultipartPresignRequest)
	}{
		{"zero action", func(r *R2MultipartPresignRequest) { r.Action = 0 }},
		{"unknown action", func(r *R2MultipartPresignRequest) { r.Action = 255 }},
		{"missing upload ID", func(r *R2MultipartPresignRequest) { r.UploadID = R2UploadID{} }},
		{"zero part", func(r *R2MultipartPresignRequest) { r.PartNumber = 0 }},
		{"part beyond provider bound", func(r *R2MultipartPresignRequest) { r.PartNumber = 10001 }},
		{"content type on part", func(r *R2MultipartPresignRequest) { r.ContentType = core.HTTPMediaTypeJSON() }},
		{"create cannot reuse upload ID", func(r *R2MultipartPresignRequest) {
			r.Action = R2MultipartCreate
			r.PartNumber = 0
			r.ContentType = core.HTTPMediaTypeJSON()
		}},
		{"complete cannot name one part", func(r *R2MultipartPresignRequest) { r.Action = R2MultipartComplete }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := testR2Server(t, R2JurisdictionDefault)
			intent := multipartIntent(t, R2MultipartPart)
			if _, err := server.PresignMultipart(t.Context(), intent); err != nil {
				t.Fatal(err)
			}
			tc.mutate(&intent)
			got, err := server.PresignMultipart(t.Context(), intent)
			if !errors.Is(err, core.ErrCloudflareBinding) || got != (R2MultipartGrant{}) {
				t.Fatalf("PresignMultipart=(%v,%v), want zero and binding refusal", got, err)
			}
		})
	}
}
