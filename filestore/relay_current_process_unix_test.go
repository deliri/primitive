//go:build unix

package filestore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/temporal"
)

const currentRelayBackstop = 8 * time.Second

func TestCurrentRelayOutputPreservesPartialNativeCause(t *testing.T) {
	t.Parallel()
	file, err := os.CreateTemp(t.TempDir(), "closed-output")
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := file.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	nativeCause := raw.Control(func(uintptr) {})
	if nativeCause == nil {
		t.Fatal("closed native RawControl = nil, want the native closed-descriptor refusal")
	}
	output := currentRelayOutput{streams: process.Streams{Stdin: bytes.NewReader(nil), Stdout: file, Stderr: io.Discard}}
	if n, err := output.Write([]byte("unwritten")); n != 0 || !errors.Is(err, nativeCause) || !errors.Is(err, core.ErrProcessStream) {
		t.Fatalf("closed native output = %d/%v, want zero bytes and native RawControl/Process stream causes", n, err)
	}
}

func TestCurrentRelayOutputPartialPipeNativeCause(t *testing.T) {
	t.Parallel()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close(); _ = writer.Close() }()
	if err := reader.SetReadDeadline(currentRelayReadDeadline(t)); err != nil {
		t.Fatal(err)
	}
	payload := bytes.Repeat([]byte{0x91, 0x34, 0x6b}, 1<<20)
	output := currentRelayOutput{streams: process.Streams{Stdin: bytes.NewReader(nil), Stdout: writer, Stderr: io.Discard}}
	type observed struct {
		written int
		err     error
	}
	done := make(chan observed, 1)
	go func() {
		n, err := output.Write(payload)
		done <- observed{written: n, err: err}
	}()
	var prefix [1]byte
	_, readErr := io.ReadFull(reader, prefix[:])
	_ = reader.Close()
	backstop, release := newNativeBackstop(t.Context(), t, currentRelayBackstop)
	defer release()
	select {
	case result := <-done:
		if readErr != nil || prefix[0] != payload[0] || result.written <= 0 || result.written >= len(payload) || !errors.Is(result.err, syscall.EPIPE) {
			t.Fatalf("released pipe output = %d/%v, observed prefix %x/%v; want a native partial count below %d and EPIPE after exact first byte", result.written, result.err, prefix, readErr, len(payload))
		}
	case <-backstop.Done():
		_ = writer.Close()
		joinBackstop, releaseJoin := newNativeBackstop(t.Context(), t, currentRelayBackstop)
		defer releaseJoin()
		select {
		case <-done:
		case <-joinBackstop.Done():
			t.Fatalf("released pipe writer cleanup exceeded %v, want joined write", currentRelayBackstop)
		}
		t.Fatalf("released pipe output exceeded %v, want native EPIPE and joined write", currentRelayBackstop)
	}
}

func currentRelayReadDeadline(t *testing.T) time.Time {
	t.Helper()
	observation, err := temporal.Observe()
	if err != nil {
		t.Fatal(err)
	}
	instant, err := observation.Instant()
	if err != nil {
		t.Fatal(err)
	}
	duration, err := temporal.NewDuration(currentRelayBackstop)
	if err != nil {
		t.Fatal(err)
	}
	deadline, err := instant.Add(duration)
	if err != nil {
		t.Fatal(err)
	}
	native, err := deadline.Time()
	if err != nil {
		t.Fatal(err)
	}
	return native
}

