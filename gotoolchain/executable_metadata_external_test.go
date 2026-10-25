package gotoolchain_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"debug/buildinfo"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/gotoolchain"
)

func metadataArtifactPath(t testing.TB, name string) string {
	t.Helper()
	path, present := os.LookupEnv(name)
	if !present || path == "" {
		t.Fatalf("metadata fixture %s = %q/present %v, want an explicitly retained native Go artifact", name, path, present)
	}
	return path
}

func metadataArtifactBytes(t testing.TB) []byte {
	t.Helper()
	data, err := os.ReadFile(metadataArtifactPath(t, "PRIMITIVE_METADATA_LINUX"))
	if err != nil {
		t.Fatalf("ReadFile(metadata fixture) error = %v, want nil", err)
	}
	return data
}

func metadataBorrow(file *os.File) gotoolchain.ExecutableMetadataRequest {
	return gotoolchain.ExecutableMetadataRequest{
		File: file, Header: func(gotoolchain.ExecutableMetadataHeader) error { return nil },
		Dependency: func(debug.Module) error { return nil }, Setting: func(debug.BuildSetting) error { return nil },
	}
}

func sameNativeModule(left, right debug.Module) bool {
	for {
		if left.Path != right.Path || left.Version != right.Version || left.Sum != right.Sum || (left.Replace == nil) != (right.Replace == nil) {
			return false
		}
		if left.Replace == nil {
			return true
		}
		left, right = *left.Replace, *right.Replace
	}
}

func TestExecutableMetadataNativeLayerTriad(t *testing.T) {
	t.Parallel()
	for _, fixture := range []struct{ name, variable, goos, goarch string }{
		{name: "Linux AMD64 native metadata", variable: "PRIMITIVE_METADATA_LINUX", goos: "linux", goarch: "amd64"},
		{name: "Darwin ARM64 native metadata", variable: "PRIMITIVE_METADATA_DARWIN", goos: "darwin", goarch: "arm64"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			file, err := os.Open(metadataArtifactPath(t, fixture.variable))
			if err != nil {
				t.Fatalf("Open(native artifact) error = %v, want nil", err)
			}
			defer func() {
				if err := file.Close(); err != nil {
					t.Errorf("Close(native artifact) error = %v, want nil", err)
				}
			}()
			metadataNativeAgreement(t, file, fixture.goos, fixture.goarch)
		})
	}

	t.Run("local replacement remains an observed fact", func(t *testing.T) {
		t.Parallel()
		file, err := os.Open(metadataArtifactPath(t, "PRIMITIVE_METADATA_REPLACED"))
		if err != nil {
			t.Fatalf("Open(replaced artifact) error = %v, want nil", err)
		}
		defer func() {
			if err := file.Close(); err != nil {
				t.Errorf("Close(replaced artifact) error = %v, want nil", err)
			}
		}()
		var replaced bool
		request := metadataBorrow(file)
		request.Dependency = func(module debug.Module) error {
			if module.Replace != nil {
				replaced = true
			}
			return nil
		}
		got, err := gotoolchain.ObserveExecutableMetadata(t.Context(), request)
		if err != nil || !replaced || got.Validate() != nil {
			t.Fatalf("ObserveExecutableMetadata(replacement) = %+v/%v/replaced %v, want valid content/nil/true", got, err, replaced)
		}
	})
}

