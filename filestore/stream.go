package filestore

import (
	"context"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

type streamDestination uint8

const (
	streamDestinationUnknown streamDestination = iota
	streamDestinationCaller
	streamDestinationFile
	streamDestinationLimit
)

func streamDestinationDiagnostics() [streamDestinationLimit]string {
	return [streamDestinationLimit]string{
		streamDestinationCaller: "caller",
		streamDestinationFile:   "file",
	}
}

func (d streamDestination) Validate() error {
	if !d.IsValid() {
		return contractError(errors.New("filestore stream destination is invalid"))
	}
	return nil
}

func (d streamDestination) IsValid() bool {
	return d > streamDestinationUnknown && d < streamDestinationLimit &&
		streamDestinationDiagnostics()[d] != ""
}

func (d streamDestination) String() string {
	if !d.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return streamDestinationDiagnostics()[d]
}

func (streamDestination) OffWireEnum() {}

type streamCopyRequest struct {
	buffer      []byte
	ctx         context.Context
	destination io.Writer
	source      io.Reader

	knownExtent uint64
	kind        streamDestination
	extentKnown bool
}

// streamReader validates one caller read; Go owns the copy and read-full loops.
// It deliberately exposes only io.Reader so optional transfer methods cannot
// bypass context, count, progress, or native-error validation.
type streamReader struct {
	ctx        context.Context
	source     io.Reader
	cause      error
	emptyReads int
}

func (r *streamReader) Read(buffer []byte) (int, error) {
	if err := contextstate.Validate(r.ctx); err != nil {
		r.cause = err
		return 0, err
	}
	count, err := r.source.Read(buffer)
	if count < 0 || count > len(buffer) {
		r.cause = sourceError(errors.Join(errors.New("filestore source returned an invalid byte count"), err))
		return 0, r.cause
	}
	return count, r.observe(count, err)
}

func (r *streamReader) observe(count int, err error) error {
	// witness:waiver doctrine/error/sentinel_compare -- Only Go's unwrapped EOF is clean termination; a joined EOF and native failure must retain Source refusal.
	if err == io.EOF {
		return io.EOF
	}
	if err != nil {
		r.cause = sourceError(err)
		return r.cause
	}
	if err := contextstate.Validate(r.ctx); err != nil {
		r.cause = err
		return err
	}
	return r.observeProgress(count)
}

func (r *streamReader) observeProgress(count int) error {
	if count > 0 {
		r.emptyReads = 0
		return nil
	}
	r.emptyReads++
	if r.emptyReads >= core.ReaderConsecutiveEmptyReadMaximum {
		r.cause = sourceError(io.ErrNoProgress)
		return r.cause
	}
	return nil
}

// streamWriter preserves Go's single-write accounting and adds the destination
// identity. It exposes no ReaderFrom shortcut around the validated reader.
type streamWriter struct {
	destination io.Writer
	kind        streamDestination
}

func (w streamWriter) Write(buffer []byte) (int, error) {
	count, err := w.destination.Write(buffer)
	if count < 0 || count > len(buffer) {
		return 0, classifyDestinationError(w.kind, errors.Join(io.ErrShortWrite, err))
	}
	if err == nil && count != len(buffer) {
		err = io.ErrShortWrite
	}
	if err != nil {
		return count, classifyDestinationError(w.kind, err)
	}
	return count, nil
}

// copyStream delegates transfer loops to Go with borrowed or Go-allocated scratch.
// The observed file extent detects truncation; it never caps accepted growth.
func copyStream(request streamCopyRequest) (core.ByteLength, error) {
	if err := request.kind.Validate(); err != nil {
		return core.ByteLength{}, err
	}
	source := streamReader{ctx: request.ctx, source: request.source}
	destination := streamWriter{destination: request.destination, kind: request.kind}
	buffer := request.buffer
	if len(buffer) == 0 {
		buffer = nil
	}
	total, err := io.CopyBuffer(destination, &source, buffer)
	if err != nil {
		return finishStream(uint64(total), errors.Join(err, source.cause))
	}
	if request.extentKnown && uint64(total) < request.knownExtent {
		err = sourceError(io.ErrUnexpectedEOF)
	}
	return finishStream(uint64(total), err)
}

func finishStream(total uint64, cause error) (core.ByteLength, error) {
	length, err := core.NewByteLength(total)
	return length, errors.Join(cause, err)
}

func classifyDestinationError(kind streamDestination, err error) error {
	if validationErr := kind.Validate(); validationErr != nil {
		return errors.Join(validationErr, err)
	}
	switch kind {
	case streamDestinationCaller:
		return destinationError(err)
	case streamDestinationFile:
		return activationError(err)
	default:
		return contractError(err)
	}
}

var (
	_ core.Validatable = streamDestinationUnknown
	_ core.OffWireEnum = streamDestinationUnknown
)
