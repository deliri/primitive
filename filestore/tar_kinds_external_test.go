package filestore_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func nativeTARLinkTarget(t testing.TB, directory, name, target string) filestore.SymbolicLinkTarget {
	t.Helper()
	if err := os.Symlink(target, filepath.Join(directory, name)); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		t.Fatal(err)
	}
	path, err := core.ParseRelativePath(name)
	if err != nil {
		t.Fatal(err)
	}
	got, readErr := filestore.ReadSymbolicLink(t.Context(), filestore.Location{Root: root, Path: path})
	if err := errors.Join(readErr, root.Close()); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestTAREntryKindLayerTriad(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	target := nativeTARLinkTarget(t, directory, "link", "../opaque:[1]/missing")
	path, err := core.ParseRelativePath("entry")
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		name       string
		change     func(*filestore.TAREntry)
		wantErr    error
		wantKind   byte
		wantTarget string
		wantBody   string
	}{
		{name: "regular source retains its declared body", wantKind: tar.TypeReg, wantBody: "abc"},
		{name: "empty regular file remains a regular member", change: func(e *filestore.TAREntry) { e.Source = bytes.NewReader(nil); e.Bytes = core.ByteLength{} }, wantKind: tar.TypeReg},
		{name: "zero permissions are legitimate archive metadata", change: func(e *filestore.TAREntry) { e.Mode = 0 }, wantKind: tar.TypeReg, wantBody: "abc"},
		{name: "directory is an empty header", change: func(e *filestore.TAREntry) {
			e.Kind = filestore.PathKindDirectory
			e.Source = nil
			e.Bytes = core.ByteLength{}
		}, wantKind: tar.TypeDir},
		{name: "directory retains zero permissions", change: func(e *filestore.TAREntry) {
			e.Kind = filestore.PathKindDirectory
			e.Source = nil
			e.Bytes = core.ByteLength{}
			e.Mode = 0
		}, wantKind: tar.TypeDir},
		{name: "link target stays opaque without content", change: func(e *filestore.TAREntry) {
			e.Kind = filestore.PathKindSymbolicLink
			e.Source = nil
			e.Bytes = core.ByteLength{}
			e.LinkTarget = target
		}, wantKind: tar.TypeSymlink, wantTarget: target.String()},
		{name: "link retains zero permissions", change: func(e *filestore.TAREntry) {
			e.Kind = filestore.PathKindSymbolicLink
			e.Source = nil
			e.Bytes = core.ByteLength{}
			e.LinkTarget = target
			e.Mode = 0
		}, wantKind: tar.TypeSymlink, wantTarget: target.String()},
		{name: "regular member refuses a link target", change: func(e *filestore.TAREntry) { e.LinkTarget = target }, wantErr: core.ErrFilestoreContract},
		{name: "regular member refuses a missing source", change: func(e *filestore.TAREntry) { e.Source = nil }, wantErr: core.ErrFilestoreContract},
		{name: "directory refuses discarded source bytes", change: func(e *filestore.TAREntry) { e.Kind = filestore.PathKindDirectory; e.Bytes = core.ByteLength{} }, wantErr: core.ErrFilestoreContract},
		{name: "directory refuses a declared body", change: func(e *filestore.TAREntry) { e.Kind = filestore.PathKindDirectory; e.Source = nil }, wantErr: core.ErrFilestoreContract},
		{name: "directory refuses a link target", change: func(e *filestore.TAREntry) {
			e.Kind = filestore.PathKindDirectory
			e.Source = nil
			e.Bytes = core.ByteLength{}
			e.LinkTarget = target
		}, wantErr: core.ErrFilestoreContract},
		{name: "link refuses discarded source bytes", change: func(e *filestore.TAREntry) {
			e.Kind = filestore.PathKindSymbolicLink
			e.Bytes = core.ByteLength{}
			e.LinkTarget = target
		}, wantErr: core.ErrFilestoreContract},
		{name: "link refuses a declared body", change: func(e *filestore.TAREntry) {
			e.Kind = filestore.PathKindSymbolicLink
			e.Source = nil
			e.LinkTarget = target
		}, wantErr: core.ErrFilestoreContract},
		{name: "link refuses an absent native target", change: func(e *filestore.TAREntry) {
			e.Kind = filestore.PathKindSymbolicLink
			e.Source = nil
			e.Bytes = core.ByteLength{}
		}, wantErr: core.ErrFilestoreContract},
		{name: "unset kind never defaults to regular", change: func(e *filestore.TAREntry) { e.Kind = filestore.PathKindUnknown }, wantErr: core.ErrFilestoreContract},
		{name: "absent path cannot become an archive member", change: func(e *filestore.TAREntry) { e.Kind = filestore.PathKindAbsent }, wantErr: core.ErrFilestoreContract},
		{name: "other native kinds cannot become regular files", change: func(e *filestore.TAREntry) { e.Kind = filestore.PathKindOther }, wantErr: core.ErrFilestoreContract},
		{name: "unreachable path cannot become an archive member", change: func(e *filestore.TAREntry) { e.Kind = filestore.PathKindUnreachable }, wantErr: core.ErrFilestoreContract},
		{name: "future kind cannot become an archive member", change: func(e *filestore.TAREntry) { e.Kind = filestore.PathKind(255) }, wantErr: core.ErrFilestoreContract},
		{name: "directory mode bit is not a permission", change: func(e *filestore.TAREntry) { e.Mode |= fs.ModeDir }, wantErr: core.ErrFilestoreContract},
		{name: "link mode bit is not a permission", change: func(e *filestore.TAREntry) { e.Mode |= fs.ModeSymlink }, wantErr: core.ErrFilestoreContract},
		{name: "setuid mode bit is not a permission", change: func(e *filestore.TAREntry) { e.Mode |= fs.ModeSetuid }, wantErr: core.ErrFilestoreContract},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			extent, err := core.NewByteLength(3)
			if err != nil {
				t.Fatal(err)
			}
			entry := filestore.TAREntry{Source: bytes.NewReader([]byte("abc")), Path: path, Bytes: extent, Mode: 0600, Kind: filestore.PathKindRegularFile}
			if row.change != nil {
				row.change(&entry)
			}
			validation := entry.Validate()
			var output bytes.Buffer
			content, err := filestore.WriteTAR(t.Context(), filestore.TARWriteRequest{Destination: &output, Compression: filestore.TARPlain, Produce: func(_ context.Context, add func(filestore.TAREntry) (filestore.ContentIndexEntry, error)) error {
				observed, addErr := add(entry)
				if addErr != nil {
					if observed != (filestore.ContentIndexEntry{}) {
						t.Fatalf("refused entry content=%+v, want zero", observed)
					}
					// An ignored refusal still owns the terminal archive result.
					return nil
				}
				if row.wantErr != nil {
					return nil
				}
				want := filestore.ContentIndexEntry{Digest: core.SHA256Of([]byte(row.wantBody)), Extent: entry.Bytes}
				if observed != want {
					t.Fatalf("entry content=%+v, want body-only %+v", observed, want)
				}
				return nil
			}})
			archive := tar.NewReader(bytes.NewReader(output.Bytes()))
			if row.wantErr != nil {
				if !errors.Is(validation, row.wantErr) || !errors.Is(err, row.wantErr) || content != (filestore.ContentIndexEntry{}) {
					t.Fatalf("entry validation=%v archive=%+v/%v, want %v and zero content", validation, content, err, row.wantErr)
				}
				if header, nextErr := archive.Next(); nextErr != io.EOF || header != nil {
					t.Fatalf("refused header=%+v/%v, want no member and EOF", header, nextErr)
				}
				return
			}
			if validation != nil || err != nil || content.Digest != core.SHA256Of(output.Bytes()) || content.Extent.Uint64() != uint64(output.Len()) {
				t.Fatalf("entry validation=%v archive=%+v/%v, want exact completed output", validation, content, err)
			}
			header, err := archive.Next()
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(archive)
			if err != nil || header.Name != "entry" || header.Typeflag != row.wantKind || header.Linkname != row.wantTarget || header.Mode != int64(entry.Mode) || header.Size != int64(len(row.wantBody)) || string(body) != row.wantBody {
				t.Fatalf("native member=%+v body=%q/%v, want kind=%d target=%q mode=%#o body=%q", header, body, err, row.wantKind, row.wantTarget, entry.Mode, row.wantBody)
			}
			if _, err := archive.Next(); err != io.EOF {
				t.Fatalf("native closure=%v, want EOF", err)
			}
		})
	}
}

