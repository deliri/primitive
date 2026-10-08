package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestStageRestorePreservesNativePrefixAndResumesAtItsTail(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		prefix uint64
	}{
		{name: "empty_prefix", prefix: 0},
		{name: "partial_prefix", prefix: 3},
		{name: "complete_prefix", prefix: 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := requireTestRoot(t, t.TempDir())
			destination, err := filestore.OpenStageDestination(t.Context(), filestore.StageDestinationRequest{Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, "stage")}, Mode: 0o600})
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
			payload := []byte{0, 255, 31, 10, 17, 5}
			written, err := filestore.WriteStage(t.Context(), filestore.StageWriteRequest{Destination: destination, Data: payload})
			if err != nil || written.Validate() != nil || written.BytesWritten.Uint64() != uint64(len(payload)) {
				t.Fatalf("initial write = (%v,%v), want exact payload", written, err)
			}
			prefix, err := core.NewByteLength(tc.prefix)
			if err != nil {
				t.Fatal(err)
			}
			observation, err := filestore.RestoreStage(t.Context(), filestore.StageRestoreRequest{Destination: destination, Prefix: prefix})
			if err != nil || observation.Validate() != nil || observation.BeforeBytes().Uint64() != uint64(len(payload)) || observation.AfterBytes() != prefix || observation.Offset() != prefix {
				t.Fatalf("restore = (%v,%v), want native prefix and cursor %d", observation, err, tc.prefix)
			}
			suffix := []byte{14, 255}
			written, err = filestore.WriteStage(t.Context(), filestore.StageWriteRequest{Destination: destination, Data: suffix})
			if err != nil || written.Validate() != nil || written.BytesWritten.Uint64() != uint64(len(suffix)) {
				t.Fatalf("resumed write = (%v,%v), want exact suffix", written, err)
			}
			stage, err := filestore.FinishStageDestination(t.Context(), destination)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := filestore.Discard(t.Context(), stage); err != nil {
					t.Error(err)
				}
			}()
			file, err := filestore.OpenStagedRead(t.Context(), stage)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := file.Close(); err != nil {
					t.Error(err)
				}
			}()
			got := make([]byte, tc.prefix+uint64(len(suffix)))
			n, err := io.ReadFull(file, got)
			want := append(bytes.Clone(payload[:tc.prefix]), suffix...)
			if err != nil || n != len(want) || !bytes.Equal(got, want) || stage.BytesWritten().Uint64() != uint64(len(want)) {
				t.Fatalf("native body = (%v,%d,%v), want %v", got, n, err, want)
			}
		})
	}
}

func TestStageRestoreRefusesInvalidCustodyAndExtendingPrefixes(t *testing.T) {
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
		{name: "prefix_above_native_extent", kind: stageBoundaryExtendingPrefix},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := requireTestRoot(t, t.TempDir())
			destination, err := filestore.OpenStageDestination(t.Context(), filestore.StageDestinationRequest{Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, "stage")}, Mode: 0o600})
			if err != nil {
				t.Fatal(err)
			}
			payload := []byte{0, 255, 31, 10}
			written, err := filestore.WriteStage(t.Context(), filestore.StageWriteRequest{Destination: destination, Data: payload})
			if err != nil || written.Validate() != nil || written.BytesWritten.Uint64() != uint64(len(payload)) {
				t.Fatalf("initial write = (%v,%v), want complete native payload", written, err)
			}
			selected, ctx := destination, t.Context()
			prefix := core.ByteLength{}
			var want error = core.ErrFilestoreContract
			closed, settled := false, false
			switch tc.kind {
			case stageBoundaryNilCustody:
				selected = nil
			case stageBoundaryZeroCustody:
				selected = &filestore.StageDestination{}
			case stageBoundaryCopiedCustody:
				copied := *destination
				selected = &copied
			case stageBoundarySettledCustody:
				if err := filestore.AbandonStageDestination(destination); err != nil {
					t.Fatal(err)
				}
				settled = true
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
				ctx = observed
				want = context.DeadlineExceeded
			case stageBoundaryExtendingPrefix:
				prefix, err = core.NewByteLength(uint64(len(payload) + 1))
				if err != nil {
					t.Fatal(err)
				}
				want = core.ErrFilestoreSize
			default:
				t.Fatal("unclassified restore boundary")
			}
			defer func() {
				if destination.Validate() == nil {
					err := filestore.AbandonStageDestination(destination)
					if err != nil && !(closed && errors.Is(err, fs.ErrClosed)) {
						t.Error(err)
					}
				}
			}()
			observation, err := filestore.RestoreStage(ctx, filestore.StageRestoreRequest{Destination: selected, Prefix: prefix})
			if !errors.Is(err, want) || observation.Validate() == nil {
				t.Fatalf("restore refusal = (%v,%v), want unavailable restoration and %v", observation, err, want)
			}
			if closed || settled {
				return
			}
			suffix := []byte{14, 255}
			written, err = filestore.WriteStage(t.Context(), filestore.StageWriteRequest{Destination: destination, Data: suffix})
			if err != nil || written.Validate() != nil || written.BytesWritten.Uint64() != uint64(len(suffix)) {
				t.Fatalf("original custody = (%v,%v), want exact suffix", written, err)
			}
			stage, err := filestore.FinishStageDestination(t.Context(), destination)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := filestore.Discard(t.Context(), stage); err != nil {
					t.Error(err)
				}
			}()
			file, err := filestore.OpenStagedRead(t.Context(), stage)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := file.Close(); err != nil {
					t.Error(err)
				}
			}()
			wantBody := append(bytes.Clone(payload), suffix...)
			got := make([]byte, len(wantBody))
			n, err := io.ReadFull(file, got)
			if err != nil || n != len(wantBody) || !bytes.Equal(got, wantBody) || stage.BytesWritten().Uint64() != uint64(len(wantBody)) {
				t.Fatalf("native body after refusal = (%v,%d,%v), want %v", got, n, err, wantBody)
			}
		})
	}
}
