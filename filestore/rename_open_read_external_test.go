package filestore_test

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

const renameEntryPermissions = 0o600

// renameFixture builds the tree one case needs and returns nothing, so each
// parallel subtest owns every byte it touches.
type renameFixture func(testing.TB, string)

func renameNoFixture(testing.TB, string) {}

func renameWriteFile(name string) renameFixture {
	return func(tb testing.TB, root string) {
		tb.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), renameEntryPermissions); err != nil {
			tb.Fatalf("WriteFile(%s) error = %v, want nil", name, err)
		}
	}
}

func renameMakeDirectory(name string) renameFixture {
	return func(tb testing.TB, root string) {
		tb.Helper()
		if err := os.MkdirAll(filepath.Join(root, name), 0o700); err != nil {
			tb.Fatalf("MkdirAll(%s) error = %v, want nil", name, err)
		}
	}
}

func renameAll(fixtures ...renameFixture) renameFixture {
	return func(tb testing.TB, root string) {
		tb.Helper()
		for _, fixture := range fixtures {
			fixture(tb, root)
		}
	}
}

// TestRenameMovesOneExistingEntryOrRefusesTheRequest pressures both sides of
// every boundary Rename owns: what may move, what may not, and which requests
// are rejected before any effect reaches the filesystem.
func TestRenameMovesOneExistingEntryOrRefusesTheRequest(t *testing.T) {
	t.Parallel()

	cases := []struct {
		build             renameFixture
		name              string
		source            string
		target            string
		wantErr           error
		wantMove          bool
		wantSourceRemains bool
	}{
		{
			name:     "regular file moves to a new name in the same directory",
			build:    renameWriteFile("source"),
			source:   "source",
			target:   "target",
			wantMove: true,
		},
		{
			name:     "regular file moves into another directory",
			build:    renameAll(renameWriteFile("source"), renameMakeDirectory("elsewhere")),
			source:   "source",
			target:   "elsewhere/target",
			wantMove: true,
		},
		{
			name:     "regular file moves out of a directory",
			build:    renameAll(renameMakeDirectory("nested"), renameWriteFile("nested/source")),
			source:   "nested/source",
			target:   "target",
			wantMove: true,
		},
		{
			name:     "empty directory moves, which Commit cannot express",
			build:    renameMakeDirectory("source"),
			source:   "source",
			target:   "target",
			wantMove: true,
		},
		{
			name:     "populated directory moves with its contents",
			build:    renameAll(renameMakeDirectory("source"), renameWriteFile("source/child")),
			source:   "source",
			target:   "target",
			wantMove: true,
		},
		{
			name:     "existing regular target is replaced",
			build:    renameAll(renameWriteFile("source"), renameWriteFile("target")),
			source:   "source",
			target:   "target",
			wantMove: true,
		},
		{
			name:     "symbolic link moves as the link itself",
			build:    renameAll(renameWriteFile("elsewhere"), renameSymlink("elsewhere", "source")),
			source:   "source",
			target:   "target",
			wantMove: true,
		},
		{
			name:     "name at the depth limit of this fixture still moves",
			build:    renameAll(renameMakeDirectory("a/b/c"), renameWriteFile("a/b/c/source")),
			source:   "a/b/c/source",
			target:   "a/b/c/target",
			wantMove: true,
		},
		{
			name:    "absent source is an activation failure, not a silent success",
			build:   renameNoFixture,
			source:  "source",
			target:  "target",
			wantErr: core.ErrFilestoreActivation,
		},
		{
			name:              "target under an absent directory is refused",
			wantSourceRemains: true,
			build:             renameWriteFile("source"),
			source:            "source",
			target:            "missing/target",
			wantErr:           core.ErrFilestoreActivation,
		},
		{
			name:              "directory cannot replace a regular file",
			wantSourceRemains: true,
			build:             renameAll(renameMakeDirectory("source"), renameWriteFile("target")),
			source:            "source",
			target:            "target",
			wantErr:           core.ErrFilestoreActivation,
		},
		{
			name:              "regular file cannot replace a directory",
			wantSourceRemains: true,
			build:             renameAll(renameWriteFile("source"), renameMakeDirectory("target")),
			source:            "source",
			target:            "target",
			wantErr:           core.ErrFilestoreActivation,
		},
		{
			name:              "directory cannot replace a populated directory",
			wantSourceRemains: true,
			build:             renameAll(renameMakeDirectory("source"), renameMakeDirectory("target"), renameWriteFile("target/child")),
			source:            "source",
			target:            "target",
			wantErr:           core.ErrFilestoreActivation,
		},
		{
			name:              "source below the target is refused rather than looping",
			wantSourceRemains: true,
			build:             renameMakeDirectory("source/nested"),
			source:            "source",
			target:            "source/nested/target",
			wantErr:           core.ErrFilestoreActivation,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			directory := t.TempDir()
			tc.build(t, directory)
			root := requireTestRoot(t, directory)

			gotErr := filestore.Rename(t.Context(), filestore.RenameRequest{
				Location: filestore.Location{Root: root, Path: mustRelativePath(t, tc.source)},
				Target:   mustRelativePath(t, tc.target),
			})
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("Rename() error = %v, want errors.Is %v", gotErr, tc.wantErr)
				}
				if tc.wantSourceRemains {
					requireEntryPresent(t, directory, tc.source)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("Rename() error = %v, want nil", gotErr)
			}
			requireEntryAbsent(t, directory, tc.source)
			requireEntryPresent(t, directory, tc.target)
		})
	}
}

