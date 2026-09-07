package sourceproof_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/sourceproof"
)

func BenchmarkStateUnmarshalJSON(b *testing.B) {
	data := []byte(`"proven"`)
	var wantErr error
	b.ReportAllocs()
	var last sourceproof.State
	for b.Loop() {
		var got sourceproof.State
		err := got.UnmarshalJSON(data)
		if !errors.Is(err, wantErr) {
			b.Fatalf("sourceproof.State.UnmarshalJSON() error = %v, want %v", err, wantErr)
		}
		last = got
	}
	if last != sourceproof.StateProven {
		b.Fatalf("sourceproof.State.UnmarshalJSON() = %v, want %v", last, sourceproof.StateProven)
	}
}
