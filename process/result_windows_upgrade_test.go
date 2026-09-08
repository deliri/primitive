//go:build windows

package process_test

import (
	"bytes"
	"errors"
	"io"
	"math"
	"strconv"
	"syscall"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

// Retaining a second handle prevents process-object deletion and PID reuse
// after Wait. Exit 259 must still be gone: GetExitCodeProcess alone cannot
// distinguish that completed exit from its STILL_ACTIVE sentinel.
func TestWindowsReapedExitLivenessLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		exit int64
	}{
		{name: "neutral/successful silent exit", exit: core.ProcessExitCodeSuccess},
		{name: "positive/nonzero exit", exit: 1},
		{name: "boundary/below native still-active code", exit: 258},
		{name: "negative/completed still-active code cannot claim alive", exit: 259},
		{name: "boundary/above native still-active code", exit: 260},
		{name: "boundary/unsigned sign bit retains exact exit", exit: math.MaxInt32 + 1},
		{name: "boundary/maximum native exit remains unsigned", exit: core.ProcessExitCodeMaximum},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := processRequest(t, "exit:"+strconv.FormatInt(tc.exit, 10), process.Streams{Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard})
			execution, err := process.Begin(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			waited := false
			t.Cleanup(func() {
				if !waited {
					_, err := execution.Wait()
					if err != nil {
						t.Error(err)
					}
				}
			})
			identity, err := execution.Identity()
			if err != nil {
				t.Fatal(err)
			}
			handle, err := syscall.OpenProcess(syscall.SYNCHRONIZE, false, uint32(identity))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := syscall.CloseHandle(handle); err != nil {
					t.Error(err)
				}
			})
			result, err := execution.Wait()
			waited = true
			if err != nil {
				t.Fatal(err)
			}
			facts, factsErr := result.Observation()
			peak, peakErr := result.PeakMemoryBytes()
			if factsErr != nil || facts.ExitCode != tc.exit || facts.PeakMemoryBytes != nil || facts.StdinBytes.Uint64() != 0 || facts.StdoutBytes.Uint64() != 0 || facts.StderrBytes.Uint64() != 0 || !errors.Is(peakErr, core.ErrProcessUnsupported) || peak.Uint64() != 0 {
				t.Fatalf("exit facts=%+v, %v; peak=%v, %v; want exact exit %d, zero streams and unreported RSS", facts, factsErr, peak, peakErr, tc.exit)
			}
			liveness, err := process.Alive(identity)
			if err != nil || liveness != process.LivenessGone {
				t.Fatalf("retained completed process=%v, %v; want gone for exit %d", liveness, err, tc.exit)
			}
		})
	}
}
