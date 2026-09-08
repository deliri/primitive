package filestore_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// Acquisition, exact parent/child inode observations, public identity proof and
// caller Close are the fixed timed workload; filesystem construction is untimed.
func BenchmarkOpenParentNativeIdentityCustody(b *testing.B) {
	directory := b.TempDir()
	payload := bytes.Repeat([]byte{0, 255, 1, 127}, 32)
	name := filepath.Join(directory, "child")
	if err := os.WriteFile(name, payload, 0o600); err != nil {
		b.Fatal(err)
	}
	parentBefore, err := os.Stat(directory)
	if err != nil {
		b.Fatal(err)
	}
	childBefore, err := os.Stat(name)
	if err != nil {
		b.Fatal(err)
	}
	path, err := core.ParseAbsolutePath(name)
	if err != nil {
		b.Fatal(err)
	}
	parent, err := core.ParseAbsolutePath(directory)
	if err != nil {
		b.Fatal(err)
	}
	child, err := core.ParseRelativePath("child")
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		got, err := filestore.OpenParent(b.Context(), path)
		if err != nil {
			b.Fatal(err)
		}
		parentAfter, parentErr := got.Root.Stat(".")
		childAfter, childErr := got.Root.Lstat(got.Path.String())
		identityErr := filestore.ValidateRootIdentity(got.Root, parent)
		closeErr := got.Root.Close()
		if got.Path != child || parentErr != nil || childErr != nil || identityErr != nil || closeErr != nil || !os.SameFile(parentBefore, parentAfter) || !os.SameFile(childBefore, childAfter) || childBefore.Mode() != childAfter.Mode() || childBefore.Size() != childAfter.Size() || childBefore.ModTime().UnixNano() != childAfter.ModTime().UnixNano() {
			b.Fatalf("parent/child identity custody = (%v,%v,%v,%v,%v), want exact retained objects", got.Path, parentErr, childErr, identityErr, closeErr)
		}
	}
	got, err := os.ReadFile(name)
	if err != nil || !bytes.Equal(got, payload) {
		b.Fatalf("final child bytes = (%v,%v), want %v", got, err, payload)
	}
}

// Each operation checks one owned directory and one identically populated
// foreign directory. The refusal makes an always-successful check observable.
func BenchmarkRootIdentityOwnedAndForeignDirectories(b *testing.B) {
	container := b.TempDir()
	payload := bytes.Repeat([]byte{0, 255, 1, 127}, 32)
	var paths [2]core.AbsolutePath
	for i, name := range [...]string{"owned", "other"} {
		directory := filepath.Join(container, name)
		if err := os.Mkdir(directory, 0o700); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, "child"), payload, 0o600); err != nil {
			b.Fatal(err)
		}
		var err error
		paths[i], err = core.ParseAbsolutePath(directory)
		if err != nil {
			b.Fatal(err)
		}
	}
	root, err := filestore.OpenRoot(b.Context(), paths[0])
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := root.Close(); err != nil {
			b.Error(err)
		}
	})
	owned, err := os.Stat(paths[0].String())
	if err != nil {
		b.Fatal(err)
	}
	other, err := os.Stat(paths[1].String())
	if err != nil || os.SameFile(owned, other) {
		b.Fatalf("foreign fixture = (%v,%v), want distinct inode", other, err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if err := filestore.ValidateRootIdentity(root, paths[0]); err != nil {
			b.Fatalf("owned identity = %v, want nil", err)
		}
		if err := filestore.ValidateRootIdentity(root, paths[1]); !errors.Is(err, core.ErrFilestoreContract) || errors.Is(err, core.ErrFilestoreSource) {
			b.Fatalf("foreign identity = %v, want exact contract refusal", err)
		}
	}
	held, err := root.Stat(".")
	if err != nil || !os.SameFile(owned, held) {
		b.Fatalf("retained root = (%v,%v), want original", held, err)
	}
	for _, path := range paths {
		data, err := os.ReadFile(filepath.Join(path.String(), "child"))
		if err != nil || !bytes.Equal(data, payload) {
			b.Fatalf("retained child bytes = (%v,%v), want %v", data, err, payload)
		}
	}
}
