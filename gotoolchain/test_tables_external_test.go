package gotoolchain_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"github.com/deliri/primitive/v2026/hostfacts"
)

// testTablesFixture is one module whose test selection differs by GOOS and
// race, plus packages with no tests, a custom TestMain and a parse error.
func testTablesFixture() map[string]string {
	return map[string]string{
		"go.mod":                       "module example.com/tables\n\ngo 1.27.1\n",
		"tables.go":                    "package tables\n\nfunc Value() int { return 7 }\n",
		"portable_test.go":             "package tables\n\nimport (\n\t\"fmt\"\n\t\"testing\"\n)\n\nfunc TestPortable(t *testing.T) {}\nfunc Testlower(t *testing.T) {}\nfunc BenchmarkPortable(b *testing.B) {}\nfunc FuzzPortable(f *testing.F) { f.Fuzz(func(*testing.T, []byte) {}) }\nfunc ExamplePortable() { fmt.Println(Value()) }\n\n// Output omitted: not runnable.\nfunc ExampleValue_silent() {}\n\nfunc ExampleValue() {\n\tfmt.Println(Value())\n\t// Output: 7\n}\n",
		"darwin_only_test.go":          "//go:build darwin\n\npackage tables\n\nimport \"testing\"\n\nfunc TestDarwinOnly(t *testing.T) {}\n",
		"only_linux_test.go":           "package tables\n\nimport \"testing\"\n\nfunc TestLinuxOnly(t *testing.T) {}\n",
		"race_only_test.go":            "//go:build race\n\npackage tables\n\nimport \"testing\"\n\nfunc TestRaceOnly(t *testing.T) {}\n",
		"external_test.go":             "package tables_test\n\nimport \"testing\"\n\nfunc TestExternal(t *testing.T) {}\n",
		"darwintests/code.go":          "package darwintests\n\nfunc Value() int { return 1 }\n",
		"darwintests/x_darwin_test.go": "package darwintests\n\nimport \"testing\"\n\nfunc TestOnlyOnDarwin(t *testing.T) {}\n",
		"notests/code.go":              "package notests\n\nfunc Value() int { return 2 }\n",
		"custommain/code.go":           "package custommain\n",
		"custommain/main_test.go":      "package custommain\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestMain(m *testing.M) { os.Exit(0) }\nfunc TestHidden(t *testing.T) {}\n",
		"many/code.go":                 "package many\n",
		"many/many_test.go":            "package many\n\nimport \"testing\"\n\nfunc TestMany00(t *testing.T) {}\nfunc TestMany01(t *testing.T) {}\nfunc TestMany02(t *testing.T) {}\nfunc TestMany03(t *testing.T) {}\nfunc TestMany04(t *testing.T) {}\nfunc TestMany05(t *testing.T) {}\nfunc TestMany06(t *testing.T) {}\nfunc TestMany07(t *testing.T) {}\nfunc TestMany08(t *testing.T) {}\nfunc TestMany09(t *testing.T) {}\nfunc TestMany10(t *testing.T) {}\nfunc TestMany11(t *testing.T) {}\nfunc TestMany12(t *testing.T) {}\nfunc TestMany13(t *testing.T) {}\nfunc TestMany14(t *testing.T) {}\nfunc TestMany15(t *testing.T) {}\nfunc TestMany16(t *testing.T) {}\nfunc TestMany17(t *testing.T) {}\nfunc TestMany18(t *testing.T) {}\nfunc TestMany19(t *testing.T) {}\nfunc TestMany20(t *testing.T) {}\nfunc TestMany21(t *testing.T) {}\nfunc TestMany22(t *testing.T) {}\nfunc TestMany23(t *testing.T) {}\nfunc TestMany24(t *testing.T) {}\nfunc TestMany25(t *testing.T) {}\nfunc TestMany26(t *testing.T) {}\nfunc TestMany27(t *testing.T) {}\nfunc TestMany28(t *testing.T) {}\nfunc TestMany29(t *testing.T) {}\nfunc TestMany30(t *testing.T) {}\nfunc TestMany31(t *testing.T) {}\nfunc TestMany32(t *testing.T) {}\nfunc TestMany33(t *testing.T) {}\nfunc TestMany34(t *testing.T) {}\nfunc TestMany35(t *testing.T) {}\nfunc TestMany36(t *testing.T) {}\nfunc TestMany37(t *testing.T) {}\nfunc TestMany38(t *testing.T) {}\nfunc TestMany39(t *testing.T) {}\nfunc TestMany40(t *testing.T) {}\nfunc TestMany41(t *testing.T) {}\nfunc TestMany42(t *testing.T) {}\nfunc TestMany43(t *testing.T) {}\nfunc TestMany44(t *testing.T) {}\nfunc TestMany45(t *testing.T) {}\nfunc TestMany46(t *testing.T) {}\nfunc TestMany47(t *testing.T) {}\nfunc TestMany48(t *testing.T) {}\nfunc TestMany49(t *testing.T) {}\nfunc TestMany50(t *testing.T) {}\nfunc TestMany51(t *testing.T) {}\nfunc TestMany52(t *testing.T) {}\nfunc TestMany53(t *testing.T) {}\nfunc TestMany54(t *testing.T) {}\nfunc TestMany55(t *testing.T) {}\nfunc TestMany56(t *testing.T) {}\nfunc TestMany57(t *testing.T) {}\nfunc TestMany58(t *testing.T) {}\nfunc TestMany59(t *testing.T) {}\nfunc TestMany60(t *testing.T) {}\nfunc TestMany61(t *testing.T) {}\nfunc TestMany62(t *testing.T) {}\nfunc TestMany63(t *testing.T) {}\nfunc TestMany64(t *testing.T) {}\nfunc TestMany65(t *testing.T) {}\nfunc TestMany66(t *testing.T) {}\nfunc TestMany67(t *testing.T) {}\nfunc TestMany68(t *testing.T) {}\nfunc TestMany69(t *testing.T) {}\nfunc TestMany70(t *testing.T) {}\nfunc TestMany71(t *testing.T) {}\nfunc TestMany72(t *testing.T) {}\nfunc TestMany73(t *testing.T) {}\nfunc TestMany74(t *testing.T) {}\nfunc TestMany75(t *testing.T) {}\nfunc TestMany76(t *testing.T) {}\nfunc TestMany77(t *testing.T) {}\nfunc TestMany78(t *testing.T) {}\nfunc TestMany79(t *testing.T) {}\n",
		"broken/code.go":               "package broken\n",
		"broken/broken_test.go":        "package broken\n\nfunc TestBroken(t *testing.T {\n",
	}
}

