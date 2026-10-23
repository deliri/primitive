package process

import (
	"io"

	"github.com/deliri/primitive/v2026/core"
)

// InheritedStream names one of the calling process's own standard streams
// lent to a child as an additional descriptor.
//
// A relay needs a child to reach the caller's standard streams while the
// child's own standard streams belong to someone else: rsync gives its remote
// shell child a private socket as stdin and stdout, so a bridge that must
// reach the SSH session around it can only find that session on a lent
// descriptor. Only the caller's own standard streams can be lent; no other
// descriptor can cross this boundary. The child sees the lent streams at
// descriptors 3 onward, in request order.
type InheritedStream uint8

const (
	// InheritedStreamUnknown is outside the admitted domain.
	InheritedStreamUnknown InheritedStream = iota
	// InheritedStandardInput lends the caller's standard input.
	InheritedStandardInput
	// InheritedStandardOutput lends the caller's standard output.
	InheritedStandardOutput
	inheritedStreamLimit
)

// InheritedStreamMaximum bounds one request: each standard stream at most once.
const InheritedStreamMaximum = 2

// inheritedDescriptorBase is the child's first descriptor after stdin,
// stdout and stderr, where os/exec places lent files in order.
const inheritedDescriptorBase = 3

func inheritedStreamDiagnostics() [inheritedStreamLimit]string {
	return [...]string{
		InheritedStandardInput:  "standard input",
		InheritedStandardOutput: "standard output",
	}
}

const (
	inheritedStreamOutsideDomainDiagnostic = "inherited stream is outside the admitted domain"
	inheritedStreamRepeatedDiagnostic      = "inherited stream is lent twice"
	inheritedStreamCountDiagnostic         = "more inherited streams than standard streams"
	lentStreamsDiagnostic                  = "needs exactly the lent standard input and output"
)

// Validate rejects values outside the closed inherited-stream domain.
func (s InheritedStream) Validate() error {
	if !s.IsValid() {
		return contractError(inheritedStreamOutsideDomainDiagnostic)
	}
	return nil
}

// IsValid reports whether s is admitted.
func (s InheritedStream) IsValid() bool {
	diagnostics := inheritedStreamDiagnostics()
	return s > InheritedStreamUnknown && s < inheritedStreamLimit && diagnostics[s] != ""
}

// OffWireEnum declares that InheritedStream is not a wire encoding.
func (InheritedStream) OffWireEnum() {}

// String returns the compiler-owned label for s.
func (s InheritedStream) String() string {
	diagnostics := inheritedStreamDiagnostics()
	if s < inheritedStreamLimit && diagnostics[s] != "" {
		return diagnostics[s]
	}
	return core.UnknownEnumDiagnostic
}

// validateInherited admits at most one lending of each standard stream.
func validateInherited(streams []InheritedStream) error {
	if len(streams) > InheritedStreamMaximum {
		return contractError(inheritedStreamCountDiagnostic)
	}
	var seen [inheritedStreamLimit]bool
	for _, stream := range streams {
		if err := stream.Validate(); err != nil {
			return err
		}
		if seen[stream] {
			return contractError(inheritedStreamRepeatedDiagnostic)
		}
		seen[stream] = true
	}
	return nil
}

// LentRequest names, in the order the parent lent them, the standard streams
// this process inherited at descriptors 3 onward.
type LentRequest struct {
	Inherited []InheritedStream
}

// Validate admits exactly the lent standard input and output, in either order.
func (r LentRequest) Validate() error {
	if err := validateInherited(r.Inherited); err != nil {
		return err
	}
	if len(r.Inherited) != InheritedStreamMaximum {
		return contractError(lentStreamsDiagnostic)
	}
	return nil
}

// descriptors returns the inherited descriptor numbers carrying the parent's
// standard input and standard output.
func (r LentRequest) descriptors() (uintptr, uintptr) {
	input, output := uintptr(inheritedDescriptorBase), uintptr(inheritedDescriptorBase+1)
	if r.Inherited[0] == InheritedStandardOutput {
		input, output = output, input
	}
	return input, output
}

// Lent is the parent's standard input and output as this process received
// them. The caller owns both and closes them.
type Lent struct {
	Input  io.ReadCloser
	Output io.WriteCloser
}

// LentStreams names the parent's lent standard streams in a child the parent
// started with Request.Inherited. The trusted parent and child must agree on
// that inheritance and its order. Descriptor numbers alone cannot establish
// their origin: unrelated open descriptors may occupy the same numbers.
// It performs no filesystem effect and does not close unrelated descriptors.
// An absent descriptor fails its first I/O with the native bad-descriptor cause.
func LentStreams(request LentRequest) (Lent, error) {
	if err := request.Validate(); err != nil {
		return Lent{}, err
	}
	input, output := request.descriptors()
	return openLent(input, output)
}
