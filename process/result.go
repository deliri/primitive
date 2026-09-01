package process

import (
	"errors"

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

// SignalNumber is the exact platform signal that ended one signaled child,
// held for durable execution records. It is an observation of what happened
// to the child, distinct from the CancelSignal a caller intended.
type SignalNumber int32

// Validate rejects the zero and negative values no delivered signal carries.
func (n SignalNumber) Validate() error {
	if n <= 0 {
		return contractError("signal number is outside the admitted domain")
	}
	return nil
}

// Int returns the signal number for a durable record or diagnostic.
func (n SignalNumber) Int() (int, error) {
	if err := n.Validate(); err != nil {
		return 0, err
	}
	return int(n), nil
}

// Result contains fixed-size observations from one started and reaped child.
type Result struct {
	exit           ExitCode
	cpu            temporal.Duration
	stdinBytes     core.ByteLength
	stdoutBytes    core.ByteLength
	stderrBytes    core.ByteLength
	peakMemory     core.ByteLength
	signal         SignalNumber
	signalReported bool
	set            bool
}

// ResultObservation is the durable, stream-free projection of one reaped
// direct child. It retains exact exit, CPU, byte, and signal facts.
type ResultObservation struct {
	ExitCode          int32             `json:"exit_code"`
	CPUTime           temporal.Duration `json:"cpu_time_nanoseconds"`
	StdinBytes        core.ByteLength   `json:"stdin_bytes"`
	StdoutBytes       core.ByteLength   `json:"stdout_bytes"`
	StderrBytes       core.ByteLength   `json:"stderr_bytes"`
	PeakMemoryBytes   core.ByteLength   `json:"peak_memory_bytes"`
	TerminationSignal *SignalNumber     `json:"termination_signal,omitempty"`
}

func (o ResultObservation) Validate() error {
	if o.ExitCode < -1 {
		return contractError("result observation exit code is outside the admitted domain")
	}
	if err := errors.Join(o.CPUTime.Validate(), o.StdinBytes.Validate(), o.StdoutBytes.Validate(), o.StderrBytes.Validate(), o.PeakMemoryBytes.Validate()); err != nil {
		return errors.Join(core.ErrProcessContract, err)
	}
	if o.ExitCode == -1 {
		if o.TerminationSignal != nil {
			return o.TerminationSignal.Validate()
		}
		return nil
	}
	if o.TerminationSignal != nil {
		return contractError("normally exited result observation carries a termination signal")
	}
	return nil
}

// Observation projects the exact durable facts from a validated result.
func (r Result) Observation() (ResultObservation, error) {
	if err := r.Validate(); err != nil {
		return ResultObservation{}, err
	}
	exit, err := r.exit.Int()
	if err != nil {
		return ResultObservation{}, err
	}
	exitCode, err := core.CheckedInt32FromInt(exit)
	if err != nil {
		return ResultObservation{}, errors.Join(core.ErrProcessContract, err)
	}
	observation := ResultObservation{
		ExitCode: exitCode, CPUTime: r.cpu,
		StdinBytes: r.stdinBytes, StdoutBytes: r.stdoutBytes, StderrBytes: r.stderrBytes, PeakMemoryBytes: r.peakMemory,
	}
	if r.signalReported {
		signal := r.signal
		observation.TerminationSignal = &signal
	}
	if err := observation.Validate(); err != nil {
		return ResultObservation{}, err
	}
	return observation, nil
}

// Validate rejects the unset zero result.
func (r Result) Validate() error {
	if !r.set {
		return contractError("result is unset")
	}
	if err := r.exit.Validate(); err != nil {
		return err
	}
	return errors.Join(r.cpu.Validate(), r.peakMemory.Validate())
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

// PeakMemoryBytes returns the maximum resident set observed for the reaped
// child process tree by the host kernel.
func (r Result) PeakMemoryBytes() (core.ByteLength, error) {
	if err := r.Validate(); err != nil {
		return core.ByteLength{}, err
	}
	return r.peakMemory, nil
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

// TerminationSignal returns the exact signal that ended a signaled child.
//
// Only a signaled child has one, and only a platform that names it reports
// one. A normally exited child is refused rather than answered with zero,
// which a durable record would store as a real signal.
func (r Result) TerminationSignal() (SignalNumber, error) {
	signaled, err := r.exit.Signaled()
	if err != nil {
		return 0, err
	}
	if !signaled {
		return 0, contractError("a normally exited child has no termination signal")
	}
	if !r.signalReported {
		return 0, contractError("this platform reports no termination signal")
	}
	if err := r.signal.Validate(); err != nil {
		return 0, err
	}
	return r.signal, nil
}