func renameSymlink(target, name string) renameFixture {
	return func(tb testing.TB, root string) {
		tb.Helper()
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			tb.Fatalf("Symlink(%s) error = %v, want nil", name, err)
		}
	}
}

func requireEntryPresent(tb testing.TB, root, name string) {
	tb.Helper()
	if _, err := os.Lstat(filepath.Join(root, name)); err != nil {
		tb.Fatalf("Lstat(%s) error = %v, want the entry to be present", name, err)
	}
}

func requireEntryAbsent(tb testing.TB, root, name string) {
	tb.Helper()
	_, err := os.Lstat(filepath.Join(root, name))
	if !errors.Is(err, fs.ErrNotExist) {
		tb.Fatalf("Lstat(%s) error = %v, want the entry to be gone", name, err)
	}
}

// TestRenameRefusesAnUnusableRequestBeforeAnyEffect proves the contract gate
// runs before the filesystem is touched. Each case names a request that cannot
// mean anything, and none of them may move a byte.
func TestRenameRefusesAnUnusableRequestBeforeAnyEffect(t *testing.T) {
	t.Parallel()

	cases := []struct {
		build   func(*testing.T, string) filestore.RenameRequest
		name    string
		wantErr error
	}{
		{
			name: "missing root capability",
			build: func(t *testing.T, _ string) filestore.RenameRequest {
				return filestore.RenameRequest{
					Location: filestore.Location{Path: mustRelativePath(t, "source")},
					Target:   mustRelativePath(t, "target"),
				}
			},
			wantErr: core.ErrFilestoreContract,
		},
		{
			name: "unset source path",
			build: func(t *testing.T, directory string) filestore.RenameRequest {
				return filestore.RenameRequest{
					Location: filestore.Location{Root: requireTestRoot(t, directory)},
					Target:   mustRelativePath(t, "target"),
				}
			},
			wantErr: core.ErrFilestoreContract,
		},
		{
			name: "unset target path",
			build: func(t *testing.T, directory string) filestore.RenameRequest {
				return filestore.RenameRequest{
					Location: filestore.Location{
						Root: requireTestRoot(t, directory),
						Path: mustRelativePath(t, "source"),
					},
				}
			},
			wantErr: core.ErrFilestoreContract,
		},
		{
			name: "source names the root entry",
			build: func(t *testing.T, directory string) filestore.RenameRequest {
				return filestore.RenameRequest{
					Location: filestore.Location{
						Root: requireTestRoot(t, directory),
						Path: mustRelativePath(t, "."),
					},
					Target: mustRelativePath(t, "target"),
				}
			},
			wantErr: core.ErrFilestoreContract,
		},
		{
			name: "target names the root entry",
			build: func(t *testing.T, directory string) filestore.RenameRequest {
				return filestore.RenameRequest{
					Location: filestore.Location{
						Root: requireTestRoot(t, directory),
						Path: mustRelativePath(t, "source"),
					},
					Target: mustRelativePath(t, "."),
				}
			},
			wantErr: core.ErrFilestoreContract,
		},
		{
			name: "target equals its source",
			build: func(t *testing.T, directory string) filestore.RenameRequest {
				return filestore.RenameRequest{
					Location: filestore.Location{
						Root: requireTestRoot(t, directory),
						Path: mustRelativePath(t, "source"),
					},
					Target: mustRelativePath(t, "source"),
				}
			},
			wantErr: core.ErrFilestoreContract,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			directory := t.TempDir()
			renameWriteFile("source")(t, directory)
			if gotErr := filestore.Rename(t.Context(), tc.build(t, directory)); !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Rename() error = %v, want errors.Is %v", gotErr, tc.wantErr)
			}
			requireEntryPresent(t, directory, "source")
			requireEntryAbsent(t, directory, "target")
		})
	}
}

