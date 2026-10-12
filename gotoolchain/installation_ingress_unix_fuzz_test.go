//go:build linux || darwin

package gotoolchain

import (
	"encoding/json/v2"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func FuzzInstallationDirectoryNativeJSONSemanticClosure(f *testing.F) {
	directory := f.TempDir()
	bin := filepath.Join(directory, "bin")
	goPath := directory
	seed, err := json.Marshal(installationEnvironmentWire{GOBIN: &bin, GOPATH: &goPath})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	f.Add([]byte{})
	f.Add([]byte("null"))
	f.Fuzz(func(t *testing.T, data []byte) {
		owned := t.TempDir()
		payload := filepath.Join(owned, "native-output")
		if err := os.WriteFile(payload, data, 0o600); err != nil {
			t.Fatal(err)
		}
		capability, request := scriptedInstallationCapability(t, owned, payload, 0)
		var wire installationEnvironmentWire
		decoded := json.Unmarshal(data, &wire, json.RejectUnknownMembers(true))
		var want core.AbsolutePath
		if decoded == nil && wire.GOBIN != nil && wire.GOPATH != nil {
			value := *wire.GOBIN
			if value == "" {
				first, _, _ := strings.Cut(*wire.GOPATH, string(os.PathListSeparator))
				if first != "" {
					value = filepath.Join(first, "bin")
				}
			}
			want, _ = core.ParseAbsolutePath(value)
		}
		got, err := capability.ObserveInstallationDirectory(t.Context(), request)
		if want == (core.AbsolutePath{}) {
			if got != (core.AbsolutePath{}) || err == nil {
				t.Fatalf("refused source installation=%v/%v, want zero refusal", got, err)
			}
			if !errors.Is(err, core.ErrGoToolchainOutput) {
				t.Fatalf("installation refusal=%v, want typed JSON/path boundary", err)
			}
			return
		}
		if err != nil || got != want || got.Validate() != nil {
			t.Fatalf("native installation=%v/%v, want independently decoded %v", got, err, want)
		}
	})
}

func FuzzModuleDownloadNativeJSONBindingSemanticClosure(f *testing.F) {
	directory := f.TempDir()
	modulePath, err := gomodule.ParsePath("example.com/native")
	if err != nil {
		f.Fatal(err)
	}
	version, err := ParseModuleVersion("v1.2.3")
	if err != nil {
		f.Fatal(err)
	}
	seed, err := json.Marshal(moduleDownloadWire{Path: modulePath.String(), Version: version.String(), Dir: directory})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	f.Add([]byte{})
	f.Add([]byte("null"))
	f.Fuzz(func(t *testing.T, data []byte) {
		owned := t.TempDir()
		payload := filepath.Join(owned, "native-output")
		if err := os.WriteFile(payload, data, 0o600); err != nil {
			t.Fatal(err)
		}
		capability, scope := scriptedInstallationCapability(t, owned, payload, 0)
		request := ModuleDownloadRequest{WorkingDirectory: scope.WorkingDirectory, Module: modulePath, Version: version}
		if err := request.Validate(); err != nil {
			t.Fatal(err)
		}
		var wire moduleDownloadWire
		decoded := json.Unmarshal(data, &wire, json.RejectUnknownMembers(true))
		want, pathErr := core.ParseAbsolutePath(wire.Dir)
		admitted := decoded == nil && pathErr == nil && wire.Error == "" && wire.Path == modulePath.String() && wire.Version == version.String()
		got, err := capability.DownloadModule(t.Context(), request)
		if !admitted {
			if got != (ModuleDownloadObservation{}) || !errors.Is(err, core.ErrGoToolchainOutput) {
				t.Fatalf("refused module download=%+v/%v, want zero typed output refusal", got, err)
			}
			return
		}
		if err != nil || got.Validate() != nil || got.Directory != want || got.Process.StdoutBytes.Uint64() != uint64(len(data)) {
			t.Fatalf("native module download=%+v/%v, want bound directory %v and exact bytes=%d", got, err, want, len(data))
		}
	})
}