func metadataNativeAgreement(t testing.TB, file *os.File, goos, goarch string) {
	t.Helper()
	oracle, err := buildinfo.Read(file)
	if err != nil {
		t.Fatalf("native buildinfo.Read() error = %v, want nil", err)
	}
	var headers, dependencies, settings, sums int
	var nativeOS, nativeArchitecture string
	request := metadataBorrow(file)
	request.Header = func(header gotoolchain.ExecutableMetadataHeader) error {
		headers++
		if header.GoVersion != oracle.GoVersion || header.GoVersion != "go1.27.3" || header.Path != oracle.Path || !sameNativeModule(header.Main, oracle.Main) {
			t.Errorf("header = %+v, want exact native %+v and go1.27.3", header, oracle)
		}
		return nil
	}
	request.Dependency = func(module debug.Module) error {
		if dependencies >= len(oracle.Deps) || !sameNativeModule(module, *oracle.Deps[dependencies]) {
			t.Errorf("dependency at %d = %+v, want exact native occurrence", dependencies, module)
		}
		dependencies++
		if module.Sum != "" {
			sums++
		}
		return nil
	}
	request.Setting = func(setting debug.BuildSetting) error {
		if settings >= len(oracle.Settings) || setting != oracle.Settings[settings] {
			t.Errorf("setting at %d = %+v, want exact native occurrence", settings, setting)
		}
		settings++
		switch setting.Key {
		case "GOOS":
			nativeOS = setting.Value
		case "GOARCH":
			nativeArchitecture = setting.Value
		}
		return nil
	}
	got, err := gotoolchain.ObserveExecutableMetadata(t.Context(), request)
	if err != nil || got.Validate() != nil || headers != 1 || dependencies != len(oracle.Deps) || settings != len(oracle.Settings) || sums == 0 || nativeOS != goos || nativeArchitecture != goarch {
		t.Fatalf("metadata = %+v/%v/header %d/dependencies %d/settings %d/sums %d/platform %s/%s, want exact nonempty native facts/%s/%s", got, err, headers, dependencies, settings, sums, nativeOS, nativeArchitecture, goos, goarch)
	}
	info, err := file.Stat()
	if err != nil {
		t.Fatalf("Stat(native artifact after observation) error = %v, want nil", err)
	}
	digest := sha256.New()
	count, err := io.Copy(digest, io.NewSectionReader(file, 0, info.Size()))
	var sum [sha256.Size]byte
	copy(sum[:], digest.Sum(nil))
	if err != nil || count != info.Size() || got.Extent.Uint64() != uint64(count) || got.Digest != core.NewSHA256Digest(sum) {
		t.Fatalf("content = %+v/count %d/error %v, want exact native SHA256 and extent %d", got, count, err, info.Size())
	}
}

func TestExecutableMetadataRefusalLayerTriad(t *testing.T) {
	t.Parallel()
	data := metadataArtifactBytes(t)
	magic := bytes.Index(data, []byte("\xff Go buildinf:"))
	if magic < 0 {
		t.Fatalf("native build-info offset = %d, want present", magic)
	}
	for _, fixture := range []struct {
		name string
		data []byte
	}{
		{name: "empty file has no metadata"},
		{name: "non executable data has no metadata", data: []byte("ordinary text")},
		{name: "native header ends before metadata", data: data[:magic+16]},
		{name: "native metadata magic is corrupted", data: append(append(append([]byte(nil), data[:magic]...), 'x'), data[magic+1:]...)},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			path := filepath.Join(directory, "artifact")
			if err := os.WriteFile(path, fixture.data, 0o600); err != nil {
				t.Fatalf("WriteFile(refusal fixture) error = %v, want nil", err)
			}
			file, err := os.Open(path)
			if err != nil {
				t.Fatalf("Open(refusal fixture) error = %v, want nil", err)
			}
			defer func() {
				if err := file.Close(); err != nil {
					t.Errorf("Close(refusal fixture) error = %v, want nil", err)
				}
			}()
			calls := 0
			request := metadataBorrow(file)
			request.Header = func(gotoolchain.ExecutableMetadataHeader) error { calls++; return nil }
			got, err := gotoolchain.ObserveExecutableMetadata(t.Context(), request)
			_, nativeErr := buildinfo.Read(file)
			if !errors.Is(err, core.ErrGoToolchainOutput) || nativeErr == nil || got != (filestore.ContentIndexEntry{}) || calls != 0 {
				t.Fatalf("refused metadata = %+v/%v/calls %d/native %v, want zero/typed output/zero/native refusal", got, err, calls, nativeErr)
			}
		})
	}
}

func TestExecutableMetadataOwnsOnlyBorrowedReads(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	data := metadataArtifactBytes(t)
	path := filepath.Join(directory, "artifact")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile(borrowed artifact) error = %v, want nil", err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open(borrowed artifact) error = %v, want nil", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("Close(borrowed artifact) error = %v, want nil", err)
		}
	}()
	if _, err := file.Seek(17, io.SeekStart); err != nil {
		t.Fatalf("Seek(borrowed artifact) error = %v, want nil", err)
	}
	if err := os.Rename(path, filepath.Join(directory, "original")); err != nil {
		t.Fatalf("Rename(borrowed artifact) error = %v, want nil", err)
	}
	if err := os.WriteFile(path, []byte("replacement path is not the held file"), 0o600); err != nil {
		t.Fatalf("WriteFile(replacement path) error = %v, want nil", err)
	}
	got, err := gotoolchain.ObserveExecutableMetadata(t.Context(), metadataBorrow(file))
	offset, offsetErr := file.Seek(0, io.SeekCurrent)
	var first [16]byte
	count, readErr := file.ReadAt(first[:], 0)
	if err != nil || got.Digest != core.NewSHA256Digest(sha256.Sum256(data)) || got.Extent.Uint64() != uint64(len(data)) || offsetErr != nil || offset != 17 || readErr != nil || count != len(first) || !bytes.Equal(first[:], data[:len(first)]) {
		t.Fatalf("borrowed observation = %+v/%v/offset %d/%v/read %d/%v, want original content/unchanged offset/open descriptor", got, err, offset, offsetErr, count, readErr)
	}
}

