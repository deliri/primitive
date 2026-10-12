package gotoolchain

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"golang.org/x/mod/semver"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
	"github.com/deliri/primitive/v2026/process"
)

func installationFixtureCapability(t *testing.T, directory string, extra []string) (Capability, ObservationRequest) {
	t.Helper()
	root, err := core.ParseAbsolutePath(directory)
	if err != nil {
		t.Fatal(err)
	}
	environment := append(os.Environ(), extra...)
	exact, err := process.ParseEffectiveEnvironment(environment)
	if err != nil {
		t.Fatal(err)
	}
	limits, err := DefaultLimits()
	if err != nil {
		t.Fatal(err)
	}
	capability, err := Open(t.Context(), Configuration{Workspace: WorkspaceModeDisabled, Limits: limits, Environment: &exact})
	if err != nil {
		t.Fatal(err)
	}
	return capability, ObservationRequest{WorkingDirectory: root}
}

func TestInstallationDirectoryMatchesNativeGoSelectionLayerTriad(t *testing.T) {
	t.Parallel()
	for _, explicit := range []bool{false, true} {
		t.Run(installationCaseName(explicit), func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			goPath := filepath.Join(directory, "first") + string(os.PathListSeparator) + filepath.Join(directory, "second")
			bin := ""
			want := filepath.Join(directory, "first", "bin")
			if explicit {
				bin = filepath.Join(directory, "selected-bin")
				want = bin
			}
			capability, request := installationFixtureCapability(t, directory, []string{"GOBIN=" + bin, "GOPATH=" + goPath})
			got, err := capability.ObserveInstallationDirectory(t.Context(), request)
			if err != nil || got.String() != want {
				t.Fatalf("installation=%v/%v, want %q", got, err, want)
			}
			native := exec.CommandContext(t.Context(), capability.command.String(), "env", "GOBIN", "GOPATH")
			native.Dir = directory
			native.Env, _ = capability.environment.Strings()
			output, err := native.Output()
			if err != nil {
				t.Fatal(err)
			}
			fields := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
			if len(fields) != 2 || fields[0] != want || fields[1] != goPath {
				t.Fatalf("native environment=%q, want selected storage", output)
			}
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			if refused, err := capability.ObserveInstallationDirectory(ctx, request); refused != (core.AbsolutePath{}) || !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled installation=%v/%v, want zero typed refusal", refused, err)
			}
		})
	}
}
func installationCaseName(value bool) string {
	if value {
		return "explicit_bin"
	}
	return "first_gopath_fallback"
}

