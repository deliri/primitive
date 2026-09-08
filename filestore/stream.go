package filestore

import (
	"context"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

const (
	streamSourceOverflowDiagnostic = "filestore source exceeds its maximum byte count"
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

type boundedCopyRequest struct {
	ctx         context.Context
	destination io.Writer
	source      io.Reader
	maximum     core.ByteCount
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
	eof        bool
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
		r.eof = true
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

func copyBounded(request boundedCopyRequest) (core.ByteLength, error) {
	maximum, err := validatedStreamMaximum(request.maximum, request.kind)
	if err != nil {
		return core.ByteLength{}, err
	}
	source := streamReader{ctx: request.ctx, source: request.source}
	destination := streamWriter{destination: request.destination, kind: request.kind}
	// LimitedReader supplies the ceiling and lets Go size its own scratch
	// buffer for small transfers. No maximum+1 arithmetic can overflow.
	total, err := io.Copy(destination, io.LimitReader(&source, maximum))
	if err != nil {
		return finishStream(uint64(total), errors.Join(err, source.cause))
	}
	if !source.eof {
		err = probeBoundedSourceEnd(&source)
	}
	if err == nil && request.extentKnown && uint64(total) < request.knownExtent {
		err = sourceError(io.ErrUnexpectedEOF)
	}
	return finishStream(uint64(total), err)
}

func probeBoundedSourceEnd(source *streamReader) error {
	var probe [1]byte
	count, err := io.ReadFull(source, probe[:])
	if count > 0 {
		// ReadFull normalizes a full read's error. Preserve the independently
		// observed native failure as well as the overflow it revealed.
		return errors.Join(sizeError(errors.New(streamSourceOverflowDiagnostic)), source.cause)
	}
	// witness:waiver doctrine/error/sentinel_compare -- Only Go's unwrapped EOF is clean termination; a joined EOF and native failure must retain Source refusal.
	if err == io.EOF {
		return nil
	}
	return err
}

func validatedStreamMaximum(maximum core.ByteCount, kind streamDestination) (int64, error) {
	if err := kind.Validate(); err != nil {
		return 0, err
	}
	maximumBytes, err := maximum.Int64()
	if err != nil {
		return 0, contractError(err)
	}
	return maximumBytes, nil
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
