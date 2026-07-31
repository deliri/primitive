package upgrade

import (
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/filestore"
)

func BenchmarkResolvePrimaryFourKiB(b *testing.B) {
	directory := b.TempDir()
	root, err := os.OpenRoot(directory)
	if err != nil {
		b.Fatalf("os.OpenRoot() error = %v, want nil", err)
	}
	b.Cleanup(func() { _ = root.Close() })
	data := make([]byte, 4<<10)
	artifact := artifactForTest(b, data, 1)
	installArtifactForTest(b, root, SlotA, artifact, data)
	document := selectionDocument{
		Revision: selectionRevisionCurrent, Slot: SlotA, Artifact: artifact,
	}
	if err := writeSelection(
		b.Context(), root, document, filestore.InstallCreate,
	); err != nil {
		b.Fatalf("writeSelection() error = %v, want nil", err)
	}
	request := ResolveRequest{
		Root: root, Directory: absolutePathForTest(b, directory),
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()
	for range b.N {
		if _, err := ResolvePrimary(b.Context(), request); err != nil {
			b.Fatalf("ResolvePrimary() error = %v, want nil", err)
		}
	}
}
