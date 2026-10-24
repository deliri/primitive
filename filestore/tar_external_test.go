package filestore_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/testserial"
)

func TestTARNativeEncodingPreservesContentMetadataAndClosure(t *testing.T) {
	t.Parallel()
	for _, compression := range []filestore.TARCompression{filestore.TARPlain, filestore.TARGzip} {
		for _, size := range []int{0, 1, 2, 31, 32, 33, 127, 128, 129, 255, 256, 257, 511, 512, 513, 1023, 1024, 1025, 4095, 4096, 4097, 32767, 32768, 32769, 65535, 65536, 65537} {
			t.Run(fmt.Sprintf("%s/bytes_%d", compression, size), func(t *testing.T) {
				t.Parallel()
				data := make([]byte, size)
				for i := range data {
					data[i] = byte(i)
				}
				path, err := core.ParseRelativePath("directory/evidence.bin")
				if err != nil {
					t.Fatal(err)
				}
				extent, err := core.NewByteLength(uint64(size))
				if err != nil {
					t.Fatal(err)
				}
				var output bytes.Buffer
				content, err := filestore.WriteTAR(t.Context(), filestore.TARWriteRequest{Destination: &output, Compression: compression, Produce: func(ctx context.Context, add func(filestore.TAREntry) (filestore.ContentIndexEntry, error)) error {
					for _, mode := range []uint32{0400, 0755} {
						entry := filestore.TAREntry{Kind: filestore.PathKindRegularFile, Path: path, Source: bytes.NewReader(data), Bytes: extent, Mode: fs.FileMode(mode)}
						got, err := add(entry)
						if err != nil {
							return err
						}
						if got.Extent != extent || got.Digest != core.NewSHA256Digest(sha256.Sum256(data)) {
							t.Fatalf("native entry content=%+v,want exact original", got)
						}
					}
					return nil
				}})
				if err != nil || content.Extent.Uint64() != uint64(output.Len()) || content.Digest != core.NewSHA256Digest(sha256.Sum256(output.Bytes())) {
					t.Fatalf("native archive content=%+v/%v,want exact encoded bytes", content, err)
				}
				if compression == filestore.TARPlain {
					wantExtent := 2*(512+((size+511)/512)*512) + 1024
					if output.Len() != wantExtent {
						t.Fatalf("native TAR physical extent=%d,want%d including both end blocks", output.Len(), wantExtent)
					}
				}
				var source io.Reader = bytes.NewReader(output.Bytes())
				var gz *gzip.Reader
				if compression == filestore.TARGzip {
					gz, err = gzip.NewReader(source)
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
				for _, mode := range []int64{0400, 0755} {
					header, err := archive.Next()
					if err != nil {
						t.Fatal(err)
					}
					body, err := io.ReadAll(archive)
					if err != nil {
						t.Fatal(err)
					}
					if header.Name != "directory/evidence.bin" || header.Typeflag != tar.TypeReg || header.Size != int64(size) || header.Mode != mode || !bytes.Equal(body, data) {
						t.Fatalf("native archive entry=%+v,body=%d,want exact name,mode,extent and bytes", header, len(body))
					}
				}
				if _, err := archive.Next(); err != io.EOF {
					t.Fatalf("native TAR closure=%v,want bare EOF", err)
				}
				if _, err := io.Copy(io.Discard, source); err != nil {
					t.Fatalf("native compression closure=%v,want valid trailer", err)
				}
			})
		}
	}
}

func TestTARNativeRefusalWithdrawsContentAndSurvivesIgnoredEntryError(t *testing.T) {
	t.Parallel()
	for _, compression := range []filestore.TARCompression{filestore.TARPlain, filestore.TARGzip} {
		for _, fault := range []string{"short source", "long source", "source failure", "producer refusal", "producer panic", "unset path", "nil source", "invalid mode", "destination failure"} {
			t.Run(compression.String()+"/"+fault, func(t *testing.T) {
				t.Parallel()
				path, err := core.ParseRelativePath("entry")
				if err != nil {
					t.Fatal(err)
				}
				extent, err := core.NewByteLength(3)
				if err != nil {
					t.Fatal(err)
				}
				entry := filestore.TAREntry{Kind: filestore.PathKindRegularFile, Path: path, Source: bytes.NewReader([]byte("abc")), Bytes: extent, Mode: 0600}
				var output bytes.Buffer
				var destination io.Writer = &output
				var want error
				switch fault {
				case "short source":
					entry.Source = bytes.NewReader([]byte("ab"))
					want = io.ErrUnexpectedEOF
				case "long source":
					entry.Source = bytes.NewReader([]byte("abcd"))
					want = tar.ErrWriteTooLong
				case "source failure":
					entry.Source = &prefixNativeRefusalReader{data: []byte("abc"), cause: io.ErrNoProgress}
					want = io.ErrNoProgress
				case "producer refusal":
					want = io.ErrClosedPipe
				case "producer panic":
					want = core.ErrFilestoreCallbackPanic
				case "nil source":
					entry.Source = nil
					want = core.ErrFilestoreContract
				case "unset path":
					entry.Path = core.RelativePath{}
					want = core.ErrPrimitiveContract
				case "invalid mode":
					entry.Mode = fs.ModeDir
					want = core.ErrFilestoreContract
				case "destination failure":
					destination = prefixNativeRefusalWriter{cause: io.ErrClosedPipe}
					want = io.ErrClosedPipe
				default:
					t.Fatal("unowned fault")
				}
				got, err := filestore.WriteTAR(t.Context(), filestore.TARWriteRequest{Destination: destination, Compression: compression, Produce: func(_ context.Context, add func(filestore.TAREntry) (filestore.ContentIndexEntry, error)) error {
					_, entryErr := add(entry)
					if fault == "producer panic" {
						panic(io.ErrNoProgress)
					}
					if fault == "producer refusal" {
						return io.ErrClosedPipe
					}
					if entryErr != nil {
						if _, err := add(entry); !errors.Is(err, entryErr) {
							t.Fatalf("ignored native entry refusal was replaced: %v/%v", entryErr, err)
						}
					}
					return nil
				}})
				if got != (filestore.ContentIndexEntry{}) || !errors.Is(err, want) {
					t.Fatalf("refused native TAR=%+v/%v,want no proof and %v", got, err, want)
				}
			})
		}
	}
}

func TestTARClosedCompressionAndStoppedIngress(t *testing.T) {
	t.Parallel()
	for value := 0; value < 256; value++ {
		kind := filestore.TARCompression(value)
		valid := kind == filestore.TARPlain || kind == filestore.TARGzip
		if kind.IsValid() != valid || (kind.Validate() == nil) != valid {
			t.Fatalf("compression %d disagrees with closed set", value)
		}
		if valid == (kind.String() == core.UnknownEnumDiagnostic) {
			t.Fatalf("compression %d canonical name=%q", value, kind.String())
		}
	}
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(io.ErrNoProgress)
	for _, row := range []struct {
		name string
		ctx  context.Context
		want error
	}{{name: "cancelled", ctx: ctx, want: io.ErrNoProgress}, {name: "absent", want: core.ErrNilContext}} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			var output bytes.Buffer
			calls := 0
			got, err := filestore.WriteTAR(row.ctx, filestore.TARWriteRequest{Destination: &output, Compression: filestore.TARGzip, Produce: func(context.Context, func(filestore.TAREntry) (filestore.ContentIndexEntry, error)) error {
				calls++
				return nil
			}})
			if got != (filestore.ContentIndexEntry{}) || !errors.Is(err, row.want) || calls != 0 || output.Len() != 0 {
				t.Fatalf("stopped TAR=%+v/%v,calls=%d,bytes=%d", got, err, calls, output.Len())
			}
		})
	}
}

