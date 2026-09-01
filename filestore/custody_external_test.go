package filestore_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filelock"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	custodyLockFileMode fs.FileMode = 0o600
	custodyPayload                  = "payload"
	custodyDiagnostics              = "pid 4242"
)

// custodyRoot opens one directory as a rooted capability and closes it when the
// calling test or subtest ends, so every case below names entries the way
// production does and no two parallel cases share one handle.
func custodyRoot(t *testing.T, directory string) *os.Root {
	t.Helper()

	root, err := filestore.OpenRoot(t.Context(), custodyAbsolute(t, directory))
	if err != nil {
		t.Fatalf("OpenRoot(%s) error = %v, want nil", directory, err)
	}
	t.Cleanup(func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Errorf("Close() error = %v, want nil", closeErr)
		}
	})
	return root
}

func custodyAbsolute(t *testing.T, elements ...string) core.AbsolutePath {
	t.Helper()

	path, err := core.ParseAbsolutePath(filepath.Join(elements...))
	if err != nil {
		t.Fatalf("ParseAbsolutePath(%v) error = %v, want nil", elements, err)
	}
	return path
}

func custodyRelative(t *testing.T, name string) core.RelativePath {
	t.Helper()

	path, err := core.ParseRelativePath(name)
	if err != nil {
		t.Fatalf("ParseRelativePath(%s) error = %v, want nil", name, err)
	}
	return path
}

func custodyInstant(t *testing.T, value time.Time) temporal.Instant {
	t.Helper()

	instant, err := temporal.NewInstant(value)
	if err != nil {
		t.Fatalf("NewInstant(%v) error = %v, want nil", value, err)
	}
	return instant
}

// custodyWriteFile plants one regular file with known bytes and a known custody
// stamp so a later assertion can tell "unchanged" from "rewritten".
func custodyWriteFile(t *testing.T, directory, name, contents string, stamp time.Time) string {
	t.Helper()

	path := filepath.Join(directory, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v, want nil", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("WriteFile(%s) error = %v, want nil", name, err)
	}
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatalf("Chtimes(%s) error = %v, want nil", name, err)
	}
	return path
}

// custodyObservedStamp reads one path's custody instant back through the only
// door that exposes it, truncated to the second. Truncation is deliberate: a
// filesystem that keeps coarser timestamps must still return the caller's
// instant to the second, and an assertion demanding nanoseconds would pass on
// APFS and fail on ext3 for a reason that has nothing to do with this package.
func custodyObservedStamp(t *testing.T, path core.AbsolutePath) time.Time {
	t.Helper()

	inspection, err := filestore.Inspect(t.Context(), path)
	if err != nil {
		t.Fatalf("Inspect(%s) error = %v, want nil", path.String(), err)
	}
	modified, err := inspection.ModifiedAt()
	if err != nil {
		t.Fatalf("ModifiedAt() error = %v, want nil", err)
	}
	got, err := modified.Time()
	if err != nil {
		t.Fatalf("Time() error = %v, want nil", err)
	}
	return got.UTC().Truncate(time.Second)
}

func custodyEntryCount(t *testing.T, directory string) int {
	t.Helper()

	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("ReadDir(%s) error = %v, want nil", directory, err)
	}
	return len(entries)
}

// custodyValidator names one request whose Validate gate is under pressure.
// The closure receives the case's own root so no two parallel subtests share a
// capability handle or a directory.
type custodyValidator func(t *testing.T, root *os.Root) error

func custodyTouch(path core.RelativePath, instant temporal.Instant) custodyValidator {
	return func(_ *testing.T, root *os.Root) error {
		return filestore.TouchRequest{
			Location:   filestore.Location{Root: root, Path: path},
			ModifiedAt: instant,
		}.Validate()
	}
}

func custodyDurability(path core.RelativePath) custodyValidator {
	return func(_ *testing.T, root *os.Root) error {
		return filestore.DurabilityRequest{
			Location: filestore.Location{Root: root, Path: path},
		}.Validate()
	}
}

