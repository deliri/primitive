package filestore_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestDurableWriterLayerTriadCreateReplaceAndNeutralEffects(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr   error
		name      string
		initial   string
		source    string
		want      string
		install   filestore.InstallMode
		wantExist bool
	}{
		{name: "positive create publishes an absent target", source: "created", install: filestore.InstallCreate, want: "created", wantExist: true},
		{name: "negative create refuses an existing target", initial: "existing", source: "new", install: filestore.InstallCreate, want: "existing", wantErr: core.ErrFilestoreConflict, wantExist: true},
		{name: "positive replace publishes over an existing target", initial: "existing", source: "replacement", install: filestore.InstallReplace, want: "replacement", wantExist: true},
		{name: "positive replace also publishes an absent target", source: "replacement", install: filestore.InstallReplace, want: "replacement", wantExist: true},
		{name: "neutral empty create publishes exactly one empty regular file", source: "", install: filestore.InstallCreate, want: "", wantExist: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rootDirectory := t.TempDir()
			target := filepath.Join(rootDirectory, "target")
			if tc.initial != "" {
				if err := os.WriteFile(target, []byte(tc.initial), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			root, err := os.OpenRoot(rootDirectory)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if closeErr := root.Close(); closeErr != nil {
					t.Errorf("os.Root.Close() error = %v, want nil", closeErr)
				}
			}()
			maximum := max(len(tc.source), 1)
			gotRecovery, gotErr := filestore.Write(t.Context(), filestore.WriteRequest{
				Source: strings.NewReader(tc.source),
				Location: filestore.Location{
					Root: root,
					Path: mustRelativePath(t, "target"),
				},
				Temporary:    mustRelativePath(t, ".target-stage"),
				Mode:         0o640,
				Install:      tc.install,
				MaximumBytes: mustByteCount(t, uint64(maximum)),
			})
			if !errors.Is(gotRecovery.Validate(), core.ErrFilestoreContract) {
				t.Fatalf("Write() recovery = %v, want zero request after success or definite failure", gotRecovery)
			}
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("Write() error = %v, want %v", gotErr, tc.wantErr)
				}
			} else if gotErr != nil {
				t.Fatalf("Write() error = %v, want nil", gotErr)
			}
			got, readErr := os.ReadFile(target)
			if !tc.wantExist {
				if !errors.Is(readErr, fs.ErrNotExist) {
					t.Fatalf("ReadFile(target) error = %v, want %v", readErr, fs.ErrNotExist)
				}
				return
			}
			if readErr != nil {
				t.Fatal(readErr)
			}
			if string(got) != tc.want {
				t.Fatalf("target bytes = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWriteRejectsOverflowWithoutChangingTarget(t *testing.T) {
	t.Parallel()

	rootDirectory := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDirectory, "target"), []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(rootDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Errorf("os.Root.Close() error = %v, want nil", closeErr)
		}
	}()
	gotRecovery, gotErr := filestore.Write(t.Context(), filestore.WriteRequest{
		Source: bytes.NewReader([]byte("12345")),
		Location: filestore.Location{
			Root: root,
			Path: mustRelativePath(t, "target"),
		},
		Temporary:    mustRelativePath(t, ".target-stage"),
		Mode:         0o600,
		Install:      filestore.InstallReplace,
		MaximumBytes: mustByteCount(t, 4),
	})
	if !errors.Is(gotRecovery.Validate(), core.ErrFilestoreContract) {
		t.Fatalf("Write(over maximum) recovery = %v, want zero request after definite rejection", gotRecovery)
	}
	if !errors.Is(gotErr, core.ErrFilestoreSize) {
		t.Fatalf("Write(over maximum) error = %v, want %v", gotErr, core.ErrFilestoreSize)
	}
	got, err := os.ReadFile(filepath.Join(rootDirectory, "target"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "original" {
		t.Fatalf("target after rejected write = %q, want %q", got, "original")
	}
	entries, err := os.ReadDir(rootDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("directory entries after rejected write = %v, want only target", entries)
	}
}

func TestWritePreservesRealClosedSourceError(t *testing.T) {
	t.Parallel()

	rootDirectory := t.TempDir()
	sourcePath := filepath.Join(rootDirectory, "source")
	source, err := os.Create(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(rootDirectory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Errorf("os.Root.Close() error = %v, want nil", closeErr)
		}
	}()
	gotRecovery, gotErr := filestore.Write(t.Context(), filestore.WriteRequest{
		Source: source,
		Location: filestore.Location{
			Root: root,
			Path: mustRelativePath(t, "target"),
		},
		Temporary:    mustRelativePath(t, ".target-stage"),
		Mode:         0o600,
		Install:      filestore.InstallCreate,
		MaximumBytes: mustByteCount(t, 1),
	})
	if !errors.Is(gotRecovery.Validate(), core.ErrFilestoreContract) {
		t.Fatalf("Write(closed source) recovery = %v, want zero request after definite source failure", gotRecovery)
	}
	if !errors.Is(gotErr, os.ErrClosed) || !errors.Is(gotErr, core.ErrFilestoreSource) {
		t.Fatalf("Write(closed source) error = %v, want %v and %v", gotErr, os.ErrClosed, core.ErrFilestoreSource)
	}
}

func TestReadStreamsAtMostTheDeclaredMaximum(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr   error
		name      string
		content   string
		want      string
		maximum   uint64
		wantCount uint64
	}{
		{name: "empty file stays empty", content: "", maximum: 1, want: "", wantCount: 0},
		{name: "one below maximum is complete", content: "abc", maximum: 4, want: "abc", wantCount: 3},
		{name: "exact maximum is complete", content: "abcd", maximum: 4, want: "abcd", wantCount: 4},
		{name: "one above maximum streams only maximum", content: "abcde", maximum: 4, want: "abcd", wantCount: 4, wantErr: core.ErrFilestoreSize},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			rootDirectory := t.TempDir()
			if err := os.WriteFile(filepath.Join(rootDirectory, "target"), []byte(tc.content), 0o600); err != nil {
				t.Fatal(err)
			}
			root, err := os.OpenRoot(rootDirectory)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if closeErr := root.Close(); closeErr != nil {
					t.Errorf("os.Root.Close() error = %v, want nil", closeErr)
				}
			}()
			var destination bytes.Buffer
			gotCount, gotErr := filestore.Read(t.Context(), filestore.ReadRequest{
				Destination: &destination,
				Location: filestore.Location{
					Root: root,
					Path: mustRelativePath(t, "target"),
				},
				MaximumBytes: mustByteCount(t, tc.maximum),
			})
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("Read() error = %v, want %v", gotErr, tc.wantErr)
				}
			} else if gotErr != nil {
				t.Fatalf("Read() error = %v, want nil", gotErr)
			}
			if gotCount.Uint64() != tc.wantCount {
				t.Fatalf("Read() byte count = %d, want %d", gotCount.Uint64(), tc.wantCount)
			}
			if destination.String() != tc.want {
				t.Fatalf("Read() destination = %q, want %q", destination.String(), tc.want)
			}
		})
	}
}
