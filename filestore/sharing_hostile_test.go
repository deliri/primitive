package filestore_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// TestObserveSharingHoldsItsContractGates pins the door's ingress: a
// terminal context and the unset zero path refuse before any platform ask.
func TestObserveSharingHoldsItsContractGates(t *testing.T) {
	t.Parallel()

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	path, err := core.ParseAbsolutePath(t.TempDir())
	if err != nil {
		t.Fatalf("core.ParseAbsolutePath() error = %v, want nil", err)
	}
	if _, err := filestore.ObserveSharing(cancelled, path); !errors.Is(err, context.Canceled) {
		t.Fatalf("ObserveSharing(cancelled context) error = %v, want errors.Is %v", err, context.Canceled)
	}
	if _, err := filestore.ObserveSharing(t.Context(), core.AbsolutePath{}); !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatalf("ObserveSharing(zero path) error = %v, want errors.Is %v", err, core.ErrFilestoreContract)
	}
}

// TestSharingAdmitsOnlyTheClosedDomain walks every backing value across the
// enum boundary on both sides.
func TestSharingAdmitsOnlyTheClosedDomain(t *testing.T) {
	t.Parallel()

	valid := []filestore.Sharing{filestore.SharingAvailable, filestore.SharingHeld}
	for _, value := range valid {
		if err := value.Validate(); err != nil {
			t.Fatalf("Sharing(%d).Validate() error = %v, want nil", value, err)
		}
		if value.String() == core.UnknownEnumDiagnostic {
			t.Fatalf("Sharing(%d).String() = %q, want a named diagnostic", value, value.String())
		}
	}
	for _, value := range []filestore.Sharing{filestore.SharingUnknown, filestore.SharingHeld + 1, filestore.Sharing(255)} {
		if err := value.Validate(); err == nil {
			t.Fatalf("Sharing(%d).Validate() error = nil, want the domain refusal", value)
		}
		if value.String() != core.UnknownEnumDiagnostic {
			t.Fatalf("Sharing(%d).String() = %q, want %q", value, value.String(), core.UnknownEnumDiagnostic)
		}
	}
}
