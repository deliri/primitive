package machineprobe_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/machineprobe"
)

func BenchmarkFailureKindUnmarshalJSON(b *testing.B) {
	data := []byte(`"probe_exit"`)
	var wantErr error
	b.ReportAllocs()
	var last machineprobe.FailureKind
	for b.Loop() {
		var got machineprobe.FailureKind
		err := got.UnmarshalJSON(data)
		if !errors.Is(err, wantErr) {
			b.Fatalf("machineprobe.FailureKind.UnmarshalJSON() error = %v, want %v", err, wantErr)
		}
		last = got
	}
	if last != machineprobe.FailureExit {
		b.Fatalf("machineprobe.FailureKind.UnmarshalJSON() = %v, want %v", last, machineprobe.FailureExit)
	}
}
