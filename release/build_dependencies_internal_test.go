package release

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

const (
	testModuleSumA = "h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	testModuleSumB = "h1:AQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	testMainModule = "example.com/product"
)

// TestDependencyDocumentBoundIsDerivedFromItsDocumentCeiling names the invariant
// the compiler proves: the largest admissible closure must still fit the
// document ceiling. The constant exists so raising the module ceiling past its
// document ceiling fails to compile instead of failing at a customer's verifier.
func TestDependencyDocumentBoundIsDerivedFromItsDocumentCeiling(t *testing.T) {
	t.Parallel()

	if dependencyDocumentHeadroom == 0 {
		t.Fatal("dependency document headroom = 0, want the module ceiling to leave room under the document ceiling")
	}
	worst := dependencyDocumentHeaderBytes + BuildDependencyMaximumCount*
		(mainPackageMaximumBytes+goModuleVersionMaximumBytes+
			goModuleSumMaximumBytes+dependencyEntryPunctuationBytes)
	if worst > dependencyDocumentExtentMaximum {
		t.Fatalf("worst-case dependency document = %d bytes, want at most %d",
			worst, dependencyDocumentExtentMaximum)
	}
}

// TestParseGoModulePathPressuresTheModulePathGrammar attacks the path grammar
// that reaches both argv and the signed dependency document.
func TestParseGoModulePathPressuresTheModulePathGrammar(t *testing.T) {
	t.Parallel()

	longest := "example.com/" + strings.Repeat("a", mainPackageMaximumBytes-len("example.com/"))
	cases := []struct {
		name    string
		in      string
		wantErr error
	}{
		{name: "canonical two element path is accepted", in: "example.com/product"},
		{name: "major version suffix is accepted", in: "example.com/product/v2"},
		{name: "deep path is accepted", in: "github.com/deliri/primitive/v2026/release"},
		{name: "hyphenated element is accepted", in: "example.com/my-module"},
		{name: "underscored element is accepted", in: "example.com/my_module"},
		{name: "tilde escaped element is accepted", in: "example.com/My~1module"},
		{name: "digit initial element is accepted", in: "example.com/2fa"},
		{name: "uppercase element is accepted", in: "Example.com/Product"},
		{name: "inner dot is accepted", in: "example.com/go.mod.helper"},
		{name: "exact maximum length is accepted", in: longest},
		{name: "one below maximum length is accepted", in: longest[:len(longest)-1]},

		{name: "empty path is rejected", wantErr: core.ErrReleaseContract},
		{name: "one above maximum length is rejected", in: longest + "a", wantErr: core.ErrReleaseContract},
		{name: "single element without a slash is rejected", in: "product", wantErr: core.ErrReleaseContract},
		{name: "absolute path is rejected", in: "/example.com/product", wantErr: core.ErrReleaseContract},
		{name: "trailing slash is rejected", in: "example.com/product/", wantErr: core.ErrReleaseContract},
		{name: "empty interior element is rejected", in: "example.com//product", wantErr: core.ErrReleaseContract},
		{name: "dot element is rejected", in: "example.com/./product", wantErr: core.ErrReleaseContract},
		{name: "parent element is rejected", in: "example.com/../product", wantErr: core.ErrReleaseContract},
		{name: "flag prefix element is rejected", in: "example.com/-flag", wantErr: core.ErrReleaseContract},
		{name: "leading flag prefix is rejected", in: "-example.com/product", wantErr: core.ErrReleaseContract},
		{name: "element ending in a dot is rejected", in: "example.com/product.", wantErr: core.ErrReleaseContract},
		{name: "space is rejected", in: "example.com/my product", wantErr: core.ErrReleaseContract},
		{name: "invalid UTF8 is rejected", in: "example.com/pro\xffduct", wantErr: core.ErrReleaseContract},
		{name: "NUL byte is rejected", in: "example.com/pro\x00duct", wantErr: core.ErrReleaseContract},
		{name: "quote is rejected", in: `example.com/pro"duct`, wantErr: core.ErrReleaseContract},
		{name: "backslash is rejected", in: `example.com\product`, wantErr: core.ErrReleaseContract},
		{name: "angle bracket is rejected", in: "example.com/<product>", wantErr: core.ErrReleaseContract},
		{name: "ampersand is rejected", in: "example.com/a&b", wantErr: core.ErrReleaseContract},
		{name: "colon is rejected", in: "example.com:443/product", wantErr: core.ErrReleaseContract},
		{name: "at sign version is rejected", in: "example.com/product@v1", wantErr: core.ErrReleaseContract},
		{name: "non-ASCII letter is rejected", in: "example.com/prodüct", wantErr: core.ErrReleaseContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseGoModulePath(tc.in)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("parseGoModulePath(%q) error = %v, want errors.Is(..., %v)", tc.in, err, core.ErrReleaseContract)
				}
				if got != (GoModulePath{}) {
					t.Fatalf("parseGoModulePath(%q) = %q, want zero path on rejection", tc.in, got.value)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseGoModulePath(%q) error = %v, want nil", tc.in, err)
			}
			if got.String() != tc.in {
				t.Fatalf("parseGoModulePath(%q).String() = %q, want %q", tc.in, got.String(), tc.in)
			}
		})
	}
}

