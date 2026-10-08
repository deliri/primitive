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
	for _, tc := range []struct {
		name string
		kind stageBoundaryKind
	}{
		{name: "nil_custody", kind: stageBoundaryNilCustody},
		{name: "zero_custody", kind: stageBoundaryZeroCustody},
		{name: "copied_custody", kind: stageBoundaryCopiedCustody},
		{name: "settled_custody", kind: stageBoundarySettledCustody},
		{name: "closed_native_file", kind: stageBoundaryClosedNativeFile},
		{name: "nil_context", kind: stageBoundaryNilContext},
		{name: "canceled_context", kind: stageBoundaryCanceledContext},
		{name: "expired_context", kind: stageBoundaryExpiredContext},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := requireTestRoot(t, t.TempDir())
			destination, err := filestore.OpenStageDestination(t.Context(), filestore.StageDestinationRequest{Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, "stage")}, Mode: 0o600})
			if err != nil {
				t.Fatal(err)
			}
			selected, ctx := destination, t.Context()
			var want error = core.ErrFilestoreContract
			closed := false
			switch tc.kind {
			case stageBoundaryNilCustody:
				selected = nil
			case stageBoundaryZeroCustody:
				selected = &filestore.StageDestination{}
			case stageBoundaryCopiedCustody:
				copy := *destination
				selected = &copy
			case stageBoundarySettledCustody:
				if err := filestore.AbandonStageDestination(destination); err != nil {
					t.Fatal(err)
				}
			case stageBoundaryClosedNativeFile:
				file, err := destination.File()
				if err != nil {
					t.Fatal(err)
				}
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
				closed = true
				want = fs.ErrClosed
			case stageBoundaryNilContext:
				ctx = nil
				want = core.ErrNilContext
			case stageBoundaryCanceledContext:
				observed, cancel, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: t.Context()})
				if err != nil {
					t.Fatal(err)
				}
				cancel(context.Canceled)
				ctx = observed
				want = context.Canceled
			case stageBoundaryExpiredContext:
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
