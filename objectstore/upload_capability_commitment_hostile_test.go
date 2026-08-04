package objectstore

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// TestUploadCapabilityCommitmentBindsTheExactIssuerAndReceiverCapability proves
// a higher protocol can sign a non-secret commitment beside the bearer without
// making the decode-only capability serializable. The independent digest uses
// the exact capability bytes carried by the enclosing document.
func TestUploadCapabilityCommitmentBindsTheExactIssuerAndReceiverCapability(t *testing.T) {
	t.Parallel()

	projection := uploadCapabilityCommitmentProjection(
		t,
		ProviderGoogleCloudStorage,
		capabilityGCSMetadataRunURL,
		1893456000000000000,
		"run-<&>-41",
	)
	commitment, gotCommitmentErr := projection.Commitment()
	if gotCommitmentErr != nil {
		t.Fatalf("UploadCapabilityProjection.Commitment() error = %v, want nil", gotCommitmentErr)
	}
	if gotValidateErr := commitment.Validate(); gotValidateErr != nil {
		t.Fatalf("UploadCapabilityCommitment.Validate() error = %v, want nil", gotValidateErr)
	}

	capabilityWire, gotCapabilityErr := projection.MarshalJSON()
	if gotCapabilityErr != nil {
		t.Fatalf("UploadCapabilityProjection.MarshalJSON() error = %v, want nil", gotCapabilityErr)
	}
	gotCommitmentWire, gotCommitmentWireErr := json.Marshal(commitment)
	if gotCommitmentWireErr != nil {
		t.Fatalf("json.Marshal(UploadCapabilityCommitment) error = %v, want nil", gotCommitmentWireErr)
	}
	wantCommitmentWire := independentUploadCapabilityCommitmentWire(t, capabilityWire)
	if string(gotCommitmentWire) != string(wantCommitmentWire) {
		t.Fatalf("commitment wire = %s, want independent digest %s", gotCommitmentWire, wantCommitmentWire)
	}

	type issuerEnvelope struct {
		Capability UploadCapabilityProjection `json:"capability"`
		Commitment UploadCapabilityCommitment `json:"commitment"`
	}
	type receiverEnvelope struct {
		Capability UploadCapability           `json:"capability"`
		Commitment UploadCapabilityCommitment `json:"commitment"`
	}
	envelopeWire, gotEnvelopeErr := json.Marshal(issuerEnvelope{
		Capability: projection,
		Commitment: commitment,
	})
	if gotEnvelopeErr != nil {
		t.Fatalf("json.Marshal(issuer envelope) error = %v, want nil", gotEnvelopeErr)
	}
	var received receiverEnvelope
	if gotDecodeErr := json.Unmarshal(envelopeWire, &received); gotDecodeErr != nil {
		t.Fatalf("json.Unmarshal(receiver envelope) error = %v, want nil", gotDecodeErr)
	}
	receivedCommitment, gotReceivedErr := received.Capability.Commitment()
	if gotReceivedErr != nil {
		t.Fatalf("UploadCapability.Commitment() error = %v, want nil", gotReceivedErr)
	}
	if receivedCommitment != received.Commitment {
		t.Fatalf("receiver-derived commitment = %v, want signed envelope commitment %v",
			receivedCommitment, received.Commitment)
	}
}

