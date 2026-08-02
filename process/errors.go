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
	streamLimit
)

func streamDiagnostics() [streamLimit]string {
	return [streamLimit]string{
		StreamUnknown: unknownEnumLabel,
		StreamStdin:   "stdin",
		StreamStdout:  "stdout",
		StreamStderr:  "stderr",
	}
}

// Validate rejects values outside the closed stream domain.
func (s Stream) Validate() error {
	if !s.IsValid() {
		return contractError("stream is outside the admitted domain")
	}
	return nil
}

// IsValid reports whether s is admitted.
func (s Stream) IsValid() bool {
	diagnostics := streamDiagnostics()
	return s > StreamUnknown && s < streamLimit && diagnostics[s] != ""
}

// OffWireEnum declares that Stream is not a wire encoding.
func (Stream) OffWireEnum() {}

// String returns the compiler-owned label for s.
func (s Stream) String() string {
	diagnostics := streamDiagnostics()
	if s < streamLimit && diagnostics[s] != "" {
		return diagnostics[s]
	}
	return unknownEnumLabel
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
	failureKindLimit
)

func failureKindDiagnostics() [failureKindLimit]string {
	return [failureKindLimit]string{
		FailureKindUnknown: unknownEnumLabel,
		FailureKindStart:   "start",
		FailureKindWait:    "wait",
	}
}

func failureKindIdentities() [failureKindLimit]core.ErrorIdentity {
	return [failureKindLimit]core.ErrorIdentity{
		FailureKindStart: core.ErrProcessStart,
		FailureKindWait:  core.ErrProcessWait,
	}
}

// Validate rejects values outside the closed failure-kind domain.
func (k FailureKind) Validate() error {
	if !k.IsValid() {
		return contractError("failure kind is outside the admitted domain")
	}
	return nil
}

// IsValid reports whether k is admitted.
func (k FailureKind) IsValid() bool {
	diagnostics := failureKindDiagnostics()
	identities := failureKindIdentities()
	return k > FailureKindUnknown &&
		k < failureKindLimit &&
		diagnostics[k] != "" &&
		identities[k] != core.ErrUnknown
}

// OffWireEnum declares that FailureKind is not a wire encoding.
func (FailureKind) OffWireEnum() {}

// String returns the compiler-owned label for k.
func (k FailureKind) String() string {
	diagnostics := failureKindDiagnostics()
	if k < failureKindLimit && diagnostics[k] != "" {
		return diagnostics[k]
	}
	return unknownEnumLabel
}

func (k FailureKind) identity() core.ErrorIdentity {
	identities := failureKindIdentities()
	if k < failureKindLimit && identities[k] != core.ErrUnknown {
		return identities[k]
	}
	return core.ErrProcessContract
}

// Failure is a sealed Process report that preserves one native os/exec
// failure with its typed phase and command.
type Failure interface {
	error
	Validate() error
	Kind() FailureKind
	Command() core.AbsolutePath
	Cause() error
	processFailure()
}

type failure struct {
	cause   error
	command core.AbsolutePath
	kind    FailureKind
}

func newFailure(kind FailureKind, command core.AbsolutePath, cause error) error {
	value := failure{kind: kind, command: command, cause: cause}
	if err := value.Validate(); err != nil {
		return err
	}
	return value
}

// Error reports the typed phase without replacing the native cause.
func (f failure) Error() string {
	if f.Validate() != nil {
		return core.ErrProcessContract.Error()
	}
	return f.kind.String() + " process " + f.command.String() + ": " + f.cause.Error()
}

// Unwrap preserves the stable identity and native cause only for a complete
// Process-built report.
func (f failure) Unwrap() []error {
	if f.Validate() != nil {
		return []error{core.ErrProcessContract}
	}
	return []error{f.kind.identity(), f.cause}
}

// Validate rejects incomplete failure detail.
func (f failure) Validate() error {
	if err := f.kind.Validate(); err != nil {
		return err
	}
	if err := f.command.Validate(); err != nil {
		return errors.Join(core.ErrProcessContract, err)
	}
	if f.cause == nil {
		return contractError("process failure cause is nil")
	}
	return nil
}

