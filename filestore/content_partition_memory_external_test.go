package filestore_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/testserial"
)

type partitionNativeHeapWriter struct {
	owner    context.Context
	expected uint64
	written  uint64
	retained uint64
	observed bool
}

func (w *partitionNativeHeapWriter) Write(data []byte) (int, error) {
	w.written += uint64(len(data))
	if w.written == w.expected {
		heap, err := hostfacts.ObserveCollectedGoHeap(w.owner)
		if err != nil {
			return 0, err
		}
		w.retained, w.observed = heap.Uint64(), true
	}
	return len(data), nil
}

func TestCopyPartitionNativeWorkingWindowDoesNotScaleWithPartitionSize(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	const allowance = 512 << 10
	for _, size := range [...]int64{(64 << 10) + 1, (64 << 20) + 1} {
		file, err := os.Create(filepath.Join(t.TempDir(), "native-partition-size"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := file.Close(); err != nil {
				t.Error(err)
			}
		})
		if err := file.Truncate(size); err != nil {
			t.Fatal(err)
		}
		source, err := filestore.NewContentPartitionSource(t.Context(), filestore.ContentPartitionSourceRequest{Source: file})
		if err != nil {
			t.Fatal(err)
		}
		maximum, err := core.NewByteCount(uint64(size))
		if err != nil {
			t.Fatal(err)
		}
		output := &partitionNativeHeapWriter{owner: t.Context(), expected: uint64(size)}
		before, err := hostfacts.ObserveCollectedGoHeap(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		observation, err := filestore.CopyPartition(t.Context(), filestore.CopyPartitionRequest{Source: source, Destination: output, Maximum: maximum})
		if err != nil {
			t.Fatal(err)
		}
		growth := int64(output.retained) - int64(before.Uint64())
		t.Logf("partition_bytes=%d retained_bytes=%d allowance=%d", size, growth, allowance)
		if observation.Validate() != nil || observation.Kind != filestore.ContentPartitionLast || observation.Content.Extent.Uint64() != uint64(size) || output.written != uint64(size) || !output.observed || growth > allowance {
			t.Fatalf("partition=%+v written=%d observed=%v growth=%d, want native complete fixed window", observation, output.written, output.observed, growth)
		}
	}
}
