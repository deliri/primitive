package hostfacts

import (
	"errors"
	"runtime"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestCurrentPlatformProjectsTheExactRuntimeTarget(t *testing.T) {
	t.Parallel()

	got, gotErr := CurrentPlatform()
	want, wantErr := platformFromRuntime(runtime.GOOS, runtime.GOARCH)
	if (gotErr == nil) != (wantErr == nil) {
		t.Fatalf("CurrentPlatform() error = %v, platformFromRuntime() error = %v", gotErr, wantErr)
	}
	if gotErr != nil {
		if !errors.Is(gotErr, core.ErrPrimitiveContract) {
			t.Fatalf("CurrentPlatform() error = %v, want %v", gotErr, core.ErrPrimitiveContract)
		}
		return
	}
	if got != want {
		t.Fatalf("CurrentPlatform() = %v, want %v", got, want)
	}
}

func TestPlatformFromRuntimePressuresBothClosedDomains(t *testing.T) {
	t.Parallel()

	operatingSystems := [...]struct {
		value string
		want  core.OperatingSystem
	}{
		{value: core.OperatingSystemDarwin.String(), want: core.OperatingSystemDarwin},
		{value: core.OperatingSystemLinux.String(), want: core.OperatingSystemLinux},
		{value: core.OperatingSystemWindows.String(), want: core.OperatingSystemWindows},
	}
	architectures := [...]struct {
		value string
		want  core.CPUArchitecture
	}{
		{value: core.CPUArchitectureAMD64.String(), want: core.CPUArchitectureAMD64},
		{value: core.CPUArchitectureARM64.String(), want: core.CPUArchitectureARM64},
	}
	for _, operatingSystem := range operatingSystems {
		for _, architecture := range architectures {
			got, gotErr := platformFromRuntime(operatingSystem.value, architecture.value)
			want := core.Platform{OperatingSystem: operatingSystem.want, Architecture: architecture.want}
			if gotErr != nil || got != want {
				t.Fatalf("platformFromRuntime(%q, %q) = (%v, %v), want (%v, nil)", operatingSystem.value, architecture.value, got, gotErr, want)
			}
		}
	}

	for _, input := range [][2]string{
		{"", core.CPUArchitectureAMD64.String()},
		{"Darwin", core.CPUArchitectureAMD64.String()},
		{"freebsd", core.CPUArchitectureAMD64.String()},
		{core.OperatingSystemLinux.String(), ""},
		{core.OperatingSystemLinux.String(), "AMD64"},
		{core.OperatingSystemLinux.String(), "386"},
	} {
		got, gotErr := platformFromRuntime(input[0], input[1])
		if got != (core.Platform{}) || !errors.Is(gotErr, core.ErrPrimitiveContract) {
			t.Fatalf("platformFromRuntime(%q, %q) = (%v, %v), want (zero, %v)", input[0], input[1], got, gotErr, core.ErrPrimitiveContract)
		}
	}
}