// TestUploadCapabilityCommitmentRejectsCrossCapabilityReuse pressures every
// product-neutral fact the commitment closes. The neutral reconstruction also
// proves the result is deterministic rather than instance-bound.
func TestUploadCapabilityCommitmentRejectsCrossCapabilityReuse(t *testing.T) {
	t.Parallel()

	base := uploadCapabilityCommitmentProjection(
		t,
		ProviderGoogleCloudStorage,
		capabilityGCSMetadataRunURL,
		1893456000000000000,
		"run-41",
	)
	wantBase, gotBaseErr := base.Commitment()
	if gotBaseErr != nil {
		t.Fatalf("base Commitment() error = %v, want nil", gotBaseErr)
	}

	cases := []struct {
		name      string
		rawURL    string
		header    string
		expiresAt int64
		provider  Provider
		wantEqual bool
	}{
		{
			name:      "same semantic capability has the same commitment",
			provider:  ProviderGoogleCloudStorage,
			rawURL:    capabilityGCSMetadataRunURL,
			expiresAt: 1893456000000000000,
			header:    "run-41",
			wantEqual: true,
		},
		{
			name:      "one changed signature byte changes the commitment",
			provider:  ProviderGoogleCloudStorage,
			rawURL:    strings.Replace(capabilityGCSMetadataRunURL, "signature", "signaturf", 1),
			expiresAt: 1893456000000000000,
			header:    "run-41",
		},
		{
			name:      "expiry one nanosecond before changes the commitment",
			provider:  ProviderGoogleCloudStorage,
			rawURL:    capabilityGCSMetadataRunURL,
			expiresAt: 1893455999999999999,
			header:    "run-41",
		},
		{
			name:      "one changed signed-header byte changes the commitment",
			provider:  ProviderGoogleCloudStorage,
			rawURL:    capabilityGCSMetadataRunURL,
			expiresAt: 1893456000000000000,
			header:    "run-42",
		},
		{
			name:      "a different published provider capability cannot reuse the commitment",
			provider:  ProviderAmazonS3,
			rawURL:    capabilityS3MetadataURL,
			expiresAt: 1893456000000000000,
			header:    "run-41",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			projection := uploadCapabilityCommitmentProjection(
				t, tc.provider, tc.rawURL, tc.expiresAt, tc.header,
			)
			got, gotErr := projection.Commitment()
			if gotErr != nil {
				t.Fatalf("Commitment() error = %v, want nil", gotErr)
			}
			if (got == wantBase) != tc.wantEqual {
				t.Fatalf("Commitment() equals base = %t, want %t", got == wantBase, tc.wantEqual)
			}
		})
	}
}

func independentUploadCapabilityCommitmentWire(
	t *testing.T,
	capabilityWire []byte,
) []byte {
	t.Helper()

	digest := sha256.New()
	_, _ = digest.Write([]byte(UploadCapabilityCommitmentDomain))
	_, _ = digest.Write([]byte{UploadCapabilityCommitmentFrameSeparator})
	_, _ = digest.Write(capabilityWire)
	var wantDigestBytes [sha256.Size]byte
	copy(wantDigestBytes[:], digest.Sum(nil))
	wantDigest := core.NewSHA256Digest(wantDigestBytes)
	wantCommitmentWire, gotWantWireErr := json.Marshal(wantDigest)
	if gotWantWireErr != nil {
		t.Fatalf("json.Marshal(independent SHA-256 digest) error = %v, want nil", gotWantWireErr)
	}
	return wantCommitmentWire
}

