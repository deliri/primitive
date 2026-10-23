//go:build unix

package process_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime/pprof"
	"syscall"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/process"
	"github.com/deliri/primitive/v2026/testserial"
)

func outputFlags(file *os.File) (uintptr, error) {
	raw, err := file.SyscallConn()
	if err != nil {
		return 0, err
	}
	var flags uintptr
	var native syscall.Errno
	err = raw.Control(func(fd uintptr) { flags, _, native = syscall.Syscall(syscall.SYS_FCNTL, fd, syscall.F_GETFL, 0) })
	if native != 0 {
		return 0, errors.Join(err, native)
	}
	return flags, err
}

func TestStandardOutputAliasHelper(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardProcessOutput, Scope: core.TestIsolationScopePackageProcess})
	role, _, selected := helperBehavior()
	if !selected {
		return
	}
	before, err := outputFlags(os.Stdin)
	if err != nil {
		os.Exit(90)
	}
	output, err := process.StandardOutput()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(91)
	}
	if role == "broken-output" {
		n, writeErr := output.Write([]byte("native pipe refusal"))
		closeErr := output.Close()
		if n != 0 || !errors.Is(writeErr, core.ErrProcessStream) || !errors.Is(writeErr, syscall.EPIPE) || closeErr != nil {
			_, _ = fmt.Fprintln(os.Stderr, "broken output got", n, writeErr, closeErr, "want zero, EPIPE, stream identity and successful close")
			os.Exit(95)
		}
		os.Exit(0)
	}
	if role == "unconnected-output" {
		closeErr := output.Close()
		_, writeErr := os.Stdout.Write([]byte("must be closed"))
		if !errors.Is(closeErr, core.ErrProcessStream) || !errors.Is(closeErr, syscall.ENOTCONN) || !errors.Is(writeErr, os.ErrClosed) {
			_, _ = fmt.Fprintln(os.Stderr, closeErr, writeErr)
			os.Exit(92)
		}
		os.Exit(0)
	}
	lent, err := process.LentStreams(process.LentRequest{Inherited: []process.InheritedStream{process.InheritedStandardInput, process.InheritedStandardOutput}})
	if err != nil {
		os.Exit(93)
	}
	_, relayErr := filestore.Relay(filestore.RelayRequest{NearInput: os.Stdin, NearOutput: output, FarInput: lent.Input, FarOutput: lent.Output})
	closeErr := lent.Input.Close()
	after, flagErr := outputFlags(os.Stdin)
	if err := errors.Join(relayErr, closeErr, flagErr); err != nil || before != after {
		_, _ = fmt.Fprintln(os.Stderr, err, before, after)
		os.Exit(94)
	}
	profile := pprof.Lookup("goroutineleak")
	if profile == nil {
		os.Exit(96)
	}
	if err := profile.WriteTo(os.Stderr, 1); err != nil {
		os.Exit(97)
	}
	os.Exit(0)
}

