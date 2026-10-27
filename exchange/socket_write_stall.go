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
		w.mutex.Lock()
		defer w.mutex.Unlock()
		w.cancelLocked()
	})
	defer func() {
		if !stop() {
			<-joined
		}
		w.mutex.Lock()
		defer w.mutex.Unlock()
		// An unconsumed final HTTP chunk must still have a finite bound.
		if w.failure == nil {
			w.failure = w.armLocked()
		}
		resultErr = errors.Join(resultErr, w.failure)
	}()
	r.Handler.ServeHTTP(w, r.Call.request)
	return nil
}

type socketStallWriter struct {
	writer   http.ResponseWriter
	ctx      context.Context
	timeout  temporal.Duration
	mutex    sync.Mutex
	failure  error
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
	w.mutex.Lock()
	defer w.mutex.Unlock()
	if w.failure == nil {
		w.failure = w.armLocked()
	}
	return w.failure
}

func (w *socketStallWriter) end(cause error) error {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	if cause != nil {
		w.failure = errors.Join(w.failure, core.ErrExchangeResponse, core.ErrExchangeWrite, cause)
	}
	if w.ctx.Err() != nil {
		w.cancelLocked()
	}
	if w.failure == nil {
		stamp, err := socketWriteDeadline(temporal.Duration{})
		if err != nil {
			w.failure = errors.Join(core.ErrExchangeResponse, core.ErrExchangeWrite, err)
		} else if !stamp.Before(w.deadline) {
			// A buffered native Write may report success after its deadline.
			// Never clear/rearm that expired response for a later Flush.
			w.failure = errors.Join(core.ErrExchangeResponse, core.ErrExchangeWrite, os.ErrDeadlineExceeded)
		}
	}
	if w.failure == nil {
		if err := w.controller().SetWriteDeadline(time.Time{}); err != nil {
			w.failure = errors.Join(core.ErrExchangeResponse, core.ErrExchangeWrite, err)
		}
	}
	return w.failure
}

func (w *socketStallWriter) armLocked() error {
	if w.ctx.Err() != nil {
		w.cancelLocked()
		return w.failure
	}
	deadline, err := socketWriteDeadline(w.timeout)
	if err == nil {
		if parent, ok := w.ctx.Deadline(); ok && parent.Before(deadline) {
			deadline = parent
		}
		err = w.controller().SetWriteDeadline(deadline)
		if err == nil {
			w.deadline = deadline
		}
	}
	if err != nil {
		return errors.Join(core.ErrExchangeResponse, core.ErrExchangeWrite, err)
	}
	return nil
}

func (w *socketStallWriter) cancelLocked() {
	deadline, err := socketWriteDeadline(temporal.Duration{})
	if err == nil {
		err = w.controller().SetWriteDeadline(deadline)
	}
	w.failure = errors.Join(w.failure, core.ErrExchangeResponse, core.ErrExchangeWrite, w.ctx.Err(), context.Cause(w.ctx), err)
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
