//go:build linux || darwin

package gotoolchain

import (
	"encoding/json/v2"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallationNativeProcessSourceLayerTriad(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name    string
		wide    bool
		refusal bool
		exit    bool
	}{
		{name: "completed scalar source"},
		{name: "completed source beyond former probe ceiling", wide: true},
		{name: "null field cannot become installation", refusal: true},
		{name: "native exit cannot become installation", exit: true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			bin := filepath.Join(directory, "selected")
			goPath := filepath.Join(directory, "cache")
			wire := installationEnvironmentWire{GOBIN: &bin, GOPATH: &goPath}
			if row.refusal {
				wire.GOBIN = nil
			}
			payload := filepath.Join(directory, "native-output")
			file, err := os.Create(payload)
			if err != nil {
				t.Fatal(err)
			}
			if row.wide {
				if _, err := io.CopyN(file, zeroWhitespaceSource{}, 2<<20+1); err != nil {
					t.Fatal(err)
				}
			}
			if err := json.MarshalWrite(file, wire); err != nil {
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			exitCode := 0
			if row.exit {
				exitCode = 7
			}
			capability, request := scriptedInstallationCapability(t, directory, payload, exitCode)
			got, err := capability.ObserveInstallationDirectory(t.Context(), request)
			if row.refusal || row.exit {
				want := core.ErrGoToolchainOutput
				if row.exit {
					want = core.ErrGoToolchainExecution
				}
				if got != (core.AbsolutePath{}) || !errors.Is(err, want) {
					t.Fatalf("installation=%v/%v, want zero and %v", got, err, want)
				}
			} else if err != nil || got.String() != bin {
				t.Fatalf("installation=%v/%v, want exact %q across native source extent", got, err, bin)
			}
		})
	}
}

type zeroWhitespaceSource struct{}

func (zeroWhitespaceSource) Read(data []byte) (int, error) {
	for i := range data {
		data[i] = ' '
	}
	return len(data), nil
}

func scriptedInstallationCapability(t *testing.T, directory, payload string, exitCode int) (Capability, ObservationRequest) {
	t.Helper()
	executable := filepath.Join(directory, "go-producer")
	script := "#!/bin/sh\ncat '" + payload + "'\n"
	if exitCode != 0 {
		script += "exit 7\n"
	}
	if err := os.WriteFile(executable, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	command, err := core.ParseAbsolutePath(executable)
	if err != nil {
		t.Fatal(err)
	}
	capability, request := installationFixtureCapability(t, directory, nil)
	capability.command = command
	maximum, err := core.NewByteCount(1)
	if err != nil {
		t.Fatal(err)
	}
	capability.configuration.Limits.OutputBytes = maximum
	return capability, request
}

func TestModuleDownloadBindsNativeDirectoryToRequestedCoordinates(t *testing.T) {
	t.Parallel()
	for _, wrongVersion := range []bool{false, true} {
		name := "foreign_module"
		if wrongVersion {
			name = "foreign_version"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			modulePath, err := gomodule.ParsePath("example.com/native")
			if err != nil {
				t.Fatal(err)
			}
			version, err := ParseModuleVersion("v1.2.3")
			if err != nil {
				t.Fatal(err)
			}
			wire := moduleDownloadWire{Path: "example.com/foreign", Version: version.String(), Dir: directory}
			if wrongVersion {
				wire.Path = modulePath.String()
				wire.Version = "v1.2.4"
			}
			payload := filepath.Join(directory, "native-output")
			file, err := os.Create(payload)
			if err != nil {
				t.Fatal(err)
			}
			if err := errors.Join(json.MarshalWrite(file, wire), file.Close()); err != nil {
				t.Fatal(err)
			}
			capability, scope := scriptedInstallationCapability(t, directory, payload, 0)
			got, err := capability.DownloadModule(t.Context(), ModuleDownloadRequest{WorkingDirectory: scope.WorkingDirectory, Module: modulePath, Version: version})
			if got != (ModuleDownloadObservation{}) || !errors.Is(err, core.ErrGoToolchainOutput) {
				t.Fatalf("foreign coordinate download=%+v/%v, want zero typed refusal", got, err)
			}
		})
	}
}

func TestGoCommandNativeSourceRefusalPreservesPermission(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	command := filepath.Join(directory, "not-executable")
	if err := os.WriteFile(command, []byte("#!/bin/sh\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	capability, request := installationFixtureCapability(t, directory, nil)
	capability.command, _ = core.ParseAbsolutePath(command)
	got, err := capability.ObserveInstallationDirectory(t.Context(), request)
	if got != (core.AbsolutePath{}) || !errors.Is(err, os.ErrPermission) || !errors.Is(err, core.ErrGoToolchainExecution) {
		t.Fatalf("native permission refusal=%v/%v, want zero and preserved native cause", got, err)
	}
}

var _ io.Reader = zeroWhitespaceSource{}
