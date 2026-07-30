package objectstore

import (
	"bufio"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/core"
)

const exactExtentBufferBytes = 32 * 1024

type exactReader struct {
	source    *bufio.Reader
	failure   error
	remaining int64
	delivered uint64
	verified  bool
}

func newExactReader(source io.Reader, length int64) *exactReader {
	return &exactReader{
		source:    bufio.NewReaderSize(source, exactExtentBufferBytes),
		remaining: length,
	}
}

func (r *exactReader) Read(destination []byte) (int, error) {
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

func (r *exactReader) finish(count int, readErr error) (int, error) {
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

func (r *exactReader) addDelivered(count int) error {
	value, err := core.CheckedUint64FromInt64(int64(count))
	if err != nil {
		return err
	}
	r.delivered += value
	return nil
}

func (r *exactReader) proveEmpty() error {
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

func (r *exactReader) fail(count int, cause error) (int, error) {
	r.failure = errors.Join(coreSourceIntegrity(), cause)
	return count, r.failure
}
