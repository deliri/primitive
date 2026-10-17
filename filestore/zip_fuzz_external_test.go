package filestore_test

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func FuzzZIPNativeEntriesPreserveBytesAndRefusals(f *testing.F) {
	f.Add([]byte{}, uint8(filestore.ZIPStore), false)
	f.Add([]byte{0, 255, 31, 8, 128}, uint8(filestore.ZIPDeflate), false)
	f.Add(bytes.Repeat([]byte{0, 255, 31, 8, 128}, 6554), uint8(filestore.ZIPDeflate), false)
	f.Add([]byte{0, 255}, uint8(filestore.ZIPCompressionUnknown), false)
	f.Add([]byte{0, 255}, uint8(255), true)
	f.Add([]byte{0, 255}, uint8(filestore.ZIPStore), true)
	f.Fuzz(func(t *testing.T, payload []byte, method uint8, refused bool) {
		var encoded bytes.Buffer
		split := len(payload) / 2
		modified := zipFixtureInstant(t)
		err := filestore.WriteZIP(t.Context(), filestore.ZIPWriteRequest{Destination: &encoded, Produce: func(_ context.Context, add func(filestore.ZIPEntry) error) error {
			first := add(filestore.ZIPEntry{Path: mustRelativePath(t, "first"), Source: bytes.NewReader(payload[:split]), Modified: modified, Mode: 0o600, Compression: filestore.ZIPCompression(method)})
			second := add(filestore.ZIPEntry{Path: mustRelativePath(t, "second"), Source: bytes.NewReader(payload[split:]), Modified: modified, Mode: 0o640, Compression: filestore.ZIPCompression(method)})
			if refused {
				return errors.Join(first, second, fs.ErrPermission)
			}
			return errors.Join(first, second)
		}})
		validMethod := method == uint8(filestore.ZIPStore) || method == uint8(filestore.ZIPDeflate)
		if !validMethod || refused {
			if err == nil || !validMethod && !errors.Is(err, core.ErrFilestoreContract) || refused && !errors.Is(err, fs.ErrPermission) {
				t.Fatalf("refused ZIP = %v, want native method and producer refusals for selected boundaries", err)
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		archive, err := zip.NewReader(bytes.NewReader(encoded.Bytes()), int64(encoded.Len()))
		if err != nil || len(archive.File) != 2 {
			t.Fatalf("completed ZIP = (%v,%v), want two ordered entries", archive, err)
		}
		for index, entry := range archive.File {
			want, name := payload[:split], "first"
			mode := fs.FileMode(0o600)
			if index == 1 {
				want, name, mode = payload[split:], "second", 0o640
			}
			source, err := entry.Open()
			if err != nil {
				t.Fatal(err)
			}
			data, readErr := io.ReadAll(source)
			if err := errors.Join(readErr, source.Close()); err != nil || !bytes.Equal(data, want) || entry.Name != name || entry.Mode().Perm() != mode {
				t.Fatalf("entry %d = (%q,%d bytes,%v,%v), want ordered %q with exact bytes and mode %v", index, entry.Name, len(data), entry.Mode(), err, name, mode)
			}
		}
	})
}
