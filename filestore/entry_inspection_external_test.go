package filestore_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestEntryInspectionNativeExtentAndKindMatrix(t *testing.T) {
	t.Parallel()
	for _, extent := range []int64{0, 1, 4095, 4096, 4097, 6<<20 - 1, 6 << 20, 6<<20 + 1, 1 << 32, 1 << 40} {
		for _, shape := range []struct {
			name, path string
			kind       filestore.PathKind
			want       error
			prepare    func(*testing.T, string, *os.Root)
		}{
			{name: "regular", path: "target", kind: filestore.PathKindRegularFile},
			{name: "directory", path: "directory", kind: filestore.PathKindDirectory, prepare: func(t *testing.T, directory string, _ *os.Root) {
				t.Helper()
				if err := os.Mkdir(filepath.Join(directory, "directory"), 0700); err != nil {
					t.Fatal(err)
				}
			}},
			{name: "symlink", path: "link", kind: filestore.PathKindSymbolicLink, prepare: func(t *testing.T, directory string, _ *os.Root) {
				t.Helper()
				if err := os.Symlink("target", filepath.Join(directory, "link")); err != nil {
					t.Fatal(err)
				}
			}},
			{name: "absent", path: "missing", want: fs.ErrNotExist},
			{name: "closed_root", path: "target", want: fs.ErrClosed, prepare: func(t *testing.T, _ string, root *os.Root) {
				t.Helper()
				if err := root.Close(); err != nil {
					t.Fatal(err)
				}
			}},
		} {
			t.Run(fmt.Sprintf("extent_%d_%s", extent, shape.name), func(t *testing.T) {
				t.Parallel()
				directory := t.TempDir()
				file, err := os.Create(filepath.Join(directory, "target"))
				if err != nil {
					t.Fatal(err)
				}
				if err := file.Truncate(extent); err != nil {
					_ = file.Close()
					t.Fatal(err)
				}
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
				root, err := os.OpenRoot(directory)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := root.Close(); err != nil && !errors.Is(err, fs.ErrClosed) {
						t.Error(err)
					}
				})
				if shape.prepare != nil {
					shape.prepare(t, directory, root)
				}
				path, err := core.ParseRelativePath(shape.path)
				if err != nil {
					t.Fatal(err)
				}
				got, err := filestore.InspectEntry(t.Context(), filestore.Location{Root: root, Path: path})
				if shape.want != nil {
					var native *fs.PathError
					if got != (filestore.Inspection{}) || !errors.Is(err, shape.want) || !errors.Is(err, core.ErrFilestoreSource) || !errors.As(err, &native) {
						t.Fatalf("native entry proof/refusal=%+v/%v,want zero proof and %v with native PathError", got, err, shape.want)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				kind, err := got.Kind()
				if err != nil || kind != shape.kind || got.Validate() != nil {
					t.Fatalf("native entry kind=%v/%v,want %v", kind, err, shape.kind)
				}
				native, err := root.Lstat(shape.path)
				if err != nil {
					t.Fatal(err)
				}
				modified, err := got.ModifiedAt()
				if err != nil {
					t.Fatal(err)
				}
				nanos, err := modified.Nanoseconds()
				if err != nil || nanos != native.ModTime().UnixNano() {
					t.Fatalf("native modified time=%d/%v,want exact %d", nanos, err, native.ModTime().UnixNano())
				}
				if kind == filestore.PathKindRegularFile {
					size, err := got.SizeBytes()
					if err != nil || size.Uint64() != uint64(extent) {
						t.Fatalf("native entry extent=%v/%v,want %d", size, err, extent)
					}
				}
			})
		}
	}
}

func TestEntryInspectionUsesBorrowedRootAfterNamespaceReplacement(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	original := filepath.Join(parent, "original")
	if err := os.Mkdir(original, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(original, "target"), []byte("owned"), 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(original)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := os.Rename(original, filepath.Join(parent, "moved")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(original, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(original, "target"), []byte("foreign replacement namespace"), 0600); err != nil {
		t.Fatal(err)
	}
	path, err := core.ParseRelativePath("target")
	if err != nil {
		t.Fatal(err)
	}
	got, err := filestore.InspectEntry(t.Context(), filestore.Location{Root: root, Path: path})
	if err != nil {
		t.Fatal(err)
	}
	size, err := got.SizeBytes()
	if err != nil || size.Uint64() != 5 {
		t.Fatalf("borrowed-root extent=%v/%v,want original native object extent5", size, err)
	}
	// An absolute-path observation is an independently different authority.
	absolute, err := core.ParseAbsolutePath(filepath.Join(original, "target"))
	if err != nil {
		t.Fatal(err)
	}
	other, err := filestore.Inspect(t.Context(), absolute)
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := other.SizeBytes()
	if err != nil || foreign.Uint64() == size.Uint64() {
		t.Fatalf("absolute replacement extent=%v/%v,want distinct namespace", foreign, err)
	}
}

func TestEntryInspectionPreservesNativeCancellationCauseAndRootLifetime(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "target"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	path, err := core.ParseRelativePath("target")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)
	cause := &fs.PathError{Op: "inspect", Path: "borrowed-entry", Err: fs.ErrPermission}
	cancel(cause)
	got, err := filestore.InspectEntry(ctx, filestore.Location{Root: root, Path: path})
	var native *fs.PathError
	if got != (filestore.Inspection{}) || !errors.Is(err, context.Canceled) || !errors.Is(err, fs.ErrPermission) || !errors.As(err, &native) || native != cause {
		t.Fatalf("canceled native inspection=%+v/%v/%v,want no proof and exact original cause", got, err, native)
	}
	if _, err := root.Lstat("target"); err != nil {
		t.Fatalf("borrowed root lifetime=%v,want still usable", err)
	}
}

func TestEntryInspectionAdmissionReturnsNoNativeProof(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                       string
		nilContext, noRoot, noPath bool
		want                       error
	}{
		{name: "nil_context", nilContext: true, want: core.ErrNilContext},
		{name: "no_root", noRoot: true, want: core.ErrFilestoreContract},
		{name: "no_path", noPath: true, want: core.ErrFilestoreContract},
		{name: "empty_location", noRoot: true, noPath: true, want: core.ErrFilestoreContract},
		{name: "nil_context_and_empty_location", nilContext: true, noRoot: true, noPath: true, want: core.ErrNilContext},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			root, err := os.OpenRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := root.Close(); err != nil {
					t.Error(err)
				}
			})
			path, err := core.ParseRelativePath(".")
			if err != nil {
				t.Fatal(err)
			}
			location := filestore.Location{Root: root, Path: path}
			if tc.noRoot {
				location.Root = nil
			}
			if tc.noPath {
				location.Path = core.RelativePath{}
			}
			var ctx context.Context = t.Context()
			if tc.nilContext {
				ctx = nil
			}
			got, err := filestore.InspectEntry(ctx, location)
			if got != (filestore.Inspection{}) || !errors.Is(err, tc.want) {
				t.Fatalf("entry admission proof/refusal=%+v/%v,want no proof and %v", got, err, tc.want)
			}
			if tc.nilContext && tc.noRoot && !errors.Is(err, core.ErrFilestoreContract) {
				t.Fatalf("combined admission refusal=%v,want both invalid context and location identities", err)
			}
			if _, err := root.Lstat("."); err != nil {
				t.Fatalf("invalid admission closed borrowed root: %v", err)
			}
		})
	}
}

