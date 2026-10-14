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

func TestWaitGroupAbsentNativeExecutionLayerTriad(t *testing.T) {
	t.Parallel()
	owner, cancel := context.WithTimeout(t.Context(), processTestBackstop)
	defer cancel()
	ready := make(chan struct{})
	request := processRequest(t, "wait", process.Streams{Stdin: bytes.NewReader(nil), Stdout: &readyWriter{ready: ready}, Stderr: io.Discard})
	request.Containment.Isolation = process.IsolationGroup
	execution, err := process.Begin(owner, request)
	if err != nil {
		t.Fatal(err)
	}
	defer execution.Sweep()
	select {
	case <-ready:
	case <-owner.Done():
		execution.Terminate()
		execution.Wait()
		t.Fatal(owner.Err())
	}
	budget, err := temporal.DurationFromMilliseconds(10)
	if err != nil {
		t.Fatal(err)
	}
	interval, err := temporal.DurationFromMilliseconds(1)
	if err != nil {
		t.Fatal(err)
	}
	got, err := process.WaitGroupAbsent(owner, process.WaitGroupAbsentRequest{Execution: execution, Timeout: budget, Interval: interval})
	if got != process.LivenessUnknown || !errors.Is(err, context.DeadlineExceeded) {
		execution.Terminate()
		execution.Wait()
		t.Fatalf("livegroup=%v/%v,want noabsence/native deadline", got, err)
	}
	if err := execution.Terminate(); err != nil {
		execution.Wait()
		t.Fatal(err)
	}
	if _, err := execution.Wait(); err != nil {
		t.Fatal(err)
	}
	got, err = process.WaitGroupAbsent(owner, process.WaitGroupAbsentRequest{Execution: execution, Timeout: budget, Interval: interval})
	if got != process.LivenessGone || err != nil {
		t.Fatalf("reapedgroup=%v/%v,want nativeabsence", got, err)
	}
	ctx, stop := context.WithCancel(owner)
	stop()
	got, err = process.WaitGroupAbsent(ctx, process.WaitGroupAbsentRequest{Execution: execution, Timeout: budget, Interval: interval})
	if got != process.LivenessUnknown || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled=%v/%v,want nativecause/unknown", got, err)
	}
	got, err = process.WaitGroupAbsent(owner, process.WaitGroupAbsentRequest{Timeout: budget, Interval: interval})
	if got != process.LivenessUnknown || !errors.Is(err, core.ErrProcessContract) {
		t.Fatalf("missing=%v/%v,want typedrefusal", got, err)
	}
	got, err = process.WaitGroupAbsent(owner, process.WaitGroupAbsentRequest{Execution: execution, Timeout: budget})
	if got != process.LivenessUnknown || !errors.Is(err, core.ErrProcessContract) {
		t.Fatalf("zero interval=%v/%v,want typed refusal", got, err)
	}
}

type waitAbsentOutputGate struct {
	ready   chan struct{}
	release chan struct{}
	once    sync.Once
}

func (g *waitAbsentOutputGate) Write(data []byte) (int, error) {
	g.once.Do(func() { close(g.ready) })
	<-g.release
	return len(data), nil
}

