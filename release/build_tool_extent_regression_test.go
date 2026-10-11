package release

import (
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/process"
)

func TestBuildToolDigestAdmitsNativeCompilerBeyondFormerCeiling(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	name, err := core.ParsePathComponent("go")
	if err != nil {
		t.Fatal(err)
	}
	sourcePath, err := process.Resolve(t.Context(), name)
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.Open(sourcePath.String())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "native-go")
	destination, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o700)
	if err != nil {
		_ = source.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := destination.Close(); err != nil {
			t.Error(err)
		}
	})
	_, copyErr := io.CopyBuffer(destination, source, make([]byte, 65536))
	if err := errors.Join(copyErr, source.Close()); err != nil {
		t.Fatal(err)
	}
	const size = 128<<20 + 1
	if err := destination.Truncate(size); err != nil {
		t.Fatal(err)
	}
	typedPath, err := core.ParseAbsolutePath(path)
	if err != nil {
		t.Fatal(err)
	}
	got, digest, gotErr := inspectOpenedBuildTool(t.Context(), destination, typedPath)
	oracle := sha256.New()
	count, oracleErr := io.CopyBuffer(oracle, io.NewSectionReader(destination, 0, size), make([]byte, 65536))
	if gotErr != nil || oracleErr != nil || count != size || got == nil || got.Path != goCommandModulePath || digest != core.NewSHA256Digest([32]byte(oracle.Sum(nil))) {
		t.Fatalf("native compiler = (%v,%v,%v), oracle (%d,%v); want exact %d-byte compiler and native SHA-256", got, digest, gotErr, count, oracleErr, size)
	}
}

// Direct unit ratchet for the held-file boundary after Stat. Adjacent captured
// extents model growth and shrinkage between observation and reading without
// relying on scheduling a concurrent writer. The native public verifier is
// separately exercised by TestVerifyBuildToolsProvesTheExactInstalledExecutables.
func TestBuildToolInspectionBindsTheCapturedExtent(t *testing.T) {
	t.Parallel()
	name, err := core.ParsePathComponent("go")
	if err != nil {
		t.Fatalf("core.ParsePathComponent(go) error = %v, want nil", err)
	}
	path, err := process.Resolve(t.Context(), name)
	if err != nil {
		t.Fatalf("process.Resolve(go) error = %v, want nil", err)
	}
	for _, tc := range []struct {
		wantErr     error
		wantCause   error
		name        string
		extentDelta int64
	}{
		{name: "exact captured extent admits the whole compiler"},
		{name: "growth beyond captured extent cannot be silently hashed", extentDelta: -1, wantErr: core.ErrReleaseContract},
		{name: "shrink below captured extent cannot become complete proof", extentDelta: 1, wantErr: core.ErrReleaseContract, wantCause: io.ErrUnexpectedEOF},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			location, err := filestore.OpenParent(t.Context(), path)
			if err != nil {
				t.Fatalf("filestore.OpenParent(compiler) error = %v, want nil", err)
			}
			t.Cleanup(func() {
				if err := location.Root.Close(); err != nil {
					t.Errorf("compiler parent Close() error = %v, want nil", err)
				}
			})
			file, err := filestore.OpenRead(t.Context(), filestore.ReadHandleRequest{Location: location})
			if err != nil {
				t.Fatalf("filestore.OpenRead(compiler) error = %v, want nil", err)
			}
			t.Cleanup(func() {
				if err := file.Close(); err != nil {
					t.Errorf("compiler Close() error = %v, want nil", err)
				}
			})
			info, err := file.Stat()
			if err != nil {
				t.Fatalf("compiler Stat() error = %v, want nil", err)
			}
			if info.Size() < 2 {
				t.Fatalf("compiler fixture extent = %d, want room below the captured extent", info.Size())
			}
			got, digest, gotErr := inspectBuildToolExtent(t.Context(), file, info.Size()+tc.extentDelta)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("inspectBuildToolExtent(delta=%d) error = %v, want %v", tc.extentDelta, gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if tc.wantCause != nil && !errors.Is(gotErr, tc.wantCause) {
					t.Fatalf("extent refusal cause = %v, want %v", gotErr, tc.wantCause)
				}
				if got != nil || digest != (core.SHA256Digest{}) {
					t.Fatalf("refused compiler = (%v, %v), want no identity or digest", got, digest)
				}
				return
			}
			version, err := CurrentGoToolchain().Version()
			if err != nil {
				t.Fatalf("GoToolchain.Version() error = %v, want nil", err)
			}
			if got == nil {
				t.Fatal("compiler build info = nil, want an admitted identity")
			}
			if got.Path != goCommandModulePath || got.GoVersion != version {
				t.Fatalf("compiler identity = (%q, %q), want (%q, %q)", got.Path, got.GoVersion, goCommandModulePath, version)
			}
			oracle := sha256.New()
			count, err := filestore.Read(t.Context(), filestore.ReadRequest{Location: location, Destination: oracle})
			if err != nil || count.Uint64() != uint64(info.Size()) {
				t.Fatalf("compiler oracle bytes = %v, error = %v, want %d, nil", count, err, info.Size())
			}
			wantDigest := core.NewSHA256Digest([core.SHA256DigestBytes]byte(oracle.Sum(nil)))
			if digest != wantDigest {
				t.Fatalf("compiler digest = %v, want Go SHA256 %v", digest, wantDigest)
			}
		})
	}
}