func TestGoModuleDownloadUsesRealCacheAndWithholdsMissingVersion(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	// Download an already present module through the real Go command. Network is
	// disabled, so a missing version can only return its actual native refusal.
	cache, err := exec.CommandContext(t.Context(), "go", "env", "GOMODCACHE").Output()
	if err != nil {
		t.Fatal(err)
	}
	capability, scope := installationFixtureCapability(t, directory, []string{"GOPROXY=off", "GOSUMDB=off", "GOMODCACHE=" + strings.TrimSpace(string(cache))})
	modulePath, err := gomodule.ParsePath("golang.org/x/mod")
	if err != nil {
		t.Fatal(err)
	}
	// This exact dependency is selected by Primitive's own go.mod.
	dependency, err := exec.CommandContext(t.Context(), "go", "list", "-m", "-f={{.Version}}", "golang.org/x/mod").Output()
	if err != nil {
		t.Fatal(err)
	}
	version, err := ParseModuleVersion(strings.TrimSpace(string(dependency)))
	if err != nil {
		t.Fatal(err)
	}
	request := ModuleDownloadRequest{WorkingDirectory: scope.WorkingDirectory, Module: modulePath, Version: version}
	got, err := capability.DownloadModule(t.Context(), request)
	if err != nil || got.Validate() != nil {
		t.Fatalf("download=%+v/%v, want completed native cache observation", got, err)
	}
	command := exec.CommandContext(t.Context(), capability.command.String(), "mod", "download", "-json", modulePath.String()+"@"+version.String())
	command.Dir = directory
	command.Env, _ = capability.environment.Strings()
	var reference bytes.Buffer
	command.Stdout = &reference
	if err := command.Run(); err != nil {
		t.Fatal(err)
	}
	var nativeDirectory struct{ Dir string }
	if err := json.Unmarshal(reference.Bytes(), &nativeDirectory); err != nil {
		t.Fatal(err)
	}
	if got.Process.StdoutBytes.Uint64() != uint64(reference.Len()) || nativeDirectory.Dir != got.Directory.String() {
		t.Fatalf("download receipt=%+v, want native output extent=%d and exact cache path", got, reference.Len())
	}
	absent, err := ParseModuleVersion("v99999.0.0")
	if err != nil {
		t.Fatal(err)
	}
	request.Version = absent
	refused, err := capability.DownloadModule(t.Context(), request)
	// A module-path major mismatch is refused before execution; a compatible
	// missing minor version below exercises the actual offline native failure.
	if refused != (ModuleDownloadObservation{}) || !errors.Is(err, core.ErrGoToolchainContract) {
		t.Fatalf("major mismatch=%+v/%v, want zero typed contract refusal", refused, err)
	}
	absent, err = ParseModuleVersion("v0.99999.0")
	if err != nil {
		t.Fatal(err)
	}
	request.Version = absent
	refused, err = capability.DownloadModule(t.Context(), request)
	var exit CommandExitError
	if refused != (ModuleDownloadObservation{}) || !errors.Is(err, core.ErrGoToolchainExecution) || !errors.As(err, &exit) || exit.Process.ExitCode == 0 || exit.Process.Validate() != nil {
		t.Fatalf("missing version=%+v/%v, want zero and native typed exit refusal", refused, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	request.Version = version
	if refused, err := capability.DownloadModule(ctx, request); refused != (ModuleDownloadObservation{}) || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled download=%+v/%v, want zero typed refusal", refused, err)
	}
}

func TestModuleVersionCanonicalAdmissionLayerTriad(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, value string
		wantErr     error
	}{
		{name: "canonical tagged version", value: "v1.2.3"},
		{name: "canonical zero major", value: "v0.0.0"},
		{name: "canonical prerelease", value: "v1.2.3-beta.1"},
		{name: "canonical pseudo version", value: "v0.0.0-20260101000000-0123456789ab"},
		{name: "canonical incompatible marker", value: "v2.0.0+incompatible"},
		{name: "absent version", wantErr: core.ErrGoToolchainContract},
		{name: "noncanonical minor abbreviation", value: "v1.2", wantErr: core.ErrGoToolchainContract},
		{name: "noncanonical build annotation", value: "v1.2.3+private", wantErr: core.ErrGoToolchainContract},
		{name: "query cannot masquerade as exact version", value: "latest", wantErr: core.ErrGoToolchainContract},
		{name: "trailing command text", value: "v1.2.3 --help", wantErr: core.ErrGoToolchainContract},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseModuleVersion(row.value)
			if !errors.Is(err, row.wantErr) {
				t.Fatalf("version=%q error=%v, want %v", row.value, err, row.wantErr)
			}
			if row.wantErr != nil {
				if got != (ModuleVersion{}) {
					t.Fatalf("refused version=%v, want zero", got)
				}
			} else if got.String() != row.value || got.Validate() != nil {
				t.Fatalf("version=%q, want canonical %q", got.String(), row.value)
			}
		})
	}
}

func FuzzModuleVersionCanonicalNativeSemanticClosure(f *testing.F) {
	for _, text := range []string{"v0.0.0", "v1.2.3", "v1.2.3-beta.1", "v0.0.0-20260101000000-0123456789ab", "v2.0.0+incompatible"} {
		value, err := ParseModuleVersion(text)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(value.String())
	}
	f.Add("")
	f.Add("v1.2")
	f.Add("latest")
	f.Fuzz(func(t *testing.T, text string) {
		canonical := semver.Canonical(text)
		if canonical != "" && strings.HasSuffix(text, "+incompatible") {
			canonical += "+incompatible"
		}
		admitted := canonical != "" && canonical == text
		got, err := ParseModuleVersion(text)
		if !admitted {
			if !errors.Is(err, core.ErrGoToolchainContract) || got != (ModuleVersion{}) {
				t.Fatalf("refused version=%q/%v, want zero typed refusal", got.String(), err)
			}
			return
		}
		if err != nil || got.Validate() != nil || got.String() != text || semver.Compare(got.String(), canonical) != 0 {
			t.Fatalf("canonical version=%q/%v, want exact native %q", got.String(), err, canonical)
		}
		again, err := ParseModuleVersion(got.String())
		if err != nil || again != got {
			t.Fatalf("second admission=%v/%v, want %v", again, err, got)
		}
	})
}
