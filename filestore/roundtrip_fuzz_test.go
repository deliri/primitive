package filestore_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

const filestoreFuzzPayloadMaximum = 1 << 20

func FuzzWriteReadRoundTrip(f *testing.F) {
	for _, seed := range [][]byte{
		nil,
		{},
		{0},
		{0xff},
		[]byte("ledger\n"),
		deterministicPayload(255),
		deterministicPayload(256),
		deterministicPayload(257),
		deterministicPayload(4095),
		deterministicPayload(4096),
		deterministicPayload(4097),
		deterministicPayload(filestoreFuzzPayloadMaximum + 1),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, payload []byte) {
		if len(payload) > filestoreFuzzPayloadMaximum {
			proveWriteRefusesOversizedFuzzPayload(t, payload)
			return
		}
		rootDirectory := t.TempDir()
		root, err := os.OpenRoot(rootDirectory)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if closeErr := root.Close(); closeErr != nil {
				t.Errorf("os.Root.Close() error = %v, want nil", closeErr)
			}
		}()
		maximum := mustByteCount(t, uint64(max(len(payload), 1)))
		_, gotErr := filestore.Write(t.Context(), filestore.WriteRequest{
			Source:       bytes.NewReader(payload),
			Location:     filestore.Location{Root: root, Path: mustRelativePath(t, "target")},
			Temporary:    mustRelativePath(t, ".stage"),
			Mode:         0o600,
			Install:      filestore.InstallCreate,
			MaximumBytes: maximum,
		})
		if gotErr != nil {
			t.Fatalf("Write(%d fuzz bytes) error = %v, want nil", len(payload), gotErr)
		}
		stored, err := os.ReadFile(filepath.Join(rootDirectory, "target"))
		if err != nil {
			t.Fatalf("os.ReadFile(target) error = %v, want nil", err)
		}
		if !bytes.Equal(stored, payload) {
			t.Fatalf("OS-visible target bytes = %d, want %d exact fuzz bytes", len(stored), len(payload))
		}
		var destination bytes.Buffer
		gotCount, gotErr := filestore.Read(t.Context(), filestore.ReadRequest{
			Destination:  &destination,
			Location:     filestore.Location{Root: root, Path: mustRelativePath(t, "target")},
			MaximumBytes: maximum,
		})
		if gotErr != nil {
			t.Fatalf("Read(%d fuzz bytes) error = %v, want nil", len(payload), gotErr)
		}
		if gotCount.Uint64() != uint64(len(payload)) ||
			!bytes.Equal(destination.Bytes(), payload) {
			t.Fatalf("round trip = count:%d bytes:%d, want %d exact bytes", gotCount.Uint64(), destination.Len(), len(payload))
		}
	})
}

func FuzzStageCommitRoundTrip(f *testing.F) {
	for _, seed := range [][]byte{
		nil,
		{},
		{0},
		[]byte("target-late"),
		deterministicPayload(255),
		deterministicPayload(256),
		deterministicPayload(257),
		deterministicPayload(65535),
		deterministicPayload(65536),
		deterministicPayload(65537),
		deterministicPayload(filestoreFuzzPayloadMaximum + 1),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, payload []byte) {
		if len(payload) > filestoreFuzzPayloadMaximum {
			proveStageRefusesOversizedFuzzPayload(t, payload)
			return
		}
		rootDirectory := t.TempDir()
		root, err := os.OpenRoot(rootDirectory)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if closeErr := root.Close(); closeErr != nil {
				t.Errorf("os.Root.Close() error = %v, want nil", closeErr)
			}
		}()
		staged, gotErr := filestore.Stage(t.Context(), filestore.StageRequest{
			Source:       bytes.NewReader(payload),
			Temporary:    filestore.Location{Root: root, Path: mustRelativePath(t, ".stage")},
			Mode:         0o600,
			MaximumBytes: mustByteCount(t, uint64(max(len(payload), 1))),
		})
		if gotErr != nil {
			t.Fatalf("Stage(%d fuzz bytes) error = %v, want nil", len(payload), gotErr)
		}
		gotErr = filestore.Commit(t.Context(), filestore.CommitRequest{
			Staged:  staged,
			Target:  mustRelativePath(t, "target"),
			Install: filestore.InstallCreate,
		})
		if gotErr != nil {
			t.Fatalf("Commit(%d fuzz bytes) error = %v, want nil", len(payload), gotErr)
		}
		got, err := os.ReadFile(filepath.Join(rootDirectory, "target"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, payload) {
			t.Fatalf("stage/commit round trip bytes = %d, want %d exact bytes", len(got), len(payload))
		}
	})
}

