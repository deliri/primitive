package exchange

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// SocketWriteStallRequest lends one HTTP call to one handler. Timeout bounds
// active socket writes, not quiet computation or the handler's total lifetime.
// The caller owns that lifetime through Call's context. The handler must not
// independently change response deadlines or retain the writer after returning.
type SocketWriteStallRequest struct {
	Call    SocketServerCall
	Handler http.Handler
	Timeout temporal.Duration
}

// Validate admits the call, handler and positive write bound without invoking
// the handler. Native deadline support is checked by ServeSocketWithWriteStall.
func (r SocketWriteStallRequest) Validate() error {
	if err := errors.Join(r.Call.validateWrite(), r.Timeout.Validate()); err != nil {
		return errors.Join(core.ErrExchangeContract, err)
	}
	if core.IsNil(r.Handler) || r.Timeout.IsZero() {
		return core.ErrExchangeContract
	}
	return nil
}

// ServeSocketWithWriteStall serves the original request exactly once. It clears
// the server's whole-response deadline during quiet work, bounds each Write and
// Flush, and interrupts a blocked write when the call is canceled. Failed writes
// are never rearmed. Its cancellation callback joins before return; a finite
// deadline remains for net/http's final framing/flush after the handler returns.
// Other routes and ServerRuntimePolicy are unchanged. This owns no retries,
// buffers, request decoding or execution timeout.
func ServeSocketWithWriteStall(r SocketWriteStallRequest) (resultErr error) {
	if err := r.Validate(); err != nil {
		return err
	}
	w := &socketStallWriter{writer: r.Call.writer, ctx: r.Call.request.Context(), timeout: r.Timeout}
	// This is both the native capability gate and removal of the inherited
	// whole-response deadline. No handler/header/body effects precede it.
	if err := w.controller().SetWriteDeadline(time.Time{}); err != nil {
		return errors.Join(core.ErrExchangeResponse, core.ErrExchangeWrite, err)
	}
	joined := make(chan struct{})
	stop := context.AfterFunc(w.ctx, func() {
		defer close(joined)
		w.cancel()
	})
	defer func() {
		if !stop() {
			<-joined
		}
		// An unconsumed final HTTP chunk must still have a finite bound.
		if w.failure() == nil {
			_ = w.arm()
		}
		resultErr = errors.Join(resultErr, w.failure())
	}()
	r.Handler.ServeHTTP(w, r.Call.request)
	return nil
}

type socketStallWriter struct {
	writer     http.ResponseWriter
	ctx        context.Context
	timeout    temporal.Duration
	mutex      sync.Mutex // Protects failure; never spans an opaque writer call.
	failureErr error
	// Only the serial handler and its final cleanup access deadline.
	deadline time.Time
}

func (w *socketStallWriter) controller() *http.ResponseController {
	return http.NewResponseController(w.writer)
}

func (w *socketStallWriter) Header() http.Header { return w.writer.Header() }

// Unwrap preserves Go's ResponseController capability discovery. FlushError
// above that seam retains this scope's write deadline ownership.
func (w *socketStallWriter) Unwrap() http.ResponseWriter { return w.writer }

func (w *socketStallWriter) WriteHeader(status int) {
	if w.begin() != nil {
		return
	}
	w.writer.WriteHeader(status)
	_ = w.end(nil)
}

func (w *socketStallWriter) Write(data []byte) (int, error) {
	if err := w.begin(); err != nil {
		return 0, err
	}
	// No mutex spans a blocking native write: cancellation can expire it.
	n, err := w.writer.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	return n, w.end(err)
}

func (w *socketStallWriter) FlushError() error {
	if err := w.begin(); err != nil {
		return err
	}
	return w.end(w.controller().Flush())
}

func (w *socketStallWriter) Flush() { _ = w.FlushError() }

func (w *socketStallWriter) begin() error {
	if err := w.failure(); err != nil {
		return err
	}
	return w.arm()
}

func (w *socketStallWriter) end(cause error) error {
	w.recordFailure(cause)
	if w.ctx.Err() != nil {
		w.cancel()
	}
	if w.failure() == nil {
		stamp, err := socketWriteDeadline(temporal.Duration{})
		if err != nil {
			w.recordFailure(err)
		} else if !stamp.Before(w.deadline) {
			// A buffered native Write may report success after its deadline.
			// Never clear/rearm that expired response for a later Flush.
			w.recordFailure(os.ErrDeadlineExceeded)
		}
	}
	if w.failure() == nil {
		return w.setDeadline(time.Time{})
	}
	return w.failure()
}

func (w *socketStallWriter) arm() error {
	if w.ctx.Err() != nil {
		w.cancel()
		return w.failure()
	}
	deadline, err := socketWriteDeadline(w.timeout)
	if err != nil {
		w.recordFailure(err)
		return w.failure()
	}
	if parent, ok := w.ctx.Deadline(); ok && parent.Before(deadline) {
		deadline = parent
	}
	if err := w.setDeadline(deadline); err != nil {
		return err
	}
	w.deadline = deadline
	return nil
}

// A pending arm or clear can return after cancellation's setter. Recheck only
// after that opaque call has finished, then expire it again: cancellation must
// remain terminal even when the earlier setter applied its deadline last.
func (w *socketStallWriter) setDeadline(deadline time.Time) error {
	w.recordFailure(w.controller().SetWriteDeadline(deadline))
	if w.ctx.Err() != nil {
		w.cancel()
	}
	return w.failure()
}

func (w *socketStallWriter) cancel() {
	deadline, err := socketWriteDeadline(temporal.Duration{})
	// Publish refusal before opaque I/O, so an in-flight setter cannot prevent
	// another operation from observing the caller's terminal cancellation.
	w.recordFailure(errors.Join(w.ctx.Err(), context.Cause(w.ctx), err))
	if err == nil {
		w.recordFailure(w.controller().SetWriteDeadline(deadline))
	}
}

func (w *socketStallWriter) recordFailure(cause error) {
	if cause == nil {
		return
	}
	w.mutex.Lock()
	defer w.mutex.Unlock()
	w.failureErr = errors.Join(w.failureErr, core.ErrExchangeResponse, core.ErrExchangeWrite, cause)
}

func (w *socketStallWriter) failure() error {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	return w.failureErr
}

func socketWriteDeadline(after temporal.Duration) (time.Time, error) {
	observed, err := temporal.Observe()
	if err != nil {
		return time.Time{}, err
	}
	instant, err := observed.Instant()
	if err != nil {
		return time.Time{}, err
	}
	deadline, err := instant.Add(after)
	if err != nil {
		return time.Time{}, err
	}
	return deadline.Time()
}
