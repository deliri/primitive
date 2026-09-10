package github

import (
	"bytes"
	"io"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func BenchmarkTarArchive1MiB(b *testing.B) {
	content := bytes.Repeat([]byte{0xa5}, 1<<20)
	wantDigest := core.SHA256Of(content)
	var archiveCalls atomic.Uint64
	server := archiveServer(b, content, &archiveCalls)
	defer server.Close()
	client := clientFixture(b, server.URL)
	request := TarArchiveRequest{
		Destination: io.Discard, Repository: parsedRepository(b, "owner/repository"),
		Commit: parsedCommit(b),
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(content)))
	var got TarArchiveObservation
	for b.Loop() {
		var err error
		got, err = client.ReadTarArchive(b.Context(), request)
		if err != nil {
			b.Fatal(err)
		}
	}
	if got.Validate() != nil || got.State != ArchiveTransferComplete || got.Length.Uint64() != uint64(len(content)) || got.SHA256 != wantDigest || archiveCalls.Load() != uint64(b.N) {
		b.Fatalf("archive = %+v, calls=%d; want exact digest, %d bytes and %d calls", got, archiveCalls.Load(), len(content), b.N)
	}
}
