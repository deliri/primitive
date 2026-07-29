package filestore_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestStreamingAcceptsAndRejectsStandardLibraryReaderBehaviors(t *testing.T) {
	t.Parallel()

	payload := deterministicPayload((32 << 10) + 17)
	cases := []struct {
		name       string
		reader     func([]byte) io.Reader
		wantErr    error
		wantNative error
	}{
		{
			name:   "bytes reader may return a full bounded chunk",
			reader: func(data []byte) io.Reader { return bytes.NewReader(data) },
		},
		{
			name:   "strings reader preserves arbitrary binary string bytes",
			reader: func(data []byte) io.Reader { return strings.NewReader(string(data)) },
		},
		{
			name:   "one-byte reader forces maximum read fragmentation",
			reader: func(data []byte) io.Reader { return iotest.OneByteReader(bytes.NewReader(data)) },
		},
		{
			name:   "half reader forces repeated partial chunks",
			reader: func(data []byte) io.Reader { return iotest.HalfReader(bytes.NewReader(data)) },
		},
		{
			name:   "data-error reader returns final bytes with eof",
			reader: func(data []byte) io.Reader { return iotest.DataErrReader(bytes.NewReader(data)) },
		},
		{
			name: "one-byte data-error reader combines fragmentation and terminal data",
			reader: func(data []byte) io.Reader {
				return iotest.OneByteReader(iotest.DataErrReader(bytes.NewReader(data)))
			},
		},
		{
			name: "half data-error reader combines partial chunks and terminal data",
			reader: func(data []byte) io.Reader {
				return iotest.HalfReader(iotest.DataErrReader(bytes.NewReader(data)))
			},
		},
		{
			name: "multi-reader crosses empty and nonempty source boundaries",
			reader: func(data []byte) io.Reader {
				middle := len(data) / 2
				return io.MultiReader(
					bytes.NewReader(nil),
					bytes.NewReader(data[:middle]),
					bytes.NewReader(nil),
					bytes.NewReader(data[middle:]),
				)
			},
		},
		{
			name: "limit reader ends at the exact admitted extent",
			reader: func(data []byte) io.Reader {
				return io.LimitReader(bytes.NewReader(data), int64(len(data)))
			},
		},
		{
			name: "section reader exposes the exact admitted extent",
			reader: func(data []byte) io.Reader {
				return io.NewSectionReader(bytes.NewReader(data), 0, int64(len(data)))
			},
		},
		{
			name:       "error reader fails before producing bytes",
			reader:     func([]byte) io.Reader { return iotest.ErrReader(io.ErrUnexpectedEOF) },
			wantErr:    core.ErrFilestoreSource,
			wantNative: io.ErrUnexpectedEOF,
		},
		{
			name: "timeout reader fails after producing its first real chunk",
			reader: func(data []byte) io.Reader {
				return iotest.TimeoutReader(iotest.OneByteReader(bytes.NewReader(data)))
			},
			wantErr:    core.ErrFilestoreSource,
			wantNative: iotest.ErrTimeout,
		},
		{
			name: "multi-reader partial prefix then native error rejects the entire stage",
			reader: func(data []byte) io.Reader {
				return io.MultiReader(
					bytes.NewReader(data[:17]),
					iotest.ErrReader(io.ErrUnexpectedEOF),
				)
			},
			wantErr:    core.ErrFilestoreSource,
			wantNative: io.ErrUnexpectedEOF,
		},
	}
	operations := []struct {
		name string
		run  func(*testing.T, *os.Root, io.Reader, uint64) (string, error)
	}{
		{
			name: "stage",
			run: func(t *testing.T, root *os.Root, source io.Reader, maximum uint64) (string, error) {
				staged, err := filestore.Stage(t.Context(), filestore.StageRequest{
					Source:       source,
					Temporary:    filestore.Location{Root: root, Path: mustRelativePath(t, ".stage")},
					Mode:         0o600,
					MaximumBytes: mustByteCount(t, maximum),
				})
				return staged.Path().String(), err
			},
		},
		{
			name: "write",
			run: func(t *testing.T, root *os.Root, source io.Reader, maximum uint64) (string, error) {
				_, err := filestore.Write(t.Context(), filestore.WriteRequest{
					Source:       source,
					Location:     filestore.Location{Root: root, Path: mustRelativePath(t, "target")},
					Temporary:    mustRelativePath(t, ".stage"),
					Mode:         0o600,
					Install:      filestore.InstallCreate,
					MaximumBytes: mustByteCount(t, maximum),
				})
				return "target", err
			},
		},
	}
	for _, operation := range operations {
		for _, tc := range cases {
			t.Run(operation.name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()

				rootDirectory := t.TempDir()
				root := requireTestRoot(t, rootDirectory)
				gotPath, gotErr := operation.run(
					t,
					root,
					tc.reader(payload),
					uint64(len(payload)),
				)
				if tc.wantErr != nil {
					if !errors.Is(gotErr, tc.wantErr) ||
						!errors.Is(gotErr, tc.wantNative) {
						t.Fatalf("%s error = %v, want %v and %v", operation.name, gotErr, tc.wantErr, tc.wantNative)
					}
					requireDirectoryEntryNames(t, rootDirectory, nil)
					return
				}
				if gotErr != nil {
					t.Fatalf("%s error = %v, want nil", operation.name, gotErr)
				}
				got, err := os.ReadFile(filepath.Join(rootDirectory, gotPath))
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(got, payload) {
					t.Fatalf("%s bytes = %d, want exact payload length %d", operation.name, len(got), len(payload))
				}
			})
		}
	}
}
