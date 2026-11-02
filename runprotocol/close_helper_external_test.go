package runprotocol_test

import (
	"io"
	"testing"
)

// mustClose records a close failure without hiding the caller's own failure:
// it reports with Errorf, so a Fatal that follows still names the first cause.
func mustClose(tb testing.TB, closer io.Closer) {
	tb.Helper()
	if err := closer.Close(); err != nil {
		tb.Errorf("Close() error = %v, want nil", err)
	}
}
