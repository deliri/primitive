package gotoolchain_test

import (
	"bytes"
	"crypto/sha256"
	"debug/buildinfo"
	"errors"
	"os"
	"path/filepath"
	"runtime/debug"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/gotoolchain"
)

func FuzzExecutableMetadataConservesNativeFacts(f *testing.F) {
	seed, err := os.ReadFile(metadataArtifactPath(f, "PRIMITIVE_METADATA_REPLACED"))
	if err != nil {
		f.Fatalf("ReadFile(native compiler seed) error = %v, want nil", err)
	}
	if _, err := buildinfo.Read(bytes.NewReader(seed)); err != nil {
		f.Fatalf("buildinfo.Read(native compiler seed) error = %v, want nil", err)
	}
	f.Add(seed)
	f.Add([]byte{})
	f.Add([]byte("not an executable"))
	f.Fuzz(func(t *testing.T, data []byte) {
		directory := t.TempDir()
		path := filepath.Join(directory, "artifact")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatalf("WriteFile(metadata fuzz input) error = %v, want nil", err)
		}
		file, err := os.Open(path)
		if err != nil {
			t.Fatalf("Open(metadata fuzz input) error = %v, want nil", err)
		}
		defer func() {
			if err := file.Close(); err != nil {
				t.Errorf("Close(metadata fuzz input) error = %v, want nil", err)
			}
		}()
		oracle, nativeErr := buildinfo.Read(bytes.NewReader(data))
		headers, dependencies, settings := 0, 0, 0
		request := metadataBorrow(file)
		request.Header = func(header gotoolchain.ExecutableMetadataHeader) error {
			headers++
			if nativeErr != nil || header.GoVersion != oracle.GoVersion || header.Path != oracle.Path || !sameNativeModule(header.Main, oracle.Main) {
				t.Errorf("fuzz header = %+v/native error %v, want exact native facts", header, nativeErr)
			}
			return nil
		}
		request.Dependency = func(module debug.Module) error {
			if nativeErr != nil || dependencies >= len(oracle.Deps) || !sameNativeModule(module, *oracle.Deps[dependencies]) {
				t.Errorf("fuzz dependency at %d = %+v, want exact native occurrence", dependencies, module)
			}
			dependencies++
			return nil
		}
		request.Setting = func(setting debug.BuildSetting) error {
			if nativeErr != nil || settings >= len(oracle.Settings) || setting != oracle.Settings[settings] {
				t.Errorf("fuzz setting at %d = %+v, want exact native occurrence", settings, setting)
			}
			settings++
			return nil
		}
		got, err := gotoolchain.ObserveExecutableMetadata(t.Context(), request)
		if nativeErr != nil {
			if !errors.Is(err, core.ErrGoToolchainOutput) || got != (filestore.ContentIndexEntry{}) || headers != 0 || dependencies != 0 || settings != 0 {
				t.Fatalf("fuzz refusal = %+v/%v/visits %d/%d/%d, want zero/typed refusal/no facts", got, err, headers, dependencies, settings)
			}
			return
		}
		if err != nil || got.Validate() != nil || got.Extent.Uint64() != uint64(len(data)) || got.Digest != core.NewSHA256Digest(sha256.Sum256(data)) || headers != 1 || dependencies != len(oracle.Deps) || settings != len(oracle.Settings) {
			t.Fatalf("fuzz observation = %+v/%v/visits %d/%d/%d, want complete exact native facts and content", got, err, headers, dependencies, settings)
		}
	})
}
