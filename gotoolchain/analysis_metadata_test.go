package gotoolchain

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// Each row changes a field actually read by the metadata admission boundary.
// The seed is emitted by cmd/go; synthetic mutations here are decoder tests,
// while toolchain_external_test.go drives full compiler execution.
func TestAnalysisMetadataGraphLayerTriad(t *testing.T) {
	t.Parallel()
	data, limits := compilerMetadataSeed(t, t.TempDir())
	var seed analysisPackageWire
	if err := json.Unmarshal(data, &seed); err != nil {
		t.Fatal(err)
	}
	canonicalSeed, err := json.Marshal(seed)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name                              string
		mutate                            func(*analysisPackageWire) []analysisPackageWire
		wantErr                           error
		wantImports, wantErrors           int
		wantName, wantForTest, wantExport string
	}{
		{name: "cgo pseudo-import has no ordinary dependency unit", mutate: func(w *analysisPackageWire) []analysisPackageWire {
			w.Imports = []string{goCgoImportPath}
			return nil
		}},
		{name: "cgo pseudo-import cannot hide a missing ordinary dependency", mutate: func(w *analysisPackageWire) []analysisPackageWire {
			w.Imports = []string{goCgoImportPath, "missing"}
			return nil
		}, wantErr: core.ErrGoToolchainOutput},
		{name: "compiler package name is retained", mutate: func(w *analysisPackageWire) []analysisPackageWire { w.Name = "renamed"; return nil }, wantName: "renamed"},
		{name: "test owner is retained independently of logical path", mutate: func(w *analysisPackageWire) []analysisPackageWire { w.ForTest = w.ImportPath; return nil }, wantForTest: "example.com/metadata"},
		{name: "compiler export location is retained", mutate: func(w *analysisPackageWire) []analysisPackageWire { w.Export = "/compiler/export.a"; return nil }, wantExport: "/compiler/export.a"},
		{name: "explicit compiler refusal remains a list error", mutate: func(w *analysisPackageWire) []analysisPackageWire {
			w.Error = &analysisErrorWire{Err: "compiler refusal", Pos: "value.go:1"}
			return nil
		}, wantErrors: 1},
		{name: "incomplete closure has a retained refusal even without direct error", mutate: func(w *analysisPackageWire) []analysisPackageWire { w.Incomplete = true; return nil }, wantErrors: 1},
		{name: "incomplete missing import cannot become positive metadata", mutate: func(w *analysisPackageWire) []analysisPackageWire {
			w.Incomplete = true
			w.Imports = []string{"missing"}
			return nil
		}, wantErrors: 1},
		{name: "missing identity refuses a record with valid source files", mutate: func(w *analysisPackageWire) []analysisPackageWire { w.ImportPath = ""; return nil }, wantErr: core.ErrGoToolchainOutput},
		{name: "empty Go source path refuses before filesystem access", mutate: func(w *analysisPackageWire) []analysisPackageWire { w.GoFiles = []string{""}; return nil }, wantErr: core.ErrGoToolchainOutput},
		{name: "relative Go source needs an absolute directory", mutate: func(w *analysisPackageWire) []analysisPackageWire {
			w.Dir = "relative"
			w.GoFiles = []string{"value.go"}
			return nil
		}, wantErr: core.ErrGoToolchainOutput},
		{name: "empty compiled source path refuses before compiler access", mutate: func(w *analysisPackageWire) []analysisPackageWire { w.CompiledGoFiles = []string{""}; return nil }, wantErr: core.ErrGoToolchainOutput},
		{name: "relative compiled source needs an absolute directory", mutate: func(w *analysisPackageWire) []analysisPackageWire {
			w.GoFiles = nil
			w.Dir = "relative"
			w.CompiledGoFiles = []string{"value.go"}
			return nil
		}, wantErr: core.ErrGoToolchainOutput},
		{name: "missing dependency refuses a complete closure", mutate: func(w *analysisPackageWire) []analysisPackageWire { w.Imports = []string{"missing"}; return nil }, wantErr: core.ErrGoToolchainOutput},
		{name: "missing remap target refuses a complete closure", mutate: func(w *analysisPackageWire) []analysisPackageWire {
			w.ImportMap = map[string]string{"alias": "missing"}
			return nil
		}, wantErr: core.ErrGoToolchainOutput},
		{name: "missing remap target cannot hide behind incomplete flag", mutate: func(w *analysisPackageWire) []analysisPackageWire {
			w.Incomplete = true
			w.ImportMap = map[string]string{"alias": "missing"}
			return nil
		}, wantErr: core.ErrGoToolchainOutput},
		{name: "forward dependency refers to a retained producer unit", mutate: func(w *analysisPackageWire) []analysisPackageWire {
			w.Imports = []string{"dependency"}
			return []analysisPackageWire{{ImportPath: "dependency", Name: "dependency"}}
		}, wantImports: 1},
		{name: "repeated identical dependency is idempotent", mutate: func(w *analysisPackageWire) []analysisPackageWire {
			w.Imports = []string{"dependency", "dependency"}
			return []analysisPackageWire{{ImportPath: "dependency", Name: "dependency"}}
		}, wantImports: 1},
		{name: "import alias keeps the producer target identity", mutate: func(w *analysisPackageWire) []analysisPackageWire {
			w.Imports = []string{"dependency"}
			w.ImportMap = map[string]string{"alias": "dependency"}
			return []analysisPackageWire{{ImportPath: "dependency", Name: "dependency"}}
		}, wantImports: 2},
		{name: "conflicting test variants cannot win by stream order", mutate: func(w *analysisPackageWire) []analysisPackageWire {
			w.Imports = []string{"dependency", "dependency [owner.test]"}
			return []analysisPackageWire{{ImportPath: "dependency", Name: "dependency"}, {ImportPath: "dependency [owner.test]", Name: "dependency"}}
		}, wantErr: core.ErrGoToolchainOutput},
		{name: "reversed conflicting test variants remain refused", mutate: func(w *analysisPackageWire) []analysisPackageWire {
			w.Imports = []string{"dependency [owner.test]", "dependency"}
			return []analysisPackageWire{{ImportPath: "dependency [owner.test]", Name: "dependency"}, {ImportPath: "dependency", Name: "dependency"}}
		}, wantErr: core.ErrGoToolchainOutput},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var wire analysisPackageWire
			if err := json.Unmarshal(data, &wire); err != nil {
				t.Fatal(err)
			}
			siblings := tc.mutate(&wire)
			encoded, err := json.Marshal(wire)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Equal(canonicalSeed, encoded) {
				t.Fatalf("mutated wire = %q, want different from compiler seed", encoded)
			}
			for _, sibling := range siblings {
				part, err := json.Marshal(sibling)
				if err != nil {
					t.Fatal(err)
				}
				encoded = append(encoded, part...)
			}
			got, err := decodeAnalysisMetadata(encoded, limits, types.SizesFor("gc", "amd64"))
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("metadata graph error = %v, want %v", err, tc.wantErr)
			}
			if err != nil {
				if got != nil {
					t.Fatalf("refused graph = %+v, want nil", got)
				}
				return
			}
			if len(got) != len(siblings)+1 {
				t.Fatalf("graph units = %d, want %d producer records", len(got), len(siblings)+1)
			}
			unit := got[0]
			wantName, wantExport := seed.Name, seed.Export
			if tc.wantName != "" {
				wantName = tc.wantName
			}
			if tc.wantExport != "" {
				wantExport = tc.wantExport
			}
			if unit.ID != seed.ImportPath || unit.Name != wantName || unit.ForTest != tc.wantForTest || unit.ExportFile != wantExport || len(unit.Errors) != tc.wantErrors || len(unit.Imports) != tc.wantImports {
				t.Fatalf("metadata unit = %+v, want producer identity, name %q, owner %q, export %q, %d errors and %d imports", unit, wantName, tc.wantForTest, wantExport, tc.wantErrors, tc.wantImports)
			}
			if wire.Error != nil && (unit.Errors[0].Msg != wire.Error.Err || unit.Errors[0].Pos != wire.Error.Pos) {
				t.Fatalf("compiler refusal = %+v, want unchanged %+v", unit.Errors, wire.Error)
			}
			for key, target := range unit.Imports {
				if !slices.Contains(got, target) || target.ID != "dependency" {
					t.Fatalf("import %s = %+v, want exact retained dependency", key, target)
				}
			}
		})
	}
}

