package objectstore

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzTransferEvidenceJSONSemanticClosure(f *testing.F) {
	transfer := sealedTransferEvidenceFixture(f, transferEvidenceFixtureRequest{
		Provider: ProviderGoogleCloudStorage, Direction: DirectionUpload,
		Version: "7", Bytes: 4096,
	})
	projection, err := transfer.Evidence()
	if err != nil {
		f.Fatalf("Transfer.Evidence(seed) error = %v, want nil", err)
	}
	canonical, err := projection.MarshalJSON()
	if err != nil {
		f.Fatalf("TransferEvidenceProjection.MarshalJSON(seed) error = %v, want nil", err)
	}
	var preserved TransferEvidence
	if err := preserved.UnmarshalJSON(canonical); err != nil {
		f.Fatalf("TransferEvidence.UnmarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte(`{}`))
	f.Add(append(bytes.Clone(canonical), 0))

	f.Fuzz(func(t *testing.T, data []byte) {
		got := preserved
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) ||
				!errors.Is(gotErr, core.ErrObjectStoreContract) || got != preserved {
				t.Fatalf("TransferEvidence.UnmarshalJSON(rejected) = (%v, %v), want preserved and typed JSON/Objectstore rejection",
					got, gotErr)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("TransferEvidence.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
		}
		encoded, err := got.MarshalJSON()
		if err != nil || len(encoded) > TransferEvidenceJSONMaximumBytes {
			t.Fatalf("TransferEvidence.MarshalJSON(accepted) = (%d bytes, %v), want <= %d and nil",
				len(encoded), err, TransferEvidenceJSONMaximumBytes)
		}
		var roundTrip TransferEvidence
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
			t.Fatalf("TransferEvidence canonical round trip = (%v, %v), want exact %v and nil", roundTrip, err, got)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("TransferEvidence second canonical projection = (%q, %v), want %q and nil", second, err, encoded)
		}
	})
}

func FuzzDownloadCapabilityJSONSemanticClosure(f *testing.F) {
	projection := downloadCapabilityProjectionFixture(f, ProviderGoogleCloudStorage)
	canonical, err := projection.MarshalJSON()
	if err != nil {
		f.Fatalf("DownloadCapabilityProjection.MarshalJSON(seed) error = %v, want nil", err)
	}
	var preserved DownloadCapability
	if err := preserved.UnmarshalJSON(canonical); err != nil {
		f.Fatalf("DownloadCapability.UnmarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte(`{}`))
	f.Add(append(bytes.Clone(canonical), 0))

	f.Fuzz(func(t *testing.T, data []byte) {
		got := preserved
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrObjectStoreContract) || !sameDownloadCapability(got, preserved) {
				t.Fatalf("DownloadCapability.UnmarshalJSON(rejected) = (%v, %v), want preserved and typed Objectstore rejection",
					got, gotErr)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("DownloadCapability.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
		}
		provider, err := got.Provider()
		if err != nil {
			t.Fatalf("DownloadCapability.Provider(accepted) error = %v, want nil", err)
		}
		target, err := got.Target()
		if err != nil {
			t.Fatalf("DownloadCapability.Target(accepted) error = %v, want nil", err)
		}
		issued, err := NewDownloadCapabilityProjection(provider, target)
		if err != nil {
			t.Fatalf("NewDownloadCapabilityProjection(accepted) error = %v, want nil", err)
		}
		encoded, err := issued.MarshalJSON()
		if err != nil || len(encoded) > CapabilityJSONMaximumBytes {
			t.Fatalf("DownloadCapabilityProjection.MarshalJSON(accepted) = (%d bytes, %v), want <= %d and nil",
				len(encoded), err, CapabilityJSONMaximumBytes)
		}
		var roundTrip DownloadCapability
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || !sameDownloadCapability(roundTrip, got) {
			t.Fatalf("DownloadCapability canonical round trip error = %v, want exact commitment and nil", err)
		}
	})
}

func FuzzUploadCapabilityCommitmentJSONSemanticClosure(f *testing.F) {
	target := providerUploadTarget(f, ProviderGoogleCloudStorage)
	projection, err := NewUploadCapabilityProjection(ProviderGoogleCloudStorage, target)
	if err != nil {
		f.Fatalf("NewUploadCapabilityProjection(seed) error = %v, want nil", err)
	}
	preserved, err := projection.Commitment()
	if err != nil {
		f.Fatalf("UploadCapabilityProjection.Commitment(seed) error = %v, want nil", err)
	}
	fuzzUploadCommitmentJSON(f, preserved)
}

func fuzzUploadCommitmentJSON(f *testing.F, preserved UploadCapabilityCommitment) {
	f.Helper()
	canonical, err := preserved.MarshalJSON()
	if err != nil {
		f.Fatalf("UploadCapabilityCommitment.MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte(`null`))
	f.Add(append(bytes.Clone(canonical), 0))
	f.Fuzz(func(t *testing.T, data []byte) {
		got := preserved
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrObjectStoreContract) || got != preserved {
				t.Fatalf("UploadCapabilityCommitment.UnmarshalJSON(rejected) = (%v, %v), want preserved and typed Objectstore rejection",
					got, gotErr)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("UploadCapabilityCommitment.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
		}
		encoded, err := got.MarshalJSON()
		if err != nil {
			t.Fatalf("UploadCapabilityCommitment.MarshalJSON(accepted) error = %v, want nil", err)
		}
		var roundTrip UploadCapabilityCommitment
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
			t.Fatalf("UploadCapabilityCommitment canonical round trip = (%v, %v), want exact %v and nil", roundTrip, err, got)
		}
	})
}

func FuzzDownloadCapabilityCommitmentJSONSemanticClosure(f *testing.F) {
	projection := downloadCapabilityProjectionFixture(f, ProviderGoogleCloudStorage)
	preserved, err := projection.Commitment()
	if err != nil {
		f.Fatalf("DownloadCapabilityProjection.Commitment(seed) error = %v, want nil", err)
	}
	canonical, err := preserved.MarshalJSON()
	if err != nil {
		f.Fatalf("DownloadCapabilityCommitment.MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte(`null`))
	f.Add(append(bytes.Clone(canonical), 0))

	f.Fuzz(func(t *testing.T, data []byte) {
		got := preserved
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrObjectStoreContract) || got != preserved {
				t.Fatalf("DownloadCapabilityCommitment.UnmarshalJSON(rejected) = (%v, %v), want preserved and typed Objectstore rejection",
					got, gotErr)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("DownloadCapabilityCommitment.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
		}
		encoded, err := got.MarshalJSON()
		if err != nil {
			t.Fatalf("DownloadCapabilityCommitment.MarshalJSON(accepted) error = %v, want nil", err)
		}
		var roundTrip DownloadCapabilityCommitment
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
			t.Fatalf("DownloadCapabilityCommitment canonical round trip = (%v, %v), want exact %v and nil", roundTrip, err, got)
		}
	})
}
