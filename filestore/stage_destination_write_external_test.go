package filestore_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestStageWriteRefusesStoppedContextBeforeNativeOutput(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		kind stageBoundaryKind
	}{
		{name: "nil_context", kind: stageBoundaryNilContext},
		{name: "canceled_context", kind: stageBoundaryCanceledContext},
		{name: "expired_context", kind: stageBoundaryExpiredContext},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var ctx context.Context
			var want error = core.ErrNilContext
			switch tc.kind {
			case stageBoundaryCanceledContext:
				observed, cancel, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: t.Context()})
				if err != nil {
					t.Fatal(err)
				}
				cancel(context.Canceled)
				ctx, want = observed, context.Canceled
			case stageBoundaryExpiredContext:
				observed, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: t.Context(), Duration: temporal.Duration{}})
				if err != nil {
					t.Fatal(err)
				}
				defer cancel()
				ctx, want = observed, context.DeadlineExceeded
			case stageBoundaryNilContext:
			default:
				t.Fatal("unclassified context case")
			}
			directory := t.TempDir()
			root := requireTestRoot(t, directory)
			destination, err := filestore.OpenStageDestination(t.Context(), filestore.StageDestinationRequest{Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, "stage")}, Mode: 0o600})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := filestore.AbandonStageDestination(destination); err != nil {
					t.Error(err)
				}
			}()
			request := filestore.StageWriteRequest{Destination: destination, Data: []byte{255, 0, 10}}
			if err := request.Validate(); err != nil {
				t.Fatal(err)
			}
			observation, err := filestore.WriteStage(ctx, request)
			if !errors.Is(err, want) || observation.Validate() == nil {
				t.Fatalf("stopped write = (%v, %v), want unobserved output and %v", observation, err, want)
			}
			if err := destination.Validate(); err != nil {
				t.Fatalf("refusal lost original custody: %v", err)
			}
			body, err := os.ReadFile(filepath.Join(directory, "stage"))
			if err != nil || len(body) != 0 {
				t.Fatalf("native output after refusal = (%v, %v), want empty", body, err)
			}
		})
	}
}

func TestStageDestinationStreamsLargeExtentFromOneReusableChunk(t *testing.T) {
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
	chunk := make([]byte, 64<<10)
	const chunks = 145
	for range chunks {
		if observation, err := filestore.WriteStage(t.Context(), filestore.StageWriteRequest{Destination: destination, Data: chunk}); observation.Validate() != nil || observation.BytesWritten.Uint64() != uint64(len(chunk)) || err != nil {
			t.Fatalf("streamed chunk = (%d, %v), want (%d, nil)", observation.BytesWritten.Uint64(), err, len(chunk))
		}
	}
	staged, err := filestore.FinishStageDestination(t.Context(), destination)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := filestore.Discard(t.Context(), staged); err != nil {
			t.Error(err)
		}
	}()
	if err := staged.Validate(); err != nil {
		t.Fatal(err)
	}
	if got, want := staged.BytesWritten().Uint64(), uint64(chunks*len(chunk)); got != want {
		t.Fatalf("native synchronized extent = %d, want %d", got, want)
	}
}

func TestStageDestinationWritesThroughLinearNativeCustody(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                                                  string
		nilHandle, zeroHandle, copied, settled, closed, empty bool
		want                                                  error
	}{
		{name: "owned_writer_preserves_exact_binary_bytes"},
		{name: "empty_write_preserves_a_real_empty_file", empty: true},
		{name: "nil_custody_refuses_output", nilHandle: true, want: core.ErrFilestoreContract},
		{name: "zero_custody_refuses_output", zeroHandle: true, want: core.ErrFilestoreContract},
		{name: "copied_custody_cannot_write_the_original_inode", copied: true, want: core.ErrFilestoreContract},
		{name: "settled_custody_cannot_acquire_new_output", settled: true, want: core.ErrFilestoreContract},
		{name: "externally_closed_native_file_preserves_closed_identity", closed: true, want: fs.ErrClosed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			root := requireTestRoot(t, directory)
			destination, err := filestore.OpenStageDestination(t.Context(), filestore.StageDestinationRequest{Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, "stage")}, Mode: 0o600})
			if err != nil {
				t.Fatal(err)
			}
			file, err := destination.File()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if destination.Validate() != nil {
					return
				}
				cleanupErr := filestore.AbandonStageDestination(destination)
				if cleanupErr != nil && !(tc.closed && errors.Is(cleanupErr, fs.ErrClosed)) {
					t.Errorf("abandon original custody: %v", cleanupErr)
				}
				if _, err := os.Stat(filepath.Join(directory, "stage")); !errors.Is(err, fs.ErrNotExist) {
					t.Errorf("abandoned stage observation = %v, want absent", err)
				}
			})
			selected := destination
			switch {
			case tc.nilHandle:
				selected = nil
			case tc.zeroHandle:
				selected = &filestore.StageDestination{}
			case tc.copied:
				copy := *destination
				selected = &copy
			case tc.settled:
				if err := filestore.AbandonStageDestination(destination); err != nil {
					t.Fatal(err)
				}
			case tc.closed:
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
			}
			payload := []byte{0, 255, 31, 10}
			if tc.empty {
				payload = nil
			}
			observation, err := filestore.WriteStage(t.Context(), filestore.StageWriteRequest{Destination: selected, Data: payload})
			n := observation.BytesWritten.Uint64()
			if refused := tc.nilHandle || tc.zeroHandle || tc.copied || tc.settled; refused != (observation.Validate() != nil) {
				t.Fatalf("observation validity disagrees with native execution: %v, %v", observation.Validate(), err)
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("write error = %v, want %v", err, tc.want)
			}
			wantBytes := len(payload)
			if tc.want != nil {
				wantBytes = 0
			}
			if n != uint64(wantBytes) {
				t.Fatalf("accepted bytes = %d, want %d", n, wantBytes)
			}
			if tc.closed && !errors.Is(err, core.ErrFilestoreDestination) {
				t.Fatalf("native write refusal = %v, want destination identity", err)
			}
			if tc.settled {
				if _, err := os.Stat(filepath.Join(directory, "stage")); !errors.Is(err, fs.ErrNotExist) {
					t.Fatalf("settled output namespace = %v, want absent", err)
				}
				return
			}
			body, err := os.ReadFile(filepath.Join(directory, "stage"))
			if err != nil {
				t.Fatal(err)
			}
			if len(body) != wantBytes {
				t.Fatalf("native extent = %d, want %d", len(body), wantBytes)
			}
			for i := range body {
				if body[i] != payload[i] {
					t.Fatalf("native byte %d = %d, want %d", i, body[i], payload[i])
				}
			}
		})
	}
}
