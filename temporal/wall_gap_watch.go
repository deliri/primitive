package temporal

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// WallGapWatchRequest supplies the prior native instant and probe cadence.
// The watcher observes wall-clock gaps; it assigns no suspend or failure meaning.
type WallGapWatchRequest struct {
	Parent   context.Context
	Start    Instant
	Interval Duration
}

func (r WallGapWatchRequest) Validate() error {
	parentErr := contextstate.Validate(r.Parent)
	if errors.Is(parentErr, context.Canceled) || errors.Is(parentErr, context.DeadlineExceeded) {
		parentErr = errors.Join(parentErr, context.Cause(r.Parent))
	}
	return errors.Join(parentErr, r.Start.Validate(), (TickerRequest{Interval: r.Interval}).Validate())
}

// WallGapSnapshot retains only the greatest positive gap. Equal gaps keep the
// first observation. A backwards clock correction resets the next baseline.
type WallGapSnapshot struct {
	Maximum  Interval
	Observed bool
}

func (s WallGapSnapshot) Validate() error {
	if !s.Observed {
		if s.Maximum != (Interval{}) {
			return core.ErrTemporalContract
		}
		return nil
	}
	if err := s.Maximum.Validate(); err != nil {
		return err
	}
	if s.Maximum.elapsed.IsZero() {
		return core.ErrTemporalContract
	}
	return nil
}

// WallGapWatch owns one standard-library ticker, goroutine and cancellation
// lifetime. Snapshot borrows no mutable state. Close joins before returning.
// The capability must not be copied; concurrent snapshots and closes are safe.
type WallGapWatch struct {
	self         *WallGapWatch
	parent       context.Context
	ctx          context.Context
	cancel       context.CancelFunc
	done         chan struct{}
	mu           sync.Mutex
	previous     Instant
	snapshot     WallGapSnapshot
	executionErr error
}

func OpenWallGapWatch(request WallGapWatchRequest) (*WallGapWatch, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	ticker, err := OpenTicker(TickerRequest{Interval: request.Interval})
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(request.Parent)
	watch := &WallGapWatch{parent: request.Parent, ctx: ctx, cancel: cancel, done: make(chan struct{}), previous: request.Start}
	watch.self = watch
	go watch.run(ticker)
	return watch, nil
}

func (w *WallGapWatch) Validate() error {
	if w == nil || w.self != w || w.parent == nil || w.ctx == nil || w.cancel == nil || w.done == nil {
		return core.ErrTemporalContract
	}
	return nil
}

func (w *WallGapWatch) Snapshot() (WallGapSnapshot, error) {
	if err := w.Validate(); err != nil {
		return WallGapSnapshot{}, err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.snapshot, nil
}

// Close's error reports an observation failure. Ordinary owner cancellation
// has no observation failure; a parent cancellation retains its native cause.
func (w *WallGapWatch) Close() error {
	if err := w.Validate(); err != nil {
		return err
	}
	w.cancel()
	<-w.done
	return w.executionErr
}

func (w *WallGapWatch) run(ticker *Ticker) {
	defer close(w.done)
	defer ticker.Stop()
	for {
		select {
		case <-w.ctx.Done():
			w.executionErr = errors.Join(w.parent.Err(), context.Cause(w.parent))
			return
		case <-ticker.Ticks():
			instant, err := NewInstant(time.Now())
			if err == nil {
				err = w.observe(instant)
			}
			if err != nil {
				w.executionErr = err
				return
			}
		}
	}
}

func (w *WallGapWatch) observe(now Instant) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	comparison, err := now.Compare(w.previous)
	if err != nil {
		return err
	}
	if comparison != core.ComparisonGreater {
		w.previous = now
		return nil
	}
	interval, err := IntervalFromBounds(IntervalBounds{Start: w.previous, End: now})
	if err != nil {
		return err
	}
	w.previous = now
	if !w.snapshot.Observed || interval.elapsed.nanoseconds > w.snapshot.Maximum.elapsed.nanoseconds {
		w.snapshot = WallGapSnapshot{Maximum: interval, Observed: true}
	}
	return nil
}
