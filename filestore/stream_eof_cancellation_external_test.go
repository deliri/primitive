package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/filestore"
)

type stageEOFSource struct {
	source *bytes.Reader
	cancel context.CancelFunc
}

func (s stageEOFSource) Read(p []byte) (int, error) {
	n, err := s.source.Read(p)
	if s.source.Len() == 0 {
		if s.cancel != nil {
			s.cancel()
		}
		return n, io.EOF
	}
	return n, err
}

func TestStageCancellationAtExactEOFRemovesOwnedFileLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		size   int
		cancel bool
	}{
		{name: "empty successful EOF"},
		{name: "empty canceled EOF", cancel: true},
		{name: "one byte successful EOF", size: 1},
		{name: "one byte canceled EOF", size: 1, cancel: true},
		{name: "copy window and final byte successful EOF", size: 32769},
		{name: "copy window and final byte canceled EOF", size: 32769, cancel: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			root := requireTestRoot(t, directory)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			source := stageEOFSource{source: bytes.NewReader(deterministicPayload(tc.size))}
			if tc.cancel {
				source.cancel = cancel
			}
			got, err := filestore.Stage(ctx, filestore.StageRequest{
				Source: source, Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, "stage")}, Mode: 0600, Buffer: make([]byte, 32768),
			})
			var wantErr error
			if tc.cancel {
				wantErr = context.Canceled
			}
			if !errors.Is(err, wantErr) {
				t.Fatalf("Stage=%v, want %v", err, wantErr)
			}
			if tc.cancel {
				names, readErr := directoryEntryNames(directory)
				if readErr != nil || len(names) != 0 || got.BytesWritten().Uint64() != 0 {
					t.Fatalf("canceled stage entries/error/receipt bytes=%v/%v/%d, want no owned file and no receipt", names, readErr, got.BytesWritten().Uint64())
				}
				return
			}
			if got.Validate() != nil || got.BytesWritten().Uint64() != uint64(tc.size) {
				t.Fatalf("stage=%v/%d, want valid %d-byte receipt", got.Validate(), got.BytesWritten().Uint64(), tc.size)
			}
			if err := filestore.Discard(t.Context(), got); err != nil {
				t.Fatalf("Discard=%v", err)
			}
		})
	}
}
