package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/filestore"
)

func FuzzStageProducedPublicationRequiresCompletion(f *testing.F) {
	f.Add([]byte{}, false, false)
	f.Add([]byte{0, 255, 31, 8, 128}, false, false)
	f.Add([]byte{0, 255, 31, 8, 128}, true, false)
	f.Add(bytes.Repeat([]byte{0, 255, 31, 8, 128}, 6554), false, true)
	f.Fuzz(func(t *testing.T, payload []byte, refused, canceled bool) {
		root := requireTestRoot(t, t.TempDir())
		ctx, cancel := context.WithCancelCause(t.Context())
		defer cancel(nil)
		got, err := filestore.StageProduced(ctx, filestore.StageProductionRequest{
			Destination: filestore.StageDestinationRequest{Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, "stage")}, Mode: 0o600},
			Produce: func(_ context.Context, destination io.Writer) error {
				if _, err := io.Copy(destination, bytes.NewReader(payload)); err != nil {
					return err
				}
				if canceled {
					cancel(fs.ErrPermission)
				}
				if refused {
					return fs.ErrPermission
				}
				return nil
			},
		})
		if refused || canceled {
			if got != (filestore.StagedFile{}) || !errors.Is(err, fs.ErrPermission) || canceled && !errors.Is(err, context.Canceled) {
				t.Fatalf("refused production = (%+v,%v), want zero, native cause and cancellation when selected", got, err)
			}
			entries, readErr := os.ReadDir(root.Name())
			if readErr != nil || len(entries) != 0 {
				t.Fatalf("refused production left (%d entries,%v), want empty namespace", len(entries), readErr)
			}
			return
		}
		if err != nil || got.Validate() != nil || got.BytesWritten().Uint64() != uint64(len(payload)) {
			t.Fatalf("completed production = (%+v,%v), want exact %d bytes", got, err, len(payload))
		}
		if err := filestore.Commit(t.Context(), filestore.CommitRequest{Staged: got, Target: mustRelativePath(t, "target"), Install: filestore.InstallCreate}); err != nil {
			t.Fatal(err)
		}
		data, err := root.ReadFile("target")
		if err != nil || !bytes.Equal(data, payload) {
			t.Fatalf("published production = (%d bytes,%v), want exact input %d bytes", len(data), err, len(payload))
		}
	})
}
