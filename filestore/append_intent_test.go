package filestore

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// TestAppendIntentLayerTriad proves that callers select namespace behavior
// explicitly instead of inheriting create-or-open behavior from disk timing.
func TestAppendIntentLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive exclusive create followed by existing-only append preserves both records", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		root, err := os.OpenRoot(dir)
		if err != nil {
			t.Fatalf("OpenRoot() error = %v", err)
		}
		t.Cleanup(func() { _ = root.Close() })
		path := mustAppendIntentPath(t, "ledger.0.jsonl")

		created, err := OpenAppend(t.Context(), AppendRequest{
			Location: Location{Root: root, Path: path},
			Mode:     0o600,
			Append:   AppendCreate,
		})
		if err != nil {
			t.Fatalf("OpenAppend(AppendCreate) error = %v, want nil", err)
		}
		if _, err := created.Write([]byte("first\n")); err != nil {
			t.Fatalf("created Write() error = %v", err)
		}
		if err := created.Close(); err != nil {
			t.Fatalf("created Close() error = %v", err)
		}

		existing, err := OpenAppend(t.Context(), AppendRequest{
			Location: Location{Root: root, Path: path},
			Mode:     0o600,
			Append:   AppendExisting,
		})
		if err != nil {
			t.Fatalf("OpenAppend(AppendExisting) error = %v, want nil", err)
		}
		if _, err := existing.Write([]byte("second\n")); err != nil {
			t.Fatalf("existing Write() error = %v", err)
		}
		if err := existing.Close(); err != nil {
			t.Fatalf("existing Close() error = %v", err)
		}

		got, err := os.ReadFile(filepath.Join(dir, path.String()))
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if string(got) != "first\nsecond\n" {
			t.Fatalf("append bytes = %q, want %q", got, "first\nsecond\n")
		}
	})

	t.Run("negative incompatible and invalid intents fail without namespace residue", func(t *testing.T) {
		t.Parallel()

		absentDir := t.TempDir()
		absentRoot, err := os.OpenRoot(absentDir)
		if err != nil {
			t.Fatalf("OpenRoot(absent) error = %v", err)
		}
		t.Cleanup(func() { _ = absentRoot.Close() })
		absentPath := mustAppendIntentPath(t, "missing")
		file, gotErr := OpenAppend(t.Context(), AppendRequest{
			Location: Location{Root: absentRoot, Path: absentPath},
			Mode:     0o600,
			Append:   AppendExisting,
		})
		if file != nil {
			_ = file.Close()
			t.Fatalf("OpenAppend(AppendExisting absent) handle = %v, want nil", file)
		}
		if !errors.Is(gotErr, core.ErrFilestoreActivation) ||
			!errors.Is(gotErr, fs.ErrNotExist) {
			t.Fatalf("OpenAppend(AppendExisting absent) error = %v, want %v and %v",
				gotErr, core.ErrFilestoreActivation, fs.ErrNotExist)
		}
		if _, err := os.Stat(filepath.Join(absentDir, absentPath.String())); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("AppendExisting absent target stat error = %v, want %v", err, fs.ErrNotExist)
		}

		occupiedDir := t.TempDir()
		occupiedRoot, err := os.OpenRoot(occupiedDir)
		if err != nil {
			t.Fatalf("OpenRoot(occupied) error = %v", err)
		}
		t.Cleanup(func() { _ = occupiedRoot.Close() })
		occupiedPath := mustAppendIntentPath(t, "occupied")
		if err := os.WriteFile(filepath.Join(occupiedDir, occupiedPath.String()), []byte("keep"), 0o600); err != nil {
			t.Fatalf("seed occupied target: %v", err)
		}
		file, gotErr = OpenAppend(t.Context(), AppendRequest{
			Location: Location{Root: occupiedRoot, Path: occupiedPath},
			Mode:     0o600,
			Append:   AppendCreate,
		})
		if file != nil {
			_ = file.Close()
			t.Fatalf("OpenAppend(AppendCreate occupied) handle = %v, want nil", file)
		}
		if !errors.Is(gotErr, core.ErrFilestoreConflict) ||
			!errors.Is(gotErr, fs.ErrExist) {
			t.Fatalf("OpenAppend(AppendCreate occupied) error = %v, want %v and %v",
				gotErr, core.ErrFilestoreConflict, fs.ErrExist)
		}
		got, err := os.ReadFile(filepath.Join(occupiedDir, occupiedPath.String()))
		if err != nil {
			t.Fatalf("ReadFile(occupied) error = %v", err)
		}
		if string(got) != "keep" {
			t.Fatalf("occupied bytes = %q, want %q", got, "keep")
		}

		for _, mode := range []AppendMode{AppendUnknown, AppendMode(200)} {
			if err := mode.Validate(); !errors.Is(err, core.ErrFilestoreContract) {
				t.Errorf("AppendMode(%d).Validate() error = %v, want %v",
					mode, err, core.ErrFilestoreContract)
			}
		}
		for _, mode := range []AppendMode{AppendCreate, AppendExisting, AppendCreateOrOpen} {
			if err := mode.Validate(); err != nil {
				t.Errorf("AppendMode(%d).Validate() error = %v, want nil", mode, err)
			}
		}
	})

	t.Run("neutral create-or-open leaves an existing file untouched when no bytes are written", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		root, err := os.OpenRoot(dir)
		if err != nil {
			t.Fatalf("OpenRoot() error = %v", err)
		}
		t.Cleanup(func() { _ = root.Close() })
		path := mustAppendIntentPath(t, "ordinary.log")
		hostPath := filepath.Join(dir, path.String())
		if err := os.WriteFile(hostPath, []byte("prefix\n"), 0o600); err != nil {
			t.Fatalf("seed existing target: %v", err)
		}
		before, err := os.Stat(hostPath)
		if err != nil {
			t.Fatalf("Stat(before) error = %v", err)
		}

		file, err := OpenAppend(t.Context(), AppendRequest{
			Location: Location{Root: root, Path: path},
			Mode:     0o640,
			Append:   AppendCreateOrOpen,
		})
		if err != nil {
			t.Fatalf("OpenAppend(AppendCreateOrOpen) error = %v, want nil", err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		after, err := os.Stat(hostPath)
		if err != nil {
			t.Fatalf("Stat(after) error = %v", err)
		}
		if !os.SameFile(before, after) {
			t.Fatalf("os.SameFile(before, after) = false, want true; create-or-open " +
				"must preserve the existing OS identity")
		}
		got, err := os.ReadFile(hostPath)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if string(got) != "prefix\n" {
			t.Fatalf("neutral bytes = %q, want %q", got, "prefix\n")
		}
		if after.Mode().Perm() != 0o600 {
			t.Fatalf("neutral mode = %#o, want existing mode %#o", after.Mode().Perm(), os.FileMode(0o600))
		}
	})
}

