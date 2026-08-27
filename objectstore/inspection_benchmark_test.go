package objectstore_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
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
	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	b.ResetTimer()

	var got objectstore.Inspection
	var gotErr error
	for b.Loop() {
		got, gotErr = objectstore.Inspect(context.Background(), objectstore.InspectionRequest{
			Source: bytes.NewReader(payload), MaximumBytes: maximum,
		})
	}
	if gotErr != nil || got.Validate() != nil {
		b.Fatalf("objectstore.Inspect(%d bytes) = (%+v, %v), want validated and nil", size, got, gotErr)
	}
}
