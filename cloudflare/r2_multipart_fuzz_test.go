package cloudflare

import (
	"encoding/hex"
	"encoding/xml"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// Exercise the public transfer/receipt boundary with protocol-shaped ETags.
// The content is one byte; the oracle checks exact part/ETag preservation and
// refuses ambiguity, missing receipts and the provider metadata byte ceiling.
func FuzzR2MultipartPartResponseBinding(f *testing.F) {
	part := R2CompletedPart{PartNumber: 1, ETag: `"p01"`}
	if err := part.Validate(); err != nil {
		f.Fatal(err)
	}
	f.Add([]byte{1}, uint8(0))
	f.Add([]byte{1}, uint8(1))
	f.Add([]byte{}, uint8(2))
	f.Fuzz(func(t *testing.T, material []byte, mutation uint8) {
		if len(material) > core.CloudflareR2ETagMaximumBytes {
			material = material[:core.CloudflareR2ETagMaximumBytes]
		}
		etag := `"p` + hex.EncodeToString(material) + `"`
		wantETag := etag
		selector := mutation % 5
		switch selector {
		case 2:
			etag = ""
		case 3:
			etag = strings.Trim(etag, `"`)
		case 4:
			etag = "W/" + etag
		}
		peer := testExchange(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("ETag", etag)
			if selector == 1 {
				w.Header().Add("ETag", etag)
			}
		})
		client, err := NewR2Client(peer)
		if err != nil {
			t.Fatal(err)
		}
		server := testR2Server(t, R2JurisdictionDefault)
		grant, err := server.PresignMultipart(t.Context(), multipartIntent(t, R2MultipartPart))
		if err != nil {
			t.Fatal(err)
		}
		length, err := core.NewByteLength(1)
		if err != nil {
			t.Fatal(err)
		}
		got, err := client.UploadPart(t.Context(), grant, strings.NewReader("x"), length, testPolicy())
		if selector != 0 || len(wantETag) > core.CloudflareR2ETagMaximumBytes {
			if !errors.Is(err, core.ErrCloudflareResponse) || got != (R2CompletedPart{}) {
				t.Fatalf("part refusal = %+v/%v, want zero/response refusal", got, err)
			}
			return
		}
		if err != nil || got != (R2CompletedPart{PartNumber: 1, ETag: wantETag}) || got.Validate() != nil {
			t.Fatalf("part receipt = %+v/%v, want exact provider tag, requested part and nil", got, err)
		}
	})
}

func FuzzR2UploadIDRepresentationClosure(f *testing.F) {
	seed, err := ParseR2UploadID("opaque+/upload==")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed.String())
	f.Add("")
	f.Add("broken\x00id")
	f.Fuzz(func(t *testing.T, value string) {
		got, err := ParseR2UploadID(value)
		if err != nil {
			if !errors.Is(err, core.ErrCloudflareBinding) || got != (R2UploadID{}) {
				t.Fatalf("rejection=%v/%v, want zero and binding", got, err)
			}
			return
		}
		if got.String() != value || got.Validate() != nil {
			t.Fatal("admitted upload identity changed")
		}
		next, err := ParseR2UploadID(got.String())
		if err != nil || next != got {
			t.Fatal("upload identity round trip changed")
		}
	})
}

func FuzzR2MultipartSignedCoordinate(f *testing.F) {
	seed := multipartIntent(f, R2MultipartPart)
	if err := seed.Validate(); err != nil {
		f.Fatal(err)
	}
	f.Add(uint8(seed.Action), seed.UploadID.String(), seed.PartNumber)
	f.Fuzz(func(t *testing.T, action uint8, value string, part uint16) {
		server := testR2Server(t, R2JurisdictionDefault)
		input := multipartIntent(t, R2MultipartCreate)
		input.Action = R2MultipartAction(action)
		input.PartNumber = part
		if value != "" {
			id, err := ParseR2UploadID(value)
			if err != nil {
				return
			}
			input.UploadID = id
		}
		if input.Action != R2MultipartCreate {
			input.ContentType = core.HTTPMediaType{}
		}
		got, err := server.PresignMultipart(t.Context(), input)
		if err != nil {
			if input.Validate() == nil || got != (R2MultipartGrant{}) {
				t.Fatalf("signing refusal=%v/%v, want invalid input and zero", got, err)
			}
			return
		}
		endpoint, err := got.Endpoint()
		if err != nil || input.Validate() != nil {
			t.Fatal("admitted signature has invalid coordinate")
		}
		method, err := input.Action.method()
		if err != nil {
			t.Fatal(err)
		}
		request, err := http.NewRequest(method.String(), endpoint.String(), nil)
		if err != nil {
			t.Fatal(err)
		}
		if !input.ContentType.IsZero() {
			request.Header.Set("Content-Type", input.ContentType.String())
		}
		if !verifyR2TestSignature(request, []byte("r2-secret-key")) {
			t.Fatal("issued signature does not bind exact wire request")
		}
		if request.URL.Path != "/"+input.Bucket.String()+"/"+input.Key.String() {
			t.Fatal("issued request crossed object identity")
		}
		query := request.URL.Query()
		query.Set("partNumber", "10001")
		request.URL.RawQuery = query.Encode()
		if verifyR2TestSignature(request, []byte("r2-secret-key")) {
			t.Fatal("tampered coordinate retains valid signature")
		}
	})
}