func TestTARRequestRefusesUnownedOutputBeforeProducer(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"nil destination", "nil producer", "unknown compression"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var output bytes.Buffer
			calls := 0
			request := filestore.TARWriteRequest{Destination: &output, Compression: filestore.TARGzip, Produce: func(context.Context, func(filestore.TAREntry) (filestore.ContentIndexEntry, error)) error {
				calls++
				return nil
			}}
			switch name {
			case "nil destination":
				request.Destination = nil
			case "nil producer":
				request.Produce = nil
			case "unknown compression":
				request.Compression = filestore.TARCompressionUnknown
			default:
				t.Fatal("unknown request")
			}
			got, err := filestore.WriteTAR(t.Context(), request)
			if got != (filestore.ContentIndexEntry{}) || !errors.Is(err, core.ErrFilestoreContract) || calls != 0 || output.Len() != 0 {
				t.Fatalf("unowned TAR request=%+v/%v,calls=%d,bytes=%d", got, err, calls, output.Len())
			}
		})
	}
}

func FuzzTARNativeEntryAndOutputAgreement(f *testing.F) {
	directory := f.TempDir()
	target := nativeTARLinkTarget(f, directory, "seed-link", "../opaque:[1]/missing")
	for _, kind := range []filestore.PathKind{filestore.PathKindRegularFile, filestore.PathKindDirectory, filestore.PathKindSymbolicLink} {
		if err := kind.Validate(); err != nil {
			f.Fatal(err)
		}
		f.Add([]byte("native TAR"), true, uint8(kind))
		f.Add([]byte{}, false, uint8(kind))
	}
	f.Add([]byte("unknown kind"), false, uint8(filestore.PathKindUnknown))
	f.Add([]byte("future kind"), true, uint8(255))
	f.Fuzz(func(t *testing.T, data []byte, compressed bool, rawKind uint8) {
		path, err := core.ParseRelativePath("entry")
		if err != nil {
			t.Fatal(err)
		}
		extent, err := core.NewByteLength(uint64(len(data)))
		if err != nil {
			t.Fatal(err)
		}
		compression := filestore.TARPlain
		if compressed {
			compression = filestore.TARGzip
		}
		var output bytes.Buffer
		got, err := filestore.WriteTAR(t.Context(), filestore.TARWriteRequest{Destination: &output, Compression: compression, Produce: func(_ context.Context, add func(filestore.TAREntry) (filestore.ContentIndexEntry, error)) error {
			entry := filestore.TAREntry{Kind: filestore.PathKind(rawKind), Source: bytes.NewReader(data), Path: path, Bytes: extent, Mode: 0600}
			if entry.Kind == filestore.PathKindDirectory || entry.Kind == filestore.PathKindSymbolicLink {
				entry.Source = nil
				entry.Bytes = core.ByteLength{}
			}
			if entry.Kind == filestore.PathKindSymbolicLink {
				entry.LinkTarget = target
			}
			observed, err := add(entry)
			wantBody := data
			if entry.Kind != filestore.PathKindRegularFile {
				wantBody = nil
			}
			if err == nil && (observed.Extent != entry.Bytes || observed.Digest != core.SHA256Of(wantBody)) {
				t.Fatalf("entry content=%+v,want exact mutated bytes", observed)
			}
			return err
		}})
		kind := filestore.PathKind(rawKind)
		if kind != filestore.PathKindRegularFile && kind != filestore.PathKindDirectory && kind != filestore.PathKindSymbolicLink {
			if !errors.Is(err, core.ErrFilestoreContract) || got != (filestore.ContentIndexEntry{}) {
				t.Fatalf("unknown kind=%d content=%+v/%v, want typed refusal and zero", rawKind, got, err)
			}
			return
		}
		if err != nil || got.Extent.Uint64() != uint64(output.Len()) || got.Digest != core.NewSHA256Digest(sha256.Sum256(output.Bytes())) {
			t.Fatalf("encoded output content=%+v/%v,want exact encoded bytes", got, err)
		}
		var source io.Reader = bytes.NewReader(output.Bytes())
		if compressed {
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
		header, err := archive.Next()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(archive)
		wantKind, wantTarget := byte(tar.TypeReg), ""
		wantBody := data
		switch kind {
		case filestore.PathKindRegularFile:
			wantKind = tar.TypeReg
		case filestore.PathKindDirectory:
			wantKind, wantBody = tar.TypeDir, nil
		case filestore.PathKindSymbolicLink:
			wantKind, wantTarget, wantBody = tar.TypeSymlink, target.String(), nil
		default:
			t.Fatalf("admitted kind=%d, want a supported native kind", rawKind)
		}
		if err != nil || header.Name != "entry" || header.Mode != 0600 || header.Typeflag != wantKind || header.Linkname != wantTarget || header.Size != int64(len(wantBody)) || !bytes.Equal(body, wantBody) {
			t.Fatalf("decoded mutation=%+v/%v,bytes=%d,want original admitted bytes", header, err, len(body))
		}
		if _, err := archive.Next(); err != io.EOF {
			t.Fatalf("native TAR end=%v,want bare EOF", err)
		}
		if _, err := io.Copy(io.Discard, source); err != nil {
			t.Fatal(err)
		}
	})
}

func BenchmarkTARForwardEncodingCardinality(b *testing.B) {
	for _, count := range []int{1, 1024, 65537} {
		b.Run(fmt.Sprintf("entries_%d", count), func(b *testing.B) {
			path, err := core.ParseRelativePath("entry")
			if err != nil {
				b.Fatal(err)
			}
			source := bytes.NewReader(nil)
			request := filestore.TARWriteRequest{Destination: io.Discard, Compression: filestore.TARGzip, Produce: func(_ context.Context, add func(filestore.TAREntry) (filestore.ContentIndexEntry, error)) error {
				for range count {
					source.Reset(nil)
					got, err := add(filestore.TAREntry{Kind: filestore.PathKindRegularFile, Source: source, Path: path, Bytes: core.ByteLength{}, Mode: 0600})
					if err != nil {
						return err
					}
					if got.Extent.Uint64() != 0 {
						b.Fatal("empty TAR entry invented content")
					}
				}
				return nil
			}}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				got, err := filestore.WriteTAR(b.Context(), request)
				if err != nil || got.Extent.Uint64() == 0 {
					b.Fatalf("encoded TAR=%+v/%v,want completed native output", got, err)
				}
			}
		})
	}
}

