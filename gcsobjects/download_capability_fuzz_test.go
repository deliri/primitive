package gcsobjects

import (
	"context"
	"encoding/base64"
	json "encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
	"google.golang.org/api/googleapi"
	iamcredentials "google.golang.org/api/iamcredentials/v1"
	"google.golang.org/api/option"
)

func FuzzGCSDownloadCapabilityProviderSignatureSemanticClosure(f *testing.F) {
	provider := newGCSCapabilityFuzzProvider(f)
	canonicalMinimum := base64.StdEncoding.EncodeToString([]byte{1})
	canonicalOrdinary := base64.StdEncoding.EncodeToString([]byte("provider-signature"))
	canonicalMaximum := base64.StdEncoding.EncodeToString(make([]byte, GCSSignatureMaximumBytes))
	f.Add(canonicalMinimum)
	f.Add(canonicalOrdinary)
	f.Add(canonicalMaximum)
	f.Add("")
	f.Add("not-base64")
	f.Add(canonicalMinimum + "=")
	f.Add(base64.StdEncoding.EncodeToString(make([]byte, GCSSignatureMaximumBytes+1)))

	f.Fuzz(func(t *testing.T, signedBlob string) {
		if len(signedBlob) > GCSSignatureMaximumEncodedBytes+1 {
			signedBlob = signedBlob[:GCSSignatureMaximumEncodedBytes+1]
		}
		signedBlob = strings.ToValidUTF8(signedBlob, "\uFFFD")
		decoded, decodeErr := base64.StdEncoding.DecodeString(signedBlob)
		wantAccept := signedBlob != "" && len(signedBlob) <= GCSSignatureMaximumEncodedBytes &&
			decodeErr == nil && len(decoded) > 0 && len(decoded) <= GCSSignatureMaximumBytes &&
			base64.StdEncoding.EncodeToString(decoded) == signedBlob

		issuer := provider.issuer(t, signedBlob)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		got, gotErr := IssueGCSDownloadCapability(
			ctx, issuer, gcsDownloadCapabilityRequest(t),
		)
		if errors.Is(gotErr, context.DeadlineExceeded) {
			t.Fatalf("IssueGCSDownloadCapability() error = %v, want provider response before fuzz deadlock backstop", gotErr)
		}
		if !wantAccept {
			if !errors.Is(gotErr, core.ErrObjectStoreDestination) || !got.IsZero() {
				t.Fatalf("IssueGCSDownloadCapability(rejected provider signature) = (%v, %v), want zero and errors.Is(..., %v)", got, gotErr, core.ErrObjectStoreDestination)
			}
			if providerErr, gotProviderErr := errors.AsType[*googleapi.Error](gotErr); gotProviderErr {
				t.Fatalf("IssueGCSDownloadCapability(rejected provider signature) provider error = %v, want successful provider response followed by typed signature refusal", providerErr)
			}
			return
		}
		if gotErr != nil || got.IsZero() || got.Validate() != nil {
			t.Fatalf("IssueGCSDownloadCapability(accepted provider signature) = (%v, %v), want validated nonzero projection and nil", got, gotErr)
		}
		encoded, gotMarshalErr := got.MarshalJSON()
		if gotMarshalErr != nil || len(encoded) > objectstore.CapabilityJSONMaximumBytes {
			t.Fatalf("DownloadCapabilityProjection.MarshalJSON() = (%d bytes, %v), want bounded and nil", len(encoded), gotMarshalErr)
		}
		var received objectstore.DownloadCapability
		if gotDecodeErr := json.Unmarshal(encoded, &received); gotDecodeErr != nil || received.Validate() != nil {
			t.Fatalf("DownloadCapability.UnmarshalJSON() = (%v, %v), want validated capability and nil", received, gotDecodeErr)
		}
		gotProvider, gotProviderErr := received.Provider()
		if gotProviderErr != nil || gotProvider != objectstore.ProviderGoogleCloudStorage {
			t.Fatalf("DownloadCapability.Provider() = (%v, %v), want (%v, nil)", gotProvider, gotProviderErr, objectstore.ProviderGoogleCloudStorage)
		}
		gotTarget, gotTargetErr := received.Target()
		if gotTargetErr != nil || gotTarget.ValidateFor(gotProvider) != nil {
			t.Fatalf("DownloadCapability.Target() = (%v, %v), want provider-valid target and nil", gotTarget, gotTargetErr)
		}
	})
}

type gcsCapabilityFuzzProvider struct {
	client   *http.Client
	endpoint string
}

func newGCSCapabilityFuzzProvider(t testing.TB) *gcsCapabilityFuzzProvider {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(serveGCSCapabilityFuzzProvider))
	t.Cleanup(server.Close)
	return &gcsCapabilityFuzzProvider{endpoint: server.URL, client: server.Client()}
}

func (p *gcsCapabilityFuzzProvider) issuer(
	t testing.TB,
	signedBlob string,
) *GCSCapabilityIssuer {
	t.Helper()

	response := base64.RawURLEncoding.EncodeToString([]byte(signedBlob))
	service, gotServiceErr := iamcredentials.NewService(
		context.Background(),
		option.WithHTTPClient(p.client),
		option.WithEndpoint(p.endpoint+"/response-"+response+"/"),
	)
	if gotServiceErr != nil {
		t.Fatalf("iamcredentials.NewService() error = %v, want nil", gotServiceErr)
	}
	return &GCSCapabilityIssuer{service: service}
}

func serveGCSCapabilityFuzzProvider(
	writer http.ResponseWriter,
	request *http.Request,
) {
	var received iamcredentials.SignBlobRequest
	gotDecodeErr := json.UnmarshalRead(request.Body, &received)
	payload, gotPayloadErr := base64.StdEncoding.DecodeString(received.Payload)
	requestValid := gotDecodeErr == nil && gotPayloadErr == nil && len(payload) > 0
	if !requestValid {
		writer.WriteHeader(http.StatusTeapot)
		return
	}
	path := strings.TrimPrefix(request.URL.Path, "/response-")
	segment, _, _ := strings.Cut(path, "/")
	signedBlob, gotResponseErr := base64.RawURLEncoding.DecodeString(segment)
	if gotResponseErr != nil {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	encoded, gotEncodeErr := json.Marshal(&iamcredentials.SignBlobResponse{
		SignedBlob: string(signedBlob),
	})
	if gotEncodeErr != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	writer.Header().Set(core.HTTPHeaderContentType().String(), "application/json")
	_, _ = writer.Write(encoded)
}
