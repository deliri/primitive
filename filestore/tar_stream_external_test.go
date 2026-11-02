package filestore_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestTARStreamMatchesIndexedNativeBytes(t *testing.T) {
	t.Parallel()
	target := nativeTARLinkTarget(t, t.TempDir(), "opaque-link", "../opaque:[1]/missing")
	for _, compression := range []filestore.TARCompression{filestore.TARPlain, filestore.TARGzip} {
		for _, size := range []int{0, 1, 511, 512, 513, 32767, 32768, 32769} {
			t.Run(fmt.Sprintf("%s/%d", compression, size), func(t *testing.T) {
				t.Parallel()
				data := make([]byte, size)
				for i := range data {
					data[i] = byte(i)
				}
				file, _ := core.ParseRelativePath("directory/file")
				directory, _ := core.ParseRelativePath("empty")
				link, _ := core.ParseRelativePath("link")
				extent, _ := core.NewByteLength(uint64(size))
				produce := func(ctx context.Context, add func(filestore.TAREntry) (filestore.ContentIndexEntry, error)) error {
					for _, entry := range []filestore.TAREntry{
						{Kind: filestore.PathKindRegularFile, Path: file, Bytes: extent, Source: bytes.NewReader(data), Mode: 0},
						{Kind: filestore.PathKindDirectory, Path: directory, Mode: 0755},
						{Kind: filestore.PathKindSymbolicLink, Path: link, LinkTarget: target, Mode: 0777},
					} {
						got, err := add(entry)
						if err != nil {
							return err
						}
						body := data
						if entry.Kind != filestore.PathKindRegularFile {
							body = nil
						}
						if got.Extent != entry.Bytes || got.Digest != core.SHA256Of(body) {
							t.Fatalf("entry observation changed: %+v", got)
						}
					}
					return nil
				}
				var indexed, stream bytes.Buffer
				before, err := filestore.WriteTAR(t.Context(), filestore.TARWriteRequest{Destination: &indexed, Compression: compression, Produce: produce})
				if err != nil {
					t.Fatal(err)
				}
				got, err := filestore.StreamTAR(t.Context(), filestore.TARWriteRequest{Destination: &stream, Compression: compression, Produce: produce})
				if err != nil || got.Uint64() != uint64(stream.Len()) || got != before.Extent || !bytes.Equal(indexed.Bytes(), stream.Bytes()) {
					t.Fatalf("encoded bytes/extent changed: indexed=%d stream=%d observed=%v err=%v", indexed.Len(), stream.Len(), got, err)
				}
				var source io.Reader = bytes.NewReader(stream.Bytes())
				if compression == filestore.TARGzip {
					gz, err := gzip.NewReader(source)
					if err != nil {
						t.Fatal(err)
					}
					defer func() {
						if err := gz.Close(); err != nil {
							t.Error(err)
						}
					}()
					source = gz
				}
				archive := tar.NewReader(source)
				for i, want := range []byte{tar.TypeReg, tar.TypeDir, tar.TypeSymlink} {
					header, err := archive.Next()
					if err != nil || header.Typeflag != want {
						t.Fatalf("native header %d=%+v/%v", i, header, err)
					}
					body, err := io.ReadAll(archive)
					if err != nil {
						t.Fatal(err)
					}
					if i == 0 && (header.Mode != 0 || !bytes.Equal(body, data)) {
						t.Fatalf("native file bytes/mode changed")
					}
					if i == 2 && header.Linkname != target.String() {
						t.Fatalf("opaque target changed: %q", header.Linkname)
					}
				}
				if _, err := archive.Next(); err != io.EOF {
					t.Fatalf("native TAR closure=%v", err)
				}
				if _, err := io.Copy(io.Discard, source); err != nil {
					t.Fatalf("native compression footer=%v", err)
				}
			})
		}
	}
}

