package filestore_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestInspectOpenFileObservesHeldInodeAfterNameReplacement(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	path := filepath.Join(directory, "subject")
	if err := os.WriteFile(path, []byte("original"), 0o640); err != nil {
		t.Fatal(err)
	}
	held, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := held.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := os.Rename(path, filepath.Join(directory, "previous")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("replacement is longer"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := filestore.InspectOpenFile(t.Context(), filestore.HandleInspectionRequest{File: held})
	if err != nil {
		t.Fatal(err)
	}
	length, err := got.SizeBytes()
	if err != nil || length.Uint64() != uint64(len("original")) {
		t.Fatalf("held extent = (%v,%v), want original inode", length, err)
	}
	kind, err := got.Kind()
	if err != nil || kind != filestore.PathKindRegularFile {
		t.Fatalf("held kind = (%v,%v)", kind, err)
	}
	var first [1]byte
	if _, err := held.Read(first[:]); err != nil || first[0] != 'o' {
		t.Fatalf("inspection consumed or closed caller handle: (%q,%v)", first, err)
	}
}

func TestInspectOpenFileRefusesInvalidContextAndHandle(t *testing.T) {
	t.Parallel()
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	cases := []struct {
		name string
		ctx  context.Context
		file *os.File
		want error
	}{
		{name: "nil context", want: core.ErrNilContext},
		{name: "canceled context", ctx: canceled, want: context.Canceled},
		{name: "missing handle", ctx: t.Context(), want: core.ErrFilestoreContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := filestore.InspectOpenFile(tc.ctx, filestore.HandleInspectionRequest{File: tc.file})
			if !errors.Is(err, tc.want) || got.Validate() == nil {
				t.Fatalf("refused inspection = (%+v,%v), want unobserved result and %v", got, err, tc.want)
			}
		})
	}
	file, err := os.Create(filepath.Join(t.TempDir(), "closed"))
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := filestore.InspectOpenFile(t.Context(), filestore.HandleInspectionRequest{File: file})
	if !errors.Is(err, fs.ErrClosed) || !errors.Is(err, core.ErrFilestoreSource) || got.Validate() == nil {
		t.Fatalf("closed inspection = (%+v,%v), want preserved native refusal", got, err)
	}
}

func FuzzInspectOpenFilePreservesNativeExtent(f *testing.F) {
	f.Add([]byte("content"))
	f.Add([]byte{})
	f.Add([]byte{0, 1, 255})
	f.Fuzz(func(t *testing.T, content []byte) {
		if len(content) > 4096 {
			t.Skip("bounded development corpus")
		}
		path := filepath.Join(t.TempDir(), "subject")
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		got, observeErr := filestore.InspectOpenFile(t.Context(), filestore.HandleInspectionRequest{File: file})
		if err := errors.Join(observeErr, file.Close()); err != nil {
			t.Fatal(err)
		}
		size, err := got.SizeBytes()
		if err != nil || size.Uint64() != uint64(len(content)) || got.Validate() != nil {
			t.Fatalf("native observed extent = (%v,%v), want %d", size, err, len(content))
		}
	})
}
