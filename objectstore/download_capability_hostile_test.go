package objectstore

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestDownloadCapabilityProjectionRoundTripBindsExactBearer(t *testing.T) {
	t.Parallel()

	projection := downloadCapabilityProjectionFixture(t, ProviderGoogleCloudStorage)
	encoded, err := projection.MarshalJSON()
	if err != nil {
		t.Fatalf("DownloadCapabilityProjection.MarshalJSON() error = %v, want nil", err)
	}
	var received DownloadCapability
	if err := received.UnmarshalJSON(encoded); err != nil {
		t.Fatalf("DownloadCapability.UnmarshalJSON(projection) error = %v, want nil", err)
	}
	projectionCommitment, err := projection.Commitment()
	if err != nil {
		t.Fatalf("DownloadCapabilityProjection.Commitment() error = %v, want nil", err)
	}
	receivedCommitment, err := received.Commitment()
	if err != nil {
		t.Fatalf("DownloadCapability.Commitment() error = %v, want nil", err)
	}
	if receivedCommitment != projectionCommitment {
		t.Fatalf("received commitment = %v, want projection commitment %v",
			receivedCommitment, projectionCommitment)
	}
	provider, err := received.Provider()
	if err != nil || provider != ProviderGoogleCloudStorage {
		t.Fatalf("DownloadCapability.Provider() = (%v, %v), want (%v, nil)",
			provider, err, ProviderGoogleCloudStorage)
	}
}

func TestDownloadCapabilityStrictDecodeRefusesEveryRequiredMemberLossWithoutMutation(t *testing.T) {
	t.Parallel()

	projection := downloadCapabilityProjectionFixture(t, ProviderAmazonS3)
	encoded, err := projection.MarshalJSON()
	if err != nil {
		t.Fatalf("DownloadCapabilityProjection.MarshalJSON() error = %v, want nil", err)
	}
	var before DownloadCapability
	if err := before.UnmarshalJSON(encoded); err != nil {
		t.Fatalf("DownloadCapability.UnmarshalJSON(valid) error = %v, want nil", err)
	}
	var validWire uploadCapabilityWire
	if err := json.Unmarshal(encoded, &validWire); err != nil {
		t.Fatalf("json.Unmarshal(valid capability wire) error = %v, want nil", err)
	}
	cases := []struct {
		mutate func(*uploadCapabilityWire)
		name   string
	}{
		{name: "provider absent", mutate: func(w *uploadCapabilityWire) { w.Provider = nil }},
		{name: "method absent", mutate: func(w *uploadCapabilityWire) { w.Method = nil }},
		{name: "url absent", mutate: func(w *uploadCapabilityWire) { w.URL = nil }},
		{name: "expiry absent", mutate: func(w *uploadCapabilityWire) { w.ExpiresAt = nil }},
		{name: "empty url", mutate: func(w *uploadCapabilityWire) { empty := ""; w.URL = &empty }},
		{
			name: "upload method substituted",
			mutate: func(w *uploadCapabilityWire) {
				method := UploadMethodTokenSignedPut
				w.Method = &method
			},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			wire := validWire
			testCase.mutate(&wire)
			candidate, marshalErr := core.MarshalCanonicalJSONDocument(wire)
			if marshalErr != nil {
				t.Fatalf("core.MarshalCanonicalJSONDocument(mutated wire) error = %v, want nil", marshalErr)
			}
			got := before
			gotErr := got.UnmarshalJSON(candidate)
			if !errors.Is(gotErr, core.ErrObjectStoreContract) {
				t.Fatalf("DownloadCapability.UnmarshalJSON(mutated) error = %v, want errors.Is %v",
					gotErr, core.ErrObjectStoreContract)
			}
			beforeCommitment, beforeErr := before.Commitment()
			gotCommitment, gotCommitmentErr := got.Commitment()
			if beforeErr != nil || gotCommitmentErr != nil || gotCommitment != beforeCommitment {
				t.Fatalf("rejected decode commitments = (%v, %v, %v, %v), want identical valid receiver",
					gotCommitment, gotCommitmentErr, beforeCommitment, beforeErr)
			}
		})
	}
}

func TestDownloadCapabilityRefusesDocumentsBeyondOwnedBound(t *testing.T) {
	t.Parallel()

	data := bytes.Repeat([]byte{' '}, CapabilityJSONMaximumBytes+1)
	var capability DownloadCapability
	gotErr := capability.UnmarshalJSON(data)
	if !errors.Is(gotErr, core.ErrObjectStoreContract) || !capability.IsZero() {
		t.Fatalf("DownloadCapability.UnmarshalJSON(oversize) = (%v, zero=%v), want errors.Is %v and zero",
			gotErr, capability.IsZero(), core.ErrObjectStoreContract)
	}
}

func TestDownloadCapabilityAndProjectionRedactEveryFormattingPath(t *testing.T) {
	t.Parallel()

	projection := downloadCapabilityProjectionFixture(t, ProviderGoogleCloudStorage)
	encoded, err := projection.MarshalJSON()
	if err != nil {
		t.Fatalf("DownloadCapabilityProjection.MarshalJSON() error = %v, want nil", err)
	}
	var received DownloadCapability
	if err := received.UnmarshalJSON(encoded); err != nil {
		t.Fatalf("DownloadCapability.UnmarshalJSON() error = %v, want nil", err)
	}
	verbs := []string{"%v", "%+v", "%#v", "%s", "%q", "%x", "%X"}
	for _, verb := range verbs {
		if got := fmt.Sprintf(verb, projection); got != core.RedactedValueText {
			t.Fatalf("fmt.Sprintf(%q, projection) = %q, want %q", verb, got, core.RedactedValueText)
		}
		if got := fmt.Sprintf(verb, received); got != core.RedactedValueText {
			t.Fatalf("fmt.Sprintf(%q, received) = %q, want %q", verb, got, core.RedactedValueText)
		}
	}
}

func downloadCapabilityProjectionFixture(t *testing.T, provider Provider) DownloadCapabilityProjection {
	t.Helper()

	headers, err := NewSignedHeaders(nil)
	if err != nil {
		t.Fatalf("NewSignedHeaders(nil) error = %v, want nil", err)
	}
	projection, err := NewDownloadCapabilityProjection(provider, DownloadTarget{
		URL: providerSignedURL(t, provider, DirectionDownload), Headers: headers,
		ExpiresAt: providerFutureInstant(t),
	})
	if err != nil {
		t.Fatalf("NewDownloadCapabilityProjection(%v) error = %v, want nil", provider, err)
	}
	return projection
}