func TestEntryInspectionConfinesParentsAndObservesFinalLink(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "target"), []byte("foreign metadata"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(directory, "outside")); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	path, err := core.ParseRelativePath("outside/target")
	if err != nil {
		t.Fatal(err)
	}
	got, err := filestore.InspectEntry(t.Context(), filestore.Location{Root: root, Path: path})
	var native *fs.PathError
	if got != (filestore.Inspection{}) || !errors.Is(err, core.ErrFilestoreSource) || !errors.As(err, &native) {
		t.Fatalf("escaped-parent proof/refusal=%+v/%v,want no proof and native source refusal", got, err)
	}
	path, err = core.ParseRelativePath("outside")
	if err != nil {
		t.Fatal(err)
	}
	got, err = filestore.InspectEntry(t.Context(), filestore.Location{Root: root, Path: path})
	if err != nil {
		t.Fatal(err)
	}
	kind, err := got.Kind()
	if err != nil || kind != filestore.PathKindSymbolicLink {
		t.Fatalf("final native link kind=%v/%v,want symlink without following its target", kind, err)
	}
}

func FuzzEntryInspectionNativeFacts(f *testing.F) {
	f.Add(uint32(0), false)
	f.Add(uint32(4097), false)
	f.Add(uint32(6<<20+1), true)
	f.Fuzz(func(t *testing.T, extent uint32, link bool) {
		directory := t.TempDir()
		file, err := os.Create(filepath.Join(directory, "target"))
		if err != nil {
			t.Fatal(err)
		}
		if err := file.Truncate(int64(extent)); err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		name := "target"
		wantKind := filestore.PathKindRegularFile
		if link {
			if err := os.Symlink("target", filepath.Join(directory, "link")); err != nil {
				t.Fatal(err)
			}
			name = "link"
			wantKind = filestore.PathKindSymbolicLink
		}
		root, err := os.OpenRoot(directory)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := root.Close(); err != nil {
				t.Error(err)
			}
		})
		path, err := core.ParseRelativePath(name)
		if err != nil {
			t.Fatal(err)
		}
		got, err := filestore.InspectEntry(t.Context(), filestore.Location{Root: root, Path: path})
		if err != nil {
			t.Fatal(err)
		}
		native, err := root.Lstat(name)
		if err != nil {
			t.Fatal(err)
		}
		kind, err := got.Kind()
		if err != nil || kind != wantKind || got.Validate() != nil {
			t.Fatalf("native kind=%v/%v,want %v", kind, err, wantKind)
		}
		if !link {
			size, err := got.SizeBytes()
			if err != nil || size.Uint64() != uint64(native.Size()) || native.Size() != int64(extent) {
				t.Fatalf("native source/inspection extent=%d/%v/%v,want %d", native.Size(), size, err, extent)
			}
		}
	})
}

func BenchmarkEntryInspectionNativeMetadata(b *testing.B) {
	directory := b.TempDir()
	file, err := os.Create(filepath.Join(directory, "target"))
	if err != nil {
		b.Fatal(err)
	}
	if err := file.Truncate(1 << 40); err != nil {
		_ = file.Close()
		b.Fatal(err)
	}
	if err := file.Close(); err != nil {
		b.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := root.Close(); err != nil {
			b.Error(err)
		}
	})
	path, err := core.ParseRelativePath("target")
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		got, err := filestore.InspectEntry(b.Context(), filestore.Location{Root: root, Path: path})
		if err != nil {
			b.Fatal(err)
		}
		size, err := got.SizeBytes()
		if err != nil || size.Uint64() != 1<<40 {
			b.Fatalf("native metadata=%v/%v,want exact sparse extent", size, err)
		}
	}
}
