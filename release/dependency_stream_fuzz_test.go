package release

import (
	"bytes"
	"encoding/base64"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzGoPackageStreamExactFacts(f *testing.F) {
	main := goListPackage(goListModuleWire{Path: testMainModule, Main: true})
	a := goListPackage(goListModuleWire{Path: "example.com/a", Version: "v1.2.3", Sum: testModuleSumA})
	b := goListPackage(goListModuleWire{Path: "example.com/b", Version: "v2.3.4", Sum: testModuleSumB})
	for _, packages := range [][]goListPackageWire{
		{main}, {main, a}, {b, a, main, a},
		{{ImportPath: "io", Standard: true}, main},
		{main, a, mutateGoListPackage(a, func(w *goListPackageWire) { w.Module.Sum = testModuleSumB })},
		{main, mutateGoListPackage(a, func(w *goListPackageWire) { w.Module.Error = &goListErrorWire{} })},
		{mutateGoListPackage(main, func(w *goListPackageWire) { w.Module.Version = a.Module.Version })},
	} {
		f.Add(goListStreamFixture(f, packages...))
	}
	f.Add(append(goListStreamFixture(f, main, a), '{'))
	f.Add([]byte{})
	previous := mustModulePath(f, "example.com/previous")
	priorModules := numberedModules(f, 1)
	f.Fuzz(func(t *testing.T, data []byte) {
		// The process boundary already imposes this byte ceiling. Bound the
		// independent oracle to the same admitted external input size.
		if len(data) > buildDependencyObservationMaximumBytes {
			return
		}
		got := dependencyObservation{main: previous, modules: slices.Clone(priorModules)}
		err := decodeBuildDependencies(bytes.NewReader(data), &got)
		if err != nil {
			if !errors.Is(err, core.ErrReleaseContract) || got.main != (GoModulePath{}) || len(got.modules) != 0 {
				t.Fatalf("refused Go stream = (%v, %v), want typed refusal and zero facts", got, err)
			}
			return
		}
		// Decode actual input fields with Go, then compare the full raw facts.
		// This oracle never calls production's add/merge or module parsers.
		decoder := jsontext.NewDecoder(bytes.NewReader(data))
		var root string
		var want []buildDependencyWire
		for count := 0; ; count++ {
			var pkg goListPackageWire
			err := json.UnmarshalDecode(decoder, &pkg)
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil || count >= buildPackageObservationMaximumCount || pkg.ImportPath == "" || pkg.Incomplete || pkg.Error != nil {
				t.Fatalf("admitted package %d = (%v, %v), want a complete bounded Go observation", count, pkg, err)
			}
			if pkg.Module == nil {
				if !pkg.Standard {
					t.Fatalf("admitted package %d = %v, want standard standing without module facts", count, pkg)
				}
				continue
			}
			module := *pkg.Module
			if pkg.Standard || module.Error != nil || module.Replace != nil {
				t.Fatalf("admitted module %d = %v, want unsubstituted and error-free module facts", count, module)
			}
			if module.Main {
				if module.Path == "" || module.Version != "" || module.Sum != "" || (root != "" && root != module.Path) {
					t.Fatalf("admitted main = %v, want one root %q without dependency facts", module, root)
				}
				root = module.Path
				continue
			}
			encoded, ok := strings.CutPrefix(module.Sum, goModuleSumPrefix)
			digest, err := base64.StdEncoding.DecodeString(encoded)
			if !ok || err != nil || len(digest) != core.SHA256DigestBytes || base64.StdEncoding.EncodeToString(digest) != encoded {
				t.Fatalf("admitted checksum = (%q, %v), want exact canonical Go SHA-256 encoding", module.Sum, err)
			}
			fact := buildDependencyWire{Path: module.Path, Version: module.Version, Sum: module.Sum}
			index := slices.IndexFunc(want, func(existing buildDependencyWire) bool { return existing.Path == fact.Path })
			if index >= 0 {
				if want[index] != fact {
					t.Fatalf("admitted repeated module = %v, want unchanged prior fact %v", fact, want[index])
				}
				continue
			}
			if len(want) >= BuildDependencyMaximumCount {
				t.Fatalf("admitted distinct module count exceeds %d", BuildDependencyMaximumCount)
			}
			want = append(want, fact)
		}
		slices.SortFunc(want, func(a, b buildDependencyWire) int { return strings.Compare(a.Path, b.Path) })
		if root == "" || got.main.String() != root || len(got.modules) != len(want) {
			t.Fatalf("admitted stream = %v, want root %q and %d exact modules", got, root, len(want))
		}
		for index, fact := range want {
			module := got.modules[index]
			if module.Validate() != nil || module.Path().String() != fact.Path || module.Version().String() != fact.Version || module.Sum().String() != fact.Sum {
				t.Fatalf("admitted module %d = %v, want exact input facts %v", index, module, fact)
			}
		}
	})
}
