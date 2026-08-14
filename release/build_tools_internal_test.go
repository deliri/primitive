package release

import (
	"debug/buildinfo"
	"errors"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/garble"
)

// TestParseGoVersionOutputPressuresEverySideOfTheProbeGrammar exhausts the
// exact `go version` grammar. The probe is the only evidence that the file on
// disk is the admitted compiler, so every malformed shape must be rejected.
func TestParseGoVersionOutputPressuresEverySideOfTheProbeGrammar(t *testing.T) {
	t.Parallel()

	const want = goToolchainVersionPrimitive2026
	darwinARM64 := core.Platform{
		OperatingSystem: core.OperatingSystemDarwin, Architecture: core.CPUArchitectureARM64,
	}
	linuxAMD64 := core.Platform{
		OperatingSystem: core.OperatingSystemLinux, Architecture: core.CPUArchitectureAMD64,
	}

	cases := []struct {
		wantErr      error
		name         string
		output       string
		wantPlatform core.Platform
	}{
		{name: "canonical darwin arm64 line is accepted", output: "go version " + want + " darwin/arm64\n", wantPlatform: darwinARM64},
		{name: "canonical linux amd64 line is accepted", output: "go version " + want + " linux/amd64\n", wantPlatform: linuxAMD64},
		{name: "canonical windows amd64 line is accepted", output: "go version " + want + " windows/amd64\n", wantPlatform: core.Platform{OperatingSystem: core.OperatingSystemWindows, Architecture: core.CPUArchitectureAMD64}},
		{name: "canonical linux arm64 line is accepted", output: "go version " + want + " linux/arm64\n", wantPlatform: core.Platform{OperatingSystem: core.OperatingSystemLinux, Architecture: core.CPUArchitectureARM64}},
		{name: "empty output is rejected", output: "", wantErr: core.ErrReleaseContract},
		{name: "whitespace only output is rejected", output: " \t\n ", wantErr: core.ErrReleaseContract},
		{name: "carriage return line ending is rejected", output: "go version " + want + " linux/amd64\r\n", wantErr: core.ErrReleaseContract},
		{name: "leading whitespace is rejected", output: " go version " + want + " linux/amd64\n", wantErr: core.ErrReleaseContract},
		{name: "trailing whitespace is rejected", output: "go version " + want + " linux/amd64 \n", wantErr: core.ErrReleaseContract},
		{name: "second trailing newline is rejected", output: "go version " + want + " linux/amd64\n\n", wantErr: core.ErrReleaseContract},
		{name: "no trailing newline is rejected", output: "go version " + want + " linux/amd64", wantErr: core.ErrReleaseContract},
		{name: "repeated interior whitespace is rejected", output: "go  version " + want + " linux/amd64\n", wantErr: core.ErrReleaseContract},
		{name: "tab separated fields are rejected", output: "go\tversion\t" + want + "\tlinux/amd64\n", wantErr: core.ErrReleaseContract},
		{name: "three fields is rejected", output: "go version " + want, wantErr: core.ErrReleaseContract},
		{name: "two fields is rejected", output: "go version", wantErr: core.ErrReleaseContract},
		{name: "one field is rejected", output: "go", wantErr: core.ErrReleaseContract},
		{name: "five fields is rejected", output: "go version " + want + " linux/amd64 extra", wantErr: core.ErrReleaseContract},
		{name: "appended toolchain build settings are rejected", output: "go version " + want + " linux/amd64 X:fieldtrack", wantErr: core.ErrReleaseContract},
		{name: "wrong program token is rejected", output: "tinygo version " + want + " linux/amd64", wantErr: core.ErrReleaseContract},
		{name: "wrong subcommand token is rejected", output: "go env " + want + " linux/amd64", wantErr: core.ErrReleaseContract},
		{name: "one patch below the pinned toolchain is rejected", output: "go version go1.26.4 linux/amd64", wantErr: core.ErrReleaseContract},
		{name: "one patch above the pinned toolchain is rejected", output: "go version go1.26.6 linux/amd64", wantErr: core.ErrReleaseContract},
		{name: "one minor above the pinned toolchain is rejected", output: "go version go1.27.0 linux/amd64", wantErr: core.ErrReleaseContract},
		{name: "devel toolchain is rejected", output: "go version devel go1.26.5 linux/amd64", wantErr: core.ErrReleaseContract},
		{name: "pinned version as a prefix is rejected", output: "go version " + want + "rc1 linux/amd64", wantErr: core.ErrReleaseContract},
		{name: "empty version token is rejected", output: "go version  linux/amd64", wantErr: core.ErrReleaseContract},
		{name: "platform without a separator is rejected", output: "go version " + want + " linuxamd64", wantErr: core.ErrReleaseContract},
		{name: "platform with two separators is rejected", output: "go version " + want + " linux/amd64/v3", wantErr: core.ErrReleaseContract},
		{name: "platform with a trailing separator is rejected", output: "go version " + want + " linux/", wantErr: core.ErrReleaseContract},
		{name: "platform with a leading separator is rejected", output: "go version " + want + " /amd64", wantErr: core.ErrReleaseContract},
		{name: "unknown operating system is rejected", output: "go version " + want + " plan9/amd64", wantErr: core.ErrReleaseContract},
		{name: "unknown architecture is rejected", output: "go version " + want + " linux/riscv64", wantErr: core.ErrReleaseContract},
		{name: "already hyphenated platform is rejected", output: "go version " + want + " linux-amd64", wantErr: core.ErrReleaseContract},
		{name: "uppercase platform is rejected", output: "go version " + want + " Linux/AMD64", wantErr: core.ErrReleaseContract},
		{name: "oversized trailing field is rejected", output: "go version " + want + " " + strings.Repeat("a", 4096), wantErr: core.ErrReleaseContract},
		{name: "embedded NUL platform is rejected", output: "go version " + want + " linux/amd64\x00", wantErr: core.ErrReleaseContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := parseGoVersionOutput(tc.output, want)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("parseGoVersionOutput(%q) error = %v, want %v", tc.output, gotErr, core.ErrReleaseContract)
				}
				if got != (core.Platform{}) {
					t.Fatalf("parseGoVersionOutput(%q) platform = %v, want zero on rejection", tc.output, got)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("parseGoVersionOutput(%q) error = %v, want nil", tc.output, gotErr)
			}
			if got != tc.wantPlatform {
				t.Fatalf("parseGoVersionOutput(%q) platform = %v, want %v", tc.output, got, tc.wantPlatform)
			}
		})
	}
}