func TestObserveTestTablesNativeLayerTriad(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	for name, content := range testTablesFixture() {
		path := filepath.Join(directory, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	root, err := core.ParseAbsolutePath(directory)
	if err != nil {
		t.Fatal(err)
	}
	type want struct {
		tests, benchmarks, fuzz, examples []string
		customTestMain                    bool
	}
	portable := []string{"TestPortable", "TestExternal"}
	for _, tc := range []struct {
		layer, name, goos, goarch, pkg string
		race                           bool
		want                           *want
		cancel                         bool
		wantErr                        error
	}{
		{layer: "positive", name: "darwin selects darwin-only tests and runnable examples", goos: "darwin", goarch: "arm64", pkg: "example.com/tables", want: &want{tests: append([]string{"TestDarwinOnly"}, portable...), benchmarks: []string{"BenchmarkPortable"}, fuzz: []string{"FuzzPortable"}, examples: []string{"ExampleValue"}}},
		{layer: "positive", name: "linux selects linux-only tests instead", goos: "linux", goarch: "amd64", pkg: "example.com/tables", want: &want{tests: append([]string{"TestLinuxOnly"}, portable...), benchmarks: []string{"BenchmarkPortable"}, fuzz: []string{"FuzzPortable"}, examples: []string{"ExampleValue"}}},
		{layer: "positive", name: "race adds race-only tests", goos: "linux", goarch: "amd64", pkg: "example.com/tables", race: true, want: &want{tests: append([]string{"TestLinuxOnly", "TestRaceOnly"}, portable...), benchmarks: []string{"BenchmarkPortable"}, fuzz: []string{"FuzzPortable"}, examples: []string{"ExampleValue"}}},
		{layer: "positive", name: "a package with only darwin test files is complete and positive on darwin", goos: "darwin", goarch: "arm64", pkg: "example.com/tables/darwintests", want: &want{tests: []string{"TestOnlyOnDarwin"}}},
		{layer: "neutral", name: "the same package is complete and empty on linux", goos: "linux", goarch: "amd64", pkg: "example.com/tables/darwintests", want: &want{}},
		{layer: "neutral", name: "a package without test files is complete and empty", goos: "linux", goarch: "amd64", pkg: "example.com/tables/notests", want: &want{}},
		{layer: "positive", name: "a custom TestMain is reported beside its tests", goos: "linux", goarch: "amd64", pkg: "example.com/tables/custommain", want: &want{tests: []string{"TestHidden"}, customTestMain: true}},
		{layer: "negative", name: "a test file cmd/go cannot load is an error, never empty", goos: "linux", goarch: "amd64", pkg: "example.com/tables/broken", wantErr: core.ErrGoToolchainExecution},
		{layer: "negative", name: "a missing package is an error, never empty", goos: "linux", goarch: "amd64", pkg: "example.com/tables/absent", wantErr: core.ErrGoToolchainExecution},
		{layer: "negative", name: "a cancelled query observes nothing", goos: "linux", goarch: "amd64", pkg: "example.com/tables", cancel: true, wantErr: context.Canceled},
	} {
		t.Run(tc.layer+"/"+tc.name, func(t *testing.T) {
			t.Parallel()
			capability := testTablesCapability(t, tc.goos, tc.goarch)
			pkg, err := gomodule.ParseImportPath(tc.pkg)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.cancel {
				cancel()
			}
			got, err := capability.ObserveTestTables(ctx, gotoolchain.TestTableRequest{WorkingDirectory: root, Package: pkg, Race: tc.race})
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("ObserveTestTables = %+v/%v, want %v", got, err, tc.wantErr)
				}
				if len(got.Tests)+len(got.Benchmarks)+len(got.FuzzTargets)+len(got.Examples) != 0 || got.CustomTestMain {
					t.Fatalf("refused query returned tables %+v, want none", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ObserveTestTables = %v, want tables", err)
			}
			for _, table := range []struct {
				name      string
				got       []gotoolchain.TestName
				wantNames []string
			}{{"tests", got.Tests, tc.want.tests}, {"benchmarks", got.Benchmarks, tc.want.benchmarks}, {"fuzz targets", got.FuzzTargets, tc.want.fuzz}, {"examples", got.Examples, tc.want.examples}} {
				names := make([]string, 0, len(table.got))
				for _, name := range table.got {
					names = append(names, name.String())
				}
				slices.Sort(names)
				expected := slices.Clone(table.wantNames)
				slices.Sort(expected)
				if !slices.Equal(names, expected) {
					t.Fatalf("%s = %q, want exactly %q", table.name, names, expected)
				}
			}
			if got.CustomTestMain != tc.want.customTestMain {
				t.Fatalf("custom TestMain = %t, want %t", got.CustomTestMain, tc.want.customTestMain)
			}
		})
	}
}

func TestObserveTestTablesRefusesInvalidIngress(t *testing.T) {
	t.Parallel()
	capability := testTablesCapability(t, "linux", "amd64")
	pkg, err := gomodule.ParseImportPath("example.com/tables")
	if err != nil {
		t.Fatal(err)
	}
	root, err := core.ParseAbsolutePath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		ctx     context.Context
		request gotoolchain.TestTableRequest
		want    error
	}{
		{name: "nil context", request: gotoolchain.TestTableRequest{WorkingDirectory: root, Package: pkg}, want: core.ErrNilContext},
		{name: "unset directory", ctx: t.Context(), request: gotoolchain.TestTableRequest{Package: pkg}, want: core.ErrGoToolchainContract},
		{name: "unset package", ctx: t.Context(), request: gotoolchain.TestTableRequest{WorkingDirectory: root}, want: core.ErrGoToolchainContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := capability.ObserveTestTables(tc.ctx, tc.request)
			if !errors.Is(err, tc.want) || len(got.Tests) != 0 {
				t.Fatalf("ObserveTestTables = %+v/%v, want %v and no tables", got, err, tc.want)
			}
		})
	}
	var zero gotoolchain.Capability
	if got, err := zero.ObserveTestTables(t.Context(), gotoolchain.TestTableRequest{WorkingDirectory: root, Package: pkg}); !errors.Is(err, core.ErrGoToolchainContract) || len(got.Tests) != 0 {
		t.Fatalf("zero capability = %+v/%v, want contract refusal", got, err)
	}
}

