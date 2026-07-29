package filestore

import (
	"context"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

const (
	streamBufferBytes               = 32 << 10
	streamMaximumConsecutiveEmpties = 100
)

type streamDestination uint8

const (
	streamDestinationCaller streamDestination = iota + 1
	streamDestinationFile
)

func copyBounded(
	ctx context.Context,
	destination io.Writer,
	source io.Reader,
	maximum core.ByteCount,
	kind streamDestination,
) (core.ByteLength, error) {
	maximumBytes, err := maximum.Uint64()
	if err != nil {
		return core.ByteLength{}, contractError(err)
	}
	buffer := make([]byte, streamBufferBytes)
	var total uint64
	emptyReads := 0
	for {
		if err := contextstate.Validate(ctx); err != nil {
			return core.NewByteLength(total), err
		}
		limit := nextReadSize(maximumBytes, total)
		count, readErr, validationErr := readBoundedChunk(ctx, source, buffer, limit)
		if validationErr != nil {
			return core.NewByteLength(total), validationErr
		}
		if count > 0 {
			emptyReads = 0
			if total == maximumBytes {
				return core.NewByteLength(total), sizeError(errors.New("filestore source exceeds its maximum byte count"))
			}
			written, writeErr := writeFull(destination, buffer[:count])
			total, writeErr = accountWrite(total, written, writeErr)
			if writeErr != nil {
				return core.NewByteLength(total), classifyDestinationError(kind, writeErr)
			}
		} else {
			emptyReads++
		}
		if readErr != nil {
			return finishRead(total, readErr)
		}
		if emptyReads >= streamMaximumConsecutiveEmpties {
			return core.NewByteLength(total), sourceError(io.ErrNoProgress)
		}
	}
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
	if contextErr := contextstate.Validate(ctx); contextErr != nil {
		return 0, nil, contextErr
	}
	if count < 0 || count > limit {
		return 0, nil, sourceError(errors.New("filestore source returned an invalid byte count"))
	}
	return count, err, nil
}

func nextReadSize(maximum, total uint64) int {
	if total == maximum {
		return 1
	}
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

func finishRead(total uint64, err error) (core.ByteLength, error) {
	if errors.Is(err, io.EOF) {
		return core.NewByteLength(total), nil
	}
	return core.NewByteLength(total), sourceError(err)
}

func classifyDestinationError(kind streamDestination, err error) error {
	if kind == streamDestinationCaller {
		return destinationError(err)
	}
	return activationError(err)
}
