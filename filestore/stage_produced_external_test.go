package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestStageProducedNativeCompletionAndRefusal(t *testing.T) {
	t.Parallel()
	for _, extent := range []int{0, 1, 32767, 32768, 32769, 131073} {
		for _, refusal := range []string{"complete", "producer", "cancel", "panic", "closed"} {
			t.Run(strconv.Itoa(extent)+"/"+refusal, func(t *testing.T) {
				t.Parallel()
				root := requireTestRoot(t, t.TempDir())
				ctx, cancel := context.WithCancelCause(t.Context())
				defer cancel(nil)
				payload := bytes.Repeat([]byte{0, 255, 9, 31, 128}, (extent+4)/5)[:extent]
				var borrowed *os.File
				got, gotErr := filestore.StageProduced(ctx, filestore.StageProductionRequest{
					Destination: filestore.StageDestinationRequest{Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, "stage")}, Mode: 0o600},
					Produce: func(_ context.Context, destination io.Writer) error {
						borrowed = destination.(*os.File)
						for offset := 0; offset < len(payload); {
							end := min(len(payload), offset+4093)
							if n, err := destination.Write(payload[offset:end]); err != nil || n != end-offset {
								return errors.Join(err, io.ErrShortWrite)
							}
							offset = end
						}
						switch refusal {
						case "producer":
							return fs.ErrPermission
						case "cancel":
							cancel(fs.ErrPermission)
						case "panic":
							panic(fs.ErrPermission)
						case "closed":
							return borrowed.Close()
						}
						return nil
					},
				})
				if borrowed == nil {
					t.Fatal("producer received no native file")
				}
				if _, err := borrowed.Stat(); !errors.Is(err, fs.ErrClosed) {
					t.Fatalf("borrowed file after scope = %v, want closed", err)
				}
				if refusal != "complete" {
					want := fs.ErrPermission
					if refusal == "panic" {
						want = core.ErrFilestoreCallbackPanic
					} else if refusal == "closed" {
						want = fs.ErrClosed
					}
					if got != (filestore.StagedFile{}) || !errors.Is(gotErr, want) {
						t.Fatalf("refused stage = (%+v,%v), want zero and %v", got, gotErr, want)
					}
					if refusal == "cancel" && !errors.Is(gotErr, context.Canceled) {
						t.Fatalf("canceled stage = %v, want cancellation and its cause", gotErr)
					}
					if _, err := root.Lstat("stage"); !errors.Is(err, fs.ErrNotExist) {
						t.Fatalf("refused stage namespace = %v, want absent", err)
					}
					return
				}
				if gotErr != nil || got.Validate() != nil || got.BytesWritten().Uint64() != uint64(len(payload)) {
					t.Fatalf("completed stage = (%+v,%v), want exact extent %d", got, gotErr, len(payload))
				}
				if err := filestore.Commit(t.Context(), filestore.CommitRequest{Target: mustRelativePath(t, "target"), Staged: got, Install: filestore.InstallCreate}); err != nil {
					t.Fatal(err)
				}
				data, err := root.ReadFile("target")
				info, statErr := root.Stat("target")
				if err != nil || statErr != nil || !bytes.Equal(data, payload) || info.Mode().Perm() != 0o600 {
					t.Fatalf("published stage = (%d bytes,%v,%v), want exact bytes and private mode", len(data), err, statErr)
				}
				if _, err := root.Lstat("stage"); !errors.Is(err, fs.ErrNotExist) {
					t.Fatalf("committed temporary = %v, want absent", err)
				}
			})
		}
	}
}

func TestStageProducedAdmissionHasNoEffects(t *testing.T) {
	t.Parallel()
	for _, invalid := range []string{"nil_context", "canceled", "nil_producer", "invalid_destination", "existing_temporary"} {
		t.Run(invalid, func(t *testing.T) {
			t.Parallel()
			root := requireTestRoot(t, t.TempDir())
			ctx := t.Context()
			called := false
			request := filestore.StageProductionRequest{
				Destination: filestore.StageDestinationRequest{Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, "stage")}, Mode: 0o600},
				Produce:     func(context.Context, io.Writer) error { called = true; return nil },
			}
			want := core.ErrFilestoreContract
			switch invalid {
			case "nil_context":
				ctx = nil
				want = core.ErrNilContext
			case "canceled":
				canceled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = canceled
				want = context.Canceled
			case "nil_producer":
				request.Produce = nil
			case "invalid_destination":
				request.Destination = filestore.StageDestinationRequest{}
			case "existing_temporary":
				if err := root.WriteFile("stage", []byte("neighbor"), 0o600); err != nil {
					t.Fatal(err)
				}
				want = fs.ErrExist
			}
			got, err := filestore.StageProduced(ctx, request)
			if got != (filestore.StagedFile{}) || !errors.Is(err, want) || called {
				t.Fatalf("admission = (%+v,%v,called=%t), want zero, %v and no callback", got, err, called, want)
			}
			entries, readErr := os.ReadDir(root.Name())
			if readErr != nil {
				t.Fatal(readErr)
			}
			if invalid == "existing_temporary" {
				data, err := root.ReadFile("stage")
				if err != nil || string(data) != "neighbor" || len(entries) != 1 {
					t.Fatalf("existing temporary changed: (%q,%v,%d entries)", data, err, len(entries))
				}
			} else if len(entries) != 0 {
				t.Fatalf("refused admission created %d entries", len(entries))
			}
		})
	}
}