func custodyLockFile(path core.RelativePath, mode fs.FileMode) custodyValidator {
	return func(_ *testing.T, root *os.Root) error {
		return filestore.LockFileRequest{
			Location: filestore.Location{Root: root, Path: path},
			Mode:     mode,
		}.Validate()
	}
}

// custodyRootless drops the rooted capability from an otherwise well-formed
// request, which is the one dimension the case's own root cannot express.
func custodyRootless(validator custodyValidator) custodyValidator {
	return func(t *testing.T, _ *os.Root) error {
		return validator(t, nil)
	}
}

var custodyEpoch = temporal.InstantFromNanoseconds(0)

// TestCustodyRequestAdmissionRefusesEveryMalformedRequest pressures all three
// custody gates from both sides of every dimension they own: the rooted
// capability, the named entry, the permission mode, and the custody instant.
// Each gate stands in front of a real disk effect, so a request that Validate
// lets through is a request that mutates the filesystem.
func TestCustodyRequestAdmissionRefusesEveryMalformedRequest(t *testing.T) {
	t.Parallel()

	unsetPath := core.RelativePath{}
	entry := custodyRelative(t, "entry")
	rooted := custodyRelative(t, ".")
	var unsetInstant temporal.Instant

	cases := []struct {
		validate custodyValidator
		name     string
	}{
		{name: "touch with no rooted capability", validate: custodyRootless(custodyTouch(entry, custodyEpoch))},
		{name: "touch with an unset entry name", validate: custodyTouch(unsetPath, custodyEpoch)},
		{name: "touch naming the rooted entry itself", validate: custodyTouch(rooted, custodyEpoch)},
		{name: "touch with an unset custody instant", validate: custodyTouch(entry, unsetInstant)},
		{name: "touch that is entirely the Go zero request", validate: custodyRootless(custodyTouch(unsetPath, unsetInstant))},
		{name: "touch missing both capability and instant", validate: custodyRootless(custodyTouch(entry, unsetInstant))},
		{name: "touch naming the rooted entry with no instant", validate: custodyTouch(rooted, unsetInstant)},

		{name: "durability with no rooted capability", validate: custodyRootless(custodyDurability(entry))},
		{name: "durability with an unset entry name", validate: custodyDurability(unsetPath)},
		{name: "durability naming the rooted entry itself", validate: custodyDurability(rooted)},
		{name: "durability that is entirely the Go zero request", validate: custodyRootless(custodyDurability(unsetPath))},

		{name: "lock file with no rooted capability", validate: custodyRootless(custodyLockFile(entry, custodyLockFileMode))},
		{name: "lock file with an unset entry name", validate: custodyLockFile(unsetPath, custodyLockFileMode)},
		{name: "lock file naming the rooted entry itself", validate: custodyLockFile(rooted, custodyLockFileMode)},
		{name: "lock file with no permission mode", validate: custodyLockFile(entry, 0)},
		{name: "lock file that is entirely the Go zero request", validate: custodyRootless(custodyLockFile(unsetPath, 0))},
		{name: "lock file mode carrying the directory type bit", validate: custodyLockFile(entry, fs.ModeDir|custodyLockFileMode)},
		{name: "lock file mode carrying the symlink type bit", validate: custodyLockFile(entry, fs.ModeSymlink|custodyLockFileMode)},
		{name: "lock file mode carrying the device type bit", validate: custodyLockFile(entry, fs.ModeDevice|custodyLockFileMode)},
		{name: "lock file mode carrying the named-pipe type bit", validate: custodyLockFile(entry, fs.ModeNamedPipe|custodyLockFileMode)},
		{name: "lock file mode carrying the irregular type bit", validate: custodyLockFile(entry, fs.ModeIrregular|custodyLockFileMode)},
		{name: "lock file mode carrying setuid", validate: custodyLockFile(entry, fs.ModeSetuid|custodyLockFileMode)},
		{name: "lock file mode carrying setgid", validate: custodyLockFile(entry, fs.ModeSetgid|custodyLockFileMode)},
		{name: "lock file mode carrying the sticky bit", validate: custodyLockFile(entry, fs.ModeSticky|custodyLockFileMode)},
		{name: "lock file mode carrying the append-only bit", validate: custodyLockFile(entry, fs.ModeAppend|custodyLockFileMode)},
		{name: "lock file mode carrying the exclusive bit", validate: custodyLockFile(entry, fs.ModeExclusive|custodyLockFileMode)},
		{name: "lock file mode one bit above the permission field", validate: custodyLockFile(entry, fs.FileMode(0o1000))},
		{name: "lock file mode that is only type bits", validate: custodyLockFile(entry, fs.ModeDir)},
		{name: "lock file with every bit set", validate: custodyLockFile(entry, ^fs.FileMode(0))},
		{name: "lock file missing both capability and mode", validate: custodyRootless(custodyLockFile(entry, 0))},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.validate(t, custodyRoot(t, t.TempDir()))
			if !errors.Is(err, core.ErrFilestoreContract) {
				t.Fatalf("Validate() error = %v, want %v", err, core.ErrFilestoreContract)
			}
		})
	}
}

