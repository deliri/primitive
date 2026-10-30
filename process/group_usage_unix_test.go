//go:build unix

package process_test

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

const groupUsageBurnMarker = "--primitive-group-usage-burn"

// TestGroupUsageBurnHelper spins on CPU until its stdin closes.
func TestGroupUsageBurnHelper(t *testing.T) {
	t.Parallel()
	if !slices.Contains(os.Args, groupUsageBurnMarker) {
		return
	}
	var stopped atomic.Bool
	go func() {
		_, _ = io.Copy(io.Discard, os.Stdin)
		stopped.Store(true)
	}()
	if _, err := io.WriteString(os.Stdout, "burning\n"); err != nil {
		t.Fatal(err)
	}
	// The spin is the workload: the parent measures this group's CPU time.
	for !stopped.Load() {
	}
}

// The native ps on this host reports a burning group's CPU time rising
// between two samples while its resident memory stays visible, then zero once
// the group is reaped, and refuses a cancelled context.
func TestGroupUsageNativeCPUTimeRises(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), processTestBackstop)
	defer cancel()
	in, send := io.Pipe()
	out, receive := io.Pipe()
	defer in.Close()
	defer send.Close()
	defer out.Close()
	defer receive.Close()
	stopOutput := context.AfterFunc(ctx, func() { _ = out.Close() })
	defer stopOutput()
	request := processRequest(t, "copy", process.Streams{Stdin: in, Stdout: receive, Stderr: io.Discard})
	request.Arguments = arguments(t, "-test.run=^TestGroupUsageBurnHelper$", "--", groupUsageBurnMarker)
	request.Containment.Isolation = process.IsolationGroup
	execution, err := process.Begin(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	waited := false
	defer func() {
		if !waited {
			cancel()
			_ = send.Close()
			if err := execution.Sweep(); err != nil {
				t.Error(err)
			}
			_, _ = execution.Wait()
		}
	}()
	reader := bufio.NewReader(out)
	if line, err := reader.ReadString('\n'); err != nil || line != "burning\n" {
		t.Fatalf("helper readiness=(%q,%v)", line, err)
	}
	// The helper's later test-runner output must not block its exit.
	go func() { _, _ = io.Copy(io.Discard, reader) }()
	identity, err := execution.Identity()
	if err != nil {
		t.Fatal(err)
	}
	usageRequest := process.GroupMemoryRequest{Group: identity, WorkingDirectory: request.WorkingDirectory, Environment: request.Environment}
	first, err := process.ObserveGroupUsage(ctx, usageRequest)
	if err != nil || first.Resident.Uint64() == 0 {
		t.Fatalf("first usage=(%v,%v), want positive native RSS", first, err)
	}
	// Linux procps reports whole seconds, so a rise can take over a second.
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("CPU time stayed at %dns until %s backstop", first.CPUTime.Nanoseconds(), processTestBackstop)
		case <-ticker.C:
		}
		next, err := process.ObserveGroupUsage(ctx, usageRequest)
		if err != nil || next.Resident.Uint64() == 0 {
			t.Fatalf("next usage=(%v,%v), want positive native RSS", next, err)
		}
		if next.CPUTime.Nanoseconds() > first.CPUTime.Nanoseconds() {
			break
		}
	}
	if err = send.Close(); err != nil {
		t.Fatal(err)
	}
	result, err := execution.Wait()
	waited = true
	if err != nil {
		t.Fatal(err)
	}
	if code, err := result.ExitCode(); err != nil {
		t.Fatal(err)
	} else if success, err := code.Success(); err != nil || !success {
		t.Fatalf("burn helper exit=(%v,%v), want success", code, err)
	}
	gone, err := process.ObserveGroupUsage(ctx, usageRequest)
	if err != nil || gone != (process.GroupUsage{}) {
		t.Fatalf("reaped usage=(%v,%v), want zero", gone, err)
	}
	closed, cancelClosed := context.WithCancel(ctx)
	cancelClosed()
	if got, err := process.ObserveGroupUsage(closed, usageRequest); !errors.Is(err, context.Canceled) || got != (process.GroupUsage{}) {
		t.Fatalf("cancelled usage=(%v,%v), want zero cancellation", got, err)
	}
	if _, err := process.ObserveGroupUsage(ctx, process.GroupMemoryRequest{}); !errors.Is(err, core.ErrProcessContract) {
		t.Fatalf("zero request=%v, want contract refusal", err)
	}
}
