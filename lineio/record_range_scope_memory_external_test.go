package lineio_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/lineio"
)

func withGeneratedRecordRange(t testing.TB, extent int64, use func(lineio.RecordFragmentRequest)) {
	t.Helper()
	directory, err := core.ParseAbsolutePath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root, err := filestore.OpenRoot(t.Context(), directory)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	}()
	path, err := core.ParseRelativePath("range-source.txt")
	if err != nil {
		t.Fatal(err)
	}
	window, err := core.NewByteCount(65536)
	if err != nil {
		t.Fatal(err)
	}
	scope, err := filestore.WithScratchReplayScope(t.Context(), filestore.ScratchReplayScopeRequest{Scratch: filestore.ScratchRequest{Location: filestore.Location{Root: root, Path: path}, Mode: 0600}, Use: func(ctx context.Context, source filestore.ScratchReplayFile) error {
		copied, err := filestore.CopyContent(ctx, filestore.CopyContentRequest{Source: io.MultiReader(&generatedRecordBody{remaining: extent}, strings.NewReader("\n")), Destination: source})
		if err != nil {
			return err
		}
		if copied.Bytes.Uint64() != uint64(extent+1) {
			return io.ErrShortWrite
		}
		use(lineio.RecordFragmentRequest{Source: source, Record: lineio.RecordRange{Bytes: copied.Bytes, Framing: lineio.RecordFramingLF}, BufferBytes: window})
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(scope.Validate(), scope.OperationError(), scope.CleanupError()); err != nil {
		t.Fatal(err)
	}
}

func TestRecordRangeScopeNativeFileHasNoFormerRecordCeiling(t *testing.T) {
	t.Parallel()
	const extent = int64(32<<20) + 1
	withGeneratedRecordRange(t, extent, func(request lineio.RecordFragmentRequest) {
		var got filestore.ContentIndexEntry
		err := lineio.WithRecordRange(t.Context(), lineio.RecordRangeReadRequest{Range: request, Use: func(ctx context.Context, r io.Reader) error {
			var err error
			got, err = filestore.CopyContent(ctx, filestore.CopyContentRequest{Source: r, Destination: io.Discard})
			return err
		}})
		if err != nil {
			t.Fatal(err)
		}
		want := sha256.New()
		var buffer [32768]byte
		for index := range buffer {
			buffer[index] = 'x'
		}
		for remaining := extent; remaining > 0; {
			count := min(remaining, int64(len(buffer)))
			_, _ = want.Write(buffer[:count])
			remaining -= count
		}
		_, _ = want.Write([]byte{'\n'})
		digest, err := got.Digest.Bytes()
		if err != nil || got.Bytes.Uint64() != uint64(extent+1) || !bytes.Equal(digest[:], want.Sum(nil)) {
			t.Fatalf("large selected range = (%d bytes,%x,%v), want complete independent native-file digest", got.Bytes.Uint64(), digest, err)
		}
	})
}

func BenchmarkRecordRangeScopeFixedWorkingMemory(b *testing.B) {
	for _, extent := range []int64{1024, 65537, (32 << 20) + 1} {
		b.Run(fmt.Sprintf("bytes_%d", extent), func(b *testing.B) {
			withGeneratedRecordRange(b, extent, func(request lineio.RecordFragmentRequest) {
				b.SetBytes(extent + 1)
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					var consumed int64
					err := lineio.WithRecordRange(b.Context(), lineio.RecordRangeReadRequest{Range: request, Use: func(_ context.Context, r io.Reader) error {
						var err error
						consumed, err = io.Copy(io.Discard, r)
						return err
					}})
					if err != nil || consumed != extent+1 {
						b.Fatalf("range = (%d,%v), want complete native extent %d", consumed, err, extent+1)
					}
				}
			})
		})
	}
}