func TestAnalysisMetadataRepresentationBoundaries(t *testing.T) {
	t.Parallel()
	data, limits := compilerMetadataSeed(t, t.TempDir())
	var seed analysisPackageWire
	if err := json.Unmarshal(data, &seed); err != nil {
		t.Fatal(err)
	}
	canonical, err := json.Marshal(seed)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name           string
		data           []byte
		count, maximum uint32
		noSizes        bool
		wantCount      int
		wantErr        error
	}{
		{name: "canonical compiler record remains admitted", data: canonical, wantCount: 1},
		{name: "harmless stream whitespace creates no extra unit", data: append([]byte(" \t\n"), canonical...), wantCount: 1},
		{name: "unknown additive Go metadata cannot create an extra unit", data: append(append([]byte{}, canonical[:len(canonical)-1]...), []byte(",\"FutureMetadata\":true}")...), wantCount: 1},
		{name: "duplicate member cannot replace compiler identity", data: append(append([]byte{}, canonical[:len(canonical)-1]...), []byte(",\"ImportPath\":\"foreign\"}")...), wantErr: core.ErrGoToolchainOutput},
		{name: "trailing scalar cannot escape as partial package output", data: append(append([]byte{}, canonical...), []byte(" 1")...), wantErr: core.ErrGoToolchainOutput},
		{name: "null record is not empty metadata", data: []byte("null"), wantErr: core.ErrGoToolchainOutput},
		{name: "array cannot stand in for independently framed packages", data: append(append([]byte{'['}, canonical...), ']'), wantErr: core.ErrGoToolchainOutput},
		{name: "numeric identity cannot coerce to a package path", data: []byte(`{"ImportPath":42}`), wantErr: core.ErrGoToolchainOutput},
		{name: "scalar file list cannot coerce to compiler inputs", data: []byte(`{"ImportPath":"subject","GoFiles":"value.go"}`), wantErr: core.ErrGoToolchainOutput},
		{name: "scalar import map cannot coerce to dependencies", data: []byte(`{"ImportPath":"subject","ImportMap":1}`), wantErr: core.ErrGoToolchainOutput},
		{name: "invalid UTF8 cannot enter package identity", data: []byte("{\"ImportPath\":\"\xff\"}"), wantErr: core.ErrGoToolchainOutput},
		{name: "unknown target sizes cannot admit source metadata", data: canonical, noSizes: true, wantErr: core.ErrGoToolchainOutput},
		{name: "package cardinality one below bound", count: 1, maximum: 2, wantCount: 1},
		{name: "package cardinality exactly at bound", count: 2, maximum: 2, wantCount: 2},
		{name: "package cardinality one above bound emits no prefix", count: 3, maximum: 2, wantErr: core.ErrGoToolchainOutput},
		{name: "package cardinality extreme cannot overflow the admission counter", count: PackageMaximumCount + 1, maximum: PackageMaximumCount, wantErr: core.ErrGoToolchainOutput},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			input := bytes.Clone(tc.data)
			bound := limits
			if tc.maximum != 0 {
				bound.PackageMaximum = tc.maximum
			}
			for i := range tc.count {
				wire := seed
				wire.ImportPath = fmt.Sprintf("example.com/unit%d", i)
				encoded, err := json.Marshal(wire)
				if err != nil {
					t.Fatal(err)
				}
				input = append(input, encoded...)
			}
			sizes := types.SizesFor("gc", "amd64")
			if tc.noSizes {
				sizes = nil
			}
			got, err := decodeAnalysisMetadata(input, bound, sizes)
			if !errors.Is(err, tc.wantErr) || len(got) != tc.wantCount {
				t.Fatalf("metadata boundary = %d/%v, want %d/%v", len(got), err, tc.wantCount, tc.wantErr)
			}
			if err != nil && got != nil {
				t.Fatalf("refused metadata = %+v, want zero retained prefix", got)
			}
			for i, unit := range got {
				wantID := seed.ImportPath
				if tc.count != 0 {
					wantID = fmt.Sprintf("example.com/unit%d", i)
				}
				wantFiles := []string{filepath.Join(seed.Dir, "value.go")}
				if unit.ID != wantID || !slices.Equal(unit.CompiledGoFiles, wantFiles) {
					t.Fatalf("retained unit = %s/%q, want %s/%q", unit.ID, unit.CompiledGoFiles, wantID, wantFiles)
				}
			}
		})
	}
}