// TestCustodyRequestAdmissionAcceptsEveryWellFormedRequest is the other side of
// the same gates. A validator that rejects everything would pass the refusal
// table above and break every product, so the accepted spectrum is pinned too:
// the permission field at both ends, the custody instant at both signs of the
// epoch, and entry names at depth.
func TestCustodyRequestAdmissionAcceptsEveryWellFormedRequest(t *testing.T) {
	t.Parallel()

	entry := custodyRelative(t, "entry")
	nested := custodyRelative(t, "nested/deeper/entry")
	dotted := custodyRelative(t, ".hidden.lock")
	farPast := custodyInstant(t, time.Date(1901, time.December, 14, 0, 0, 0, 0, time.UTC))
	farFuture := custodyInstant(t, time.Date(2200, time.January, 2, 3, 4, 5, 6, time.UTC))

	cases := []struct {
		validate custodyValidator
		name     string
	}{
		{name: "touch at the Unix epoch", validate: custodyTouch(entry, custodyEpoch)},
		{name: "touch one nanosecond after the epoch", validate: custodyTouch(entry, temporal.InstantFromNanoseconds(1))},
		{name: "touch one nanosecond before the epoch", validate: custodyTouch(entry, temporal.InstantFromNanoseconds(-1))},
		{name: "touch at the smallest representable instant", validate: custodyTouch(entry, temporal.InstantFromNanoseconds(-1<<62))},
		{name: "touch at the largest representable instant", validate: custodyTouch(entry, temporal.InstantFromNanoseconds(1<<62-1))},
		{name: "touch far in the past", validate: custodyTouch(entry, farPast)},
		{name: "touch far in the future", validate: custodyTouch(entry, farFuture)},
		{name: "touch a nested entry", validate: custodyTouch(nested, custodyEpoch)},
		{name: "touch a dot-prefixed entry", validate: custodyTouch(dotted, custodyEpoch)},

		{name: "durability of an entry in the root", validate: custodyDurability(entry)},
		{name: "durability of a nested entry", validate: custodyDurability(nested)},
		{name: "durability of a dot-prefixed entry", validate: custodyDurability(dotted)},

		{name: "lock file at the smallest non-zero permission mode", validate: custodyLockFile(entry, fs.FileMode(0o001))},
		{name: "lock file at the owner-only mode products use", validate: custodyLockFile(entry, custodyLockFileMode)},
		{name: "lock file at a read-only permission mode", validate: custodyLockFile(entry, fs.FileMode(0o400))},
		{name: "lock file at the widest permission mode", validate: custodyLockFile(entry, fs.FileMode(0o777))},
		{name: "lock file naming a nested entry", validate: custodyLockFile(nested, custodyLockFileMode)},
		{name: "lock file naming a dot-prefixed entry", validate: custodyLockFile(dotted, custodyLockFileMode)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := tc.validate(t, custodyRoot(t, t.TempDir())); err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
		})
	}
}