func TestTARRetainedMemoryDoesNotGrowWithEntryCardinality(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	const allowance = 2 << 20
	for _, count := range []int{256, 8192, 65537} {
		before, err := hostfacts.ObserveCollectedGoHeap(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		var retained int64
		path, err := core.ParseRelativePath("entry")
		if err != nil {
			t.Fatal(err)
		}
		source := bytes.NewReader(nil)
		got, err := filestore.WriteTAR(t.Context(), filestore.TARWriteRequest{Destination: io.Discard, Compression: filestore.TARGzip, Produce: func(_ context.Context, add func(filestore.TAREntry) (filestore.ContentIndexEntry, error)) error {
			for range count {
				source.Reset(nil)
				if _, err := add(filestore.TAREntry{Kind: filestore.PathKindRegularFile, Source: source, Path: path, Bytes: core.ByteLength{}, Mode: 0600}); err != nil {
					return err
				}
			}
			after, err := hostfacts.ObserveCollectedGoHeap(t.Context())
			if err != nil {
				return err
			}
			retained = int64(after.Uint64()) - int64(before.Uint64())
			return nil
		}})
		t.Logf("entries=%d retained_bytes=%d allowance=%d", count, retained, allowance)
		if err != nil || got.Extent.Uint64() == 0 || retained > allowance {
			t.Fatalf("native TAR=%+v/%v,retained=%d,want completed encoding and <=%d", got, err, retained, allowance)
		}
	}
}
