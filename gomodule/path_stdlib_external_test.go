package gomodule_test

import (
	"errors"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
)

func TestNativePathAdmissionHasNoExternalParserDependency(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"path.go", "import_path.go", "declared_path.go"} {
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imported := range file.Imports {
			path, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(path, ".") && path != "github.com/deliri/primitive/v2026/core" {
				t.Fatalf("native admission %s depends on external parser %q", name, path)
			}
		}
	}
}

func TestNativePathAdmissionKeepsConstantWorkingMemory(t *testing.T) {
	// witness:waiver test/parallel/default -- testing.AllocsPerRun changes process GOMAXPROCS while measuring.
	for _, segments := range []int{1, 32, 1 << 15} {
		t.Run(strconv.Itoa(segments), func(t *testing.T) {
			// witness:waiver test/parallel/default -- owned allocation measurement changes process GOMAXPROCS.
			value := "example.com/" + strings.Repeat("part/", segments) + "v123456789012345678901234567890"
			var observed gomodule.Path
			allocations := testing.AllocsPerRun(10, func() {
				var err error
				observed, err = gomodule.ParsePath(value)
				if err != nil {
					t.Fatal(err)
				}
			})
			if observed.String() != value || allocations != 0 {
				t.Fatalf("admission changed identity or materialized elements: allocations=%v identity=%t", allocations, observed.String() == value)
			}
		})
	}
}

func TestNativePathAdmissionSeparatesDeclaredImportAndModuleDomains(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, value      string
		module, imported bool
	}{
		{name: "first domain resembles version", value: "v1.0", module: true, imported: true},
		{name: "local main module", value: "app", imported: true},
		{name: "import plus punctuation", value: "example.com/c++", imported: true},
		{name: "major version one", value: "example.com/project/v1", imported: true},
		{name: "major version zero", value: "example.com/project/v0", imported: true},
		{name: "major version two", value: "example.com/project/v2", module: true, imported: true},
		{name: "nonnumeric version prefix", value: "example.com/project/v1x", module: true, imported: true},
		{name: "dot in version", value: "example.com/project/v1.2", imported: true},
		{name: "gopkg stable zero", value: "gopkg.in/project.v0", module: true, imported: true},
		{name: "gopkg unstable zero", value: "gopkg.in/project.v0-unstable", imported: true},
		{name: "gopkg unstable one", value: "gopkg.in/project.v1-unstable", module: true, imported: true},
		{name: "gopkg missing version", value: "gopkg.in/project", imported: true},
		{name: "reserved stem before extension", value: "example.com/COM1.txt"},
		{name: "short name before extension", value: "example.com/part~1.txt"},
		{name: "long name before extension", value: "example.com/part~x.txt", module: true, imported: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			module, moduleErr := gomodule.ParsePath(tc.value)
			imported, importErr := gomodule.ParseImportPath(tc.value)
			if (moduleErr == nil) != tc.module || (importErr == nil) != tc.imported {
				t.Fatalf("admission %q = module:%v import:%v, want %t/%t", tc.value, moduleErr, importErr, tc.module, tc.imported)
			}
			if !tc.module && (!errors.Is(moduleErr, core.ErrGoModuleContract) || module != (gomodule.Path{})) {
				t.Fatalf("module refusal leaked identity or lost constant: %v/%v", module, moduleErr)
			}
			if !tc.imported && (!errors.Is(importErr, core.ErrGoModuleContract) || imported != (gomodule.ImportPath{})) {
				t.Fatalf("import refusal leaked identity or lost constant: %v/%v", imported, importErr)
			}
		})
	}
}
