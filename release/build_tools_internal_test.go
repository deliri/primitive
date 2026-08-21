package release

import (
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
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
		{name: "previous minor toolchain is rejected", output: "go version go1.26.9 linux/amd64", wantErr: core.ErrReleaseContract},
		{name: "one patch above the pinned toolchain is rejected", output: "go version go1.27.1 linux/amd64", wantErr: core.ErrReleaseContract},
		{name: "one minor above the pinned toolchain is rejected", output: "go version go1.28.0 linux/amd64", wantErr: core.ErrReleaseContract},
		{name: "devel toolchain is rejected", output: "go version devel go1.27.0 linux/amd64", wantErr: core.ErrReleaseContract},
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
