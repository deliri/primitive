package projectstandards

import (
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestInventoryHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	coverageZero := uint16(0)
	coverageFull := uint16(10_000)
	coverageAbove := uint16(10_001)
	cases := []struct {
		name    string
		in      Inventory
		wantErr error
	}{
		{name: "one non-Go document is a complete one-file inventory", in: Inventory{Files: 1, Documents: 1}},
		{name: "one Go package can occupy one file", in: Inventory{Files: 1, GoPackages: 1}},
		{name: "one JavaScript unit can occupy one file", in: Inventory{Files: 1, JavaScriptUnits: 1}},
		{name: "one test file may contain the declaration ceiling", in: Inventory{Files: 1, TestFiles: 1, TestDeclarations: 256}},
		{name: "benchmarks may account for every test declaration", in: Inventory{Files: 2, TestFiles: 1, TestDeclarations: 2, Benchmarks: 2}},
		{name: "fuzz targets may account for every test declaration", in: Inventory{Files: 2, TestFiles: 1, TestDeclarations: 2, FuzzTargets: 2}},
		{name: "benchmarks and fuzz targets may partition declarations", in: Inventory{Files: 3, TestFiles: 1, TestDeclarations: 4, Benchmarks: 2, FuzzTargets: 2}},
		{name: "zero observed coverage is explicit and valid", in: Inventory{Files: 1, CoverageBasisPoints: &coverageZero}},
		{name: "complete observed coverage is valid", in: Inventory{Files: 1, CoverageBasisPoints: &coverageFull}},
		{name: "large internally closed inventory remains valid", in: Inventory{Files: math.MaxUint32, GoPackages: math.MaxUint32, JavaScriptUnits: math.MaxUint32, TestFiles: math.MaxUint32, Documents: math.MaxUint32, TestDeclarations: math.MaxUint32, Benchmarks: math.MaxUint32}},

		{name: "zero files cannot claim an empty tree as observed", in: Inventory{}, wantErr: core.ErrProjectStandardsContract},
		{name: "Go packages cannot exceed files", in: Inventory{Files: 1, GoPackages: 2}, wantErr: core.ErrProjectStandardsContract},
		{name: "JavaScript units cannot exceed files", in: Inventory{Files: 1, JavaScriptUnits: 2}, wantErr: core.ErrProjectStandardsContract},
		{name: "test files cannot exceed files", in: Inventory{Files: 1, TestFiles: 2}, wantErr: core.ErrProjectStandardsContract},
		{name: "documents cannot exceed files", in: Inventory{Files: 1, Documents: 2}, wantErr: core.ErrProjectStandardsContract},
		{name: "one test file cannot claim 257 declarations", in: Inventory{Files: 1, TestFiles: 1, TestDeclarations: 257}, wantErr: core.ErrProjectStandardsConflict},
		{name: "benchmarks cannot exceed declarations", in: Inventory{Files: 1, TestFiles: 1, Benchmarks: 1}, wantErr: core.ErrProjectStandardsConflict},
		{name: "fuzz targets cannot exceed declarations", in: Inventory{Files: 1, TestFiles: 1, FuzzTargets: 1}, wantErr: core.ErrProjectStandardsConflict},
		{name: "benchmarks plus fuzz targets cannot exceed declarations", in: Inventory{Files: 1, TestFiles: 1, TestDeclarations: 1, Benchmarks: 1, FuzzTargets: 1}, wantErr: core.ErrProjectStandardsConflict},
		{name: "coverage cannot exceed one hundred percent", in: Inventory{Files: 1, CoverageBasisPoints: &coverageAbove}, wantErr: core.ErrProjectStandardsContract},

		{name: "Go package count at file count is accepted", in: Inventory{Files: 8, GoPackages: 8}},
		{name: "Go package count one below file count is accepted", in: Inventory{Files: 8, GoPackages: 7}},
		{name: "Go package count one above file count is refused", in: Inventory{Files: 8, GoPackages: 9}, wantErr: core.ErrProjectStandardsContract},
		{name: "Go package extreme above one file is refused", in: Inventory{Files: 1, GoPackages: math.MaxUint32}, wantErr: core.ErrProjectStandardsContract},
		{name: "JavaScript unit count at file count is accepted", in: Inventory{Files: 8, JavaScriptUnits: 8}},
		{name: "JavaScript unit count one below file count is accepted", in: Inventory{Files: 8, JavaScriptUnits: 7}},
		{name: "JavaScript unit count one above file count is refused", in: Inventory{Files: 8, JavaScriptUnits: 9}, wantErr: core.ErrProjectStandardsContract},
		{name: "JavaScript unit extreme above one file is refused", in: Inventory{Files: 1, JavaScriptUnits: math.MaxUint32}, wantErr: core.ErrProjectStandardsContract},
		{name: "test declaration count at per-file ceiling is accepted", in: Inventory{Files: 4, TestFiles: 4, TestDeclarations: 1024}},
		{name: "test declaration count one below per-file ceiling is accepted", in: Inventory{Files: 4, TestFiles: 4, TestDeclarations: 1023}},
		{name: "test declaration count one above per-file ceiling is refused", in: Inventory{Files: 4, TestFiles: 4, TestDeclarations: 1025}, wantErr: core.ErrProjectStandardsConflict},
		{name: "test declaration extreme with one test file is refused", in: Inventory{Files: 1, TestFiles: 1, TestDeclarations: math.MaxUint32}, wantErr: core.ErrProjectStandardsConflict},
		{name: "benchmark and fuzz sum at declarations is accepted", in: Inventory{Files: 4, TestFiles: 2, TestDeclarations: 20, Benchmarks: 10, FuzzTargets: 10}},
		{name: "benchmark and fuzz sum one below declarations is accepted", in: Inventory{Files: 4, TestFiles: 2, TestDeclarations: 20, Benchmarks: 10, FuzzTargets: 9}},
		{name: "benchmark and fuzz sum one above declarations is refused", in: Inventory{Files: 4, TestFiles: 2, TestDeclarations: 20, Benchmarks: 11, FuzzTargets: 10}, wantErr: core.ErrProjectStandardsConflict},
		{name: "benchmark and fuzz extreme sum is refused without overflow", in: Inventory{Files: math.MaxUint32, TestFiles: math.MaxUint32, TestDeclarations: math.MaxUint32, Benchmarks: math.MaxUint32, FuzzTargets: math.MaxUint32}, wantErr: core.ErrProjectStandardsConflict},
		{name: "coverage at zero is accepted", in: Inventory{Files: 8, CoverageBasisPoints: &coverageZero}},
		{name: "coverage at ceiling is accepted", in: Inventory{Files: 8, CoverageBasisPoints: &coverageFull}},
		{name: "coverage one above ceiling is refused", in: Inventory{Files: 8, CoverageBasisPoints: &coverageAbove}, wantErr: core.ErrProjectStandardsContract},
		{name: "absent coverage stays neutral", in: Inventory{Files: 8, TestFiles: 2, TestDeclarations: 4}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotErr := tc.in.Validate()
			if tc.wantErr == nil && gotErr != nil {
				t.Fatalf("Inventory.Validate() error = %v, want nil", gotErr)
			}
			if tc.wantErr != nil && !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Inventory.Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}
