package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestCreatedDirectoryDestinationNativeCustodyCannotBeCopied(t *testing.T) {
	t.Parallel()
	parent := requireTestRoot(t, t.TempDir())
	request := filestore.DirectoryRequest{Location: filestore.Location{Root: parent, Path: mustRelativePath(t, "owned")}, Mode: 0750}
	destination, err := filestore.OpenCreatedDirectoryDestination(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	root, err := destination.Root()
	if err != nil {
		t.Fatal(err)
	}
	location, err := destination.Location()
	if err != nil || location != request.Location {
		t.Fatalf("native acquisition coordinate=%+v/%v", location, err)
	}
	copied := *destination
	for _, candidate := range []*filestore.CreatedDirectoryDestination{nil, {}, &copied} {
		if !errors.Is(candidate.Validate(), core.ErrFilestoreContract) {
			t.Fatal("unset or copied directory acquired custody")
		}
		if loan, err := candidate.Root(); loan != nil || !errors.Is(err, core.ErrFilestoreContract) {
			t.Fatalf("copied native root=%v/%v", loan, err)
		}
		if loan, err := candidate.Location(); loan != (filestore.Location{}) || !errors.Is(err, core.ErrFilestoreContract) {
			t.Fatalf("copied location=%+v/%v", loan, err)
		}
		if err := filestore.FinishCreatedDirectoryDestination(t.Context(), candidate); !errors.Is(err, core.ErrFilestoreContract) {
			t.Fatalf("copied retention=%v", err)
		}
		if err := filestore.AbandonCreatedDirectoryDestination(candidate); !errors.Is(err, core.ErrFilestoreContract) {
			t.Fatalf("copied cleanup=%v", err)
		}
	}
	if _, err := root.Stat("."); err != nil {
		t.Fatalf("a refused copy settled original native custody: %v", err)
	}
	if err := filestore.FinishCreatedDirectoryDestination(t.Context(), destination); err != nil {
		t.Fatal(err)
	}
	if _, err := root.Stat("."); !errors.Is(err, fs.ErrClosed) {
		t.Fatalf("settled native root remained open: %v", err)
	}
	if _, err := parent.Stat("owned"); err != nil {
		t.Fatalf("retained directory vanished: %v", err)
	}
	if err := filestore.AbandonCreatedDirectoryDestination(destination); !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatalf("retention acknowledged a second cleanup: %v", err)
	}
	if err := filestore.FinishCreatedDirectoryDestination(t.Context(), destination); !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatalf("retention acknowledged a second finish: %v", err)
	}
}

func TestCreatedDirectoryDestinationCancelledRetentionRemovesNativeTree(t *testing.T) {
	t.Parallel()
	for _, closeRoot := range []bool{false, true} {
		t.Run(map[bool]string{false: "cancelled original operation", true: "closed borrowed root"}[closeRoot], func(t *testing.T) {
			directory := t.TempDir()
			parent := requireTestRoot(t, directory)
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			destination, err := filestore.OpenCreatedDirectoryDestination(ctx, filestore.DirectoryRequest{Location: filestore.Location{Root: parent, Path: mustRelativePath(t, "owned")}, Mode: 0700})
			if err != nil {
				t.Fatal(err)
			}
			root, err := destination.Root()
			if err != nil {
				t.Fatal(err)
			}
			file, err := root.OpenFile("accepted", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if err != nil {
				t.Fatal(err)
			}
			_, writeErr := file.Write([]byte("accepted native bytes"))
			if err := errors.Join(writeErr, file.Close()); err != nil {
				t.Fatal(err)
			}
			if closeRoot {
				if err := root.Close(); err != nil {
					t.Fatal(err)
				}
			} else {
				cancel(io.ErrClosedPipe)
			}
			err = filestore.FinishCreatedDirectoryDestination(ctx, destination)
			if closeRoot && !errors.Is(err, fs.ErrClosed) || !closeRoot && (!errors.Is(err, context.Canceled) || !errors.Is(err, io.ErrClosedPipe)) {
				t.Fatalf("failed native retention = %v", err)
			}
			if _, err := os.Stat(filepath.Join(directory, "owned")); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("refused retention left a native tree: %v", err)
			}
			if _, err := root.Stat("."); !errors.Is(err, fs.ErrClosed) {
				t.Fatalf("failed retention retained loan: %v", err)
			}
			if err := destination.Validate(); !errors.Is(err, core.ErrFilestoreContract) {
				t.Fatalf("failed retention left usable custody: %v", err)
			}
		})
	}
}

