package testserial

import (
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// Declare validates one compiler-owned test-isolation declaration.
//
// The pinned analyzer requires this exact call as the first statement in a
// deliberately non-parallel test or subtest. Declare itself does not change
// Go test scheduling.
func Declare(t *testing.T, declaration core.TestIsolationDeclaration) {
	t.Helper()
	if err := declaration.Validate(); err != nil {
		t.Fatal(err)
	}
}

var _ func(*testing.T, core.TestIsolationDeclaration) = Declare
