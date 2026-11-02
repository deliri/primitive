package hostfacts

import (
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/temporal"
)

// A real user-manager scope running a CPU-bound native command must show a
// cgroup CPU sample inside the host window, bounded by its 100% CPU quota and
// by host busy CPU. A never-started scope is absent with zero usage.
func TestResourceUsageScopeCPUNativeLifetime(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	self, err := process.Self()
	if err != nil {
		t.Fatal(err)
	}
	pid, err := self.Int()
	if err != nil {
		t.Fatal(err)
	}
	busyName, err := core.ParsePathComponent("primitive-cpu-" + strconv.Itoa(pid) + systemdScopeSuffix)
	if err != nil {
		t.Fatal(err)
	}
	absentName, err := core.ParsePathComponent("primitive-cpu-absent-" + strconv.Itoa(pid) + systemdScopeSuffix)
	if err != nil {
		t.Fatal(err)
	}
	busy := SystemdScopeRequest{Unit: busyName, UserManager: true}
	absent := SystemdScopeRequest{Unit: absentName, UserManager: true}
	tool, err := core.ParsePathComponent("systemd-run")
	if err != nil {
		t.Fatal(err)
	}
	command, err := process.Resolve(ctx, tool)
	if err != nil {
		t.Fatal(err)
	}
	// sha256sum never reaches the end of /dev/zero: one CPU-bound native thread.
	args, err := process.ParseArguments([]string{"--user", "--scope", "--quiet", "--collect", "--slice=proof.slice", "--unit=" + busyName.String(), "--property=MemoryMax=33554432", "--property=MemorySwapMax=0", "--property=CPUQuota=100%", "--", "/usr/bin/sha256sum", "/dev/zero"})
	if err != nil {
		t.Fatal(err)
	}
	dir, err := WorkingDirectory()
	if err != nil {
		t.Fatal(err)
	}
	env, err := AmbientEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	wait, err := temporal.DurationFromSeconds(3)
	if err != nil {
		t.Fatal(err)
	}
	execution, err := process.Begin(ctx, process.Request{Command: command, Arguments: args, WorkingDirectory: dir, Environment: env, Streams: process.Streams{Stdin: strings.NewReader(""), Stdout: io.Discard, Stderr: io.Discard}, OutputPolicy: process.OutputPolicy{Mode: process.OutputModeStreaming}, WaitDelay: wait, Containment: process.Containment{Isolation: process.IsolationGroup, CancelSignal: process.CancelSignalKill}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cleanupCancel()
		presence, probeErr := ObserveSystemdScope(cleanup, busy)
		if probeErr != nil {
			t.Error(probeErr)
		}
		if presence == process.LivenessAlive {
			if _, err := resourceCommand(cleanup, "systemctl", "--user", "stop", "--", busyName.String()); err != nil {
				t.Error(err)
			}
		}
		if err := execution.Terminate(); err != nil {
			t.Error(err)
		}
		if _, err := execution.Wait(); err != nil && !errors.Is(err, context.Canceled) {
			t.Error(err)
		}
	}()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		present, err := ObserveSystemdScope(ctx, busy)
		if err != nil {
			t.Fatal(err)
		}
		if present == process.LivenessAlive {
			break
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	usage, err := ObserveResourceUsage(ctx, ResourceUsageRequest{Interval: resourceDuration(t, "1s"), Scopes: []SystemdScopeRequest{busy, absent}})
	if err != nil {
		t.Fatal(err)
	}
	scopes := usage.Scopes()
	if len(scopes) != 2 || scopes[0].Scope != busy || scopes[1].Scope != absent {
		t.Fatalf("scopes=%+v, want busy then absent", scopes)
	}
	// CPUQuota=100% caps the scope at one CPU; a spinning command over a whole
	// second must still register most of it, and cannot exceed host busy CPU.
	if !scopes[0].Present || scopes[0].BusyCPUMilli < 500 || scopes[0].BusyCPUMilli > 1000 || scopes[0].BusyCPUMilli > usage.BusyCPUMilli() {
		t.Fatalf("busy scope=%+v host busy=%d, want present in (500,1000] and <= host", scopes[0], usage.BusyCPUMilli())
	}
	if scopes[1].Present || scopes[1].BusyCPUMilli != 0 {
		t.Fatalf("absent scope=%+v, want absent zero", scopes[1])
	}
	if _, err = resourceCommand(ctx, "systemctl", "--user", "stop", "--", busyName.String()); err != nil {
		t.Fatal(err)
	}
	for {
		present, err := ObserveSystemdScope(ctx, busy)
		if err != nil {
			t.Fatal(err)
		}
		if present == process.LivenessGone {
			break
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	stopped, err := ObserveResourceUsage(ctx, ResourceUsageRequest{Interval: resourceDuration(t, "1s"), Scopes: []SystemdScopeRequest{busy}})
	if err != nil {
		t.Fatal(err)
	}
	if after := stopped.Scopes(); len(after) != 1 || after[0].Present || after[0].BusyCPUMilli != 0 {
		t.Fatalf("collected scope=%+v, want absent zero", after)
	}
}