// TestUploadCapabilityCommitmentJSONPreservesReceiverOnRejection proves the
// signed-response boundary cannot erase or replace a previously authenticated
// binding with malformed input.
func TestUploadCapabilityCommitmentJSONPreservesReceiverOnRejection(t *testing.T) {
	t.Parallel()

	projection := uploadCapabilityCommitmentProjection(
		t,
		ProviderGoogleCloudStorage,
		capabilityGCSMetadataRunURL,
		1893456000000000000,
		"run-41",
	)
	preserved, gotCommitmentErr := projection.Commitment()
	if gotCommitmentErr != nil {
		t.Fatalf("Commitment() error = %v, want nil", gotCommitmentErr)
	}
	preservedWire, gotWireErr := json.Marshal(preserved)
	if gotWireErr != nil {
		t.Fatalf("json.Marshal(preserved commitment) error = %v, want nil", gotWireErr)
	}
	hexToken := strings.Trim(string(preservedWire), `"`)
	cases := []struct {
		name string
		wire string
	}{
		{name: "empty document is rejected", wire: ""},
		{name: "null commitment is rejected", wire: "null"},
		{name: "non-string commitment is rejected", wire: "41"},
		{name: "digest one hexadecimal byte short is rejected", wire: `"` + hexToken[:len(hexToken)-1] + `"`},
		{name: "digest one hexadecimal byte long is rejected", wire: `"` + hexToken + `0"`},
		{name: "uppercase hexadecimal is rejected as non-canonical", wire: `"` + strings.ToUpper(hexToken) + `"`},
		{name: "trailing document is rejected", wire: string(preservedWire) + "null"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := preserved
			gotErr := got.UnmarshalJSON([]byte(tc.wire))
			if !errors.Is(gotErr, core.ErrObjectStoreContract) {
				t.Fatalf("UploadCapabilityCommitment.UnmarshalJSON() error = %v, want %v",
					gotErr, core.ErrObjectStoreContract)
			}
			if got != preserved {
				t.Fatalf("commitment after rejected decode = %v, want preserved %v", got, preserved)
			}
		})
	}

	var zero UploadCapabilityCommitment
	if gotValidateErr := zero.Validate(); !errors.Is(gotValidateErr, core.ErrObjectStoreContract) {
		t.Fatalf("zero UploadCapabilityCommitment.Validate() error = %v, want %v",
			gotValidateErr, core.ErrObjectStoreContract)
	}
	if gotWire, gotMarshalErr := zero.MarshalJSON(); !errors.Is(gotMarshalErr, core.ErrObjectStoreContract) || gotWire != nil {
		t.Fatalf("zero UploadCapabilityCommitment.MarshalJSON() = (%q, %v), want (nil, %v)",
			gotWire, gotMarshalErr, core.ErrObjectStoreContract)
	}
	var nilReceiver *UploadCapabilityCommitment
	if gotErr := nilReceiver.UnmarshalJSON(preservedWire); !errors.Is(gotErr, core.ErrObjectStoreContract) {
		t.Fatalf("nil UploadCapabilityCommitment.UnmarshalJSON() error = %v, want %v",
			gotErr, core.ErrObjectStoreContract)
	}
}

func uploadCapabilityCommitmentProjection(
	t *testing.T,
	provider Provider,
	rawURL string,
	expiresAt int64,
	headerValue string,
) UploadCapabilityProjection {
	t.Helper()

	signedURL, gotURLErr := ParseSignedURL(rawURL)
	if gotURLErr != nil {
		t.Fatalf("ParseSignedURL() error = %v, want nil", gotURLErr)
	}
	headerName, gotNameErr := core.ParseHTTPHeaderName("X-Goog-Meta-Run")
	if provider == ProviderAmazonS3 {
		headerName, gotNameErr = core.ParseHTTPHeaderName("X-Amz-Meta-Run")
	}
	if gotNameErr != nil {
		t.Fatalf("core.ParseHTTPHeaderName() error = %v, want nil", gotNameErr)
	}
	header, gotHeaderErr := NewSignedHeader(headerName, headerValue)
	if gotHeaderErr != nil {
		t.Fatalf("NewSignedHeader() error = %v, want nil", gotHeaderErr)
	}
	headers, gotHeadersErr := NewSignedHeaders([]SignedHeader{header})
	if gotHeadersErr != nil {
		t.Fatalf("NewSignedHeaders() error = %v, want nil", gotHeadersErr)
	}
	projection, gotProjectionErr := NewUploadCapabilityProjection(
		provider,
		UploadTarget{
			Headers:   headers,
			URL:       signedURL,
			ExpiresAt: temporal.InstantFromNanoseconds(expiresAt),
		},
	)
	if gotProjectionErr != nil {
		t.Fatalf("NewUploadCapabilityProjection() error = %v, want nil", gotProjectionErr)
	}
	return projection
}
