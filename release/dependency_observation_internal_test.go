package release

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// Each accepted stream is checked against complete independently declared
// module facts. Refusals must clear a previously populated target buffer.
func TestDecodeBuildDependenciesLayerTriadPressuresGoListProtocol(t *testing.T) {
	t.Parallel()
	main := goListPackage(goListModuleWire{Path: testMainModule, Main: true})
	depA := goListPackage(goListModuleWire{Path: "example.com/a", Version: "v1.2.3", Sum: testModuleSumA})
	depB := goListPackage(goListModuleWire{Path: "example.com/b/v2", Version: "v2.0.0-20260804010203-0123456789ab", Sum: testModuleSumB})
	wantA := buildDependencyWire{Path: "example.com/a", Version: "v1.2.3", Sum: testModuleSumA}
	wantB := buildDependencyWire{Path: "example.com/b/v2", Version: "v2.0.0-20260804010203-0123456789ab", Sum: testModuleSumB}
	standard := goListPackageWire{ImportPath: "io", Standard: true}
	for _, tc := range []struct {
		name    string
		input   []byte
		want    []buildDependencyWire
		wantErr error
	}{
		{name: "main-only stream clears previous target modules", input: goListStreamFixture(t, main)},
		{name: "standard package creates no dependency fact", input: goListStreamFixture(t, standard, main)},
		{name: "dependency retains exact path version and sum", input: goListStreamFixture(t, depA, main), want: []buildDependencyWire{wantA}},
		{name: "repeated package from one module is a no-op", input: goListStreamFixture(t, depA, depA, main), want: []buildDependencyWire{wantA}},
		{name: "reverse module order preserves every sorted fact", input: goListStreamFixture(t, depB, depA, main), want: []buildDependencyWire{wantA, wantB}},
		{name: "repeated main package retains one root", input: goListStreamFixture(t, main, main)},
		{name: "standard-only stream cannot manufacture a main root", input: goListStreamFixture(t, standard), wantErr: core.ErrReleaseContract},
		{name: "empty package path cannot borrow a valid module identity", input: goListStreamFixture(t, mutateGoListPackage(depA, func(w *goListPackageWire) { w.ImportPath = "" }), main), wantErr: core.ErrReleaseContract},
		{name: "truncated next record discards an already complete prefix", input: append(goListStreamFixture(t, depA, main), '{'), wantErr: core.ErrReleaseContract},
		{name: "malformed next token discards an already complete prefix", input: append(goListStreamFixture(t, depA, main), '!'), wantErr: core.ErrReleaseContract},
		{name: "incomplete package cannot contribute valid-looking module facts", input: goListStreamFixture(t, mutateGoListPackage(depA, func(w *goListPackageWire) { w.Incomplete = true }), main), wantErr: core.ErrReleaseContract},
		{name: "package error cannot contribute valid-looking module facts", input: goListStreamFixture(t, mutateGoListPackage(depA, func(w *goListPackageWire) { w.Error = &goListErrorWire{Err: "package failed"} }), main), wantErr: core.ErrReleaseContract},
		{name: "nonstandard package must name its module", input: goListStreamFixture(t, goListPackageWire{ImportPath: "example.com/unowned"}, main), wantErr: core.ErrReleaseContract},
		{name: "standard flag contradicts a module identity", input: goListStreamFixture(t, mutateGoListPackage(depA, func(w *goListPackageWire) { w.Standard = true }), main), wantErr: core.ErrReleaseContract},
		{name: "replacement cannot stand in for the selected dependency", input: goListStreamFixture(t, mutateGoListPackage(depA, func(w *goListPackageWire) { w.Module.Replace = &goListModuleWire{Path: "example.com/replacement"} }), main), wantErr: core.ErrReleaseContract},
		{name: "missing version cannot become a selected dependency", input: goListStreamFixture(t, mutateGoListPackage(depA, func(w *goListPackageWire) { w.Module.Version = "" }), main), wantErr: core.ErrReleaseContract},
		{name: "missing checksum cannot become authenticated bytes", input: goListStreamFixture(t, mutateGoListPackage(depA, func(w *goListPackageWire) { w.Module.Sum = "" }), main), wantErr: core.ErrReleaseContract},
		{name: "malformed module path cannot become a dependency", input: goListStreamFixture(t, mutateGoListPackage(depA, func(w *goListPackageWire) { w.Module.Path = "-flag/path" }), main), wantErr: core.ErrReleaseContract},
		{name: "different main modules cannot share one closure", input: goListStreamFixture(t, main, goListPackage(goListModuleWire{Path: "example.com/other", Main: true})), wantErr: core.ErrReleaseContract},
		{name: "same module version conflict cannot choose first evidence", input: goListStreamFixture(t, depA, mutateGoListPackage(depA, func(w *goListPackageWire) { w.Module.Version = "v1.2.4" }), main), wantErr: core.ErrReleaseContract},
		{name: "same module checksum conflict cannot choose first evidence", input: goListStreamFixture(t, depA, mutateGoListPackage(depA, func(w *goListPackageWire) { w.Module.Sum = testModuleSumB }), main), wantErr: core.ErrReleaseContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			observed := dependencyObservation{main: mustModulePath(t, "example.com/previous"), modules: numberedModules(t, 1)}
			err := decodeBuildDependencies(bytes.NewReader(tc.input), &observed)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("decodeBuildDependencies() error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if observed.main != (GoModulePath{}) || len(observed.modules) != 0 {
					t.Fatalf("refused stream = %v, want zero root and no modules", observed)
				}
				return
			}
			if observed.main.String() != testMainModule || len(observed.modules) != len(tc.want) {
				t.Fatalf("stream facts = (%v, %d modules), want (%s, %d)", observed.main, len(observed.modules), testMainModule, len(tc.want))
			}
			for index, want := range tc.want {
				got := observed.modules[index]
				if got.Validate() != nil || got.Path().String() != want.Path || got.Version().String() != want.Version || got.Sum().String() != want.Sum {
					t.Fatalf("stream module %d = %v, want exact facts %v", index, got, want)
				}
			}
		})
	}
}

func goListPackage(module goListModuleWire) goListPackageWire {
	return goListPackageWire{ImportPath: module.Path + "/package", Module: &module}
}

func mutateGoListPackage(base goListPackageWire, mutate func(*goListPackageWire)) goListPackageWire {
	if base.Module != nil {
		module := *base.Module
		base.Module = &module
	}
	mutate(&base)
	return base
}

func goListStreamFixture(t testing.TB, packages ...goListPackageWire) []byte {
	t.Helper()
	var stream bytes.Buffer
	for _, pkg := range packages {
		encoded, err := json.Marshal(pkg)
		if err != nil {
			t.Fatalf("Go Marshal(package stream) error = %v, want nil", err)
		}
		stream.Write(encoded)
	}
	return stream.Bytes()
}
