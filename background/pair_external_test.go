package background_test

import (
	"context"
	"errors"
	"io"
	"runtime"
	"testing"
	"testing/synctest"

	"github.com/deliri/primitive/v2026/background"
	"github.com/deliri/primitive/v2026/core"
)

func TestNativePairPreservesConcurrentSourcesAndSynchronousConsumption(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		ready, release, joined := make(chan struct{}, 2), make(chan struct{}), make(chan struct{})
		firstCleaned, secondCleaned := false, false
		firstConsumed, secondConsumed := 0, 0
		var result background.PairResult
		var resultErr error
		go func() {
			defer close(joined)
			result, resultErr = background.RunPair(t.Context(), background.PairRequest[int, int]{
				First: func(ctx context.Context, offer func(int) error) error {
					defer func() { firstCleaned = true }()
					ready <- struct{}{}
					<-release
					for i := range 513 {
						if firstConsumed != i {
							return core.ErrBackgroundContract
						}
						if err := offer(i); err != nil {
							return err
						}
					}
					return nil
				},
				Second: func(ctx context.Context, offer func(int) error) error {
					defer func() { secondCleaned = true }()
					ready <- struct{}{}
					<-release
					for i := range 257 {
						if secondConsumed != i {
							return core.ErrBackgroundContract
						}
						if err := offer(i); err != nil {
							return err
						}
					}
					return nil
				},
				ConsumeFirst: func(_ context.Context, value int) error {
					if value != firstConsumed {
						return io.ErrNoProgress
					}
					firstConsumed++
					return nil
				},
				ConsumeSecond: func(_ context.Context, value int) error {
					if value != secondConsumed {
						return io.ErrUnexpectedEOF
					}
					secondConsumed++
					return nil
				},
			})
		}()
		<-ready
		<-ready
		close(release)
		<-joined
		if resultErr != nil || result.FirstError != nil || result.SecondError != nil || firstConsumed != 513 || secondConsumed != 257 || !firstCleaned || !secondCleaned {
			t.Fatalf("native pair=%+v/%v consumed=%d/%d cleanup=%t/%t", result, resultErr, firstConsumed, secondConsumed, firstCleaned, secondCleaned)
		}
	})
}

func TestNativePairPreservesRefusalsAndIndependentSibling(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                                                  string
		sourcePanic, sourceExit, consumerPanic, ignoreRefusal bool
		sourceError, consumerError, want                      error
	}{
		{name: "source refusal", sourceError: io.ErrClosedPipe, want: io.ErrClosedPipe},
		{name: "consumer refusal", consumerError: io.ErrShortWrite, want: io.ErrShortWrite},
		{name: "source cannot erase ignored consumer refusal", consumerError: io.ErrShortWrite, ignoreRefusal: true, want: io.ErrShortWrite},
		{name: "source panic", sourcePanic: true, want: core.ErrBackgroundCallbackPanic},
		{name: "source Goexit", sourceExit: true, want: core.ErrBackgroundStopped},
		{name: "consumer panic", consumerPanic: true, want: core.ErrBackgroundCallbackPanic},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			firstCleaned, secondCleaned := false, false
			siblingConsumed := 0
			result, err := background.RunPair(t.Context(), background.PairRequest[int, int]{First: func(_ context.Context, offer func(int) error) error {
				defer func() { firstCleaned = true }()
				if tc.sourcePanic {
					panic(io.ErrNoProgress)
				}
				if tc.sourceExit {
					runtime.Goexit()
				}
				offerErr := offer(1)
				if tc.ignoreRefusal {
					return tc.sourceError
				}
				return errors.Join(offerErr, tc.sourceError)
			}, Second: func(ctx context.Context, offer func(int) error) error {
				defer func() { secondCleaned = true }()
				for i := range 17 {
					if err := offer(i); err != nil {
						return err
					}
				}
				return ctx.Err()
			}, ConsumeFirst: func(context.Context, int) error {
				if tc.consumerPanic {
					panic(io.ErrUnexpectedEOF)
				}
				return tc.consumerError
			}, ConsumeSecond: func(_ context.Context, value int) error {
				if value != siblingConsumed {
					return io.ErrNoProgress
				}
				siblingConsumed++
				return nil
			}})
			if err != nil || !errors.Is(result.FirstError, tc.want) || result.SecondError != nil || siblingConsumed != 17 || !firstCleaned || !secondCleaned {
				t.Fatalf("pair=%+v/%v sibling=%d cleanup=%t/%t", result, err, siblingConsumed, firstCleaned, secondCleaned)
			}
			if tc.sourcePanic && !errors.Is(result.FirstError, io.ErrNoProgress) {
				t.Fatalf("source panic cause lost: %v", result.FirstError)
			}
			if tc.consumerPanic && !errors.Is(result.FirstError, io.ErrUnexpectedEOF) {
				t.Fatalf("consumer panic cause lost: %v", result.FirstError)
			}
		})
	}
}

