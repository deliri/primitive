package objectstore

import (
	"bufio"
	"errors"
	"io"
	"io/fs"

	"github.com/deliri/primitive/v2026/core"
)

const exactExtentBufferBytes = 32 * 1024

// ExactReader delivers exactly the declared extent from a source and proves
// the source held precisely that many bytes, no more and no fewer. It is the
// integrity-bound streaming reader an exact object transfer wraps its source
// in, shared by objectstore's capability transfers and the authenticated GCS
// transfer in gcsobjects. A short or overlong source is a source-integrity
// failure, reachable through Failure after the stream ends.
type ExactReader struct {
	input      io.Reader
	source     *bufio.Reader
	failure    error
	remaining  int64
	delivered  uint64
	emptyReads int
	verified   bool
}

// NewExactReader wraps source to deliver exactly length bytes. The reader
// never trusts the source to stop on its own.
func NewExactReader(source io.Reader, length core.ByteLength) (*ExactReader, error) {
	if core.ReaderIsNil(source) {
		return nil, errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreSource)
	}
	remaining, err := length.Int64()
	if err != nil {
		return nil, errors.Join(core.ErrObjectStoreContract, core.ErrObjectStoreSize, err)
	}
	return &ExactReader{
		input:     source,
		source:    bufio.NewReaderSize(source, exactExtentBufferBytes),
		remaining: remaining,
	}, nil
}

// Failure reports the source-integrity error that ended the stream, or nil
// when the source delivered its exact extent. Callers read it after a copy so
// a wrapped tee reader's error can be distinguished from a destination error.
func (r *ExactReader) Failure() error {
	return r.failure
}

// Read delivers the next bytes, never more than the declared extent remains.
func (r *ExactReader) Read(destination []byte) (int, error) {
	if r.failure != nil {
		return 0, r.failure
	}
	if r.remaining == 0 {
		return 0, io.EOF
	}
	if int64(len(destination)) > r.remaining {
		destination = destination[:r.remaining]
	}
	count, err := r.source.Read(destination)
	if count < 0 || count > len(destination) {
		return r.fail(0, coreSourceIntegrity())
	}
	if count == 0 && err == nil {
		return r.recordEmptyRead()
	}
	r.emptyReads = 0
	if int64(count) == r.remaining {
		return r.finish(count, err)
	}
	return r.continueRead(count, err)
}

func (r *ExactReader) recordEmptyRead() (int, error) {
	r.emptyReads++
	if r.emptyReads >= core.ReaderConsecutiveEmptyReadMaximum {
		return r.fail(0, io.ErrNoProgress)
	}
	return 0, nil
}

func (r *ExactReader) continueRead(count int, readErr error) (int, error) {
	r.remaining -= int64(count)
	if err := r.addDelivered(count); err != nil {
		return r.fail(0, err)
	}
	if readErr != nil {
		return r.fail(count, readErr)
	}
	return count, nil
}

func (r *ExactReader) finish(count int, readErr error) (int, error) {
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return r.fail(count, readErr)
	}
	if r.source.Buffered() != 0 {
		return r.fail(0, nil)
	}
	remaining, proven, extentErr := exactSourceRemaining(r.input)
	if extentErr != nil || proven && remaining != 0 {
		return r.fail(0, extentErr)
	}
	if !proven && !errors.Is(readErr, io.EOF) {
		return r.fail(0, io.ErrNoProgress)
	}
	r.remaining = 0
	if err := r.addDelivered(count); err != nil {
		return r.fail(0, err)
	}
	r.verified = true
	return count, readErr
}

func (r *ExactReader) addDelivered(count int) error {
	value, err := core.CheckedUint64FromInt64(int64(count))
	if err != nil {
		return err
	}
	r.delivered += value
	return nil
}

// ProveEmpty verifies the source is empty when the declared extent is zero:
// a zero-length object still has to prove the source delivered nothing rather
// than being assumed empty without a read.
func (r *ExactReader) ProveEmpty() error {
	if r.remaining != 0 {
		return coreSourceIntegrity()
	}
	if r.source.Buffered() != 0 {
		r.failure = coreSourceIntegrity()
		return r.failure
	}
	remaining, proven, err := exactSourceRemaining(r.input)
	if err != nil || !proven || remaining != 0 {
		if err == nil && !proven {
			err = io.ErrNoProgress
		}
		r.failure = errors.Join(coreSourceIntegrity(), err)
		return r.failure
	}
	r.verified = true
	return nil
}

type exactRemainingSource interface {
	RemainingBytes() uint64
}

type exactLengthSource interface {
	Len() int
}

type exactFileSource interface {
	io.Seeker
	Stat() (fs.FileInfo, error)
}

func exactSourceRemaining(source io.Reader) (uint64, bool, error) {
	if exact, ok := source.(exactRemainingSource); ok {
		return exact.RemainingBytes(), true, nil
	}
	if measured, ok := source.(exactLengthSource); ok {
		if measured.Len() < 0 {
			return 0, true, coreSourceIntegrity()
		}
		remaining, err := core.CheckedUint64FromInt64(int64(measured.Len()))
		return remaining, true, err
	}
	if section, ok := source.(*io.SectionReader); ok {
		position, err := section.Seek(0, io.SeekCurrent)
		if err != nil || position < 0 || position > section.Size() {
			return 0, true, errors.Join(coreSourceIntegrity(), err)
		}
		remaining, conversionErr := core.CheckedUint64FromInt64(section.Size() - position)
		return remaining, true, conversionErr
	}
	if file, ok := source.(exactFileSource); ok {
		return exactFileRemaining(file)
	}
	return 0, false, nil
}

func exactFileRemaining(file exactFileSource) (uint64, bool, error) {
	position, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, true, err
	}
	info, err := file.Stat()
	if err != nil {
		return 0, true, err
	}
	if position < 0 || position > info.Size() {
		return 0, true, coreSourceIntegrity()
	}
	remaining, conversionErr := core.CheckedUint64FromInt64(info.Size() - position)
	return remaining, true, conversionErr
}

func (r *ExactReader) fail(count int, cause error) (int, error) {
	r.failure = errors.Join(coreSourceIntegrity(), cause)
	return count, r.failure
}
