package objectstore

import (
	"errors"
	"io"
	"io/fs"

	"github.com/deliri/primitive/v2026/core"
)

// ExactReader delivers exactly the declared extent from a source and proves
// the source held precisely that many bytes, no more and no fewer. It is the
// integrity-bound streaming reader an exact object transfer wraps its source
// in, shared by objectstore's capability transfers and the authenticated GCS
// transfer in gcsobjects. A short or overlong source is a source-integrity
// failure, reachable through Failure after the stream ends.
type ExactReader struct {
	source     io.Reader
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
		source:    source,
		remaining: remaining,
	}, nil
}

// Failure reports the source-integrity error that ended the stream, or nil
// when the source delivered its exact extent. Callers read it after a copy so
// a wrapped tee reader's error can be distinguished from a destination error.
func (r *ExactReader) Failure() error {
	if r == nil || r.source == nil {
		return errors.Join(core.ErrObjectStoreContract, coreSourceIntegrity())
	}
	return r.failure
}

// Read delivers the next bytes, never more than the declared extent remains.
func (r *ExactReader) Read(destination []byte) (int, error) {
	if r == nil || r.source == nil {
		return 0, errors.Join(core.ErrObjectStoreContract, coreSourceIntegrity())
	}
	if r.failure != nil {
		return 0, r.failure
	}
	// A caller supplying no storage has not observed source progress.
	if len(destination) == 0 {
		return 0, nil
	}
	if r.remaining == 0 {
		return 0, io.EOF
	}
	count, err := r.readNext(destination)
	// readNext admits only nonnegative counts within the declared int64 extent.
	// Account exactly the bytes returned, including a prefix beside a native error.
	// #nosec G115 -- readNext rejects negative counts and returns zero on that rejection.
	r.delivered += uint64(count)
	return count, err
}

func (r *ExactReader) readNext(destination []byte) (int, error) {
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
	if readErr != nil {
		return r.fail(count, readErr)
	}
	return count, nil
}

func (r *ExactReader) finish(count int, readErr error) (int, error) {
	// witness:waiver doctrine/error/sentinel_compare -- Objectstore owns this io.Reader boundary. Go requires unwrapped EOF for graceful completion; wrapped/joined failures must survive. Review by 2026-12-08 against Go's io.Reader contract.
	ended := readErr == io.EOF
	if readErr != nil && !ended {
		return r.fail(count, readErr)
	}
	remaining, proven, extentErr := exactSourceRemaining(r.source)
	if extentErr != nil || proven && remaining != 0 {
		return r.fail(0, extentErr)
	}
	if !proven && !ended {
		return r.fail(0, io.ErrNoProgress)
	}
	r.remaining = 0
	r.verified = true
	return count, readErr
}

// ProveEmpty verifies the source is empty when the declared extent is zero:
// a zero-length object still has to prove the source delivered nothing rather
// than being assumed empty without a read.
func (r *ExactReader) ProveEmpty() error {
	if r == nil || r.source == nil {
		return errors.Join(core.ErrObjectStoreContract, coreSourceIntegrity())
	}
	if r.failure != nil {
		return r.failure
	}

	if r.remaining != 0 {
		return coreSourceIntegrity()
	}
	remaining, proven, err := exactSourceRemaining(r.source)
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
		length := measured.Len()
		if length < 0 {
			return 0, true, coreSourceIntegrity()
		}
		remaining, err := core.CheckedUint64FromInt64(int64(length))
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
	if info == nil || position < 0 || position > info.Size() {
		return 0, true, coreSourceIntegrity()
	}
	remaining, conversionErr := core.CheckedUint64FromInt64(info.Size() - position)
	return remaining, true, conversionErr
}

func (r *ExactReader) fail(count int, cause error) (int, error) {
	r.failure = errors.Join(coreSourceIntegrity(), cause)
	return count, r.failure
}
