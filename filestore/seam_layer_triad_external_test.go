package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

type writeObservation struct {
	calls int
	bytes int
	err   error
}

func (w *writeObservation) Write(data []byte) (int, error) {
	w.calls++
	count := len(data)
	if w.bytes >= 0 && w.bytes < count {
		count = w.bytes
	}
	return count, w.err
}

func TestTypedIngressLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive complete typed request reaches the real stage effect", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		staged, gotErr := filestore.Stage(t.Context(), filestore.StageRequest{
			Source:       bytes.NewReader([]byte("typed")),
			Temporary:    filestore.Location{Root: root, Path: mustRelativePath(t, ".stage")},
			Mode:         0o600,
			MaximumBytes: mustByteCount(t, 5),
		})
		if gotErr != nil || staged.BytesWritten().Uint64() != 5 {
			t.Fatalf("Stage(valid typed request) = bytes:%d error:%v, want 5/nil", staged.BytesWritten().Uint64(), gotErr)
		}
		requireDirectoryEntryNames(t, rootDirectory, []string{".stage"})
	})
	t.Run("negative zero request is rejected before any namespace effect", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		staged, gotErr := filestore.Stage(t.Context(), filestore.StageRequest{})
		if !errors.Is(gotErr, core.ErrFilestoreContract) {
			t.Fatalf("Stage(zero request) error = %v, want %v", gotErr, core.ErrFilestoreContract)
		}
		if !errors.Is(staged.Validate(), core.ErrFilestoreContract) {
			t.Fatalf("Stage(zero request) receipt Validate() = %v, want %v", staged.Validate(), core.ErrFilestoreContract)
		}
		requireDirectoryEntryNames(t, rootDirectory, nil)
	})
	t.Run("neutral minimal empty request admits no fabricated bytes or target", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		staged, gotErr := filestore.Stage(t.Context(), filestore.StageRequest{
			Source:       bytes.NewReader(nil),
			Temporary:    filestore.Location{Root: root, Path: mustRelativePath(t, ".stage")},
			Mode:         0o600,
			MaximumBytes: mustByteCount(t, 1),
		})
		if gotErr != nil || staged.BytesWritten().Uint64() != 0 {
			t.Fatalf("Stage(empty typed request) = bytes:%d error:%v, want 0/nil", staged.BytesWritten().Uint64(), gotErr)
		}
		requireDirectoryEntryNames(t, rootDirectory, []string{".stage"})
	})
}

func TestContextIngressLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive live context admits the requested effect", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		if err := os.WriteFile(filepath.Join(rootDirectory, "target"), []byte("remove"), 0o600); err != nil {
			t.Fatal(err)
		}
		gotErr := filestore.Remove(t.Context(), filestore.RemovalRequest{
			Location: filestore.Location{Root: root, Path: mustRelativePath(t, "target")},
		})
		if gotErr != nil {
			t.Fatalf("Remove(live context) error = %v, want nil", gotErr)
		}
		requireDirectoryEntryNames(t, rootDirectory, nil)
	})
	t.Run("negative terminal context is rejected before the requested effect", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		if err := os.WriteFile(filepath.Join(rootDirectory, "target"), []byte("keep"), 0o600); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		gotErr := filestore.Remove(ctx, filestore.RemovalRequest{
			Location: filestore.Location{Root: root, Path: mustRelativePath(t, "target")},
		})
		if !errors.Is(gotErr, context.Canceled) {
			t.Fatalf("Remove(cancelled context) error = %v, want %v", gotErr, context.Canceled)
		}
		requireDirectoryEntryNames(t, rootDirectory, []string{"target"})
	})
	t.Run("neutral live context admits an absent-name no-op without namespace noise", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		gotErr := filestore.Remove(t.Context(), filestore.RemovalRequest{
			Location: filestore.Location{Root: root, Path: mustRelativePath(t, "missing")},
		})
		if gotErr != nil {
			t.Fatalf("Remove(missing with live context) error = %v, want nil", gotErr)
		}
		requireDirectoryEntryNames(t, rootDirectory, nil)
	})
}

func TestRootedConfinementLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive ordinary local name reads only inside the root", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		if err := os.WriteFile(filepath.Join(rootDirectory, "target"), []byte("inside"), 0o600); err != nil {
			t.Fatal(err)
		}
		var destination bytes.Buffer
		gotCount, gotErr := filestore.Read(t.Context(), filestore.ReadRequest{
			Destination:  &destination,
			Location:     filestore.Location{Root: root, Path: mustRelativePath(t, "target")},
			MaximumBytes: mustByteCount(t, 6),
		})
		if gotErr != nil || gotCount.Uint64() != 6 || destination.String() != "inside" {
			t.Fatalf("Read(local) = count:%d bytes:%q error:%v, want 6/%q/nil", gotCount.Uint64(), destination.String(), gotErr, "inside")
		}
	})
	t.Run("negative escaping symlink cannot read or change the outside file", func(t *testing.T) {
		t.Parallel()

		parent := t.TempDir()
		rootDirectory := filepath.Join(parent, "root")
		outsideDirectory := filepath.Join(parent, "outside")
		if err := os.Mkdir(rootDirectory, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(outsideDirectory, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(outsideDirectory, "target"), []byte("outside"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outsideDirectory, filepath.Join(rootDirectory, "escape")); err != nil {
			t.Skipf("os.Symlink() unavailable: %v", err)
		}
		root := requireTestRoot(t, rootDirectory)
		var destination bytes.Buffer
		_, gotErr := filestore.Read(t.Context(), filestore.ReadRequest{
			Destination:  &destination,
			Location:     filestore.Location{Root: root, Path: mustRelativePath(t, filepath.Join("escape", "target"))},
			MaximumBytes: mustByteCount(t, 7),
		})
		if !errors.Is(gotErr, core.ErrFilestoreSource) {
			t.Fatalf("Read(escaping symlink) error = %v, want %v", gotErr, core.ErrFilestoreSource)
		}
		got, err := os.ReadFile(filepath.Join(outsideDirectory, "target"))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "outside" || destination.Len() != 0 {
			t.Fatalf("escape result = outside:%q destination:%q, want %q/empty", got, destination.Bytes(), "outside")
		}
	})
	t.Run("neutral in-root symlink resolves through os root without escaping", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		if err := os.WriteFile(filepath.Join(rootDirectory, "target"), []byte("inside"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("target", filepath.Join(rootDirectory, "alias")); err != nil {
			t.Skipf("os.Symlink() unavailable: %v", err)
		}
		var destination bytes.Buffer
		gotCount, gotErr := filestore.Read(t.Context(), filestore.ReadRequest{
			Destination:  &destination,
			Location:     filestore.Location{Root: root, Path: mustRelativePath(t, "alias")},
			MaximumBytes: mustByteCount(t, 6),
		})
		if gotErr != nil || gotCount.Uint64() != 6 || destination.String() != "inside" {
			t.Fatalf("Read(in-root symlink) = count:%d bytes:%q error:%v, want 6/%q/nil", gotCount.Uint64(), destination.String(), gotErr, "inside")
		}
	})
}

func TestDestinationStreamLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive caller writer receives every source byte and exact accounting", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		if err := os.WriteFile(filepath.Join(rootDirectory, "source"), []byte("stream"), 0o600); err != nil {
			t.Fatal(err)
		}
		var destination bytes.Buffer
		gotCount, gotErr := filestore.Read(t.Context(), filestore.ReadRequest{
			Destination:  &destination,
			Location:     filestore.Location{Root: root, Path: mustRelativePath(t, "source")},
			MaximumBytes: mustByteCount(t, 6),
		})
		if gotErr != nil || gotCount.Uint64() != 6 || destination.String() != "stream" {
			t.Fatalf("Read(destination) = count:%d bytes:%q error:%v, want 6/%q/nil", gotCount.Uint64(), destination.String(), gotErr, "stream")
		}
	})
	t.Run("negative partial writer error preserves written count and native identity", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		if err := os.WriteFile(filepath.Join(rootDirectory, "source"), []byte("stream"), 0o600); err != nil {
			t.Fatal(err)
		}
		destination := &writeObservation{bytes: 3, err: io.ErrClosedPipe}
		gotCount, gotErr := filestore.Read(t.Context(), filestore.ReadRequest{
			Destination:  destination,
			Location:     filestore.Location{Root: root, Path: mustRelativePath(t, "source")},
			MaximumBytes: mustByteCount(t, 6),
		})
		if !errors.Is(gotErr, core.ErrFilestoreDestination) ||
			!errors.Is(gotErr, io.ErrClosedPipe) {
			t.Fatalf("Read(partial destination) error = %v, want %v and %v", gotErr, core.ErrFilestoreDestination, io.ErrClosedPipe)
		}
		if gotCount.Uint64() != 3 || destination.calls != 1 {
			t.Fatalf("Read(partial destination) = count:%d calls:%d, want 3/1", gotCount.Uint64(), destination.calls)
		}
	})
	t.Run("neutral empty source never calls the destination writer", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		if err := os.WriteFile(filepath.Join(rootDirectory, "source"), nil, 0o600); err != nil {
			t.Fatal(err)
		}
		destination := &writeObservation{bytes: -1}
		gotCount, gotErr := filestore.Read(t.Context(), filestore.ReadRequest{
			Destination:  destination,
			Location:     filestore.Location{Root: root, Path: mustRelativePath(t, "source")},
			MaximumBytes: mustByteCount(t, 1),
		})
		if gotErr != nil || gotCount.Uint64() != 0 || destination.calls != 0 {
			t.Fatalf("Read(empty destination) = count:%d calls:%d error:%v, want 0/0/nil", gotCount.Uint64(), destination.calls, gotErr)
		}
	})
}

func TestReceiptIdentityLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive current staged identity activates the exact receipt bytes", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		staged := mustStage(t, root, ".stage", "candidate")
		gotErr := filestore.Commit(t.Context(), filestore.CommitRequest{
			Staged: staged, Target: mustRelativePath(t, "target"), Install: filestore.InstallCreate,
		})
		if gotErr != nil {
			t.Fatalf("Commit(current receipt) error = %v, want nil", gotErr)
		}
		requireOptionalFile(t, filepath.Join(rootDirectory, "target"), "candidate")
	})
	t.Run("negative foreign file at the receipt name is rejected and preserved", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		staged := mustStage(t, root, ".stage", "candidate")
		if err := os.Remove(filepath.Join(rootDirectory, ".stage")); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(rootDirectory, ".stage"), []byte("foreign"), 0o600); err != nil {
			t.Fatal(err)
		}
		gotErr := filestore.Commit(t.Context(), filestore.CommitRequest{
			Staged: staged, Target: mustRelativePath(t, "target"), Install: filestore.InstallCreate,
		})
		if !errors.Is(gotErr, core.ErrFilestoreActivationIndeterminate) {
			t.Fatalf("Commit(foreign receipt name) error = %v, want %v", gotErr, core.ErrFilestoreActivationIndeterminate)
		}
		requireOptionalFile(t, filepath.Join(rootDirectory, ".stage"), "foreign")
	})
	t.Run("neutral zero receipt is rejected without observing or changing the namespace", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		gotErr := filestore.Commit(t.Context(), filestore.CommitRequest{
			Target: mustRelativePath(t, "target"), Install: filestore.InstallCreate,
		})
		if !errors.Is(gotErr, core.ErrFilestoreContract) {
			t.Fatalf("Commit(zero receipt) error = %v, want %v", gotErr, core.ErrFilestoreContract)
		}
		requireDirectoryEntryNames(t, rootDirectory, nil)
	})
}

func TestOpenAppendLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive absent name creates an exact-mode real append handle", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		file, gotErr := filestore.OpenAppend(t.Context(), filestore.AppendRequest{
			Location: filestore.Location{Root: root, Path: mustRelativePath(t, "ledger")},
			Mode:     0o640,
		})
		if gotErr != nil {
			t.Fatalf("OpenAppend(absent) error = %v, want nil", gotErr)
		}
		if _, err := file.Write([]byte("entry\n")); err != nil {
			t.Fatal(err)
		}
		if err := file.Sync(); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(filepath.Join(rootDirectory, "ledger"))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o640 {
			t.Fatalf("created append mode = %#o, want %#o", info.Mode().Perm(), os.FileMode(0o640))
		}
		requireOptionalFile(t, filepath.Join(rootDirectory, "ledger"), "entry\n")
	})
	t.Run("negative existing directory is rejected and preserved", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		if err := os.Mkdir(filepath.Join(rootDirectory, "ledger"), 0o700); err != nil {
			t.Fatal(err)
		}
		file, gotErr := filestore.OpenAppend(t.Context(), filestore.AppendRequest{
			Location: filestore.Location{Root: root, Path: mustRelativePath(t, "ledger")},
			Mode:     0o600,
		})
		if file != nil {
			_ = file.Close()
			t.Fatalf("OpenAppend(directory) handle = %v, want nil", file)
		}
		if !errors.Is(gotErr, core.ErrFilestoreActivation) {
			t.Fatalf("OpenAppend(directory) error = %v, want %v", gotErr, core.ErrFilestoreActivation)
		}
		info, err := os.Stat(filepath.Join(rootDirectory, "ledger"))
		if err != nil || !info.IsDir() {
			t.Fatalf("preserved ledger = info:%v error:%v, want directory/nil", info, err)
		}
	})
	t.Run("neutral existing regular file reopens without writing or truncating", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		if err := os.WriteFile(filepath.Join(rootDirectory, "ledger"), []byte("existing\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		file, gotErr := filestore.OpenAppend(t.Context(), filestore.AppendRequest{
			Location: filestore.Location{Root: root, Path: mustRelativePath(t, "ledger")},
			Mode:     0o640,
		})
		if gotErr != nil {
			t.Fatalf("OpenAppend(existing) error = %v, want nil", gotErr)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		requireOptionalFile(t, filepath.Join(rootDirectory, "ledger"), "existing\n")
	})
}

func TestRotateAppendLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive outgoing handle is synchronized and closed before incoming creation", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		outgoing, err := filestore.OpenAppend(t.Context(), filestore.AppendRequest{
			Location: filestore.Location{Root: root, Path: mustRelativePath(t, "current")},
			Mode:     0o600,
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := outgoing.Write([]byte("current\n")); err != nil {
			t.Fatal(err)
		}
		incoming, gotErr := filestore.RotateAppend(t.Context(), filestore.RotationRequest{
			Outgoing: outgoing,
			Incoming: filestore.AppendRequest{
				Location: filestore.Location{Root: root, Path: mustRelativePath(t, "next")},
				Mode:     0o600,
			},
		})
		if gotErr != nil {
			t.Fatalf("RotateAppend() error = %v, want nil", gotErr)
		}
		if _, err := outgoing.Write([]byte("closed")); !errors.Is(err, os.ErrClosed) {
			t.Fatalf("outgoing Write() error = %v, want %v", err, os.ErrClosed)
		}
		if _, err := incoming.Write([]byte("next\n")); err != nil {
			t.Fatal(err)
		}
		if err := incoming.Sync(); err != nil {
			t.Fatal(err)
		}
		if err := incoming.Close(); err != nil {
			t.Fatal(err)
		}
		requireOptionalFile(t, filepath.Join(rootDirectory, "current"), "current\n")
		requireOptionalFile(t, filepath.Join(rootDirectory, "next"), "next\n")
	})
	t.Run("negative occupied incoming name closes outgoing and preserves the occupant", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		if err := os.WriteFile(filepath.Join(rootDirectory, "next"), []byte("occupied"), 0o600); err != nil {
			t.Fatal(err)
		}
		outgoing, err := filestore.OpenAppend(t.Context(), filestore.AppendRequest{
			Location: filestore.Location{Root: root, Path: mustRelativePath(t, "current")},
			Mode:     0o600,
		})
		if err != nil {
			t.Fatal(err)
		}
		incoming, gotErr := filestore.RotateAppend(t.Context(), filestore.RotationRequest{
			Outgoing: outgoing,
			Incoming: filestore.AppendRequest{
				Location: filestore.Location{Root: root, Path: mustRelativePath(t, "next")},
				Mode:     0o600,
			},
		})
		if incoming != nil {
			_ = incoming.Close()
			t.Fatalf("RotateAppend(conflict) handle = %v, want nil", incoming)
		}
		if !errors.Is(gotErr, core.ErrFilestoreConflict) ||
			!errors.Is(gotErr, os.ErrExist) {
			t.Fatalf("RotateAppend(conflict) error = %v, want %v and %v", gotErr, core.ErrFilestoreConflict, os.ErrExist)
		}
		if _, err := outgoing.Write([]byte("closed")); !errors.Is(err, os.ErrClosed) {
			t.Fatalf("outgoing Write() error = %v, want %v", err, os.ErrClosed)
		}
		requireOptionalFile(t, filepath.Join(rootDirectory, "next"), "occupied")
	})
	t.Run("neutral empty outgoing rotates to one exact empty incoming generation", func(t *testing.T) {
		t.Parallel()

		rootDirectory := t.TempDir()
		root := requireTestRoot(t, rootDirectory)
		outgoing, err := filestore.OpenAppend(t.Context(), filestore.AppendRequest{
			Location: filestore.Location{Root: root, Path: mustRelativePath(t, "current")},
			Mode:     0o600,
		})
		if err != nil {
			t.Fatal(err)
		}
		incoming, gotErr := filestore.RotateAppend(t.Context(), filestore.RotationRequest{
			Outgoing: outgoing,
			Incoming: filestore.AppendRequest{
				Location: filestore.Location{Root: root, Path: mustRelativePath(t, "next")},
				Mode:     0o600,
			},
		})
		if gotErr != nil {
			t.Fatalf("RotateAppend(empty) error = %v, want nil", gotErr)
		}
		if err := incoming.Close(); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"current", "next"} {
			info, err := os.Stat(filepath.Join(rootDirectory, name))
			if err != nil {
				t.Fatal(err)
			}
			if !info.Mode().IsRegular() || info.Size() != 0 {
				t.Fatalf("%s = mode:%v size:%d, want regular/0", name, info.Mode(), info.Size())
			}
		}
		requireDirectoryEntryNames(t, rootDirectory, []string{"current", "next"})
	})
}
