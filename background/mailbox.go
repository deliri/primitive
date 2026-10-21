package background

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// MailboxOverflow selects native buffer handling, never a lifetime item quota.
type MailboxOverflow uint8

const (
	MailboxOverflowPreserveQueued MailboxOverflow = iota + 1
	MailboxOverflowReplaceOldest
)

func (m MailboxOverflow) Validate() error {
	switch m {
	case MailboxOverflowPreserveQueued, MailboxOverflowReplaceOldest:
		return nil
	default:
		return core.ErrBackgroundContract
	}
}

type MailboxDisposition uint8

const (
	MailboxAccepted MailboxDisposition = iota + 1
	MailboxDiscarded
	MailboxReplaced
)

func (d MailboxDisposition) Validate() error {
	switch d {
	case MailboxAccepted, MailboxDiscarded, MailboxReplaced:
		return nil
	default:
		return core.ErrBackgroundContract
	}
}

// ReceiveAll observes native messages or ticks. ReceiveTicks borrows only the
// tick source, allowing a consumer to retain one current item without draining
// the input into a second pending population.
type MailboxReadMode uint8

const (
	MailboxReceiveAll MailboxReadMode = iota + 1
	MailboxReceiveTicks
)

func (m MailboxReadMode) Validate() error {
	switch m {
	case MailboxReceiveAll, MailboxReceiveTicks:
		return nil
	default:
		return core.ErrBackgroundContract
	}
}

type MailboxReceptionKind uint8

const (
	MailboxEmpty MailboxReceptionKind = iota + 1
	MailboxItem
	MailboxTick
)

func (k MailboxReceptionKind) Validate() error {
	switch k {
	case MailboxEmpty, MailboxItem, MailboxTick:
		return nil
	default:
		return core.ErrBackgroundContract
	}
}

type MailboxRequest struct{ Capacity int }

func (r MailboxRequest) Validate() error {
	if r.Capacity <= 0 {
		return core.ErrBackgroundContract
	}
	return nil
}

type MailboxOffer[T any] struct {
	Value    T
	Overflow MailboxOverflow
}

func (r MailboxOffer[T]) Validate() error { return r.Overflow.Validate() }

type MailboxOfferObservation struct{ Disposition MailboxDisposition }

func (o MailboxOfferObservation) Validate() error { return o.Disposition.Validate() }

type MailboxReceiveRequest struct {
	Ticks <-chan time.Time
	Mode  MailboxReadMode
}

func (r MailboxReceiveRequest) Validate() error {
	if err := r.Mode.Validate(); err != nil {
		return err
	}
	if r.Mode == MailboxReceiveTicks && r.Ticks == nil {
		return core.ErrBackgroundContract
	}
	return nil
}

// A reception is one native selection. The kind owns which field is meaningful;
// Empty is a successful nonblocking observation, never a fabricated item.
type MailboxReception[T any] struct {
	Value T
	Tick  time.Time
	Kind  MailboxReceptionKind
}

func (o MailboxReception[T]) Validate() error { return o.Kind.Validate() }

// Mailbox owns Go's buffered channel and serializes competing producers only
// while offering. There is no goroutine, workflow, input history or close flag.
// A consumer stops through its own context. Capacity is resident buffering;
// any number of offers may pass through the same native buffer.
type Mailbox[T any] struct {
	self   *Mailbox[T]
	native chan T
	writer sync.Mutex
}

func OpenMailbox[T any](request MailboxRequest) (*Mailbox[T], error) {
	if err := request.Validate(); err != nil {
		return nil, err
	}
	mailbox := &Mailbox[T]{native: make(chan T, request.Capacity)}
	mailbox.self = mailbox
	return mailbox, nil
}
func (m *Mailbox[T]) Validate() error {
	if m == nil || m.self != m || m.native == nil || cap(m.native) == 0 {
		return core.ErrBackgroundContract
	}
	return nil
}
func (m *Mailbox[T]) Capacity() (int, error) {
	if err := m.Validate(); err != nil {
		return 0, err
	}
	return cap(m.native), nil
}

// Offer never waits for a receiver. A replacement is one native eviction and
// offer under a producer mutex. Consumers can only free capacity, so neither
// retry budgets nor a product-side eviction loop are needed.
func (m *Mailbox[T]) Offer(request MailboxOffer[T]) (MailboxOfferObservation, error) {
	if err := errors.Join(m.Validate(), request.Validate()); err != nil {
		return MailboxOfferObservation{}, err
	}
	m.writer.Lock()
	defer m.writer.Unlock()
	select {
	case m.native <- request.Value:
		return MailboxOfferObservation{Disposition: MailboxAccepted}, nil
	default:
	}
	if request.Overflow == MailboxOverflowPreserveQueued {
		return MailboxOfferObservation{Disposition: MailboxDiscarded}, nil
	}
	disposition := MailboxAccepted
	select {
	case <-m.native:
		disposition = MailboxReplaced
	default:
	}
	// Every other producer is excluded, and a receiver cannot fill the buffer.
	m.native <- request.Value
	return MailboxOfferObservation{Disposition: disposition}, nil
}

func (m *Mailbox[T]) Poll(ctx context.Context) (MailboxReception[T], error) {
	if err := errors.Join(m.Validate(), temporal.Wait(temporal.WaitRequest{Context: ctx})); err != nil {
		return MailboxReception[T]{}, err
	}
	select {
	case value := <-m.native:
		return MailboxReception[T]{Kind: MailboxItem, Value: value}, nil
	default:
		return MailboxReception[T]{Kind: MailboxEmpty}, nil
	}
}

// Receive delegates selection among messages, a caller-owned native ticker and
// cancellation to Go. A nil ticker disables that case in ReceiveAll mode.
func (m *Mailbox[T]) Receive(ctx context.Context, request MailboxReceiveRequest) (MailboxReception[T], error) {
	if err := errors.Join(m.Validate(), request.Validate(), temporal.Wait(temporal.WaitRequest{Context: ctx})); err != nil {
		return MailboxReception[T]{}, err
	}
	messages := (<-chan T)(m.native)
	if request.Mode == MailboxReceiveTicks {
		messages = nil
	}
	select {
	case <-ctx.Done():
		return MailboxReception[T]{}, temporal.Wait(temporal.WaitRequest{Context: ctx})
	case value := <-messages:
		return MailboxReception[T]{Kind: MailboxItem, Value: value}, nil
	case tick, open := <-request.Ticks:
		if !open {
			return MailboxReception[T]{}, core.ErrBackgroundStopped
		}
		return MailboxReception[T]{Kind: MailboxTick, Tick: tick}, nil
	}
}