// Seed bytes come from the real contained Go command, not a handwritten copy
// of its JSON protocol. The fixture owns no mutable state after construction.
func compilerMetadataSeed(t testing.TB, directory string) ([]byte, Limits) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(directory, "go.mod"), []byte("module example.com/metadata\n\ngo 1.27.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "value.go"), []byte("package metadata\nconst Value = 7\n"), 0o600); err != nil {
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
	root, err := core.ParseAbsolutePath(directory)
	if err != nil {
		t.Fatal(err)
	}
	data, _, err := capability.execute(t.Context(), root, "list", "-e", goDependenciesArgument, "-export", "-compiled", goModuleReadOnly, analysisJSONFields, ".")
	if err != nil || len(data) == 0 {
		t.Fatalf("compiler seed = %d bytes/%v, want nonempty real metadata", len(data), err)
	}
	return data, limits
}

func TestAnalysisMetadataLayerTriadRetainsBoundsAndIdentity(t *testing.T) {
	t.Parallel()
	data, limits := compilerMetadataSeed(t, t.TempDir())
	sizes := types.SizesFor("gc", "amd64")
	for _, tc := range []struct {
		name      string
		mutate    func([]byte) []byte
		maximum   int
		wantCount int
		wantErr   error
	}{
		{name: "real compiler metadata binds the source unit", wantCount: 1},
		{name: "empty stream remains no metadata", mutate: func([]byte) []byte { return nil }},
		{name: "truncated record emits no partial metadata", mutate: func(b []byte) []byte { return b[:len(b)/2] }, wantErr: core.ErrGoToolchainOutput},
		{name: "duplicate identity cannot make two compiler units", mutate: func(b []byte) []byte { return append(b, b...) }, wantErr: core.ErrGoToolchainOutput},
		{name: "byte ceiling one below complete record refuses", maximum: len(data) - 1, wantErr: core.ErrGoToolchainOutput},
		{name: "byte ceiling exactly fits complete record", maximum: len(data), wantCount: 1},
		{name: "byte ceiling one above complete record admits", maximum: len(data) + 1, wantCount: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			input := bytes.Clone(data)
			if tc.mutate != nil {
				input = tc.mutate(input)
				if bytes.Equal(input, data) {
					t.Fatalf("mutation bytes = %q, want different from the %d byte seed", input, len(data))
				}
			}
			bound := limits
			if tc.maximum != 0 {
				var err error
				bound.OutputBytes, err = core.NewByteCount(uint64(tc.maximum))
				if err != nil {
					t.Fatal(err)
				}
			}
			got, err := decodeAnalysisMetadata(input, bound, sizes)
			if !errors.Is(err, tc.wantErr) || len(got) != tc.wantCount {
				t.Fatalf("decode = %d units/%v, want %d/%v", len(got), err, tc.wantCount, tc.wantErr)
			}
			if len(got) > 0 && (got[0].ID != "example.com/metadata" || len(got[0].CompiledGoFiles) != 1 || filepath.Base(got[0].CompiledGoFiles[0]) != "value.go") {
				t.Fatalf("compiler metadata = %+v, want exact fixture package and compiled file", got[0])
			}
		})
	}
}

