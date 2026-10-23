//go:build unix

package process_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/temporal"
)

// drainGate holds a real child's destination until the test has observed the
// child reaped. This pressures the after-exit drain independently of child
// speed; the gate always releases and the execution always joins.
type drainGate struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
	bytes.Buffer
}

func (w *drainGate) Write(p []byte) (int, error) {
	w.once.Do(func() { close(w.entered) })
	<-w.release
	return w.Buffer.Write(p)
}

func TestZeroWaitDelayDrainsAfterChildExit(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	output := &drainGate{entered: make(chan struct{}), release: make(chan struct{})}
	var release sync.Once
	releaseOutput := func() { release.Do(func() { close(output.release) }) }
	defer releaseOutput()
	request := processRequest(t, "output:4096", process.Streams{Stdin: bytes.NewReader(nil), Stdout: output, Stderr: io.Discard})
	request.WaitDelay = temporal.Duration{}
	execution, err := process.Begin(ctx, request)
	if err != nil {
		t.Fatalf("Begin(zero drain) = %v, want nil", err)
	}
	identity, err := execution.Identity()
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan runOutcome, 1)
	go func() { result, err := execution.Wait(); done <- runOutcome{result: result, err: err} }()
	joined := false
	defer func() {
		releaseOutput()
		cancel()
		if !joined {
			select {
			case <-done:
			case <-processTestDeadline(t, processTestBackstop):
				t.Error("drain execution did not join after release")
			}
		}
	}()
	select {
	case <-output.entered:
	case <-processTestDeadline(t, processTestBackstop):
		t.Fatal("child never reached held output")
	}
	for attempt := 0; ; attempt++ {
		alive, err := process.Alive(identity)
		if err != nil {
			t.Fatal(err)
		}
		if alive == process.LivenessGone {
			break
		}
		if attempt == 1000 {
			t.Fatal("finite child did not exit while its output was held")
		}
		if err := temporal.Wait(temporal.WaitRequest{Context: ctx, Duration: milliseconds(t, 10)}); err != nil {
			t.Fatal(err)
		}
	}
	// This is pressure on the native after-exit deadline, not a synchronization
	// substitute: child absence above and final bytes below are the oracles.
	if err := temporal.Wait(temporal.WaitRequest{Context: ctx, Duration: milliseconds(t, 100)}); err != nil {
		t.Fatal(err)
	}
	releaseOutput()
	select {
	case got := <-done:
		joined = true
		observation, observeErr := got.result.Observation()
		if got.err != nil || observeErr != nil || observation.ExitCode != 0 || observation.StdoutBytes.Uint64() != 4096 || output.String() != strings.Repeat("x", 4096) {
			t.Fatalf("zero drain = %+v/%v/%v, bytes %d; want exit 0 and all 4096 bytes after held drain", observation, got.err, observeErr, output.Len())
		}
	case <-processTestDeadline(t, processTestBackstop):
		t.Fatal("released zero-delay drain did not join")
	}
}
