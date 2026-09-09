package objectstore_test

import (
	"bytes"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/zeebo/blake3"
)

func BenchmarkInspectThreeDigestsAcrossStreamExtents(b *testing.B) {
	b.ReportAllocs()

	cases := []struct {
		name string
		size int
	}{
		{name: "one_kibibyte", size: 1 << 10},
		{name: "one_mebibyte", size: 1 << 20},
		{name: "sixteen_mebibytes", size: 16 << 20},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			benchmarkInspectThreeDigests(b, tc.size)
		})
	}
}

func benchmarkInspectThreeDigests(b *testing.B, size int) {
	b.Helper()

	payload := bytes.Repeat([]byte{0xa5}, size)
	maximum, err := core.NewByteCount(uint64(len(payload)))
	if err != nil {
		b.Fatalf("core.NewByteCount(%d) setup error = %v, want nil", size, err)
	}
	want := objectstore.Inspection{Integrity: integrity(b, payload), BLAKE3: objectstore.NewBLAKE3Digest(blake3.Sum256(payload))}
	if err := want.Validate(); err != nil {
		b.Fatalf("inspection workload error = %v, want nil", err)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	for b.Loop() {
		got, gotErr := objectstore.Inspect(b.Context(), objectstore.InspectionRequest{
			Source: bytes.NewReader(payload), MaximumBytes: maximum,
		})
		if gotErr != nil || got != want {
			b.Fatalf("Inspect(%d bytes) = (%+v, %v), want (%+v, nil)", size, got, gotErr, want)
		}
	}
}
