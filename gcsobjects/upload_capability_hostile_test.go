package gcsobjects

import (
	"context"
	"encoding/base64"
	json "encoding/json/v2"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/temporal"
	iamcredentials "google.golang.org/api/iamcredentials/v1"
	"google.golang.org/api/option"
)

type gcsCapabilityProviderOutcome uint8

const (
	gcsCapabilityProviderOutcomeUnknown gcsCapabilityProviderOutcome = iota
	gcsCapabilityProviderOutcomeSigned
	gcsCapabilityProviderOutcomeRefused
	gcsCapabilityProviderOutcomeEmptySignature
)

func TestGCSServiceAccountHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		value      string
		wantAccept bool
	}{
		{name: "App Engine default account is admitted", value: "example-project@appspot.gserviceaccount.com", wantAccept: true},
		{name: "Compute default account is admitted", value: "123-compute@developer.gserviceaccount.com", wantAccept: true},
		{name: "IAM account is admitted", value: "media-signer@example-project.iam.gserviceaccount.com", wantAccept: true},
		{name: "provider managed account is admitted", value: "service-123@gcp-sa-storage.iam.gserviceaccount.com", wantAccept: true},
		{name: "numeric principal is admitted", value: "123@project.iam.gserviceaccount.com", wantAccept: true},
		{name: "hyphenated principal is admitted", value: "a-b-c@project.iam.gserviceaccount.com", wantAccept: true},
		{name: "dotted principal is admitted", value: "a.b@project.iam.gserviceaccount.com", wantAccept: true},
		{name: "single byte principal is admitted", value: "a@project.iam.gserviceaccount.com", wantAccept: true},
		{name: "maximum mailbox local part is admitted", value: strings.Repeat("a", 64) + "@project.iam.gserviceaccount.com", wantAccept: true},
		{name: "deep provider domain is admitted", value: "signer@one.two.three.iam.gserviceaccount.com", wantAccept: true},
		{name: "zero value is refused"},
		{name: "missing mailbox separator is refused", value: "example-project.appspot.gserviceaccount.com"},
		{name: "empty local part is refused", value: "@project.iam.gserviceaccount.com"},
		{name: "empty domain is refused", value: "signer@"},
		{name: "foreign Google mailbox is refused", value: "signer@gmail.com"},
		{name: "suffix lookalike is refused", value: "signer@project.gserviceaccount.com.example"},
		{name: "display name wrapper is refused", value: "Signer <signer@project.iam.gserviceaccount.com>"},
		{name: "multiple separators are refused", value: "signer@@project.iam.gserviceaccount.com"},
		{name: "leading local dot is refused", value: ".signer@project.iam.gserviceaccount.com"},
		{name: "trailing local dot is refused", value: "signer.@project.iam.gserviceaccount.com"},
		{name: "consecutive local dots are refused", value: "signer..name@project.iam.gserviceaccount.com"},
		{name: "line break is refused", value: "signer\n@project.iam.gserviceaccount.com"},
		{name: "Unicode mailbox is refused", value: "signér@project.iam.gserviceaccount.com"},
		{name: "uppercase provider suffix is refused", value: "signer@project.iam.GSERVICEACCOUNT.COM"},
		{name: "mailbox beyond aggregate ceiling is refused", value: strings.Repeat("a", GCSServiceAccountMaximumBytes) + "@project.iam.gserviceaccount.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := ParseGCSServiceAccount(tc.value)
			if tc.wantAccept {
				if gotErr != nil || got.String() != tc.value || got.Validate() != nil {
					t.Fatalf("ParseGCSServiceAccount(%q) = (%v, %v), want exact validated value and nil", tc.value, got, gotErr)
				}
				return
			}
			if !errors.Is(gotErr, core.ErrObjectStoreContract) || got != (GCSServiceAccount{}) {
				t.Fatalf("ParseGCSServiceAccount(%q) = (%v, %v), want zero and errors.Is(..., %v)", tc.value, got, gotErr, core.ErrObjectStoreContract)
			}
		})
	}
}

