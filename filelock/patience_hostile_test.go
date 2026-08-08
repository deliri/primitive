package filelock_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filelock"
)

// TestPatienceClosesItsEntireByteDomain walks every backing value: the two
// published patiences must validate and carry unique diagnostic labels, all
// two hundred fifty four others must refuse and render the shared unknown
// diagnostic, and the off-wire declaration is exercised rather than merely
// asserted, so the marker cannot rot into an unreachable ceremony.
func TestPatienceClosesItsEntireByteDomain(t *testing.T) {
	t.Parallel()

	var offWire core.OffWireEnum = filelock.Immediate
	offWire.OffWireEnum()

	seen := map[string]filelock.Patience{}
	admitted := 0
	for value := 0; value <= 255; value++ {
		patience := filelock.Patience(value)
		if err := patience.Validate(); err != nil {
			if !errors.Is(err, core.ErrPrimitiveContract) {
				t.Fatalf("Patience(%d).Validate() error = %v, want errors.Is %v", value, err, core.ErrPrimitiveContract)
			}
			if patience.IsValid() {
				t.Fatalf("Patience(%d).IsValid() = true beside a Validate refusal", value)
			}
			if got := patience.String(); got != core.UnknownEnumDiagnostic {
				t.Fatalf("Patience(%d).String() = %q, want %q", value, got, core.UnknownEnumDiagnostic)
			}
			continue
		}
		admitted++
		label := patience.String()
		if label == "" || label == core.UnknownEnumDiagnostic {
			t.Fatalf("Patience(%d).String() = %q, want a member's own label", value, label)
		}
		if prior, duplicate := seen[label]; duplicate {
			t.Fatalf("Patience(%d) and Patience(%d) share the label %q", value, prior, label)
		}
		seen[label] = patience
	}
	if admitted != 2 {
		t.Fatalf("admitted patiences = %d, want exactly immediate and blocking", admitted)
	}
}
