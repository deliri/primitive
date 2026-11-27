package cloudflare

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

type r2ListFixturePage struct {
	XMLName               xml.Name `xml:"http://s3.amazonaws.com/doc/2006-03-01/ ListBucketResult"`
	Name                  string
	Prefix                string
	EncodingType          string
	ContinuationToken     string `xml:",omitempty"`
	NextContinuationToken string `xml:",omitempty"`
	Contents              []r2ListFixtureObject
	MaxKeys               uint16
	KeyCount              int
	IsTruncated           bool
}

type r2ListFixtureObject struct {
	Key          string
	ETag         string
	LastModified string
	Size         uint64
}

const r2ListFixtureToken = "t+ /%?=&"
const r2ListFixtureModified = "2026-11-27T12:30:00.000000123Z"

func r2ListFixture(t testing.TB) (R2ListRequest, r2ListFixturePage) {
	t.Helper()
	intent := testR2Intent(t, exchange.MethodGet, "unused")
	prefix, err := ParseR2ListPrefix("files/")
	if err != nil {
		t.Fatal(err)
	}
	request := R2ListRequest{Bucket: intent.Bucket, Prefix: prefix, MaximumKeys: 1, SignedAt: intent.SignedAt}
	return request, r2ListFixturePage{
		Name: "media-bucket", Prefix: "files%2F", EncodingType: "url", MaxKeys: 1, KeyCount: 1,
		Contents: []r2ListFixtureObject{{Key: "files%2Fcaf%C3%A9%2B%252F", ETag: `"opaque-2"`, LastModified: r2ListFixtureModified, Size: 0}},
	}
}

