package core

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

type documentationViolationKind uint8

const (
	documentationViolationUnknown documentationViolationKind = iota
	documentationViolationFunction
	documentationViolationType
	documentationViolationConstant
	documentationViolationField
)

type documentationViolation struct {
	symbol string
	kind   documentationViolationKind
}

const documentationViolationMaximum = 256

type documentationViolationInventory struct {
	values [documentationViolationMaximum]documentationViolation
	count  uint16
}

func TestEveryExportedCoreDeclarationHasPublicDocumentation(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("os.ReadDir(core) error = %v, want nil", err)
	}
	files := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(files, entry.Name(), nil, parser.ParseComments|parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("parser.ParseFile(%q) error = %v, want nil", entry.Name(), parseErr)
		}
		gotViolations, gotViolationErr := collectDocumentationViolations(file)
		if gotViolationErr != nil {
			t.Fatalf("collectDocumentationViolations(%q) error = %v, want nil", entry.Name(), gotViolationErr)
		}
		for _, gotViolation := range gotViolations.Values() {
			t.Errorf(
				"%s exported %v %s has no doc comment",
				entry.Name(),
				gotViolation.kind,
				gotViolation.symbol,
			)
		}
	}
}

func TestDocumentationMatcherSyntheticRedGreenRatchet(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		source        string
		wantViolation documentationViolation
		wantCount     int
	}{
		{
			name:   "documented exported function is accepted",
			source: "package fixture\n// Exported documents the function.\nfunc Exported() {}\n",
		},
		{
			name:          "undocumented exported function is reported",
			source:        "package fixture\nfunc Exported() {}\n",
			wantViolation: documentationViolation{kind: documentationViolationFunction, symbol: "Exported"},
			wantCount:     1,
		},
		{
			name:   "documented exported method is accepted",
			source: "package fixture\ntype local struct{}\n// Exported documents the method.\nfunc (local) Exported() {}\n",
		},
		{
			name:          "undocumented exported method is reported",
			source:        "package fixture\ntype local struct{}\nfunc (local) Exported() {}\n",
			wantViolation: documentationViolation{kind: documentationViolationFunction, symbol: "Exported"},
			wantCount:     1,
		},
		{
			name:   "documented exported type is accepted",
			source: "package fixture\n// Exported documents the type.\ntype Exported uint8\n",
		},
		{
			name:          "undocumented exported type is reported",
			source:        "package fixture\ntype Exported uint8\n",
			wantViolation: documentationViolation{kind: documentationViolationType, symbol: "Exported"},
			wantCount:     1,
		},
		{
			name:   "documented exported constant is accepted",
			source: "package fixture\nconst (\n// Exported documents the constant.\nExported = 1\n)\n",
		},
		{
			name:          "undocumented exported constant is reported",
			source:        "package fixture\nconst Exported = 1\n",
			wantViolation: documentationViolation{kind: documentationViolationConstant, symbol: "Exported"},
			wantCount:     1,
		},
		{
			name:   "declaration comment documents grouped constants",
			source: "package fixture\n// Exported constants document the group.\nconst (\nExportedOne = 1\nExportedTwo = 2\n)\n",
		},
		{
			name:   "private declarations require no public documentation",
			source: "package fixture\nconst private = 1\ntype local struct{ field uint8 }\nfunc hidden() {}\n",
		},
		{
			name:   "documented exported struct field is accepted",
			source: "package fixture\n// Exported documents the type.\ntype Exported struct {\n// Field documents the field.\nField uint8\n}\n",
		},
		{
			name:          "undocumented exported struct field is reported",
			source:        "package fixture\n// Exported documents the type.\ntype Exported struct { Field uint8 }\n",
			wantViolation: documentationViolation{kind: documentationViolationField, symbol: "Exported.Field"},
			wantCount:     1,
		},
		{
			name:   "embedded private field requires no public documentation",
			source: "package fixture\ntype local struct{}\n// Exported documents the type.\ntype Exported struct { local }\n",
		},
		{
			name:   "multiline documentation is accepted",
			source: "package fixture\n/* Exported documents the function. */\nfunc Exported() {}\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			file, gotParseErr := parser.ParseFile(
				token.NewFileSet(),
				"synthetic_documentation.go",
				tc.source,
				parser.ParseComments|parser.SkipObjectResolution,
			)
			if gotParseErr != nil {
				t.Fatalf("parser.ParseFile(synthetic source) error = %v, want nil", gotParseErr)
			}
			gotViolations, gotViolationErr := collectDocumentationViolations(file)
			if gotViolationErr != nil {
				t.Fatalf("collectDocumentationViolations(synthetic source) error = %v, want nil", gotViolationErr)
			}
			if gotViolations.Count() != tc.wantCount {
				t.Fatalf(
					"documentation violation count = %d, want %d; violations = %v",
					gotViolations.Count(),
					tc.wantCount,
					gotViolations.Values(),
				)
			}
			if tc.wantCount == 0 {
				return
			}
			gotViolation := gotViolations.Values()[0]
			if gotViolation != tc.wantViolation {
				t.Fatalf("documentation violation = %v, want %v", gotViolation, tc.wantViolation)
			}
		})
	}
}