func TestExecutableMetadataRefusesMissingAndNativeInputs(t *testing.T) {
	t.Parallel()
	for _, fixture := range []struct {
		name string
		kind uint8
		want error
	}{
		{name: "nil descriptor", kind: 0, want: core.ErrGoToolchainContract},
		{name: "nil header visitor", kind: 1, want: core.ErrGoToolchainContract},
		{name: "nil dependency visitor", kind: 2, want: core.ErrGoToolchainContract},
		{name: "nil setting visitor", kind: 3, want: core.ErrGoToolchainContract},
		{name: "closed descriptor preserves native identity", kind: 4, want: os.ErrClosed},
		{name: "directory is not a regular executable", kind: 5, want: core.ErrGoToolchainOutput},
		{name: "write only descriptor preserves native read cause", kind: 6, want: core.ErrGoToolchainOutput},
		{name: "nil context refuses before visitors", kind: 7, want: core.ErrNilContext},
		{name: "cancelled context refuses before visitors", kind: 8, want: context.Canceled},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			path := filepath.Join(directory, "artifact")
			if err := os.WriteFile(path, []byte("source must not be interpreted after invalid admission"), 0o600); err != nil {
				t.Fatalf("WriteFile(admission fixture) error = %v, want nil", err)
			}
			if fixture.kind == 5 {
				path = directory
			}
			mode := os.O_RDONLY
			if fixture.kind == 6 {
				mode = os.O_WRONLY
			}
			file, err := os.OpenFile(path, mode, 0)
			if err != nil {
				t.Fatalf("OpenFile(admission fixture) error = %v, want nil", err)
			}
			if fixture.kind == 4 {
				if err := file.Close(); err != nil {
					t.Fatalf("Close(closed fixture) error = %v, want nil", err)
				}
			} else {
				defer func() {
					if err := file.Close(); err != nil {
						t.Errorf("Close(admission fixture) error = %v, want nil", err)
					}
				}()
			}
			request := metadataBorrow(file)
			calls := 0
			request.Header = func(gotoolchain.ExecutableMetadataHeader) error { calls++; return nil }
			ctx := t.Context()
			var nativeReadErr error
			switch fixture.kind {
			case 0:
				request.File = nil
			case 1:
				request.Header = nil
			case 2:
				request.Dependency = nil
			case 3:
				request.Setting = nil
			case 6:
				var one [1]byte
				_, nativeReadErr = file.ReadAt(one[:], 0)
			case 7:
				ctx = nil
			case 8:
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			got, err := gotoolchain.ObserveExecutableMetadata(ctx, request)
			if !errors.Is(err, fixture.want) || got != (filestore.ContentIndexEntry{}) || calls != 0 {
				t.Fatalf("admission = %+v/%v/visits %d, want zero/%v/zero", got, err, calls, fixture.want)
			}
			if fixture.kind == 6 {
				var nativePath *os.PathError
				if !errors.As(nativeReadErr, &nativePath) || !errors.Is(err, nativePath.Err) {
					t.Fatalf("write-only observation error = %v/native %v, want retained native read cause", err, nativeReadErr)
				}
			}
		})
	}
}

