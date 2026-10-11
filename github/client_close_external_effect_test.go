package github

import (
	"bytes"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestClientCloseRevokesEveryCopiedSocket(t *testing.T) {
	t.Parallel()
	for index := range 64 {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			t.Parallel()
			var calls atomic.Uint64
			server := archiveServer(t, []byte("archive"), &calls)
			defer server.Close()
			client := clientFixture(t, server.URL)
			if index%2 == 1 {
				if err := client.Close(); err != nil {
					t.Fatal(err)
				}
				observed, err := temporal.Observe()
				if err != nil {
					t.Fatal(err)
				}
				client = authenticatedClientFixture(t, server.URL, observed)
			} else {
				var destination bytes.Buffer
				_, err := client.ReadTarArchive(t.Context(), TarArchiveRequest{Destination: &destination, Repository: parsedRepository(t, "owner/repository"), Commit: parsedCommit(t)})
				if err != nil || calls.Load() != 1 {
					t.Fatalf("live native socket: calls=%d err=%v", calls.Load(), err)
				}
			}
			alias := client
			closer := client
			if err := closer.Close(); err != nil {
				t.Fatal(err)
			}
			before := calls.Load()
			for copyIndex := range index + 1 {
				copy := alias
				var destination bytes.Buffer
				_, err := copy.ReadTarArchive(t.Context(), TarArchiveRequest{Destination: &destination, Repository: parsedRepository(t, "owner/repository"), Commit: parsedCommit(t)})
				if !errors.Is(err, core.ErrGitHubContract) || copy.Validate() == nil || destination.Len() != 0 || calls.Load() != before {
					t.Fatalf("copy %d reopened destroyed socket: err=%v calls=%d", copyIndex, err, calls.Load())
				}
			}
		})
	}
}
