package filestore_test

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/lineio"
)

func TestReadSnapshotLendsBoundedIndependentRecordCoordinates(t *testing.T) {
	t.Parallel()
	var expired lineio.RecordSource
	withSnapshotChunkFile(t, func(ctx context.Context, file filestore.ScratchReplayFile) {
		if _, err := filestore.WriteChunk(ctx, filestore.ChunkWriteRequest{Destination: file, Bytes: []byte("abcdefgh")}); err != nil {
			t.Fatal(err)
		}
		snapshot, err := filestore.ObserveReadSnapshot(ctx, filestore.ReadSnapshotRequest{Source: file})
		if err != nil {
			t.Fatal(err)
		}
		// The public field satisfies the native record-source contract directly.
		expired = snapshot.Reader
		if _, err := filestore.WriteChunk(ctx, filestore.ChunkWriteRequest{Destination: file, Bytes: []byte("tail")}); err != nil {
			t.Fatal(err)
		}
		var partial [4]byte
		if n, err := snapshot.Reader.ReadAt(partial[:], 6); n != 2 || !errors.Is(err, io.EOF) || string(partial[:n]) != "gh" {
			t.Fatalf("bounded positional read = %d/%q/%v, want gh and EOF", n, partial[:n], err)
		}
		var pair [2]byte
		if n, err := io.ReadFull(snapshot.Reader, pair[:]); err != nil || n != 2 || string(pair[:]) != "ab" {
			t.Fatalf("initial sequential cursor = %d/%q/%v", n, pair[:], err)
		}
		if n, err := snapshot.Reader.ReadAt(pair[:], 4); err != nil || n != 2 || string(pair[:]) != "ef" {
			t.Fatalf("middle positional read = %d/%q/%v", n, pair[:], err)
		}
		if n, err := io.ReadFull(snapshot.Reader, pair[:]); err != nil || n != 2 || string(pair[:]) != "cd" {
			t.Fatalf("positional read moved sequential cursor = %d/%q/%v", n, pair[:], err)
		}
		if offset, err := snapshot.Reader.Seek(0, io.SeekEnd); err != nil || offset != 8 {
			t.Fatalf("snapshot end = %d/%v, want original extent 8", offset, err)
		}
		if n, err := snapshot.Reader.Read(pair[:]); n != 0 || !errors.Is(err, io.EOF) {
			t.Fatalf("snapshot tail = %d/%v, want EOF despite later native bytes", n, err)
		}
		if offset, err := file.Seek(0, io.SeekCurrent); err != nil || offset != 12 {
			t.Fatalf("snapshot access changed native write cursor = %d/%v", offset, err)
		}
	})
	var pair [2]byte
	if n, err := expired.ReadAt(pair[:], 0); n != 0 || !errors.Is(err, os.ErrClosed) {
		t.Fatalf("expired positional source = %d/%v, want native closed handle", n, err)
	}
}
