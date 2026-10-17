package filestore_test

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/temporal"
)

func zipFixtureInstant(t testing.TB) temporal.Instant {
	t.Helper()
	instant, err := temporal.NewInstant(time.Date(2026, time.October, 17, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return instant
}

func TestZIPNativeByteConservationAndMetadata(t *testing.T) {
	t.Parallel()
	for _, method := range []filestore.ZIPCompression{filestore.ZIPStore, filestore.ZIPDeflate} {
		for _, extent := range []int{0, 1, 32767, 32768, 32769, 131073} {
			t.Run(method.String()+"/"+strconv.Itoa(extent), func(t *testing.T) {
				t.Parallel()
				payload := bytes.Repeat([]byte{0, 255, 31, 8, 128}, (extent+4)/5)[:extent]
				modified := zipFixtureInstant(t)
				var encoded bytes.Buffer
				err := filestore.WriteZIP(t.Context(), filestore.ZIPWriteRequest{Destination: &encoded, Produce: func(_ context.Context, add func(filestore.ZIPEntry) error) error {
					return add(filestore.ZIPEntry{Path: mustRelativePath(t, "docs/README.md"), Source: bytes.NewReader(payload), Modified: modified, Mode: 0o640, Compression: method})
				}})
				if err != nil {
					t.Fatal(err)
				}
				archive, err := zip.NewReader(bytes.NewReader(encoded.Bytes()), int64(encoded.Len()))
				if err != nil || len(archive.File) != 1 {
					t.Fatalf("native ZIP = (%v,%v), want one complete entry", archive, err)
				}
				entry := archive.File[0]
				wantMethod := uint16(zip.Store)
				if method == filestore.ZIPDeflate {
					wantMethod = zip.Deflate
				}
				wantTime, err := modified.Time()
				if err != nil {
					t.Fatal(err)
				}
				if entry.Name != "docs/README.md" || entry.Mode().Perm() != 0o640 || entry.Method != wantMethod || !entry.Modified.Equal(wantTime) || entry.UncompressedSize64 != uint64(extent) {
					t.Fatalf("entry metadata = (%q,%v,%d,%v,%d), want exact name, mode, compression, instant and extent", entry.Name, entry.Mode(), entry.Method, entry.Modified, entry.UncompressedSize64)
				}
				source, err := entry.Open()
				if err != nil {
					t.Fatal(err)
				}
				data, readErr := io.ReadAll(source)
				if err := errors.Join(readErr, source.Close()); err != nil || !bytes.Equal(data, payload) {
					t.Fatalf("extracted ZIP = (%d bytes,%v), want exact %d bytes", len(data), err, extent)
				}
			})
		}
	}
}

func TestZIPNativeRefusalAndProducerLifetime(t *testing.T) {
	t.Parallel()
	for _, boundary := range []string{"nil_destination", "nil_producer", "nil_source", "zero_path", "root_path", "zero_instant", "zero_mode", "typed_mode", "unknown_method", "method_after_domain", "producer_refusal", "producer_panic", "closed_source", "closed_destination", "canceled_admission"} {
		t.Run(boundary, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			var encoded bytes.Buffer
			entry := filestore.ZIPEntry{Path: mustRelativePath(t, "README.md"), Source: bytes.NewReader([]byte{0, 255, 31}), Modified: zipFixtureInstant(t), Mode: 0o600, Compression: filestore.ZIPDeflate}
			request := filestore.ZIPWriteRequest{Destination: &encoded, Produce: func(_ context.Context, add func(filestore.ZIPEntry) error) error { return add(entry) }}
			var want error = core.ErrFilestoreContract
			switch boundary {
			case "nil_destination":
				request.Destination = nil
			case "nil_producer":
				request.Produce = nil
			case "nil_source":
				entry.Source = nil
			case "zero_path":
				entry.Path = core.RelativePath{}
				want = core.ErrPrimitiveContract
			case "root_path":
				entry.Path = mustRelativePath(t, ".")
			case "zero_instant":
				entry.Modified = temporal.Instant{}
				want = core.ErrTemporalContract
			case "zero_mode":
				entry.Mode = 0
			case "typed_mode":
				entry.Mode |= fs.ModeDir
			case "unknown_method":
				entry.Compression = filestore.ZIPCompressionUnknown
			case "method_after_domain":
				entry.Compression = filestore.ZIPCompression(255)
			case "producer_refusal":
				request.Produce = func(context.Context, func(filestore.ZIPEntry) error) error { return fs.ErrPermission }
				want = fs.ErrPermission
			case "producer_panic":
				request.Produce = func(context.Context, func(filestore.ZIPEntry) error) error { panic(fs.ErrPermission) }
				want = core.ErrFilestoreCallbackPanic
			case "closed_source", "closed_destination":
				file, err := os.Create(filepath.Join(t.TempDir(), "native"))
				if err != nil {
					t.Fatal(err)
				}
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
				if boundary == "closed_source" {
					entry.Source = file
				} else {
					request.Destination = file
				}
				want = fs.ErrClosed
			case "canceled_admission":
				canceled, cancel := context.WithCancelCause(ctx)
				cancel(fs.ErrPermission)
				ctx = canceled
				want = context.Canceled
			}
			err := filestore.WriteZIP(ctx, request)
			if !errors.Is(err, want) {
				t.Fatalf("ZIP refusal = %v, want %v", err, want)
			}
			if boundary == "canceled_admission" && (!errors.Is(err, fs.ErrPermission) || encoded.Len() != 0) {
				t.Fatalf("canceled ZIP = (%d bytes,%v), want no output and native cause", encoded.Len(), err)
			}
		})
	}
}

func TestZIPIgnoredEntryRefusalStopsLaterNativeReads(t *testing.T) {
	t.Parallel()
	for _, panicAfter := range []bool{false, true} {
		t.Run(strconv.FormatBool(panicAfter), func(t *testing.T) {
			t.Parallel()
			file, err := os.Create(filepath.Join(t.TempDir(), "source"))
			if err != nil {
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			second, err := os.Create(filepath.Join(t.TempDir(), "second"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := second.Close(); err != nil {
					t.Error(err)
				}
			})
			if _, err := second.Write([]byte("unconsumed")); err != nil {
				t.Fatal(err)
			}
			if _, err := second.Seek(0, io.SeekStart); err != nil {
				t.Fatal(err)
			}
			var encoded bytes.Buffer
			err = filestore.WriteZIP(t.Context(), filestore.ZIPWriteRequest{Destination: &encoded, Produce: func(_ context.Context, add func(filestore.ZIPEntry) error) error {
				entry := filestore.ZIPEntry{Path: mustRelativePath(t, "first"), Source: file, Modified: zipFixtureInstant(t), Mode: 0o600, Compression: filestore.ZIPDeflate}
				_ = add(entry)
				entry.Path, entry.Source = mustRelativePath(t, "second"), second
				_ = add(entry)
				if panicAfter {
					panic(fs.ErrPermission)
				}
				return nil
			}})
			position, seekErr := second.Seek(0, io.SeekCurrent)
			if !errors.Is(err, fs.ErrClosed) || seekErr != nil || position != 0 || panicAfter && !errors.Is(err, core.ErrFilestoreCallbackPanic) {
				t.Fatalf("ignored native refusal = (%v, next position %d,%v), want retained refusal and untouched later source", err, position, seekErr)
			}
		})
	}
}
