package background

import (
	"context"
	"errors"
	"sync"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// WallGapWatchRequest supplies the prior native instant and probe cadence.
// The watcher observes wall-clock gaps; it assigns no suspend or failure meaning.
type WallGapWatchRequest struct {
	Parent   context.Context
	Start    temporal.Instant
	Interval temporal.Duration
}

func (r WallGapWatchRequest) Validate() error {
	parentErr := contextstate.Validate(r.Parent)
	if errors.Is(parentErr, context.Canceled) || errors.Is(parentErr, context.DeadlineExceeded) {
		parentErr = errors.Join(parentErr, context.Cause(r.Parent))
	}
	return errors.Join(parentErr, r.Start.Validate(), (temporal.TickerRequest{Interval: r.Interval}).Validate())
}

// WallGapSnapshot retains only the greatest positive gap. Equal gaps keep the
// first observation. A backwards clock correction resets the next baseline.
type WallGapSnapshot struct {
	Maximum  temporal.Interval
	Observed bool
}

func (s WallGapSnapshot) Validate() error {
	if !s.Observed {
		if s.Maximum != (temporal.Interval{}) {
			return core.ErrBackgroundContract
		}
		return nil
	}
	if err := s.Maximum.Validate(); err != nil {
		return err
	}
	elapsed, err := s.Maximum.Elapsed()
	if err != nil {
		return err
	}
	if elapsed.IsZero() {
		return core.ErrBackgroundContract
	}
	return nil
}

// WallGapWatch owns one standard-library ticker, goroutine and cancellation
// lifetime. Snapshot borrows no mutable state. Close joins before returning.
// The capability must not be copied; concurrent snapshots and closes are safe.
type WallGapWatch struct {
	self     *WallGapWatch
	task     *Task
	mu       sync.Mutex
	previous temporal.Instant
	snapshot WallGapSnapshot
}

func OpenWallGapWatch(request WallGapWatchRequest) (*WallGapWatch, error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	ticker, err := temporal.OpenTicker(temporal.TickerRequest{Interval: request.Interval})
	if err != nil {
		return nil, err
	}
	watch := &WallGapWatch{previous: request.Start}
	watch.self = watch
	task, err := StartTask(TaskRequest{Parent: request.Parent, Work: Work{
		Run:      func(ctx context.Context) error { return watch.run(ctx, ticker) },
		Complete: func(error) { ticker.Stop() },
	}})
	if err != nil {
		ticker.Stop()
		return nil, err
	}
	watch.task = task

	return watch, nil
}

func (w *WallGapWatch) Validate() error {
	if w == nil || w.self != w || w.task == nil {
		return core.ErrBackgroundContract
	}
	return w.task.Validate()
}

func (w *WallGapWatch) Snapshot() (WallGapSnapshot, error) {
	if err := w.Validate(); err != nil {
		return WallGapSnapshot{}, err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.snapshot, nil
}

// Close returns the native task observation only after sampling and ticker
// cleanup have joined. Execution cancellation stays separate from join failure.
func (w *WallGapWatch) Close() (TaskResult, error) {
	if err := w.Validate(); err != nil {
		return TaskResult{}, err
	}
	return w.task.Close()
}

func (w *WallGapWatch) run(ctx context.Context, ticker *temporal.Ticker) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.Ticks():
			observation, err := temporal.Observe()
			if err != nil {
				return err
			}
			instant, err := observation.Instant()
			if err != nil {
				return err
			}
			if err := w.observe(instant); err != nil {
				return err
			}
		}
	}
}

func (w *WallGapWatch) observe(now temporal.Instant) error {
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
	interval, err := temporal.IntervalFromBounds(temporal.IntervalBounds{Start: w.previous, End: now})
	if err != nil {
		return err
	}
	w.previous = now
	elapsed, err := interval.Elapsed()
	if err != nil {
		return err
	}
	maximum := temporal.Duration{}
	if w.snapshot.Observed {
		maximum, err = w.snapshot.Maximum.Elapsed()
		if err != nil {
			return err
		}
	}
	if !w.snapshot.Observed || elapsed.Nanoseconds() > maximum.Nanoseconds() {
		w.snapshot = WallGapSnapshot{Maximum: interval, Observed: true}
	}
	return nil
}