func TestExecutableMetadataVisitorFailuresWithholdContent(t *testing.T) {
	t.Parallel()
	for _, fixture := range []struct {
		name string
		kind uint8
		want error
	}{
		{name: "header consumer refusal retains cause", kind: 0, want: io.ErrClosedPipe},
		{name: "dependency consumer refusal retains cause", kind: 1, want: io.ErrClosedPipe},
		{name: "last setting consumer refusal retains cause", kind: 2, want: io.ErrClosedPipe},
		{name: "last setting cancellation retains cause", kind: 3, want: context.Canceled},
		{name: "source shrinks during last setting", kind: 4, want: core.ErrGoToolchainOutput},
		{name: "source grows during last setting", kind: 5, want: core.ErrGoToolchainOutput},
		{name: "last setting cancellation and refusal retain both causes", kind: 6, want: io.ErrClosedPipe},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			path := filepath.Join(directory, "artifact")
			data := metadataArtifactBytes(t)
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatalf("WriteFile(visitor fixture) error = %v, want nil", err)
			}
			file, err := os.OpenFile(path, os.O_RDWR, 0)
			if err != nil {
				t.Fatalf("OpenFile(visitor fixture) error = %v, want nil", err)
			}
			defer func() {
				if err := file.Close(); err != nil {
					t.Errorf("Close(visitor fixture) error = %v, want nil", err)
				}
			}()
			oracle, err := buildinfo.Read(file)
			if err != nil || len(oracle.Deps) == 0 || len(oracle.Settings) == 0 {
				t.Fatalf("native visitor fixture = %+v/%v, want dependency and setting facts", oracle, err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			request := metadataBorrow(file)
			headers, dependencies, settings := 0, 0, 0
			request.Header = func(gotoolchain.ExecutableMetadataHeader) error {
				headers++
				if fixture.kind == 0 {
					return io.ErrClosedPipe
				}
				return nil
			}
			request.Dependency = func(debug.Module) error {
				dependencies++
				if fixture.kind == 1 {
					return io.ErrClosedPipe
				}
				return nil
			}
			request.Setting = func(debug.BuildSetting) error {
				settings++
				if settings != len(oracle.Settings) {
					return nil
				}
				switch fixture.kind {
				case 2:
					return io.ErrClosedPipe
				case 3:
					cancel()
				case 4:
					return file.Truncate(0)
				case 5:
					_, err := file.WriteAt([]byte{1}, int64(len(data)))
					return err
				case 6:
					cancel()
					return io.ErrClosedPipe
				}
				return nil
			}
			got, err := gotoolchain.ObserveExecutableMetadata(ctx, request)
			wantDependencies, wantSettings := len(oracle.Deps), len(oracle.Settings)
			switch fixture.kind {
			case 0:
				wantDependencies, wantSettings = 0, 0
			case 1:
				wantDependencies, wantSettings = 1, 0
			}
			if !errors.Is(err, fixture.want) || got != (filestore.ContentIndexEntry{}) || headers != 1 || dependencies != wantDependencies || settings != wantSettings {
				t.Fatalf("visitor refusal = %+v/%v/prefix %d/%d/%d, want zero/%v/prefix 1/%d/%d", got, err, headers, dependencies, settings, fixture.want, wantDependencies, wantSettings)
			}
			if fixture.kind == 6 && !errors.Is(err, context.Canceled) {
				t.Fatalf("combined visitor error = %v, want both %v and %v", err, io.ErrClosedPipe, context.Canceled)
			}
		})
	}
}

func TestExecutableMetadataValidTamperingIsNotAuthentication(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	data := metadataArtifactBytes(t)
	original, err := buildinfo.Read(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("native original metadata error = %v, want nil", err)
	}
	var revision string
	for _, setting := range original.Settings {
		if setting.Key == "vcs.revision" {
			revision = setting.Value
		}
	}
	magic := bytes.Index(data, []byte("\xff Go buildinf:"))
	if revision == "" || magic < 0 {
		t.Fatalf("revision/magic = %q/%d, want genuine retained VCS metadata", revision, magic)
	}
	position := bytes.Index(data[magic:], []byte(revision))
	if position < 0 {
		t.Fatalf("native revision byte position = %d, want present", position)
	}
	originalDigest := sha256.Sum256(data)
	position += magic
	if data[position] == '0' {
		data[position] = '1'
	} else {
		data[position] = '0'
	}
	path := filepath.Join(directory, "tampered")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile(tampered artifact) error = %v, want nil", err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open(tampered artifact) error = %v, want nil", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Errorf("Close(tampered artifact) error = %v, want nil", err)
		}
	}()
	var observed string
	request := metadataBorrow(file)
	request.Setting = func(setting debug.BuildSetting) error {
		if setting.Key == "vcs.revision" {
			observed = setting.Value
		}
		return nil
	}
	got, err := gotoolchain.ObserveExecutableMetadata(t.Context(), request)
	if err != nil || observed == "" || observed == revision || got.Digest == core.NewSHA256Digest(originalDigest) || got.Digest != core.NewSHA256Digest(sha256.Sum256(data)) {
		t.Fatalf("tampered observation = %+v/%v/revision %q, want changed native fact and changed content, never original proof", got, err, observed)
	}
}
