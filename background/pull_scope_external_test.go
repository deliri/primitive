package background_test

import (
	"context"
	"errors"
	"io"
	"iter"
	"runtime"
	"testing"

	"github.com/deliri/primitive/v2026/background"
	"github.com/deliri/primitive/v2026/core"
)

func TestPullScopeOwnsNativeStopAndBackpressure(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                  string
		items, take           int
		refuse, panicConsumer bool
	}{
		{name: "empty source", items: 0, take: 1},
		{name: "exhaustion", items: 7, take: 8},
		{name: "one item then early return", items: 1000000, take: 1},
		{name: "many windows without a lifetime quota", items: 65537, take: 65538},
		{name: "consumer refuses suspended producer", items: 1000000, take: 3, refuse: true},
		{name: "consumer panic joins suspended producer", items: 1000000, take: 2, panicConsumer: true},
		{name: "no consumer demand never starts source", items: 1000000, take: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			produced, consumed, cleaned := 0, 0, false
			var escaped *background.PullSource[int]
			err := background.WithPullScope(t.Context(), background.PullScopeRequest[int]{
				Source: func(yield func(int, error) bool) {
					defer func() { cleaned = true }()
					for i := range tc.items {
						produced++
						if !yield(i, nil) {
							return
						}
					}
				},
				Use: func(_ context.Context, source *background.PullSource[int]) error {
					escaped = source
					for range tc.take {
						if produced != consumed {
							t.Fatalf("source advanced without demand: produced=%d consumed=%d", produced, consumed)
						}
						item, ok, err := source.Next()
						if err != nil {
							return err
						}
						if !ok {
							break
						}
						if item != consumed {
							t.Fatalf("item=%d want=%d", item, consumed)
						}
						consumed++
					}
					if tc.panicConsumer {
						panic(io.ErrShortWrite)
					}
					if tc.refuse {
						return io.ErrClosedPipe
					}
					return nil
				},
			})
			want := min(tc.items, tc.take)
			if consumed != want || produced != want || cleaned != (tc.take > 0) {
				t.Fatalf("production/consumption/cleanup=%d/%d/%t want=%d/%d/%t", produced, consumed, cleaned, want, want, tc.take > 0)
			}
			if tc.panicConsumer {
				if !errors.Is(err, core.ErrBackgroundCallbackPanic) || !errors.Is(err, io.ErrShortWrite) {
					t.Fatalf("consumer panic lost identity/cause: %v", err)
				}
			} else if tc.refuse {
				if !errors.Is(err, io.ErrClosedPipe) {
					t.Fatalf("consumer refusal=%v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if value, ok, err := escaped.Next(); value != 0 || ok || !errors.Is(err, core.ErrBackgroundStopped) {
				t.Fatalf("expired loan=%d/%t/%v", value, ok, err)
			}
		})
	}
}

func TestPullScopeRefusesIngressWithoutBorrowingSource(t *testing.T) {
	t.Parallel()
	canceled, cancel := context.WithCancelCause(t.Context())
	cancel(io.ErrNoProgress)
	for _, tc := range []struct {
		name            string
		ctx             context.Context
		noSource, noUse bool
		want            error
	}{
		{name: "nil source", ctx: t.Context(), noSource: true, want: core.ErrBackgroundContract},
		{name: "nil consumer", ctx: t.Context(), noUse: true, want: core.ErrBackgroundContract},
		{name: "nil context", want: core.ErrNilContext},
		{name: "canceled parent", ctx: canceled, want: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sourceCalled, useCalled := false, false
			var source iter.Seq2[int, error] = func(func(int, error) bool) { sourceCalled = true }
			use := func(context.Context, *background.PullSource[int]) error { useCalled = true; return nil }
			if tc.noSource {
				source = nil
			}
			if tc.noUse {
				use = nil
			}
			err := background.WithPullScope(tc.ctx, background.PullScopeRequest[int]{Source: source, Use: use})
			if !errors.Is(err, tc.want) || sourceCalled || useCalled {
				t.Fatalf("refusal=%v source/use=%t/%t", err, sourceCalled, useCalled)
			}
			if tc.ctx == canceled && !errors.Is(err, io.ErrNoProgress) {
				t.Fatalf("lost native first cause: %v", err)
			}
		})
	}
}

