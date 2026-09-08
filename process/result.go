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

// Validate rejects unset codes and values outside the portable native domain.
func (c ExitCode) Validate() error {
	if !c.set {
		return contractError("exit code is unset")
	}
	if int64(c.value) < core.ProcessExitCodeSignaled || int64(c.value) > core.ProcessExitCodeMaximum {
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
	return int64(c.value) == core.ProcessExitCodeSuccess, nil
}

// Signaled reports whether the platform says the child did not exit normally.
func (c ExitCode) Signaled() (bool, error) {
	if err := c.Validate(); err != nil {
		return false, err
	}
	return int64(c.value) == core.ProcessExitCodeSignaled, nil
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
	exit               ExitCode
	cpu                temporal.Duration
	stdinBytes         core.ByteLength
	stdoutBytes        core.ByteLength
	stderrBytes        core.ByteLength
	peakMemory         core.ByteLength
	peakMemoryReported bool
	signal             SignalNumber
	signalReported     bool
	set                bool
}

// ResultObservation is the durable, stream-free projection of one reaped
// direct child. It retains exact exit, CPU, byte, and signal facts. A nil
// PeakMemoryBytes means Go did not report that observation, not measured zero.
type ResultObservation struct {
	TerminationSignal *SignalNumber     `json:"termination_signal,omitempty"`
	CPUTime           temporal.Duration `json:"cpu_time_nanoseconds"`
	StdinBytes        core.ByteLength   `json:"stdin_bytes"`
	StdoutBytes       core.ByteLength   `json:"stdout_bytes"`
	StderrBytes       core.ByteLength   `json:"stderr_bytes"`
	PeakMemoryBytes   *core.ByteLength  `json:"peak_memory_bytes,omitempty"`
	ExitCode          int64             `json:"exit_code"`
}

func (o ResultObservation) Validate() error {
	if o.ExitCode < core.ProcessExitCodeSignaled || o.ExitCode > core.ProcessExitCodeMaximum {
		return contractError("result observation exit code is outside the admitted domain")
	}
	if err := errors.Join(o.CPUTime.Validate(), o.StdinBytes.Validate(), o.StdoutBytes.Validate(), o.StderrBytes.Validate()); err != nil {
		return errors.Join(core.ErrProcessContract, err)
	}
	if o.PeakMemoryBytes != nil {
		if err := o.PeakMemoryBytes.Validate(); err != nil {
			return errors.Join(core.ErrProcessContract, err)
		}
	}
	var signal SignalNumber
	if o.TerminationSignal != nil {
		signal = *o.TerminationSignal
	}
	return validateResultSignal(o.ExitCode, signal, o.TerminationSignal != nil)
}

func validateResultSignal(exit int64, signal SignalNumber, reported bool) error {
	if !reported {
		if signal != 0 {
			return contractError("unreported termination signal carries a value")
		}
		return nil
	}
	if exit != core.ProcessExitCodeSignaled {
		return contractError("normally exited result observation carries a termination signal")
	}
	return signal.Validate()
}

// Observation projects the exact durable facts from a validated result.
func (r Result) Observation() (ResultObservation, error) {
	if err := r.Validate(); err != nil {
		return ResultObservation{}, err
	}
	observation := ResultObservation{
		ExitCode: int64(r.exit.value), CPUTime: r.cpu,
		StdinBytes: r.stdinBytes, StdoutBytes: r.stdoutBytes, StderrBytes: r.stderrBytes,
	}
	if r.peakMemoryReported {
		memory := r.peakMemory
		observation.PeakMemoryBytes = &memory
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

// Validate rejects unset results and contradictory exit, signal, usage, or
// stream observations before any fact is projected.
func (r Result) Validate() error {
	if !r.set {
		return contractError("result is unset")
	}
	if err := r.exit.Validate(); err != nil {
		return err
	}
	if err := errors.Join(r.cpu.Validate(), r.peakMemory.Validate(), r.stdinBytes.Validate(), r.stdoutBytes.Validate(), r.stderrBytes.Validate()); err != nil {
		return errors.Join(core.ErrProcessContract, err)
	}
	if !r.peakMemoryReported && r.peakMemory.Uint64() != 0 {
		return contractError("unreported peak memory carries a value")
	}
	return validateResultSignal(int64(r.exit.value), r.signal, r.signalReported)
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

// PeakMemoryBytes returns the peak resident set in Go's reaped-child usage
// record. Hosts whose Go wait result has no RSS return ErrProcessUnsupported;
// the result's other exit and stream observations remain available.
func (r Result) PeakMemoryBytes() (core.ByteLength, error) {
	if err := r.Validate(); err != nil {
		return core.ByteLength{}, err
	}
	if !r.peakMemoryReported {
		return core.ByteLength{}, core.ErrProcessUnsupported
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
	if err := r.Validate(); err != nil {
		return 0, err
	}
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
