//go:build unix

package process_test

import (
	"bufio"
	"context"
	"io"
	"strconv"
	"syscall"
	"testing"

	"github.com/deliri/primitive/v2026/process"
)

// A spinning helper group must become visible as nonzero native %CPU, and the
// same group must read zero once reaped. Observation repeats on facts only; the
// backstop context bounds a host whose ps never reports the busy group.
func TestObserveGroupCPUSeesBusyNativeGroup(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), processTestBackstop)
	defer cancel()
	in, send := io.Pipe()
	out, receive := io.Pipe()
	defer in.Close()
	defer send.Close()
	defer out.Close()
	defer receive.Close()
	request := processRequest(t, "copy", process.Streams{Stdin: in, Stdout: receive, Stderr: io.Discard})
	request.Arguments = arguments(t, "-test.run=^TestOwnedGroupProcessHelper$", "--", groupOwnershipMarker, groupSpinMarker)
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
			_ = out.Close()
			if err := execution.Sweep(); err != nil {
				t.Error(err)
			}
			_, _ = execution.Wait()
		}
	}()
	line, err := bufio.NewReader(out).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	identity, err := execution.Identity()
	if err != nil || line != strconv.FormatUint(uint64(identity), 10)+"\n" {
		t.Fatalf("helper identity=(%q,%v), want %d", line, err, identity)
	}
	observation := process.GroupObservationRequest{Group: identity, WorkingDirectory: request.WorkingDirectory, Environment: request.Environment}
	var busy uint64
	for busy == 0 {
		busy, err = process.ObserveGroupCPU(ctx, observation)
		if err != nil {
			t.Fatalf("busy CPU observation=(%d,%v), want nonzero native sample", busy, err)
		}
	}
	if _, err = send.Write([]byte{'K'}); err != nil {
		t.Fatal(err)
	}
	result, err := execution.Wait()
	waited = true
	if err != nil {
		t.Fatal(err)
	}
	signal, err := result.TerminationSignal()
	if err != nil || signal != process.SignalNumber(syscall.SIGKILL) {
		t.Fatalf("native signal=(%v,%v), want SIGKILL", signal, err)
	}
	reaped, err := process.ObserveGroupCPU(ctx, observation)
	if err != nil || reaped != 0 {
		t.Fatalf("reaped CPU=(%d,%v), want zero after busy %d", reaped, err, busy)
	}
}