// TestRotateAppendValidationOwnershipLayerTriad pins the exact point at which
// RotateAppend adopts the outgoing handle.
func TestRotateAppendValidationOwnershipLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive valid rotation adopts and closes outgoing before returning incoming", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		root, err := os.OpenRoot(dir)
		if err != nil {
			t.Fatalf("OpenRoot() error = %v", err)
		}
		t.Cleanup(func() { _ = root.Close() })
		outgoing := mustCreatedAppend(t, root, "current")
		incoming, err := RotateAppend(t.Context(), RotationRequest{
			Outgoing: outgoing,
			Incoming: AppendRequest{
				Location: Location{Root: root, Path: mustAppendIntentPath(t, "next")},
				Mode:     0o600,
				Append:   AppendCreate,
			},
		})
		if err != nil {
			t.Fatalf("RotateAppend() error = %v, want nil", err)
		}
		t.Cleanup(func() { _ = incoming.Close() })
		if _, err := outgoing.Write([]byte("closed")); !errors.Is(err, os.ErrClosed) {
			t.Fatalf("outgoing Write() error = %v, want %v", err, os.ErrClosed)
		}
	})

	t.Run("negative invalid incoming intent leaves outgoing caller-owned and creates nothing", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		root, err := os.OpenRoot(dir)
		if err != nil {
			t.Fatalf("OpenRoot() error = %v", err)
		}
		t.Cleanup(func() { _ = root.Close() })
		outgoing := mustCreatedAppend(t, root, "current")
		incoming, gotErr := RotateAppend(t.Context(), RotationRequest{
			Outgoing: outgoing,
			Incoming: AppendRequest{
				Location: Location{Root: root, Path: mustAppendIntentPath(t, "next")},
				Mode:     0o600,
				Append:   AppendExisting,
			},
		})
		if incoming != nil {
			_ = incoming.Close()
			t.Fatalf("RotateAppend(invalid intent) handle = %v, want nil", incoming)
		}
		if !errors.Is(gotErr, core.ErrFilestoreContract) {
			t.Fatalf("RotateAppend(invalid intent) error = %v, want %v", gotErr, core.ErrFilestoreContract)
		}
		if _, err := outgoing.Write([]byte("still owned\n")); err != nil {
			t.Fatalf("outgoing Write() after validation rejection error = %v, want nil", err)
		}
		if err := outgoing.Close(); err != nil {
			t.Fatalf("outgoing Close() error = %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, "next")); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("incoming stat error = %v, want %v", err, fs.ErrNotExist)
		}
	})

	t.Run("neutral cancelled call leaves outgoing caller-owned and performs no cutover", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		root, err := os.OpenRoot(dir)
		if err != nil {
			t.Fatalf("OpenRoot() error = %v", err)
		}
		t.Cleanup(func() { _ = root.Close() })
		outgoing := mustCreatedAppend(t, root, "current")
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		incoming, gotErr := RotateAppend(ctx, RotationRequest{
			Outgoing: outgoing,
			Incoming: AppendRequest{
				Location: Location{Root: root, Path: mustAppendIntentPath(t, "next")},
				Mode:     0o600,
				Append:   AppendCreate,
			},
		})
		if incoming != nil {
			_ = incoming.Close()
			t.Fatalf("RotateAppend(cancelled) handle = %v, want nil", incoming)
		}
		if !errors.Is(gotErr, context.Canceled) {
			t.Fatalf("RotateAppend(cancelled) error = %v, want %v", gotErr, context.Canceled)
		}
		if _, err := outgoing.Write([]byte("still owned\n")); err != nil {
			t.Fatalf("outgoing Write() after cancellation error = %v, want nil", err)
		}
		if err := outgoing.Close(); err != nil {
			t.Fatalf("outgoing Close() error = %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, "next")); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("incoming stat error = %v, want %v", err, fs.ErrNotExist)
		}
	})
}

func mustCreatedAppend(t *testing.T, root *os.Root, name string) *os.File {
	t.Helper()
	file, err := OpenAppend(t.Context(), AppendRequest{
		Location: Location{Root: root, Path: mustAppendIntentPath(t, name)},
		Mode:     0o600,
		Append:   AppendCreate,
	})
	if err != nil {
		t.Fatalf("OpenAppend(AppendCreate %q) error = %v", name, err)
	}
	return file
}

func mustAppendIntentPath(t *testing.T, value string) core.RelativePath {
	t.Helper()
	path, err := core.ParseRelativePath(value)
	if err != nil {
		t.Fatalf("ParseRelativePath(%q) error = %v", value, err)
	}
	return path
}