func FuzzCreatedDirectoryDestinationNativeBytesAndSettlement(f *testing.F) {
	for _, data := range [][]byte{nil, []byte("actual native bytes\x00\n"), {0xff, 0x80}, bytes.Repeat([]byte("z"), 65537)} {
		for _, retain := range []bool{false, true} {
			f.Add(data, retain, false)
			f.Add(data, retain, true)
		}
	}
	f.Fuzz(func(t *testing.T, data []byte, retain, foreign bool) {
		directory := t.TempDir()
		parent := requireTestRoot(t, directory)
		request := filestore.DirectoryRequest{Location: filestore.Location{Root: parent, Path: mustRelativePath(t, "owned")}, Mode: 0750}
		destination, err := filestore.OpenCreatedDirectoryDestination(t.Context(), request)
		if err != nil {
			t.Fatal(err)
		}
		root, err := destination.Root()
		if err != nil {
			t.Fatal(err)
		}
		location, err := destination.Location()
		if err != nil || location != request.Location {
			t.Fatalf("native location=%+v/%v", location, err)
		}
		file, err := root.OpenFile("accepted", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			t.Fatal(err)
		}
		n, writeErr := file.Write(data)
		if err := errors.Join(writeErr, file.Close()); err != nil || n != len(data) {
			t.Fatalf("native producer accepted %d/%d: %v", n, len(data), err)
		}
		if foreign {
			if err := parent.Rename("owned", "moved-original"); err != nil {
				t.Fatal(err)
			}
			if err := parent.Mkdir("owned", 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, "owned", "foreign"), []byte("preserve foreign identity"), 0600); err != nil {
				t.Fatal(err)
			}
		}
		if retain {
			err = filestore.FinishCreatedDirectoryDestination(t.Context(), destination)
		} else {
			err = filestore.AbandonCreatedDirectoryDestination(destination)
		}
		if foreign {
			if !errors.Is(err, core.ErrFilestoreConflict) {
				t.Fatalf("foreign native namespace acknowledged settlement: %v", err)
			}
			body, readErr := os.ReadFile(filepath.Join(directory, "owned", "foreign"))
			if readErr != nil || string(body) != "preserve foreign identity" {
				t.Fatalf("foreign native bytes changed: %q/%v", body, readErr)
			}
			original, readErr := os.ReadFile(filepath.Join(directory, "moved-original", "accepted"))
			if readErr != nil || !bytes.Equal(original, data) {
				t.Fatalf("foreign-name refusal lost original native evidence: %v", readErr)
			}
		} else if err != nil {
			t.Fatalf("native settlement: %v", err)
		} else if retain {
			body, readErr := os.ReadFile(filepath.Join(directory, "owned", "accepted"))
			info, statErr := os.Stat(filepath.Join(directory, "owned"))
			if readErr != nil || !bytes.Equal(body, data) || statErr != nil || info.Mode().Perm() != request.Mode {
				t.Fatalf("native retained bytes/mode disagree: %v/%v", readErr, statErr)
			}
		} else if _, statErr := os.Stat(filepath.Join(directory, "owned")); !errors.Is(statErr, fs.ErrNotExist) {
			t.Fatalf("abandoned native directory survived: %v", statErr)
		}
		if _, err := root.Stat("."); !errors.Is(err, fs.ErrClosed) {
			t.Fatalf("settlement retained native root: %v", err)
		}
		if err := destination.Validate(); !errors.Is(err, core.ErrFilestoreContract) {
			t.Fatalf("settled destination retained custody: %v", err)
		}
	})
}