// TestTouchStampsTheInstantTheCallerChoseAndInspectReadsItBack proves the
// custody stamp survives the round trip through the only door that reads it,
// and that stamping custody is not a disguised write: the bytes must be the
// bytes the caller never handed over.
func TestTouchStampsTheInstantTheCallerChoseAndInspectReadsItBack(t *testing.T) {
	t.Parallel()

	cases := []struct {
		want time.Time
		name string
	}{
		{name: "an instant before the epoch", want: time.Date(1969, time.July, 20, 20, 17, 40, 0, time.UTC)},
		{name: "the Unix epoch itself", want: time.Unix(0, 0).UTC()},
		{name: "an instant in the recent past", want: time.Date(2019, time.March, 4, 5, 6, 7, 0, time.UTC)},
		{name: "an instant beyond the 32-bit second ceiling", want: time.Date(2100, time.June, 1, 2, 3, 4, 0, time.UTC)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			directory := t.TempDir()
			custodyWriteFile(t, directory, "object", custodyPayload, time.Unix(1, 0))
			err := filestore.Touch(t.Context(), filestore.TouchRequest{
				Location: filestore.Location{
					Root: custodyRoot(t, directory),
					Path: custodyRelative(t, "object"),
				},
				ModifiedAt: custodyInstant(t, tc.want),
			})
			if err != nil {
				t.Fatalf("Touch() error = %v, want nil", err)
			}
			got := custodyObservedStamp(t, custodyAbsolute(t, directory, "object"))
			if !got.Equal(tc.want.Truncate(time.Second)) {
				t.Fatalf("ModifiedAt() = %v, want %v", got, tc.want.Truncate(time.Second))
			}
			bytes, err := os.ReadFile(filepath.Join(directory, "object"))
			if err != nil {
				t.Fatalf("ReadFile() error = %v, want nil", err)
			}
			if string(bytes) != custodyPayload {
				t.Fatalf("ReadFile() = %q, want %q", string(bytes), custodyPayload)
			}
		})
	}
}

// custodyPlant plants one hostile thing at a name and reports what the door
// must refuse it with.
type custodyPlant struct {
	wantErr error
	plant   func(t *testing.T, directory string)
	name    string
	entry   string
}

// custodyHostileNames is the shared refusal matrix for both name-addressed
// custody doors. Touch and ConfirmDurable resolve a name the same way, so a
// name one of them refuses and the other accepts is the bug, and one table
// keeps them from drifting apart.
func custodyHostileNames() []custodyPlant {
	return []custodyPlant{
		{
			name:    "the name was never written",
			entry:   "missing",
			plant:   func(*testing.T, string) {},
			wantErr: fs.ErrNotExist,
		},
		{
			name:  "a directory took the name",
			entry: "taken",
			plant: func(t *testing.T, directory string) {
				if err := os.Mkdir(filepath.Join(directory, "taken"), 0o700); err != nil {
					t.Fatalf("Mkdir() error = %v, want nil", err)
				}
			},
			wantErr: fs.ErrInvalid,
		},
		{
			name:  "a symbolic link to a file outside the root took the name",
			entry: "escaping",
			plant: func(t *testing.T, directory string) {
				outside := filepath.Join(t.TempDir(), "target")
				if err := os.WriteFile(outside, []byte("elsewhere"), 0o600); err != nil {
					t.Fatalf("WriteFile() error = %v, want nil", err)
				}
				if err := os.Symlink(outside, filepath.Join(directory, "escaping")); err != nil {
					t.Fatalf("Symlink() error = %v, want nil", err)
				}
			},
			wantErr: nil,
		},
		{
			name:  "a dangling symbolic link took the name",
			entry: "dangling",
			plant: func(t *testing.T, directory string) {
				if err := os.Symlink("nowhere", filepath.Join(directory, "dangling")); err != nil {
					t.Fatalf("Symlink() error = %v, want nil", err)
				}
			},
			wantErr: nil,
		},
		{
			name:  "the parent of the name is a regular file",
			entry: "file/child",
			plant: func(t *testing.T, directory string) {
				if err := os.WriteFile(filepath.Join(directory, "file"), []byte("x"), 0o600); err != nil {
					t.Fatalf("WriteFile() error = %v, want nil", err)
				}
			},
			wantErr: nil,
		},
	}
}

