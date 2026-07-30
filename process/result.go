package process

import (
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// ExitCode is one observed direct-child exit code. A value of -1 means the
// platform reports that the child did not exit normally.
type ExitCode struct {
	value int
	set   bool
}

func newExitCode(value int) (ExitCode, error) {
	exit := ExitCode{value: value, set: true}
	if err := exit.Validate(); err != nil {
		return ExitCode{}, err
	}
	return exit, nil
}

// Validate rejects an unset code or a value below the os/exec signaled marker.
func (c ExitCode) Validate() error {
	if !c.set {
		return contractError("exit code is unset")
	}
	if c.value < -1 {
		return contractError("exit code is outside the admitted domain")
	}
	return nil
}

// Int returns the exact os/exec exit code.
func (c ExitCode) Int() (int, error) {
	if err := c.Validate(); err != nil {
		return 0, err
	}
	return c.value, nil
}

// Success reports whether the direct child exited normally with code zero.
func (c ExitCode) Success() (bool, error) {
	if err := c.Validate(); err != nil {
		return false, err
	}
	return c.value == 0, nil
}

// Signaled reports whether the platform says the child did not exit normally.
func (c ExitCode) Signaled() (bool, error) {
	if err := c.Validate(); err != nil {
		return false, err
	}
	return c.value == -1, nil
}

// Result contains fixed-size observations from one started and reaped child.
type Result struct {
	exit        ExitCode
	cpu         temporal.Duration
	stdinBytes  core.ByteLength
	stdoutBytes core.ByteLength
	stderrBytes core.ByteLength
	set         bool
}

// Validate rejects the unset zero result.
func (r Result) Validate() error {
	if !r.set {
		return contractError("result is unset")
	}
	if err := r.exit.Validate(); err != nil {
		return err
	}
	return r.cpu.Validate()
}

// ExitCode returns the observed direct-child exit code.
func (r Result) ExitCode() (ExitCode, error) {
	if err := r.Validate(); err != nil {
		return ExitCode{}, err
	}
	return r.exit, nil
}

// CPUTime returns direct-child user and system CPU time.
func (r Result) CPUTime() (temporal.Duration, error) {
	if err := r.Validate(); err != nil {
		return temporal.Duration{}, err
	}
	return r.cpu, nil
}

// StdinBytes returns bytes obtained from caller stdin.
func (r Result) StdinBytes() (core.ByteLength, error) {
	if err := r.Validate(); err != nil {
		return core.ByteLength{}, err
	}
	return r.stdinBytes, nil
}

// StdoutBytes returns child stdout bytes accepted by the caller's writer.
func (r Result) StdoutBytes() (core.ByteLength, error) {
	if err := r.Validate(); err != nil {
		return core.ByteLength{}, err
	}
	return r.stdoutBytes, nil
}

// StderrBytes returns child stderr bytes accepted by the caller's writer.
func (r Result) StderrBytes() (core.ByteLength, error) {
	if err := r.Validate(); err != nil {
		return core.ByteLength{}, err
	}
	return r.stderrBytes, nil
}