// The helper is a real, dedicated native process. A valid relay must bypass
// its continuation and deferred marker; only its parent owns final reaping.
func TestRelayCurrentProcessNativeChild(t *testing.T) {
	t.Parallel()
	separator := -1
	for index, argument := range os.Args {
		if argument == "--current-relay" {
			separator = index
		}
	}
	if separator < 0 {
		return
	}
	if separator+1 >= len(os.Args) {
		os.Exit(91)
	}
	defer func() { _, _ = fmt.Fprintln(os.Stderr, "UNOWNED DEFER REACHED") }()
	request := process.LentRequest{Inherited: []process.InheritedStream{process.InheritedStandardInput, process.InheritedStandardOutput}}
	switch os.Args[separator+1] {
	case "reversed":
		request.Inherited[0], request.Inherited[1] = request.Inherited[1], request.Inherited[0]
	case "known-copy", "broken-stderr":
		if err := os.Stdin.Close(); err != nil {
			os.Exit(92)
		}
	case "known-close":
		own, err := process.StandardStreams()
		if err != nil {
			os.Exit(93)
		}
		lent, err := process.LentStreams(request)
		if err != nil {
			os.Exit(94)
		}
		// Native directory reads fail with EISDIR. Closing the actual lent
		// output twice supplies a distinct native ErrClosed close cause.
		directory, err := os.Open(".")
		if err != nil {
			os.Exit(95)
		}
		own.Stdin = directory
		lent.Output = currentRelayDoubleClose{output: lent.Output}
		_ = relayCurrentProcess(own, lent)
		_, _ = fmt.Fprintln(os.Stderr, "UNOWNED RETURN REACHED")
		return
	case "close-near":
		if err := os.Stdout.Close(); err != nil {
			os.Exit(96)
		}
	}
	_ = RelayCurrentProcess(request)
	_, _ = fmt.Fprintln(os.Stderr, "UNOWNED RETURN REACHED")
}

type currentRelayDoubleClose struct {
	output io.WriteCloser
}

func (output currentRelayDoubleClose) Write(payload []byte) (int, error) {
	return output.output.Write(payload)
}

func (output currentRelayDoubleClose) Close() error {
	return errors.Join(output.output.Close(), output.output.Close())
}

type currentRelayChild struct {
	command    *exec.Cmd
	cancel     context.CancelFunc
	done       chan error
	joined     bool
	nearInput  *os.File
	farInput   *os.File
	farOutput  *os.File
	stdout     bytes.Buffer
	stderr     bytes.Buffer
	outputRead *os.File
}

func startCurrentRelayChild(t *testing.T, mode string, brokenOutput, observeOutput bool) *currentRelayChild {
	t.Helper()
	ctx, cancel := newNativeBackstop(context.Background(), t, currentRelayBackstop)
	child := &currentRelayChild{cancel: cancel, done: make(chan error, 1)}
	nearRead, nearWrite, err := os.Pipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	farRead, farWrite, err := os.Pipe()
	if err != nil {
		_ = nearRead.Close()
		_ = nearWrite.Close()
		cancel()
		t.Fatal(err)
	}
	upRead, upWrite, err := os.Pipe()
	if err != nil {
		_ = nearRead.Close()
		_ = nearWrite.Close()
		_ = farRead.Close()
		_ = farWrite.Close()
		cancel()
		t.Fatal(err)
	}
	child.nearInput, child.farInput, child.farOutput = nearWrite, farWrite, upRead
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRelayCurrentProcessNativeChild$", "--", "--current-relay", mode)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error { return syscall.Kill(-command.Process.Pid, syscall.SIGKILL) }
	command.WaitDelay = time.Second
	command.Stdin, command.Stdout, command.Stderr = nearRead, &child.stdout, &child.stderr
	command.ExtraFiles = []*os.File{farRead, upWrite}
	if mode == "reversed" {
		command.ExtraFiles[0], command.ExtraFiles[1] = command.ExtraFiles[1], command.ExtraFiles[0]
	}
	var outputWrite *os.File
	var diagnosticWrite *os.File
	if mode == "broken-stderr" {
		diagnosticRead, write, pipeErr := os.Pipe()
		if pipeErr != nil {
			_ = nearRead.Close()
			_ = farRead.Close()
			_ = upWrite.Close()
			child.cleanup(t)
			t.Fatal(pipeErr)
		}
		_ = diagnosticRead.Close()
		diagnosticWrite = write
		command.Stderr = write
	}
	if brokenOutput || observeOutput {
		child.outputRead, outputWrite, err = os.Pipe()
		if err != nil {
			_ = nearRead.Close()
			_ = farRead.Close()
			_ = upWrite.Close()
			child.cleanup(t)
			t.Fatal(err)
		}
		command.Stdout = outputWrite
		if brokenOutput {
			_ = child.outputRead.Close()
			child.outputRead = nil
		}
	}
	child.command = command
	err = command.Start()
	_ = nearRead.Close()
	_ = farRead.Close()
	_ = upWrite.Close()
	if outputWrite != nil {
		_ = outputWrite.Close()
	}
	if diagnosticWrite != nil {
		_ = diagnosticWrite.Close()
	}
	if err != nil {
		child.command = nil
		child.cleanup(t)
		t.Fatal(err)
	}
	go func() { child.done <- command.Wait() }()
	t.Cleanup(func() { child.cleanup(t) })
	return child
}

