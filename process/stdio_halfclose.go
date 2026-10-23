package process

import (
	"errors"
	"io"
	"os"

	"github.com/deliri/primitive/v2026/core"
)

// OutputStream owns the calling process's standard output and its scoped
// error-returning writer. Close ends the Unix socket's write direction before
// closing those handles, so an aliased standard input remains readable. Pipes
// and ordinary files use their normal close. The caller closes exactly once;
// repeated close retains the native closed-file cause.
type OutputStream struct {
	file    *os.File
	writer  io.Writer
	release func() error
}

// StandardOutput lends an owned output capability for the calling process.
// StandardStreams stays unchanged. The caller must not separately close or
// replace stdout while this capability is in use. Closing it does not close
// stdin, even when both descriptors name one socket.
func StandardOutput() (OutputStream, error) {
	if os.Stdout == nil {
		return OutputStream{}, core.ErrProcessContract
	}
	writer, release, err := pipeErrorDestination(os.Stdout)
	if err != nil {
		return OutputStream{}, errors.Join(core.ErrProcessStream, err)
	}
	return OutputStream{file: os.Stdout, writer: writer, release: release}, nil
}

func (s OutputStream) Validate() error {
	if s.file == nil || core.WriterIsNil(s.writer) {
		return core.ErrProcessContract
	}
	return nil
}

func (s OutputStream) Write(data []byte) (int, error) {
	if err := s.Validate(); err != nil {
		return 0, err
	}
	count, err := s.writer.Write(data)
	if err != nil {
		return count, errors.Join(core.ErrProcessStream, err)
	}
	return count, nil
}

func (s OutputStream) Close() error {
	if err := s.Validate(); err != nil {
		return err
	}
	shutdownErr := shutdownOutput(s.file)
	var releaseErr error
	if s.release != nil {
		releaseErr = s.release()
	}
	if err := errors.Join(shutdownErr, releaseErr, s.file.Close()); err != nil {
		return errors.Join(core.ErrProcessStream, err)
	}
	return nil
}

var _ io.WriteCloser = OutputStream{}
var _ core.Validatable = OutputStream{}
