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

// Real user-manager scope: present while sleep runs, absent after native stop
// and collection. This is Linux interoperability proof, not a parser fixture.
func TestSystemdScopeNativeLifetime(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	self, err := process.Self()
	if err != nil {
		t.Fatal(err)
	}
	pid, err := self.Int()
	if err != nil {
		t.Fatal(err)
	}
	name, err := core.ParsePathComponent("primitive-presence-" + strconv.Itoa(pid) + systemdScopeSuffix)
	if err != nil {
		t.Fatal(err)
	}
	request := SystemdScopeRequest{Unit: name, UserManager: true}
	before, err := ObserveSystemdScope(ctx, request)
	if err != nil || before != process.LivenessGone {
		t.Fatalf("before native start=(%v,%v), want gone and nil", before, err)
	}
	tool, err := core.ParsePathComponent("systemd-run")
	if err != nil {
		t.Fatal(err)
	}
	command, err := process.Resolve(ctx, tool)
	if err != nil {
		t.Fatal(err)
	}
	args, err := process.ParseArguments([]string{"--user", "--scope", "--quiet", "--collect", "--slice=proof.slice", "--unit=" + name.String(), "--property=MemoryMax=33554432", "--property=MemorySwapMax=0", "--property=CPUQuota=100%", "--", "/usr/bin/sleep", "60"})
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
		presence, probeErr := ObserveSystemdScope(cleanup, request)
		if probeErr != nil {
			t.Error(probeErr)
		}
		if presence == process.LivenessAlive {
			if _, err := resourceCommand(cleanup, "systemctl", "--user", "stop", "--", name.String()); err != nil {
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
		present, err := ObserveSystemdScope(ctx, request)
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
	if _, err = resourceCommand(ctx, "systemctl", "--user", "stop", "--", name.String()); err != nil {
		t.Fatal(err)
	}
	for {
		present, err := ObserveSystemdScope(ctx, request)
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
}