func TestGCSUploadCapabilityRequestHostileBoundaries(t *testing.T) {
	t.Parallel()

	valid := gcsCapabilityRequest(t)
	cases := []struct {
		wantErr error
		mutate  func(*GCSUploadCapabilityRequest)
		name    string
	}{
		{name: "complete request remains valid", mutate: func(*GCSUploadCapabilityRequest) {}},
		{name: "unset bucket is refused", mutate: func(value *GCSUploadCapabilityRequest) { value.Bucket = GCSBucket{} }, wantErr: core.ErrObjectStoreContract},
		{name: "unset object name is refused", mutate: func(value *GCSUploadCapabilityRequest) { value.Name = GCSObjectName{} }, wantErr: core.ErrObjectStoreContract},
		{name: "unset service account is refused", mutate: func(value *GCSUploadCapabilityRequest) { value.ServiceAccount = GCSServiceAccount{} }, wantErr: core.ErrObjectStoreContract},
		{name: "unset integrity is refused", mutate: func(value *GCSUploadCapabilityRequest) { value.Integrity = objectstore.Integrity{} }, wantErr: core.ErrObjectStoreContract},
		{name: "unset content type is refused", mutate: func(value *GCSUploadCapabilityRequest) { value.ContentType = core.HTTPMediaType{} }, wantErr: core.ErrObjectStoreContract},
		{name: "zero lifetime is refused", mutate: func(value *GCSUploadCapabilityRequest) { value.Lifetime = temporal.Duration{} }, wantErr: core.ErrObjectStoreContract},
		{name: "exact seven day lifetime is admitted", mutate: func(value *GCSUploadCapabilityRequest) {
			lifetime, err := temporal.DurationFromDays(GCSCapabilityMaximumDays)
			if err != nil {
				t.Fatalf("temporal.DurationFromDays(maximum) error = %v, want nil", err)
			}
			value.Lifetime = lifetime
		}},
		{name: "one nanosecond beyond seven days is refused", mutate: func(value *GCSUploadCapabilityRequest) {
			maximum, err := temporal.DurationFromDays(GCSCapabilityMaximumDays)
			if err != nil {
				t.Fatalf("temporal.DurationFromDays(maximum) error = %v, want nil", err)
			}
			lifetime, err := temporal.DurationFromNanoseconds(maximum.Nanoseconds() + 1)
			if err != nil {
				t.Fatalf("temporal.DurationFromNanoseconds(maximum + 1) error = %v, want nil", err)
			}
			value.Lifetime = lifetime
		}, wantErr: core.ErrObjectStoreContract},
		{name: "one byte beyond GCS extent is refused", mutate: func(value *GCSUploadCapabilityRequest) {
			length, err := core.NewByteLength(objectstore.GoogleCloudStorageObjectMaximumBytes + 1)
			if err != nil {
				t.Fatalf("core.NewByteLength(GCS maximum + 1) error = %v, want nil", err)
			}
			value.Integrity.Length = length
		}, wantErr: core.ErrObjectStoreContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			value := valid
			tc.mutate(&value)
			gotErr := value.Validate()
			if tc.wantErr == nil && gotErr != nil {
				t.Fatalf("GCSUploadCapabilityRequest.Validate() error = %v, want nil", gotErr)
			}
			if tc.wantErr != nil && !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("GCSUploadCapabilityRequest.Validate() error = %v, want errors.Is(..., %v)", gotErr, tc.wantErr)
			}
		})
	}
}

func TestGCSUploadCapabilityIssuanceLayerTriadUsesOfficialSDKSigningLeaf(t *testing.T) {
	t.Parallel()

	t.Run("positive official IAM response closes one valid Objectstore capability", func(t *testing.T) {
		t.Parallel()

		issuer, calls := gcsCapabilityIssuer(t, gcsCapabilityProviderOutcomeSigned)
		request := gcsCapabilityRequest(t)
		got, gotErr := IssueGCSUploadCapability(context.Background(), issuer, request)
		if gotErr != nil || got.IsZero() {
			t.Fatalf("IssueGCSUploadCapability() = (%v, %v), want nonzero and nil", got, gotErr)
		}
		if gotCalls := calls.Load(); gotCalls != 1 {
			t.Fatalf("official IAM SignBlob calls = %d, want 1", gotCalls)
		}
		encoded, marshalErr := got.MarshalJSON()
		if marshalErr != nil {
			t.Fatalf("UploadCapabilityProjection.MarshalJSON() error = %v, want nil", marshalErr)
		}
		var received objectstore.UploadCapability
		if err := json.Unmarshal(encoded, &received); err != nil {
			t.Fatalf("json.Unmarshal(UploadCapability) error = %v, want nil", err)
		}
		browser, browserErr := objectstore.NewUploadHTTPProjection(
			got,
			request.Integrity,
			request.ContentType,
		)
		if browserErr != nil || browser.IsZero() {
			t.Fatalf("NewUploadHTTPProjection(issued) = (%v, %v), want nonzero and nil", browser, browserErr)
		}
	})

	t.Run("negative provider refusal preserves typed destination identity and zero capability", func(t *testing.T) {
		t.Parallel()

		issuer, calls := gcsCapabilityIssuer(t, gcsCapabilityProviderOutcomeRefused)
		got, gotErr := IssueGCSUploadCapability(
			context.Background(),
			issuer,
			gcsCapabilityRequest(t),
		)
		if !errors.Is(gotErr, core.ErrObjectStoreDestination) || !got.IsZero() {
			t.Fatalf(
				"IssueGCSUploadCapability(provider refusal) = (%v, %v), want zero and errors.Is(..., %v)",
				got,
				gotErr,
				core.ErrObjectStoreDestination,
			)
		}
		if gotCalls := calls.Load(); gotCalls != 1 {
			t.Fatalf("official IAM SignBlob calls = %d, want 1", gotCalls)
		}
	})

	t.Run("negative empty provider signature preserves typed destination identity and zero capability", func(t *testing.T) {
		t.Parallel()

		issuer, calls := gcsCapabilityIssuer(t, gcsCapabilityProviderOutcomeEmptySignature)
		got, gotErr := IssueGCSUploadCapability(
			context.Background(),
			issuer,
			gcsCapabilityRequest(t),
		)
		if !errors.Is(gotErr, core.ErrObjectStoreDestination) || !got.IsZero() {
			t.Fatalf(
				"IssueGCSUploadCapability(empty signature) = (%v, %v), want zero and errors.Is(..., %v)",
				got,
				gotErr,
				core.ErrObjectStoreDestination,
			)
		}
		if gotCalls := calls.Load(); gotCalls != 1 {
			t.Fatalf("official IAM SignBlob calls = %d, want 1", gotCalls)
		}
	})

	t.Run("neutral canceled ingress performs no provider request and releases no capability", func(t *testing.T) {
		t.Parallel()

		issuer, calls := gcsCapabilityIssuer(t, gcsCapabilityProviderOutcomeSigned)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		got, gotErr := IssueGCSUploadCapability(ctx, issuer, gcsCapabilityRequest(t))
		if !errors.Is(gotErr, context.Canceled) || !got.IsZero() {
			t.Fatalf(
				"IssueGCSUploadCapability(canceled) = (%v, %v), want zero and errors.Is(..., context.Canceled)",
				got,
				gotErr,
			)
		}
		if gotCalls := calls.Load(); gotCalls != 0 {
			t.Fatalf("official IAM SignBlob calls after canceled ingress = %d, want 0", gotCalls)
		}
	})
}