func FuzzAnalysisMetadataSemanticClosure(f *testing.F) {
	data, limits := compilerMetadataSeed(f, f.TempDir())
	f.Add(data)
	f.Add([]byte{})
	f.Add(append(bytes.Clone(data), data...))
	sizes := types.SizesFor("gc", "amd64")
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := decodeAnalysisMetadata(data, limits, sizes)
		if err != nil {
			if !errors.Is(err, core.ErrGoToolchainOutput) || got != nil {
				t.Fatalf("metadata refusal = %d units/%v, want nil and typed output refusal", len(got), err)
			}
			return
		}
		if len(got) > int(limits.PackageMaximum) {
			t.Fatalf("metadata count = %d, want <= %d", len(got), limits.PackageMaximum)
		}
		identities := make(map[string]bool, len(got))
		for _, unit := range got {
			if unit == nil || unit.ID == "" || identities[unit.ID] {
				t.Fatalf("metadata identity = %+v, want unique nonempty unit", unit)
			}
			identities[unit.ID] = true
			for _, path := range unit.CompiledGoFiles {
				if !filepath.IsAbs(path) {
					t.Fatalf("compiler path = %q, want absolute", path)
				}
			}
			for _, dependency := range unit.Imports {
				if !slices.Contains(got, dependency) {
					t.Fatalf("dependency = %+v, want a producer-emitted unit", dependency)
				}
			}
		}
	})
}
