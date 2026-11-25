package gcsobjects

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
	iamcredentials "google.golang.org/api/iamcredentials/v1"
	"google.golang.org/api/option"
)

const gcsGenerationProofValue int64 = 1360887697105000

func TestGCSGenerationDownloadCapabilityRequestBoundaryTable(t *testing.T) {
	t.Parallel()

	ordinary := gcsGenerationDownloadCapabilityRequest(t, gcsGenerationProofValue)
	cases := []struct {
		mutate     func(*GCSGenerationDownloadCapabilityRequest)
		name       string
		wantAccept bool
	}{
		{name: "ordinary generation-bound request is admitted", mutate: func(*GCSGenerationDownloadCapabilityRequest) {}, wantAccept: true},
		{name: "generation one is admitted", mutate: func(r *GCSGenerationDownloadCapabilityRequest) { r.Generation = GCSGeneration{value: 1} }, wantAccept: true},
		{name: "maximum generation is admitted", mutate: func(r *GCSGenerationDownloadCapabilityRequest) { r.Generation = GCSGeneration{value: 1<<63 - 1} }, wantAccept: true},
		{name: "unset generation is refused", mutate: func(r *GCSGenerationDownloadCapabilityRequest) { r.Generation = GCSGeneration{} }},
		{name: "negative generation is refused", mutate: func(r *GCSGenerationDownloadCapabilityRequest) { r.Generation = GCSGeneration{value: -1} }},
		{name: "missing bucket is refused", mutate: func(r *GCSGenerationDownloadCapabilityRequest) { r.Download.Bucket = GCSBucket{} }},
		{name: "missing object name is refused", mutate: func(r *GCSGenerationDownloadCapabilityRequest) { r.Download.Name = GCSObjectName{} }},
		{name: "missing service account is refused", mutate: func(r *GCSGenerationDownloadCapabilityRequest) {
			r.Download.ServiceAccount = GCSServiceAccount{}
		}},
		{name: "beyond provider maximum lifetime is refused", mutate: func(r *GCSGenerationDownloadCapabilityRequest) {
			r.Download.Lifetime = gcsCapabilityLifetime(t, gcsCapabilityMaximumLifetime(t).Nanoseconds()+1)
		}},
		{name: "zero request is refused", mutate: func(r *GCSGenerationDownloadCapabilityRequest) { *r = GCSGenerationDownloadCapabilityRequest{} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := ordinary
			tc.mutate(&got)
			gotErr := got.Validate()
			if tc.wantAccept {
				if gotErr != nil {
					t.Fatalf("GCSGenerationDownloadCapabilityRequest.Validate() error = %v, want nil", gotErr)
				}
				return
			}
			if !errors.Is(gotErr, core.ErrObjectStoreContract) {
				t.Fatalf("GCSGenerationDownloadCapabilityRequest.Validate() error = %v, want errors.Is(..., %v)", gotErr, core.ErrObjectStoreContract)
			}
		})
	}
}

// TestGCSGenerationDownloadCapabilitySignsExactGeneration rebuilds the V4
// canonical request from the official SDK's returned URL and proves that the
// hash the provider signed covers exactly one generation query value. The
// unbound operation is the counterexample: it carries and signs no generation.
func TestGCSGenerationDownloadCapabilitySignsExactGeneration(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		generation int64
		bound      bool
	}{
		{name: "generation-bound issuance signs the exact generation", generation: gcsGenerationProofValue, bound: true},
		{name: "generation one is signed exactly", generation: 1, bound: true},
		{name: "maximum generation is signed exactly", generation: 1<<63 - 1, bound: true},
		{name: "unbound issuance carries and signs no generation"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			issuer, payloads := gcsCapturingCapabilityIssuer(t)
			var got objectstore.DownloadCapabilityProjection
			var gotErr error
			if tc.bound {
				got, gotErr = IssueGCSGenerationDownloadCapability(context.Background(), issuer, gcsGenerationDownloadCapabilityRequest(t, tc.generation))
			} else {
				got, gotErr = IssueGCSDownloadCapability(context.Background(), issuer, gcsDownloadCapabilityRequest(t))
			}
			if gotErr != nil || got.IsZero() || got.Validate() != nil {
				t.Fatalf("issue capability = (%v, %v), want validated nonzero projection and nil", got, gotErr)
			}
			signed := payloads.only(t)
			rawURL := gcsCapabilityBearerURL(t, got)
			parsed, err := url.Parse(rawURL)
			if err != nil {
				t.Fatalf("url.Parse(issued capability) error = %v, want nil", err)
			}
			generations := parsed.Query()[gcsGenerationQuery]
			if tc.bound {
				want := strconv.FormatInt(tc.generation, 10)
				if len(generations) != 1 || generations[0] != want {
					t.Fatalf("issued generation query = %q, want exactly [%q]", generations, want)
				}
			} else if len(generations) != 0 {
				t.Fatalf("unbound issued generation query = %q, want none", generations)
			}
			if got, want := gcsCanonicalRequestDigest(parsed, nil), gcsSignedCanonicalDigest(t, signed); got != want {
				t.Fatalf("rebuilt canonical request digest = %s, want signed digest %s", got, want)
			}
			if tc.bound {
				altered := strconv.FormatInt(tc.generation-1, 10)
				if got, signedDigest := gcsCanonicalRequestDigest(parsed, []string{altered}), gcsSignedCanonicalDigest(t, signed); got == signedDigest {
					t.Fatalf("canonical digest with generation %s = signed digest, want generation inside signature", altered)
				}
			}
		})
	}
}

