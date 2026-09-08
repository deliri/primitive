package release

import (
	"crypto/sha256"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/process"
)

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
		name        string
		extentDelta int64
		wantErr     error
		wantCause   error
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
			if info.Size() < 2 || info.Size() >= buildToolExecutableMaximumBytes {
				t.Fatalf("compiler fixture extent = %d, want room on both sides inside %d", info.Size(), buildToolExecutableMaximumBytes)
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
			maximum, err := core.NewByteCount(uint64(info.Size()))
			if err != nil {
				t.Fatalf("core.NewByteCount(compiler) error = %v, want nil", err)
			}
			oracle := sha256.New()
			count, err := filestore.Read(t.Context(), filestore.ReadRequest{Location: location, Destination: oracle, MaximumBytes: maximum})
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
