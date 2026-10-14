package hostfacts_test

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/hostfacts"
)

func TestGoCPUConcurrencyMatchesNativeRuntimeWithoutMutation(t *testing.T) {
	t.Parallel()
	before := runtime.GOMAXPROCS(0)
	got, err := hostfacts.ObserveGoCPUConcurrency()
	if err != nil || got.Validate() != nil || got.Int() != before || runtime.GOMAXPROCS(0) != before {
		t.Fatalf("Go CPU concurrency = (%d,%v), want native budget %d and no mutation", got.Int(), err, before)
	}
}

// An owned native subprocess isolates the process-wide runtime override. This
// proves override/observation semantics, not a container quota execution claim.
func TestGoCPUConcurrencyUsesOwnedProcessBudget(t *testing.T) {
	t.Parallel()
	if os.Getenv("PRIMITIVE_CPU_CONCURRENCY_CHILD") == "1" {
		got, err := hostfacts.ObserveGoCPUConcurrency()
		if err != nil || got.Int() != 2 {
			t.Fatalf("owned native budget = (%d,%v), want 2", got.Int(), err)
		}
		return
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v, want nil", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, self, "-test.run=^TestGoCPUConcurrencyUsesOwnedProcessBudget$")
	command.WaitDelay = 10 * time.Second
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GOMAXPROCS=") && !strings.HasPrefix(entry, "PRIMITIVE_CPU_CONCURRENCY_CHILD=") {
			command.Env = append(command.Env, entry)
		}
	}
	command.Env = append(command.Env, "GOMAXPROCS="+strconv.Itoa(2), "PRIMITIVE_CPU_CONCURRENCY_CHILD=1")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("owned native process = (%q,%v), want verified two-CPU execution budget", output, err)
	}
}