func (child *currentRelayChild) wait(t *testing.T) error {
	t.Helper()
	backstop, release := newNativeBackstop(t.Context(), t, currentRelayBackstop+time.Second)
	defer release()
	select {
	case err := <-child.done:
		child.joined = true
		if err := syscall.Kill(-child.command.Process.Pid, 0); !errors.Is(err, syscall.ESRCH) {
			t.Fatalf("reaped relay group observation = %v, want native ESRCH", err)
		}
		if strings.Contains(child.stderr.String(), "UNOWNED") {
			t.Fatalf("terminal relay stderr = %q, want no continuation or deferred cleanup after the ownership boundary", child.stderr.String())
		}
		return err
	case <-backstop.Done():
		child.cleanup(t)
		t.Fatalf("native relay wait exceeded %v after release/join; want parent-owned exit", currentRelayBackstop)
		return nil
	}
}

func (child *currentRelayChild) cleanup(t *testing.T) {
	t.Helper()
	child.cancel()
	for _, file := range []*os.File{child.nearInput, child.farInput, child.farOutput, child.outputRead} {
		if file != nil {
			_ = file.Close()
		}
	}
	if child.command != nil && !child.joined {
		_ = syscall.Kill(-child.command.Process.Pid, syscall.SIGKILL)
		// Cleanup runs after t.Context is cancelled, so its backstop has no test parent.
		backstop, release := newNativeBackstop(context.Background(), t, currentRelayBackstop)
		defer release()
		select {
		case waitErr := <-child.done:
			if waitErr != nil {
				t.Logf("released native child join = %v", waitErr)
			}
			child.joined = true
		case <-backstop.Done():
			t.Fatalf("released native relay cleanup exceeded %v, want joined child", currentRelayBackstop)
		}
	}
}

// Far EOF must reap the bridge while Near stdin remains open. Exact output
// and upward acknowledgement challenge truncation and direction swaps. There
// is no elapsed-time non-return assertion; parent Wait is the native oracle.
func TestRelayCurrentProcessNativeLifetime(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"ordinary", "reversed"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			child := startCurrentRelayChild(t, mode, false, false)
			up := []byte("upward request\x00exact")
			if _, err := child.nearInput.Write(up); err != nil {
				t.Fatal(err)
			}
			gotUp := make([]byte, len(up))
			if _, err := io.ReadFull(child.farOutput, gotUp); err != nil || !bytes.Equal(gotUp, up) {
				t.Fatalf("upward native bytes = %x/%v, want %x", gotUp, err, up)
			}
			down := bytes.Repeat([]byte("position-dependent-down\x00"), 18000)
			if _, err := child.farInput.Write(down); err != nil {
				t.Fatal(err)
			}
			if err := child.farInput.Close(); err != nil {
				t.Fatal(err)
			}
			if err := child.wait(t); err != nil || !bytes.Equal(child.stdout.Bytes(), down) || child.stderr.Len() != 0 {
				t.Fatalf("terminal relay = %v, down %d/%d, stderr %q; want exact drained bytes and successful process exit with stdin held", err, child.stdout.Len(), len(down), child.stderr.String())
			}
		})
	}
}

