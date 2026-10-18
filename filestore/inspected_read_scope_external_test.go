package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	primitiverelease "github.com/deliri/primitive/v2026/release"
)

func inspectedReadLocation(t testing.TB, body []byte, mode fs.FileMode) (filestore.Location, string) {
	t.Helper()
	dir := t.TempDir()
	native := filepath.Join(dir, "source")
	if err := os.WriteFile(native, body, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(native, mode); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	path, err := core.ParseRelativePath("source")
	if err != nil {
		t.Fatal(err)
	}
	return filestore.Location{Root: root, Path: path}, native
}

func TestInspectedReadScopeBindsMetadataAndBytesToHeldNativeObject(t *testing.T) {
	t.Parallel()
	for _, size := range []int{0, 1, 4097, 65537} {
		for _, mode := range []fs.FileMode{0600, 0640, 0755} {
			for _, replace := range []bool{false, true} {
				t.Run(fmt.Sprintf("bytes_%d/mode_%o/replaced_%v", size, mode, replace), func(t *testing.T) {
					t.Parallel()
					body := make([]byte, size)
					for i := range body {
						body[i] = byte(i)
					}
					location, path := inspectedReadLocation(t, body, mode)
					var borrowed io.ReadSeeker
					scope, err := filestore.WithInspectedReadScope(t.Context(), filestore.InspectedReadScopeRequest{Location: location, Use: func(ctx context.Context, read filestore.InspectedRead) error {
						borrowed = read.Source
						if err := read.Validate(); err != nil {
							return err
						}
						if replace {
							if err := os.Rename(path, path+"-original"); err != nil {
								return err
							}
							if err := os.WriteFile(path, []byte("foreign path occupant"), 0600); err != nil {
								return err
							}
						}
						extent, err := read.Metadata.SizeBytes()
						if err != nil {
							return err
						}
						permissions, err := read.Metadata.Permissions()
						if err != nil {
							return err
						}
						bits, err := permissions.Bits()
						if err != nil {
							return err
						}
						kind, err := read.Metadata.Kind()
						if err != nil {
							return err
						}
						if extent.Uint64() != uint64(size) || bits != uint32(mode) || kind != filestore.PathKindRegularFile {
							return fmt.Errorf("metadata=%d/%o/%v,want %d/%o/regular", extent.Uint64(), bits, kind, size, mode)
						}
						var actual bytes.Buffer
						content, err := filestore.CopyContent(ctx, filestore.CopyContentRequest{Source: read.Source, Destination: &actual})
						if err != nil {
							return err
						}
						if !bytes.Equal(actual.Bytes(), body) || content.Digest != core.SHA256Of(body) || content.Extent != extent {
							return fmt.Errorf("observed bytes/metadata disagree: content=%+v actual=%d", content, actual.Len())
						}
						return nil
					}})
					if err != nil || scope.Validate() != nil || scope.OperationError() != nil || scope.CleanupError() != nil {
						t.Fatalf("native inspected scope=%+v/%v", scope, err)
					}
					if borrowed == nil {
						t.Fatal("owning Native source was never lent")
					}
					if _, err := borrowed.Read(make([]byte, 1)); !errors.Is(err, os.ErrClosed) {
						t.Fatalf("expired native reader=%v,want closed", err)
					}
					if replace {
						foreign, err := os.ReadFile(path)
						if err != nil || string(foreign) != "foreign path occupant" {
							t.Fatalf("scope changed replacement=%q/%v", foreign, err)
						}
					}
				})
			}
		}
	}
}

func TestInspectedReadScopeWithdrawsAuthorityAcrossConsumerFailures(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		Name          string
		Failure       error
		Panic, Cancel bool
	}{
		{Name: "ordinary source refusal", Failure: io.ErrUnexpectedEOF},
		{Name: "contract refusal", Failure: core.ErrFilestoreContract},
		{Name: "sanitized consumer panic", Failure: core.ErrFilestoreCallbackPanic, Panic: true},
		{Name: "cancelled successful callback", Failure: context.Canceled, Cancel: true},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			location, _ := inspectedReadLocation(t, []byte("source"), 0600)
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			var borrowed io.ReadSeeker
			scope, err := filestore.WithInspectedReadScope(ctx, filestore.InspectedReadScopeRequest{Location: location, Use: func(_ context.Context, read filestore.InspectedRead) error {
				borrowed = read.Source
				if tc.Panic {
					panic(io.ErrShortWrite)
				}
				if tc.Cancel {
					cancel(io.ErrNoProgress)
					return nil
				}
				return tc.Failure
			}})
			if err != nil || scope.Validate() != nil || scope.CleanupError() != nil || !errors.Is(scope.OperationError(), tc.Failure) {
				t.Fatalf("refused scope=%+v/%v,want completed native close and %v", scope, err, tc.Failure)
			}
			if tc.Cancel && !errors.Is(scope.OperationError(), io.ErrNoProgress) {
				t.Fatalf("lost native cause=%v", scope.OperationError())
			}
			if tc.Panic && errors.Is(scope.OperationError(), io.ErrShortWrite) {
				t.Fatal("panic payload escaped Native boundary")
			}
			if borrowed == nil {
				t.Fatal("missing borrowed native source")
			}
			if _, err := borrowed.Read(make([]byte, 1)); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("refused loan remains live=%v", err)
			}
		})
	}
}

