package filestore_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/testserial"
)

func nativeReadTARFixture(t testing.TB, compressed bool, size int, members int) []byte {
	t.Helper()
	var encoded bytes.Buffer
	var destination io.Writer = &encoded
	var compression *gzip.Writer
	if compressed {
		compression = gzip.NewWriter(destination)
		destination = compression
	}
	archive := tar.NewWriter(destination)
	for index := range members {
		if err := archive.WriteHeader(&tar.Header{Name: fmt.Sprintf("entry%d", index), Size: int64(size), Mode: 0751, Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		for remaining := size; remaining > 0; {
			var body [4096]byte
			for i := range body {
				body[i] = byte(i + index)
			}
			n := min(remaining, len(body))
			if _, err := archive.Write(body[:n]); err != nil {
				t.Fatal(err)
			}
			remaining -= n
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if compression != nil {
		if err := compression.Close(); err != nil {
			t.Fatal(err)
		}
	}
	return encoded.Bytes()
}

func TestReadTARPreservesNativeEntriesAcrossWindows(t *testing.T) {
	t.Parallel()
	for _, compressed := range []bool{false, true} {
		for _, size := range []int{0, 1, 511, 512, 513, 4095, 4096, 4097, 32767, 32768, 32769, 65537} {
			t.Run(fmt.Sprintf("gzip_%t/bytes_%d", compressed, size), func(t *testing.T) {
				t.Parallel()
				source := nativeReadTARFixture(t, compressed, size, 3)
				for _, unread := range []bool{false, true} {
					count := 0
					kind := filestore.TARPlain
					if compressed {
						kind = filestore.TARGzip
					}
					err := filestore.ReadTAR(t.Context(), filestore.TARReadRequest{Source: bytes.NewReader(source), Compression: kind, Visit: func(ctx context.Context, entry filestore.TARReadEntry) error {
						if err := entry.Validate(); err != nil {
							return err
						}
						if entry.Header.Name != fmt.Sprintf("entry%d", count) || entry.Header.Size != int64(size) || entry.Header.Mode != 0751 || entry.Header.Typeflag != tar.TypeReg {
							t.Fatalf("native TAR member%+v want exact member%d extent%d", entry.Header, count, size)
						}
						if !unread {
							var window [4096]byte
							position := 0
							for {
								n, err := entry.Source.Read(window[:])
								for i, value := range window[:n] {
									if value != byte(position+i+count) {
										t.Fatalf("native member%d byte%d got%d want%d", count, position+i, value, byte(position+i+count))
									}
								}
								position += n
								if err == io.EOF {
									break
								}
								if err != nil {
									return err
								}
							}
							if position != size {
								t.Fatalf("native member extent%d want%d", position, size)
							}
						}
						count++
						return nil
					}})
					if err != nil || count != 3 {
						t.Fatalf("native TAR unread%t count%d error%v want3/nil", unread, count, err)
					}
				}
			})
		}
	}
}

func TestReadTARRefusesBrokenNativeGZIPClosure(t *testing.T) {
	t.Parallel()
	for _, truncate := range []bool{false, true} {
		t.Run(fmt.Sprint(truncate), func(t *testing.T) {
			t.Parallel()
			source := nativeReadTARFixture(t, true, 8, 1)
			want := error(gzip.ErrChecksum)
			if truncate {
				source = source[:len(source)-4]
				want = io.ErrUnexpectedEOF
			} else {
				source[len(source)-8] ^= 1
			}
			count := 0
			err := filestore.ReadTAR(t.Context(), filestore.TARReadRequest{Source: bytes.NewReader(source), Compression: filestore.TARGzip, Visit: func(context.Context, filestore.TARReadEntry) error { count++; return nil }})
			if !errors.Is(err, want) || !errors.Is(err, core.ErrFilestoreSource) || count != 1 {
				t.Fatalf("native gzip loans%d refusal%v want1/%v and source identity", count, err, want)
			}
		})
	}
}

func TestReadTARStopsAtConsumerRefusalAndCancellation(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"refusal", "cancellation", "panic"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			count := 0
			err := filestore.ReadTAR(ctx, filestore.TARReadRequest{Source: bytes.NewReader(nativeReadTARFixture(t, false, 1, 3)), Compression: filestore.TARPlain, Visit: func(context.Context, filestore.TARReadEntry) error {
				count++
				switch mode {
				case "panic":
					panic(io.ErrClosedPipe)
				case "cancellation":
					cancel(io.ErrClosedPipe)
					return nil
				default:
					return io.ErrClosedPipe
				}
			}})
			want := error(io.ErrClosedPipe)
			if mode == "panic" {
				want = core.ErrFilestoreCallbackPanic
			}
			if count != 1 || !errors.Is(err, want) {
				t.Fatalf("native TAR mode%s loans%d refusal%v want1/%v", mode, count, err, want)
			}
		})
	}
}

func TestReadTARLoansBeforeFutureBytesWithFixedRetention(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, members := range []int{64, 4097} {
		t.Run(fmt.Sprint(members), func(t *testing.T) {
			source := nativeReadTARFixture(t, false, 1, members)
			reader := bytes.NewReader(source)
			before, err := hostfacts.ObserveGoAllocationTotal(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			err = filestore.ReadTAR(t.Context(), filestore.TARReadRequest{Source: reader, Compression: filestore.TARPlain, Visit: func(context.Context, filestore.TARReadEntry) error {
				count++
				if reader.Len() != len(source)-512 {
					t.Fatalf("native first header consumed%d bytes want512 before body or future member", len(source)-reader.Len())
				}
				return io.ErrClosedPipe
			}})
			after, observeErr := hostfacts.ObserveGoAllocationTotal(t.Context())
			if observeErr != nil {
				t.Fatal(observeErr)
			}
			allocated := after.Uint64() - before.Uint64()
			if count != 1 || !errors.Is(err, io.ErrClosedPipe) || allocated > 80<<10 {
				t.Fatalf("native TAR population%d loans%d refusal%v allocated%d want1/preserved/fixed window", members, count, err, allocated)
			}
			t.Logf("native member population%d first loan allocated%d bytes", members, allocated)
		})
	}
}

func TestReadTARRefusesInvalidRequestBeforeSource(t *testing.T) {
	t.Parallel()
	var typedNil *bytes.Reader
	for _, row := range []struct {
		name    string
		request filestore.TARReadRequest
	}{
		{name: "absent source", request: filestore.TARReadRequest{Compression: filestore.TARPlain, Visit: func(context.Context, filestore.TARReadEntry) error { return nil }}},
		{name: "typed nil source", request: filestore.TARReadRequest{Source: typedNil, Compression: filestore.TARPlain, Visit: func(context.Context, filestore.TARReadEntry) error { return nil }}},
		{name: "absent consumer", request: filestore.TARReadRequest{Source: bytes.NewReader(nil), Compression: filestore.TARPlain}},
		{name: "unknown compression", request: filestore.TARReadRequest{Source: bytes.NewReader(nil), Compression: filestore.TARCompression(255), Visit: func(context.Context, filestore.TARReadEntry) error { return nil }}},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			if err := filestore.ReadTAR(t.Context(), row.request); !errors.Is(err, core.ErrFilestoreContract) {
				t.Fatalf("native invalid TAR request refusal%v want contract identity", err)
			}
		})
	}
	for _, entry := range []filestore.TARReadEntry{{}, {Header: &tar.Header{}}, {Source: bytes.NewReader(nil)}, {Header: &tar.Header{}, Source: typedNil}} {
		if err := entry.Validate(); !errors.Is(err, core.ErrFilestoreContract) {
			t.Fatalf("invalid native TAR loan refusal%v want contract identity", err)
		}
	}
	count := 0
	request := filestore.TARReadRequest{Source: bytes.NewReader(nil), Compression: filestore.TARPlain, Visit: func(context.Context, filestore.TARReadEntry) error { count++; return nil }}
	if err := filestore.ReadTAR(nil, request); !errors.Is(err, core.ErrNilContext) || count != 0 {
		t.Fatalf("nil native context visits%d refusal%v want0/nil-context identity", count, err)
	}
}

