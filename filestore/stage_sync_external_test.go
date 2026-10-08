package filestore_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestStageSyncObservesNativeExtentWithoutSettlingCustody(t *testing.T) {
	t.Parallel()
	for _, payload := range [][]byte{nil, {0, 255, 10}} {
		name := "empty_native_extent"
		if len(payload) != 0 {
			name = "partial_declared_extent"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := requireTestRoot(t, t.TempDir())
			declared, err := core.NewByteLength(uint64(len(payload) + 1))
			if err != nil {
				t.Fatal(err)
			}
			destination, err := filestore.OpenStageDestination(t.Context(), filestore.StageDestinationRequest{Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, "stage")}, Mode: 0o600, ExpectedBytes: &declared})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if destination.Validate() == nil {
					if err := filestore.AbandonStageDestination(destination); err != nil {
						t.Error(err)
					}
				}
			}()
			written, err := filestore.WriteStage(t.Context(), filestore.StageWriteRequest{Destination: destination, Data: payload})
			if err != nil || written.Validate() != nil {
				t.Fatalf("write = (%v,%v), want observed", written, err)
			}
			observation, err := filestore.SyncStage(t.Context(), filestore.StageSyncRequest{Destination: destination})
			if err != nil || observation.Validate() != nil || observation.BytesWritten().Uint64() != uint64(len(payload)) {
				t.Fatalf("sync = (%v,%v), want native extent %d", observation, err, len(payload))
			}
			written, err = filestore.WriteStage(t.Context(), filestore.StageWriteRequest{Destination: destination, Data: []byte{31}})
			if err != nil || written.Validate() != nil || written.BytesWritten.Uint64() != 1 {
				t.Fatalf("continued custody = (%v,%v), want one native byte", written, err)
			}
			stage, err := filestore.FinishStageDestination(t.Context(), destination)
			if err != nil || stage.Validate() != nil || stage.BytesWritten() != declared {
				t.Fatalf("finish = (%v,%v), want exact declared extent", stage, err)
			}
			if err := filestore.Discard(t.Context(), stage); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestStageSyncRefusesUnownedOrStoppedExecution(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"nil_custody", "zero_custody", "copied_custody", "settled_custody", "closed_native_file", "nil_context", "canceled_context", "expired_context"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := requireTestRoot(t, t.TempDir())
			destination, err := filestore.OpenStageDestination(t.Context(), filestore.StageDestinationRequest{Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, "stage")}, Mode: 0o600})
			if err != nil {
				t.Fatal(err)
			}
			selected, ctx := destination, t.Context()
			var want error = core.ErrFilestoreContract
			closed := false
			switch name {
			case "nil_custody":
				selected = nil
			case "zero_custody":
				selected = &filestore.StageDestination{}
			case "copied_custody":
				copy := *destination
				selected = &copy
			case "settled_custody":
				if err := filestore.AbandonStageDestination(destination); err != nil {
					t.Fatal(err)
				}
			case "closed_native_file":
				file, err := destination.File()
				if err != nil {
					t.Fatal(err)
				}
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
				closed = true
				want = fs.ErrClosed
			case "nil_context":
				ctx = nil
				want = core.ErrNilContext
			case "canceled_context":
				observed, cancel, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: t.Context()})
				if err != nil {
					t.Fatal(err)
				}
				cancel(context.Canceled)
				ctx = observed
				want = context.Canceled
			case "expired_context":
				observed, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: t.Context(), Duration: temporal.Duration{}})
				if err != nil {
					t.Fatal(err)
				}
				defer cancel()
				ctx, want = observed, context.DeadlineExceeded
			default:
				t.Fatal("unclassified boundary")
			}
			defer func() {
				if destination.Validate() == nil {
					err := filestore.AbandonStageDestination(destination)
					if err != nil && !(closed && errors.Is(err, os.ErrClosed)) {
						t.Error(err)
					}
				}
			}()
			observation, err := filestore.SyncStage(ctx, filestore.StageSyncRequest{Destination: selected})
			if !errors.Is(err, want) || observation.Validate() == nil {
				t.Fatalf("sync refusal = (%v,%v), want unavailable observation and %v", observation, err, want)
			}
		})
	}
}