func TestRelayCurrentProcessNativeFailures(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, mode string
		broken     bool
		causes     []string
	}{
		{name: "known native upward copy cause", mode: "known-copy", causes: []string{os.ErrClosed.Error()}},
		{name: "known copy and close causes survive together", mode: "known-close", causes: []string{syscall.EISDIR.Error(), os.ErrClosed.Error()}},
		{name: "native near close cause", mode: "close-near", causes: []string{os.ErrClosed.Error()}},
		{name: "broken stdout reports EPIPE instead of SIGPIPE", mode: "ordinary", broken: true, causes: []string{syscall.EPIPE.Error()}},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			child := startCurrentRelayChild(t, row.mode, row.broken, false)
			if row.mode == "known-copy" || row.mode == "known-close" {
				// FarOutput EOF happens only after the worker publishes its
				// copy result and closes the native output. Far EOF follows it.
				var probe [1]byte
				if n, err := child.farOutput.Read(probe[:]); n != 0 || err != io.EOF {
					t.Fatalf("known-copy completion observation = %d/%v, want native EOF before far input release", n, err)
				}
			}
			if row.broken {
				if _, err := child.farInput.Write([]byte("must report broken pipe")); err != nil {
					t.Fatal(err)
				}
			}
			_ = child.farInput.Close()
			err := child.wait(t)
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != int(process.ExitFailure) {
				t.Fatalf("native failure exit = %v, stderr %q; want explicit ExitFailure rather than signal", err, child.stderr.String())
			}
			for _, cause := range row.causes {
				if !strings.Contains(child.stderr.String(), cause) {
					t.Fatalf("native diagnostics = %q, want known cause %q retained", child.stderr.String(), cause)
				}
			}
		})
	}
}

func TestRelayCurrentProcessNativeCancellation(t *testing.T) {
	t.Parallel()
	child := startCurrentRelayChild(t, "ordinary", false, true)
	prefix := []byte("both copies are now owned")
	if _, err := child.farInput.Write(prefix); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(prefix))
	if _, err := io.ReadFull(child.outputRead, got); err != nil || !bytes.Equal(got, prefix) {
		t.Fatalf("native relay start observation = %q/%v, want %q before cancellation", got, err, prefix)
	}
	child.cancel()
	err := child.wait(t)
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != -1 {
		t.Fatalf("cancelled native process = %v, want signalled child reaped with group gone", err)
	}
}

func TestRelayCurrentProcessBrokenDiagnosticsNativeFailure(t *testing.T) {
	t.Parallel()
	child := startCurrentRelayChild(t, "broken-stderr", false, false)
	var probe [1]byte
	if n, err := child.farOutput.Read(probe[:]); n != 0 || err != io.EOF {
		t.Fatalf("known-copy completion = %d/%v, want native EOF before far input release", n, err)
	}
	_ = child.farInput.Close()
	err := child.wait(t)
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != int(process.ExitFailure) {
		t.Fatalf("broken diagnostic output exit = %v, want explicit ExitFailure rather than SIGPIPE or success", err)
	}
}

// Compiler-bound public ingress proof; its platform-specific implementation
// exercises the process, not an arbitrary-stream stand-in for the public door.
func RelayCurrentProcessNativeProofForTest(t *testing.T) {
	t.Helper()
	t.Run("lifetime", TestRelayCurrentProcessNativeLifetime)
	t.Run("failures", TestRelayCurrentProcessNativeFailures)
	t.Run("cancellation", TestRelayCurrentProcessNativeCancellation)
	t.Run("broken diagnostics", TestRelayCurrentProcessBrokenDiagnosticsNativeFailure)
}
