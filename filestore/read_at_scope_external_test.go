package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/lineio"
)

func TestReadAtScopeKeepsOneNativeInodeAndCursor(t *testing.T) {
	t.Parallel()
	for _, outcome := range []string{"complete", "refusal", "panic", "namespace_replaced"} {
		t.Run(outcome, func(t *testing.T) {
			t.Parallel()
			root := requireTestRoot(t, t.TempDir())
			payload := []byte{0, 255, 31, 8, 128, 7, 0}
			if err := root.WriteFile("source", payload, 0o600); err != nil {
				t.Fatal(err)
			}
			var borrowed lineio.RecordSource
			result, err := filestore.WithReadAtScope(t.Context(), filestore.ReadAtScopeRequest{
				Location: filestore.Location{Root: root, Path: mustRelativePath(t, "source")},
				Use: func(_ context.Context, source lineio.RecordSource) error {
					borrowed = source
					if outcome == "namespace_replaced" {
						if err := root.Rename("source", "original"); err != nil {
							return err
						}
						if err := root.WriteFile("source", []byte("foreign"), 0o600); err != nil {
							return err
						}
					}
					var sequential [2]byte
					if n, err := io.ReadFull(source, sequential[:]); err != nil || n != 2 || !bytes.Equal(sequential[:], payload[:2]) {
						t.Fatalf("sequential read = (%x,%d,%v), want original inode prefix", sequential, n, err)
					}
					var positional [3]byte
					if n, err := source.ReadAt(positional[:], 3); err != nil || n != 3 || !bytes.Equal(positional[:], payload[3:6]) {
						t.Fatalf("positional read = (%x,%d,%v), want original inode range", positional, n, err)
					}
					if position, err := source.Seek(0, io.SeekCurrent); err != nil || position != 2 {
						t.Fatalf("sequential cursor = (%d,%v), want undisturbed coordinate 2", position, err)
					}
					if outcome == "refusal" {
						return fs.ErrPermission
					}
					if outcome == "panic" {
						panic(fs.ErrPermission)
					}
					return nil
				},
			})
			if err != nil || result.Validate() != nil || result.CleanupError() != nil || borrowed == nil {
				t.Fatalf("read-at scope = (%+v,%v), want completed native lifetime", result, err)
			}
			var want error
			if outcome == "refusal" {
				want = fs.ErrPermission
			} else if outcome == "panic" {
				want = core.ErrFilestoreCallbackPanic
			}
			if !errors.Is(result.OperationError(), want) {
				t.Fatalf("operation = %v, want %v", result.OperationError(), want)
			}
			if _, err := borrowed.ReadAt(make([]byte, 1), 0); !errors.Is(err, fs.ErrClosed) {
				t.Fatalf("borrowed positional capability after scope = %v, want closed", err)
			}
			if outcome == "namespace_replaced" {
				data, err := root.ReadFile("source")
				if err != nil || string(data) != "foreign" {
					t.Fatalf("foreign namespace = (%q,%v), want unchanged replacement", data, err)
				}
			}
		})
	}
}

func FuzzReadAtScopeMatchesNativeByteRanges(f *testing.F) {
	f.Add([]byte{}, uint16(0), uint16(1))
	f.Add([]byte{0, 255, 31, 8, 128}, uint16(2), uint16(4))
	f.Add(bytes.Repeat([]byte{0, 255, 31, 8, 128}, 6554), uint16(32767), uint16(3))
	f.Fuzz(func(t *testing.T, payload []byte, offset, count uint16) {
		root := requireTestRoot(t, t.TempDir())
		if err := root.WriteFile("source", payload, 0o600); err != nil {
			t.Fatal(err)
		}
		var borrowed lineio.RecordSource
		result, err := filestore.WithReadAtScope(t.Context(), filestore.ReadAtScopeRequest{Location: filestore.Location{Root: root, Path: mustRelativePath(t, "source")}, Use: func(_ context.Context, source lineio.RecordSource) error {
			borrowed = source
			position, err := source.Seek(1, io.SeekStart)
			if err != nil {
				return err
			}
			destination := make([]byte, int(count)+1)
			n, readErr := source.ReadAt(destination, int64(offset))
			start := min(len(payload), int(offset))
			end := min(len(payload), start+len(destination))
			want := payload[start:end]
			if n != len(want) || !bytes.Equal(destination[:n], want) || n < len(destination) && readErr != io.EOF || n == len(destination) && readErr != nil {
				t.Fatalf("native range = (%d,%v), want exact independent range %d bytes and EOF only on short read", n, readErr, len(want))
			}
			if after, err := source.Seek(0, io.SeekCurrent); err != nil || after != position {
				t.Fatalf("positional read changed native cursor: (%d,%v), want %d", after, err, position)
			}
			return nil
		}})
		if err != nil || errors.Join(result.Validate(), result.OperationError(), result.CleanupError()) != nil || borrowed == nil {
			t.Fatalf("read-at scope = (%+v,%v), want complete byte-range loan", result, err)
		}
		if _, err := borrowed.ReadAt(make([]byte, 1), 0); !errors.Is(err, os.ErrClosed) {
			t.Fatalf("expired source = %v, want native closed refusal", err)
		}
	})
}