// TestParseGoModuleVersionPressuresTheVersionGrammar attacks the version
// grammar. Its character set is load-bearing twice over: it keeps the signed
// dependency document's byte ceiling provable by admitting nothing the JSON
// encoder escapes, and it keeps the value safe as argv.
func TestParseGoModuleVersionPressuresTheVersionGrammar(t *testing.T) {
	t.Parallel()

	longest := "v" + strings.Repeat("1", goModuleVersionMaximumBytes-1)
	cases := []struct {
		name    string
		in      string
		wantErr error
	}{
		{name: "release version is accepted", in: "v1.2.3"},
		{name: "major version zero is accepted", in: "v0.0.0"},
		{name: "pseudo version is accepted", in: "v0.0.0-20260804010203-0123456789ab"},
		{name: "prerelease version is accepted", in: "v1.2.3-alpha.1"},
		{name: "build metadata version is accepted", in: "v1.2.3+incompatible"},
		{name: "prerelease and metadata version is accepted", in: "v1.2.3-rc.1+build.5"},
		{name: "large major version is accepted", in: "v2026.0.11"},
		{name: "shortest admissible version is accepted", in: "v1"},
		{name: "uppercase metadata is accepted", in: "v1.2.3-RC1"},
		{name: "exact maximum length is accepted", in: longest},
		{name: "one below maximum length is accepted", in: longest[:len(longest)-1]},

		{name: "empty version is rejected", wantErr: core.ErrReleaseContract},
		{name: "one below the shortest admissible version is rejected", in: "v", wantErr: core.ErrReleaseContract},
		{name: "one above maximum length is rejected", in: longest + "1", wantErr: core.ErrReleaseContract},
		{name: "missing v prefix is rejected", in: "1.2.3", wantErr: core.ErrReleaseContract},
		{name: "uppercase V prefix is rejected", in: "V1.2.3", wantErr: core.ErrReleaseContract},
		{name: "leading space is rejected", in: " v1.2.3", wantErr: core.ErrReleaseContract},
		{name: "trailing space is rejected", in: "v1.2.3 ", wantErr: core.ErrReleaseContract},
		{name: "assignment character is rejected", in: "v1.2.3=4", wantErr: core.ErrReleaseContract},
		{name: "quote is rejected because it would escape in the sealed document", in: `v1.2.3"`, wantErr: core.ErrReleaseContract},
		{name: "backslash is rejected because it would escape in the sealed document", in: `v1.2.3\`, wantErr: core.ErrReleaseContract},
		{name: "angle bracket is rejected because it would escape in the sealed document", in: "v1.2.3<", wantErr: core.ErrReleaseContract},
		{name: "ampersand is rejected because it would escape in the sealed document", in: "v1.2.3&", wantErr: core.ErrReleaseContract},
		{name: "newline is rejected", in: "v1.2.3\n", wantErr: core.ErrReleaseContract},
		{name: "tab is rejected", in: "v1.2.3\t", wantErr: core.ErrReleaseContract},
		{name: "NUL byte is rejected", in: "v1.2.3\x00", wantErr: core.ErrReleaseContract},
		{name: "invalid UTF8 is rejected", in: "v1.2.3\xff", wantErr: core.ErrReleaseContract},
		{name: "non-ASCII digit is rejected", in: "v1.2.٣", wantErr: core.ErrReleaseContract},
		{name: "slash is rejected", in: "v1.2.3/4", wantErr: core.ErrReleaseContract},
		{name: "at sign is rejected", in: "v1.2.3@sha", wantErr: core.ErrReleaseContract},
		{name: "comma is rejected", in: "v1.2,3", wantErr: core.ErrReleaseContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseGoModuleVersion(tc.in)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("parseGoModuleVersion(%q) error = %v, want errors.Is(..., %v)", tc.in, err, core.ErrReleaseContract)
				}
				if got != (GoModuleVersion{}) {
					t.Fatalf("parseGoModuleVersion(%q) = %q, want zero version on rejection", tc.in, got.value)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseGoModuleVersion(%q) error = %v, want nil", tc.in, err)
			}
			if got.String() != tc.in {
				t.Fatalf("parseGoModuleVersion(%q).String() = %q, want %q", tc.in, got.String(), tc.in)
			}
			encoded, err := json.Marshal(tc.in)
			if err != nil {
				t.Fatalf("json.Marshal(%q) error = %v, want nil", tc.in, err)
			}
			// The document ceiling arithmetic assumes an admitted version costs
			// exactly its own bytes plus two quotes. An admitted version that
			// escapes would silently break that proof.
			if len(encoded) != len(tc.in)+2 {
				t.Fatalf("json.Marshal(%q) = %s (%d bytes), want an unescaped %d byte projection",
					tc.in, encoded, len(encoded), len(tc.in)+2)
			}
		})
	}
}

// TestParseGoModuleSumPressuresTheChecksumGrammar attacks the one field that
// makes a dependency fact pinned rather than merely named.
func TestParseGoModuleSumPressuresTheChecksumGrammar(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		in      string
		wantErr error
	}{
		{name: "zero digest checksum is accepted", in: testModuleSumA},
		{name: "distinct digest checksum is accepted", in: testModuleSumB},
		{name: "real cmd/go checksum is accepted", in: "h1:o7XGOvZQCADBQQ4Y7VNq2dRWQR7JmOUW8Kxx4ZsNgWs="},
		{name: "all bits set checksum is accepted", in: "h1://////////////////////////////////////////8="},

		{name: "empty checksum is rejected", wantErr: core.ErrReleaseContract},
		{name: "absent checksum prefix is rejected", in: strings.TrimPrefix(testModuleSumA, "h1:"), wantErr: core.ErrReleaseContract},
		{name: "unknown checksum generation is rejected", in: "h2:" + strings.TrimPrefix(testModuleSumA, "h1:"), wantErr: core.ErrReleaseContract},
		{name: "uppercase prefix is rejected", in: "H1:" + strings.TrimPrefix(testModuleSumA, "h1:"), wantErr: core.ErrReleaseContract},
		{name: "prefix only is rejected", in: "h1:", wantErr: core.ErrReleaseContract},
		{name: "one base64 character short is rejected", in: "h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", wantErr: core.ErrReleaseContract},
		{name: "one base64 character long is rejected", in: "h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", wantErr: core.ErrReleaseContract},
		{name: "thirty-one byte digest is rejected", in: "h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA==", wantErr: core.ErrReleaseContract},
		{name: "unpadded encoding is rejected", in: "h1:" + strings.TrimSuffix(strings.TrimPrefix(testModuleSumA, "h1:"), "="), wantErr: core.ErrReleaseContract},
		{name: "url safe alphabet is rejected", in: "h1:----------------------------------------__8=", wantErr: core.ErrReleaseContract},
		{name: "non base64 body is rejected", in: "h1:not-base64", wantErr: core.ErrReleaseContract},
		{name: "whitespace inside the body is rejected", in: "h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAA AAAAAAAAAAAAA=", wantErr: core.ErrReleaseContract},
		{name: "go.mod checksum suffix is rejected", in: testModuleSumA + "/go.mod", wantErr: core.ErrReleaseContract},
		{name: "invalid UTF8 is rejected", in: "h1:\xff", wantErr: core.ErrReleaseContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseGoModuleSum(tc.in)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("parseGoModuleSum(%q) error = %v, want errors.Is(..., %v)", tc.in, err, core.ErrReleaseContract)
				}
				if got != (GoModuleSum{}) {
					t.Fatalf("parseGoModuleSum(%q) = %q, want zero checksum on rejection", tc.in, got.String())
				}
				return
			}
			if err != nil {
				t.Fatalf("parseGoModuleSum(%q) error = %v, want nil", tc.in, err)
			}
			if got.String() != tc.in {
				t.Fatalf("parseGoModuleSum(%q).String() = %q, want %q", tc.in, got.String(), tc.in)
			}
			if len(tc.in) != goModuleSumMaximumBytes {
				t.Fatalf("admitted checksum %q is %d bytes, want the %d byte ceiling the document bound assumes",
					tc.in, len(tc.in), goModuleSumMaximumBytes)
			}
		})
	}
}

// TestNewBuildDependenciesLayerTriadCanonicalizesTheModuleUnion proves the
// container contract every published dependency document rests on.
func TestNewBuildDependenciesLayerTriadCanonicalizesTheModuleUnion(t *testing.T) {
	t.Parallel()

	main := mustModulePath(t, testMainModule)
	cases := []struct {
		modules   func(*testing.T) []BuildDependency
		name      string
		main      GoModulePath
		wantOrder []string
		toolchain GoToolchainIdentity
		wantErr   error
	}{
		{
			name: "neutral empty closure is a valid zero-module union",
			main: main, toolchain: CurrentGoToolchain(),
			modules: func(*testing.T) []BuildDependency { return nil },
		},
		{
			name: "positive single module closure", main: main, toolchain: CurrentGoToolchain(),
			modules:   func(t *testing.T) []BuildDependency { return moduleFixtures(t, "example.com/a") },
			wantOrder: []string{"example.com/a"},
		},
		{
			name: "positive reverse ordered input is canonicalized",
			main: main, toolchain: CurrentGoToolchain(),
			modules: func(t *testing.T) []BuildDependency {
				return moduleFixtures(t, "example.com/c", "example.com/b", "example.com/a")
			},
			wantOrder: []string{"example.com/a", "example.com/b", "example.com/c"},
		},
		{
			name: "positive prefix module and its extension stay distinct",
			main: main, toolchain: CurrentGoToolchain(),
			modules:   func(t *testing.T) []BuildDependency { return moduleFixtures(t, "example.com/ab", "example.com/a") },
			wantOrder: []string{"example.com/a", "example.com/ab"},
		},
		{
			name: "positive exact maximum module count is accepted",
			main: main, toolchain: CurrentGoToolchain(),
			modules:   func(t *testing.T) []BuildDependency { return numberedModules(t, BuildDependencyMaximumCount) },
			wantOrder: numberedModulePaths(BuildDependencyMaximumCount),
		},
		{
			name: "positive one below maximum module count is accepted",
			main: main, toolchain: CurrentGoToolchain(),
			modules:   func(t *testing.T) []BuildDependency { return numberedModules(t, BuildDependencyMaximumCount-1) },
			wantOrder: numberedModulePaths(BuildDependencyMaximumCount - 1),
		},

		{
			name: "negative one above maximum module count",
			main: main, toolchain: CurrentGoToolchain(),
			modules: func(t *testing.T) []BuildDependency { return numberedModules(t, BuildDependencyMaximumCount+1) },
			wantErr: core.ErrReleaseContract,
		},
		{
			name: "negative duplicate module path",
			main: main, toolchain: CurrentGoToolchain(),
			modules: func(t *testing.T) []BuildDependency { return moduleFixtures(t, "example.com/a", "example.com/a") },
			wantErr: core.ErrReleaseContract,
		},
		{
			name: "negative main module listed as its own dependency",
			main: main, toolchain: CurrentGoToolchain(),
			modules: func(t *testing.T) []BuildDependency { return moduleFixtures(t, testMainModule) },
			wantErr: core.ErrReleaseContract,
		},
		{
			name: "negative zero module in the closure",
			main: main, toolchain: CurrentGoToolchain(),
			modules: func(*testing.T) []BuildDependency { return []BuildDependency{{}} },
			wantErr: core.ErrReleaseContract,
		},
		{
			name: "negative unset main module", toolchain: CurrentGoToolchain(),
			modules: func(t *testing.T) []BuildDependency { return moduleFixtures(t, "example.com/a") },
			wantErr: core.ErrReleaseContract,
		},
		{
			name: "negative unknown toolchain", main: main, toolchain: GoToolchainUnknown,
			modules: func(t *testing.T) []BuildDependency { return moduleFixtures(t, "example.com/a") },
			wantErr: core.ErrReleaseContract,
		},
		{
			name: "negative future toolchain", main: main, toolchain: GoToolchainPrimitive2026 + 1,
			modules: func(t *testing.T) []BuildDependency { return moduleFixtures(t, "example.com/a") },
			wantErr: core.ErrReleaseContract,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := newBuildDependencies(tc.main, tc.toolchain, tc.modules(t))
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("newBuildDependencies() error = %v, want errors.Is(..., %v)", err, core.ErrReleaseContract)
				}
				if got != (BuildDependencies{}) {
					t.Fatalf("newBuildDependencies() = %v, want zero facts on rejection", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("newBuildDependencies() error = %v, want nil", err)
			}
			if got.Count() != len(tc.wantOrder) {
				t.Fatalf("newBuildDependencies() count = %d, want %d", got.Count(), len(tc.wantOrder))
			}
			for index, want := range tc.wantOrder {
				module, ok := got.At(index)
				if !ok || module.Path().String() != want {
					t.Fatalf("module %d = (%q, %t), want %q", index, module.Path().String(), ok, want)
				}
			}
		})
	}
}

// TestDependencyObservationMergeUnionsTargetClosures proves the cross-target
// union directly. The live four-target observation cannot prove conflict
// detection, because a healthy repository reports agreeing facts on every
// target, so the disagreement paths need their own proof.
func TestDependencyObservationMergeUnionsTargetClosures(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		left      string
		right     string
		wantMain  string
		wantOrder []string
		wantErr   error
	}{
		{
			name:  "positive disjoint target closures union in canonical order",
			left:  goListPackageFixture("example.com/b", "v1.0.0", testModuleSumA, false) + mainFixture(),
			right: goListPackageFixture("example.com/a", "v1.0.0", testModuleSumA, false) + mainFixture(),
			// This is the exact shape a platform-conditional import produces.
			wantMain: testMainModule, wantOrder: []string{"example.com/a", "example.com/b"},
		},
		{
			name:     "positive identical target closures deduplicate",
			left:     goListPackageFixture("example.com/a", "v1.0.0", testModuleSumA, false) + mainFixture(),
			right:    goListPackageFixture("example.com/a", "v1.0.0", testModuleSumA, false) + mainFixture(),
			wantMain: testMainModule, wantOrder: []string{"example.com/a"},
		},
		{
			name:     "neutral empty right closure preserves the left union",
			left:     goListPackageFixture("example.com/a", "v1.0.0", testModuleSumA, false) + mainFixture(),
			right:    mainFixture(),
			wantMain: testMainModule, wantOrder: []string{"example.com/a"},
		},
		{
			name:     "neutral empty left closure adopts the right union",
			left:     mainFixture(),
			right:    goListPackageFixture("example.com/a", "v1.0.0", testModuleSumA, false) + mainFixture(),
			wantMain: testMainModule, wantOrder: []string{"example.com/a"},
		},
		{
			name:    "negative disagreeing main modules",
			left:    mainFixture(),
			right:   goListPackageFixture("example.com/other", "", "", true),
			wantErr: core.ErrReleaseContract,
		},
		{
			name:    "negative same module at conflicting versions across targets",
			left:    goListPackageFixture("example.com/a", "v1.0.0", testModuleSumA, false) + mainFixture(),
			right:   goListPackageFixture("example.com/a", "v1.0.1", testModuleSumA, false) + mainFixture(),
			wantErr: core.ErrReleaseContract,
		},
		{
			name:    "negative same module at conflicting checksums across targets",
			left:    goListPackageFixture("example.com/a", "v1.0.0", testModuleSumA, false) + mainFixture(),
			right:   goListPackageFixture("example.com/a", "v1.0.0", testModuleSumB, false) + mainFixture(),
			wantErr: core.ErrReleaseContract,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			left, right := &dependencyObservation{}, &dependencyObservation{}
			if err := decodeBuildDependencies(strings.NewReader(tc.left), left); err != nil {
				t.Fatalf("decode left closure error = %v, want nil", err)
			}
			if err := decodeBuildDependencies(strings.NewReader(tc.right), right); err != nil {
				t.Fatalf("decode right closure error = %v, want nil", err)
			}
			before := *left
			err := left.merge(right)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("merge() error = %v, want errors.Is(..., %v)", err, core.ErrReleaseContract)
				}
				if *left != before {
					t.Fatalf("merge() mutated the left closure on rejection")
				}
				return
			}
			if err != nil {
				t.Fatalf("merge() error = %v, want nil", err)
			}
			if left.main.String() != tc.wantMain || left.count != len(tc.wantOrder) {
				t.Fatalf("merge() = (%q, %d modules), want (%q, %d)",
					left.main.String(), left.count, tc.wantMain, len(tc.wantOrder))
			}
			for index, want := range tc.wantOrder {
				if got := left.modules[index].Path().String(); got != want {
					t.Fatalf("merged module %d = %q, want %q", index, got, want)
				}
			}
		})
	}
}

// TestDependencyObservationRejectsClosuresPastItsBound proves the fixed storage
// refuses one module past its ceiling instead of silently truncating the
// closure, which would publish an incomplete dependency document.
func TestDependencyObservationRejectsClosuresPastItsBound(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		count   int
		wantErr error
	}{
		{name: "exact maximum module count is admitted", count: BuildDependencyMaximumCount},
		{name: "one below maximum module count is admitted", count: BuildDependencyMaximumCount - 1},
		{name: "one above maximum module count is rejected", count: BuildDependencyMaximumCount + 1, wantErr: core.ErrReleaseContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			observed := &dependencyObservation{}
			var err error
			for _, module := range numberedModules(t, tc.count) {
				if err = observed.addModule(module); err != nil {
					break
				}
			}
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("addModule() error = %v, want errors.Is(..., %v)", err, core.ErrReleaseContract)
				}
				return
			}
			if err != nil {
				t.Fatalf("addModule() error = %v, want nil", err)
			}
			if observed.count != tc.count {
				t.Fatalf("observed module count = %d, want %d", observed.count, tc.count)
			}
		})
	}
}

// TestBuildDependenciesDocumentRejectsNoncanonicalPublications proves the
// decode side of the customer-visible document. A verifier that silently
// canonicalizes a reordered or duplicated document accepts two byte sequences
// for one set of facts, which breaks the seal the manifest places over it.
func TestBuildDependenciesDocumentRejectsNoncanonicalPublications(t *testing.T) {
	t.Parallel()

	valid, err := newBuildDependencies(mustModulePath(t, testMainModule), CurrentGoToolchain(),
		moduleFixtures(t, "example.com/a", "example.com/b"))
	if err != nil {
		t.Fatalf("newBuildDependencies() error = %v, want nil", err)
	}
	encoded, err := valid.MarshalJSON()
	if err != nil {
		t.Fatalf("BuildDependencies.MarshalJSON() error = %v, want nil", err)
	}
	cases := []struct {
		name string
		data string
	}{
		{name: "reversed module order is rejected", data: strings.Replace(string(encoded),
			`"path":"example.com/a"`, `"path":"example.com/zz"`, 1)},
		{name: "duplicated module is rejected", data: strings.Replace(string(encoded),
			`"path":"example.com/b"`, `"path":"example.com/a"`, 1)},
		{name: "empty document is rejected", data: ""},
		{name: "empty object is rejected", data: `{}`},
		{name: "truncated document is rejected", data: string(encoded[:len(encoded)/2])},
		{name: "array document is rejected", data: `[]`},
		{name: "null document is rejected", data: `null`},
		{name: "unknown field is rejected", data: strings.Replace(string(encoded),
			`"main_module"`, `"main_modules"`, 1)},
		{name: "absent main module is rejected", data: strings.Replace(string(encoded),
			testMainModule, "", 1)},
		{name: "unknown toolchain is rejected", data: strings.Replace(string(encoded),
			`"go_toolchain":"`, `"go_toolchain":"z`, 1)},
		{name: "invalid module checksum is rejected", data: strings.Replace(string(encoded),
			testModuleSumA, "h1:not-base64", 1)},
		{name: "invalid module version is rejected", data: strings.Replace(string(encoded),
			`"version":"v1.0.0"`, `"version":"1.0.0"`, 1)},
		{name: "main module listed as its own dependency is rejected", data: strings.Replace(string(encoded),
			`"path":"example.com/a"`, `"path":"`+testMainModule+`"`, 1)},
		{name: "module count past the ceiling is rejected", data: oversizedDependencyDocument()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.data == string(encoded) {
				t.Fatalf("mutated document = valid document %q, want a distinct hostile fixture", tc.data)
			}
			var decoded BuildDependencies
			err := decoded.UnmarshalJSON([]byte(tc.data))
			if !errors.Is(err, core.ErrReleaseContract) {
				t.Fatalf("BuildDependencies.UnmarshalJSON() error = %v, want errors.Is(..., %v)",
					err, core.ErrReleaseContract)
			}
			if decoded != (BuildDependencies{}) {
				t.Fatalf("BuildDependencies.UnmarshalJSON() receiver = %v, want zero after rejection", decoded)
			}
		})
	}
}

func oversizedDependencyDocument() string {
	var document strings.Builder
	document.WriteString(`{"main_module":"` + testMainModule + `","go_toolchain":"1.26.1","modules":[`)
	for index := range BuildDependencyMaximumCount + 1 {
		if index > 0 {
			document.WriteString(",")
		}
		document.WriteString(`{"path":"example.com/m` + strconv.Itoa(index) +
			`","version":"v1.0.0","sum":"` + testModuleSumA + `"}`)
	}
	document.WriteString(`]}`)
	return document.String()
}

func mainFixture() string { return goListPackageFixture(testMainModule, "", "", true) }

func mustModulePath(t *testing.T, value string) GoModulePath {
	t.Helper()

	path, err := parseGoModulePath(value)
	if err != nil {
		t.Fatalf("parseGoModulePath(%q) error = %v, want nil", value, err)
	}
	return path
}

func moduleFixtures(t *testing.T, paths ...string) []BuildDependency {
	t.Helper()

	modules := make([]BuildDependency, 0, len(paths))
	for _, path := range paths {
		module, err := newBuildDependency(path, "v1.0.0", testModuleSumA)
		if err != nil {
			t.Fatalf("newBuildDependency(%q) error = %v, want nil", path, err)
		}
		modules = append(modules, module)
	}
	return modules
}

func numberedModules(t *testing.T, count int) []BuildDependency {
	t.Helper()

	return moduleFixtures(t, numberedModulePaths(count)...)
}

// numberedModulePaths returns zero-padded paths so lexical module order matches
// numeric order, which keeps the expected canonical order readable.
func numberedModulePaths(count int) []string {
	paths := make([]string, 0, count)
	for index := range count {
		paths = append(paths, "example.com/m"+strconv.Itoa(1_000_000+index))
	}
	return paths
}