type multipartCompletedFixture struct {
	XMLName           xml.Name `xml:"CompleteMultipartUploadResult"`
	Bucket, Key, ETag string
	ChecksumCRC64NVME string `xml:",omitempty"`
}

func FuzzR2MultipartControlResponseBinding(f *testing.F) {
	created := multipartCreatedBytes(f)
	completed, err := xml.Marshal(multipartCompletedFixture{Bucket: "media-bucket", Key: "video/owner/large file.mp4", ETag: `"complete"`, ChecksumCRC64NVME: "AAAAAAAAAAA="})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(false, created)
	f.Add(true, completed)
	f.Add(true, []byte(`<Error/>`))
	f.Fuzz(func(t *testing.T, complete bool, data []byte) {
		peer := testExchange(t, func(w http.ResponseWriter, r *http.Request) {
			if _, err := w.Write(data); err != nil {
				t.Error(err)
			}
		})
		client, err := NewR2Client(peer)
		if err != nil {
			t.Fatal(err)
		}
		action := R2MultipartCreate
		if complete {
			action = R2MultipartComplete
		}
		input := multipartIntent(t, action)
		server := testR2Server(t, R2JurisdictionDefault)
		grant, err := server.PresignMultipart(t.Context(), input)
		if err != nil {
			t.Fatal(err)
		}
		if complete {
			got, err := client.CompleteMultipart(t.Context(), grant, []R2CompletedPart{{PartNumber: 1, ETag: `"part"`}}, testPolicy())
			if err != nil {
				if !errors.Is(err, core.ErrCloudflareResponse) || got != (R2MultipartResult{}) {
					t.Fatalf("completion refusal=%v/%v, want zero and response error", got, err)
				}
				return
			}
			if got.Validate() != nil || got.Key != input.Key || got.Bucket != input.Bucket {
				t.Fatal("accepted completion crossed identity")
			}
			canonical, err := xml.Marshal(multipartCompletedFixture{Bucket: got.Bucket.String(), Key: got.Key.String(), ETag: got.ETag, ChecksumCRC64NVME: got.ChecksumCRC64NVME})
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := decodeR2MultipartXML(canonical, "CompleteMultipartUploadResult")
			if err != nil || parsed.ETag != got.ETag || parsed.ChecksumCRC64NVME != got.ChecksumCRC64NVME {
				t.Fatal("completion checksum/etag changed on canonical projection")
			}
			return
		}
		got, err := client.CreateMultipart(t.Context(), grant, testPolicy())
		if err != nil {
			if !errors.Is(err, core.ErrCloudflareResponse) || got != (R2MultipartUpload{}) {
				t.Fatalf("creation refusal=%v/%v, want zero and response error", got, err)
			}
			return
		}
		if got.Validate() != nil || got.Key != input.Key || got.Bucket != input.Bucket {
			t.Fatal("accepted creation crossed identity")
		}
		canonical, err := xml.Marshal(multipartCreatedFixture{Bucket: got.Bucket.String(), Key: got.Key.String(), UploadID: got.UploadID.String()})
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := decodeR2MultipartXML(canonical, "InitiateMultipartUploadResult")
		if err != nil || parsed.UploadID != got.UploadID.String() {
			t.Fatal("creation upload identity changed on canonical projection")
		}
	})
}