// TestOpenReadAcquiresOnlyARegularFile proves the read handle refuses every
// impostor before the caller can read a byte. A caller receiving a handle is
// entitled to assume it names the regular file it asked for; a symbolic link
// or device answering instead is how the wrong bytes enter an evidence path.
func TestOpenReadAcquiresOnlyARegularFile(t *testing.T) {
	t.Parallel()

	cases := []struct {
		build       renameFixture
		name        string
		path        string
		wantContent string
		wantErr     error
	}{
		{
			name:        "regular file hands back its exact bytes",
			build:       renameWriteFile("source"),
			path:        "source",
			wantContent: "source",
		},
		{
			name: "empty regular file is a valid handle over no bytes",
			build: func(tb testing.TB, root string) {
				tb.Helper()
				if err := os.WriteFile(filepath.Join(root, "source"), nil, renameEntryPermissions); err != nil {
					tb.Fatalf("WriteFile(source) error = %v, want nil", err)
				}
			},
			path: "source",
		},
		{
			name:        "nested regular file is reachable through the root",
			build:       renameAll(renameMakeDirectory("nested"), renameWriteFile("nested/source")),
			path:        "nested/source",
			wantContent: "nested/source",
		},
		{
			name:    "absent entry has no handle",
			build:   renameNoFixture,
			path:    "source",
			wantErr: core.ErrFilestoreSource,
		},
		{
			name:    "directory is refused",
			build:   renameMakeDirectory("source"),
			path:    "source",
			wantErr: core.ErrFilestoreSource,
		},
		{
			// Confinement, not link-avoidance, is this package's rule: a link
			// that stays inside the capability names bytes the caller already
			// has, so it is followed exactly as Read follows it. The two cases
			// below are the ones that matter, because they leave the root or
			// name nothing at all.
			name:        "symbolic link staying inside the root is followed to its target",
			build:       renameAll(renameWriteFile("elsewhere"), renameSymlink("elsewhere", "source")),
			path:        "source",
			wantContent: "elsewhere",
		},
		{
			name:    "dangling symbolic link is refused",
			build:   renameSymlink("nothing-is-here", "source"),
			path:    "source",
			wantErr: core.ErrFilestoreSource,
		},
		{
			name:    "symbolic link escaping the root is refused",
			build:   renameSymlink("/etc/hosts", "source"),
			path:    "source",
			wantErr: core.ErrFilestoreSource,
		},
		{
			name:    "entry below a regular file is refused",
			build:   renameWriteFile("nested"),
			path:    "nested/source",
			wantErr: core.ErrFilestoreSource,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			directory := t.TempDir()
			tc.build(t, directory)
			root := requireTestRoot(t, directory)

			handle, gotErr := filestore.OpenRead(t.Context(), filestore.ReadHandleRequest{
				Location: filestore.Location{Root: root, Path: mustRelativePath(t, tc.path)},
			})
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("OpenRead() error = %v, want errors.Is %v", gotErr, tc.wantErr)
				}
				if handle != nil {
					t.Fatalf("OpenRead() handle = %v, want nil alongside a refusal", handle.Name())
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("OpenRead() error = %v, want nil", gotErr)
			}
			content, readErr := io.ReadAll(handle)
			closeErr := handle.Close()
			if readErr != nil || closeErr != nil {
				t.Fatalf("read handle = (%v, %v), want (nil, nil)", readErr, closeErr)
			}
			if string(content) != tc.wantContent {
				t.Fatalf("handle content = %q, want %q", content, tc.wantContent)
			}
		})
	}
}

