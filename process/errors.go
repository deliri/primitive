package process

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

// Stream identifies one process byte stream.
type Stream uint8

const (
	// StreamUnknown is outside the admitted domain.
	StreamUnknown Stream = iota
	// StreamStdin identifies bytes read from the caller for the child.
	StreamStdin
	// StreamStdout identifies child standard output.
	StreamStdout
	// StreamStderr identifies child standard error.
	StreamStderr
)

// Validate rejects values outside the closed stream domain.
func (s Stream) Validate() error {
	if !s.IsValid() {
		return contractError("stream is outside the admitted domain")
	}
	return nil
}

// IsValid reports whether s is admitted.
func (s Stream) IsValid() bool {
	return s >= StreamStdin && s <= StreamStderr
}

// OffWireEnum declares that Stream is not a wire encoding.
func (Stream) OffWireEnum() {}

// String returns the compiler-owned label for s.
func (s Stream) String() string {
	switch s {
	case StreamStdin:
		return "stdin"
	case StreamStdout:
		return "stdout"
	case StreamStderr:
		return "stderr"
	default:
		return unknownEnumLabel
	}
}

// FailureKind identifies the os/exec phase that failed.
type FailureKind uint8

const (
	// FailureKindUnknown is outside the admitted domain.
	FailureKindUnknown FailureKind = iota
	// FailureKindStart identifies failure before the direct child started.
	FailureKindStart
	// FailureKindWait identifies failure while reaping the direct child.
	FailureKindWait
)

// Validate rejects values outside the closed failure-kind domain.
func (k FailureKind) Validate() error {
	if !k.IsValid() {
		return contractError("failure kind is outside the admitted domain")
	}
	return nil
}

// IsValid reports whether k is admitted.
func (k FailureKind) IsValid() bool {
	return k == FailureKindStart || k == FailureKindWait
}

// OffWireEnum declares that FailureKind is not a wire encoding.
func (FailureKind) OffWireEnum() {}

// String returns the compiler-owned label for k.
func (k FailureKind) String() string {
	switch k {
	case FailureKindStart:
		return "start"
	case FailureKindWait:
		return "wait"
	default:
		return unknownEnumLabel
	}
}

func (k FailureKind) identity() core.ErrorIdentity {
	if k == FailureKindStart {
		return core.ErrProcessStart
	}
	return core.ErrProcessWait
}

// Failure preserves one native os/exec failure with its typed phase and
// command.
type Failure struct {
	Cause   error
	Command core.AbsolutePath
	Kind    FailureKind
}

// Error reports the typed phase without replacing the native cause.
func (f Failure) Error() string {
	return f.Kind.String() + " process " + f.Command.String() + ": " + causeText(f.Cause)
}

// Unwrap preserves both the stable identity and native cause.
func (f Failure) Unwrap() []error {
	return []error{f.Kind.identity(), f.Cause}
}

// Validate rejects incomplete failure detail.
func (f Failure) Validate() error {
	if err := f.Kind.Validate(); err != nil {
		return err
	}
	if err := f.Command.Validate(); err != nil {
		return errors.Join(core.ErrProcessContract, err)
	}
	if f.Cause == nil {
		return contractError("process failure cause is nil")
	}
	return nil
}

// StreamFailure preserves one caller stream failure and its native cause.
type StreamFailure struct {
	Cause  error
	Stream Stream
}

// Error reports the failed stream without replacing the native cause.
func (f StreamFailure) Error() string {
	return f.Stream.String() + ": " + causeText(f.Cause)
}

// Unwrap preserves both the stable identity and native cause.
func (f StreamFailure) Unwrap() []error {
	return []error{core.ErrProcessStream, f.Cause}
}

// Validate rejects incomplete stream failure detail.
func (f StreamFailure) Validate() error {
	if err := f.Stream.Validate(); err != nil {
		return err
	}
	if f.Cause == nil {
		return contractError("stream failure cause is nil")
	}
	return nil
}

// OutputLimitExceeded identifies the bounded output stream and exact bound.
type OutputLimitExceeded struct {
	Stream Stream
	Limit  core.ByteCount
}

// Error reports which bounded stream reached its limit.
func (e OutputLimitExceeded) Error() string {
	return e.Stream.String() + ": " + core.ErrProcessOutputLimit.Error()
}

// Unwrap preserves the stable output-limit identity.
func (e OutputLimitExceeded) Unwrap() error {
	return core.ErrProcessOutputLimit
}

// Validate rejects incomplete output-limit detail.
func (e OutputLimitExceeded) Validate() error {
	if err := e.Stream.Validate(); err != nil {
		return err
	}
	if err := e.Limit.Validate(); err != nil {
		return errors.Join(core.ErrProcessContract, err)
	}
	if e.Stream == StreamStdin {
		return contractError("stdin cannot have an output limit")
	}
	return nil
}

func causeText(cause error) string {
	if cause == nil {
		return "missing cause"
	}
	return cause.Error()
}
