package filestore_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestStreamExtentGoSizeDomainLayerTriad(t *testing.T) {
	t.Parallel()
	type streamExtentDoor uint8
	const (
		streamExtentRead streamExtentDoor = iota
		streamExtentStage
		streamExtentWrite
	)
	for _, tc := range []struct {
		name    string
		maximum uint64
		wantErr error
	}{
		{name: "zero count cannot open or create a file", wantErr: core.ErrPrimitiveContract},
		{name: "smallest positive ceiling admits empty content", maximum: 1},
		{name: "one below Go size ceiling admits empty content", maximum: math.MaxInt64 - 1},
		{name: "exact Go size ceiling admits empty content", maximum: math.MaxInt64},
		{name: "one above Go size ceiling cannot promise an unrepresentable receipt", maximum: uint64(math.MaxInt64) + 1, wantErr: core.ErrNumericOverflow},
		{name: "unsigned maximum cannot wrap to a negative Go limit", maximum: math.MaxUint64, wantErr: core.ErrNumericOverflow},
	} {
		for _, operation := range []struct {
			name string
			door streamExtentDoor
		}{
			{name: "read", door: streamExtentRead},
			{name: "stage", door: streamExtentStage},
			{name: "write", door: streamExtentWrite},
		} {
			t.Run(fmt.Sprintf("%s/%s", operation.name, tc.name), func(t *testing.T) {
				t.Parallel()
				directory := t.TempDir()
				root := requireTestRoot(t, directory)
				source, err := root.OpenFile("source", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
				if err != nil {
					t.Fatal(err)
				}
				if err := source.Close(); err != nil {
					t.Fatal(err)
				}
				var maximum core.ByteCount
				if tc.maximum > 0 {
					maximum, err = core.NewByteCount(tc.maximum)
					if err != nil {
						t.Fatal(err)
					}
				}
				var validation, gotErr error
				var gotCount core.ByteLength
				var gotStage filestore.StagedFile
				var gotRecovery filestore.CommitRequest
				switch operation.door {
				case streamExtentRead:
					request := filestore.ReadRequest{Destination: io.Discard, Location: filestore.Location{Root: root, Path: mustRelativePath(t, "source")}, MaximumBytes: maximum}
					validation = request.Validate()
					gotCount, gotErr = filestore.Read(t.Context(), request)
				case streamExtentStage:
					request := filestore.StageRequest{Source: bytes.NewReader(nil), Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, "stage")}, Mode: 0o600, MaximumBytes: maximum}
					validation = request.Validate()
					gotStage, gotErr = filestore.Stage(t.Context(), request)
				case streamExtentWrite:
					request := filestore.WriteRequest{Source: bytes.NewReader(nil), Location: filestore.Location{Root: root, Path: mustRelativePath(t, "target")}, Temporary: mustRelativePath(t, "stage"), Mode: 0o600, Install: filestore.InstallCreate, MaximumBytes: maximum}
					validation = request.Validate()
					gotRecovery, gotErr = filestore.Write(t.Context(), request)
				default:
					t.Fatalf("operation = %v, want a declared stream boundary", operation.door)
				}
				if gotCount.Uint64() != 0 || gotRecovery != (filestore.CommitRequest{}) {
					t.Fatalf("receipts = (%v,%v), want empty read and no recovery", gotCount, gotRecovery)
				}
				if operation.door == streamExtentRead && tc.wantErr != nil && gotCount != (core.ByteLength{}) {
					t.Fatalf("refused read = %v, want zero receipt", gotCount)
				}
				if operation.door == streamExtentStage && tc.wantErr == nil {
					if gotStage.Validate() != nil || gotStage.Path() != mustRelativePath(t, "stage") || gotStage.BytesWritten().Uint64() != 0 {
						t.Fatalf("stage = %v, want valid exact empty stage", gotStage)
					}
				} else if gotStage != (filestore.StagedFile{}) {
					t.Fatalf("refused stage = %v, want zero receipt", gotStage)
				}
				for _, got := range []error{validation, gotErr} {
					for _, class := range []error{core.ErrFilestoreSource, core.ErrFilestoreDestination, core.ErrFilestoreActivation, core.ErrFilestoreCleanup, core.ErrFilestoreConflict, core.ErrFilestoreSize} {
						if errors.Is(got, class) {
							t.Fatalf("admission = %v, want no %v effect classification", got, class)
						}
					}
					if (got == nil) != (tc.wantErr == nil) || tc.wantErr != nil && (!errors.Is(got, tc.wantErr) || !errors.Is(got, core.ErrFilestoreContract)) {
						t.Fatalf("validation/effect = (%v,%v), want filestore contract/%v", validation, gotErr, tc.wantErr)
					}
				}
				entries, err := os.ReadDir(directory)
				if err != nil {
					t.Fatal(err)
				}
				wantEntries := 1
				if tc.wantErr == nil && operation.door != streamExtentRead {
					wantEntries++
				}
				if len(entries) != wantEntries {
					t.Fatalf("directory entries = %v, want %d", entries, wantEntries)
				}
				for _, entry := range entries {
					wantName := "source"
					if entry.Name() != wantName && tc.wantErr == nil {
						switch operation.door {
						case streamExtentStage:
							wantName = "stage"
						case streamExtentWrite:
							wantName = "target"
						}
					}
					if entry.Name() != wantName {
						t.Fatalf("entry = %q, want %q", entry.Name(), wantName)
					}
					info, err := entry.Info()
					if err != nil {
						t.Fatal(err)
					}
					if !info.Mode().IsRegular() || info.Size() != 0 {
						t.Fatalf("entry %s = (%v,%d), want empty regular file", entry.Name(), info.Mode(), info.Size())
					}
				}
			})
		}
	}
}