func TestWaitGroupAbsentDoesNotConsumeWaitOrReleaseBorrowedOutput(t *testing.T) {
	t.Parallel()
	owner, cancel := context.WithTimeout(t.Context(), processTestBackstop)
	defer cancel()
	gate := &waitAbsentOutputGate{ready: make(chan struct{}), release: make(chan struct{})}
	var release sync.Once
	defer release.Do(func() { close(gate.release) })
	request := processRequest(t, "wait", process.Streams{Stdin: bytes.NewReader(nil), Stdout: gate, Stderr: io.Discard})
	request.Containment.Isolation = process.IsolationGroup
	execution, err := process.Begin(owner, request)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-gate.ready:
	case <-owner.Done():
		release.Do(func() { close(gate.release) })
		execution.Terminate()
		execution.Wait()
		t.Fatal(owner.Err())
	}
	if err := execution.Terminate(); err != nil {
		release.Do(func() { close(gate.release) })
		execution.Wait()
		t.Fatal(err)
	}
	budget, err := temporal.DurationFromSeconds(1)
	if err != nil {
		t.Fatal(err)
	}
	interval, err := temporal.DurationFromMilliseconds(1)
	if err != nil {
		t.Fatal(err)
	}
	// A concurrent native Wait reaps the killed child while its output copier
	// remains blocked on caller-owned data-plane custody.
	completed := make(chan error, 1)
	go func() { _, err := execution.Wait(); completed <- err }()
	got, err := process.WaitGroupAbsent(owner, process.WaitGroupAbsentRequest{Execution: execution, Timeout: budget, Interval: interval})
	if err != nil || got != process.LivenessGone {
		release.Do(func() { close(gate.release) })
		<-completed
		t.Fatalf("absence=%v/%v,want nativegone with pending output custody", got, err)
	}
	select {
	case err := <-completed:
		t.Fatalf("Wait completed before caller output release: %v", err)
	default:
	}
	release.Do(func() { close(gate.release) })
	select {
	case err := <-completed:
		if err != nil {
			t.Fatal(err)
		}
	case <-owner.Done():
		t.Fatal(owner.Err())
	}
}

func TestWaitGroupAbsentDoesNotProbeReapedDirectIdentity(t *testing.T) {
	t.Parallel()
	request := processRequest(t, "copy", process.Streams{Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard})
	execution, err := process.Begin(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	duration, err := temporal.DurationFromMilliseconds(1)
	if err != nil {
		t.Fatal(err)
	}
	got, err := process.WaitGroupAbsent(t.Context(), process.WaitGroupAbsentRequest{Execution: execution, Timeout: duration, Interval: duration})
	if got != process.LivenessUnknown || !errors.Is(err, core.ErrProcessContract) {
		execution.Terminate()
		execution.Wait()
		t.Fatalf("direct group request=%v/%v,want explicit containment refusal", got, err)
	}
	if _, err := execution.Wait(); err != nil {
		t.Fatal(err)
	}
	got, err = process.WaitGroupAbsent(t.Context(), process.WaitGroupAbsentRequest{Execution: execution, Timeout: duration, Interval: duration})
	if got != process.LivenessUnknown || !errors.Is(err, core.ErrProcessContract) {
		t.Fatalf("reaped direct=%v/%v,want refusal without reused identity probe", got, err)
	}
}

func FuzzWaitGroupAbsentConservesNativeLifecycle(f *testing.F) {
	f.Add([]byte{0, 0xff, 'x'}, false)
	f.Add([]byte("native"), true)
	f.Fuzz(func(t *testing.T, data []byte, cancelled bool) {
		// Bound only the helper echo workload, not any production admission.
		data = data[:min(len(data), 1024)]
		var output bytes.Buffer
		request := processRequest(t, "copy", process.Streams{Stdin: bytes.NewReader(data), Stdout: &output, Stderr: io.Discard})
		request.Containment.Isolation = process.IsolationGroup
		execution, err := process.Begin(t.Context(), request)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := execution.Wait(); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(output.Bytes(), data) {
			t.Fatalf("nativeecho=%x,want%x", output.Bytes(), data)
		}
		budget, err := temporal.DurationFromSeconds(1)
		if err != nil {
			t.Fatal(err)
		}
		interval, err := temporal.DurationFromMilliseconds(1)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		if cancelled {
			cancel()
		}
		got, err := process.WaitGroupAbsent(ctx, process.WaitGroupAbsentRequest{Execution: execution, Timeout: budget, Interval: interval})
		if cancelled {
			if got != process.LivenessUnknown || !errors.Is(err, context.Canceled) {
				t.Fatalf("cancelled=%v/%v,want noobservation/nativecause", got, err)
			}
			return
		}
		native, probeErr := execution.GroupLiveness()
		if err != nil || probeErr != nil || got != process.LivenessGone || native != got {
			t.Fatalf("wait/native/errors=%v/%v/%v/%v,want matchingkernelabsence", got, native, err, probeErr)
		}
	})
}
