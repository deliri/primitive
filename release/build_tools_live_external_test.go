package release_test

import (
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/release"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestVerifyBuildToolsProvesTheExactInstalledExecutables(t *testing.T) {
	t.Parallel()

	request := buildToolVerificationRequestForLiveTest(t)
	verified, err := release.VerifyBuildTools(t.Context(), request)
	if err != nil {
		t.Fatalf("release.VerifyBuildTools() error = %v, want nil", err)
	}
	if err := verified.Validate(); err != nil {
		t.Fatalf("release.VerifiedBuildTools.Validate() error = %v, want nil", err)
	}
	if verified.GoExecutable() != request.GoExecutable {
		t.Fatalf("verified executable paths differ from the inspected paths")
	}
	if verified.GoToolchain() != release.CurrentGoToolchain() {
		t.Fatalf("verified tool identity = %v, want current", verified.GoToolchain())
	}
	wantPlatform := core.Platform{OperatingSystem: core.OperatingSystemDarwin, Architecture: core.CPUArchitectureARM64}
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Skip("Primitive release toolchain is currently admitted on the Darwin/ARM64 build host")
	}
	if verified.HostPlatform() != wantPlatform {
		t.Fatalf("verified host platform = %v, want %v", verified.HostPlatform(), wantPlatform)
	}
	if err := verified.GoExecutableDigest().Validate(); err != nil {
		t.Fatalf("verified Go executable digest error = %v, want nil", err)
	}
}

func buildToolVerificationRequestForLiveTest(t *testing.T) release.BuildToolVerificationRequest {
	t.Helper()

	goPath, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("exec.LookPath(go) error = %v, want installed Go", err)
	}
	goExecutable, err := core.ParseAbsolutePath(goPath)
	if err != nil {
		t.Fatalf("core.ParseAbsolutePath(go) error = %v, want nil", err)
	}
	workingText, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v, want nil", err)
	}
	workingDirectory, err := core.ParseAbsolutePath(workingText)
	if err != nil {
		t.Fatalf("core.ParseAbsolutePath(working directory) error = %v, want nil", err)
	}
	environment, err := process.ParseExactEnvironment([]string{})
	if err != nil {
		t.Fatalf("process.ParseExactEnvironment(empty) error = %v, want nil", err)
	}
	waitDelay, err := temporal.NewDuration(10 * time.Second)
	if err != nil {
		t.Fatalf("temporal.NewDuration() error = %v, want nil", err)
	}
	return release.BuildToolVerificationRequest{
		HostEnvironment: environment, GoExecutable: goExecutable,
		WorkingDirectory: workingDirectory,
		WaitDelay:        waitDelay,
	}
}

func verifiedBuildToolsForLiveTest(t *testing.T) release.VerifiedBuildTools {
	t.Helper()

	verified, err := release.VerifyBuildTools(t.Context(), buildToolVerificationRequestForLiveTest(t))
	if err != nil {
		t.Fatalf("release.VerifyBuildTools() error = %v, want nil", err)
	}
	return verified
}
