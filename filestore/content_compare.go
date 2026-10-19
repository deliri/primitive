package filestore

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// CompareContentRequest lends a source and a synchronous expected-content
// producer. Neither stream is retained or closed. Expected borrows its writer
// only for the callback; it must return any encoding failure.
type CompareContentRequest struct {
	Source   io.Reader
	Expected func(context.Context, io.Writer) error
}

func (r CompareContentRequest) Validate() error {
	if core.ReaderIsNil(r.Source) || r.Expected == nil {
		return core.ErrFilestoreContract
	}
	return nil
}

// ContentComparison observes exact bytes and both complete extents. Equal is
// byte equality, not digest equality. A failure returns no comparison proof.
type ContentComparison struct {
	ExpectedBytes core.ByteLength
	ObservedBytes core.ByteLength
	Equal         bool
}

func (c ContentComparison) Validate() error {
	if err := errors.Join(c.ExpectedBytes.Validate(), c.ObservedBytes.Validate()); err != nil {
		return err
	}
	if c.Equal && c.ExpectedBytes != c.ObservedBytes {
		return core.ErrFilestoreContract
	}
	return nil
}

// CompareContent compares a streamed projection against a borrowed source in
// forward order with one fixed working window. It consumes trailing source
// bytes even after a mismatch, so both extents describe complete streams. Go
// owns reads and byte comparison; the existing stream boundary preserves native
// errors, cancellation, invalid read counts and lack of progress.
func CompareContent(ctx context.Context, request CompareContentRequest) (ContentComparison, error) {
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return ContentComparison{}, err
	}
	comparison := contentComparisonWriter{source: streamReader{ctx: ctx, source: request.Source}, equal: true, active: true}
	defer func() { comparison.active = false }()
	if err := request.Expected(ctx, &comparison); err != nil {
		return ContentComparison{}, err
	}
	if comparison.failure != nil {
		return ContentComparison{}, comparison.failure
	}
	if !comparison.ended {
		length, err := copyStream(streamCopyRequest{ctx: ctx, source: &comparison.source, destination: io.Discard, buffer: comparison.buffer[:], kind: streamDestinationCaller})
		if err != nil {
			return ContentComparison{}, err
		}
		comparison.observed += length.Uint64()
	}
	expected, expectedErr := core.NewByteLength(comparison.expected)
	observed, observedErr := core.NewByteLength(comparison.observed)
	if err := errors.Join(contextstate.Validate(ctx), comparison.source.cause, expectedErr, observedErr); err != nil {
		return ContentComparison{}, err
	}
	result := ContentComparison{ExpectedBytes: expected, ObservedBytes: observed, Equal: comparison.equal && expected == observed}
	return result, result.Validate()
}

type contentComparisonWriter struct {
	source   streamReader
	failure  error
	buffer   [32 << 10]byte
	expected uint64
	observed uint64
	equal    bool
	ended    bool
	active   bool
}

func (w *contentComparisonWriter) Write(content []byte) (int, error) {
	if !w.active {
		return 0, io.ErrClosedPipe
	}
	if w.failure != nil {
		return 0, w.failure
	}
	if err := contextstate.Validate(w.source.ctx); err != nil {
		w.failure = err
		return 0, err
	}
	if _, err := core.NewByteLength(w.expected + uint64(len(content))); err != nil {
		w.failure = sizeError(err)
		return 0, w.failure
	}
	w.expected += uint64(len(content))
	consumed := 0
	for consumed < len(content) && !w.ended {
		chunk := content[consumed : consumed+min(len(content)-consumed, len(w.buffer))]
		count, err := io.ReadFull(&w.source, w.buffer[:len(chunk)])
		w.observed += uint64(count)
		if w.source.cause != nil {
			w.failure = w.source.cause
			return consumed + count, w.failure
		}
		if !bytes.Equal(chunk, w.buffer[:count]) {
			w.equal = false
		}
		consumed += len(chunk)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			w.ended = true
		} else if err != nil {
			w.failure = sourceError(err)
			return consumed, w.failure
		}
	}
	if w.ended && consumed < len(content) {
		w.equal = false
	}
	return len(content), nil
}
