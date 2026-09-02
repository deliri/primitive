package hostfacts

import (
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func mustAbsolutePathForHostfactsTest(t *testing.T, value string) core.AbsolutePath {
	t.Helper()
	path, err := core.ParseAbsolutePath(value)
	if err != nil {
		t.Fatalf("core.ParseAbsolutePath(%s) error = %v, want nil", value, err)
	}
	return path
}
