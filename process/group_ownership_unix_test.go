//go:build unix

package process_test

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

const groupOwnershipMarker = "--primitive-owned-group-proof"
const groupSpinMarker = "--spin"

func TestOwnedGroupProcessHelper(t *testing.T) {
	t.Parallel()
	if !slices.Contains(os.Args, groupOwnershipMarker) {
		return
	}
	group, err := process.OwnGroup()
	if slices.Contains(os.Args, "--shared-group") {
		if group != nil || !errors.Is(err, core.ErrProcessContract) {
			t.Fatalf("shared OwnGroup=(%v,%v), want nil contract refusal", group, err)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	identity, err := group.Identity()
	if err != nil || identity != process.ProcessIdentity(os.Getpid()) {
		t.Fatalf("owned identity=(%d,%v), want self %d", identity, err, os.Getpid())
	}
	copied := *group
	if err := copied.Kill(); !errors.Is(err, core.ErrProcessContract) {
		t.Fatalf("copied Kill=%v, want contract refusal", err)
	}
	if _, err = io.WriteString(os.Stdout, strconv.FormatUint(uint64(identity), 10)+"\n"); err != nil {
		t.Fatal(err)
	}
	// The reader owns stdin until the parent's stop token or pipe closure.
	stopped := make(chan error, 1)
	go func() {
		var token [1]byte
		_, err := io.ReadFull(os.Stdin, token[:])
		if err == nil && token[0] != 'K' {
			err = core.ErrProcessContract
		}
		stopped <- err
	}()
	if slices.Contains(os.Args, groupSpinMarker) {
		for len(stopped) == 0 {
		}
	}
	if err = <-stopped; err != nil {
		t.Fatal(err)
	}
	if err = group.Kill(); err != nil {
		t.Fatal(err)
	}
	t.Fatal("owned group kill returned without terminating self")
}

func TestOwnGroupRefusesSharedNativeGroup(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), processTestBackstop)
	defer cancel()
	request := processRequest(t, "copy", process.Streams{Stdin: strings.NewReader(""), Stdout: io.Discard, Stderr: io.Discard})
	request.Arguments = arguments(t, "-test.run=^TestOwnedGroupProcessHelper$", "--", groupOwnershipMarker, "--shared-group")
	result, err := process.Run(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	code, err := result.ExitCode()
	if err != nil {
		t.Fatal(err)
	}
	if success, err := code.Success(); err != nil || !success {
		t.Fatalf("shared helper exit=(%v,%v), want success", code, err)
	}
}

func TestOwnedGroupNativeLayerTriad(t *testing.T) {
	t.Parallel()
	for _, group := range []*process.OwnedGroup{nil, new(process.OwnedGroup)} {
		if identity, err := group.Identity(); identity != 0 || !errors.Is(err, core.ErrProcessContract) {
			t.Fatalf("unowned Identity=(%d,%v), want zero contract refusal", identity, err)
		}
		if err := group.Kill(); !errors.Is(err, core.ErrProcessContract) {
			t.Fatalf("unowned Kill=%v, want contract refusal", err)
		}
	}
	for _, identity := range []process.ProcessIdentity{0, 1, process.ProcessIdentity(core.ProcessPOSIXIdentityMaximum) + 1} {
		got, err := process.ObserveGroupLiveness(identity)
		if got != process.LivenessUnknown || !errors.Is(err, core.ErrProcessContract) {
			t.Fatalf("invalid group %d=(%v,%v)", identity, got, err)
		}
	}
	ctx, cancel := context.WithTimeout(t.Context(), processTestBackstop)
	defer cancel()
	in, send := io.Pipe()
	out, receive := io.Pipe()
	defer in.Close()
	defer send.Close()
	defer out.Close()
	defer receive.Close()
	closedOutput := make(chan struct{})
	stopOutput := context.AfterFunc(ctx, func() { _ = out.Close(); close(closedOutput) })
	defer func() {
		if !stopOutput() {
			<-closedOutput
		}
	}()
	request := processRequest(t, "copy", process.Streams{Stdin: in, Stdout: receive, Stderr: io.Discard})
	request.Arguments = arguments(t, "-test.run=^TestOwnedGroupProcessHelper$", "--", groupOwnershipMarker)
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
	// Native output is the synchronization fact; timeout cancellation closes
	// the child's descriptors through os/exec if startup or shutdown wedges.
	line, err := bufio.NewReader(out).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	identity, err := execution.Identity()
	if err != nil || line != strconv.FormatUint(uint64(identity), 10)+"\n" {
		t.Fatalf("helper identity=(%q,%v), want %d", line, err, identity)
	}
	alive, err := process.ObserveGroupLiveness(identity)
	if err != nil || alive != process.LivenessAlive {
		t.Fatalf("live observation=(%v,%v)", alive, err)
	}
	memoryRequest := process.GroupObservationRequest{Group: identity, WorkingDirectory: request.WorkingDirectory, Environment: request.Environment}
	memory, err := process.ObserveGroupMemory(ctx, memoryRequest)
	if err != nil || memory.Uint64() == 0 {
		t.Fatalf("live RSS=(%d,%v), want positive native sample", memory.Uint64(), err)
	}
	// Every native row must parse, so this also proves the host's %CPU spelling.
	// The helper is blocked on its pipe, so only success is asserted here.
	if _, err = process.ObserveGroupCPU(ctx, memoryRequest); err != nil {
		t.Fatalf("live CPU observation=%v, want native sample", err)
	}
	if _, err = send.Write([]byte{'K'}); err != nil {
		t.Fatal(err)
	}
	if err = send.Close(); err != nil {
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
	alive, err = process.ObserveGroupLiveness(identity)
	if err != nil || alive != process.LivenessGone {
		t.Fatalf("reaped group=(%v,%v), want gone", alive, err)
	}
	memory, err = process.ObserveGroupMemory(ctx, memoryRequest)
	if err != nil || memory.Uint64() != 0 {
		t.Fatalf("absent RSS=(%d,%v), want zero", memory.Uint64(), err)
	}
	cpu, err := process.ObserveGroupCPU(ctx, memoryRequest)
	if err != nil || cpu != 0 {
		t.Fatalf("absent CPU=(%d,%v), want zero", cpu, err)
	}
	closed, cancelClosed := context.WithCancel(ctx)
	cancelClosed()
	memory, err = process.ObserveGroupMemory(closed, memoryRequest)
	if !errors.Is(err, context.Canceled) || memory.Uint64() != 0 {
		t.Fatalf("cancelled RSS=(%d,%v), want zero cancellation", memory.Uint64(), err)
	}
	cpu, err = process.ObserveGroupCPU(closed, memoryRequest)
	if !errors.Is(err, context.Canceled) || cpu != 0 {
		t.Fatalf("cancelled CPU=(%d,%v), want zero cancellation", cpu, err)
	}
}
