package cloudflare

import (
	"bytes"
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"slices"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func FuzzImagesDetailsResponseBinding(f *testing.F) {
	wire := imageDetailsFixture()
	if err := wire.Validate(); err != nil {
		f.Fatal(err)
	}
	seed, err := core.MarshalCanonicalJSONDocument(apiEnvelope[imageDetailsWire]{Success: new(true), Result: wire})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	f.Add([]byte(`{"success":true}`))
	f.Add([]byte(`{"success":true,"result":null}`))
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
		id, err := ParseImageID(wire.ID)
		if err != nil {
			t.Fatal(err)
		}
		got, err := server.Details(t.Context(), id, testPolicy())
		if calls != 1 {
			t.Fatalf("requests=%d, want one metadata request", calls)
		}
		if err != nil {
			if !errors.Is(err, core.ErrCloudflareContract) || got.ID != (ImageID{}) || got.Ready() || got.Variants != nil {
				t.Fatalf("refusal=(%+v,%v), want zero and typed refusal", got, err)
			}
			return
		}
		var independent apiEnvelope[imageDetailsWire]
		if err := json.Unmarshal(data, &independent); err != nil {
			t.Fatal(err)
		}
		source := independent.Result
		if independent.Success == nil || !*independent.Success || len(independent.Errors) != 0 || source.ID != id.String() || got.ID != id || got.Creator != source.Creator || got.Filename != source.Filename || got.Draft != source.Draft || got.RequireSignedURLs != source.RequireSignedURLs || len(got.Variants) != len(source.Variants) {
			t.Fatal("accepted response differs from independently decoded identity/state")
		}
		if err := got.Validate(); err != nil {
			t.Fatal(err)
		}
		for i, v := range got.Variants {
			if v.HTTPURL().Host != core.CloudflareImagesDeliveryHost || v.HTTPURL().Scheme != core.SchemeHTTPS {
				t.Fatalf("variant[%d] escapes provider", i)
			}
		}
		encoded, err := core.MarshalCanonicalJSONDocument(source)
		if err != nil {
			t.Fatal(err)
		}
		round, err := core.DecodeStrictJSON[imageDetailsWire](bytes.NewReader(encoded), core.DefaultStrictJSONLimits())
		if err != nil {
			t.Fatal(err)
		}
		next, err := round.details()
		if err != nil || next.ID != got.ID || next.Uploaded != got.Uploaded || next.Creator != got.Creator || next.Filename != got.Filename || next.Draft != got.Draft || next.RequireSignedURLs != got.RequireSignedURLs || !slices.Equal(next.Variants, got.Variants) {
			t.Fatal("canonical details changed admitted facts")
		}
	})
}

func FuzzImagesDeletionAcceptance(f *testing.F) {
	result := imageDeleteWire{body: []byte(`{}`)}
	if err := result.Validate(); err != nil {
		f.Fatal(err)
	}
	seed, err := core.MarshalCanonicalJSONDocument(apiEnvelope[imageDeleteWire]{Success: new(true), Result: result})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	f.Add([]byte(`{"success":true}`))
	f.Add([]byte(`{"success":true,"result":"removed"}`))
	f.Add([]byte(`{"success":true,"result":null}`))
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
		id, err := ParseImageID("delete-fuzz")
		if err != nil {
			t.Fatal(err)
		}
		err = server.Delete(t.Context(), id, testPolicy())
		if calls != 1 {
			t.Fatalf("calls=%d, want one", calls)
		}
		if err != nil {
			if !errors.Is(err, core.ErrCloudflareContract) {
				t.Fatalf("refusal=%v, want Cloudflare identity", err)
			}
			return
		}
		var independent apiEnvelope[imageDeleteWire]
		if err := json.Unmarshal(data, &independent); err != nil || independent.Success == nil || !*independent.Success || len(independent.Errors) != 0 || len(independent.Result.body) == 0 {
			t.Fatalf("deletion accepted without independent explicit acceptance: %v", err)
		}
		encoded, err := independent.Result.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		var next imageDeleteWire
		if err := next.UnmarshalJSON(encoded); err != nil || !bytes.Equal(next.body, independent.Result.body) {
			t.Fatalf("opaque result changed: %v", err)
		}
	})
}

type headMetadataTransport struct {
	header http.Header
	calls  *int
}

func (h headMetadataTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	*h.calls++
	return &http.Response{StatusCode: http.StatusOK, Header: h.header, Body: io.NopCloser(bytes.NewReader(nil)), Request: r}, nil
}

func FuzzR2HeadMetadataConservation(f *testing.F) {
	media, err := core.ParseHTTPMediaType("video/mp4")
	if err != nil {
		f.Fatal(err)
	}
	extent, err := core.NewByteLength(27)
	if err != nil {
		f.Fatal(err)
	}
	seed := R2ObjectMetadata{ContentType: media, Bytes: extent, ETag: `"multipart-2"`}
	if err := seed.Validate(); err != nil {
		f.Fatal(err)
	}
	f.Add(strconv.FormatUint(seed.Bytes.Uint64(), 10), seed.ContentType.String(), seed.ETag)
	f.Add("-1", "video/mp4", `"opaque"`)
	f.Add("0", "", "")
	f.Fuzz(func(t *testing.T, length, media, etag string) {
		calls := 0
		header := http.Header{core.HTTPHeaderContentLength().String(): []string{length}, core.HTTPHeaderContentType().String(): []string{media}, core.CloudflareR2ETagHeader: []string{etag}}
		client, err := exchange.NewClient(&http.Client{Transport: headMetadataTransport{header: header, calls: &calls}})
		if err != nil {
			t.Fatal(err)
		}
		provider, err := NewR2Client(client)
		if err != nil {
			t.Fatal(err)
		}
		server := testR2Server(t, R2JurisdictionDefault)
		grant, err := server.Presign(t.Context(), testR2Intent(t, exchange.MethodHead, "fuzz/head"))
		if err != nil {
			t.Fatal(err)
		}
		got, err := provider.Head(t.Context(), grant, testPolicy())
		if calls != 1 {
			t.Fatalf("calls=%d,want one HEAD", calls)
		}
		if err != nil {
			if (!errors.Is(err, core.ErrCloudflareContract) && !errors.Is(err, core.ErrExchangeContract)) || got != (R2ObjectMetadata{}) {
				t.Fatalf("refusal=(%+v,%v),want zero metadata and typed error", got, err)
			}
			return
		}
		n, err := strconv.ParseUint(length, 10, 64)
		if err != nil || n != got.Bytes.Uint64() || etag != got.ETag || got.Validate() != nil {
			t.Fatal("HEAD admission invented source metadata")
		}
	})
}
