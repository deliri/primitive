package projectstandards

import (
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestSourceImportKindExhaustsEveryUint8Value(t *testing.T) {
	t.Parallel()

	for raw := 0; raw <= math.MaxUint8; raw++ {
		kind := SourceImportKind(raw)
		wantValid := kind >= SourceImportKindStandardLibrary && kind <= SourceImportKindExternal
		if kind.IsValid() != wantValid {
			t.Errorf("SourceImportKind(%d).IsValid() = %t, want %t", raw, kind.IsValid(), wantValid)
		}
	}
}

func TestSourceFileImportsHostileBoundaries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		setup   func(*testing.T) SourceFileImports
		wantErr error
		name    string
	}{
		{name: "observed file with zero imports remains distinct from not observed", setup: func(*testing.T) SourceFileImports { return SourceFileImports{} }},
		{name: "standard library import is admitted without a project module", setup: func(t *testing.T) SourceFileImports {
			return SourceFileImports{Values: []SourceImport{sourceImportFixture(t, SourceImportKindStandardLibrary, "encoding/json/v2", "")}}
		}},
		{name: "external module import is admitted without a project module", setup: func(t *testing.T) SourceFileImports {
			return SourceFileImports{Values: []SourceImport{sourceImportFixture(t, SourceImportKindExternal, "cloud.google.com/go/storage", "")}}
		}},
		{name: "project import carries its exact module identity", setup: func(t *testing.T) SourceFileImports {
			return SourceFileImports{Values: []SourceImport{sourceImportFixture(t, SourceImportKindProject, "github.com/deliri/primitive/v2026/core", "github.com/deliri/primitive/v2026")}}
		}},
		{name: "project module root import is admitted", setup: func(t *testing.T) SourceFileImports {
			return SourceFileImports{Values: []SourceImport{sourceImportFixture(t, SourceImportKindProject, "example.com/project", "example.com/project")}}
		}},
		{name: "mixed imports in canonical path order are admitted", setup: func(t *testing.T) SourceFileImports {
			return SourceFileImports{Values: []SourceImport{
				sourceImportFixture(t, SourceImportKindExternal, "cloud.google.com/go/storage", ""),
				sourceImportFixture(t, SourceImportKindStandardLibrary, "encoding/json/v2", ""),
				sourceImportFixture(t, SourceImportKindProject, "github.com/deliri/primitive/v2026/core", "github.com/deliri/primitive/v2026"),
			}}
		}},
		{name: "exact import-count ceiling is admitted", setup: func(t *testing.T) SourceFileImports {
			return sourceImportCatalogOfSize(t, SourceFileImportMaximum)
		}},
		{name: "unknown import kind is rejected", setup: func(t *testing.T) SourceFileImports {
			return SourceFileImports{Values: []SourceImport{{Path: fixturePath(t, "encoding/json"), Kind: SourceImportKindUnknown}}}
		}, wantErr: core.ErrProjectStandardsContract},
		{name: "future import kind is rejected", setup: func(t *testing.T) SourceFileImports {
			return SourceFileImports{Values: []SourceImport{{Path: fixturePath(t, "encoding/json"), Kind: SourceImportKind(math.MaxUint8)}}}
		}, wantErr: core.ErrProjectStandardsContract},
		{name: "empty import path is rejected", setup: func(*testing.T) SourceFileImports {
			return SourceFileImports{Values: []SourceImport{{Kind: SourceImportKindStandardLibrary}}}
		}, wantErr: core.ErrProjectStandardsContract},
		{name: "project import without a resolved module is rejected", setup: func(t *testing.T) SourceFileImports {
			return SourceFileImports{Values: []SourceImport{{Path: fixturePath(t, "github.com/deliri/primitive/v2026/core"), Kind: SourceImportKindProject}}}
		}, wantErr: core.ErrProjectStandardsConflict},
		{name: "project import with an invalid module is rejected", setup: func(t *testing.T) SourceFileImports {
			invalid := SourcePath{}
			return SourceFileImports{Values: []SourceImport{{Path: fixturePath(t, "github.com/deliri/primitive/v2026/core"), ProjectModule: &invalid, Kind: SourceImportKindProject}}}
		}, wantErr: core.ErrProjectStandardsContract},
		{name: "project import outside its claimed module is rejected", setup: func(t *testing.T) SourceFileImports {
			return SourceFileImports{Values: []SourceImport{sourceImportFixture(t, SourceImportKindProject, "github.com/deliri/primitive/v2026/core", "github.com/deliri/forge")}}
		}, wantErr: core.ErrProjectStandardsConflict},
		{name: "standard library import cannot carry a project module", setup: func(t *testing.T) SourceFileImports {
			return SourceFileImports{Values: []SourceImport{sourceImportFixture(t, SourceImportKindStandardLibrary, "encoding/json", "github.com/deliri/primitive/v2026")}}
		}, wantErr: core.ErrProjectStandardsConflict},
		{name: "external import cannot carry a project module", setup: func(t *testing.T) SourceFileImports {
			return SourceFileImports{Values: []SourceImport{sourceImportFixture(t, SourceImportKindExternal, "cloud.google.com/go/storage", "github.com/deliri/primitive/v2026")}}
		}, wantErr: core.ErrProjectStandardsConflict},
		{name: "duplicate import paths are rejected", setup: func(t *testing.T) SourceFileImports {
			entry := sourceImportFixture(t, SourceImportKindStandardLibrary, "encoding/json", "")
			return SourceFileImports{Values: []SourceImport{entry, entry}}
		}, wantErr: core.ErrProjectStandardsConflict},
		{name: "out-of-order import paths are rejected", setup: func(t *testing.T) SourceFileImports {
			return SourceFileImports{Values: []SourceImport{
				sourceImportFixture(t, SourceImportKindStandardLibrary, "strings", ""),
				sourceImportFixture(t, SourceImportKindStandardLibrary, "errors", ""),
			}}
		}, wantErr: core.ErrProjectStandardsConflict},
		{name: "one import above the count ceiling is rejected", setup: func(t *testing.T) SourceFileImports {
			return sourceImportCatalogOfSize(t, SourceFileImportMaximum+1)
		}, wantErr: core.ErrProjectStandardsContract},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.setup(t).Validate()
			if tc.wantErr == nil && gotErr != nil {
				t.Fatalf("SourceFileImports.Validate() error = %v, want nil", gotErr)
			}
			if tc.wantErr != nil && !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("SourceFileImports.Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

func TestSourceFileDistinguishesUnobservedImportsFromObservedZero(t *testing.T) {
	t.Parallel()

	unobserved := sourceFileFixture(t)
	if unobserved.Imports != nil {
		t.Fatalf("SourceFile.Imports = %v, want nil not-observed state", unobserved.Imports)
	}
	if err := unobserved.Validate(); err != nil {
		t.Fatalf("SourceFile.Validate(unobserved imports) error = %v, want nil", err)
	}

	observed := sourceFileFixture(t)
	observed.Imports = &SourceFileImports{}
	if err := observed.Validate(); err != nil {
		t.Fatalf("SourceFile.Validate(observed zero imports) error = %v, want nil", err)
	}
}

func sourceImportFixture(t *testing.T, kind SourceImportKind, importPath, projectModule string) SourceImport {
	t.Helper()

	entry := SourceImport{Path: fixturePath(t, importPath), Kind: kind}
	if projectModule != "" {
		resolved := fixturePath(t, projectModule)
		entry.ProjectModule = &resolved
	}
	return entry
}

func sourceImportCatalogOfSize(t *testing.T, count int) SourceFileImports {
	t.Helper()

	values := make([]SourceImport, count)
	for index := range values {
		values[index] = sourceImportFixture(t, SourceImportKindExternal, fmt.Sprintf("example.com/dependency/%04d", index), "")
	}
	return SourceFileImports{Values: values}
}
