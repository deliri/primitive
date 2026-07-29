package filestore_test

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// TestStagingEffectLayerTriad proves the staging seam on its own terms: one
// caller-named source becomes one exclusively created, chmodded, synchronized
// temporary, and the returned receipt is the only ownership token for it.
func TestStagingEffectLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive fragmented source becomes one synchronized temporary described exactly by its receipt", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		payload := deterministicPayload((32 << 10) + 3)
		staged, gotErr := filestore.Stage(t.Context(), filestore.StageRequest{
			Source:       iotest.HalfReader(bytes.NewReader(payload)),
			Temporary:    filestore.Location{Root: root, Path: mustRelativePath(t, ".stage")},
			Mode:         0o640,
			MaximumBytes: mustByteCount(t, uint64(len(payload))),
		})
		if gotErr != nil {
			t.Fatalf("Stage() error = %v, want nil", gotErr)
		}
		requireStagedReceiptDescribesDisk(t, rootDirectory, staged, payload, 0o640)
		requireDirectoryEntryNames(t, rootDirectory, []string{".stage"})
	})
	t.Run("negative terminal source failure after real bytes leaves no temporary and no receipt", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		if err := os.WriteFile(filepath.Join(rootDirectory, "sibling"), []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}
		payload := deterministicPayload(1 << 16)
		staged, gotErr := filestore.Stage(t.Context(), filestore.StageRequest{
			Source: io.MultiReader(
				bytes.NewReader(payload),
				iotest.ErrReader(io.ErrUnexpectedEOF),
			),
			Temporary:    filestore.Location{Root: root, Path: mustRelativePath(t, ".stage")},
			Mode:         0o600,
			MaximumBytes: mustByteCount(t, uint64(len(payload))+1),
		})
		if !errors.Is(gotErr, core.ErrFilestoreSource) ||
			!errors.Is(gotErr, io.ErrUnexpectedEOF) {
			t.Fatalf("Stage() error = %v, want %v and %v", gotErr, core.ErrFilestoreSource, io.ErrUnexpectedEOF)
		}
		if !errors.Is(staged.Validate(), core.ErrFilestoreContract) {
			t.Fatalf("StagedFile.Validate() after rejected stage = %v, want %v", staged.Validate(), core.ErrFilestoreContract)
		}
		if staged.BytesWritten().Uint64() != 0 {
			t.Fatalf("StagedFile.BytesWritten() after rejected stage = %d, want 0", staged.BytesWritten().Uint64())
		}
		requireDirectoryEntryNames(t, rootDirectory, []string{"sibling"})
	})
	t.Run("neutral empty source publishes one zero-byte temporary and activates no target name", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		staged, gotErr := filestore.Stage(t.Context(), filestore.StageRequest{
			Source:       bytes.NewReader(nil),
			Temporary:    filestore.Location{Root: root, Path: mustRelativePath(t, ".stage")},
			Mode:         0o600,
			MaximumBytes: mustByteCount(t, 1),
		})
		if gotErr != nil {
			t.Fatalf("Stage() error = %v, want nil", gotErr)
		}
		requireStagedReceiptDescribesDisk(t, rootDirectory, staged, []byte{}, 0o600)
		requireDirectoryEntryNames(t, rootDirectory, []string{".stage"})
	})
}

// TestStageRefusesAnAbsentTemporaryParentWithoutResidue pins that staging is a
// single exclusive create and never silently builds a directory chain the
// caller did not ask EnsureDirectory to build.
func TestStageRefusesAnAbsentTemporaryParentWithoutResidue(t *testing.T) {
	t.Parallel()

	rootDirectory := t.TempDir()
	root := requireTestRoot(t, rootDirectory)
	staged, gotErr := filestore.Stage(t.Context(), filestore.StageRequest{
		Source:       bytes.NewReader([]byte("candidate")),
		Temporary:    filestore.Location{Root: root, Path: mustRelativePath(t, filepath.Join("objects", ".stage"))},
		Mode:         0o600,
		MaximumBytes: mustByteCount(t, 9),
	})
	var pathErr *os.PathError
	if !errors.Is(gotErr, core.ErrFilestoreActivation) ||
		!errors.Is(gotErr, fs.ErrNotExist) ||
		!errors.As(gotErr, &pathErr) {
		t.Fatalf("Stage(absent parent) error = %v, want %v and %v and *os.PathError", gotErr, core.ErrFilestoreActivation, fs.ErrNotExist)
	}
	if !errors.Is(staged.Validate(), core.ErrFilestoreContract) {
		t.Fatalf("StagedFile.Validate() after absent parent = %v, want %v", staged.Validate(), core.ErrFilestoreContract)
	}
	requireDirectoryEntryNames(t, rootDirectory, nil)
}

// TestOpenAppendRefusesAnExistingDirectoryNameAndPreservesIt pins that the
// append reopen path never returns a handle for a name the OS cannot append
// to, and never disturbs the entry it refused.
func TestOpenAppendRefusesAnExistingDirectoryNameAndPreservesIt(t *testing.T) {
	t.Parallel()

	rootDirectory := t.TempDir()
	root := requireTestRoot(t, rootDirectory)
	if err := os.Mkdir(filepath.Join(rootDirectory, "ledger"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootDirectory, "ledger", "child"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, gotErr := filestore.OpenAppend(t.Context(), filestore.AppendRequest{
		Location: filestore.Location{Root: root, Path: mustRelativePath(t, "ledger")},
		Mode:     0o600,
		Append:   filestore.AppendExisting,
	})
	if file != nil {
		if closeErr := file.Close(); closeErr != nil {
			t.Errorf("refused append handle Close() error = %v, want nil", closeErr)
		}
		t.Fatalf("OpenAppend(directory name) handle = %v, want nil", file.Name())
	}
	if !errors.Is(gotErr, core.ErrFilestoreActivation) {
		t.Fatalf("OpenAppend(directory name) error = %v, want %v", gotErr, core.ErrFilestoreActivation)
	}
	got, err := os.ReadFile(filepath.Join(rootDirectory, "ledger", "child"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "keep" {
		t.Fatalf("preserved directory child bytes = %q, want %q", got, "keep")
	}
}

// requireStagedReceiptDescribesDisk proves the three facts that must agree for
// one stage: the receipt validates, the receipt's byte count matches the real
// file, and the named temporary on disk is a regular file with the exact
// requested permission mode and the exact streamed bytes.
func requireStagedReceiptDescribesDisk(
	t *testing.T,
	rootDirectory string,
	staged filestore.StagedFile,
	want []byte,
	wantMode fs.FileMode,
) {
	t.Helper()

	if err := staged.Validate(); err != nil {
		t.Fatalf("StagedFile.Validate() error = %v, want nil", err)
	}
	if staged.BytesWritten().Uint64() != uint64(len(want)) {
		t.Fatalf("StagedFile.BytesWritten() = %d, want %d", staged.BytesWritten().Uint64(), len(want))
	}
	path := filepath.Join(rootDirectory, staged.Path().String())
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("staged temporary mode = %v, want regular file", info.Mode())
	}
	if info.Mode().Perm() != wantMode {
		t.Fatalf("staged temporary permissions = %#o, want %#o", info.Mode().Perm(), wantMode)
	}
	if info.Size() != int64(len(want)) {
		t.Fatalf("staged temporary size = %d, want receipt byte count %d", info.Size(), len(want))
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("staged temporary byte length = %d, want exact source length %d", len(got), len(want))
	}
}