func TestPullScopeSourceFailureWithdrawsPartialValueAndJoinsCleanup(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                      string
		panicSource, panicCleanup bool
	}{
		{name: "source refusal"}, {name: "source panic", panicSource: true}, {name: "producer stop panic", panicCleanup: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cleaned := false
			err := background.WithPullScope(t.Context(), background.PullScopeRequest[int]{Source: func(yield func(int, error) bool) {
				defer func() {
					cleaned = true
					if tc.panicCleanup {
						panic(io.ErrUnexpectedEOF)
					}
				}()
				if tc.panicSource {
					panic(io.ErrNoProgress)
				}
				if tc.panicCleanup {
					yield(7, nil)
					return
				}
				yield(99, io.ErrClosedPipe)
			}, Use: func(_ context.Context, source *background.PullSource[int]) error {
				value, ok, err := source.Next()
				if tc.panicCleanup {
					if value != 7 || !ok || err != nil {
						t.Fatalf("valid loan=%d/%t/%v", value, ok, err)
					}
					return io.ErrShortWrite
				}
				if value != 0 || ok || !errors.Is(err, io.ErrClosedPipe) {
					t.Fatalf("partial failure escaped=%d/%t/%v", value, ok, err)
				}
				return err
			}})
			if !cleaned {
				t.Fatal("producer cleanup outlived scope")
			}
			switch {
			case tc.panicSource:
				if !errors.Is(err, core.ErrBackgroundCallbackPanic) || !errors.Is(err, io.ErrNoProgress) {
					t.Fatalf("source panic=%v", err)
				}
			case tc.panicCleanup:
				if !errors.Is(err, core.ErrBackgroundCallbackPanic) || !errors.Is(err, io.ErrUnexpectedEOF) || !errors.Is(err, io.ErrShortWrite) {
					t.Fatalf("cleanup refusal=%v", err)
				}
			default:
				if !errors.Is(err, io.ErrClosedPipe) {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestPullScopeCancellationStopsBeforeNextAndInvalidatesCopies(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)
	produced, cleaned := 0, false
	err := background.WithPullScope(ctx, background.PullScopeRequest[int]{Source: func(yield func(int, error) bool) {
		defer func() { cleaned = true }()
		for i := 0; ; i++ {
			produced++
			if !yield(i, nil) {
				return
			}
		}
	}, Use: func(_ context.Context, source *background.PullSource[int]) error {
		copyOfSource := *source
		if value, ok, err := copyOfSource.Next(); value != 0 || ok || !errors.Is(err, core.ErrBackgroundContract) {
			t.Fatalf("copied source=%d/%t/%v", value, ok, err)
		}
		if value, ok, err := source.Next(); value != 0 || !ok || err != nil {
			return errors.Join(core.ErrBackgroundContract, err)
		}
		cancel(io.ErrNoProgress)
		value, ok, err := source.Next()
		if value != 0 || ok || !errors.Is(err, context.Canceled) || !errors.Is(err, io.ErrNoProgress) {
			t.Fatalf("canceled loan=%d/%t/%v", value, ok, err)
		}
		return nil
	}})
	if produced != 1 || !cleaned || !errors.Is(err, context.Canceled) || !errors.Is(err, io.ErrNoProgress) {
		t.Fatalf("canceled scope=%v produced=%d cleaned=%t", err, produced, cleaned)
	}
}

func TestPullScopeGoexitStillStopsSuspendedProducer(t *testing.T) {
	t.Parallel()
	cleaned := false
	joined := make(chan struct{})
	go func() {
		defer close(joined)
		_ = background.WithPullScope(t.Context(), background.PullScopeRequest[int]{Source: func(yield func(int, error) bool) { defer func() { cleaned = true }(); yield(1, nil) }, Use: func(_ context.Context, source *background.PullSource[int]) error {
			_, _, _ = source.Next()
			runtime.Goexit()
			return nil
		}})
	}()
	<-joined
	if !cleaned {
		t.Fatal("Goexit abandoned suspended producer")
	}
}

func FuzzPullScopeExactForwardConservation(f *testing.F) {
	f.Add(uint16(0), uint16(0))
	f.Add(uint16(1), uint16(2))
	f.Add(uint16(65535), uint16(65535))
	f.Fuzz(func(t *testing.T, count, stop uint16) {
		produced, consumed, cleanups := 0, 0, 0
		err := background.WithPullScope(t.Context(), background.PullScopeRequest[int]{Source: func(yield func(int, error) bool) {
			defer func() { cleanups++ }()
			for i := range int(count) {
				produced++
				if !yield(i, nil) {
					return
				}
			}
		}, Use: func(_ context.Context, source *background.PullSource[int]) error {
			for range int(stop) {
				value, ok, err := source.Next()
				if err != nil {
					return err
				}
				if !ok {
					break
				}
				if value != consumed {
					t.Fatalf("sequence value=%d ordinal=%d", value, consumed)
				}
				consumed++
			}
			return nil
		}})
		want := min(int(count), int(stop))
		wantCleanup := 0
		if stop > 0 {
			wantCleanup = 1
		}
		if err != nil || produced != want || consumed != want || cleanups != wantCleanup {
			t.Fatalf("conservation=%d/%d/%d err=%v want=%d/%d/%d", produced, consumed, cleanups, err, want, want, wantCleanup)
		}
	})
}
