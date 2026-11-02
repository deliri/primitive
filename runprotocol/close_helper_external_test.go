package runprotocol_test

import (
	"errors"
	"io"
	"io/fs"
	"os"
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

// mustAlreadyBeClosed proves a fixture closed the file itself, which is the
// precondition of a native-closed-source scenario. Any other outcome fails.
func mustAlreadyBeClosed(tb testing.TB, file *os.File) {
	tb.Helper()
	if err := file.Close(); !errors.Is(err, fs.ErrClosed) {
		tb.Errorf("Close() error = %v, want fs.ErrClosed from the fixture's own close", err)
	}
}