func TestNativePairCancellationPreservesFirstCauseAndJoinsBothSources(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)
	firstCleaned, secondCleaned := false, false
	result, err := background.RunPair(ctx, background.PairRequest[int, int]{First: func(ctx context.Context, offer func(int) error) error {
		defer func() { firstCleaned = true }()
		return offer(1)
	}, Second: func(ctx context.Context, offer func(int) error) error {
		defer func() { secondCleaned = true }()
		<-ctx.Done()
		return ctx.Err()
	}, ConsumeFirst: func(context.Context, int) error { cancel(io.ErrNoProgress); return nil }, ConsumeSecond: func(context.Context, int) error { return core.ErrBackgroundContract }})
	if err != nil || !errors.Is(result.FirstError, context.Canceled) || !errors.Is(result.FirstError, io.ErrNoProgress) || !errors.Is(result.SecondError, context.Canceled) || !errors.Is(result.SecondError, io.ErrNoProgress) || !firstCleaned || !secondCleaned {
		t.Fatalf("canceled pair=%+v/%v cleanup=%t/%t", result, err, firstCleaned, secondCleaned)
	}
}

func TestNativePairCompletingOneSourceDoesNotCancelItsSibling(t *testing.T) {
	t.Parallel()
	firstLifetime := make(chan (<-chan struct{}), 1)
	consumed := false
	result, err := background.RunPair(t.Context(), background.PairRequest[int, int]{
		First: func(ctx context.Context, _ func(int) error) error { firstLifetime <- ctx.Done(); return nil },
		Second: func(ctx context.Context, offer func(int) error) error {
			<-<-firstLifetime
			if err := ctx.Err(); err != nil {
				return err
			}
			return offer(42)
		},
		ConsumeFirst: func(context.Context, int) error { return core.ErrBackgroundContract },
		ConsumeSecond: func(_ context.Context, value int) error {
			if value != 42 {
				return core.ErrBackgroundContract
			}
			consumed = true
			return nil
		},
	})
	if err != nil || result.FirstError != nil || result.SecondError != nil || !consumed {
		t.Fatalf("completed sibling canceled remaining source: %+v/%v consumed=%t", result, err, consumed)
	}
}

func TestNativePairConsumerGoexitJoinsBlockedSiblingBeforeCallerUnwinds(t *testing.T) {
	t.Parallel()
	firstCleaned, secondCleaned := false, false
	joined := make(chan struct{})
	go func() {
		defer close(joined)
		_, _ = background.RunPair(t.Context(), background.PairRequest[int, int]{First: func(ctx context.Context, offer func(int) error) error {
			defer func() { firstCleaned = true }()
			return offer(1)
		}, Second: func(ctx context.Context, offer func(int) error) error {
			defer func() { secondCleaned = true }()
			return offer(2)
		}, ConsumeFirst: func(context.Context, int) error { runtime.Goexit(); return nil }, ConsumeSecond: func(context.Context, int) error { runtime.Goexit(); return nil }})
	}()
	<-joined
	if !firstCleaned || !secondCleaned {
		t.Fatalf("Goexit returned before producer cleanup=%t/%t", firstCleaned, secondCleaned)
	}
}

func TestNativePairRefusesInvalidIngressAndExpiredOffer(t *testing.T) {
	t.Parallel()
	var escaped func(int) error
	consumed := 0
	request := background.PairRequest[int, int]{First: func(_ context.Context, offer func(int) error) error { escaped = offer; return nil }, Second: func(context.Context, func(int) error) error { return nil }, ConsumeFirst: func(context.Context, int) error { consumed++; return nil }, ConsumeSecond: func(context.Context, int) error { consumed++; return nil }}
	result, err := background.RunPair(t.Context(), request)
	if err != nil || result.FirstError != nil || result.SecondError != nil {
		t.Fatalf("empty pair=%+v/%v", result, err)
	}
	if err := escaped(42); !errors.Is(err, context.Canceled) || consumed != 0 {
		t.Fatalf("expired offer=%v consumed=%d", err, consumed)
	}
	for _, tc := range []struct {
		name       string
		missing    int
		nilContext bool
	}{
		{name: "first source absent", missing: 1}, {name: "second source absent", missing: 2}, {name: "first consumer absent", missing: 3}, {name: "second consumer absent", missing: 4}, {name: "nil parent", nilContext: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			called := false
			ctx := t.Context()
			r := background.PairRequest[int, int]{First: func(context.Context, func(int) error) error { called = true; return nil }, Second: func(context.Context, func(int) error) error { called = true; return nil }, ConsumeFirst: func(context.Context, int) error { called = true; return nil }, ConsumeSecond: func(context.Context, int) error { called = true; return nil }}
			switch tc.missing {
			case 1:
				r.First = nil
			case 2:
				r.Second = nil
			case 3:
				r.ConsumeFirst = nil
			case 4:
				r.ConsumeSecond = nil
			}
			want := error(core.ErrBackgroundContract)
			if tc.nilContext {
				ctx = nil
				want = core.ErrNilContext
			}
			result, err := background.RunPair(ctx, r)
			if !errors.Is(err, want) || result.FirstError != nil || result.SecondError != nil || called {
				t.Fatalf("refused pair=%+v/%v borrowed=%t", result, err, called)
			}
		})
	}
}