// TestOpenReadRefusesAnUnusableRequest keeps the contract gate ahead of the
// effect, matching every other operation in this package.
func TestOpenReadRefusesAnUnusableRequest(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	renameWriteFile("source")(t, directory)

	if _, err := filestore.OpenRead(t.Context(), filestore.ReadHandleRequest{
		Location: filestore.Location{Path: mustRelativePath(t, "source")},
	}); !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatalf("OpenRead(no root) error = %v, want errors.Is %v", err, core.ErrFilestoreContract)
	}
	if _, err := filestore.OpenRead(t.Context(), filestore.ReadHandleRequest{
		Location: filestore.Location{Root: requireTestRoot(t, directory)},
	}); !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatalf("OpenRead(no path) error = %v, want errors.Is %v", err, core.ErrFilestoreContract)
	}
}

// TestInspectionModifiedAtAnswersOnlyForAnEntryThatExists proves the accessor
// cannot invent a time. An absent path has no modification time, and returning
// a zero instant would read as 1970 to a staleness decision — the oldest
// possible answer, which is exactly the answer that reaps something it should
// have left alone.
func TestInspectionModifiedAtAnswersOnlyForAnEntryThatExists(t *testing.T) {
	t.Parallel()

	cases := []struct {
		build      renameFixture
		name       string
		path       string
		wantReport bool
	}{
		{name: "regular file reports when it changed", build: renameWriteFile("entry"), path: "entry", wantReport: true},
		{name: "directory reports when it changed", build: renameMakeDirectory("entry"), path: "entry", wantReport: true},
		{name: "symbolic link reports its own time, not its target's", build: renameSymlink("elsewhere", "entry"), path: "entry", wantReport: true},
		{name: "absent entry has no time to report", build: renameNoFixture, path: "entry"},
		{name: "entry below a regular file is unreachable and has no time", build: renameWriteFile("nested"), path: "nested/entry"},
		{name: "entry below an absent directory has no time", build: renameNoFixture, path: "missing/entry"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			directory := t.TempDir()
			tc.build(t, directory)
			path, err := core.ParseAbsolutePath(filepath.Join(directory, tc.path))
			if err != nil {
				t.Fatalf("ParseAbsolutePath() error = %v, want nil", err)
			}
			inspection, err := filestore.Inspect(t.Context(), path)
			if err != nil {
				t.Fatalf("Inspect() error = %v, want nil", err)
			}

			modified, gotErr := inspection.ModifiedAt()
			if !tc.wantReport {
				if !errors.Is(gotErr, core.ErrFilestoreContract) {
					t.Fatalf("ModifiedAt() error = %v, want errors.Is %v", gotErr, core.ErrFilestoreContract)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("ModifiedAt() error = %v, want nil", gotErr)
			}
			nanoseconds, err := modified.Nanoseconds()
			if err != nil {
				t.Fatalf("Nanoseconds() error = %v, want nil", err)
			}
			if nanoseconds <= 0 {
				t.Fatalf("ModifiedAt() nanoseconds = %d, want a positive instant", nanoseconds)
			}
		})
	}
}