func TestNewGCSCapabilityIssuerRefusesInvalidConstructionIngress(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		ctx    context.Context
		config GCSClientConfig
	}{
		{name: "nil context is rejected before official sdk construction", config: GCSClientConfig{Authentication: GCSAuthenticationApplicationDefault}},
		{name: "unset authentication is rejected before official sdk construction", ctx: context.Background()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := NewGCSCapabilityIssuer(tc.ctx, tc.config)
			if !errors.Is(gotErr, core.ErrObjectStoreContract) || got != nil {
				t.Fatalf("NewGCSCapabilityIssuer() = (%v, %v), want nil and errors.Is(..., %v)", got, gotErr, core.ErrObjectStoreContract)
			}
		})
	}
}

func gcsCapabilityIssuer(
	t testing.TB,
	outcome gcsCapabilityProviderOutcome,
) (*GCSCapabilityIssuer, *atomic.Uint64) {
	t.Helper()

	signedBlob := base64.StdEncoding.EncodeToString([]byte("deterministic-provider-signature"))
	if outcome == gcsCapabilityProviderOutcomeEmptySignature {
		signedBlob = ""
	}

	var calls atomic.Uint64
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		if outcome == gcsCapabilityProviderOutcomeRefused {
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
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
		encoded, err := json.Marshal(&iamcredentials.SignBlobResponse{SignedBlob: signedBlob})
		if err != nil {
			t.Errorf("json.Marshal(SignBlobResponse) error = %v, want nil", err)
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(encoded)
	}))
	t.Cleanup(server.Close)
	service, err := iamcredentials.NewService(
		context.Background(),
		option.WithoutAuthentication(),
		option.WithEndpoint(server.URL+"/"),
	)
	if err != nil {
		t.Fatalf("iamcredentials.NewService() error = %v, want nil", err)
	}
	return &GCSCapabilityIssuer{service: service}, &calls
}

func gcsCapabilityRequest(t testing.TB) GCSUploadCapabilityRequest {
	t.Helper()

	bucket, err := ParseGCSBucket("example-project-stage-media")
	if err != nil {
		t.Fatalf("ParseGCSBucket() error = %v, want nil", err)
	}
	name, err := ParseGCSObjectName("users/account/media/upload.webp")
	if err != nil {
		t.Fatalf("ParseGCSObjectName() error = %v, want nil", err)
	}
	account, err := ParseGCSServiceAccount("example-project@appspot.gserviceaccount.com")
	if err != nil {
		t.Fatalf("ParseGCSServiceAccount() error = %v, want nil", err)
	}
	contentType, err := core.ParseHTTPMediaType("image/webp")
	if err != nil {
		t.Fatalf("core.ParseHTTPMediaType() error = %v, want nil", err)
	}
	lifetime, err := temporal.DurationFromMinutes(5)
	if err != nil {
		t.Fatalf("temporal.DurationFromMinutes() error = %v, want nil", err)
	}
	return GCSUploadCapabilityRequest{
		Bucket:         bucket,
		Name:           name,
		ServiceAccount: account,
		Integrity:      observationIntegrity(t, []byte("browser media")),
		ContentType:    contentType,
		Lifetime:       lifetime,
	}
}
