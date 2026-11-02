package process

import (
	"io"

	"github.com/deliri/primitive/v2026/core"
)

// OutputWriter lends one validated output route to a native io.Writer consumer.
// It retains no output, owns no close, and writes through Streams.WriteOutput.
type OutputWriter struct {
	streams Streams
	stream  Stream
}

// OutputWriter selects stdout or stderr without writing to either destination.
func (s Streams) OutputWriter(stream Stream) (OutputWriter, error) {
	writer := OutputWriter{streams: s, stream: stream}
	if err := writer.Validate(); err != nil {
		return OutputWriter{}, err
	}
	return writer, nil
}

// Validate rejects an unset capability, stdin, or an unknown output route.
func (w OutputWriter) Validate() error {
	if err := w.streams.Validate(); err != nil {
		return err
	}
	switch w.stream {
	case StreamStdout, StreamStderr:
		return nil
	default:
		return core.ErrProcessContract
	}
}

// Write preserves the native byte count and typed cause of WriteOutput,
// including partial writes and EPIPE. Empty writes never contact the sink.
func (w OutputWriter) Write(payload []byte) (int, error) {
	written, err := w.streams.WriteOutput(w.stream, payload)
	return int(written.Uint64()), err
}

var _ io.Writer = OutputWriter{}
