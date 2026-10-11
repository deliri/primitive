package filestore_test

import (
	"os"
	"runtime/debug"
	"testing"
)

// The retained namespace fixture releases 10,000 independent writers together.
// Go's default 10,000-thread ceiling leaves no room for its own runtime threads
// when native HDD operations block. This process-only test budget preserves all
// writers; it changes neither production scheduling nor input extent policy.
func TestMain(m *testing.M) {
	debug.SetMaxThreads(20_000)
	os.Exit(m.Run())
}
