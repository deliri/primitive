package filestore_test

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

const retiredTransferFixtureBytes uint64 = 64
const transferPatternByte byte = 0xa5

type transferExtentDoor uint8

const (
	transferExtentRead transferExtentDoor = iota
	transferExtentStage
	transferExtentWrite
)

type generatedTransferPattern struct{}

func (generatedTransferPattern) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = transferPatternByte
	}
	return len(p), nil
}

type observedTransferPattern struct {
	count   int64
	changed bool
}

func (w *observedTransferPattern) Write(p []byte) (int, error) {
	w.count += int64(len(p))
	w.changed = w.changed || bytes.Count(p, []byte{transferPatternByte}) != len(p)
	return len(p), nil
}

func TestTransferContinuesBeyondFormerExtentLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		extent int64
	}{
		{name: "empty stream remains an exact empty effect"},
		{name: "below former ceiling remains exact", extent: int64(retiredTransferFixtureBytes) - 1},
		{name: "at former ceiling remains exact", extent: int64(retiredTransferFixtureBytes)},
		{name: "above former ceiling continues", extent: int64(retiredTransferFixtureBytes) + 1},
		{name: "below ordinary Go copy window continues", extent: (32 << 10) - 1},
		{name: "at ordinary Go copy window continues", extent: 32 << 10},
		{name: "above ordinary Go copy window continues", extent: (32 << 10) + 1},
		{name: "many windows and a partial tail continue", extent: (1 << 20) + 1},
	}
	operations := []struct {
		name string
		door transferExtentDoor
	}{
		{name: "read", door: transferExtentRead},
		{name: "stage", door: transferExtentStage},
		{name: "write", door: transferExtentWrite},
	}
	for _, operation := range operations {
		for _, tc := range cases {
			t.Run(operation.name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				directory := t.TempDir()
				root := requireTestRoot(t, directory)
				target := mustRelativePath(t, "target")
				temporary := mustRelativePath(t, "stage")
				location := filestore.Location{Root: root, Path: target}
				source := io.LimitReader(generatedTransferPattern{}, tc.extent)
				switch operation.door {
				case transferExtentRead:
					file, err := root.OpenFile(target.String(), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
					if err != nil {
						t.Fatalf("OpenFile fixture = %v, want nil", err)
					}
					count, copyErr := io.Copy(file, source)
					closeErr := file.Close()
					if count != tc.extent || copyErr != nil || closeErr != nil {
						t.Fatalf("fixture = (%d,%v,%v), want (%d,nil,nil)", count, copyErr, closeErr, tc.extent)
					}
				case transferExtentStage:
					staged, err := filestore.Stage(t.Context(), filestore.StageRequest{Source: source, Temporary: filestore.Location{Root: root, Path: temporary}, Mode: 0o600})
					if err != nil || staged.Validate() != nil || staged.BytesWritten().Uint64() != uint64(tc.extent) {
						t.Fatalf("Stage = (%v,%v), want valid exact %d-byte receipt", staged, err, tc.extent)
					}
					location.Path = staged.Path()
				case transferExtentWrite:
					recovery, err := filestore.Write(t.Context(), filestore.WriteRequest{Source: source, Location: location, Temporary: temporary, Mode: 0o600, Install: filestore.InstallCreate})
					if err != nil || recovery != (filestore.CommitRequest{}) {
						t.Fatalf("Write = (%v,%v), want no recovery and nil for %d bytes", recovery, err, tc.extent)
					}
				default:
					t.Fatalf("door = %v, want declared transfer operation", operation.door)
				}
				var destination observedTransferPattern
				count, err := filestore.Read(t.Context(), filestore.ReadRequest{Location: location, Destination: &destination})
				if err != nil || count.Uint64() != uint64(tc.extent) || destination.count != tc.extent || destination.changed {
					t.Fatalf("Read = (%d,%v,%d observed,changed %t), want %d exact bytes without extent refusal", count.Uint64(), err, destination.count, destination.changed, tc.extent)
				}
			})
		}
	}
}

func TestTransferRejectsTypedNilOwnersLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		source      io.Reader
		destination io.Writer
		door        transferExtentDoor
	}{
		{name: "read rejects typed nil buffer", destination: (*bytes.Buffer)(nil), door: transferExtentRead},
		{name: "read rejects typed nil file", destination: (*os.File)(nil), door: transferExtentRead},
		{name: "stage rejects typed nil reader", source: (*bytes.Reader)(nil), door: transferExtentStage},
		{name: "stage rejects typed nil buffer", source: (*bytes.Buffer)(nil), door: transferExtentStage},
		{name: "stage rejects typed nil file", source: (*os.File)(nil), door: transferExtentStage},
		{name: "write rejects typed nil reader", source: (*bytes.Reader)(nil), door: transferExtentWrite},
		{name: "write rejects typed nil file", source: (*os.File)(nil), door: transferExtentWrite},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			root := requireTestRoot(t, directory)
			location := filestore.Location{Root: root, Path: mustRelativePath(t, "target")}
			var validationErr, executionErr error
			switch tc.door {
			case transferExtentRead:
				request := filestore.ReadRequest{Destination: tc.destination, Location: location}
				validationErr = request.Validate()
				count, err := filestore.Read(t.Context(), request)
				executionErr = err
				if count != (core.ByteLength{}) {
					t.Fatalf("Read typed nil count = %v, want zero", count)
				}
			case transferExtentStage:
				request := filestore.StageRequest{Source: tc.source, Temporary: location, Mode: 0o600}
				validationErr = request.Validate()
				staged, err := filestore.Stage(t.Context(), request)
				executionErr = err
				if staged != (filestore.StagedFile{}) {
					t.Fatalf("Stage typed nil receipt = %v, want zero", staged)
				}
			case transferExtentWrite:
				request := filestore.WriteRequest{Source: tc.source, Location: location, Temporary: mustRelativePath(t, "stage"), Mode: 0o600, Install: filestore.InstallCreate}
				validationErr = request.Validate()
				recovery, err := filestore.Write(t.Context(), request)
				executionErr = err
				if recovery != (filestore.CommitRequest{}) {
					t.Fatalf("Write typed nil recovery = %v, want zero", recovery)
				}
			default:
				t.Fatalf("door = %v, want declared transfer operation", tc.door)
			}
			if !errors.Is(validationErr, core.ErrFilestoreContract) || !errors.Is(executionErr, core.ErrFilestoreContract) {
				t.Fatalf("typed nil validation/execution = (%v,%v), want (%v,%v)", validationErr, executionErr, core.ErrFilestoreContract, core.ErrFilestoreContract)
			}
			for _, err := range []error{validationErr, executionErr} {
				var pathErr *fs.PathError
				if errors.Is(err, core.ErrFilestoreSource) || errors.Is(err, core.ErrFilestoreDestination) || errors.Is(err, core.ErrFilestoreActivation) || errors.As(err, &pathErr) {
					t.Fatalf("nil owner crossed validation into an effect: %v", err)
				}
			}
			entries, readErr := os.ReadDir(directory)
			if readErr != nil || len(entries) != 0 {
				t.Fatalf("validation namespace = (%v,%v), want empty and nil", entries, readErr)
			}
		})
	}
}