func TestTARStreamRefusalsPreserveCausesAndWithdrawExtent(t *testing.T) {
	t.Parallel()
	for _, compression := range []filestore.TARCompression{filestore.TARPlain, filestore.TARGzip} {
		for _, fault := range []string{"short-source", "long-source", "source-failure", "invalid-path", "invalid-mode", "nil-source", "producer-error", "producer-panic", "short-write", "destination-error", "close-error", "cancel-during-body"} {
			t.Run(compression.String()+"/"+fault, func(t *testing.T) {
				t.Parallel()
				ctx, cancel := context.WithCancelCause(t.Context())
				defer cancel(nil)
				path, _ := core.ParseRelativePath("entry")
				length, _ := core.NewByteLength(3)
				entry := filestore.TAREntry{Kind: filestore.PathKindRegularFile, Path: path, Bytes: length, Mode: 0600, Source: bytes.NewReader([]byte("abc"))}
				var output bytes.Buffer
				var destination io.Writer = &output
				var want error
				producerReturned := false
				var closeFailure *tarStreamCloseFault
				switch fault {
				case "short-source":
					entry.Source = bytes.NewReader([]byte("ab"))
					want = io.ErrUnexpectedEOF
				case "long-source":
					entry.Source = bytes.NewReader([]byte("abcd"))
					want = tar.ErrWriteTooLong
				case "source-failure":
					entry.Source = &prefixNativeRefusalReader{data: []byte("abc"), cause: io.ErrNoProgress}
					want = io.ErrNoProgress
				case "invalid-path":
					entry.Path = core.RelativePath{}
					want = core.ErrPrimitiveContract
				case "invalid-mode":
					entry.Mode = fs.ModeDir
					want = core.ErrFilestoreContract
				case "nil-source":
					entry.Source = nil
					want = core.ErrFilestoreContract
				case "producer-error":
					want = io.ErrClosedPipe
				case "producer-panic":
					want = core.ErrFilestoreCallbackPanic
				case "short-write":
					destination = tarStreamShortWriter{}
					want = io.ErrShortWrite
				case "destination-error":
					destination = prefixNativeRefusalWriter{cause: io.ErrClosedPipe}
					want = io.ErrClosedPipe
				case "close-error":
					closeFailure = &tarStreamCloseFault{returned: &producerReturned}
					destination = closeFailure
					want = io.ErrClosedPipe
				case "cancel-during-body":
					entry.Source = &tarStreamCancelReader{cancel: cancel}
					want = io.ErrNoProgress
				}
				got, err := filestore.StreamTAR(ctx, filestore.TARWriteRequest{Destination: destination, Compression: compression, Produce: func(_ context.Context, add func(filestore.TAREntry) (filestore.ContentIndexEntry, error)) error {
					if fault == "close-error" {
						producerReturned = true
						return nil
					}
					_, entryErr := add(entry)
					if fault == "producer-error" {
						return io.ErrClosedPipe
					}
					if fault == "producer-panic" {
						panic("owned callback panic")
					}
					if entryErr != nil {
						if _, again := add(entry); !errors.Is(again, entryErr) {
							t.Fatalf("ignored entry error replaced: %v/%v", entryErr, again)
						}
					}
					return nil
				}})
				if got != (core.ByteLength{}) || !errors.Is(err, want) {
					t.Fatalf("failure returned extent=%v cause=%v want=%v", got, err, want)
				}
				if closeFailure != nil && !closeFailure.failed {
					t.Fatal("fixture never reached a real native encoder-close write")
				}
			})
		}
	}
}

type tarStreamShortWriter struct{}

func (tarStreamShortWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return len(p) - 1, nil
}

type tarStreamCloseFault struct {
	returned *bool
	failed   bool
}

func (w *tarStreamCloseFault) Write(p []byte) (int, error) {
	if *w.returned {
		w.failed = true
		return 0, io.ErrClosedPipe
	}
	return len(p), nil
}

type tarStreamCancelReader struct{ cancel context.CancelCauseFunc }

func (r *tarStreamCancelReader) Read(p []byte) (int, error) {
	r.cancel(io.ErrNoProgress)
	if len(p) > 0 {
		p[0] = 'a'
		return 1, nil
	}
	return 0, nil
}

func TestTARStreamRefusesStoppedOrUnownedIngress(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"nil-context", "cancelled", "nil-destination", "nil-producer", "unknown-compression"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			calls := 0
			var output bytes.Buffer
			request := filestore.TARWriteRequest{Destination: &output, Compression: filestore.TARPlain, Produce: func(context.Context, func(filestore.TAREntry) (filestore.ContentIndexEntry, error)) error {
				calls++
				return nil
			}}
			var want error = core.ErrFilestoreContract
			switch name {
			case "nil-context":
				ctx = nil
				want = core.ErrNilContext
			case "cancelled":
				cancel(io.ErrNoProgress)
				want = io.ErrNoProgress
			case "nil-destination":
				request.Destination = nil
			case "nil-producer":
				request.Produce = nil
			case "unknown-compression":
				request.Compression = filestore.TARCompressionUnknown
			}
			got, err := filestore.StreamTAR(ctx, request)
			if got != (core.ByteLength{}) || !errors.Is(err, want) || calls != 0 || output.Len() != 0 {
				t.Fatalf("unowned ingress extent=%v err=%v calls=%d bytes=%d", got, err, calls, output.Len())
			}
		})
	}
}

func FuzzTARStreamMatchesIndexedBytes(f *testing.F) {
	for _, data := range [][]byte{nil, {}, []byte("native TAR"), bytes.Repeat([]byte{0xa5}, 513)} {
		f.Add(data, false)
		f.Add(data, true)
	}
	f.Fuzz(func(t *testing.T, data []byte, compressed bool) {
		path, _ := core.ParseRelativePath("entry")
		extent, _ := core.NewByteLength(uint64(len(data)))
		compression := filestore.TARPlain
		if compressed {
			compression = filestore.TARGzip
		}
		produce := func(_ context.Context, add func(filestore.TAREntry) (filestore.ContentIndexEntry, error)) error {
			got, err := add(filestore.TAREntry{Kind: filestore.PathKindRegularFile, Path: path, Bytes: extent, Mode: 0600, Source: bytes.NewReader(data)})
			if err == nil && (got.Extent != extent || got.Digest != core.SHA256Of(data)) {
				t.Fatalf("entry content changed: %+v", got)
			}
			return err
		}
		var indexed, stream bytes.Buffer
		before, beforeErr := filestore.WriteTAR(t.Context(), filestore.TARWriteRequest{Destination: &indexed, Compression: compression, Produce: produce})
		got, err := filestore.StreamTAR(t.Context(), filestore.TARWriteRequest{Destination: &stream, Compression: compression, Produce: produce})
		if beforeErr != nil || err != nil || got != before.Extent || got.Uint64() != uint64(stream.Len()) || !bytes.Equal(indexed.Bytes(), stream.Bytes()) {
			t.Fatalf("native stream agreement lost: %v/%v", beforeErr, err)
		}
	})
}
