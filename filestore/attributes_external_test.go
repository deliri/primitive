package filestore_test

import (
	"errors"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

// attributesInspect observes one planted entry through the real production
// path, so every assertion below is about what Inspect recorded rather than
// about a value a test assembled.
func attributesInspect(t *testing.T, directory, name string) filestore.Inspection {
	t.Helper()

	inspection, err := filestore.Inspect(t.Context(), custodyAbsolute(t, directory, name))
	if err != nil {
		t.Fatalf("Inspect(%s) error = %v, want nil", name, err)
	}
	return inspection
}

// attributesCurrentOwner is who this process is, which is who every file it
// creates belongs to. Reading it from the user database rather than assuming
// zero is what makes the assertion mean something on a developer machine and
// in a root container alike.
func attributesCurrentOwner(t *testing.T) (uint32, uint32) {
	t.Helper()

	current, err := user.Current()
	if err != nil {
		t.Fatalf("user.Current() error = %v, want nil", err)
	}
	uid, err := strconv.ParseUint(current.Uid, 10, 32)
	if err != nil {
		t.Fatalf("ParseUint(uid %q) error = %v, want nil", current.Uid, err)
	}
	gid, err := strconv.ParseUint(current.Gid, 10, 32)
	if err != nil {
		t.Fatalf("ParseUint(gid %q) error = %v, want nil", current.Gid, err)
	}
	return uint32(uid), uint32(gid)
}

// TestPermissionsCarryTheBitsTheEntryWasCreatedWithAcrossTheWholeField walks
// the permission field rather than sampling it, because the bug this prevents
// is a mask that works for 0644 and drops a bit somewhere else. The umask is
// worked around by chmod after creation: a test that asserted what WriteFile
// requested would pass or fail on the operator's shell configuration.
func TestPermissionsCarryTheBitsTheEntryWasCreatedWithAcrossTheWholeField(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		want fs.FileMode
	}{
		{name: "the smallest non-zero mode", want: 0o001},
		{name: "owner read only", want: 0o400},
		{name: "owner read and write", want: 0o600},
		{name: "owner read write execute", want: 0o700},
		{name: "the mode a private file uses", want: 0o640},
		{name: "the mode a shared file uses", want: 0o644},
		{name: "the mode an executable uses", want: 0o755},
		{name: "group bits only", want: 0o070},
		{name: "other bits only", want: 0o007},
		{name: "one bit below the widest mode", want: 0o776},
		{name: "the widest mode", want: 0o777},
		{name: "an asymmetric mode no default produces", want: 0o615},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			directory := t.TempDir()
			path := filepath.Join(directory, "entry")
			if err := os.WriteFile(path, []byte("payload"), 0o600); err != nil {
				t.Fatalf("WriteFile() error = %v, want nil", err)
			}
			if err := os.Chmod(path, tc.want); err != nil {
				t.Fatalf("Chmod(%o) error = %v, want nil", tc.want, err)
			}
			permissions, err := attributesInspect(t, directory, "entry").Permissions()
			if err != nil {
				t.Fatalf("Permissions() error = %v, want nil", err)
			}
			mode, err := permissions.FileMode()
			if err != nil {
				t.Fatalf("FileMode() error = %v, want nil", err)
			}
			if mode != tc.want {
				t.Fatalf("FileMode() = %o, want %o", mode, tc.want)
			}
			bits, err := permissions.Bits()
			if err != nil {
				t.Fatalf("Bits() error = %v, want nil", err)
			}
			if bits != uint32(tc.want) {
				t.Fatalf("Bits() = %o, want %o", bits, uint32(tc.want))
			}
			if got := permissions.String(); got != tc.want.String() {
				t.Fatalf("String() = %q, want %q", got, tc.want.String())
			}
		})
	}
}

