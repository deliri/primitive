package release

import (
	"crypto/sha256"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/process"
)

// The installed compiler is inspected, not executed. The independently hashed
// fixture identity is printed so paired runs must name identical input bytes.
func BenchmarkInspectBuildToolExecutable(b *testing.B) {
	name, err := core.ParsePathComponent("go")
	if err != nil {
		b.Fatalf("ParsePathComponent(go) error = %v, want nil", err)
	}
	path, err := process.Resolve(b.Context(), name)
	if err != nil {
		b.Fatalf("process.Resolve(go) error = %v, want nil", err)
	}
	location, err := filestore.OpenParent(b.Context(), path)
	if err != nil {
		b.Fatalf("filestore.OpenParent(go) error = %v, want nil", err)
	}
	defer func() {
		if err := location.Root.Close(); err != nil {
			b.Errorf("compiler parent Close() error = %v, want nil", err)
		}
	}()
	maximum, err := core.NewByteCount(buildToolExecutableMaximumBytes)
	if err != nil {
		b.Fatalf("core.NewByteCount() error = %v, want nil", err)
	}
	oracle := sha256.New()
	extent, err := filestore.Read(b.Context(), filestore.ReadRequest{Location: location, Destination: oracle, MaximumBytes: maximum})
	if err != nil {
		b.Fatalf("filestore.Read(compiler oracle) error = %v, want nil", err)
	}
	want := core.NewSHA256Digest([core.SHA256DigestBytes]byte(oracle.Sum(nil)))
	version, err := CurrentGoToolchain().Version()
	if err != nil {
		b.Fatalf("GoToolchain.Version() error = %v, want nil", err)
	}
	b.Logf("compiler fixture path=%s bytes=%d sha256=%x", path.String(), extent.Uint64(), oracle.Sum(nil))
	b.ReportAllocs()
	b.SetBytes(int64(extent.Uint64()))
	for b.Loop() {
		info, got, err := inspectBuildTool(b.Context(), path)
		if err != nil || info == nil {
			b.Fatalf("inspectBuildTool() info=%v error=%v, want admitted compiler and nil", info, err)
		}
		if got != want || info.Path != goCommandModulePath || info.GoVersion != version {
			b.Fatalf("compiler inspection = (%v, %q, %q), want (%v, %q, %q)", got, info.Path, info.GoVersion, want, goCommandModulePath, version)
		}
	}
}
