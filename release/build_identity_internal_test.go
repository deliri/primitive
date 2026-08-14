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
	"github.com/deliri/primitive/v2026/testserial"
)

func TestEmbeddedBuildIdentityLinkSymbolsNameOwnedVariables(t *testing.T) {
	t.Parallel()

	want := map[string]string{
		embeddedBuildOfferingVariableName:    EmbeddedBuildOfferingLinkSymbol,
		embeddedBuildVersionVariableName:     EmbeddedBuildVersionLinkSymbol,
		embeddedBuildCommitVariableName:      EmbeddedBuildCommitLinkSymbol,
		embeddedBuildPlatformVariableName:    EmbeddedBuildPlatformLinkSymbol,
		embeddedBuildAssignmentsVariableName: embeddedBuildAssignmentsLinkSymbol,
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
							t.Fatalf("embedded build-identity variable %s has no compiler-owned link symbol", name.Name)
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

func TestEmbeddedBuildIdentityReadsExactLinkerOwnedFacts(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{
		Hazard: core.TestIsolationHazardGlobalRegistry,
		Scope:  core.TestIsolationScopePackageProcess,
	})

	previous := embeddedBuildIdentityText{
		offering:    embeddedBuildOffering,
		version:     embeddedBuildVersion,
		commit:      embeddedBuildCommit,
		platform:    embeddedBuildPlatform,
		assignments: embeddedBuildAssignments,
	}
	t.Cleanup(func() {
		embeddedBuildOffering = previous.offering
		embeddedBuildVersion = previous.version
		embeddedBuildCommit = previous.commit
		embeddedBuildPlatform = previous.platform
		embeddedBuildAssignments = previous.assignments
	})

	commit, err := core.ParseBuildCommit("0123456789abcdef0123456789abcdef01234567")
	if err != nil {
		t.Fatalf("core.ParseBuildCommit() error = %v, want nil", err)
	}
	build, err := core.NewBuildIdentity(core.BuildIdentityRequest{
		Offering: core.OfferingWitness, Version: core.NewReleaseVersion(2026, 8, 2), Commit: commit,
		Platform: core.Platform{OperatingSystem: core.OperatingSystemDarwin, Architecture: core.CPUArchitectureARM64},
	})
	if err != nil {
		t.Fatalf("core.NewBuildIdentity() error = %v, want nil", err)
	}
	commitment, err := emptyLinkerAssignmentsForTest().commitment()
	if err != nil {
		t.Fatalf("LinkerAssignments.commitment() error = %v, want nil", err)
	}
	framed := frameEmbeddedBuildIdentity(build, commitment)
	embeddedBuildOffering = framed.offering
	embeddedBuildVersion = framed.version
	embeddedBuildCommit = framed.commit
	embeddedBuildPlatform = framed.platform
	embeddedBuildAssignments = framed.assignments
	got, gotErr := EmbeddedBuildIdentity()
	if gotErr != nil || got.Validate() != nil || got != build {
		t.Fatalf("EmbeddedBuildIdentity() = (%v, %v), want exact linker-owned build facts", got, gotErr)
	}

	embeddedBuildPlatform = embeddedBuildPlatformFramePrefix + "darwin-386"
	got, gotErr = EmbeddedBuildIdentity()
	if got != (core.BuildIdentity{}) || !errors.Is(gotErr, core.ErrReleaseContract) ||
		!errors.Is(gotErr, core.ErrPrimitiveContract) {
		t.Fatalf("EmbeddedBuildIdentity(invalid platform) = (%v, %v), want zero, errors.Is %v, and errors.Is %v", got, gotErr, core.ErrReleaseContract, core.ErrPrimitiveContract)
	}
}

func TestEmbeddedBuildIdentityTextBoundary(t *testing.T) {
	t.Parallel()

	commit, err := core.ParseBuildCommit("0123456789abcdef0123456789abcdef01234567")
	if err != nil {
		t.Fatalf("core.ParseBuildCommit() error = %v, want nil", err)
	}
	want, err := core.NewBuildIdentity(core.BuildIdentityRequest{
		Offering: core.OfferingWitness, Version: core.NewReleaseVersion(2026, 8, 2), Commit: commit,
		Platform: core.Platform{OperatingSystem: core.OperatingSystemDarwin, Architecture: core.CPUArchitectureARM64},
	})
	if err != nil {
		t.Fatalf("core.NewBuildIdentity() error = %v, want nil", err)
	}
	commitment, err := emptyLinkerAssignmentsForTest().commitment()
	if err != nil {
		t.Fatalf("LinkerAssignments.commitment() error = %v, want nil", err)
	}
	valid := frameEmbeddedBuildIdentity(want, commitment)
	got, gotErr := parseEmbeddedBuildIdentity(valid)
	if gotErr != nil || got != want {
		t.Fatalf("parseEmbeddedBuildIdentity(valid) = (%v, %v), want exact embedded facts", got, gotErr)
	}

	for _, tc := range []struct {
		name  string
		value embeddedBuildIdentityText
	}{
		{name: "offering payload", value: embeddedBuildIdentityText{offering: embeddedBuildOfferingFramePrefix + "future", version: valid.version, commit: valid.commit, platform: valid.platform, assignments: valid.assignments}},
		{name: "version payload", value: embeddedBuildIdentityText{offering: valid.offering, version: embeddedBuildVersionFramePrefix + "2026.08.2", commit: valid.commit, platform: valid.platform, assignments: valid.assignments}},
		{name: "commit payload", value: embeddedBuildIdentityText{offering: valid.offering, version: valid.version, commit: embeddedBuildCommitFramePrefix + "invalid", platform: valid.platform, assignments: valid.assignments}},
		{name: "platform payload", value: embeddedBuildIdentityText{offering: valid.offering, version: valid.version, commit: valid.commit, platform: embeddedBuildPlatformFramePrefix + "darwin-386", assignments: valid.assignments}},
		{name: "assignment payload", value: embeddedBuildIdentityText{offering: valid.offering, version: valid.version, commit: valid.commit, platform: valid.platform, assignments: embeddedBuildAssignmentsFramePrefix + "invalid"}},
		{name: "raw offering lacks frame", value: embeddedBuildIdentityText{offering: "witness", version: valid.version, commit: valid.commit, platform: valid.platform, assignments: valid.assignments}},
		{name: "raw assignments lack frame", value: embeddedBuildIdentityText{offering: valid.offering, version: valid.version, commit: valid.commit, platform: valid.platform, assignments: commitment}},
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

func TestLinkerAssignmentCommitmentBindsCanonicalSet(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		baselineValues [][2]string
		values         [][2]string
		wantSame       bool
	}{
		{name: "identical canonical set", baselineValues: [][2]string{{"github.com/x/y.alpha", "one"}}, values: [][2]string{{"github.com/x/y.alpha", "one"}}, wantSame: true},
		{name: "identical set in reverse input order", baselineValues: [][2]string{{"github.com/x/y.alpha", "one"}, {"github.com/x/y.beta", "two"}}, values: [][2]string{{"github.com/x/y.beta", "two"}, {"github.com/x/y.alpha", "one"}}, wantSame: true},
		{name: "empty set differs", values: nil},
		{name: "symbol differs", values: [][2]string{{"github.com/x/y.beta", "one"}}},
		{name: "value differs", values: [][2]string{{"github.com/x/y.alpha", "two"}}},
		{name: "value gains one byte", values: [][2]string{{"github.com/x/y.alpha", "one1"}}},
		{name: "value loses one byte", values: [][2]string{{"github.com/x/y.alpha", "on"}}},
		{name: "second assignment added", values: [][2]string{{"github.com/x/y.alpha", "one"}, {"github.com/x/y.beta", "two"}}},
		{name: "prefix split cannot alias", values: [][2]string{{"github.com/x/y.alpha", "o"}, {"github.com/x/y.beta", "ne"}}},
		{name: "symbol and value transpose cannot alias", values: [][2]string{{"github.com/x/y.one", "alpha"}}},
		{name: "case-sensitive symbol differs", values: [][2]string{{"github.com/x/y.Alpha", "one"}}},
		{name: "case-sensitive value differs", values: [][2]string{{"github.com/x/y.alpha", "One"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			baselineValues := tc.baselineValues
			if baselineValues == nil {
				baselineValues = [][2]string{{"github.com/x/y.alpha", "one"}}
			}
			baseline := linkerCommitmentFixture(t, baselineValues)
			baselineCommitment, err := baseline.commitment()
			if err != nil {
				t.Fatalf("LinkerAssignments.commitment(baseline) error = %v, want nil", err)
			}
			set := linkerCommitmentFixture(t, tc.values)
			got, gotErr := set.commitment()
			if gotErr != nil {
				t.Fatalf("LinkerAssignments.commitment() error = %v, want nil", gotErr)
			}
			if (got == baselineCommitment) != tc.wantSame {
				t.Fatalf("LinkerAssignments.commitment() equality = %t, want %t", got == baselineCommitment, tc.wantSame)
			}
			var digest core.SHA256Digest
			if gotErr := digest.UnmarshalText([]byte(got)); gotErr != nil {
				t.Fatalf("SHA256Digest.UnmarshalText(commitment) error = %v, want nil", gotErr)
			}
		})
	}
}

func linkerCommitmentFixture(t *testing.T, values [][2]string) LinkerAssignments {
	t.Helper()
	assignments := make([]LinkerAssignment, 0, len(values))
	for _, value := range values {
		assignment, err := NewLinkerAssignment(value[0], value[1])
		if err != nil {
			t.Fatalf("NewLinkerAssignment(%q, %q) error = %v, want nil", value[0], value[1], err)
		}
		assignments = append(assignments, assignment)
	}
	set, err := NewLinkerAssignments(assignments)
	if err != nil {
		t.Fatalf("NewLinkerAssignments() error = %v, want nil", err)
	}
	return set
}
