package filestore

import (
	"errors"
	"fmt"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestRemovalDurabilityExhaustsClosedByteDomain(t *testing.T) {
	t.Parallel()
	for value := range 256 {
		t.Run(fmt.Sprintf("wire-byte-%03d", value), func(t *testing.T) {
			t.Parallel()
			got := RemovalDurability(value)
			wantValid := value == 1 || value == 2
			wantText := "invalid"
			switch value {
			case 0:
				wantText = "unknown"
			case 1:
				wantText = "ephemeral"
			case 2:
				wantText = "durable"
			}
			err := got.Validate()
			if got.IsValid() != wantValid || (err == nil) != wantValid || got.String() != wantText || !wantValid && !errors.Is(err, core.ErrFilestoreContract) {
				t.Fatalf("durability %d = (%t,%s,%v), want (%t,%s) with typed refusal", value, got.IsValid(), got.String(), err, wantValid, wantText)
			}
		})
	}
}

var _ core.OffWireEnum = RemovalDurabilityUnknown