// TestInspectionSizeBytesAnswersOnlyForARegularFile proves the byte count is
// refused wherever it would be a lie. A directory's reported size is a
// filesystem implementation detail and a symbolic link's is the length of its
// target text; either would read as a byte count to a caller comparing it
// against a stream it just wrote.
func TestInspectionSizeBytesAnswersOnlyForARegularFile(t *testing.T) {
	t.Parallel()

	cases := []struct {
		build      renameFixture
		name       string
		path       string
		wantBytes  uint64
		wantReport bool
	}{
		{
			name:       "regular file reports its exact byte count",
			build:      renameWriteFile("entry"),
			path:       "entry",
			wantBytes:  uint64(len("entry")),
			wantReport: true,
		},
		{
			name: "empty regular file reports zero rather than refusing",
			build: func(tb testing.TB, root string) {
				tb.Helper()
				if err := os.WriteFile(filepath.Join(root, "entry"), nil, renameEntryPermissions); err != nil {
					tb.Fatalf("WriteFile(entry) error = %v, want nil", err)
				}
			},
			path:       "entry",
			wantReport: true,
		},
		{
			name: "one byte is reported as one byte",
			build: func(tb testing.TB, root string) {
				tb.Helper()
				if err := os.WriteFile(filepath.Join(root, "entry"), []byte{0}, renameEntryPermissions); err != nil {
					tb.Fatalf("WriteFile(entry) error = %v, want nil", err)
				}
			},
			path:       "entry",
			wantBytes:  1,
			wantReport: true,
		},
		{name: "directory has no meaningful byte count", build: renameMakeDirectory("entry"), path: "entry"},
		{name: "symbolic link reports no count rather than its target text length", build: renameSymlink("elsewhere", "entry"), path: "entry"},
		{name: "absent entry has no byte count", build: renameNoFixture, path: "entry"},
		{name: "entry below a regular file is unreachable and has no byte count", build: renameWriteFile("nested"), path: "nested/entry"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			directory := t.TempDir()
			tc.build(t, directory)
			path, err := core.ParseAbsolutePath(filepath.Join(directory, tc.path))
			if err != nil {
				t.Fatalf("ParseAbsolutePath() error = %v, want nil", err)
			}
			inspection, err := filestore.Inspect(t.Context(), path)
			if err != nil {
				t.Fatalf("Inspect() error = %v, want nil", err)
			}

			gotBytes, gotErr := inspection.SizeBytes()
			if !tc.wantReport {
				if !errors.Is(gotErr, core.ErrFilestoreContract) {
					t.Fatalf("SizeBytes() error = %v, want errors.Is %v", gotErr, core.ErrFilestoreContract)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("SizeBytes() error = %v, want nil", gotErr)
			}
			if gotBytes.Uint64() != tc.wantBytes {
				t.Fatalf("SizeBytes() = %d, want %d", gotBytes.Uint64(), tc.wantBytes)
			}
		})
	}
}

// TestInspectionModifiedAtRefusesAnObservationNobodyMade proves the zero value
// cannot answer. An Inspection a caller assembled itself never observed
// anything, so it has neither a kind nor a time.
func TestInspectionModifiedAtRefusesAnObservationNobodyMade(t *testing.T) {
	t.Parallel()

	if _, err := (filestore.Inspection{}).ModifiedAt(); !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatalf("ModifiedAt(zero observation) error = %v, want errors.Is %v", err, core.ErrFilestoreContract)
	}
}