// TestTouchRefusesEveryNameThatIsNotTheRegularFileTheCallerMeant proves the
// stamp cannot land on something merely wearing the file's name. The symbolic
// link case matters most: os.Chtimes follows one, so a product stamping by bare
// path writes custody onto whatever the link points at, which is the escape the
// rooted boundary exists to stop.
func TestTouchRefusesEveryNameThatIsNotTheRegularFileTheCallerMeant(t *testing.T) {
	t.Parallel()

	for _, tc := range custodyHostileNames() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			directory := t.TempDir()
			tc.plant(t, directory)
			err := filestore.Touch(t.Context(), filestore.TouchRequest{
				Location: filestore.Location{
					Root: custodyRoot(t, directory),
					Path: custodyRelative(t, tc.entry),
				},
				ModifiedAt: custodyEpoch,
			})
			if !errors.Is(err, core.ErrFilestoreSource) {
				t.Fatalf("Touch() error = %v, want %v", err, core.ErrFilestoreSource)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("Touch() error = %v, want it to wrap %v", err, tc.wantErr)
			}
		})
	}
}

// TestTouchRefusingARequestLeavesTheNamedFileExactlyAsItFoundIt proves the
// contract gate runs before any effect. A refused request that had already
// restamped the file would make Validate a suggestion.
func TestTouchRefusingARequestLeavesTheNamedFileExactlyAsItFoundIt(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	planted := time.Date(2011, time.May, 6, 7, 8, 9, 0, time.UTC)
	custodyWriteFile(t, directory, "object", custodyPayload, planted)
	request := filestore.TouchRequest{
		Location: filestore.Location{
			Root: custodyRoot(t, directory),
			Path: custodyRelative(t, "object"),
		},
	}
	if err := filestore.Touch(t.Context(), request); !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatalf("Touch() error = %v, want %v", err, core.ErrFilestoreContract)
	}
	got := custodyObservedStamp(t, custodyAbsolute(t, directory, "object"))
	if !got.Equal(planted) {
		t.Fatalf("ModifiedAt() = %v, want %v (a refused request must not stamp)", got, planted)
	}
	if count := custodyEntryCount(t, directory); count != 1 {
		t.Fatalf("directory entries = %d, want 1", count)
	}
}

// TestConfirmDurableAcceptsAWrittenNameAndLeavesItsBytesAndStampAlone proves
// the proof is read-side. Confirming durability must not become a second way to
// modify the record it was asked about, because a reclamation pass reading the
// custody stamp afterwards would see every confirmed record as freshly wanted.
func TestConfirmDurableAcceptsAWrittenNameAndLeavesItsBytesAndStampAlone(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	planted := time.Date(2021, time.July, 8, 9, 10, 11, 0, time.UTC)
	custodyWriteFile(t, directory, "record", "record bytes", planted)
	err := filestore.ConfirmDurable(t.Context(), filestore.DurabilityRequest{
		Location: filestore.Location{
			Root: custodyRoot(t, directory),
			Path: custodyRelative(t, "record"),
		},
	})
	if err != nil {
		t.Fatalf("ConfirmDurable() error = %v, want nil", err)
	}
	bytes, err := os.ReadFile(filepath.Join(directory, "record"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v, want nil", err)
	}
	if string(bytes) != "record bytes" {
		t.Fatalf("ReadFile() = %q, want %q", string(bytes), "record bytes")
	}
	got := custodyObservedStamp(t, custodyAbsolute(t, directory, "record"))
	if !got.Equal(planted) {
		t.Fatalf("ModifiedAt() = %v, want %v (ConfirmDurable must not restamp)", got, planted)
	}
}

// TestConfirmDurableRefusesANameThatIsNoLongerTheRecordItClaimsToBe proves the
// caller learns the truth instead of receiving a false proof. A product asking
// whether a record survived a restart is exactly the product that must be told
// when the name now belongs to a directory, a link, or nothing at all.
func TestConfirmDurableRefusesANameThatIsNoLongerTheRecordItClaimsToBe(t *testing.T) {
	t.Parallel()

	for _, tc := range custodyHostileNames() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			directory := t.TempDir()
			tc.plant(t, directory)
			err := filestore.ConfirmDurable(t.Context(), filestore.DurabilityRequest{
				Location: filestore.Location{
					Root: custodyRoot(t, directory),
					Path: custodyRelative(t, tc.entry),
				},
			})
			if !errors.Is(err, core.ErrFilestoreSource) {
				t.Fatalf("ConfirmDurable() error = %v, want %v", err, core.ErrFilestoreSource)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("ConfirmDurable() error = %v, want it to wrap %v", err, tc.wantErr)
			}
		})
	}
}

