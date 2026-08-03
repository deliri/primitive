package release

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestEmbeddedBuildIdentityLinkSymbolsNameOwnedVariables(t *testing.T) {
	t.Parallel()

	want := map[string]string{
		embeddedBuildOfferingVariableName: EmbeddedBuildOfferingLinkSymbol,
		embeddedBuildVersionVariableName:  EmbeddedBuildVersionLinkSymbol,
		embeddedBuildCommitVariableName:   EmbeddedBuildCommitLinkSymbol,
		embeddedBuildPlatformVariableName: EmbeddedBuildPlatformLinkSymbol,
	}
	packagePath := reflect.TypeFor[embeddedBuildIdentityText]().PkgPath()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("os.ReadDir(release package) error = %v, want nil", err)
	}
	for _, entry := range entries {
		filename := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(filename, ".go") || strings.HasSuffix(filename, "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), filename, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("parser.ParseFile(%q) error = %v, want nil", filename, parseErr)
		}
		for _, declaration := range file.Decls {
			generic, ok := declaration.(*ast.GenDecl)
			if !ok || generic.Tok != token.VAR {
				continue
			}
			for _, raw := range generic.Specs {
				spec, ok := raw.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, name := range spec.Names {
					symbol, admitted := want[name.Name]
					if !admitted {
						if strings.HasPrefix(name.Name, embeddedBuildIdentityVariablePrefix) {
							t.Fatalf("embedded build-identity variable %s has no exported link symbol", name.Name)
						}
						continue
					}
					if symbol != packagePath+"."+name.Name {
						t.Fatalf("link symbol for %s = %q, want compiler-owned package path and variable", name.Name, symbol)
					}
					delete(want, name.Name)
				}
			}
		}
	}
	if len(want) != 0 {
		missing := make([]string, 0, len(want))
		for name := range want {
			missing = append(missing, name)
		}
		sort.Strings(missing)
		t.Fatalf("link symbols name missing variables = %v", missing)
	}
}

func TestEmbeddedBuildIdentityTextBoundary(t *testing.T) {
	t.Parallel()

	valid := embeddedBuildIdentityText{
		offering: "witness",
		version:  "2026.8.2",
		commit:   "0123456789abcdef0123456789abcdef01234567",
		platform: "darwin-arm64",
	}
	got, gotErr := parseEmbeddedBuildIdentity(valid)
	if gotErr != nil || got.Offering() != core.OfferingWitness ||
		got.Version() != core.NewReleaseVersion(2026, 8, 2) ||
		got.Platform() != (core.Platform{
			OperatingSystem: core.OperatingSystemDarwin,
			Architecture:    core.CPUArchitectureARM64,
		}) {
		t.Fatalf("parseEmbeddedBuildIdentity(valid) = (%v, %v), want exact embedded facts", got, gotErr)
	}

	for _, tc := range []struct {
		name  string
		value embeddedBuildIdentityText
	}{
		{name: "offering", value: embeddedBuildIdentityText{offering: "future", version: valid.version, commit: valid.commit, platform: valid.platform}},
		{name: "version", value: embeddedBuildIdentityText{offering: valid.offering, version: "2026.08.2", commit: valid.commit, platform: valid.platform}},
		{name: "commit", value: embeddedBuildIdentityText{offering: valid.offering, version: valid.version, commit: "invalid", platform: valid.platform}},
		{name: "platform", value: embeddedBuildIdentityText{offering: valid.offering, version: valid.version, commit: valid.commit, platform: "darwin-386"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := parseEmbeddedBuildIdentity(tc.value)
			if got != (core.BuildIdentity{}) || !errors.Is(gotErr, core.ErrReleaseContract) ||
				!errors.Is(gotErr, core.ErrPrimitiveContract) {
				t.Fatalf("parseEmbeddedBuildIdentity(%s invalid) = (%v, %v), want zero, %v, and %v", tc.name, got, gotErr, core.ErrReleaseContract, core.ErrPrimitiveContract)
			}
		})
	}
}
