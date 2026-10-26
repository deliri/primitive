//go:build linux

package process_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

// A shell holding ~300 MB is killed under a 64 MiB MemoryMax and finishes
// under 1 GiB, started as a group the way Anvil starts its jobs.
func TestMemoryMaxKillsOnlyTheOverLimitTree(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name     string
		max      uint64
		wantPass bool
	}{
		{name: "300 MB under a 64 MiB limit is killed", max: 64 << 20},
		{name: "300 MB under a 1 GiB limit finishes", max: 1 << 30, wantPass: true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			limit, err := core.NewByteLength(row.max)
			if err != nil {
				t.Fatal(err)
			}
			var stderr bytes.Buffer
			request := processRequest(t, "copy", process.Streams{Stdin: strings.NewReader(""), Stdout: io.Discard, Stderr: &stderr})
			request.Command = absolutePath(t, "/bin/sh")
			request.Arguments = arguments(t, "-c", "head -c 300000000 /dev/zero | tail -c 300000000 > /dev/null")
			// The group containment Anvil uses: Begin, then Wait and Sweep.
			request.Containment = process.Containment{Isolation: process.IsolationGroup, CancelSignal: process.CancelSignalKill, MemoryMax: limit}
			execution, err := process.Begin(t.Context(), request)
			if err != nil {
				t.Fatalf("Begin=%v (stderr %q), want a started tree", err, stderr.String())
			}
			result, err := execution.Wait()
			if sweepErr := execution.Sweep(); sweepErr != nil {
				t.Fatalf("Sweep=%v, want the group gone", sweepErr)
			}
			if err != nil {
				t.Fatalf("Wait=%v (stderr %q), want the native result", err, stderr.String())
			}
			code, err := result.ExitCode()
			if err != nil {
				t.Fatal(err)
			}
			success, err := code.Success()
			if err != nil || success != row.wantPass {
				t.Fatalf("exit %+v success=%v (%v) stderr %q, want success=%v", code, success, err, stderr.String(), row.wantPass)
			}
			if row.wantPass {
				peak, err := result.PeakMemoryBytes()
				if err != nil || peak.Uint64() < 250<<20 {
					t.Fatalf("peak %v (%v), want the shell's ~300 MB really held", peak, err)
				}
			}
		})
	}
}