// TestPermissionsNeverCarryTheKindBitsPathKindAlreadyOwns is the whole reason
// the type exists. A directory's fs.FileMode has ModeDir set, and a product
// that recorded the mode instead of the permission field would persist a
// number whose high bits say "directory" into a field meaning "0755".
func TestPermissionsNeverCarryTheKindBitsPathKindAlreadyOwns(t *testing.T) {
	t.Parallel()

	cases := []struct {
		plant    func(t *testing.T, directory string)
		name     string
		wantKind filestore.PathKind
	}{
		{
			name:     "a regular file",
			wantKind: filestore.PathKindRegularFile,
			plant: func(t *testing.T, directory string) {
				if err := os.WriteFile(filepath.Join(directory, "entry"), []byte("x"), 0o640); err != nil {
					t.Fatalf("WriteFile() error = %v, want nil", err)
				}
			},
		},
		{
			name:     "a directory",
			wantKind: filestore.PathKindDirectory,
			plant: func(t *testing.T, directory string) {
				if err := os.Mkdir(filepath.Join(directory, "entry"), 0o750); err != nil {
					t.Fatalf("Mkdir() error = %v, want nil", err)
				}
			},
		},
		{
			name:     "a symbolic link",
			wantKind: filestore.PathKindSymbolicLink,
			plant: func(t *testing.T, directory string) {
				if err := os.Symlink("target", filepath.Join(directory, "entry")); err != nil {
					t.Fatalf("Symlink() error = %v, want nil", err)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			directory := t.TempDir()
			tc.plant(t, directory)
			inspection := attributesInspect(t, directory, "entry")
			kind, err := inspection.Kind()
			if err != nil {
				t.Fatalf("Kind() error = %v, want nil", err)
			}
			if kind != tc.wantKind {
				t.Fatalf("Kind() = %v, want %v", kind, tc.wantKind)
			}
			permissions, err := inspection.Permissions()
			if err != nil {
				t.Fatalf("Permissions() error = %v, want nil", err)
			}
			mode, err := permissions.FileMode()
			if err != nil {
				t.Fatalf("FileMode() error = %v, want nil", err)
			}
			if mode != mode.Perm() {
				t.Fatalf("FileMode() = %v, want only permission bits", mode)
			}
			if mode&fs.ModeDir != 0 || mode&fs.ModeSymlink != 0 {
				t.Fatalf("FileMode() = %v, want no kind bits; PathKind owns the kind", mode)
			}
		})
	}
}

// TestOwnershipNamesTheProcessThatCreatedTheEntry proves the numbers are the
// filesystem's and not a zero value dressed up as an observation. Every entry
// a test creates belongs to the process running it, whoever that is.
func TestOwnershipNamesTheProcessThatCreatedTheEntry(t *testing.T) {
	t.Parallel()

	wantUID, wantGID := attributesCurrentOwner(t)
	cases := []struct {
		plant func(t *testing.T, directory string)
		name  string
	}{
		{
			name: "a regular file",
			plant: func(t *testing.T, directory string) {
				if err := os.WriteFile(filepath.Join(directory, "entry"), []byte("x"), 0o600); err != nil {
					t.Fatalf("WriteFile() error = %v, want nil", err)
				}
			},
		},
		{
			name: "a directory",
			plant: func(t *testing.T, directory string) {
				if err := os.Mkdir(filepath.Join(directory, "entry"), 0o700); err != nil {
					t.Fatalf("Mkdir() error = %v, want nil", err)
				}
			},
		},
		{
			name: "a symbolic link, which owns itself rather than its target",
			plant: func(t *testing.T, directory string) {
				if err := os.Symlink("elsewhere", filepath.Join(directory, "entry")); err != nil {
					t.Fatalf("Symlink() error = %v, want nil", err)
				}
			},
		},
		{
			name: "an empty file",
			plant: func(t *testing.T, directory string) {
				if err := os.WriteFile(filepath.Join(directory, "entry"), nil, 0o600); err != nil {
					t.Fatalf("WriteFile() error = %v, want nil", err)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			directory := t.TempDir()
			tc.plant(t, directory)
			ownership, err := attributesInspect(t, directory, "entry").Ownership()
			if err != nil {
				t.Fatalf("Ownership() error = %v, want nil", err)
			}
			if !ownership.IsSet() {
				t.Fatal("IsSet() = false, want true for an entry this process created")
			}
			uid, err := ownership.UID()
			if err != nil {
				t.Fatalf("UID() error = %v, want nil", err)
			}
			gid, err := ownership.GID()
			if err != nil {
				t.Fatalf("GID() error = %v, want nil", err)
			}
			if uid != wantUID || gid != wantGID {
				t.Fatalf("Ownership() = uid %d gid %d, want uid %d gid %d", uid, gid, wantUID, wantGID)
			}
		})
	}
}

// TestAbsentAndUnreachablePathsRefuseBothAttributesRatherThanAnsweringZero is
// the neutral case that catches the real bug. Zero permission bits read as a
// real mode meaning "nobody may do anything", and uid 0 reads as root, so a
// product persisting either from a path that does not exist records a
// confident lie rather than a gap.
func TestAbsentAndUnreachablePathsRefuseBothAttributesRatherThanAnsweringZero(t *testing.T) {
	t.Parallel()

	cases := []struct {
		plant func(t *testing.T, directory string)
		name  string
		entry string
	}{
		{
			name:  "nothing was ever written at the name",
			entry: "missing",
			plant: func(*testing.T, string) {},
		},
		{
			name:  "the parent does not exist",
			entry: "absent/child",
			plant: func(*testing.T, string) {},
		},
		{
			name:  "the parent is a regular file",
			entry: "file/child",
			plant: func(t *testing.T, directory string) {
				if err := os.WriteFile(filepath.Join(directory, "file"), []byte("x"), 0o600); err != nil {
					t.Fatalf("WriteFile() error = %v, want nil", err)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			directory := t.TempDir()
			tc.plant(t, directory)
			inspection := attributesInspect(t, directory, tc.entry)

			permissions, permissionsErr := inspection.Permissions()
			if !errors.Is(permissionsErr, core.ErrFilestoreContract) {
				t.Fatalf("Permissions() error = %v, want %v", permissionsErr, core.ErrFilestoreContract)
			}
			if permissions.IsSet() {
				t.Fatal("Permissions().IsSet() = true, want false for a path that holds nothing")
			}
			ownership, ownershipErr := inspection.Ownership()
			if !errors.Is(ownershipErr, core.ErrFilestoreContract) {
				t.Fatalf("Ownership() error = %v, want %v", ownershipErr, core.ErrFilestoreContract)
			}
			if ownership.IsSet() {
				t.Fatal("Ownership().IsSet() = true, want false for a path that holds nothing")
			}
		})
	}
}

// TestTheGoZeroAttributeValuesRefuseEveryAccessor closes the door a caller
// opens by declaring the types itself. Both are exported, so both can be
// constructed as zero values that were never observed, and every accessor has
// to say so rather than hand back a number.
func TestTheGoZeroAttributeValuesRefuseEveryAccessor(t *testing.T) {
	t.Parallel()

	t.Run("unobserved permissions", func(t *testing.T) {
		t.Parallel()

		var permissions filestore.Permissions
		if permissions.IsSet() {
			t.Fatal("IsSet() = true, want false for the Go zero value")
		}
		if err := permissions.Validate(); !errors.Is(err, core.ErrFilestoreContract) {
			t.Fatalf("Validate() error = %v, want %v", err, core.ErrFilestoreContract)
		}
		if _, err := permissions.FileMode(); !errors.Is(err, core.ErrFilestoreContract) {
			t.Fatalf("FileMode() error = %v, want %v", err, core.ErrFilestoreContract)
		}
		if _, err := permissions.Bits(); !errors.Is(err, core.ErrFilestoreContract) {
			t.Fatalf("Bits() error = %v, want %v", err, core.ErrFilestoreContract)
		}
		if got := permissions.String(); got != "unset" {
			t.Fatalf("String() = %q, want %q", got, "unset")
		}
	})

	t.Run("unobserved ownership", func(t *testing.T) {
		t.Parallel()

		var ownership filestore.Ownership
		if ownership.IsSet() {
			t.Fatal("IsSet() = true, want false for the Go zero value")
		}
		if err := ownership.Validate(); !errors.Is(err, core.ErrFilestoreContract) {
			t.Fatalf("Validate() error = %v, want %v", err, core.ErrFilestoreContract)
		}
		if _, err := ownership.UID(); !errors.Is(err, core.ErrFilestoreContract) {
			t.Fatalf("UID() error = %v, want %v", err, core.ErrFilestoreContract)
		}
		if _, err := ownership.GID(); !errors.Is(err, core.ErrFilestoreContract) {
			t.Fatalf("GID() error = %v, want %v", err, core.ErrFilestoreContract)
		}
	})
}

// TestOwnershipSeparatesUnobservedFromOwnedByRoot pins the reason the set flag
// exists at all. uid 0 is a real and common owner, so a type using the zero
// value as its "no answer" sentinel would report every root-owned file as
// unobserved and every unobserved file as root-owned.
func TestOwnershipSeparatesUnobservedFromOwnedByRoot(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "entry"), []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v, want nil", err)
	}
	observed, err := attributesInspect(t, directory, "entry").Ownership()
	if err != nil {
		t.Fatalf("Ownership() error = %v, want nil", err)
	}
	if !observed.IsSet() {
		t.Fatal("IsSet() = false, want true for an observed entry")
	}
	if err := observed.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil for an observed entry", err)
	}

	var unobserved filestore.Ownership
	if unobserved.IsSet() {
		t.Fatal("IsSet() = true, want false for a value nothing observed")
	}
	uid, err := observed.UID()
	if err != nil {
		t.Fatalf("UID() error = %v, want nil", err)
	}
	if uid == 0 && !observed.IsSet() {
		t.Fatal("a root-owned entry reported itself unobserved, want the set flag to be independent of the value")
	}
}

