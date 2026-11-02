//go:build darwin || linux

package hostfacts

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/temporal"
)

// A real final executable must link and observe through the published API
// with CGO_ENABLED=0. The owning Anvil request runs both build and executable;
// no standalone test runner, installed helper, daemon, or retained binary exists.
func TestResourceUsageCGOZeroNative(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "main.go")
	const program = `package main
import (
 "context"
 "fmt"
 "os"
 "github.com/deliri/primitive/v2026/hostfacts"
 "github.com/deliri/primitive/v2026/temporal"
)
func main() {
 interval, err := temporal.DurationFromMilliseconds(250)
 if err != nil { panic(err) }
 sample, err := hostfacts.ObserveResourceUsage(context.Background(), hostfacts.ResourceUsageRequest{Interval: interval})
 if err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(17) }
 if err := sample.Validate(); err != nil { panic(err) }
 fmt.Printf("RESOURCE_WINDOW %d %d %d %d %d %d\n", sample.Window().Nanoseconds(), sample.CPUCount().Int(), sample.BusyCPUMilli(), sample.TotalMemoryBytes().Uint64(), sample.AvailableMemoryBytes().Uint64(), sample.ProcessMemoryBytes().Uint64())
}
`
	if err := os.WriteFile(source, []byte(program), 0600); err != nil {
		t.Fatal(err)
	}
	goName, err := core.ParsePathComponent("go")
	if err != nil {
		t.Fatal(err)
	}
	goCommand, err := process.Resolve(t.Context(), goName)
	if err != nil {
		t.Fatal(err)
	}
	cwd, err := WorkingDirectory()
	if err != nil {
		t.Fatal(err)
	}
	env, err := AmbientEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ name, value string }{{"CGO_ENABLED", "0"}, {"GOWORK", "off"}, {"GOTOOLCHAIN", "local"}, {"GOFLAGS", "-p=1"}} {
		name, err := process.NewEnvironmentName(item.name)
		if err != nil {
			t.Fatal(err)
		}
		value, err := process.NewEnvironmentValue(item.value)
		if err != nil {
			t.Fatal(err)
		}
		env, err = env.With(process.EnvironmentVariable{Name: name, Value: value})
		if err != nil {
			t.Fatal(err)
		}
	}
	executable, err := core.ParseAbsolutePath(filepath.Join(directory, "resource-window"))
	if err != nil {
		t.Fatal(err)
	}
	run := func(command core.AbsolutePath, arguments []string) string {
		t.Helper()
		argv, err := process.ParseArguments(arguments)
		if err != nil {
			t.Fatal(err)
		}
		maximum, err := core.NewByteCount(65536)
		if err != nil {
			t.Fatal(err)
		}
		var out, diagnostic bytes.Buffer
		execution, err := process.Begin(t.Context(), process.Request{Command: command, Arguments: argv, WorkingDirectory: cwd, Environment: env, Streams: process.Streams{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &diagnostic}, OutputPolicy: process.OutputPolicy{Mode: process.OutputModeBounded, Maximum: maximum}, WaitDelay: resourceDuration(t, "3s"), Containment: process.Containment{Isolation: process.IsolationGroup, CancelSignal: process.CancelSignalKill}})
		if err != nil {
			t.Fatal(err)
		}
		// This owner has no intervening path after Begin: reap exactly once.
		// The existing group cancellation kills the owned tree during Wait.
		// After reap observe absence; never signal a possibly recycled group.
		result, err := execution.Wait()
		cleanup, cancel, cleanupErr := temporal.WithTimeout(temporal.TimeoutRequest{Parent: context.WithoutCancel(t.Context()), Duration: resourceDuration(t, "3s")})
		if cleanupErr != nil {
			t.Fatal(cleanupErr)
		}
		gone, absentErr := process.WaitGroupAbsent(cleanup, process.WaitGroupAbsentRequest{Execution: execution, Timeout: resourceDuration(t, "3s"), Interval: resourceDuration(t, "10ms")})
		cancel()
		if absentErr != nil || gone != process.LivenessGone {
			t.Fatalf("owned native build/probe group not absent after join: %v/%v", gone, absentErr)
		}
		if err != nil {
			t.Fatalf("owned native build/probe failed: %v; %s", err, diagnostic.String())
		}
		exit, err := result.ExitCode()
		if err != nil {
			t.Fatal(err)
		}
		if okay, err := exit.Success(); err != nil || !okay {
			t.Fatalf("owned native build/probe exit=%v/%v; %s", exit, err, diagnostic.String())
		}
		return out.String()
	}
	run(goCommand, []string{"build", "-p=1", "-o", executable.String(), source})
	output := run(executable, nil)
	var window int64
	var cpus int
	var busy, total, available, processes uint64
	n, err := fmt.Sscanf(output, "RESOURCE_WINDOW %d %d %d %d %d %d\n", &window, &cpus, &busy, &total, &available, &processes)
	if err != nil || n != 6 || window < 250_000_000 || window >= 1_000_000_000 || cpus < 1 || busy > uint64(cpus)*resourceCPUMilli || total == 0 || available > total || processes == 0 || processes > total {
		t.Fatalf("CGO0 native facts=%q/%v, want valid subsecond sample", output, err)
	}
	t.Logf("CGO0 final native executable joined: %s", strings.TrimSpace(output))
}
