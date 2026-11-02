package process

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/testserial"
)

func tailWindow(t testing.TB, size uint64) core.ByteCount {
	t.Helper()
	window, err := core.NewByteCount(size)
	if err != nil {
		t.Fatal(err)
	}
	return window
}

func TestTailBufferConservesOrderedSuffixAcrossChunkBoundaries(t *testing.T) {
	t.Parallel()
	for _, window := range []int{1, 3, 8, 31} {
		t.Run(fmt.Sprint(window), func(t *testing.T) {
			t.Parallel()
			b, err := NewTailBuffer(tailWindow(t, uint64(window)))
			if err != nil {
				t.Fatal(err)
			}
			var source []byte
			for _, size := range []int{0, 1, window - 1, 1, window + 7, 2, 3 * window, 1} {
				chunk := make([]byte, size)
				for index := range chunk {
					chunk[index] = byte(len(source) + index)
				}
				source = append(source, chunk...)
				n, err := b.Write(chunk)
				if err != nil || n != len(chunk) {
					t.Fatalf("Write consumed %d/%d: %v", n, len(chunk), err)
				}
				clear(chunk) // The producer retains and may reuse its own bytes.
				got, err := b.Snapshot()
				want := source[max(0, len(source)-window):]
				if err != nil || !bytes.Equal(got.Data, want) ||
					got.Received.Uint64() != uint64(len(source)) ||
					got.Dropped.Uint64() != uint64(len(source)-len(want)) {
					t.Fatalf("snapshot=%+v,%v; want suffix %x of %x", got, err, want, source)
				}
				if err := got.Validate(); err != nil {
					t.Fatal(err)
				}
				clear(got.Data) // An observer cannot modify the native window.
			}
		})
	}
}

func TestTailBufferResetAndSnapshotHaveIndependentOwnership(t *testing.T) {
	t.Parallel()
	b, err := NewTailBuffer(tailWindow(t, 4))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Write([]byte("secret")); err != nil {
		t.Fatal(err)
	}
	before, err := b.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Reset(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b.storage, make([]byte, 4)) {
		t.Fatalf("Reset retained old private bytes: %x", b.storage)
	}
	empty, err := b.Snapshot()
	if err != nil || len(empty.Data) != 0 || empty.Received.Uint64() != 0 || empty.Dropped.Uint64() != 0 {
		t.Fatalf("reset observation=%+v,%v", empty, err)
	}
	if _, err := b.Write([]byte("new")); err != nil {
		t.Fatal(err)
	}
	after, err := b.Snapshot()
	if err != nil || string(after.Data) != "new" || after.Received.Uint64() != 3 || after.Dropped.Uint64() != 0 {
		t.Fatalf("new source observation=%+v,%v", after, err)
	}
	if string(before.Data) != "cret" || before.Received.Uint64() != 6 || before.Dropped.Uint64() != 2 {
		t.Fatalf("later activity changed earlier observation: %+v", before)
	}
}

func TestTailBufferRefusesUnsetAndCorruptedCapabilities(t *testing.T) {
	t.Parallel()
	if b, err := NewTailBuffer(core.ByteCount{}); b != nil || !errors.Is(err, core.ErrProcessContract) || !errors.Is(err, core.ErrPrimitiveContract) {
		t.Fatalf("unset window=%v,%v", b, err)
	}
	if b, err := NewTailBuffer(tailWindow(t, uint64(math.MaxInt)+1)); b != nil || !errors.Is(err, core.ErrNumericOverflow) {
		t.Fatalf("unrepresentable native window=%v,%v", b, err)
	}
	for _, tc := range []struct {
		name string
		b    *TailBuffer
	}{
		{name: "nil"},
		{name: "unset", b: &TailBuffer{}},
		{name: "negative_cursor", b: &TailBuffer{storage: make([]byte, 3), cursor: -1}},
		{name: "cursor_outside_window", b: &TailBuffer{storage: make([]byte, 3), cursor: 3}},
		{name: "partial_cursor_disagrees", b: &TailBuffer{storage: make([]byte, 3), received: 1}},
		{name: "unrepresentable_received", b: &TailBuffer{storage: make([]byte, 3), received: math.MaxUint64}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.b.Validate(); !errors.Is(err, core.ErrProcessContract) {
				t.Fatalf("Validate=%v", err)
			}
			if n, err := tc.b.Write([]byte("x")); n != 0 || !errors.Is(err, core.ErrProcessContract) {
				t.Fatalf("Write=%d,%v", n, err)
			}
			if got, err := tc.b.Snapshot(); len(got.Data) != 0 || !errors.Is(err, core.ErrProcessContract) {
				t.Fatalf("Snapshot=%+v,%v", got, err)
			}
			if err := tc.b.Reset(); !errors.Is(err, core.ErrProcessContract) {
				t.Fatalf("Reset=%v", err)
			}
		})
	}
}

