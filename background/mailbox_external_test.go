package background_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/background"
	"github.com/deliri/primitive/v2026/core"
)

func TestMailboxNativeBufferLayerTriad(t *testing.T) {
	t.Parallel()
	for _, capacity := range []int{1, 2, 8, 64} {
		t.Run(fmt.Sprintf("capacity-%d", capacity), func(t *testing.T) {
			t.Parallel()
			mailbox, err := background.OpenMailbox[int](background.MailboxRequest{Capacity: capacity})
			if err != nil {
				t.Fatal(err)
			}
			if got, err := mailbox.Capacity(); got != capacity || err != nil {
				t.Fatalf("native capacity%d/%v want%d", got, err, capacity)
			}
			for value := range capacity {
				got, err := mailbox.Offer(background.MailboxOffer[int]{Value: value, Overflow: background.MailboxOverflowPreserveQueued})
				if err != nil || got.Disposition != background.MailboxAccepted || got.Validate() != nil {
					t.Fatalf("native offer%d %+v/%v", value, got, err)
				}
			}
			rejected, err := mailbox.Offer(background.MailboxOffer[int]{Value: -1, Overflow: background.MailboxOverflowPreserveQueued})
			if err != nil || rejected.Disposition != background.MailboxDiscarded {
				t.Fatalf("full native buffer%+v/%v", rejected, err)
			}
			for wanted := range capacity {
				got, err := mailbox.Poll(t.Context())
				if err != nil || got.Kind != background.MailboxItem || got.Value != wanted {
					t.Fatalf("native FIFO%+v/%v want%d", got, err, wanted)
				}
			}
			empty, err := mailbox.Poll(t.Context())
			if err != nil || empty.Kind != background.MailboxEmpty || empty.Value != 0 || empty.Validate() != nil {
				t.Fatalf("empty native buffer%+v/%v", empty, err)
			}
		})
	}
}

func TestMailboxReplacementConservesNativeFIFO(t *testing.T) {
	t.Parallel()
	for _, capacity := range []int{1, 2, 8, 64} {
		t.Run(fmt.Sprintf("capacity-%d", capacity), func(t *testing.T) {
			t.Parallel()
			mailbox, err := background.OpenMailbox[int](background.MailboxRequest{Capacity: capacity})
			if err != nil {
				t.Fatal(err)
			}
			for value := range capacity {
				if _, err := mailbox.Offer(background.MailboxOffer[int]{Value: value, Overflow: background.MailboxOverflowPreserveQueued}); err != nil {
					t.Fatal(err)
				}
			}
			got, err := mailbox.Offer(background.MailboxOffer[int]{Value: capacity, Overflow: background.MailboxOverflowReplaceOldest})
			if err != nil || got.Disposition != background.MailboxReplaced {
				t.Fatalf("native replacement%+v/%v", got, err)
			}
			for wanted := 1; wanted <= capacity; wanted++ {
				got, err := mailbox.Poll(t.Context())
				if err != nil || got.Kind != background.MailboxItem || got.Value != wanted {
					t.Fatalf("native replacement FIFO%+v/%v want%d", got, err, wanted)
				}
			}
		})
	}
}

