package release

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// cmd/go's moduleInfo emits Main only for a module with no selected version,
// returning before dependency sums are filled. ModulePublic.Error reports a
// failed observation independently of the enclosing package's Error field.
func TestGoModuleObservationLayerTriadRefusesConflictingOrFailedFacts(t *testing.T) {
	t.Parallel()
	main := goListModuleWire{Path: testMainModule, Main: true}
	dependency := goListModuleWire{Path: "example.com/dependency", Version: "v1.2.3", Sum: testModuleSumB}
	versionedMain, checksummedMain, failedMain, failedDependency := main, main, main, dependency
	versionedMain.Version = dependency.Version
	checksummedMain.Sum = dependency.Sum
	failedMain.Error = &goListErrorWire{Err: "module observation failed"}
	failedDependency.Error = failedMain.Error
	for _, tc := range []struct {
		name    string
		module  goListModuleWire
		want    []buildDependencyWire
		wantErr error
	}{
		{name: "main-only observation retains an explicitly empty dependency closure", module: main},
		{name: "dependency observation preserves selected version and checksum", module: dependency, want: []buildDependencyWire{{Path: dependency.Path, Version: dependency.Version, Sum: dependency.Sum}}},
		{name: "main flag cannot erase a selected dependency version", module: versionedMain, wantErr: core.ErrReleaseContract},
		{name: "main flag cannot erase a dependency checksum", module: checksummedMain, wantErr: core.ErrReleaseContract},
		{name: "failed main module cannot become a valid root", module: failedMain, wantErr: core.ErrReleaseContract},
		{name: "failed dependency cannot become retained build evidence", module: failedDependency, wantErr: core.ErrReleaseContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var source bytes.Buffer
			for _, module := range []goListModuleWire{main, tc.module} {
				wire := goListPackageWire{ImportPath: module.Path + "/package", Module: &module}
				encoded, err := json.Marshal(wire)
				if err != nil {
					t.Fatalf("Go Marshal(module observation) error = %v, want nil", err)
				}
				source.Write(encoded)
			}
			// A reused target buffer must also lose stale facts on refusal.
			got := dependencyObservation{main: mustModulePath(t, "example.com/previous"), modules: numberedModules(t, 1)}
			err := decodeBuildDependencies(&source, &got)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("decodeBuildDependencies() error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got.main != (GoModulePath{}) || len(got.modules) != 0 {
					t.Fatalf("refused module observation = %v, want zero root and no modules", got)
				}
				return
			}
			if got.main.String() != main.Path || len(got.modules) != len(tc.want) {
				t.Fatalf("module observation = (%v, %d modules), want (%s, %d)", got.main, len(got.modules), main.Path, len(tc.want))
			}
			for index, want := range tc.want {
				module := got.modules[index]
				if module.Path().String() != want.Path || module.Version().String() != want.Version || module.Sum().String() != want.Sum {
					t.Fatalf("module %d = %v, want exact selected facts %v", index, module, want)
				}
			}
		})
	}
}
