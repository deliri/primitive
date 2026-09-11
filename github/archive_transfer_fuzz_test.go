package github

import (
	"bytes"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"sync/atomic"
	"testing"
)

type archivePrefixDestination struct {
	bytes.Buffer
	remaining int
	refusal   error
}

func (d *archivePrefixDestination) Write(p []byte) (int, error) {
	if len(p) <= d.remaining {
		n, err := d.Buffer.Write(p)
		d.remaining -= n
		return n, err
	}
	n, err := d.Buffer.Write(p[:d.remaining])
	d.remaining -= n
	return n, errors.Join(d.refusal, err)
}
func FuzzGitHubArchiveTransferSemanticClosure(f *testing.F) {
	f.Add(marshalGitHubFixture(f, headWire{SHA: parsedCommit(f).String()}), uint16(65535))
	f.Add([]byte{}, uint16(0))
	f.Add([]byte("partial provider bytes"), uint16(3))
	f.Fuzz(func(t *testing.T, payload []byte, capacity uint16) {
		if len(payload) > 65536 {
			t.Skip("input exceeds bounded destination oracle custody")
		}
		var calls atomic.Uint64
		server := archiveServer(t, payload, &calls)
		defer server.Close()
		client := clientFixture(t, server.URL)
		defer func() {
			if err := client.Close(); err != nil {
				t.Errorf("Close()=%v, want nil", err)
			}
		}()
		refused := errors.New("destination refuses remaining bytes")
		destination := &archivePrefixDestination{remaining: int(capacity), refusal: refused}
		request := TarArchiveRequest{Repository: parsedRepository(t, "owner/repository"), Commit: parsedCommit(t), Destination: destination}
		got, err := client.ReadTarArchive(t.Context(), request)
		want := payload[:min(len(payload), int(capacity))]
		if calls.Load() != 1 || !bytes.Equal(destination.Bytes(), want) || got.Length.Uint64() != uint64(len(want)) || got.SHA256 != core.SHA256Of(want) || got.Repository != request.Repository || got.Commit != request.Commit {
			t.Fatalf("archive=%+v/%v calls=%d prefix=%d, want exact %d-byte single transfer", got, err, calls.Load(), destination.Len(), len(want))
		}
		if len(payload) > int(capacity) {
			if !errors.Is(err, refused) || got.State != ArchiveTransferIncomplete {
				t.Fatalf("destination refusal=%v/%v, want preserved refusal and incomplete observation", err, got.State)
			}
		} else if len(payload) == 0 {
			if !errors.Is(err, core.ErrGitHubResponse) || got.State != ArchiveTransferIncomplete {
				t.Fatalf("empty archive=%v/%v, want typed refusal and incomplete observation", err, got.State)
			}
		} else if err != nil || got.State != ArchiveTransferComplete || got.Validate() != nil {
			t.Fatalf("completed archive=%+v/%v, want valid complete observation", got, err)
		}
	})
}