// TestOpenLockFileProducesTheHandleFilelockAccepts is the whole reason the door
// exists: the two capabilities were built to meet and had nothing to meet
// through. The lock is taken, contended against from a second handle opened the
// same way, released, and regained, all on handles this package produced.
func TestOpenLockFileProducesTheHandleFilelockAccepts(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	root := custodyRoot(t, directory)
	first := custodyOpenLockFile(t, root, "product.lock")
	second := custodyOpenLockFile(t, root, "product.lock")

	if held := custodyTakeLock(t, first); !held {
		t.Fatal("Acquire(first) held = false, want true")
	}
	if held := custodyTakeLock(t, second); held {
		t.Fatal("Acquire(second) held = true, want false while the first holder holds it")
	}
	if err := filelock.Release(t.Context(), first); err != nil {
		t.Fatalf("Release(first) error = %v, want nil", err)
	}
	if held := custodyTakeLock(t, second); !held {
		t.Fatal("Acquire(second, after release) held = false, want true")
	}
	if err := filelock.Release(t.Context(), second); err != nil {
		t.Fatalf("Release(second) error = %v, want nil", err)
	}
	if count := custodyEntryCount(t, directory); count != 1 {
		t.Fatalf("directory entries = %d, want 1 (both handles name one lock file)", count)
	}
}

func custodyOpenLockFile(t *testing.T, root *os.Root, name string) *os.File {
	t.Helper()

	file, err := filestore.OpenLockFile(t.Context(), filestore.LockFileRequest{
		Location: filestore.Location{Root: root, Path: custodyRelative(t, name)},
		Mode:     custodyLockFileMode,
	})
	if err != nil {
		t.Fatalf("OpenLockFile(%s) error = %v, want nil", name, err)
	}
	t.Cleanup(func() {
		if closeErr := file.Close(); closeErr != nil {
			t.Errorf("Close(%s) error = %v, want nil", name, closeErr)
		}
	})
	return file
}

func custodyTakeLock(t *testing.T, file *os.File) bool {
	t.Helper()

	acquisition, err := filelock.Acquire(t.Context(), filelock.Request{
		File:        file,
		Exclusivity: filelock.Exclusive,
		Patience:    filelock.Immediate,
	})
	if err != nil {
		t.Fatalf("Acquire() error = %v, want nil", err)
	}
	held, err := acquisition.Held()
	if err != nil {
		t.Fatalf("Held() error = %v, want nil", err)
	}
	return held
}

// TestOpenLockFileHandleIsReadableAndRewritable proves the handle is not the
// append-only one OpenAppend hands out. A lock holder records its own
// diagnostics at offset zero and a later holder overwrites them, so a handle
// that can only append is the wrong handle even though it opens the same file.
func TestOpenLockFileHandleIsReadableAndRewritable(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	file := custodyOpenLockFile(t, custodyRoot(t, directory), "held.lock")
	if _, err := file.WriteAt([]byte(custodyDiagnostics), 0); err != nil {
		t.Fatalf("WriteAt() error = %v, want nil", err)
	}
	if err := file.Truncate(int64(len("pid 99"))); err != nil {
		t.Fatalf("Truncate() error = %v, want nil", err)
	}
	if _, err := file.WriteAt([]byte("pid 99"), 0); err != nil {
		t.Fatalf("WriteAt(rewrite) error = %v, want nil", err)
	}
	if got := custodyReadAll(t, file); got != "pid 99" {
		t.Fatalf("ReadAt() = %q, want %q", got, "pid 99")
	}
}