func TestTARKindsPreserveNativeTreeAndOpaqueLinksLayerTriad(t *testing.T) {
	t.Parallel()
	for _, compression := range []filestore.TARCompression{filestore.TARPlain, filestore.TARGzip} {
		t.Run(compression.String(), func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			outside := t.TempDir()
			if err := os.WriteFile(filepath.Join(outside, "marker"), []byte("outside bytes must never enter the archive"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(directory, "empty", "nested"), 0750); err != nil {
				t.Fatal(err)
			}
			data := []byte("owned regular content")
			if err := os.WriteFile(filepath.Join(directory, "body"), data, 0640); err != nil {
				t.Fatal(err)
			}
			rows := []struct {
				name, target string
				kind         filestore.PathKind
				wantType     byte
			}{
				{name: "empty", kind: filestore.PathKindDirectory, wantType: tar.TypeDir},
				{name: "empty/nested", kind: filestore.PathKindDirectory, wantType: tar.TypeDir},
				{name: "body", kind: filestore.PathKindRegularFile, wantType: tar.TypeReg},
				{name: "relative-link", target: "body", kind: filestore.PathKindSymbolicLink, wantType: tar.TypeSymlink},
				{name: "broken-link", target: "missing", kind: filestore.PathKindSymbolicLink, wantType: tar.TypeSymlink},
				{name: "opaque-link", target: "../opaque:[1]//../missing", kind: filestore.PathKindSymbolicLink, wantType: tar.TypeSymlink},
				{name: "outside-link", target: filepath.Join(outside, "marker"), kind: filestore.PathKindSymbolicLink, wantType: tar.TypeSymlink},
				{name: "long-link", target: strings.Repeat("x", 101), kind: filestore.PathKindSymbolicLink, wantType: tar.TypeSymlink},
				{name: "native-byte-link", target: string([]byte{0xff, '/', 'x'}), kind: filestore.PathKindSymbolicLink, wantType: tar.TypeSymlink},
			}
			for _, row := range rows {
				if row.target != "" {
					if err := os.Symlink(row.target, filepath.Join(directory, row.name)); err != nil {
						t.Fatal(err)
					}
				}
			}
			root, err := os.OpenRoot(directory)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := root.Close(); err != nil {
					t.Error(err)
				}
			})
			var output bytes.Buffer
			content, err := filestore.WriteTAR(t.Context(), filestore.TARWriteRequest{Destination: &output, Compression: compression, Produce: func(ctx context.Context, add func(filestore.TAREntry) (filestore.ContentIndexEntry, error)) error {
				for _, row := range rows {
					path, err := core.ParseRelativePath(row.name)
					if err != nil {
						return err
					}
					info, err := root.Lstat(row.name)
					if err != nil {
						return err
					}
					entry := filestore.TAREntry{Path: path, Mode: info.Mode().Perm(), Kind: row.kind}
					var source *os.File
					want := filestore.ContentIndexEntry{Digest: core.SHA256Of(nil)}
					if row.kind == filestore.PathKindRegularFile {
						source, err = root.Open(row.name)
						if err != nil {
							return err
						}
						entry.Source = source
						entry.Bytes, err = core.NewByteLength(uint64(len(data)))
						if err != nil {
							return errors.Join(err, source.Close())
						}
						want = filestore.ContentIndexEntry{Digest: core.SHA256Of(data), Extent: entry.Bytes}
					} else if row.kind == filestore.PathKindSymbolicLink {
						entry.LinkTarget, err = filestore.ReadSymbolicLink(ctx, filestore.Location{Root: root, Path: path})
						if err != nil {
							return err
						}
					}
					observed, addErr := add(entry)
					if source != nil {
						addErr = errors.Join(addErr, source.Close())
					}
					if addErr != nil {
						return addErr
					}
					if observed != want {
						t.Fatalf("entry=%s content=%+v, want %+v", row.name, observed, want)
					}
				}
				return nil
			}})
			if err != nil || content.Digest != core.SHA256Of(output.Bytes()) || content.Extent.Uint64() != uint64(output.Len()) {
				t.Fatalf("native tree archive=%+v/%v, want exact completed output", content, err)
			}
			var decoded io.Reader = bytes.NewReader(output.Bytes())
			if compression == filestore.TARGzip {
				gz, err := gzip.NewReader(decoded)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := gz.Close(); err != nil {
						t.Error(err)
					}
				})
				decoded = gz
			}
			archive := tar.NewReader(decoded)
			for _, row := range rows {
				header, err := archive.Next()
				if err != nil {
					t.Fatal(err)
				}
				body, err := io.ReadAll(archive)
				want := []byte(nil)
				if row.kind == filestore.PathKindRegularFile {
					want = data
				}
				info, statErr := root.Lstat(row.name)
				if err != nil || statErr != nil {
					t.Fatalf("member read=%v metadata=%v, want nil", err, statErr)
				}
				if header.Name != row.name || header.Typeflag != row.wantType || header.Linkname != row.target || header.Mode != int64(info.Mode().Perm()) || header.Size != int64(len(want)) || !bytes.Equal(body, want) {
					t.Fatalf("native member=%+v body=%q, want name=%q kind=%d target=%q mode=%#o body=%q", header, body, row.name, row.wantType, row.target, info.Mode().Perm(), want)
				}
			}
			if _, err := archive.Next(); err != io.EOF {
				t.Fatalf("tree archive closure=%v, want EOF", err)
			}
			if _, err := io.Copy(io.Discard, decoded); err != nil {
				t.Fatalf("tree compression footer=%v, want nil", err)
			}
		})
	}
}

