package gitrepo

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

const (
	gitRevisionParse  = "rev-parse"
	gitRevisionVerify = "--verify"
	gitHeadReference  = "HEAD"
)

type HeadRequest struct{ Root core.AbsolutePath }

func (r HeadRequest) Validate() error { return r.Root.Validate() }

// HeadObservation binds the fixed-shape Git object name to its native receipt.
type HeadObservation struct {
	Commit  core.BuildCommit
	Process process.ResultObservation
}

func (o HeadObservation) Validate() error {
	if err := errors.Join(o.Commit.Validate(), o.Process.Validate()); err != nil {
		return outputError(err)
	}
	if o.Process.ExitCode != 0 || o.Process.StderrBytes.Uint64() != 0 || o.Process.StdoutBytes.Uint64() != uint64(len(o.Commit.String())+1) {
		return core.ErrGitRepositoryOutput
	}
	return nil
}

// ObserveHead drains native output without a process quota. Only the supported
// Git object-name schema occupies working memory; extra bytes are discarded,
// then the completed native receipt refuses that noncanonical document.
func (c Capability) ObserveHead(ctx context.Context, request HeadRequest) (HeadObservation, error) {
	if err := errors.Join(contextstate.Validate(ctx), c.Validate(), request.Validate()); err != nil {
		return HeadObservation{}, contractError(err)
	}
	arguments, err := process.ParseArguments([]string{gitNoOptionalLocks, gitConfigOverride, gitNoFilesystemMonitor, gitRevisionParse, gitRevisionVerify, gitHeadReference})
	if err != nil {
		return HeadObservation{}, contractError(err)
	}
	var output headOutput
	result, err := process.Run(ctx, process.Request{Command: c.command, WorkingDirectory: request.Root, Arguments: arguments, Environment: c.environment, Streams: process.Streams{Stdin: bytes.NewReader(nil), Stdout: &output, Stderr: io.Discard}, OutputPolicy: process.OutputPolicy{Mode: process.OutputModeStreaming}, WaitDelay: c.configuration.WaitDelay, Containment: process.Containment{Isolation: process.IsolationDirect, CancelSignal: process.CancelSignalTerminate}})
	if err != nil {
		return HeadObservation{}, executionError(err)
	}
	if err := requireSuccess(result); err != nil {
		return HeadObservation{}, err
	}
	receipt, err := result.Observation()
	if err != nil {
		return HeadObservation{}, executionError(err)
	}
	if receipt.StderrBytes.Uint64() != 0 || receipt.StdoutBytes.Uint64() != uint64(output.used) || output.used == 0 || output.bytes[output.used-1] != '\n' {
		return HeadObservation{}, core.ErrGitRepositoryOutput
	}
	commit, err := core.ParseBuildCommit(string(output.bytes[:output.used-1]))
	if err != nil {
		return HeadObservation{}, outputError(err)
	}
	observation := HeadObservation{Commit: commit, Process: receipt}
	if err := observation.Validate(); err != nil {
		return HeadObservation{}, err
	}
	return observation, nil
}

// headOutput retains only a Git SHA-256 spelling and its terminating LF.
// The cursor describes this working window, never total stream extent.
type headOutput struct {
	bytes [core.SHA256DigestBytes*2 + 1]byte
	used  int
}

func (o *headOutput) Write(data []byte) (int, error) {
	o.used += copy(o.bytes[o.used:], data)
	return len(data), nil
}
