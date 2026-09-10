package filestore_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestStageDeclarationIsOptionalAndOwnedLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                       string
		declared, written, mutated uint64
		known, mutate              bool
		wantErr                    error
	}{
		{name: "unknown empty extent"},
		{name: "unknown nonempty extent", written: 32769},
		{name: "known empty extent", known: true},
		{name: "known empty refuses bytes", known: true, written: 1, wantErr: core.ErrFilestoreSize},
		{name: "known extent refuses truncation", known: true, declared: 3, written: 2, wantErr: core.ErrFilestoreSize},
		{name: "known extent accepts exact bytes", known: true, declared: 3, written: 3},
		{name: "known extent refuses growth", known: true, declared: 3, written: 4, wantErr: core.ErrFilestoreSize},
		{name: "caller mutation cannot shrink captured agreement", known: true, declared: 3, written: 3, mutate: true, mutated: 2},
		{name: "caller mutation cannot widen captured agreement", known: true, declared: 3, written: 4, mutate: true, mutated: 4, wantErr: core.ErrFilestoreSize},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			root := requireTestRoot(t, directory)
			request := filestore.StageDestinationRequest{Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, "stage")}, Mode: 0600}
			declared := stageDestinationLength(t, tc.declared)
			if tc.known {
				request.ExpectedBytes = &declared
			}
			destination, err := filestore.OpenStageDestination(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if destination.Validate() == nil {
					if err := filestore.AbandonStageDestination(destination); err != nil {
						t.Errorf("Abandon=%v", err)
					}
				}
			})
			if tc.mutate {
				declared = stageDestinationLength(t, tc.mutated)
			}
			file, err := destination.File()
			if err != nil {
				t.Fatal(err)
			}
			payload := deterministicPayload(int(tc.written))
			n, writeErr := file.Write(payload)
			if writeErr != nil || n != len(payload) {
				t.Fatalf("Write=%d/%v, want %d/nil", n, writeErr, len(payload))
			}
			got, err := filestore.FinishStageDestination(t.Context(), destination)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Finish=%v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				names, readErr := directoryEntryNames(directory)
				if readErr != nil || len(names) != 0 || got != (filestore.StagedFile{}) {
					t.Fatalf("refused stage=%+v names=%v/%v, want no file or receipt", got, names, readErr)
				}
				return
			}
			if got.Validate() != nil || got.BytesWritten().Uint64() != tc.written {
				t.Fatalf("receipt=%+v, want %d-byte native extent", got, tc.written)
			}
			if err := filestore.Discard(t.Context(), got); err != nil {
				t.Fatal(err)
			}
		})
	}
}
