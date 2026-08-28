package filelock_test

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filelock"
)

func TestExclusivityClosesItsEntireByteDomain(t *testing.T) {
	t.Parallel()

	var offWire core.OffWireEnum = filelock.Exclusive
	offWire.OffWireEnum()

	seen := make(map[string]filelock.Exclusivity)
	gotAdmitted := 0
	for value := 0; value <= math.MaxUint8; value++ {
		exclusivity := filelock.Exclusivity(value)
		gotErr := exclusivity.Validate()
		wantValid := exclusivity == filelock.Exclusive || exclusivity == filelock.Shared
		if gotValid := gotErr == nil; gotValid != wantValid {
			t.Fatalf("Exclusivity(%d).Validate() admitted = %t, want %t; error = %v", value, gotValid, wantValid, gotErr)
		}
		if gotIsValid := exclusivity.IsValid(); gotIsValid != wantValid {
			t.Fatalf("Exclusivity(%d).IsValid() = %t, want %t", value, gotIsValid, wantValid)
		}
		if !wantValid {
			if !errors.Is(gotErr, core.ErrPrimitiveContract) {
				t.Fatalf("Exclusivity(%d).Validate() error = %v, want %v", value, gotErr, core.ErrPrimitiveContract)
			}
			if gotDiagnostic := exclusivity.String(); gotDiagnostic != core.UnknownEnumDiagnostic {
				t.Fatalf("Exclusivity(%d).String() = %q, want %q", value, gotDiagnostic, core.UnknownEnumDiagnostic)
			}
			continue
		}
		gotAdmitted++
		gotDiagnostic := exclusivity.String()
		if gotDiagnostic == "" || gotDiagnostic == core.UnknownEnumDiagnostic {
			t.Fatalf("Exclusivity(%d).String() = %q, want a member diagnostic", value, gotDiagnostic)
		}
		if prior, duplicate := seen[gotDiagnostic]; duplicate {
			t.Fatalf("Exclusivity(%d) diagnostic = %q, want distinct from Exclusivity(%d)", value, gotDiagnostic, prior)
		}
		seen[gotDiagnostic] = exclusivity
	}
	if wantAdmitted := 2; gotAdmitted != wantAdmitted {
		t.Fatalf("admitted exclusivities = %d, want %d", gotAdmitted, wantAdmitted)
	}
}

// TestPatienceClosesItsEntireByteDomain walks every backing value: the two
// published patiences must validate and carry unique diagnostic labels, all
// two hundred fifty four others must refuse and render the shared unknown
// diagnostic, and the off-wire declaration is exercised rather than merely
// asserted, so the marker cannot rot into an unreachable ceremony.
func TestPatienceClosesItsEntireByteDomain(t *testing.T) {
	t.Parallel()

	var offWire core.OffWireEnum = filelock.Immediate
	offWire.OffWireEnum()

	seen := make(map[string]filelock.Patience)
	gotAdmitted := 0
	for value := 0; value <= math.MaxUint8; value++ {
		patience := filelock.Patience(value)
		gotErr := patience.Validate()
		wantValid := patience == filelock.Immediate || patience == filelock.Blocking
		if gotValid := gotErr == nil; gotValid != wantValid {
			t.Fatalf("Patience(%d).Validate() admitted = %t, want %t; error = %v", value, gotValid, wantValid, gotErr)
		}
		if gotIsValid := patience.IsValid(); gotIsValid != wantValid {
			t.Fatalf("Patience(%d).IsValid() = %t, want %t", value, gotIsValid, wantValid)
		}
		if !wantValid {
			if !errors.Is(gotErr, core.ErrPrimitiveContract) {
				t.Fatalf("Patience(%d).Validate() error = %v, want %v", value, gotErr, core.ErrPrimitiveContract)
			}
			if gotDiagnostic := patience.String(); gotDiagnostic != core.UnknownEnumDiagnostic {
				t.Fatalf("Patience(%d).String() = %q, want %q", value, gotDiagnostic, core.UnknownEnumDiagnostic)
			}
			continue
		}
		gotAdmitted++
		gotDiagnostic := patience.String()
		if gotDiagnostic == "" || gotDiagnostic == core.UnknownEnumDiagnostic {
			t.Fatalf("Patience(%d).String() = %q, want a member diagnostic", value, gotDiagnostic)
		}
		if prior, duplicate := seen[gotDiagnostic]; duplicate {
			t.Fatalf("Patience(%d) diagnostic = %q, want distinct from Patience(%d)", value, gotDiagnostic, prior)
		}
		seen[gotDiagnostic] = patience
	}
	if wantAdmitted := 2; gotAdmitted != wantAdmitted {
		t.Fatalf("admitted patiences = %d, want %d", gotAdmitted, wantAdmitted)
	}
}