// TestValidateGarbleBuildInfoRejectsEverySingleFieldSubstitution proves each
// pinned Garble module fact is load bearing on its own.
func TestValidateGarbleBuildInfoRejectsEverySingleFieldSubstitution(t *testing.T) {
	t.Parallel()

	tool := garble.CurrentTool()
	module, err := tool.ModulePath()
	if err != nil {
		t.Fatalf("garble.ToolIdentity.ModulePath() error = %v, want nil", err)
	}
	version, err := tool.Version()
	if err != nil {
		t.Fatalf("garble.ToolIdentity.Version() error = %v, want nil", err)
	}
	sum, err := tool.ModuleSum()
	if err != nil {
		t.Fatalf("garble.ToolIdentity.ModuleSum() error = %v, want nil", err)
	}
	goVersion, err := CurrentGoToolchain().Version()
	if err != nil {
		t.Fatalf("GoToolchainIdentity.Version() error = %v, want nil", err)
	}
	admitted := func() *buildinfo.BuildInfo {
		return &buildinfo.BuildInfo{
			Path:      module,
			Main:      debug.Module{Path: module, Version: version, Sum: sum},
			GoVersion: goVersion,
		}
	}

	if err := validateGarbleBuildInfo(admitted(), tool, goVersion); err != nil {
		t.Fatalf("validateGarbleBuildInfo(admitted) error = %v, want nil", err)
	}

	cases := []struct {
		mutate func(*buildinfo.BuildInfo)
		name   string
	}{
		{name: "empty command path is rejected", mutate: func(i *buildinfo.BuildInfo) { i.Path = "" }},
		{name: "forked command path is rejected", mutate: func(i *buildinfo.BuildInfo) { i.Path = module + "/v2" }},
		{name: "lookalike command path is rejected", mutate: func(i *buildinfo.BuildInfo) { i.Path = "mvdan.cc/garbIe" }},
		{name: "empty main module path is rejected", mutate: func(i *buildinfo.BuildInfo) { i.Main.Path = "" }},
		{name: "vendored main module path is rejected", mutate: func(i *buildinfo.BuildInfo) { i.Main.Path = "example.com/vendor/garble" }},
		{name: "empty module version is rejected", mutate: func(i *buildinfo.BuildInfo) { i.Main.Version = "" }},
		{name: "devel module version is rejected", mutate: func(i *buildinfo.BuildInfo) { i.Main.Version = "(devel)" }},
		{name: "neighbouring module version is rejected", mutate: func(i *buildinfo.BuildInfo) { i.Main.Version = version + "1" }},
		{name: "empty module sum is rejected", mutate: func(i *buildinfo.BuildInfo) { i.Main.Sum = "" }},
		{name: "one byte module sum change is rejected", mutate: func(i *buildinfo.BuildInfo) { i.Main.Sum = flipLastByteForTest(sum) }},
		{name: "empty go version is rejected", mutate: func(i *buildinfo.BuildInfo) { i.GoVersion = "" }},
		{name: "one patch below go version is rejected", mutate: func(i *buildinfo.BuildInfo) { i.GoVersion = "go1.26.4" }},
		{name: "one patch above go version is rejected", mutate: func(i *buildinfo.BuildInfo) { i.GoVersion = "go1.26.6" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			info := admitted()
			tc.mutate(info)
			if gotErr := validateGarbleBuildInfo(info, tool, goVersion); !errors.Is(gotErr, core.ErrReleaseContract) {
				t.Fatalf("validateGarbleBuildInfo() error = %v, want %v", gotErr, core.ErrReleaseContract)
			}
		})
	}

	if gotErr := validateGarbleBuildInfo(admitted(), garble.ToolIdentityUnknown, goVersion); gotErr == nil {
		t.Fatal("validateGarbleBuildInfo(unknown tool) error = nil, want rejection")
	}
}

func flipLastByteForTest(value string) string {
	if value == "" {
		return "x"
	}
	last := value[len(value)-1]
	if last == 'a' {
		return value[:len(value)-1] + "b"
	}
	return value[:len(value)-1] + "a"
}
