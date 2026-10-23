//go:build unix

package process

import (
	"os"
	"os/exec"
)

// applyInherited lends the caller's own standard streams to the child at
// descriptors 3 onward, in request order. The default arm fails closed.
func applyInherited(command *exec.Cmd, streams []InheritedStream) error {
	for _, stream := range streams {
		switch stream {
		case InheritedStandardInput:
			command.ExtraFiles = append(command.ExtraFiles, os.Stdin)
		case InheritedStandardOutput:
			command.ExtraFiles = append(command.ExtraFiles, os.Stdout)
		default:
			return contractError(inheritedStreamOutsideDomainDiagnostic)
		}
	}
	return nil
}

// openLent names the lent descriptors. It performs no filesystem effect: a
// descriptor that was never lent is closed here, so its first read or write
// fails with the native bad-descriptor cause.
func openLent(input, output uintptr) (Lent, error) {
	lentInput := os.NewFile(input, "inherited standard input")
	lentOutput := os.NewFile(output, "inherited standard output")
	if lentInput == nil || lentOutput == nil {
		return Lent{}, contractError(lentStreamsDiagnostic)
	}
	return Lent{Input: lentInput, Output: lentOutput}, nil
}
