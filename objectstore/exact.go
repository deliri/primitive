package objectstore

import (
	"bufio"
	"errors"
	"io"

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
	source    *bufio.Reader
	failure   error
	remaining int64
	delivered uint64
	verified  bool
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
	if int64(count) < r.remaining {
		r.remaining -= int64(count)
		if addErr := r.addDelivered(count); addErr != nil {
			return r.fail(0, addErr)
		}
		if err != nil {
			return r.fail(count, err)
		}
		return count, nil
	}
	return r.finish(count, err)
}

func (r *ExactReader) finish(count int, readErr error) (int, error) {
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return r.fail(count, readErr)
	}
	_, probeErr := r.source.Peek(1)
	if probeErr == nil || !errors.Is(probeErr, io.EOF) {
		return r.fail(0, probeErr)
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
	_, err := r.source.Peek(1)
	if !errors.Is(err, io.EOF) {
		r.failure = errors.Join(coreSourceIntegrity(), err)
		return r.failure
	}
	r.verified = true
	return nil
}

func (r *ExactReader) fail(count int, cause error) (int, error) {
	r.failure = errors.Join(coreSourceIntegrity(), cause)
	return count, r.failure
}
