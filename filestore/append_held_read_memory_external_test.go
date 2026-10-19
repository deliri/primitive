package filestore_test

import (
	"context"
	"runtime"
	"testing"

	"github.com/deliri/primitive/v2026/filestore"
)

func TestHeldAppendReadWorkingMemoryDoesNotFollowFileExtent(t *testing.T) {
	measure := func(size int) uint64 {
		owner, _, _ := heldReadOwner(t, filestore.AppendAccessReadWrite)
		if _, err := owner.Write(make([]byte, size)); err != nil {
			t.Fatal(err)
		}
		if err := owner.Sync(); err != nil {
			t.Fatal(err)
		}
		request := filestore.HeldAppendReadRequest{Use: func(_ context.Context, read filestore.InspectedRead) error {
			var one [1]byte
			_, err := read.Source.ReadAt(one[:], 0)
			return err
		}}
		if err := owner.WithHeldReadScope(t.Context(), request); err != nil {
			t.Fatal(err)
		}
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		for range 32 {
			if err := owner.WithHeldReadScope(t.Context(), request); err != nil {
				t.Fatal(err)
			}
		}
		runtime.ReadMemStats(&after)
		return (after.TotalAlloc - before.TotalAlloc) / 32
	}
	small, large := measure(64), measure(4<<20)
	t.Logf("native held-read allocation64byte/4MiB=%d/%d bytes", small, large)
	if large > small+64<<10 {
		t.Fatalf("held read retained file extent: small=%d large=%d", small, large)
	}
}
