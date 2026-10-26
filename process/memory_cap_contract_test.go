package process_test

import (
	"errors"
	"io"
	"strings"
	"runtime"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

// A limit alone is a half-named containment, never silently defaulted away;
// a platform without per-tree limits refuses one instead of ignoring it.
func TestMemoryMaxContract(t *testing.T) {
	t.Parallel()
	limit, err := core.NewByteLength(64 << 20)
	if err != nil {
		t.Fatal(err)
	}
	request := processRequest(t, "copy", process.Streams{Stdin: strings.NewReader(""), Stdout: io.Discard, Stderr: io.Discard})
	request.Containment = process.Containment{MemoryMax: limit}
	if _, err := process.Run(t.Context(), request); !errors.Is(err, core.ErrProcessContract) {
		t.Fatalf("limit-only containment Run=%v, want contract refusal", err)
	}
	if runtime.GOOS == "linux" {
		return
	}
	request.Containment = process.Containment{Isolation: process.IsolationDirect, CancelSignal: process.CancelSignalKill, MemoryMax: limit}
	if _, err := process.Run(t.Context(), request); !errors.Is(err, core.ErrProcessUnsupported) {
		t.Fatalf("%s limit Run=%v, want unsupported refusal", runtime.GOOS, err)
	}
}
