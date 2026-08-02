package receipt

import (
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func mustByteLength(testingContext testing.TB, value uint64) core.ByteLength {
	testingContext.Helper()
	length, err := core.NewByteLength(value)
	if err != nil {
		testingContext.Fatalf("core.NewByteLength(%d) error = %v, want nil", value, err)
	}
	return length
}
