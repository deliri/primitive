package process

import (
	"context"
	"errors"
	"io"
	"math"
	"sync"

	"github.com/deliri/primitive/v2026/core"
)

const streamCount = int(StreamStderr) + 1

type streamFailures struct {
	cancel context.CancelCauseFunc
	values [streamCount]error
	mu     sync.Mutex
}

func (f *streamFailures) record(stream Stream, cause error) {
	failure := newStreamFailure(stream, cause)
	f.mu.Lock()
	if f.values[stream] == nil {
		f.values[stream] = failure
	}
	f.mu.Unlock()
	f.cancel(failure)
}

func (f *streamFailures) joined() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return errors.Join(
		f.values[StreamStdin],
		f.values[StreamStdout],
		f.values[StreamStderr],
	)
}

// commandStreams are the three typed stream projections for one direct child.
// The two output writers share one mutex so a caller that supplies the same
// writer for stdout and stderr observes serialized, exactly accounted writes.
type commandStreams struct {
	stdin  *observedReader
	stdout *boundedWriter
	stderr *boundedWriter
}

func newCommandStreams(
	request Request,
	failures *streamFailures,
) commandStreams {
	outputMu := &sync.Mutex{}
	return commandStreams{
		stdin: &observedReader{
			source:   request.Streams.Stdin,
			failures: failures,
		},
		stdout: &boundedWriter{
			destination: request.Streams.Stdout,
			failures:    failures,
			writeMu:     outputMu,
			limit:       request.OutputLimit,
			stream:      StreamStdout,
		},
		stderr: &boundedWriter{
			destination: request.Streams.Stderr,
			failures:    failures,
			writeMu:     outputMu,
			limit:       request.OutputLimit,
			stream:      StreamStderr,
		},
	}
}

type observedReader struct {
	source   io.Reader
	failures *streamFailures
	count    uint64
}

func (r *observedReader) Read(buffer []byte) (count int, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = errors.New("stdin reader panicked")
			count = 0
			r.failures.record(StreamStdin, err)
		}
	}()
	count, err = r.source.Read(buffer)
	if count < 0 || count > len(buffer) {
		violation := errors.New("stdin reader returned an invalid byte count")
		if err != nil {
			violation = errors.Join(violation, err)
		}
		r.failures.record(StreamStdin, violation)
		return 0, violation
	}
	if count > 0 {
		if countErr := r.observeCount(count); countErr != nil {
			return 0, countErr
		}
	}
	if err != nil && !errors.Is(err, io.EOF) {
		r.failures.record(StreamStdin, err)
	}
	return count, err
}

func (r *observedReader) observeCount(count int) error {
	increment := uint64(count) // #nosec G115 -- Read proved count is positive and bounded by len(buffer).
	if increment > math.MaxInt64-r.count {
		r.failures.record(StreamStdin, core.ErrNumericOverflow)
		return core.ErrNumericOverflow
	}
	r.count += increment
	return nil
}

type boundedWriter struct {
	destination io.Writer
	failures    *streamFailures
	writeMu     *sync.Mutex
	limit       core.ByteCount
	count       uint64
	stream      Stream
}

func (w *boundedWriter) Write(buffer []byte) (count int, err error) {
	w.writeMu.Lock()
	defer w.writeMu.Unlock()
	defer func() {
		if recover() != nil {
			err = errors.New(w.stream.String() + " writer panicked")
			count = 0
			w.failures.record(w.stream, err)
		}
	}()
	maximum, limitErr := w.limit.Uint64()
	if limitErr != nil {
		w.failures.record(w.stream, limitErr)
		return 0, limitErr
	}
	available := maximum - w.count
	if uint64(len(buffer)) <= available {
		return w.forward(buffer)
	}
	// This branch proves available < len(buffer), so the uint64 index is
	// representable by the platform int that already bounds the slice.
	count, err = w.forward(buffer[:available])
	if err != nil {
		return count, err
	}
	exceeded := newOutputLimitExceeded(w.stream, w.limit)
	w.failures.record(w.stream, exceeded)
	return count, exceeded
}

func (w *boundedWriter) forward(buffer []byte) (int, error) {
	count, err := forwardFullWrite(w.destination, &w.count, buffer)
	if err != nil {
		w.failures.record(w.stream, err)
	}
	return count, err
}
