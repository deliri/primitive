package filestore_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func inspectPath(t *testing.T, path string) (filestore.PathKind, error) {
	t.Helper()

	absolute, err := core.ParseAbsolutePath(path)
	if err != nil {
		t.Fatalf("ParseAbsolutePath(%q) error = %v, want nil", path, err)
	}
	inspection, inspectErr := filestore.Inspect(context.Background(), absolute)
	if inspectErr != nil {
		return filestore.PathKindUnknown, inspectErr
	}
	kind, kindErr := inspection.Kind()
	if kindErr != nil {
		t.Fatalf("Kind() error = %v, want nil", kindErr)
	}
	return kind, nil
}

// TestInspectNamesWhatOccupiesAPath is the whole point of the contract: a
// caller admitting a configured path must be able to say which mistake the
// operator made, not merely that the path is unusable.
func TestInspectNamesWhatOccupiesAPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		build func(*testing.T, string) string
		name  string
		want  filestore.PathKind
	}{
		{
			name: "a real directory",
			want: filestore.PathKindDirectory,
			build: func(t *testing.T, root string) string {
				path := filepath.Join(root, "directory")
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatalf("Mkdir() error = %v, want nil", err)
				}
				return path
			},
		},
		{
			name: "a regular file",
			want: filestore.PathKindRegularFile,
			build: func(t *testing.T, root string) string {
				path := filepath.Join(root, "file")
				if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
					t.Fatalf("WriteFile() error = %v, want nil", err)
				}
				return path
			},
		},
		{
			name: "an empty regular file is still a file",
			want: filestore.PathKindRegularFile,
			build: func(t *testing.T, root string) string {
				path := filepath.Join(root, "empty")
				if err := os.WriteFile(path, nil, 0o600); err != nil {
					t.Fatalf("WriteFile() error = %v, want nil", err)
				}
				return path
			},
		},
		{
			name:  "nothing at a reachable path",
			want:  filestore.PathKindAbsent,
			build: func(_ *testing.T, root string) string { return filepath.Join(root, "absent") },
		},
		{
			// The link is reported as a link even though it resolves, because a
			// confined root will not traverse one that leaves the root. A caller
			// told "directory" here would be told about a path it cannot use.
			name: "a symbolic link to a directory is a link",
			want: filestore.PathKindSymbolicLink,
			build: func(t *testing.T, root string) string {
				target := filepath.Join(root, "target")
				if err := os.Mkdir(target, 0o700); err != nil {
					t.Fatalf("Mkdir() error = %v, want nil", err)
				}
				path := filepath.Join(root, "link")
				if err := os.Symlink(target, path); err != nil {
					t.Fatalf("Symlink() error = %v, want nil", err)
				}
				return path
			},
		},
		{
			name: "a dangling symbolic link is still a link",
			want: filestore.PathKindSymbolicLink,
			build: func(t *testing.T, root string) string {
				path := filepath.Join(root, "dangling")
				if err := os.Symlink(filepath.Join(root, "nothing"), path); err != nil {
					t.Fatalf("Symlink() error = %v, want nil", err)
				}
				return path
			},
		},
		{
			name: "a path below a missing parent is unreachable",
			want: filestore.PathKindUnreachable,
			build: func(_ *testing.T, root string) string {
				return filepath.Join(root, "missing", "child")
			},
		},
		{
			// Distinct from a missing parent in cause and identical in
			// consequence: nothing is there and nothing can be created there.
			name: "a path below a file is unreachable",
			want: filestore.PathKindUnreachable,
			build: func(t *testing.T, root string) string {
				parent := filepath.Join(root, "parentfile")
				if err := os.WriteFile(parent, []byte("x"), 0o600); err != nil {
					t.Fatalf("WriteFile() error = %v, want nil", err)
				}
				return filepath.Join(parent, "child")
			},
		},
		{
			name: "a directory holding entries is still a directory",
			want: filestore.PathKindDirectory,
			build: func(t *testing.T, root string) string {
				path := filepath.Join(root, "populated")
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatalf("Mkdir() error = %v, want nil", err)
				}
				if err := os.WriteFile(filepath.Join(path, "child"), []byte("x"), 0o600); err != nil {
					t.Fatalf("WriteFile() error = %v, want nil", err)
				}
				return path
			},
		},
		{
			name: "a directory with no execute bit is still a directory",
			want: filestore.PathKindDirectory,
			build: func(t *testing.T, root string) string {
				path := filepath.Join(root, "noexec")
				if err := os.Mkdir(path, 0o600); err != nil {
					t.Fatalf("Mkdir() error = %v, want nil", err)
				}
				t.Cleanup(func() { _ = os.Chmod(path, 0o700) })
				return path
			},
		},
		{
			// Lstat does not open the entry, so an unreadable file is still
			// observed as a file. A caller admitting a path must learn what is
			// there even when it could not read it.
			name: "a file with no read bit is still a file",
			want: filestore.PathKindRegularFile,
			build: func(t *testing.T, root string) string {
				path := filepath.Join(root, "unreadable")
				if err := os.WriteFile(path, []byte("x"), 0o200); err != nil {
					t.Fatalf("WriteFile() error = %v, want nil", err)
				}
				return path
			},
		},
		{
			name: "a large file is still a file",
			want: filestore.PathKindRegularFile,
			build: func(t *testing.T, root string) string {
				path := filepath.Join(root, "large")
				if err := os.WriteFile(path, make([]byte, 1<<20), 0o600); err != nil {
					t.Fatalf("WriteFile() error = %v, want nil", err)
				}
				return path
			},
		},
		{
			name: "a named pipe is neither a file nor a directory",
			want: filestore.PathKindOther,
			build: func(t *testing.T, root string) string {
				path := filepath.Join(root, "fifo")
				if err := syscallMkfifo(path); err != nil {
					t.Skipf("mkfifo unsupported here: %v", err)
				}
				return path
			},
		},
		{
			// A link whose target is a file must still report as a link, or a
			// caller would admit a path a confined root cannot traverse.
			name: "a symbolic link to a file is a link",
			want: filestore.PathKindSymbolicLink,
			build: func(t *testing.T, root string) string {
				target := filepath.Join(root, "linktarget")
				if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
					t.Fatalf("WriteFile() error = %v, want nil", err)
				}
				path := filepath.Join(root, "filelink")
				if err := os.Symlink(target, path); err != nil {
					t.Fatalf("Symlink() error = %v, want nil", err)
				}
				return path
			},
		},
		{
			name: "a symbolic link to itself is a link, not a traversal",
			want: filestore.PathKindSymbolicLink,
			build: func(t *testing.T, root string) string {
				path := filepath.Join(root, "selflink")
				if err := os.Symlink(path, path); err != nil {
					t.Fatalf("Symlink() error = %v, want nil", err)
				}
				return path
			},
		},
		{
			name: "a symbolic link chain reports the first link",
			want: filestore.PathKindSymbolicLink,
			build: func(t *testing.T, root string) string {
				first := filepath.Join(root, "chain1")
				second := filepath.Join(root, "chain2")
				if err := os.Symlink(first, second); err != nil {
					t.Fatalf("Symlink() error = %v, want nil", err)
				}
				if err := os.Mkdir(first, 0o700); err != nil {
					t.Fatalf("Mkdir() error = %v, want nil", err)
				}
				return second
			},
		},
		{
			name: "a link pointing outside the parent is still a link",
			want: filestore.PathKindSymbolicLink,
			build: func(t *testing.T, root string) string {
				path := filepath.Join(root, "escape")
				if err := os.Symlink("/etc", path); err != nil {
					t.Fatalf("Symlink() error = %v, want nil", err)
				}
				return path
			},
		},
		{
			name: "a name that differs only in case is absent when nothing made it",
			want: filestore.PathKindAbsent,
			build: func(t *testing.T, root string) string {
				return filepath.Join(root, "AbsentCased")
			},
		},
		{
			name: "an absent entry beside a real sibling is absent",
			want: filestore.PathKindAbsent,
			build: func(t *testing.T, root string) string {
				if err := os.WriteFile(filepath.Join(root, "sibling"), []byte("x"), 0o600); err != nil {
					t.Fatalf("WriteFile() error = %v, want nil", err)
				}
				return filepath.Join(root, "notsibling")
			},
		},
		{
			name: "a path two levels below a missing parent is unreachable",
			want: filestore.PathKindUnreachable,
			build: func(_ *testing.T, root string) string {
				return filepath.Join(root, "gone", "deeper", "child")
			},
		},
		{
			// The same operator mistake one level shallower is already the
			// unreachable observation; two levels below a regular file the
			// kernel answers ENOTDIR instead of ENOENT, and both spell the
			// same fact: nothing is there and nothing can be put there.
			name: "a path two levels below a regular file is unreachable",
			want: filestore.PathKindUnreachable,
			build: func(t *testing.T, root string) string {
				occupant := filepath.Join(root, "occupant")
				if err := os.WriteFile(occupant, []byte("x"), 0o600); err != nil {
					t.Fatalf("WriteFile() error = %v, want nil", err)
				}
				return filepath.Join(occupant, "deeper", "child")
			},
		},
		{
			name: "a path below a symbolic link parent is unreachable",
			want: filestore.PathKindUnreachable,
			build: func(t *testing.T, root string) string {
				parent := filepath.Join(root, "linkparent")
				if err := os.Symlink(filepath.Join(root, "nothing"), parent); err != nil {
					t.Fatalf("Symlink() error = %v, want nil", err)
				}
				return filepath.Join(parent, "child")
			},
		},
		{
			name:  "the temporary root itself is a directory",
			want:  filestore.PathKindDirectory,
			build: func(_ *testing.T, root string) string { return root },
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, err := inspectPath(t, testCase.build(t, t.TempDir()))
			if err != nil {
				t.Fatalf("Inspect() error = %v, want nil", err)
			}
			if got != testCase.want {
				t.Fatalf("Inspect() kind = %v, want %v", got, testCase.want)
			}
		})
	}
}