func gcsGenerationDownloadCapabilityRequest(t testing.TB, generation int64) GCSGenerationDownloadCapabilityRequest {
	t.Helper()

	value, err := NewGCSGeneration(generation)
	if err != nil {
		t.Fatalf("NewGCSGeneration(%d) error = %v, want nil", generation, err)
	}
	return GCSGenerationDownloadCapabilityRequest{Download: gcsDownloadCapabilityRequest(t), Generation: value}
}

// gcsCanonicalRequestDigest rebuilds the GCS V4 canonical request for a
// host-signed unsigned-payload GET from the issued URL. A non-nil generation
// replaces the issued generation to model a forged bearer.
func gcsCanonicalRequestDigest(issued *url.URL, generation []string) string {
	query := issued.Query()
	query.Del("X-Goog-Signature")
	if generation != nil {
		query[gcsGenerationQuery] = generation
	}
	canonical := strings.Join([]string{
		http.MethodGet,
		issued.EscapedPath(),
		strings.ReplaceAll(query.Encode(), "+", "%20"),
		"host:" + issued.Hostname(),
		"",
		"host",
		"UNSIGNED-PAYLOAD",
	}, "\n")
	sum := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(sum[:])
}

func gcsSignedCanonicalDigest(t testing.TB, stringToSign []byte) string {
	t.Helper()

	lines := strings.Split(string(stringToSign), "\n")
	if len(lines) != 4 || lines[0] != "GOOG4-RSA-SHA256" {
		t.Fatalf("signed V4 string-to-sign = %q, want four GOOG4-RSA-SHA256 lines", stringToSign)
	}
	return lines[3]
}

func gcsCapabilityBearerURL(t testing.TB, projection objectstore.DownloadCapabilityProjection) string {
	t.Helper()

	encoded, err := projection.MarshalJSON()
	if err != nil {
		t.Fatalf("DownloadCapabilityProjection.MarshalJSON() error = %v, want nil", err)
	}
	var bearer struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(encoded, &bearer); err != nil || bearer.URL == "" {
		t.Fatalf("decode bearer projection = %v, want one URL", err)
	}
	return bearer.URL
}

type gcsSignedPayloads struct {
	values [][]byte
	mu     sync.Mutex
}

func (p *gcsSignedPayloads) only(t testing.TB) []byte {
	t.Helper()

	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.values) != 1 {
		t.Fatalf("official IAM SignBlob payloads = %d, want 1", len(p.values))
	}
	return p.values[0]
}

func gcsCapturingCapabilityIssuer(t testing.TB) (*GCSCapabilityIssuer, *gcsSignedPayloads) {
	t.Helper()

	payloads := &gcsSignedPayloads{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var received iamcredentials.SignBlobRequest
		if err := json.UnmarshalRead(request.Body, &received); err != nil {
			t.Errorf("json.UnmarshalRead(SignBlobRequest) error = %v, want nil", err)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		payload, err := base64.StdEncoding.DecodeString(received.Payload)
		if err != nil || len(payload) == 0 {
			t.Errorf("SignBlobRequest payload = %d bytes, %v, want nonempty and nil", len(payload), err)
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		payloads.mu.Lock()
		payloads.values = append(payloads.values, payload)
		payloads.mu.Unlock()
		encoded, err := json.Marshal(&iamcredentials.SignBlobResponse{
			SignedBlob: base64.StdEncoding.EncodeToString([]byte("deterministic-provider-signature")),
		})
		if err != nil {
			t.Errorf("json.Marshal(SignBlobResponse) error = %v, want nil", err)
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(encoded)
	}))
	t.Cleanup(server.Close)
	service, err := iamcredentials.NewService(context.Background(), option.WithoutAuthentication(), option.WithEndpoint(server.URL+"/"))
	if err != nil {
		t.Fatalf("iamcredentials.NewService() error = %v, want nil", err)
	}
	return &GCSCapabilityIssuer{service: service}, payloads
}