func TestInspectedReadScopeRefusesBeforeLoanWithoutInventingCompletion(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		Name                                                           string
		NilContext, Cancel, NoConsumer, NoLocation, Missing, Directory bool
		Want                                                           error
	}{
		{Name: "nil context", NilContext: true, Want: core.ErrNilContext},
		{Name: "cancelled context", Cancel: true, Want: context.Canceled},
		{Name: "missing consumer", NoConsumer: true, Want: core.ErrFilestoreContract},
		{Name: "missing location", NoLocation: true, Want: core.ErrFilestoreContract},
		{Name: "missing native source", Missing: true, Want: os.ErrNotExist},
		{Name: "directory source", Directory: true, Want: core.ErrFilestoreSource},
	} {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			location, path := inspectedReadLocation(t, []byte("source"), 0600)
			if tc.Missing || tc.Directory {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			}
			if tc.Directory {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			var ctx context.Context = t.Context()
			if tc.NilContext {
				ctx = nil
			}
			if tc.Cancel {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			calls := 0
			request := filestore.InspectedReadScopeRequest{Location: location, Use: func(context.Context, filestore.InspectedRead) error { calls++; return nil }}
			if tc.NoConsumer {
				request.Use = nil
			}
			if tc.NoLocation {
				request.Location = filestore.Location{}
			}
			scope, err := filestore.WithInspectedReadScope(ctx, request)
			if !errors.Is(err, tc.Want) || scope.Validate() == nil || calls != 0 {
				t.Fatalf("refused acquisition=%+v/%v calls=%d,want %v and no scope", scope, err, calls, tc.Want)
			}
		})
	}
	if err := (filestore.InspectedRead{}).Validate(); !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatalf("unset native loan=%v", err)
	}
}

func FuzzInspectedReadScopePreservesHeldObjectBytesAndMetadata(f *testing.F) {
	f.Add([]byte{}, uint8(0), false)
	f.Add([]byte{0, 255, 17}, uint8(1), true)
	f.Add(bytes.Repeat([]byte("native"), 1024), uint8(2), true)
	f.Fuzz(func(t *testing.T, body []byte, selection uint8, replace bool) {
		if len(body) > 1<<20 {
			return
		}
		modes := [...]fs.FileMode{0600, 0640, 0755}
		mode := modes[int(selection)%len(modes)]
		location, path := inspectedReadLocation(t, body, mode)
		var loan io.ReadSeeker
		scope, err := filestore.WithInspectedReadScope(t.Context(), filestore.InspectedReadScopeRequest{Location: location, Use: func(ctx context.Context, read filestore.InspectedRead) error {
			loan = read.Source
			if err := read.Validate(); err != nil {
				return err
			}
			if replace {
				if err := os.Rename(path, path+"-original"); err != nil {
					return err
				}
				if err := os.WriteFile(path, []byte("replacement"), 0600); err != nil {
					return err
				}
			}
			extent, err := read.Metadata.SizeBytes()
			if err != nil {
				return err
			}
			permissions, err := read.Metadata.Permissions()
			if err != nil {
				return err
			}
			bits, err := permissions.Bits()
			if err != nil {
				return err
			}
			var probe [17]byte
			offset := len(body) / 2
			count, readErr := read.Source.ReadAt(probe[:], int64(offset))
			wantCount := min(len(probe), len(body)-offset)
			if count != wantCount || !bytes.Equal(probe[:count], body[offset:offset+wantCount]) || (wantCount < len(probe) && readErr != io.EOF) || (wantCount == len(probe) && readErr != nil) {
				t.Fatalf("native positional fuzz observation=%d/%v,want %d bytes and exact EOF", count, readErr, wantCount)
			}
			var output bytes.Buffer
			observed, err := filestore.CopyContent(ctx, filestore.CopyContentRequest{Source: read.Source, Destination: &output})
			if err != nil {
				return err
			}
			if extent != observed.Extent || extent.Uint64() != uint64(len(body)) || bits != uint32(mode) || observed.Digest != core.SHA256Of(body) || !bytes.Equal(output.Bytes(), body) {
				t.Fatalf("held native observations disagree:extent=%v/content=%+v/mode=%o", extent, observed, bits)
			}
			return nil
		}})
		if err != nil || scope.Validate() != nil || scope.OperationError() != nil || scope.CleanupError() != nil {
			t.Fatalf("native scope=%+v/%v", scope, err)
		}
		if loan == nil {
			t.Fatal("missing real native loan")
		}
		if _, err := loan.Read(make([]byte, 1)); !errors.Is(err, os.ErrClosed) {
			t.Fatalf("loan escaped close=%v", err)
		}
	})
}

