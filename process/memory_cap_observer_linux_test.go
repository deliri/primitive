//go:build linux

package process

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"github.com/deliri/primitive/v2026/testserial"
)

// These are real caller programs inside the actual capped scope, not a
// production keeper. The parent catches systemd's TERM after its child OOMs,
// then remains owned by the test's stdin until the positive sample is seen.
func TestMemoryCapNativeHelper(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	role := os.Args[len(os.Args)-1]
	switch role {
	case "memory-cap-allocate":
		data := make([]byte, 512<<20)
		for i := 0; i < len(data); i += 4096 {
			data[i] = 1
		}
		runtime.KeepAlive(data)
		os.Exit(91) // surviving this allocation means the positive setup failed.
	case "memory-cap-parent":
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGTERM)
		defer signal.Stop(signals)
		reader := bufio.NewReader(os.Stdin)
		if command, err := reader.ReadString('\n'); err != nil || command != "allocate\n" {
			os.Exit(92)
		}
		binary, err := os.Executable()
		if err != nil {
			os.Exit(93)
		}
		child := exec.Command(binary, "-test.run=^TestMemoryCapNativeHelper$", "--", "memory-cap-allocate")
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		err = child.Run()
		exit, ok := err.(*exec.ExitError)
		if !ok || !exit.ProcessState.Sys().(syscall.WaitStatus).Signaled() || exit.ProcessState.Sys().(syscall.WaitStatus).Signal() != syscall.SIGKILL {
			os.Exit(94)
		}
		if command, err := reader.ReadString('\n'); err != nil || command != "release\n" {
			os.Exit(95)
		}
		os.Exit(0)
	case "memory-cap-hold":
		_, _ = io.Copy(io.Discard, os.Stdin)
		os.Exit(0)
	case "memory-cap-fail":
		os.Exit(7)
	}
}