func TestRequestValidationExhaustsValidPolicyProductAndHostileBoundaries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		withFile    bool
		exclusivity filelock.Exclusivity
		patience    filelock.Patience
		wantErr     error
	}{
		{name: "exclusive immediate policy is admitted", withFile: true, exclusivity: filelock.Exclusive, patience: filelock.Immediate},
		{name: "exclusive blocking policy is admitted", withFile: true, exclusivity: filelock.Exclusive, patience: filelock.Blocking},
		{name: "shared immediate policy is admitted", withFile: true, exclusivity: filelock.Shared, patience: filelock.Immediate},
		{name: "shared blocking policy is admitted", withFile: true, exclusivity: filelock.Shared, patience: filelock.Blocking},
		{name: "missing file refuses exclusive immediate policy", exclusivity: filelock.Exclusive, patience: filelock.Immediate, wantErr: core.ErrPrimitiveContract},
		{name: "missing file refuses exclusive blocking policy", exclusivity: filelock.Exclusive, patience: filelock.Blocking, wantErr: core.ErrPrimitiveContract},
		{name: "missing file refuses shared immediate policy", exclusivity: filelock.Shared, patience: filelock.Immediate, wantErr: core.ErrPrimitiveContract},
		{name: "missing file refuses shared blocking policy", exclusivity: filelock.Shared, patience: filelock.Blocking, wantErr: core.ErrPrimitiveContract},
		{name: "zero exclusivity is below the closed domain", withFile: true, patience: filelock.Immediate, wantErr: core.ErrPrimitiveContract},
		{name: "first future exclusivity is above the closed domain", withFile: true, exclusivity: filelock.Exclusivity(3), patience: filelock.Immediate, wantErr: core.ErrPrimitiveContract},
		{name: "maximum exclusivity backing value is refused", withFile: true, exclusivity: filelock.Exclusivity(math.MaxUint8), patience: filelock.Immediate, wantErr: core.ErrPrimitiveContract},
		{name: "zero patience is below the closed domain", withFile: true, exclusivity: filelock.Exclusive, wantErr: core.ErrPrimitiveContract},
		{name: "first future patience is above the closed domain", withFile: true, exclusivity: filelock.Exclusive, patience: filelock.Patience(3), wantErr: core.ErrPrimitiveContract},
		{name: "maximum patience backing value is refused", withFile: true, exclusivity: filelock.Exclusive, patience: filelock.Patience(math.MaxUint8), wantErr: core.ErrPrimitiveContract},
		{name: "zero request refuses every missing contract", wantErr: core.ErrPrimitiveContract},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			request := filelock.Request{Exclusivity: tc.exclusivity, Patience: tc.patience}
			if tc.withFile {
				path := filepath.Join(dir, "request.lock")
				file, gotOpenErr := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
				if gotOpenErr != nil {
					t.Fatalf("OpenFile(%s) error = %v, want nil", path, gotOpenErr)
				}
				t.Cleanup(func() { _ = file.Close() })
				request.File = file
			}

			gotErr := request.Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Request.Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}