func proveWriteRefusesOversizedFuzzPayload(t *testing.T, payload []byte) {
	t.Helper()

	rootDirectory := t.TempDir()
	root, err := os.OpenRoot(rootDirectory)
	if err != nil {
		t.Fatalf("os.OpenRoot() error = %v, want nil", err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Errorf("os.Root.Close() error = %v, want nil", closeErr)
		}
	}()
	maximum := mustByteCount(t, filestoreFuzzPayloadMaximum)
	gotRecovery, gotErr := filestore.Write(t.Context(), filestore.WriteRequest{
		Source:       bytes.NewReader(payload[:filestoreFuzzPayloadMaximum+1]),
		Location:     filestore.Location{Root: root, Path: mustRelativePath(t, "target")},
		Temporary:    mustRelativePath(t, ".stage"),
		Mode:         0o600,
		Install:      filestore.InstallCreate,
		MaximumBytes: maximum,
	})
	if !errors.Is(gotErr, core.ErrFilestoreSize) {
		t.Fatalf("Write(maximum+1 fuzz bytes) error = %v, want %v", gotErr, core.ErrFilestoreSize)
	}
	if !errors.Is(gotRecovery.Validate(), core.ErrFilestoreContract) {
		t.Fatalf(
			"Write(maximum+1 fuzz bytes) recovery validation = %v, want %v",
			gotRecovery.Validate(),
			core.ErrFilestoreContract,
		)
	}
	proveFuzzPathAbsent(t, filepath.Join(rootDirectory, "target"))
	proveFuzzPathAbsent(t, filepath.Join(rootDirectory, ".stage"))
}

func proveStageRefusesOversizedFuzzPayload(t *testing.T, payload []byte) {
	t.Helper()

	rootDirectory := t.TempDir()
	root, err := os.OpenRoot(rootDirectory)
	if err != nil {
		t.Fatalf("os.OpenRoot() error = %v, want nil", err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Errorf("os.Root.Close() error = %v, want nil", closeErr)
		}
	}()
	gotStaged, gotErr := filestore.Stage(t.Context(), filestore.StageRequest{
		Source:       bytes.NewReader(payload[:filestoreFuzzPayloadMaximum+1]),
		Temporary:    filestore.Location{Root: root, Path: mustRelativePath(t, ".stage")},
		Mode:         0o600,
		MaximumBytes: mustByteCount(t, filestoreFuzzPayloadMaximum),
	})
	if !errors.Is(gotErr, core.ErrFilestoreSize) {
		t.Fatalf("Stage(maximum+1 fuzz bytes) error = %v, want %v", gotErr, core.ErrFilestoreSize)
	}
	if !errors.Is(gotStaged.Validate(), core.ErrFilestoreContract) {
		t.Fatalf(
			"Stage(maximum+1 fuzz bytes) staged validation = %v, want %v",
			gotStaged.Validate(),
			core.ErrFilestoreContract,
		)
	}
	proveFuzzPathAbsent(t, filepath.Join(rootDirectory, ".stage"))
}

func proveFuzzPathAbsent(t *testing.T, path string) {
	t.Helper()

	_, gotErr := os.Stat(path)
	if !errors.Is(gotErr, fs.ErrNotExist) {
		t.Fatalf("os.Stat(%q) error = %v, want %v", path, gotErr, fs.ErrNotExist)
	}
}
