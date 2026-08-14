package upgrade

import (
	"bytes"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/filestore"
)

func BenchmarkStageDownloadStreamingFourKiB(b *testing.B) {
	b.ReportAllocs()
	benchmarkStageCandidateStreaming(b, 4<<10)
}

func BenchmarkStageDownloadStreamingTenMiB(b *testing.B) {
	b.ReportAllocs()
	benchmarkStageCandidateStreaming(b, 10<<20)
}

func benchmarkStageCandidateStreaming(b *testing.B, size int) {
	b.Helper()

	data := bytes.Repeat([]byte{0x5a}, size)
	root, directory := stageRootForTest(b)
	installed := artifactForTest(b, []byte("installed"), 1)
	candidate := artifactForTest(b, data, 2)
	target := stageTargetForTest(b, stageTargetFixture{
		directory: directory, installed: installed, candidate: candidate,
	})
	request := StageRequest{
		Root: root,
		Source: stageDownloadSourceForTest(b, stageDownloadSourceFixture{
			objectName: "bucket/candidate", transport: &stageDownloadTransport{payload: data},
		}),
	}
	b.ReportAllocs()
	b.SetBytes(int64(size))
	b.ResetTimer()
	for range b.N {
		if err := prepareCandidateSlot(b.Context(), root, target); err != nil {
			b.Fatalf("prepareCandidateSlot() error = %v, want nil", err)
		}
		download, err := downloadCandidate(b.Context(), request, target)
		if err != nil || !download.owned {
			b.Fatalf("downloadCandidate() = (%v, %v), want owned staged bytes", download, err)
		}
		if err := cleanupOwnedCandidate(b.Context(), root, target, download); err != nil {
			b.Fatalf("cleanupOwnedCandidate() error = %v, want nil", err)
		}
	}
}

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