// TestInspectRefusesAnUnusableRequest closes the ingress. An unset path names
// nothing, and a cancelled context must not yield an observation that was
// never made.
func TestInspectRefusesAnUnusableRequest(t *testing.T) {
	t.Parallel()

	if _, err := filestore.Inspect(context.Background(), core.AbsolutePath{}); !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatalf("Inspect(unset path) error = %v, want errors.Is %v", err, core.ErrFilestoreContract)
	}

	absolute, err := core.ParseAbsolutePath(t.TempDir())
	if err != nil {
		t.Fatalf("ParseAbsolutePath() error = %v, want nil", err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := filestore.Inspect(cancelled, absolute)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Inspect(cancelled) error = %v, want errors.Is %v", err, context.Canceled)
	}
	if validateErr := got.Validate(); !errors.Is(validateErr, core.ErrFilestoreContract) {
		t.Fatalf("refused inspection Validate() error = %v, want errors.Is %v", validateErr, core.ErrFilestoreContract)
	}
}

// TestPathKindAdmitsOnlyObservedKinds closes the enum. An unset or future kind
// must never render as an observation, because the rendering is what an
// operator reads when their configured path is refused.
func TestPathKindAdmitsOnlyObservedKinds(t *testing.T) {
	t.Parallel()

	observed := []filestore.PathKind{
		filestore.PathKindAbsent,
		filestore.PathKindDirectory,
		filestore.PathKindRegularFile,
		filestore.PathKindSymbolicLink,
		filestore.PathKindOther,
		filestore.PathKindUnreachable,
	}
	seen := map[string]filestore.PathKind{}
	for _, kind := range observed {
		if err := kind.Validate(); err != nil {
			t.Errorf("PathKind(%d).Validate() error = %v, want nil", kind, err)
		}
		text := kind.String()
		if text == "" {
			t.Errorf("PathKind(%d).String() is empty, want operator-facing text", kind)
		}
		if prior, duplicate := seen[text]; duplicate {
			t.Errorf("PathKind(%d) and PathKind(%d) render identically as %q", kind, prior, text)
		}
		seen[text] = kind
	}

	for _, kind := range []filestore.PathKind{filestore.PathKindUnknown, filestore.PathKind(200), filestore.PathKind(^uint8(0))} {
		if kind.IsValid() {
			t.Errorf("PathKind(%d).IsValid() = true, want false", kind)
		}
		if kind.String() != "" {
			t.Errorf("PathKind(%d).String() = %q, want empty", kind, kind.String())
		}
	}

	if _, err := (filestore.Inspection{}).Kind(); !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatalf("zero Inspection Kind() error = %v, want errors.Is %v", err, core.ErrFilestoreContract)
	}
}

