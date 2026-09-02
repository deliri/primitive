package filestore

import (
	"context"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

const (
	streamBufferBytes              = 32 << 10
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

type remainingByteReader interface {
	Len() int
}

func copyBounded(request boundedCopyRequest) (core.ByteLength, error) {
	maximumBytes, err := validatedStreamMaximum(request.maximum, request.kind)
	if err != nil {
		return core.ByteLength{}, err
	}
	buffer := make([]byte, streamBufferBytes)
	var total uint64
	emptyReads := 0
	for {
		if err := contextstate.Validate(request.ctx); err != nil {
			return finishStream(total, err)
		}
		if total == maximumBytes {
			return finishStream(total, proveBoundedSourceEnd(request, maximumBytes))
		}
		limit := nextReadSize(maximumBytes, total)
		count, readErr, validationErr := readBoundedChunk(request.ctx, request.source, buffer, limit)
		if count > 0 {
			emptyReads = 0
			written, writeErr := writeFull(request.destination, buffer[:count])
			total, writeErr = accountWrite(total, written, writeErr)
			if writeErr != nil {
				return finishStream(total, classifyDestinationError(request.kind, writeErr))
			}
		} else {
			emptyReads++
		}
		if readErr != nil {
			return finishRead(request, total, readErr)
		}
		if validationErr != nil {
			return finishStream(total, validationErr)
		}
		if emptyReads >= core.ReaderConsecutiveEmptyReadMaximum {
			return finishStream(total, sourceError(io.ErrNoProgress))
		}
	}
}

func proveBoundedSourceEnd(request boundedCopyRequest, maximum uint64) error {
	if request.extentKnown {
		return probeBoundedSourceEnd(request)
	}
	remaining, ok := request.source.(remainingByteReader)
	if ok {
		return classifyRemainingBytes(int64(remaining.Len()))
	}
	if limited, ok := request.source.(*io.LimitedReader); ok {
		if limited.N == 0 {
			return nil
		}
		return sizeError(errors.New(streamSourceOverflowDiagnostic))
	}
	if section, ok := request.source.(*io.SectionReader); ok {
		position, err := section.Seek(0, io.SeekCurrent)
		if err != nil {
			return sourceError(err)
		}
		return classifyRemainingBytes(section.Size() - position)
	}
	return sourceError(errors.New("filestore source end cannot be proven without blocking"))
}

func probeBoundedSourceEnd(request boundedCopyRequest) error {
	var probe [1]byte
	count, readErr, validationErr := readBoundedChunk(request.ctx, request.source, probe[:], len(probe))
	if count > 0 {
		return errors.Join(sizeError(errors.New(streamSourceOverflowDiagnostic)), readErr)
	}
	if readErr != nil {
		if errors.Is(readErr, io.EOF) {
			return nil
		}
		return sourceError(readErr)
	}
	if validationErr != nil {
		return validationErr
	}
	return sourceError(errors.New("filestore source end cannot be proven after an empty read"))
}

func classifyRemainingBytes(remaining int64) error {
	if remaining == 0 {
		return nil
	}
	if remaining < 0 {
		return sourceError(errors.New("filestore source reported a negative remaining extent"))
	}
	return sizeError(errors.New(streamSourceOverflowDiagnostic))
}

func validatedStreamMaximum(
	maximum core.ByteCount,
	kind streamDestination,
) (uint64, error) {
	if err := kind.Validate(); err != nil {
		return 0, err
	}
	maximumBytes, err := maximum.Uint64()
	if err != nil {
		return 0, contractError(err)
	}
	return maximumBytes, nil
}

func accountWrite(total uint64, written int, err error) (uint64, error) {
	if written < 0 {
		return total, io.ErrShortWrite
	}
	return total + uint64(written), err
}

func readBoundedChunk(
	ctx context.Context,
	source io.Reader,
	buffer []byte,
	limit int,
) (int, error, error) {
	count, err := source.Read(buffer[:limit])
	if count < 0 || count > limit {
		return 0, nil, sourceError(errors.New("filestore source returned an invalid byte count"))
	}
	if err != nil {
		return count, err, nil
	}
	if contextErr := contextstate.Validate(ctx); contextErr != nil {
		return count, nil, contextErr
	}
	return count, err, nil
}

func nextReadSize(maximum, total uint64) int {
	remaining := maximum - total
	if remaining < streamBufferBytes {
		return int(remaining)
	}
	return streamBufferBytes
}

func writeFull(destination io.Writer, data []byte) (int, error) {
	total := 0
	for len(data) > 0 {
		count, err := destination.Write(data)
		if count < 0 || count > len(data) {
			return total, io.ErrShortWrite
		}
		total += count
		data = data[count:]
		if err != nil {
			return total, err
		}
		if count == 0 {
			return total, io.ErrShortWrite
		}
	}
	return total, nil
}

func finishRead(request boundedCopyRequest, total uint64, err error) (core.ByteLength, error) {
	if errors.Is(err, io.EOF) {
		if request.extentKnown && total < request.knownExtent {
			return finishStream(total, sourceError(io.ErrUnexpectedEOF))
		}
		return finishStream(total, nil)
	}
	return finishStream(total, sourceError(err))
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