// stdin and stdout are two descriptors for one socket. The peer withholds its
// reply until it has observed EOF, and the child's read direction must still
// accept that reply. Closing fd1 alone or shutting down both directions fails.
func TestStandardOutputAliasedSocketLayerTriad(t *testing.T) {
	t.Parallel()
	for _, cancelled := range []bool{false, true} {
		name := "peer EOF releases a reply through aliased stdin"
		if cancelled {
			name = "cancellation reaps a relay blocked on aliased stdin"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			pair, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
			if err != nil {
				t.Fatal(err)
			}
			near, peer := os.NewFile(uintptr(pair[0]), "alias child"), os.NewFile(uintptr(pair[1]), "alias peer")
			downRead, downWrite, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			upRead, upWrite, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				for _, f := range []*os.File{near, peer, downRead, downWrite, upRead, upWrite} {
					_ = f.Close()
				}
			}()
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			deadline, stop := context.WithTimeout(t.Context(), processTestBackstop)
			defer stop()
			ctx, cancel := context.WithCancel(deadline)
			defer cancel()
			cmd := exec.CommandContext(ctx, executable, "-test.run=^TestStandardOutputAliasHelper$", "--", "aliased-output")
			var diagnostics bytes.Buffer
			cmd.Stdin = near
			cmd.Stdout = near
			cmd.Stderr = &diagnostics
			cmd.ExtraFiles = []*os.File{downRead, upWrite}
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			waited := false
			defer func() {
				if !waited {
					cancel()
					_ = cmd.Wait()
				}
			}()
			_ = near.Close()
			_ = downRead.Close()
			_ = upWrite.Close()
			downward := bytes.Repeat([]byte{0x3a}, 4096)
			upward := bytes.Repeat([]byte{0xc5}, 4096)
			if _, err := downWrite.Write(downward); err != nil {
				t.Fatal(err)
			}
			if err := downWrite.Close(); err != nil {
				t.Fatal(err)
			}
			var received bytes.Buffer
			_, err = io.Copy(&received, peer)
			if err != nil || ctx.Err() != nil || !bytes.Equal(received.Bytes(), downward) {
				t.Fatalf("socket EOF bytes=%d/%d, copy=%v, context=%v; want exact bytes and EOF before reply", received.Len(), len(downward), err, ctx.Err())
			}
			if cancelled {
				cancel()
				err = cmd.Wait()
				waited = true
				var native *exec.ExitError
				if !errors.As(err, &native) || ctx.Err() != context.Canceled {
					t.Fatalf("cancelled alias child=%v, context=%v; want native reaped exit and cancellation", err, ctx.Err())
				}
				return
			}
			if _, err := peer.Write(upward); err != nil {
				t.Fatalf("reply after peer EOF=%v, want aliased stdin still writable", err)
			}
			raw, err := peer.SyscallConn()
			if err != nil {
				t.Fatal(err)
			}
			var shutdownErr error
			if err := raw.Control(func(fd uintptr) { shutdownErr = syscall.Shutdown(int(fd), syscall.SHUT_WR) }); err != nil || shutdownErr != nil {
				t.Fatal(errors.Join(err, shutdownErr))
			}
			var reply bytes.Buffer
			if _, err := io.Copy(&reply, upRead); err != nil {
				t.Fatal(err)
			}
			err = cmd.Wait()
			waited = true
			t.Logf("joined alias child native profile: %s", diagnostics.String())
			if err != nil || !bytes.Equal(reply.Bytes(), upward) {
				t.Fatalf("alias relay bytes=%d/%d, child=%v, stderr=%q; want exact reply, unchanged blocking flags and joined clean exit", reply.Len(), len(upward), err, diagnostics.String())
			}
		})
	}
}

func TestStandardOutputCloseFailureStillClosesOwnedHandle(t *testing.T) {
	t.Parallel()
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	socket := os.NewFile(uintptr(fd), "unconnected output")
	defer func() { _ = socket.Close() }()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), processTestBackstop)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestStandardOutputAliasHelper$", "--", "unconnected-output")
	var diagnostics bytes.Buffer
	cmd.Stdin = bytes.NewReader(nil)
	cmd.Stdout = socket
	cmd.Stderr = &diagnostics
	if err := cmd.Run(); err != nil {
		t.Fatalf("unconnected stdout=%v, stderr=%q; want ENOTCONN preserved and stdout closed", err, diagnostics.String())
	}
}

func TestStandardOutputBrokenPipeReturnsNativeCause(t *testing.T) {
	t.Parallel()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := read.Close(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = write.Close() }()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), processTestBackstop)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestStandardOutputAliasHelper$", "--", "broken-output")
	var diagnostics bytes.Buffer
	cmd.Stdin = bytes.NewReader(nil)
	cmd.Stdout = write
	cmd.Stderr = &diagnostics
	if err := cmd.Run(); err != nil {
		t.Fatalf("broken stdout child=%v, stderr=%q; want preserved EPIPE rather than caller termination", err, diagnostics.String())
	}
}

// Closed-constructor refusal comes from Unix's scoped stdout dup. Other
// platforms use their own ordinary-file path and need not make that observation.
func TestStandardOutputClosedConstructorRetainsNativeCause(t *testing.T) {
	t.Parallel()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), processTestBackstop)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestStandardOutputCopyHelper$", "--", "closed-output")
	var output, diagnostics bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &diagnostics
	if err := cmd.Run(); err != nil || output.Len() != 0 {
		t.Fatalf("closed Unix stdout bytes=%d, child=%v, stderr=%q; want zero capability and exact native RawControl refusal", output.Len(), err, diagnostics.String())
	}
}
