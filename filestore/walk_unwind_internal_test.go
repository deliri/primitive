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

// The public walker transfers its actual acquired Go handle into this owner.
// Stat tests that precise handle, avoiding descriptor census or finalizers.
func TestWalkVisitorUnwindCustodyLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                                 string
		entry, panics, returnsError, cancels bool
	}{
		{name: "empty directory never invokes a panicking visitor", panics: true},
		{name: "successful observation closes its directory", entry: true},
		{name: "visitor error keeps native identity and closes its directory", entry: true, returnsError: true},
		{name: "visitor cancellation closes before returning", entry: true, cancels: true},
		{name: "visitor panic cannot leak the acquired directory", entry: true, panics: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			if tc.entry {
				if err := os.WriteFile(filepath.Join(directory, "entry"), []byte{0, 255}, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			file, err := os.Open(directory)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := file.Close(); err != nil && !errors.Is(err, fs.ErrClosed) {
					t.Error(err)
				}
			})
			before, err := file.Stat()
			if err != nil {
				t.Fatal(err)
			}
			path, err := core.ParseRelativePath(".")
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			native := &fs.PathError{Op: "visit", Path: "entry", Err: fs.ErrPermission}
			visits := 0
			request := WalkRequest{Visit: func(entry WalkEntry) (WalkDirective, error) {
				if err := entry.Validate(); err != nil {
					return WalkDirectiveUnknown, err
				}
				visits++
				if entry.Path.String() != "entry" || !entry.Entry.Type().IsRegular() {
					t.Errorf("entry = %v, want original regular entry", entry)
				}
				if tc.panics {
					panic(native)
				}
				if tc.cancels {
					cancel()
				}
				if tc.returnsError {
					return WalkContinue, native
				}
				return WalkContinue, nil
			}}
			var gotErr error
			var gotPanic any
			returned := false
			func() {
				defer func() { gotPanic = recover() }()
				gotErr = walkOwnedDirectory(walkDirectoryInput{ctx: ctx, directoryPath: path, request: request}, file)
				returned = true
			}()
			wantPanic := tc.entry && tc.panics
			var wantErr error
			if tc.returnsError {
				wantErr = native
			}
			if tc.cancels {
				wantErr = context.Canceled
			}
			if wantPanic {
				if gotPanic != native || returned || gotErr != nil {
					t.Errorf("unwind = (%v,%t,%v), want original panic without returned success", gotPanic, returned, gotErr)
				}
			} else if gotPanic != nil || !returned || !errors.Is(gotErr, wantErr) || errors.Is(gotErr, core.ErrFilestoreSource) {
				t.Errorf("return = (%v,%t,%v), want normal return with %v", gotPanic, returned, gotErr, wantErr)
			}
			wantVisits := 0
			if tc.entry {
				wantVisits = 1
			}
			if visits != wantVisits {
				t.Errorf("visits = %d, want %d", visits, wantVisits)
			}
			if _, err := file.Stat(); !errors.Is(err, fs.ErrClosed) {
				t.Errorf("owned directory Stat = %v, want closed before return or unwind", err)
			}
			after, err := os.Stat(directory)
			if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.ModTime() != after.ModTime() {
				t.Errorf("directory custody = (%v,%v), want unchanged original", after, err)
			}
			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != wantVisits {
				t.Errorf("namespace = (%d,%v), want %d retained entries", len(entries), err, wantVisits)
			}
		})
	}
}