// Kind returns the exact os/exec phase that failed.
func (f failure) Kind() FailureKind { return f.kind }

// Command returns the exact command that failed.
func (f failure) Command() core.AbsolutePath { return f.command }

// Cause returns the preserved native os/exec failure.
func (f failure) Cause() error { return f.cause }

func (failure) processFailure() {}

// StreamFailure is a sealed Process report that preserves one caller stream
// failure and its native cause.
type StreamFailure interface {
	error
	Validate() error
	Stream() Stream
	Cause() error
	processStreamFailure()
}

type streamFailure struct {
	cause  error
	stream Stream
}

func newStreamFailure(stream Stream, cause error) error {
	value := streamFailure{stream: stream, cause: cause}
	if err := value.Validate(); err != nil {
		return err
	}
	return value
}

// Error reports the failed stream without replacing the native cause.
func (f streamFailure) Error() string {
	if f.Validate() != nil {
		return core.ErrProcessContract.Error()
	}
	return f.stream.String() + ": " + f.cause.Error()
}

// Unwrap preserves both identities only for a complete Process-built report.
func (f streamFailure) Unwrap() []error {
	if f.Validate() != nil {
		return []error{core.ErrProcessContract}
	}
	return []error{core.ErrProcessStream, f.cause}
}

// Validate rejects incomplete stream failure detail.
func (f streamFailure) Validate() error {
	if err := f.stream.Validate(); err != nil {
		return err
	}
	if f.cause == nil {
		return contractError("stream failure cause is nil")
	}
	return nil
}

// Stream returns the exact caller stream that failed.
func (f streamFailure) Stream() Stream { return f.stream }

// Cause returns the preserved caller stream failure.
func (f streamFailure) Cause() error { return f.cause }

func (streamFailure) processStreamFailure() {}

// OutputLimitExceeded is a sealed Process report that identifies the bounded
// output stream and exact bound.
type OutputLimitExceeded interface {
	error
	Validate() error
	Stream() Stream
	Limit() core.ByteCount
	processOutputLimitExceeded()
}

type outputLimitExceeded struct {
	stream Stream
	limit  core.ByteCount
}

func newOutputLimitExceeded(stream Stream, limit core.ByteCount) error {
	value := outputLimitExceeded{stream: stream, limit: limit}
	if err := value.Validate(); err != nil {
		return err
	}
	return value
}

// Error reports which bounded stream reached its limit.
func (e outputLimitExceeded) Error() string {
	if e.Validate() != nil {
		return core.ErrProcessContract.Error()
	}
	return e.stream.String() + ": " + core.ErrProcessOutputLimit.Error()
}

// Unwrap preserves the stable output-limit identity only for a complete
// Process-built report.
func (e outputLimitExceeded) Unwrap() error {
	if e.Validate() != nil {
		return core.ErrProcessContract
	}
	return core.ErrProcessOutputLimit
}

// Validate rejects incomplete output-limit detail.
func (e outputLimitExceeded) Validate() error {
	if err := e.stream.Validate(); err != nil {
		return err
	}
	if err := validateOutputLimit(e.limit); err != nil {
		return err
	}
	if e.stream == StreamStdin {
		return contractError("stdin cannot have an output limit")
	}
	return nil
}

// Stream returns the exact output stream that exceeded its bound.
func (e outputLimitExceeded) Stream() Stream { return e.stream }

// Limit returns the exact bound that was exceeded.
func (e outputLimitExceeded) Limit() core.ByteCount { return e.limit }

func (outputLimitExceeded) processOutputLimitExceeded() {}

var (
	_ Failure             = failure{}
	_ StreamFailure       = streamFailure{}
	_ OutputLimitExceeded = outputLimitExceeded{}
	_ core.Validatable    = failure{}
	_ core.Validatable    = streamFailure{}
	_ core.Validatable    = outputLimitExceeded{}
)