func custodyReadAll(t *testing.T, file *os.File) string {
	t.Helper()

	buffer := make([]byte, 64)
	count, err := file.ReadAt(buffer, 0)
	if err != nil && !errors.Is(err, os.ErrDeadlineExceeded) && count == 0 {
		t.Fatalf("ReadAt() error = %v with %d bytes, want the file contents", err, count)
	}
	return string(buffer[:count])
}

// TestOpenLockFileRefusesEveryRequestThatNamesNoRealLockCarrier proves the gate
// runs before the file is created. The zero permission mode matters most:
// os.OpenFile accepts it and leaves behind a file nobody, including the holder
// that just made it, can reopen.
func TestOpenLockFileRefusesEveryRequestThatNamesNoRealLockCarrier(t *testing.T) {
	t.Parallel()

	cases := []struct {
		request func(root *os.Root) filestore.LockFileRequest
		name    string
	}{
		{
			name: "no permission mode",
			request: func(root *os.Root) filestore.LockFileRequest {
				return filestore.LockFileRequest{
					Location: filestore.Location{Root: root, Path: custodyRelative(t, "a.lock")},
				}
			},
		},
		{
			name: "permission mode carrying type bits",
			request: func(root *os.Root) filestore.LockFileRequest {
				return filestore.LockFileRequest{
					Location: filestore.Location{Root: root, Path: custodyRelative(t, "b.lock")},
					Mode:     fs.ModeDir | custodyLockFileMode,
				}
			},
		},
		{
			name: "no rooted capability",
			request: func(*os.Root) filestore.LockFileRequest {
				return filestore.LockFileRequest{
					Location: filestore.Location{Path: custodyRelative(t, "c.lock")},
					Mode:     custodyLockFileMode,
				}
			},
		},
		{
			name: "naming the rooted entry itself",
			request: func(root *os.Root) filestore.LockFileRequest {
				return filestore.LockFileRequest{
					Location: filestore.Location{Root: root, Path: custodyRelative(t, ".")},
					Mode:     custodyLockFileMode,
				}
			},
		},
		{
			name: "an unset entry name",
			request: func(root *os.Root) filestore.LockFileRequest {
				return filestore.LockFileRequest{
					Location: filestore.Location{Root: root},
					Mode:     custodyLockFileMode,
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			directory := t.TempDir()
			file, err := filestore.OpenLockFile(t.Context(), tc.request(custodyRoot(t, directory)))
			if !errors.Is(err, core.ErrFilestoreContract) {
				t.Fatalf("OpenLockFile() error = %v, want %v", err, core.ErrFilestoreContract)
			}
			if file != nil {
				t.Fatal("OpenLockFile() file != nil, want no handle on a refused request")
			}
			if count := custodyEntryCount(t, directory); count != 0 {
				t.Fatalf("directory entries = %d, want 0 (a refused request must create nothing)", count)
			}
		})
	}
}

// TestCustodyDurableWriterLayerTriad covers the three custody doors at the
// durable-writer layer with the positive, negative, and neutral cases the layer
// owes. Neutral is the one that catches the real bug class here: a repeated
// custody operation that quietly manufactures durable noise, or a lock file
// reopen that erases the diagnostics the previous holder left behind.
func TestCustodyDurableWriterLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive: each door performs its own effect and nothing more", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir()
		root := custodyRoot(t, directory)
		custodyWriteFile(t, directory, "object", custodyPayload, time.Unix(1, 0))
		want := time.Date(2024, time.February, 29, 12, 0, 0, 0, time.UTC)
		touchErr := filestore.Touch(t.Context(), filestore.TouchRequest{
			Location:   filestore.Location{Root: root, Path: custodyRelative(t, "object")},
			ModifiedAt: custodyInstant(t, want),
		})
		if touchErr != nil {
			t.Fatalf("Touch() error = %v, want nil", touchErr)
		}
		confirmErr := filestore.ConfirmDurable(t.Context(), filestore.DurabilityRequest{
			Location: filestore.Location{Root: root, Path: custodyRelative(t, "object")},
		})
		if confirmErr != nil {
			t.Fatalf("ConfirmDurable() error = %v, want nil", confirmErr)
		}
		if got := custodyObservedStamp(t, custodyAbsolute(t, directory, "object")); !got.Equal(want) {
			t.Fatalf("ModifiedAt() = %v, want %v", got, want)
		}
		if count := custodyEntryCount(t, directory); count != 1 {
			t.Fatalf("directory entries = %d, want 1", count)
		}
	})

	t.Run("negative: a cancelled context stops every door before its effect", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir()
		root := custodyRoot(t, directory)
		planted := time.Date(2013, time.April, 5, 6, 7, 8, 0, time.UTC)
		custodyWriteFile(t, directory, "object", custodyPayload, planted)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		touchErr := filestore.Touch(ctx, filestore.TouchRequest{
			Location:   filestore.Location{Root: root, Path: custodyRelative(t, "object")},
			ModifiedAt: custodyInstant(t, time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC)),
		})
		if !errors.Is(touchErr, context.Canceled) {
			t.Fatalf("Touch() error = %v, want %v", touchErr, context.Canceled)
		}
		confirmErr := filestore.ConfirmDurable(ctx, filestore.DurabilityRequest{
			Location: filestore.Location{Root: root, Path: custodyRelative(t, "object")},
		})
		if !errors.Is(confirmErr, context.Canceled) {
			t.Fatalf("ConfirmDurable() error = %v, want %v", confirmErr, context.Canceled)
		}
		lockFile, lockErr := filestore.OpenLockFile(ctx, filestore.LockFileRequest{
			Location: filestore.Location{Root: root, Path: custodyRelative(t, "cancelled.lock")},
			Mode:     custodyLockFileMode,
		})
		if !errors.Is(lockErr, context.Canceled) {
			t.Fatalf("OpenLockFile() error = %v, want %v", lockErr, context.Canceled)
		}
		if lockFile != nil {
			t.Fatal("OpenLockFile() file != nil, want no handle on a cancelled context")
		}
		if got := custodyObservedStamp(t, custodyAbsolute(t, directory, "object")); !got.Equal(planted) {
			t.Fatalf("ModifiedAt() = %v, want %v (a cancelled Touch must not stamp)", got, planted)
		}
		if count := custodyEntryCount(t, directory); count != 1 {
			t.Fatalf("directory entries = %d, want 1 (a cancelled OpenLockFile must create nothing)", count)
		}
	})

	t.Run("neutral: repeating each door changes nothing it did not already change", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir()
		root := custodyRoot(t, directory)
		custodyWriteFile(t, directory, "object", custodyPayload, time.Unix(1, 0))
		want := time.Date(2017, time.November, 3, 4, 5, 6, 0, time.UTC)
		request := filestore.TouchRequest{
			Location:   filestore.Location{Root: root, Path: custodyRelative(t, "object")},
			ModifiedAt: custodyInstant(t, want),
		}
		for attempt := 1; attempt <= 3; attempt++ {
			if err := filestore.Touch(t.Context(), request); err != nil {
				t.Fatalf("Touch(attempt %d) error = %v, want nil", attempt, err)
			}
			if err := filestore.ConfirmDurable(t.Context(), filestore.DurabilityRequest{
				Location: filestore.Location{Root: root, Path: custodyRelative(t, "object")},
			}); err != nil {
				t.Fatalf("ConfirmDurable(attempt %d) error = %v, want nil", attempt, err)
			}
		}
		if got := custodyObservedStamp(t, custodyAbsolute(t, directory, "object")); !got.Equal(want) {
			t.Fatalf("ModifiedAt() = %v, want %v after repeats", got, want)
		}

		first := custodyOpenLockFile(t, root, "repeat.lock")
		if _, err := first.WriteAt([]byte(custodyDiagnostics), 0); err != nil {
			t.Fatalf("WriteAt() error = %v, want nil", err)
		}
		second := custodyOpenLockFile(t, root, "repeat.lock")
		if got := custodyReadAll(t, second); got != custodyDiagnostics {
			t.Fatalf("reopened lock file = %q, want %q (OpenLockFile must not truncate)", got, custodyDiagnostics)
		}
		if count := custodyEntryCount(t, directory); count != 2 {
			t.Fatalf("directory entries = %d, want 2 (repeats must not manufacture files)", count)
		}
	})
}