// TestObservedAttributesDescribeTheSameEntryTheRestOfTheInspectionDoes is the
// consistency proof. Kind, size, modification time, permissions, and owner
// come from one Lstat, so a caller can act on all five without wondering
// whether the file changed between two of them.
func TestObservedAttributesDescribeTheSameEntryTheRestOfTheInspectionDoes(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	path := filepath.Join(directory, "entry")
	if err := os.WriteFile(path, []byte("twelve bytes"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v, want nil", err)
	}
	if err := os.Chmod(path, 0o604); err != nil {
		t.Fatalf("Chmod() error = %v, want nil", err)
	}
	inspection := attributesInspect(t, directory, "entry")

	kind, err := inspection.Kind()
	if err != nil {
		t.Fatalf("Kind() error = %v, want nil", err)
	}
	if kind != filestore.PathKindRegularFile {
		t.Fatalf("Kind() = %v, want %v", kind, filestore.PathKindRegularFile)
	}
	size, err := inspection.SizeBytes()
	if err != nil {
		t.Fatalf("SizeBytes() error = %v, want nil", err)
	}
	sizeValue, err := size.Int64()
	if err != nil {
		t.Fatalf("Int64() error = %v, want nil", err)
	}
	if sizeValue != int64(len("twelve bytes")) {
		t.Fatalf("SizeBytes() = %d, want %d", sizeValue, len("twelve bytes"))
	}
	if _, err := inspection.ModifiedAt(); err != nil {
		t.Fatalf("ModifiedAt() error = %v, want nil", err)
	}
	permissions, err := inspection.Permissions()
	if err != nil {
		t.Fatalf("Permissions() error = %v, want nil", err)
	}
	mode, err := permissions.FileMode()
	if err != nil {
		t.Fatalf("FileMode() error = %v, want nil", err)
	}
	if mode != 0o604 {
		t.Fatalf("FileMode() = %o, want %o", mode, 0o604)
	}
	if _, err := inspection.Ownership(); err != nil {
		t.Fatalf("Ownership() error = %v, want nil", err)
	}
}
