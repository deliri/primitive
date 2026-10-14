package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestCopyPartitionConservesBytesAndExactTerminalBoundary(t *testing.T) {
	t.Parallel()
	for _, length := range [...]int{0, 1, 7, 8, 9, 16, 17} {
		data := bytes.Repeat([]byte{0xde}, length)
		source, err := filestore.NewContentPartitionSource(t.Context(), filestore.ContentPartitionSourceRequest{Source: iotest.OneByteReader(bytes.NewReader(data))})
		if err != nil {
			t.Fatal(err)
		}
		maximum, err := core.NewByteCount(8)
		if err != nil {
			t.Fatal(err)
		}
		for offset := 0; ; {
			var output bytes.Buffer
			observation, err := filestore.CopyPartition(t.Context(), filestore.CopyPartitionRequest{Source: source, Destination: &output, Maximum: maximum})
			if err != nil || observation.Validate() != nil {
				t.Fatalf("length=%d offset=%d observation=%+v err=%v", length, offset, observation, err)
			}
			if offset == length {
				if observation.Kind != filestore.ContentPartitionAbsent || output.Len() != 0 {
					t.Fatalf("empty=%+v bytes=%d", observation, output.Len())
				}
				break
			}
			end := min(offset+8, length)
			wantKind := filestore.ContentPartitionMore
			if end == length {
				wantKind = filestore.ContentPartitionLast
			}
			if observation.Kind != wantKind || observation.Content != contentCopyFixture(t, data[offset:end]) || !bytes.Equal(output.Bytes(), data[offset:end]) {
				t.Fatalf("offset=%d observation=%+v output=%x want=%x kind=%v", offset, observation, output.Bytes(), data[offset:end], wantKind)
			}
			offset = end
		}
	}
}

func TestCopyPartitionRefusalsWithdrawNativeObservation(t *testing.T) {
	t.Parallel()
	maximum, err := core.NewByteCount(8)
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range [...]int{7, 8, 15, 16} {
		source, err := filestore.NewContentPartitionSource(t.Context(), filestore.ContentPartitionSourceRequest{Source: io.MultiReader(bytes.NewReader(bytes.Repeat([]byte{'a'}, size)), iotest.ErrReader(io.ErrClosedPipe))})
		if err != nil {
			t.Fatal(err)
		}
		for {
			observation, err := filestore.CopyPartition(t.Context(), filestore.CopyPartitionRequest{Source: source, Destination: io.Discard, Maximum: maximum})
			if err != nil {
				if observation != (filestore.ContentPartitionObservation{}) || !errors.Is(err, io.ErrClosedPipe) || !errors.Is(err, core.ErrFilestoreSource) {
					t.Fatalf("refusal=%+v/%v", observation, err)
				}
				break
			}
			if observation.Kind != filestore.ContentPartitionMore {
				t.Fatalf("refused source admitted final=%+v", observation)
			}
		}
	}
	source, err := filestore.NewContentPartitionSource(t.Context(), filestore.ContentPartitionSourceRequest{Source: bytes.NewReader([]byte("present"))})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	observation, err := filestore.CopyPartition(ctx, filestore.CopyPartitionRequest{Source: source, Destination: io.Discard, Maximum: maximum})
	if observation != (filestore.ContentPartitionObservation{}) || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled=%+v/%v", observation, err)
	}
	observation, err = filestore.CopyPartition(t.Context(), filestore.CopyPartitionRequest{Destination: io.Discard, Maximum: maximum})
	if observation != (filestore.ContentPartitionObservation{}) || !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatalf("missing=%+v/%v", observation, err)
	}
}