// TestInspectRefusesAnObservationItWasNotAllowedToMake is the negative that
// matters most, and it is the one the doc comment promises.
//
// When a parent directory denies traversal, nothing can be learned about what
// is inside it. Reporting that as absent would be a lie with consequences: a
// caller admitting a configured path would conclude the path is free and go on
// to create something, or conclude a repository is missing when it is merely
// unreadable. The refusal must be an error, and it must not be the absent kind.
func TestInspectRefusesAnObservationItWasNotAllowedToMake(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses directory permissions, so the refusal is unreachable")
	}

	root := t.TempDir()
	sealed := filepath.Join(root, "sealed")
	if err := os.Mkdir(sealed, 0o700); err != nil {
		t.Fatalf("Mkdir() error = %v, want nil", err)
	}
	child := filepath.Join(sealed, "child")
	if err := os.WriteFile(child, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v, want nil", err)
	}
	// The entry is observable while the parent allows it, so the case cannot
	// pass by the path having been absent all along.
	if kind, err := inspectPath(t, child); err != nil || kind != filestore.PathKindRegularFile {
		t.Fatalf("Inspect(before sealing) = (%v, %v), want (%v, nil)", kind, err, filestore.PathKindRegularFile)
	}
	if err := os.Chmod(sealed, 0o000); err != nil {
		t.Fatalf("Chmod() error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = os.Chmod(sealed, 0o700) })

	got, err := filestore.Inspect(context.Background(), mustAbsolute(t, child))
	if !errors.Is(err, core.ErrFilestoreSource) {
		t.Fatalf("Inspect(unreadable parent) error = %v, want errors.Is %v", err, core.ErrFilestoreSource)
	}
	kind, kindErr := got.Kind()
	if !errors.Is(kindErr, core.ErrFilestoreContract) {
		t.Fatalf("refused inspection Kind() = %v, error = %v, want errors.Is %v", kind, kindErr, core.ErrFilestoreContract)
	}
	if kind == filestore.PathKindAbsent || kind == filestore.PathKindUnreachable {
		t.Fatalf("refused inspection reported %v, want no observation at all", kind)
	}
}

func mustAbsolute(t *testing.T, path string) core.AbsolutePath {
	t.Helper()

	absolute, err := core.ParseAbsolutePath(path)
	if err != nil {
		t.Fatalf("ParseAbsolutePath(%q) error = %v, want nil", path, err)
	}
	return absolute
}