func BenchmarkInspectedReadScopeStreamingExtent(b *testing.B) {
	for _, size := range []int{1024, 65537, 32<<20 + 1} {
		b.Run(fmt.Sprintf("bytes_%d", size), func(b *testing.B) {
			body := bytes.Repeat([]byte{17}, size)
			location, _ := inspectedReadLocation(b, body, 0600)
			expected := core.SHA256Of(body)
			b.SetBytes(int64(size))
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				scope, err := filestore.WithInspectedReadScope(b.Context(), filestore.InspectedReadScopeRequest{Location: location, Use: func(ctx context.Context, read filestore.InspectedRead) error {
					observed, err := filestore.CopyContent(ctx, filestore.CopyContentRequest{Source: read.Source, Destination: io.Discard})
					if err != nil {
						return err
					}
					extent, err := read.Metadata.SizeBytes()
					if err != nil {
						return err
					}
					if observed.Digest != expected || observed.Extent != extent || extent.Uint64() != uint64(size) {
						return core.ErrFilestoreContract
					}
					return nil
				}})
				if err != nil || scope.Validate() != nil || scope.OperationError() != nil || scope.CleanupError() != nil {
					b.Fatalf("native streaming scope=%+v/%v", scope, err)
				}
			}
		})
	}
}

func TestInspectedReadScopeLendsPositionalBytesWithoutAdvancingStream(t *testing.T) {
	t.Parallel()
	for _, replace := range []bool{false, true} {
		t.Run(fmt.Sprintf("replaced_%v", replace), func(t *testing.T) {
			t.Parallel()
			body := []byte{0, 255, 17, 31, 63}
			location, path := inspectedReadLocation(t, body, 0640)
			var borrowed filestore.InspectedReadSource
			scope, err := filestore.WithInspectedReadScope(t.Context(), filestore.InspectedReadScopeRequest{Location: location, Use: func(_ context.Context, read filestore.InspectedRead) error {
				borrowed = read.Source
				if replace {
					if err := os.Rename(path, path+"-original"); err != nil {
						return err
					}
					if err := os.WriteFile(path, []byte("foreign namespace"), 0600); err != nil {
						return err
					}
				}
				var middle [3]byte
				n, err := read.Source.ReadAt(middle[:], 1)
				if err != nil || n != 3 || middle != [3]byte{255, 17, 31} {
					t.Fatalf("native positional bytes=%v/%d/%v", middle, n, err)
				}
				var beyond [3]byte
				n, err = read.Source.ReadAt(beyond[:], 4)
				if !errors.Is(err, io.EOF) || n != 1 || beyond[0] != 63 {
					t.Fatalf("native positional EOF=%v/%d/%v", beyond, n, err)
				}
				var first [1]byte
				n, err = read.Source.Read(first[:])
				if err != nil || n != 1 || first[0] != 0 {
					t.Fatalf("positional reads advanced native stream=%v/%d/%v", first, n, err)
				}
				extent, err := read.Metadata.SizeBytes()
				if err != nil {
					return err
				}
				if extent.Uint64() != uint64(len(body)) {
					t.Fatalf("positional metadata=%v,want original extent%d", extent, len(body))
				}
				return nil
			}})
			if err != nil || scope.Validate() != nil || scope.OperationError() != nil || scope.CleanupError() != nil {
				t.Fatalf("native positional loan=%+v/%v", scope, err)
			}
			if borrowed == nil {
				t.Fatal("native positional source missing")
			}
			if _, err := borrowed.ReadAt(make([]byte, 1), 0); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("positional loan escaped scope=%v", err)
			}
		})
	}
}

func TestInspectedReadScopeSupportsNativeReleaseMetadataInspection(t *testing.T) {
	t.Parallel()
	for _, kind := range []primitiverelease.MetadataKind{primitiverelease.MetadataKindDependencies, primitiverelease.MetadataKindDocumentation, primitiverelease.MetadataKindReleaseNotes} {
		t.Run(kind.String(), func(t *testing.T) {
			t.Parallel()
			body := bytes.Repeat([]byte("native positional metadata"), 4097)
			location, _ := inspectedReadLocation(t, body, 0600)
			scope, err := filestore.WithInspectedReadScope(t.Context(), filestore.InspectedReadScopeRequest{Location: location, Use: func(_ context.Context, read filestore.InspectedRead) error {
				size, err := read.Metadata.SizeBytes()
				if err != nil {
					return err
				}
				extent, err := core.NewByteCount(size.Uint64())
				if err != nil {
					return err
				}
				asset, err := primitiverelease.InspectMetadataAsset(primitiverelease.MetadataInspectionRequest{Source: read.Source, Extent: extent, Kind: kind})
				if err != nil {
					return err
				}
				if asset.Kind() != kind || asset.Integrity().SHA256() != core.SHA256Of(body) || asset.Integrity().Extent() != extent {
					t.Fatalf("native release metadata=%+v,want actual source kind/digest/extent", asset)
				}
				return asset.Validate()
			}})
			if err != nil || scope.Validate() != nil || scope.OperationError() != nil || scope.CleanupError() != nil {
				t.Fatalf("native metadata loan=%+v/%v", scope, err)
			}
		})
	}
}