func FuzzCopyPartitionMatchesIndependentByteSlices(f *testing.F) {
	f.Add([]byte{}, uint8(1))
	f.Add([]byte("exact boundary"), uint8(7))
	f.Add([]byte{0, 255, 10, 13}, uint8(255))
	f.Fuzz(func(t *testing.T, data []byte, window uint8) {
		if len(data) > 4096 {
			return
		} // Fuzz fixture work budget; production has no input extent quota.
		maximum, err := core.NewByteCount(uint64(window) + 1)
		if err != nil {
			t.Fatal(err)
		}
		source, err := filestore.NewContentPartitionSource(t.Context(), filestore.ContentPartitionSourceRequest{Source: bytes.NewReader(data)})
		if err != nil {
			t.Fatal(err)
		}
		for offset := 0; ; {
			var out bytes.Buffer
			observation, err := filestore.CopyPartition(t.Context(), filestore.CopyPartitionRequest{Source: source, Destination: &out, Maximum: maximum})
			if err != nil || observation.Validate() != nil {
				t.Fatalf("observation=%+v/%v", observation, err)
			}
			if offset == len(data) {
				if observation.Kind != filestore.ContentPartitionAbsent || out.Len() != 0 {
					t.Fatalf("exhaustion=%+v/%x", observation, out.Bytes())
				}
				break
			}
			end := min(offset+int(window)+1, len(data))
			want := filestore.ContentPartitionMore
			if end == len(data) {
				want = filestore.ContentPartitionLast
			}
			if observation.Kind != want || observation.Content != contentCopyFixture(t, data[offset:end]) || !bytes.Equal(out.Bytes(), data[offset:end]) {
				t.Fatalf("partition=%+v/%x want=%x", observation, out.Bytes(), data[offset:end])
			}
			offset = end
		}
	})
}

func TestCopyPartitionAdmissionAndClosedObservationDomain(t *testing.T) {
	t.Parallel()
	for _, ctx := range [...]context.Context{nil, t.Context()} {
		if source, err := filestore.NewContentPartitionSource(ctx, filestore.ContentPartitionSourceRequest{}); source != nil || !errors.Is(err, core.ErrFilestoreContract) {
			t.Fatalf("missing source=%v/%v", source, err)
		}
	}
	for value := 0; value <= 255; value++ {
		kind := filestore.ContentPartitionKind(value)
		want := kind == filestore.ContentPartitionAbsent || kind == filestore.ContentPartitionMore || kind == filestore.ContentPartitionLast
		if (kind.Validate() == nil) != want {
			t.Fatalf("kind=%d error=%v wantValid=%v", value, kind.Validate(), want)
		}
	}
	if (filestore.ContentPartitionObservation{Kind: filestore.ContentPartitionMore}).Validate() == nil {
		t.Fatal("nonempty observation admitted absent content")
	}
	source, err := filestore.NewContentPartitionSource(t.Context(), filestore.ContentPartitionSourceRequest{Source: bytes.NewReader([]byte("unconsumed"))})
	if err != nil {
		t.Fatal(err)
	}
	observation, err := filestore.CopyPartition(t.Context(), filestore.CopyPartitionRequest{Source: source, Destination: io.Discard})
	if observation != (filestore.ContentPartitionObservation{}) || !errors.Is(err, core.ErrFilestoreContract) {
		t.Fatalf("invalid window=%+v/%v", observation, err)
	}
	maximum, err := core.NewByteCount(32)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	observation, err = filestore.CopyPartition(t.Context(), filestore.CopyPartitionRequest{Source: source, Destination: &out, Maximum: maximum})
	if err != nil || observation.Kind != filestore.ContentPartitionLast || out.String() != "unconsumed" {
		t.Fatalf("refusal consumed source=%+v/%q/%v", observation, out.String(), err)
	}
}

func TestCopyPartitionBorrowsNativeFileThroughCleanEnd(t *testing.T) {
	t.Parallel()
	data := bytes.Repeat([]byte{0, 255, 10}, 10923)
	path := filepath.Join(t.TempDir(), "native-partition")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	})
	source, err := filestore.NewContentPartitionSource(t.Context(), filestore.ContentPartitionSourceRequest{Source: file})
	if err != nil {
		t.Fatal(err)
	}
	maximum, err := core.NewByteCount(8192)
	if err != nil {
		t.Fatal(err)
	}
	for offset := 0; offset < len(data); {
		var output bytes.Buffer
		observation, err := filestore.CopyPartition(t.Context(), filestore.CopyPartitionRequest{Source: source, Destination: &output, Maximum: maximum})
		if err != nil {
			t.Fatal(err)
		}
		end := min(offset+8192, len(data))
		if !bytes.Equal(output.Bytes(), data[offset:end]) || observation.Content != contentCopyFixture(t, data[offset:end]) || (observation.Kind == filestore.ContentPartitionLast) != (end == len(data)) {
			t.Fatalf("native boundary=%d/%+v", offset, observation)
		}
		offset = end
	}
	if _, err := file.Stat(); err != nil {
		t.Fatalf("borrowed native file closed: %v", err)
	}
}
