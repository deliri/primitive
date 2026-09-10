package release_test

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/hostfacts"
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
	wantPath, err := filestore.Canonicalize(t.Context(), request.GoExecutable)
	if err != nil {
		t.Fatalf("Canonicalize(installed executable) error = %v, want nil", err)
	}
	if verified.GoExecutable() != wantPath {
		t.Fatalf("verified executable path = %v, want %v", verified.GoExecutable(), wantPath)
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

	name, err := core.ParsePathComponent("go")
	if err != nil {
		t.Fatalf("core.ParsePathComponent(go) error = %v, want nil", err)
	}
	goExecutable, err := process.Resolve(t.Context(), name)
	if err != nil {
		t.Fatalf("process.Resolve(go) error = %v, want nil", err)
	}
	workingDirectory, err := hostfacts.WorkingDirectory()
	if err != nil {
		t.Fatalf("hostfacts.WorkingDirectory() error = %v, want nil", err)
	}
	environment, err := process.ParseExactEnvironment([]string{})
	if err != nil {
		t.Fatalf("process.ParseExactEnvironment(empty) error = %v, want nil", err)
	}
	waitDelay, err := temporal.DurationFromSeconds(10)
	if err != nil {
		t.Fatalf("temporal.DurationFromSeconds() error = %v, want nil", err)
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

// Process owns runnability, Filestore owns resolution, and Release must retain
// the exact regular path it inspected and executed, rather than a mutable alias.
func TestBuildToolResolvedPathLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                             string
		links                            []string
		unresolvable, canceled, external bool
		wantErr                          error
	}{
		{name: "regular compiler retains exact path"},
		{name: "confined final link retains target instead of alias", links: []string{"go-real"}},
		{name: "two link hops retain the final compiler", links: []string{"middle", "go-real"}},
		{name: "absolute link outside alias directory retains explicit target", links: []string{"go-real"}, external: true},
		{name: "link cycle cannot emit verified tools", links: []string{"go-link"}, unresolvable: true, wantErr: core.ErrFilestoreSource},
		{name: "absent target cannot emit verified tools", links: []string{"absent"}, unresolvable: true, wantErr: fs.ErrNotExist},
		{name: "canceled link resolution cannot emit proof", links: []string{"go-real"}, canceled: true, wantErr: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			canonicalDirectory, err := filestore.Canonicalize(t.Context(), inspectionAbsolutePath(t, directory))
			if err != nil {
				t.Fatalf("Canonicalize(fixture directory) error = %v, want nil", err)
			}
			directory = canonicalDirectory.String()
			request := buildToolVerificationRequestForLiveTest(t)
			installed, err := filestore.Canonicalize(t.Context(), request.GoExecutable)
			if err != nil {
				t.Fatalf("Canonicalize(installed compiler) error = %v, want nil", err)
			}
			request.HostEnvironment, err = process.ParseExactEnvironment([]string{
				"GOROOT=" + filepath.Dir(filepath.Dir(installed.String())), "GOTOOLCHAIN=local", "GOENV=off",
			})
			if err != nil {
				t.Fatalf("compiler fixture environment error = %v, want nil", err)
			}
			target := inspectionAbsolutePath(t, filepath.Join(directory, "go-real"))
			copyBuildToolFixture(t, request.GoExecutable, target)
			wantPath, err := filestore.Canonicalize(t.Context(), target)
			if err != nil {
				t.Fatalf("Canonicalize(target) error = %v, want nil", err)
			}
			wantDigest, _, _ := digestInspectionFixture(t, target)
			root, err := filestore.OpenRoot(t.Context(), inspectionAbsolutePath(t, directory))
			if err != nil {
				t.Fatalf("OpenRoot(fixture) error = %v, want nil", err)
			}
			t.Cleanup(func() {
				if err := root.Close(); err != nil {
					t.Errorf("fixture root Close error = %v, want nil", err)
				}
			})
			names := []string{"go-link", "middle"}
			for i, link := range tc.links {
				if tc.external {
					link = installed.String()
					wantPath = installed
				}
				if err := root.Symlink(link, names[i]); err != nil {
					t.Fatalf("root.Symlink(%q) error = %v, want nil", link, err)
				}
			}
			request.GoExecutable = target
			if len(tc.links) != 0 {
				request.GoExecutable = inspectionAbsolutePath(t, filepath.Join(directory, names[0]))
			}
			if !tc.unresolvable {
				got, err := process.ResolveExecutable(t.Context(), request.GoExecutable)
				if err != nil || got != request.GoExecutable {
					t.Fatalf("Process resolved path = (%v, %v), want (%v, nil)", got, err, request.GoExecutable)
				}
			}
			inputPath := request.GoExecutable
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.canceled {
				cancel()
			}
			got, gotErr := release.VerifyBuildTools(ctx, request)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("VerifyBuildTools error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if !errors.Is(gotErr, core.ErrReleaseContract) || got != (release.VerifiedBuildTools{}) {
					t.Fatalf("refused tools = (%v, %v), want zero and Release refusal", got, gotErr)
				}
			} else if got.Validate() != nil || got.GoExecutable() != wantPath || got.GoExecutableDigest() != wantDigest || got.GoToolchain() != release.CurrentGoToolchain() {
				t.Fatalf("verified tools = (%v, %v, %v), want (%v, %v, current toolchain)", got.GoExecutable(), got.GoExecutableDigest(), got.GoToolchain(), wantPath, wantDigest)
			}
			if request.GoExecutable != inputPath {
				t.Fatalf("request path = %v, want preserved %v", request.GoExecutable, inputPath)
			}
			afterDigest, _, _ := digestInspectionFixture(t, target)
			if afterDigest != wantDigest {
				t.Fatalf("target digest after verification = %v, want preserved %v", afterDigest, wantDigest)
			}
		})
	}
}

func copyBuildToolFixture(t *testing.T, source, target core.AbsolutePath) {
	t.Helper()
	sourceLocation := releaseFixtureLocation(t, source)
	file, err := filestore.OpenRead(t.Context(), filestore.ReadHandleRequest{Location: sourceLocation})
	if err != nil {
		t.Fatalf("OpenRead(compiler) error = %v, want nil", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("compiler Close error = %v, want nil", err)
		}
	}()
	stage, err := core.ParseRelativePath("compiler-stage")
	if err != nil {
		t.Fatalf("compiler stage error = %v, want nil", err)
	}
	recovery, err := filestore.Write(t.Context(), filestore.WriteRequest{
		Location: releaseFixtureLocation(t, target), Temporary: stage, Source: file,
		Mode: 0o700, Install: filestore.InstallCreate,
	})
	if err != nil {
		t.Fatalf("compiler fixture write = (%v, %v), want nil error", recovery, err)
	}
}