func marshalR2ListFixture(t testing.TB, page r2ListFixturePage) []byte {
	t.Helper()
	raw, err := xml.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// Independently compute the header SigV4 MAC from the request received by TLS.
func validR2ListSignature(r *http.Request) bool {
	const algorithm = "AWS4-HMAC-SHA256"
	auth := strings.Split(r.Header.Get("Authorization"), ", ")
	stamp := r.Header.Get("X-Amz-Date")
	const names = "accept-encoding;host;x-amz-content-sha256;x-amz-date"
	if len(auth) != 3 || len(stamp) != 16 || auth[1] != "SignedHeaders="+names {
		return false
	}
	scope := stamp[:8] + "/auto/s3/aws4_request"
	if auth[0] != algorithm+" Credential=r2-access-key/"+scope {
		return false
	}
	payload := hex.EncodeToString(sha256.New().Sum(nil))
	if r.Header.Get("X-Amz-Content-Sha256") != payload {
		return false
	}
	query := strings.ReplaceAll(r.URL.Query().Encode(), "+", "%20")
	headers := "accept-encoding:" + r.Header.Get("Accept-Encoding") + "\nhost:" + r.Host + "\nx-amz-content-sha256:" + payload + "\nx-amz-date:" + stamp + "\n"
	canonical := r.Method + "\n" + r.URL.EscapedPath() + "\n" + query + "\n" + headers + "\n" + names + "\n" + payload
	hash := sha256.Sum256([]byte(canonical))
	message := algorithm + "\n" + stamp + "\n" + scope + "\n" + hex.EncodeToString(hash[:])
	key := []byte("AWS4r2-secret-key")
	for _, input := range []string{stamp[:8], "auto", "s3", "aws4_request", message} {
		signer := hmac.New(sha256.New, key)
		_, _ = io.WriteString(signer, input)
		key = signer.Sum(nil)
	}
	return hmac.Equal([]byte(auth[2]), []byte("Signature="+hex.EncodeToString(key)))
}

func TestR2ListSignedFirstAndNextPages(t *testing.T) {
	t.Parallel()
	request, specimen := r2ListFixture(t)
	first := specimen
	first.IsTruncated, first.NextContinuationToken = true, r2ListFixtureToken
	second := specimen
	second.ContinuationToken, second.KeyCount, second.Contents = r2ListFixtureToken, 0, nil
	firstXML, secondXML := marshalR2ListFixture(t, first), marshalR2ListFixture(t, second)
	var calls atomic.Int32
	client, err := NewR2Client(testExchange(t, func(w http.ResponseWriter, r *http.Request) {
		call := calls.Add(1)
		query := r.URL.Query()
		if r.Method != http.MethodGet || r.URL.Path != "/media-bucket" ||
			query.Get("list-type") != "2" || query.Get("max-keys") != "1" || query.Get("encoding-type") != "url" ||
			query.Get("prefix") != "files/" || query.Has("X-Amz-Signature") || !validR2ListSignature(r) {
			t.Error("provider received an unsigned or widened page")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		wire := firstXML
		switch call {
		case 1:
			if query.Has("continuation-token") {
				t.Error("first page sent a continuation")
			}
		case 2:
			if query.Get("continuation-token") != r2ListFixtureToken {
				t.Error("next page did not preserve opaque continuation")
			}
			wire = secondXML
		default:
			t.Error("list automatically fetched another page")
		}
		if _, err := w.Write(wire); err != nil {
			t.Error(err)
		}
	}))
	if err != nil {
		t.Fatal(err)
	}
	server := testR2Server(t, R2JurisdictionEU)
	firstPage, err := client.ListObjectsV2(t.Context(), server, request, testPolicy())
	if err != nil || len(firstPage.Objects) != 1 || firstPage.Continuation.String() != r2ListFixtureToken || calls.Load() != 1 {
		t.Fatalf("first page = %+v/%v with %d requests, want one object and exact cursor", firstPage, err, calls.Load())
	}
	got := firstPage.Objects[0]
	stamp, err := temporal.ParseRFC3339(r2ListFixtureModified)
	if err != nil {
		t.Fatal(err)
	}
	if got.Key.String() != "files/café+%2F" || got.ETag != `"opaque-2"` || got.Bytes.Uint64() != 0 || got.LastModified != stamp {
		t.Fatalf("listed object = %+v, want exact decoded metadata", got)
	}
	request.Continuation = firstPage.Continuation
	last, err := client.ListObjectsV2(t.Context(), server, request, testPolicy())
	if err != nil || len(last.Objects) != 0 || last.Continuation.IsSet() || calls.Load() != 2 {
		t.Fatalf("last page = %+v/%v with %d requests, want empty terminal page", last, err, calls.Load())
	}
}

func TestR2ListRefusesMalformedPagesAndInvalidLimits(t *testing.T) {
	t.Parallel()
	request, fixture := r2ListFixture(t)
	baseline := string(marshalR2ListFixture(t, fixture))
	invalid := []struct{ name, old, new string }{
		{"different bucket", "media-bucket", "other-bucket"},
		{"different echoed prefix", "files%2F</Prefix>", "other%2F</Prefix>"},
		{"object outside prefix", "files%2Fcaf", "other%2Fcaf"},
		{"invalid key escape", "%C3", "%XY"},
		{"missing extent", "<Size>0</Size>", ""},
		{"negative extent", "<Size>0</Size>", "<Size>-1</Size>"},
		{"duplicate scalar", "<Size>0</Size>", "<Size>0</Size><Size>0</Size>"},
		{"nested key markup", "<Key>", "<Key><Inside/>"},
		{"incorrect key count", "<KeyCount>1", "<KeyCount>0"},
		{"more rows than requested", "</Contents>", "</Contents><Contents/>"},
		{"non-RFC3339 modified date", r2ListFixtureModified, "yesterday"},
		{"unquoted entity tag", "&#34;opaque-2&#34;", "opaque-2"},
		{"missing next cursor", "<IsTruncated>false", "<IsTruncated>true"},
		{"unexpected cursor on final page", "</ListBucketResult>", "<NextContinuationToken>next</NextContinuationToken></ListBucketResult>"},
		{"trailing second document", "</ListBucketResult>", "</ListBucketResult><ListBucketResult/>"},
		{"truncated XML document", "</ListBucketResult>", ""},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := strings.Replace(baseline, tc.old, tc.new, 1)
			if body == baseline {
				t.Fatalf("hostile fixture did not replace %q", tc.old)
			}
			got, err := decodeR2ListPage(strings.NewReader(body), request)
			if !errors.Is(err, core.ErrCloudflareResponse) || got.Objects != nil || got.Continuation.IsSet() {
				t.Fatalf("malformed page = %+v/%v, want typed refusal with zero page", got, err)
			}
		})
	}
	t.Run("provider cannot repeat continuation", func(t *testing.T) {
		t.Parallel()
		cursor, err := ParseR2ListContinuation(r2ListFixtureToken)
		if err != nil {
			t.Fatal(err)
		}
		request := request
		request.Continuation = cursor
		reply := fixture
		reply.ContinuationToken, reply.NextContinuationToken, reply.IsTruncated = cursor.String(), cursor.String(), true
		got, err := decodeR2ListPage(strings.NewReader(string(marshalR2ListFixture(t, reply))), request)
		if !errors.Is(err, core.ErrCloudflareResponse) || got.Objects != nil {
			t.Fatalf("repeating cursor = %+v/%v, want refusal", got, err)
		}
	})
	t.Run("response larger than one page budget", func(t *testing.T) {
		t.Parallel()
		client, err := NewR2Client(testExchange(t, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprint(w, baseline+strings.Repeat(" ", r2ListPageFramingBytes+r2ListRowMaximumBytes))
		}))
		if err != nil {
			t.Fatal(err)
		}
		got, err := client.ListObjectsV2(t.Context(), testR2Server(t, R2JurisdictionDefault), request, testPolicy())
		if !errors.Is(err, core.ErrCloudflareResponse) || got.Objects != nil {
			t.Fatalf("oversized page = %+v/%v, want refusal and zero page", got, err)
		}
	})
	for _, limit := range []uint16{0, R2ListMaximumKeys + 1, ^uint16(0)} {
		refused := request
		refused.MaximumKeys = limit
		if err := refused.Validate(); !errors.Is(err, core.ErrCloudflareBinding) {
			t.Fatalf("page limit %d = %v, want refusal", limit, err)
		}
	}
	request.MaximumKeys = R2ListMaximumKeys
	if err := request.Validate(); err != nil {
		t.Fatalf("maximum page limit = %v, want nil", err)
	}
}

