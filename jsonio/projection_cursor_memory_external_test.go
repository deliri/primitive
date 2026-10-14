package jsonio_test

import (
	"io"
	"runtime"
	"testing"

	"github.com/deliri/primitive/v2026/jsonio"
)

type repeatingProjectionReader struct {
	remaining, ordinal int
	pending            string
	heap               uint64
}

func (r *repeatingProjectionReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if r.pending == "" {
		switch {
		case r.ordinal == 0:
			r.pending = "["
			r.ordinal++
		case r.remaining > 0:
			if r.ordinal > 1 {
				r.pending = ","
			}
			r.pending += `{"z":9,"a":"x"}`
			r.remaining--
			r.ordinal++
		case r.ordinal > 0:
			r.pending = "]"
			r.ordinal = -1
		default:
			runtime.GC()
			var memory runtime.MemStats
			runtime.ReadMemStats(&memory)
			r.heap = memory.HeapAlloc
			return 0, io.EOF
		}
	}
	n := copy(p, r.pending)
	r.pending = r.pending[n:]
	return n, nil
}

func TestProjectionCursorRetainedMemoryIndependentOfArrayPopulation(t *testing.T) {
	measure := func(population int) uint64 {
		source := &repeatingProjectionReader{remaining: population}
		cursor, err := jsonio.NewTokenDecoder(jsonio.TokenRequest{Source: source, NestingDepthMaximum: 64})
		if err != nil {
			t.Fatal(err)
		}
		if token, err := cursor.Decode(t.Context()); err != nil || token.Kind != jsonio.TokenArrayStart {
			t.Fatalf("start=%v %v", token, err)
		}
		count := 0
		sum := uint64(0)
		for {
			kind, err := cursor.PeekKind(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if kind == jsonio.TokenArrayEnd {
				break
			}
			record, err := jsonio.DecodeProjectionAt[canonicalProjection](t.Context(), cursor, jsonio.MembersDeclaredOnly)
			if err != nil {
				t.Fatal(err)
			}
			if record.A != "x" || record.Z != 9 {
				t.Fatalf("record=%+v", record)
			}
			count++
			sum += record.Z
		}
		if _, err := cursor.Decode(t.Context()); err != nil {
			t.Fatal(err)
		}
		if err := cursor.RequireEOF(t.Context()); err != nil {
			t.Fatal(err)
		}
		if count != population || sum != uint64(population)*9 {
			t.Fatalf("count=%d sum=%d population=%d", count, sum, population)
		}
		runtime.KeepAlive(cursor)
		return source.heap
	}
	small := measure(128)
	large := measure(65536)
	t.Logf("live decoder EOF heap: 128 records=%d bytes; 65536 records=%d bytes", small, large)
	if large > small && large-small > 512<<10 {
		t.Fatalf("retained population growth=%d bytes", large-small)
	}
}