func memoryCapNativeBudget(t *testing.T) {
	t.Helper()
	if runtime.GOMAXPROCS(0) < 4 || debug.SetMemoryLimit(-1) < 2<<30 || debug.SetMemoryLimit(-1) == int64(^uint64(0)>>1) {
		t.Fatal("native memory-cap fixture requires admitted semantic CPU >=4 and finite GOMEMLIMIT >=2GiB before effects")
	}
	// Driver soft heap512MiB plus the entire native subtree hard cap256MiB
	// fit below this finite >=2GiB prerequisite with >1GiB remaining headroom.
	// This is a conservative admission check, not a claim soft heaps are caps.
	oldCPU := runtime.GOMAXPROCS(1)
	t.Cleanup(func() { runtime.GOMAXPROCS(oldCPU) })
	oldMemory := debug.SetMemoryLimit(512 << 20)
	t.Cleanup(func() { debug.SetMemoryLimit(oldMemory) })
	t.Setenv("GOMAXPROCS", "1")
	t.Setenv("GOMEMLIMIT", "128MiB")
}
func memoryCapNativeRequest(t *testing.T, role string, input io.Reader) Request {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	command, err := core.ParseAbsolutePath(binary)
	if err != nil {
		t.Fatal(err)
	}
	directory, err := core.ParseAbsolutePath(cwd)
	if err != nil {
		t.Fatal(err)
	}
	argv, err := ParseArguments([]string{"-test.run=^TestMemoryCapNativeHelper$", "--", role})
	if err != nil {
		t.Fatal(err)
	}
	cap, err := core.NewByteLength(256 << 20)
	if err != nil {
		t.Fatal(err)
	}
	maximum, err := core.NewByteCount(64 << 10)
	if err != nil {
		t.Fatal(err)
	}
	delay, err := temporal.DurationFromSeconds(2)
	if err != nil {
		t.Fatal(err)
	}
	return Request{Command: command, WorkingDirectory: directory, Arguments: argv, Environment: Environment{Mode: EnvironmentModeInherit}, Streams: Streams{Stdin: input, Stdout: io.Discard, Stderr: io.Discard}, OutputPolicy: OutputPolicy{Mode: OutputModeBounded, Maximum: maximum}, WaitDelay: delay, Containment: Containment{Isolation: IsolationGroup, CancelSignal: CancelSignalKill, MemoryMax: cap}}
}
func awaitMemoryCap(t *testing.T, ctx context.Context, o *memoryCapObserver, positive bool) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		value := o.snapshot()
		if value != nil && value.State == MemoryCapObserved && (value.LocalOOMEvents > 0) == positive {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("actual owned memory observation positive=%v not captured: %+v: %v", positive, value, ctx.Err())
		case <-ticker.C:
		}
	}
}
func TestMemoryCapCapturesOwnedLocalExhaustion(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	memoryCapNativeBudget(t)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	input, output, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	defer output.Close()
	execution, err := Begin(ctx, memoryCapNativeRequest(t, "memory-cap-parent", input))
	if err != nil {
		t.Fatal(err)
	}
	waited := false
	t.Cleanup(func() {
		output.Close()
		cancel()
		if !waited {
			_, _ = execution.Wait()
		}
		if err := execution.Sweep(); err != nil {
			t.Error(err)
		}
	})
	awaitMemoryCap(t, ctx, execution.prepared.memoryCap, false)
	// A real live owned scope with a different claimed limit is refused.
	mismatched := execution.prepared.memoryCap.owner
	mismatched.limit, _ = core.NewByteLength(128 << 20)
	if got := observeMemoryCap(mismatched); got.State != MemoryCapUnknown || got.LocalOOMEvents != 0 {
		t.Fatalf("wrong actual limit admitted: %+v", got)
	}
	if _, err := io.WriteString(output, "allocate\n"); err != nil {
		t.Fatal(err)
	}
	awaitMemoryCap(t, ctx, execution.prepared.memoryCap, true)
	if _, err := io.WriteString(output, "release\n"); err != nil {
		t.Fatal(err)
	}
	// The final caller-owned stdin handshake is complete. Close its writer
	// before Wait so os/exec can join its stdin copy on every native outcome.
	output.Close()
	result, err := execution.Wait()
	waited = true
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-execution.prepared.memoryCap.done:
	default:
		t.Fatal("sampler task outlived Wait")
	}
	observation, err := result.Observation()
	if err != nil {
		t.Fatal(err)
	}
	if observation.ExitCode != 0 || observation.TerminationSignal != nil {
		t.Fatalf("native parent outcome changed: %+v", observation)
	}
	fact := observation.MemoryCap
	if fact == nil || fact.State != MemoryCapObserved || fact.Limit.Uint64() != 256<<20 || fact.LocalOOMEvents == 0 || fact.LocalOOMKills == 0 {
		t.Fatalf("missing real owned local exhaustion: %+v", fact)
	}
	raw, err := core.MarshalCanonicalJSONDocument(observation)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("OWNED_MEMORY_CAP_NATIVE pid=%d unit=%s observation=%s", execution.prepared.memoryCap.owner.pid, execution.prepared.memoryCap.owner.unit, raw)
	// Projection owns its snapshot, rather than exposing the private result.
	fact.LocalOOMEvents = 0
	again, err := result.Observation()
	if err != nil || again.MemoryCap.LocalOOMEvents == 0 {
		t.Fatalf("projection aliases result: %+v %v", again, err)
	}
}
func TestMemoryCapDoesNotInferExhaustionFromTermination(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	memoryCapNativeBudget(t)
	for _, role := range []string{"ordinary-success", "ordinary-failure", "external-term", "caller-cancel"} {
		t.Run(role, func(t *testing.T) {
			testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			input, output, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer input.Close()
			defer output.Close()
			helper := "memory-cap-hold"
			if role == "ordinary-failure" {
				helper = "memory-cap-fail"
			}
			execution, err := Begin(ctx, memoryCapNativeRequest(t, helper, input))
			if err != nil {
				t.Fatal(err)
			}
			waited := false
			t.Cleanup(func() {
				output.Close()
				cancel()
				if !waited {
					_, _ = execution.Wait()
				}
				if err := execution.Sweep(); err != nil {
					t.Error(err)
				}
			})
			if role != "ordinary-failure" {
				awaitMemoryCap(t, ctx, execution.prepared.memoryCap, false)
				if role == "ordinary-success" {
					output.Close()
				} else if role == "caller-cancel" {
					cancel()
				} else if err := execution.Deliver(CancelSignalTerminate); err != nil {
					t.Fatal(err)
				}
			}
			// The final caller-owned stdin handshake is complete. Close its writer
			// before Wait so os/exec can join its stdin copy on every native outcome.
			output.Close()
			result, err := execution.Wait()
			waited = true
			if role == "caller-cancel" {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("cancel lost: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			observation, err := result.Observation()
			if err != nil {
				t.Fatal(err)
			}
			if observation.MemoryCap == nil || observation.MemoryCap.State != MemoryCapUnknown || observation.MemoryCap.LocalOOMEvents != 0 || observation.MemoryCap.LocalOOMKills != 0 {
				t.Fatalf("termination manufactured OOM: %+v", observation)
			}
			if role == "ordinary-success" && observation.ExitCode != 0 {
				t.Fatalf("success changed: %+v", observation)
			}
			if role == "ordinary-failure" && observation.ExitCode != 7 {
				t.Fatalf("exit=%d want7", observation.ExitCode)
			}
			if role == "external-term" && (observation.TerminationSignal == nil || *observation.TerminationSignal != 15) {
				t.Fatalf("actual TERM missing: %+v", observation)
			}
		})
	}
}
func TestMemoryCapRejectsForeignAndCollectedOwners(t *testing.T) {
	t.Parallel()
	cap, _ := core.NewByteLength(256 << 20)
	for _, owner := range []memoryCapOwner{{unit: "not-owned.scope", pid: ProcessIdentity(os.Getpid()), limit: cap}, {unit: "collected.scope", pid: 0, limit: cap}} {
		got := observeMemoryCap(owner)
		if got.State != MemoryCapUnknown || got.LocalOOMEvents != 0 || got.LocalOOMKills != 0 {
			t.Fatalf("foreign owner admitted: %+v", got)
		}
	}
	// A filesystem alias cannot redirect cgroup custody.
	if file, ok := openMemoryCapDirectory("/../tmp"); ok {
		file.Close()
		t.Fatal("parent traversal admitted")
	}
	if file, ok := openMemoryCapDirectory("/."); ok {
		file.Close()
		t.Fatal("dot traversal admitted")
	}
}

func TestMemoryCapLocalParserRejectsHostileFacts(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"oom 0\n", "oom 1\noom 2\noom_kill 1\n", "oom -1\noom_kill 0\n", "oom 18446744073709551616\noom_kill 0\n", "oom 1\noom_kill 1", "oom 1\noom_kill 1\ntruncated"} {
		if got, ok := readMemoryEvents(strings.NewReader(raw)); ok {
			t.Fatalf("hostile %q admitted %+v", raw, got)
		}
	}
	raw := "unknown_" + strings.Repeat("x", 10000) + " 0\noom 5\noom_kill 2\n"
	got, ok := readMemoryEvents(strings.NewReader(raw))
	if !ok || got.oom != 5 || got.kills != 2 {
		t.Fatalf("future unknown record lost exact local facts: %+v %v", got, ok)
	}
}