func FuzzR2ListRepresentations(f *testing.F) {
	f.Add("files/café+")
	f.Add(r2ListFixtureToken)
	f.Add("")
	f.Add("line\nbreak")
	f.Add(strings.Repeat("x", r2ListTokenMaximumBytes+1))
	f.Fuzz(func(t *testing.T, input string) {
		prefix, prefixErr := ParseR2ListPrefix(input)
		token, tokenErr := ParseR2ListContinuation(input)
		if prefixErr == nil && prefix.String() != input || prefixErr != nil && prefix != (R2ListPrefix{}) {
			t.Fatal("prefix admission did not preserve exact input or zero refusal")
		}
		if tokenErr == nil && (token.String() != input || !token.IsSet() || input == "" || len(input) > r2ListTokenMaximumBytes) ||
			tokenErr != nil && (token.IsSet() || !errors.Is(tokenErr, core.ErrCloudflareBinding)) {
			t.Fatal("continuation admission did not preserve bounded opaque input or zero refusal")
		}
	})
}

func FuzzR2ListProviderPage(f *testing.F) {
	request, fixture := r2ListFixture(f)
	seed := marshalR2ListFixture(f, fixture)
	f.Add(seed)
	f.Add([]byte("<ListBucketResult>"))
	f.Fuzz(func(t *testing.T, body []byte) {
		exchangeClient, err := exchange.NewClient(&http.Client{Transport: responseTransport{calls: new(int), data: body}})
		if err != nil {
			t.Fatal(err)
		}
		client, err := NewR2Client(exchangeClient)
		if err != nil {
			t.Fatal(err)
		}
		got, err := client.ListObjectsV2(t.Context(), testR2Server(t, R2JurisdictionDefault), request, testPolicy())
		if err != nil {
			if bytes.Equal(body, seed) {
				t.Fatalf("known valid provider page refused: %v", err)
			}
			if got.Objects != nil || got.Continuation.IsSet() || !errors.Is(err, core.ErrCloudflareResponse) {
				t.Fatalf("refused input yielded %+v/%v, want typed refusal and zero page", got, err)
			}
			return
		}
		if len(got.Objects) > int(request.MaximumKeys) {
			t.Fatal("accepted page exceeds request cap")
		}
		for _, row := range got.Objects {
			if !strings.HasPrefix(row.Key.String(), request.Prefix.String()) || row.Key.Validate() != nil ||
				row.LastModified.Validate() != nil || row.Bytes.Uint64() > core.CloudflareR2MultipartMaximumObjectBytes || !r2ETag(row.ETag) {
				t.Fatal("accepted row escaped signed scope or metadata contract")
			}
		}
	})
}