func tarReadEntryDigestForTest(digest hash.Hash, header *tar.Header, source io.Reader) error {
	if _, err := fmt.Fprintf(digest, "%q/%d/%d/%d/%q;", header.Name, header.Typeflag, header.Mode, header.Size, header.Linkname); err != nil {
		return err
	}
	_, err := io.Copy(digest, source)
	return err
}

func FuzzReadTARMatchesNativeFramingAndClosure(f *testing.F) {
	path, err := core.ParseRelativePath("entry")
	if err != nil {
		f.Fatal(err)
	}
	for _, compressed := range []bool{false, true} {
		kind := filestore.TARPlain
		if compressed {
			kind = filestore.TARGzip
		}
		var seed bytes.Buffer
		_, err := filestore.WriteTAR(f.Context(), filestore.TARWriteRequest{Destination: &seed, Compression: kind, Produce: func(ctx context.Context, add func(filestore.TAREntry) (filestore.ContentIndexEntry, error)) error {
			_, err := add(filestore.TAREntry{Path: path, Source: bytes.NewReader(nil), Mode: 0600})
			return err
		}})
		if err != nil {
			f.Fatal(err)
		}
		f.Add(seed.Bytes(), compressed)
	}
	f.Add([]byte("not a native archive"), false)
	f.Fuzz(func(t *testing.T, source []byte, compressed bool) {
		var input io.Reader = bytes.NewReader(source)
		var nativeCompression *gzip.Reader
		var wantErr error
		kind := filestore.TARPlain
		if compressed {
			kind = filestore.TARGzip
			nativeCompression, wantErr = gzip.NewReader(input)
			if wantErr == nil {
				input = nativeCompression
			}
		}
		wantDigest := sha256.New()
		wantCount := 0
		if wantErr == nil {
			archive := tar.NewReader(input)
			for {
				header, err := archive.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					wantErr = err
					break
				}
				wantCount++
				if err := tarReadEntryDigestForTest(wantDigest, header, archive); err != nil {
					wantErr = err
					break
				}
			}
			if wantErr == nil && nativeCompression != nil {
				_, wantErr = io.Copy(io.Discard, nativeCompression)
			}
		}
		if nativeCompression != nil {
			wantErr = errors.Join(wantErr, nativeCompression.Close())
		}
		gotDigest := sha256.New()
		gotCount := 0
		gotErr := filestore.ReadTAR(t.Context(), filestore.TARReadRequest{Source: bytes.NewReader(source), Compression: kind, Visit: func(ctx context.Context, entry filestore.TARReadEntry) error {
			if err := entry.Validate(); err != nil {
				return err
			}
			gotCount++
			return tarReadEntryDigestForTest(gotDigest, entry.Header, entry.Source)
		}})
		if gotCount != wantCount || !bytes.Equal(gotDigest.Sum(nil), wantDigest.Sum(nil)) || (gotErr == nil) != (wantErr == nil) {
			t.Fatalf("native TAR count%d/%d refusal%v/%v or byte digest disagreement", gotCount, wantCount, gotErr, wantErr)
		}
		if gotErr != nil && !errors.Is(gotErr, core.ErrFilestoreSource) {
			t.Fatalf("native framing refusal%v want source identity", gotErr)
		}
	})
}
