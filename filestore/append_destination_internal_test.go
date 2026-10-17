package filestore

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestAppendDestinationRejectsUnsetCopiedAndSettledCustody(t *testing.T) {
	t.Parallel()
	var nilOwner *AppendDestination
	for _, d := range []*AppendDestination{nilOwner, {}} {
		if d.Validate() == nil || !errors.Is(d.Close(), core.ErrFilestoreContract) {
			t.Fatal("unset owner accepted")
		}
		if _, err := d.Write(nil); !errors.Is(err, core.ErrFilestoreContract) {
			t.Fatal(err)
		}
		if !errors.Is(d.Flush(), core.ErrFilestoreContract) || !errors.Is(d.Sync(), core.ErrFilestoreContract) {
			t.Fatal("unset I/O accepted")
		}
	}
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	path, err := core.ParseRelativePath("subject")
	if err != nil {
		t.Fatal(err)
	}
	d, err := OpenAppendDestination(t.Context(), AppendRequest{Location: Location{Root: root, Path: path}, Mode: 0o600, Append: AppendCreate})
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	copied := *d
	if copied.Validate() == nil || !errors.Is(copied.Close(), core.ErrFilestoreContract) {
		t.Fatal("copy acquired custody")
	}
	if _, err := copied.Write([]byte("foreign")); !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatal(err)
	}
	if _, err := d.Write([]byte("original")); err != nil {
		t.Fatal(err)
	}
	file := d.file
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("native handle retained=%v", err)
	}
	if d.Validate() == nil {
		t.Fatal("settled owner validated")
	}
}

func TestAppendDestinationNativeFailureStillClosesAndRefusesRotation(t *testing.T) {
	t.Parallel()
	for _, alreadyClosed := range []bool{false, true} {
		t.Run(fmt.Sprintf("closed_%t", alreadyClosed), func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			path := dir + "/subject"
			if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
				t.Fatal(err)
			}
			file, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			d := &AppendDestination{file: file, writer: bufio.NewWriter(file)}
			d.self = d
			if _, err := d.Write([]byte("refused")); err != nil {
				t.Fatal(err)
			}
			if alreadyClosed {
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if err := d.Sync(); err == nil {
				t.Fatal("failed flush claimed durability")
			}
			root, err := os.OpenRoot(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			name, err := core.ParseRelativePath("incoming")
			if err != nil {
				t.Fatal(err)
			}
			err = d.Rotate(t.Context(), AppendRequest{Location: Location{Root: root, Path: name}, Mode: 0o600, Append: AppendCreate})
			var native *os.PathError
			if !errors.Is(err, core.ErrFilestoreActivation) || !errors.As(err, &native) {
				t.Fatalf("rotate native failure=%v", err)
			}
			if alreadyClosed && !errors.Is(err, os.ErrClosed) {
				t.Fatalf("lost closed cause=%v", err)
			}
			if _, err := file.Stat(); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("failed rotation leaked handle=%v", err)
			}
			if _, err := os.Stat(dir + "/incoming"); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("created incoming after failed settle=%v", err)
			}
			if _, err := d.Write([]byte("zombie")); !errors.Is(err, core.ErrFilestoreContract) {
				t.Fatal(err)
			}
			if err := d.Close(); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != "original" {
				t.Fatalf("refused write changed native bytes=%q,%v", got, err)
			}
		})
	}
}
