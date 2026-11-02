//go:build unix

package process_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/temporal"
)

type scopeOutputGate struct {
	ready, release chan struct{}
	once           sync.Once
}

func (g *scopeOutputGate) Write(data []byte) (int, error) {
	g.once.Do(func() { close(g.ready) })
	<-g.release
	return len(data), nil
}

type scopeWaitObservation struct {
	result process.Result
	err    error
}

func TestExecutionScopeGoneChildKeepsPendingWaitAndRejectsDuplicate(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), processTestBackstop)
	defer cancel()
	gate := &scopeOutputGate{ready: make(chan struct{}), release: make(chan struct{})}
	var release sync.Once
	defer release.Do(func() { close(gate.release) })
	var scope process.ExecutionScope
	request := processRequest(t, "streams", process.Streams{Stdin: bytes.NewReader([]byte("native")), Stdout: gate, Stderr: io.Discard})
	request.Containment.Isolation = process.IsolationGroup
	execution, err := scope.Begin(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	completion := make(chan scopeWaitObservation, 1)
	go func() {
		result, err := scope.Wait(process.ExecutionWaitRequest{Execution: execution})
		completion <- scopeWaitObservation{result, err}
	}()
	defer func() {
		release.Do(func() { close(gate.release) })
		cancel()
		select {
		case <-completion:
		case <-processTestDeadline(t, processTestBackstop):
			t.Errorf("native wait cleanup completed=false after %v backstop, want completed", processTestBackstop)
		}
	}()
	select {
	case <-gate.ready:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	budget, err := temporal.DurationFromSeconds(5)
	if err != nil {
		t.Fatal(err)
	}
	interval, err := temporal.DurationFromMilliseconds(1)
	if err != nil {
		t.Fatal(err)
	}
	liveness, err := process.WaitGroupAbsent(ctx, process.WaitGroupAbsentRequest{Execution: execution, Timeout: budget, Interval: interval})
	if err != nil || liveness != process.LivenessGone {
		t.Fatalf("native group=%v error=%v,want gone while output remains borrowed", liveness, err)
	}
	if _, err := scope.Wait(process.ExecutionWaitRequest{Execution: execution}); !errors.Is(err, core.ErrProcessContract) || scope.ActiveCount() != 1 {
		t.Fatalf("duplicate wait=%v active=%d,want refusal and retained original wait", err, scope.ActiveCount())
	}
	release.Do(func() { close(gate.release) })
	select {
	case observed := <-completion:
		// Put the completed observation back so deferred cleanup always joins once.
		completion <- observed
		if observed.err != nil || observed.result.Validate() != nil || scope.ActiveCount() != 0 {
			t.Fatalf("native wait=%v validation=%v active=%d,want completed stream cleanup", observed.err, observed.result.Validate(), scope.ActiveCount())
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}