func TestTAREntryCompleteKindAndPermissionDomains(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	target := nativeTARLinkTarget(t, directory, "link", "../opaque-target")
	path, err := core.ParseRelativePath("entry")
	if err != nil {
		t.Fatal(err)
	}
	for number := range 256 {
		kind := filestore.PathKind(number)
		entry := filestore.TAREntry{Path: path, Kind: kind, Mode: 0600}
		wantValid := kind == filestore.PathKindRegularFile || kind == filestore.PathKindDirectory || kind == filestore.PathKindSymbolicLink
		if kind == filestore.PathKindRegularFile {
			entry.Source = bytes.NewReader(nil)
		} else if kind == filestore.PathKindSymbolicLink {
			entry.LinkTarget = target
		}
		err := entry.Validate()
		if (wantValid && err != nil) || (!wantValid && !errors.Is(err, core.ErrFilestoreContract)) {
			t.Fatalf("kind=%d validation=%v, want admitted=%v with typed refusal otherwise", number, err, wantValid)
		}
	}
	for mode := fs.FileMode(0); mode <= fs.ModePerm; mode++ {
		entry := filestore.TAREntry{Path: path, Kind: filestore.PathKindRegularFile, Mode: mode, Source: bytes.NewReader(nil)}
		if err := entry.Validate(); err != nil {
			t.Fatalf("permission=%#o validation=%v, want admitted", mode, err)
		}
	}
	for bit := fs.ModePerm + 1; bit != 0; bit <<= 1 {
		entry := filestore.TAREntry{Path: path, Kind: filestore.PathKindRegularFile, Mode: fs.ModePerm | bit, Source: bytes.NewReader(nil)}
		if err := entry.Validate(); !errors.Is(err, core.ErrFilestoreContract) {
			t.Fatalf("non-permission bit=%#x validation=%v, want typed refusal", bit, err)
		}
	}
}
