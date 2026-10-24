package filestore

import (
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

// RelayCurrentProcess is the terminal entrypoint of a dedicated byte-relay
// process. Its own stdin/stdout are Near; its trusted parent's lent standard
// streams are Far. The parent and child must agree on Request.Inherited as
// required by process.LentStreams. No unrelated work or deferred cleanup may
// belong to this process: once copying begins, this call terminates it through
// process.ExitCurrent and does not return. The parent must reap the process.
//
// Far EOF owns this process's lifetime. Some clients wait for their remote-shell
// child to exit before closing that child's blocking stdin; joining that read
// would deadlock. Process exit ends the still-pending copy without returning
// borrowed work or claiming a joined RelayResult. If the upward copy has already
// completed, its output close is joined and both native causes are preserved.
// Known copy/close failures are written to stderr and produce ExitFailure.
// Invalid setup is refused before copying and returns its typed/native cause.
// Call Relay instead when the caller needs a fully joined library operation.
func RelayCurrentProcess(request process.LentRequest) error {
	if err := request.Validate(); err != nil {
		return errors.Join(core.ErrFilestoreContract, err)
	}
	own, err := process.StandardStreams()
	if err != nil {
		return errors.Join(core.ErrFilestoreContract, err)
	}
	lent, err := process.LentStreams(request)
	if err != nil {
		return errors.Join(core.ErrFilestoreContract, err)
	}
	return relayCurrentProcess(own, lent)
}

// Only RelayCurrentProcess supplies these process-owned handles. Keeping the
// native I/O scope separate lets owned-child tests challenge its failure edges;
// the public door never accepts arbitrary streams or a termination callback.
func relayCurrentProcess(own process.Streams, lent process.Lent) error {
	if err := own.Validate(); err != nil {
		return errors.Join(core.ErrFilestoreContract, err)
	}
	nearOutput, ok := own.Stdout.(io.WriteCloser)
	if !ok {
		return core.ErrFilestoreContract
	}
	if err := (RelayRequest{NearInput: own.Stdin, NearOutput: nearOutput, FarInput: lent.Input, FarOutput: lent.Output}).Validate(); err != nil {
		return err
	}
	upward := make(chan relayUpwardResult, 1)
	go func() {
		_, copyErr := io.Copy(lent.Output, own.Stdin)
		// Publish the known copy cause before its close can release Far EOF.
		upward <- relayUpwardResult{copyError: copyErr}
		upward <- relayUpwardResult{closeError: lent.Output.Close()}
	}()
	_, downErr := io.Copy(currentRelayOutput{streams: own}, lent.Input)
	terminalErr := errors.Join(downErr, nearOutput.Close())
	select {
	case copied := <-upward:
		closed := <-upward
		terminalErr = errors.Join(terminalErr, copied.copyError, closed.closeError)
	default:
		// Its input still belongs to this process, not to a returning caller.
	}
	status := process.ExitSuccess
	if terminalErr != nil {
		status = process.ExitFailure
		_, writeErr := own.WriteOutput(process.StreamStderr, []byte(terminalErr.Error()+"\n"))
		if writeErr != nil {
			return process.ExitCurrent(process.ExitFailure)
		}
	}
	return process.ExitCurrent(status)
}

// currentRelayOutput delegates fd 1 writes to Process's native EPIPE-safe
// output door. A raw os.Stdout.Write could terminate on SIGPIPE before the
// already-known relay causes reach stderr and the explicit exit boundary.
type currentRelayOutput struct {
	streams process.Streams
}

func (output currentRelayOutput) Write(payload []byte) (int, error) {
	written, err := output.streams.WriteOutput(process.StreamStdout, payload)
	if written.Uint64() > uint64(len(payload)) {
		return 0, errors.Join(core.ErrFilestoreContract, err)
	}
	return int(written.Uint64()), err
}

var _ io.Writer = currentRelayOutput{}
