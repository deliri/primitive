package process

import (
	"context"
	"errors"
	"sync"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// MemoryCapState distinguishes unavailable evidence from an actual in-lifetime
// sample. Observed with zero counters is NOT proof of no OOM over the lifetime.
type MemoryCapState uint8

const (
	MemoryCapUnknown MemoryCapState = iota
	MemoryCapObserved
)

// MemoryCapObservation is a scope-local kernel observation with the exact
// enforced limit verified. LocalOOMEvents > 0 positively identifies allocation
// failure at this scope's own limit. Kills alone may have another OOM origin.
// Missing or collected scopes remain unknown; capture is not guaranteed.
type MemoryCapObservation struct {
	Limit          core.ByteLength `json:"limit_bytes"`
	State          MemoryCapState  `json:"state"`
	LocalOOMEvents uint64          `json:"local_oom_events"`
	LocalOOMKills  uint64          `json:"local_oom_kills"`
}

func validateMemoryCap(o *MemoryCapObservation) error {
	if o == nil {
		return nil
	}
	if err := o.Limit.Validate(); err != nil {
		return errors.Join(core.ErrProcessContract, err)
	}
	if o.Limit.Uint64() == 0 {
		return contractError("memory cap observation has no limit")
	}
	switch o.State {
	case MemoryCapUnknown:
		if o.LocalOOMEvents != 0 || o.LocalOOMKills != 0 {
			return contractError("unknown memory cap carries counters")
		}
	case MemoryCapObserved:
	default:
		return contractError("memory cap observation state is invalid")
	}
	return nil
}

type memoryCapUnit string

type memoryCapOwner struct {
	unit  memoryCapUnit
	pid   ProcessIdentity
	limit core.ByteLength
}

// memoryCapObserver owns one bounded native sampler task; finish cancels and
// joins it before publishing a snapshot. It never changes the native outcome.
type memoryCapObserver struct {
	owner       memoryCapOwner
	cancel      context.CancelFunc
	done        chan struct{}
	mu          sync.Mutex
	observation MemoryCapObservation
}

func newMemoryCapObserver(parent context.Context, owner memoryCapOwner) *memoryCapObserver {
	if owner.unit == "" || owner.limit.Uint64() == 0 {
		return nil
	}
	ctx, cancel := context.WithCancel(parent)
	o := &memoryCapObserver{owner: owner, cancel: cancel, done: make(chan struct{}), observation: MemoryCapObservation{Limit: owner.limit}}
	go o.run(ctx)
	return o
}
func (o *memoryCapObserver) run(ctx context.Context) {
	defer close(o.done)
	defer func() {
		o.mu.Lock()
		defer o.mu.Unlock()
		if o.observation.LocalOOMEvents == 0 {
			o.observation = MemoryCapObservation{Limit: o.owner.limit}
		}
	}()
	interval, err := temporal.DurationFromMilliseconds(10)
	if err != nil {
		return
	}
	ticker, err := temporal.OpenTicker(temporal.TickerRequest{Interval: interval})
	if err != nil {
		return
	}
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		observed := observeMemoryCap(o.owner)
		o.mu.Lock()
		// A positive local event is immutable evidence, even if later collection or
		// cancellation makes another read impossible. No zero/no-OOM claim is kept.
		if o.observation.LocalOOMEvents == 0 {
			o.observation = observed
		}
		positive := o.observation.LocalOOMEvents > 0
		o.mu.Unlock()
		if positive {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.Ticks():
		}
	}
}
func (o *memoryCapObserver) snapshot() *MemoryCapObservation {
	if o == nil {
		return nil
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	value := o.observation
	return &value
}
func (o *memoryCapObserver) finish() *MemoryCapObservation {
	if o == nil {
		return nil
	}
	o.cancel()
	<-o.done
	return o.snapshot()
}