// testTablesCapability opens cmd/go for one target build context with no
// network, workspace or environment file.
func testTablesCapability(t *testing.T, goos, goarch string) gotoolchain.Capability {
	t.Helper()
	environment, err := hostfacts.AmbientEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	for _, variable := range []struct{ name, value string }{
		{name: "GOOS", value: goos}, {name: "GOARCH", value: goarch}, {name: "CGO_ENABLED", value: "1"}, {name: "GOENV", value: "off"}, {name: "GOFLAGS", value: "-buildvcs=false"}, {name: "GOTOOLCHAIN", value: "local"}, {name: "GOPROXY", value: "off"}, {name: "GOSUMDB", value: "off"},
	} {
		environment = compilerEnvironmentVariable(t, environment, variable.name, variable.value)
	}
	limits, err := gotoolchain.DefaultLimits()
	if err != nil {
		t.Fatal(err)
	}
	capability, err := gotoolchain.Open(t.Context(), gotoolchain.Configuration{Workspace: gotoolchain.WorkspaceModeDisabled, Limits: limits, Environment: &environment})
	if err != nil {
		t.Fatal(err)
	}
	return capability
}

// A generated test main larger than the capability's output bound is refused
// before it is parsed, never truncated into a smaller selection.
func TestObserveTestTablesRefusesGeneratedMainBeyondOutputBound(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	for name, content := range testTablesFixture() {
		path := filepath.Join(directory, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	root, err := core.ParseAbsolutePath(directory)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := gomodule.ParseImportPath("example.com/tables/many")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := hostfacts.AmbientEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	for _, variable := range []struct{ name, value string }{
		{name: "GOOS", value: "linux"}, {name: "GOARCH", value: "amd64"}, {name: "CGO_ENABLED", value: "1"}, {name: "GOENV", value: "off"}, {name: "GOFLAGS", value: "-buildvcs=false"}, {name: "GOTOOLCHAIN", value: "local"}, {name: "GOPROXY", value: "off"}, {name: "GOSUMDB", value: "off"},
	} {
		environment = compilerEnvironmentVariable(t, environment, variable.name, variable.value)
	}
	limits, err := gotoolchain.DefaultLimits()
	if err != nil {
		t.Fatal(err)
	}
	// 80 test entries make the generated main several KiB; the listing itself
	// stays far below this bound.
	limits.OutputBytes, err = core.NewByteCount(1536)
	if err != nil {
		t.Fatal(err)
	}
	capability, err := gotoolchain.Open(t.Context(), gotoolchain.Configuration{Workspace: gotoolchain.WorkspaceModeDisabled, Limits: limits, Environment: &environment})
	if err != nil {
		t.Fatal(err)
	}
	got, err := capability.ObserveTestTables(t.Context(), gotoolchain.TestTableRequest{WorkingDirectory: root, Package: pkg})
	if !errors.Is(err, core.ErrGoToolchainOutput) || !strings.Contains(err.Error(), "exceeds the output bound") || len(got.Tests) != 0 {
		t.Fatalf("ObserveTestTables = %d tests/%v, want the generated main refused at the output bound", len(got.Tests), err)
	}
}
