package gotoolchain

import (
	"bytes"
	json "encoding/json/v2"
	"go/types"
	"golang.org/x/tools/go/packages"
	"path/filepath"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// Go owns these actual missing-import diagnostics. Every permutation must
// publish the same complete diagnostic payload, while retaining multiplicity
// and leaving the observed input order untouched.
func TestAnalysisMetadataDiagnosticOrder(t *testing.T) {
	t.Parallel()
	wire, limits := missingImportDiagnosticWire(t, t.TempDir())
	if len(wire.DepsErrors) != 3 {
		t.Fatalf("real Go producer supplied %d dependency errors, want three distinct imports", len(wire.DepsErrors))
	}
	wantMessages := make([]string, 0, len(wire.DepsErrors)+1)
	if wire.Error != nil {
		wantMessages = append(wantMessages, (packages.Error{Pos: wire.Error.Pos, Msg: wire.Error.Err, Kind: packages.ListError}).Error())
	}
	for _, original := range wire.DepsErrors {
		wantMessages = append(wantMessages, (packages.Error{Pos: original.Pos, Msg: original.Err, Kind: packages.ListError}).Error())
	}
	// The oracle orders Go's complete display payloads, independently of the
	// production comparator and the projection function being exercised.
	slices.Sort(wantMessages)
	for _, tc := range []struct {
		name  string
		order [3]int
	}{
		{name: "source order is retained canonically", order: [3]int{0, 1, 2}},
		{name: "later dependency swap cannot reorder output", order: [3]int{0, 2, 1}},
		{name: "first two dependencies may arrive reversed", order: [3]int{1, 0, 2}},
		{name: "first dependency may arrive last", order: [3]int{1, 2, 0}},
		{name: "last dependency may arrive first", order: [3]int{2, 0, 1}},
		{name: "fully reversed traversal retains every diagnostic", order: [3]int{2, 1, 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			permuted := wire
			permuted.DepsErrors = []analysisErrorWire{wire.DepsErrors[tc.order[0]], wire.DepsErrors[tc.order[1]], wire.DepsErrors[tc.order[2]]}
			before := slices.Clone(permuted.DepsErrors)
			encoded, err := json.Marshal(permuted)
			if err != nil {
				t.Fatal(err)
			}
			units, err := decodeAnalysisMetadata(bytes.NewReader(encoded), limits, types.SizesFor("gc", "arm64"))
			if err != nil || len(units) != 1 {
				t.Fatalf("metadata=%+v/%v, want one admitted incomplete Go package", units, err)
			}
			got := units[0].Errors
			gotMessages := make([]string, 0, len(got))
			for _, diagnostic := range got {
				if diagnostic.Kind != packages.ListError {
					t.Fatalf("diagnostic kind=%v, want Go list error", diagnostic.Kind)
				}
				gotMessages = append(gotMessages, diagnostic.Error())
			}
			if !slices.Equal(gotMessages, wantMessages) || !slices.Equal(permuted.DepsErrors, before) {
				t.Fatalf("diagnostics=%q input=%+v, want canonical %q and preserved %+v", gotMessages, permuted.DepsErrors, wantMessages, before)
			}
			for _, original := range wire.DepsErrors {
				retained := 0
				for _, diagnostic := range got {
					if diagnostic.Pos == original.Pos && diagnostic.Msg == original.Err {
						retained++
					}
				}
				if retained != 1 {
					t.Fatalf("compiler error multiplicity=%d, want one", retained)
				}
			}
			permuted.DepsErrors = append(permuted.DepsErrors, before[0])
			encoded, err = json.Marshal(permuted)
			if err != nil {
				t.Fatal(err)
			}
			repeated, err := decodeAnalysisMetadata(bytes.NewReader(encoded), limits, types.SizesFor("gc", "arm64"))
			if err != nil || len(repeated) != 1 {
				t.Fatalf("duplicate metadata=%+v/%v, want one admitted Go package", repeated, err)
			}
			duplicates := repeated[0].Errors
			if len(duplicates) != len(got)+1 {
				t.Fatalf("duplicate census=%d, want %d retained observations", len(duplicates), len(got)+1)
			}
		})
	}
}

func missingImportDiagnosticWire(t *testing.T, directory string) (analysisPackageWire, Limits) {
	t.Helper()
	root, err := core.ParseAbsolutePath(directory)
	if err != nil {
		t.Fatal(err)
	}
	held, err := filestore.OpenRoot(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := held.Close(); err != nil {
			t.Error(err)
		}
	})
	writeVendorCompilerFixture(t, held, "project/go.mod", "module example.test/diagnostic-order\n\ngo 1.27.1\n")
	writeVendorCompilerFixture(t, held, "project/value.go", "package subject\nimport _ \"example.invalid/first\"\nimport _ \"example.invalid/second\"\nimport _ \"example.invalid/third\"\n")
	working, err := core.ParseAbsolutePath(filepath.Join(root.String(), "project"))
	if err != nil {
		t.Fatal(err)
	}
	limits, err := DefaultLimits()
	if err != nil {
		t.Fatal(err)
	}
	capability, err := Open(t.Context(), Configuration{Workspace: WorkspaceModeDisabled, Limits: limits})
	if err != nil {
		t.Fatal(err)
	}
	data, _, err := capability.execute(t.Context(), working, "list", "-e", "-export", "-compiled", analysisJSONFields, ".")
	if err != nil {
		t.Fatal(err)
	}
	var wire analysisPackageWire
	if err := json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	return wire, limits
}
