package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/temporal"
)

func FuzzStageRestoreConservesNativePrefixAndCursor(f *testing.F) {
	seed, err := filestore.FilestoreRoundTripSeedForTest(f.Context(), f.TempDir())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed, uint16(len(seed)/2), false, false)
	f.Add(seed, uint16(len(seed)+1), false, false)
	f.Add(seed, uint16(0), true, false)
	f.Add(seed, uint16(0), false, true)
	f.Add([]byte{}, uint16(0), false, false)
	f.Add([]byte{0, 255, 31, 10}, uint16(0), false, false)
	f.Add([]byte{0, 255, 31, 10}, uint16(4), false, false)
	f.Fuzz(func(t *testing.T, payload []byte, rawPrefix uint16, copied, canceled bool) {
		payload = payload[:min(len(payload), 1024)]
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
		written, err := filestore.WriteStage(t.Context(), filestore.StageWriteRequest{Destination: destination, Data: payload})
		if err != nil || written.Validate() != nil || written.BytesWritten.Uint64() != uint64(len(payload)) {
			t.Fatalf("seed write = (%v,%v), want exact native payload", written, err)
		}
		prefix, err := core.NewByteLength(uint64(rawPrefix))
		if err != nil {
			t.Fatal(err)
		}
		selected := destination
		if copied {
			copy := *destination
			selected = &copy
		}
		ctx := t.Context()
		if canceled {
			observed, cancel, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: ctx})
			if err != nil {
				t.Fatal(err)
			}
			cancel(context.Canceled)
			ctx = observed
		}
		observation, restoreErr := filestore.RestoreStage(ctx, filestore.StageRestoreRequest{Destination: selected, Prefix: prefix})
		var wantErr error
		switch {
		case canceled:
			wantErr = context.Canceled
		case copied:
			wantErr = core.ErrFilestoreContract
		case int(rawPrefix) > len(payload):
			wantErr = core.ErrFilestoreSize
		}
		retained := len(payload)
		if wantErr != nil {
			if !errors.Is(restoreErr, wantErr) || observation.Validate() == nil {
				t.Fatalf("restore refusal = (%v,%v), want no receipt and %v", observation, restoreErr, wantErr)
			}
		} else {
			if restoreErr != nil || observation.Validate() != nil || observation.BeforeBytes().Uint64() != uint64(len(payload)) || observation.AfterBytes() != prefix || observation.Offset() != prefix {
				t.Fatalf("restore observation = (%v,%v), want exact native prefix %d", observation, restoreErr, rawPrefix)
			}
			retained = int(rawPrefix)
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
		want := append(bytes.Clone(payload[:retained]), suffix...)
		got := make([]byte, len(want))
		n, err := io.ReadFull(file, got)
		if err != nil || n != len(want) || !bytes.Equal(got, want) || stage.BytesWritten().Uint64() != uint64(len(want)) {
			t.Fatalf("native prefix/cursor oracle = (%v,%d,%v), want %v", got, n, err, want)
		}
	})
}