func TestTailBufferNumericOverflowIsAtomic(t *testing.T) {
	t.Parallel()
	b, err := NewTailBuffer(tailWindow(t, 3))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Write([]byte("old")); err != nil {
		t.Fatal(err)
	}
	// Simulate the native representability boundary; no enormous source is claimed.
	b.received = math.MaxInt64 - 1
	before, err := b.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if n, err := b.Write([]byte("no")); n != 0 || !errors.Is(err, core.ErrNumericOverflow) || !errors.Is(err, core.ErrProcessContract) {
		t.Fatalf("overflow consumed=%d,error=%v", n, err)
	}
	after, err := b.Snapshot()
	if err != nil || !bytes.Equal(before.Data, after.Data) || before.Received != after.Received || before.Dropped != after.Dropped {
		t.Fatalf("overflow changed native observation: before=%+v,after=%+v,%v", before, after, err)
	}
	if n, err := b.Write([]byte("!")); n != 1 || err != nil {
		t.Fatalf("last representable byte=%d,%v", n, err)
	}
	last, err := b.Snapshot()
	if err != nil || string(last.Data) != "ld!" || last.Received.Uint64() != math.MaxInt64 || last.Dropped.Uint64() != math.MaxInt64-3 {
		t.Fatalf("last byte observation=%+v,%v", last, err)
	}
	if n, err := b.Write(nil); n != 0 || err != nil {
		t.Fatalf("empty neutral write=%d,%v", n, err)
	}
}

func TestTailBufferObservationConservationLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name              string
		data              string
		received, dropped uint64
		refused           bool
	}{
		{name: "positive_suffix", data: "bc", received: 3, dropped: 1},
		{name: "negative_missing_byte", data: "bc", received: 4, dropped: 1, refused: true},
		{name: "negative_invented_byte", data: "bc", received: 2, dropped: 1, refused: true},
		{name: "negative_drop_exceeds_source", received: 1, dropped: 2, refused: true},
		{name: "neutral_empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			received, err := core.NewByteLength(tc.received)
			if err != nil {
				t.Fatal(err)
			}
			dropped, err := core.NewByteLength(tc.dropped)
			if err != nil {
				t.Fatal(err)
			}
			err = (TailBufferObservation{Data: []byte(tc.data), Received: received, Dropped: dropped}).Validate()
			if (err != nil) != tc.refused || (tc.refused && !errors.Is(err, core.ErrProcessContract)) {
				t.Fatalf("observation admission=%v,want refusal=%t", err, tc.refused)
			}
		})
	}
}

func TestTailBufferConcurrentWritesAndSnapshotsAreAtomic(t *testing.T) {
	t.Parallel()
	b, err := NewTailBuffer(tailWindow(t, 32))
	if err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	failures := make(chan error, 5)
	for writer := range 4 {
		workers.Go(func() {
			chunk := bytes.Repeat([]byte{byte(writer + 1)}, 32)
			for range 200 {
				if n, err := b.Write(chunk); n != len(chunk) || err != nil {
					failures <- fmt.Errorf("writer consumed %d: %w", n, err)
					return
				}
			}
		})
	}
	workers.Go(func() {
		for range 800 {
			got, err := b.Snapshot()
			if err != nil {
				failures <- err
				return
			}
			if err := got.Validate(); err != nil {
				failures <- err
				return
			}
			for _, value := range got.Data {
				if value != got.Data[0] {
					failures <- errors.New("torn native write")
					return
				}
			}
		}
	})
	done := make(chan struct{})
	go func() { workers.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("native writer/snapshot finished=false after %v, want finished", 5*time.Second)
	}
	close(failures)
	for failure := range failures {
		t.Error(failure)
	}
	got, err := b.Snapshot()
	if err != nil || got.Received.Uint64() != 4*200*32 || got.Dropped.Uint64() != 4*200*32-32 || len(got.Data) != 32 {
		t.Fatalf("concurrent byte conservation=%+v,%v", got, err)
	}
}

func TestTailBufferRetainsWindowInsteadOfSourceExtent(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, extent := range []int{64 << 10, 8 << 20} {
		t.Run(fmt.Sprint(extent), func(t *testing.T) {
			b, err := NewTailBuffer(tailWindow(t, 4096))
			if err != nil {
				t.Fatal(err)
			}
			chunk := bytes.Repeat([]byte{0xa5}, 64<<10)
			runtime.GC()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			for remaining := extent; remaining > 0; remaining -= len(chunk) {
				if n, err := b.Write(chunk); n != len(chunk) || err != nil {
					t.Fatalf("Write=%d,%v", n, err)
				}
			}
			runtime.GC()
			runtime.ReadMemStats(&after)
			retained := int64(after.HeapAlloc) - int64(before.HeapAlloc)
			t.Logf("actual source bytes=%d, window=4096, retained heap delta=%d", extent, retained)
			got, err := b.Snapshot()
			if err != nil || !bytes.Equal(got.Data, chunk[:4096]) || got.Received.Uint64() != uint64(extent) || got.Dropped.Uint64() != uint64(extent-4096) {
				t.Fatalf("streaming projection=%+v,%v", got, err)
			}
			runtime.KeepAlive(b)
			runtime.KeepAlive(chunk)
			if retained > 256<<10 {
				t.Fatalf("source extent retained %d bytes beyond fixed window", retained)
			}
		})
	}
}