func TestMailboxNativeReceiveOwnsTicksMessagesAndCancellation(t *testing.T) {
	t.Parallel()
	mailbox, err := background.OpenMailbox[int](background.MailboxRequest{Capacity: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mailbox.Offer(background.MailboxOffer[int]{Value: 19, Overflow: background.MailboxOverflowPreserveQueued}); err != nil {
		t.Fatal(err)
	}
	ticks := make(chan time.Time, 1)
	now := time.Unix(1, 2)
	ticks <- now
	got, err := mailbox.Receive(t.Context(), background.MailboxReceiveRequest{Mode: background.MailboxReceiveTicks, Ticks: ticks})
	if err != nil || got.Kind != background.MailboxTick || got.Tick != now {
		t.Fatalf("native tick%+v/%v", got, err)
	}
	got, err = mailbox.Receive(t.Context(), background.MailboxReceiveRequest{Mode: background.MailboxReceiveAll})
	if err != nil || got.Kind != background.MailboxItem || got.Value != 19 {
		t.Fatalf("tick-only receive consumed queued message%+v/%v", got, err)
	}
	close(ticks)
	got, err = mailbox.Receive(t.Context(), background.MailboxReceiveRequest{Mode: background.MailboxReceiveTicks, Ticks: ticks})
	if got != (background.MailboxReception[int]{}) || !errors.Is(err, core.ErrBackgroundStopped) {
		t.Fatalf("closed native tick became a tick%+v/%v", got, err)
	}
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(io.ErrClosedPipe)
	got, err = mailbox.Receive(ctx, background.MailboxReceiveRequest{Mode: background.MailboxReceiveAll})
	if got != (background.MailboxReception[int]{}) || !errors.Is(err, io.ErrClosedPipe) || !errors.Is(err, context.Canceled) {
		t.Fatalf("native receive cause%+v/%v", got, err)
	}
	got, err = mailbox.Poll(ctx)
	if got != (background.MailboxReception[int]{}) || !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("native poll cause%+v/%v", got, err)
	}
}

func TestMailboxRefusesMissingAndContradictoryContracts(t *testing.T) {
	t.Parallel()
	for _, capacity := range []int{-1, 0} {
		mailbox, err := background.OpenMailbox[int](background.MailboxRequest{Capacity: capacity})
		if mailbox != nil || !errors.Is(err, core.ErrBackgroundContract) {
			t.Fatalf("native buffer capacity%d %+v/%v", capacity, mailbox, err)
		}
	}
	for _, mailbox := range []*background.Mailbox[int]{nil, {}} {
		if got, err := mailbox.Poll(t.Context()); got != (background.MailboxReception[int]{}) || !errors.Is(err, core.ErrBackgroundContract) {
			t.Fatalf("missing native mailbox%+v/%v", got, err)
		}
	}
	mailbox, err := background.OpenMailbox[int](background.MailboxRequest{Capacity: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := mailbox.Offer(background.MailboxOffer[int]{Value: 7}); got != (background.MailboxOfferObservation{}) || !errors.Is(err, core.ErrBackgroundContract) {
		t.Fatalf("unknown overflow admitted%+v/%v", got, err)
	}
	if got, err := mailbox.Receive(t.Context(), background.MailboxReceiveRequest{}); got != (background.MailboxReception[int]{}) || !errors.Is(err, core.ErrBackgroundContract) {
		t.Fatalf("unknown receive mode admitted%+v/%v", got, err)
	}
	if got, err := mailbox.Receive(t.Context(), background.MailboxReceiveRequest{Mode: background.MailboxReceiveTicks}); got != (background.MailboxReception[int]{}) || !errors.Is(err, core.ErrBackgroundContract) {
		t.Fatalf("missing tick source admitted%+v/%v", got, err)
	}
	for value := range 256 {
		overflowErr := background.MailboxOverflow(value).Validate()
		readErr := background.MailboxReadMode(value).Validate()
		dispositionErr := background.MailboxDisposition(value).Validate()
		kindErr := background.MailboxReceptionKind(value).Validate()
		if (overflowErr == nil) != (value >= 1 && value <= 2) || (readErr == nil) != (value >= 1 && value <= 2) || (dispositionErr == nil) != (value >= 1 && value <= 3) || (kindErr == nil) != (value >= 1 && value <= 3) {
			t.Fatalf("native mailbox enum value%d errors%v/%v/%v/%v", value, overflowErr, readErr, dispositionErr, kindErr)
		}
	}
}

func TestMailboxConcurrentProducersConserveAcceptedAndDiscarded(t *testing.T) {
	t.Parallel()
	mailbox, err := background.OpenMailbox[int](background.MailboxRequest{Capacity: 8})
	if err != nil {
		t.Fatal(err)
	}
	const producers, each = 16, 64
	var group sync.WaitGroup
	accepted := make(chan background.MailboxDisposition, producers*each)
	for producer := range producers {
		group.Go(func() {
			for index := range each {
				got, err := mailbox.Offer(background.MailboxOffer[int]{Value: producer*each + index, Overflow: background.MailboxOverflowPreserveQueued})
				if err != nil {
					t.Error(err)
					return
				}
				accepted <- got.Disposition
			}
		})
	}
	group.Wait()
	close(accepted)
	kept, discarded := 0, 0
	for disposition := range accepted {
		switch disposition {
		case background.MailboxAccepted:
			kept++
		case background.MailboxDiscarded:
			discarded++
		default:
			t.Fatalf("unexpected native offer disposition%d", disposition)
		}
	}
	if kept != 8 || discarded != producers*each-8 {
		t.Fatalf("native concurrent conservation%d/%d want8/%d", kept, discarded, producers*each-8)
	}
	for range kept {
		got, err := mailbox.Poll(t.Context())
		if err != nil || got.Kind != background.MailboxItem {
			t.Fatalf("missing accepted native item%+v/%v", got, err)
		}
	}
}

func FuzzMailboxNativeFIFOAndDispositionConservation(f *testing.F) {
	f.Add(uint8(1), []byte{0, 0, 2, 1, 2, 2})
	f.Add(uint8(7), []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 2, 2})
	f.Fuzz(func(t *testing.T, size uint8, actions []byte) {
		capacity := 1 + int(size%8)
		mailbox, err := background.OpenMailbox[int](background.MailboxRequest{Capacity: capacity})
		if err != nil {
			t.Fatal(err)
		}
		var wanted [8]int
		count := 0
		for index, action := range actions {
			if action%3 == 2 {
				got, err := mailbox.Poll(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				if count == 0 {
					if got.Kind != background.MailboxEmpty {
						t.Fatalf("native empty queue invented%+v", got)
					}
				} else {
					if got.Kind != background.MailboxItem || got.Value != wanted[0] {
						t.Fatalf("native FIFO%+v want%d", got, wanted[0])
					}
					copy(wanted[:], wanted[1:count])
					count--
				}
				continue
			}
			overflow := background.MailboxOverflowPreserveQueued
			if action%3 == 1 {
				overflow = background.MailboxOverflowReplaceOldest
			}
			disposition := background.MailboxAccepted
			if count == capacity {
				if overflow == background.MailboxOverflowPreserveQueued {
					disposition = background.MailboxDiscarded
				} else {
					disposition = background.MailboxReplaced
					copy(wanted[:], wanted[1:count])
					count--
				}
			}
			if disposition != background.MailboxDiscarded {
				wanted[count] = index
				count++
			}
			got, err := mailbox.Offer(background.MailboxOffer[int]{Value: index, Overflow: overflow})
			if err != nil || got.Disposition != disposition {
				t.Fatalf("native buffer disposition%+v/%v want%d", got, err, disposition)
			}
		}
		for index := range count {
			got, err := mailbox.Poll(t.Context())
			if err != nil || got.Kind != background.MailboxItem || got.Value != wanted[index] {
				t.Fatalf("native tail%+v/%v want%d", got, err, wanted[index])
			}
		}
		empty, err := mailbox.Poll(t.Context())
		if err != nil || empty.Kind != background.MailboxEmpty {
			t.Fatalf("native tail manufactured extra%+v/%v", empty, err)
		}
	})
}