func collectDocumentationViolations(file *ast.File) (documentationViolationInventory, error) {
	var violations documentationViolationInventory
	for _, declaration := range file.Decls {
		switch typed := declaration.(type) {
		case *ast.FuncDecl:
			if ast.IsExported(typed.Name.Name) && typed.Doc == nil {
				if err := violations.Add(documentationViolation{
					kind:   documentationViolationFunction,
					symbol: typed.Name.Name,
				}); err != nil {
					return documentationViolationInventory{}, err
				}
			}
		case *ast.GenDecl:
			if err := collectSpecificationDocumentationViolations(&violations, typed); err != nil {
				return documentationViolationInventory{}, err
			}
		}
	}
	return violations, nil
}

func collectSpecificationDocumentationViolations(
	violations *documentationViolationInventory,
	declaration *ast.GenDecl,
) error {
	for _, specification := range declaration.Specs {
		switch typed := specification.(type) {
		case *ast.TypeSpec:
			if ast.IsExported(typed.Name.Name) && typed.Doc == nil && declaration.Doc == nil {
				if err := violations.Add(documentationViolation{
					kind:   documentationViolationType,
					symbol: typed.Name.Name,
				}); err != nil {
					return err
				}
			}
			if err := collectFieldDocumentationViolations(violations, typed); err != nil {
				return err
			}
		case *ast.ValueSpec:
			for _, name := range typed.Names {
				if ast.IsExported(name.Name) && typed.Doc == nil && declaration.Doc == nil {
					if err := violations.Add(documentationViolation{
						kind:   documentationViolationConstant,
						symbol: name.Name,
					}); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func collectFieldDocumentationViolations(
	violations *documentationViolationInventory,
	specification *ast.TypeSpec,
) error {
	structure, ok := specification.Type.(*ast.StructType)
	if !ok {
		return nil
	}
	for _, field := range structure.Fields.List {
		for _, name := range field.Names {
			if ast.IsExported(name.Name) && field.Doc == nil {
				if err := violations.Add(documentationViolation{
					kind:   documentationViolationField,
					symbol: specification.Name.Name + "." + name.Name,
				}); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (i *documentationViolationInventory) Add(violation documentationViolation) error {
	if violation.kind == documentationViolationUnknown || violation.symbol == "" {
		return architectureContractError("documentation violation is incomplete")
	}
	if int(i.count) >= len(i.values) {
		return architectureContractError("documentation violation inventory exceeds its fixed capacity")
	}
	i.values[i.count] = violation
	i.count++
	return nil
}

func (i documentationViolationInventory) Count() int {
	return int(i.count)
}

func (i documentationViolationInventory) Values() []documentationViolation {
	return i.values[:i.count]
}
