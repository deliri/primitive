//go:build !windows

package shutdown

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"
)

const (
	nativeHelperEnvironment = "PRIMITIVE_SHUTDOWN_NATIVE_HELPER"
	nativeHelperReady       = "ready"
)

type nativeSignalCase struct {
	name   string
	signal syscall.Signal
	want   SignalKind
}

func TestWatchNativeUnixLifecycle(t *testing.T) {
	if os.Getenv(nativeHelperEnvironment) != "" {
		// This branch owns a process-wide signal handler and is the child-process
		// entry point, so it cannot call t.Parallel.
		runNativeWatchHelper(t)
		return
	}
	t.Parallel()

	cases := []nativeSignalCase{
		{name: "interrupt", signal: syscall.SIGINT, want: SignalKindInterrupt},
		{name: "terminate", signal: syscall.SIGTERM, want: SignalKindTerminate},
		{name: "hangup", signal: syscall.SIGHUP, want: SignalKindHangup},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runNativeSignalCase(t, tc)
		})
	}
}

func runNativeSignalCase(t *testing.T, tc nativeSignalCase) {
	t.Helper()

	command := exec.CommandContext(
		t.Context(), os.Args[0], "-test.run=^TestWatchNativeUnixLifecycle$",
	)
	command.Env = append(os.Environ(), nativeHelperEnvironment+"=1")
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe() error = %v", err)
	}
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() || scanner.Text() != nativeHelperReady {
		t.Fatalf("helper readiness = %q error:%v, want %q",
			scanner.Text(), scanner.Err(), nativeHelperReady)
	}
	if err := command.Process.Signal(tc.signal); err != nil {
		t.Fatalf("Signal(%s) error = %v", tc.want, err)
	}
	if !scanner.Scan() || scanner.Text() != tc.want.String() {
		t.Fatalf("helper signal = %q error:%v, want %q",
			scanner.Text(), scanner.Err(), tc.want)
	}
	if err := command.Wait(); err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
}

func runNativeWatchHelper(t *testing.T) {
	t.Helper()

	controller, err := Watch(WatchRequest{
		Parent: context.Background(),
		Policy: defaultSignalPolicy(),
		Set:    SignalSetTerminalLifecycle,
	})
	if err != nil {
		t.Fatalf("Watch() error = %v", err)
	}
	if _, err := fmt.Fprintln(os.Stdout, nativeHelperReady); err != nil {
		t.Fatalf("write readiness error = %v", err)
	}
	controllerContext := controller.Context()
	if controllerContext == nil {
		t.Fatal("Controller.Context() = nil, want owned context")
	}
	<-controllerContext.Done()
	var cause SignalCause
	if !errors.As(context.Cause(controllerContext), &cause) ||
		cause.Validate() != nil {
		t.Fatalf("Watch() cause = %v, want authentic signal",
			context.Cause(controllerContext))
	}
	if _, err := fmt.Fprintln(os.Stdout, cause.Kind()); err != nil {
		t.Fatalf("write signal error = %v", err)
	}
	if err := controller.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
