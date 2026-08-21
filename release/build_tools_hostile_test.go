package release_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/release"
	"github.com/deliri/primitive/v2026/temporal"
)

// TestVerifyBuildToolsRejectsEveryUnusableVerificationBoundary pressures the
// pure request boundary before any file or process work is admitted.
func TestVerifyBuildToolsRejectsEveryUnusableVerificationBoundary(t *testing.T) {
	t.Parallel()

	valid := buildToolVerificationRequestForLiveTest(t)
	cases := []struct {
		wantErr error
		mutate  func(*release.BuildToolVerificationRequest)
		name    string
	}{
		{
			name:    "zero go executable is rejected",
			mutate:  func(r *release.BuildToolVerificationRequest) { r.GoExecutable = core.AbsolutePath{} },
			wantErr: core.ErrReleaseContract,
		},
		{
			name:    "zero working directory is rejected",
			mutate:  func(r *release.BuildToolVerificationRequest) { r.WorkingDirectory = core.AbsolutePath{} },
			wantErr: core.ErrReleaseContract,
		},
		{
			name:    "zero wait delay leaves the probe unbounded",
			mutate:  func(r *release.BuildToolVerificationRequest) { r.WaitDelay = temporal.Duration{} },
			wantErr: core.ErrReleaseContract,
		},
		{
			name: "unset environment mode is rejected",
			mutate: func(r *release.BuildToolVerificationRequest) {
				r.HostEnvironment = process.Environment{}
			},
			wantErr: core.ErrReleaseContract,
		},
		{
			name: "ambient inheritance is rejected",
			mutate: func(r *release.BuildToolVerificationRequest) {
				r.HostEnvironment = process.Environment{Mode: process.EnvironmentModeInherit}
			},
			wantErr: core.ErrReleaseContract,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := valid
			tc.mutate(&request)
			got, gotErr := release.VerifyBuildTools(t.Context(), request)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("release.VerifyBuildTools() error = %v, want %v", gotErr, tc.wantErr)
			}
			if got != (release.VerifiedBuildTools{}) {
				t.Fatalf("release.VerifyBuildTools() tools = %v, want zero on rejection", got)
			}
		})
	}
}

// TestVerifyBuildToolsRejectsEveryContextFailure proves the context boundary is
// closed before the request is even parsed.
func TestVerifyBuildToolsRejectsEveryContextFailure(t *testing.T) {
	t.Parallel()

	request := buildToolVerificationRequestForLiveTest(t)

	var absent context.Context
	got, gotErr := release.VerifyBuildTools(absent, request)
	if !errors.Is(gotErr, core.ErrNilContext) {
		t.Fatalf("release.VerifyBuildTools(nil context) error = %v, want %v", gotErr, core.ErrNilContext)
	}
	if got != (release.VerifiedBuildTools{}) {
		t.Fatalf("release.VerifyBuildTools(nil context) tools = %v, want zero", got)
	}

	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	got, gotErr = release.VerifyBuildTools(cancelled, request)
	if !errors.Is(gotErr, context.Canceled) || !errors.Is(gotErr, core.ErrReleaseContract) {
		t.Fatalf("release.VerifyBuildTools(cancelled) error = %v, want cancelled release contract", gotErr)
	}
	if got != (release.VerifiedBuildTools{}) {
		t.Fatalf("release.VerifyBuildTools(cancelled) tools = %v, want zero", got)
	}
}

// TestVerifyBuildToolsRejectsEveryExecutableThatIsNotTheAdmittedTool proves the
// on-disk identity check, not an operator-supplied version string.
func TestVerifyBuildToolsRejectsEveryExecutableThatIsNotTheAdmittedTool(t *testing.T) {
	t.Parallel()

	valid := buildToolVerificationRequestForLiveTest(t)
	directory := t.TempDir()
	empty := writeToolFixture(t, directory, "empty", nil)
	text := writeToolFixture(t, directory, "text", []byte("#!/bin/sh\nexit 0\n"))
	truncated := writeToolFixture(t, directory, "truncated", truncatedToolBytes(t, valid.GoExecutable))
	missing := absoluteToolPath(t, filepath.Join(directory, "absent"))
	folder := absoluteToolPath(t, directory)

	cases := []struct {
		wantErr error
		name    string
		path    core.AbsolutePath
	}{
		{name: "absent go executable is rejected", path: missing, wantErr: core.ErrReleaseContract},
		{name: "directory as go executable is rejected", path: folder, wantErr: core.ErrReleaseContract},
		{name: "empty go executable is rejected", path: empty, wantErr: core.ErrReleaseContract},
		{name: "script instead of go executable is rejected", path: text, wantErr: core.ErrReleaseContract},
		{name: "truncated go executable is rejected", path: truncated, wantErr: core.ErrReleaseContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := valid
			request.GoExecutable = tc.path
			got, gotErr := release.VerifyBuildTools(t.Context(), request)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("release.VerifyBuildTools() error = %v, want %v", gotErr, tc.wantErr)
			}
			if got != (release.VerifiedBuildTools{}) {
				t.Fatalf("release.VerifyBuildTools() tools = %v, want zero on rejection", got)
			}
		})
	}
}

// TestVerifyBuildToolsIsIndependentOfRepeatedObservation proves the digest and
// identity facts are a property of the file, not of one execution.
func TestVerifyBuildToolsIsIndependentOfRepeatedObservation(t *testing.T) {
	t.Parallel()

	request := buildToolVerificationRequestForLiveTest(t)
	first, err := release.VerifyBuildTools(t.Context(), request)
	if err != nil {
		t.Fatalf("release.VerifyBuildTools(first) error = %v, want nil", err)
	}
	second, err := release.VerifyBuildTools(t.Context(), request)
	if err != nil {
		t.Fatalf("release.VerifyBuildTools(second) error = %v, want nil", err)
	}
	if first != second {
		t.Fatalf("release.VerifyBuildTools() second observation differs from the first")
	}
	if first.GoExecutableDigest() != second.GoExecutableDigest() {
		t.Fatalf("go executable digest changed across repeated observation")
	}
}

func writeToolFixture(t *testing.T, directory, name string, content []byte) core.AbsolutePath {
	t.Helper()
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, content, 0o700); err != nil { // #nosec G306 -- executable fixture.
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", name, err)
	}
	return absoluteToolPath(t, path)
}

// truncatedToolBytes returns a real executable prefix. It parses as a native
// image header but cannot carry a complete Go build identity.
func truncatedToolBytes(t *testing.T, source core.AbsolutePath) []byte {
	t.Helper()
	content, err := os.ReadFile(source.String()) // #nosec G304 -- validated test tool path.
	if err != nil {
		t.Fatalf("os.ReadFile(tool) error = %v, want nil", err)
	}
	if len(content) < 4096 {
		t.Fatalf("tool fixture size = %d, want a multi-page executable", len(content))
	}
	return bytes.Clone(content[:4096])
}

func absoluteToolPath(t *testing.T, value string) core.AbsolutePath {
	t.Helper()
	path, err := core.ParseAbsolutePath(value)
	if err != nil {
		t.Fatalf("core.ParseAbsolutePath(%q) error = %v, want nil", value, err)
	}
	return path
}
