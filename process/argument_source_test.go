package process_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

func TestArgumentSourceConservesExactInvocationOrReturnsNoProjection(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                                                                       string
		extent                                                                     int
		refuse, invalid, lateCancel, factoryCancel, missingIterator, missingSource bool
		want                                                                       error
	}{
		{name: "empty source"},
		{name: "exact arguments preserve boundaries", extent: 7},
		{name: "below native argument count", extent: int(process.ArgumentCountMaximum) - 1},
		{name: "exact native argument count", extent: int(process.ArgumentCountMaximum)},
		{name: "above native argument count", extent: int(process.ArgumentCountMaximum) + 1, want: core.ErrProcessContract},
		{name: "producer refusal after admitted prefix", extent: 7, refuse: true, want: io.ErrUnexpectedEOF},
		{name: "unset argument after admitted prefix", extent: 7, invalid: true, want: core.ErrProcessContract},
		{name: "late cancellation withdraws complete projection", extent: 7, lateCancel: true, want: context.Canceled},
		{name: "source refusal and cancellation both survive", extent: 7, refuse: true, lateCancel: true, want: io.ErrUnexpectedEOF},
		{name: "factory cancellation never enters iterator", extent: 7, factoryCancel: true, want: context.Canceled},
		{name: "missing iterator", missingIterator: true, want: core.ErrProcessContract},
		{name: "missing source", missingSource: true, want: core.ErrProcessContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			entered := false
			source := process.ArgumentSource(func(context.Context) iter.Seq2[process.Argument, error] {
				if tc.factoryCancel {
					cancel()
				}
				if tc.missingIterator {
					return nil
				}
				return func(yield func(process.Argument, error) bool) {
					entered = true
					for index := range tc.extent {
						argument, err := process.NewArgument(fmt.Sprintf("argument %d ü", index))
						if !yield(argument, err) || err != nil {
							return
						}
					}
					if tc.lateCancel {
						cancel()
					}
					if tc.refuse {
						yield(process.Argument{}, io.ErrUnexpectedEOF)
					}
					if tc.invalid {
						yield(process.Argument{}, nil)
					}
				}
			})
			if tc.missingSource {
				source = nil
			}
			got, err := process.CollectArguments(ctx, source)
			if !errors.Is(err, tc.want) || tc.lateCancel && !errors.Is(err, context.Canceled) || err != nil && got != nil || tc.factoryCancel && entered {
				t.Fatalf("projection=%v error=%v iterator entered=%t, want exact refusal %v and no partial projection or canceled iterator entry", got, err, entered, tc.want)
			}
			if tc.want != nil {
				return
			}
			if len(got) != tc.extent {
				t.Fatalf("arguments=%d, want %d", len(got), tc.extent)
			}
			for index, argument := range got {
				value, err := argument.Value()
				if err != nil || value != fmt.Sprintf("argument %d ü", index) {
					t.Fatalf("argument %d=%q/%v, want exact source value", index, value, err)
				}
			}
		})
	}
}

func TestArgumentSourcePreservesNativeProjectionByteBoundary(t *testing.T) {
	t.Parallel()
	for _, bytes := range []int{int(process.ArgumentProjectionMaximumBytes) - 1, int(process.ArgumentProjectionMaximumBytes), int(process.ArgumentProjectionMaximumBytes) + 1} {
		t.Run(fmt.Sprintf("NUL-delimited bytes %d", bytes), func(t *testing.T) {
			t.Parallel()
			value := strings.Repeat("x", bytes-1)
			source := process.ArgumentSource(func(context.Context) iter.Seq2[process.Argument, error] {
				return func(yield func(process.Argument, error) bool) {
					argument, err := process.NewArgument(value)
					yield(argument, err)
				}
			})
			got, err := process.CollectArguments(t.Context(), source)
			if bytes > int(process.ArgumentProjectionMaximumBytes) {
				if got != nil || !errors.Is(err, core.ErrProcessContract) {
					t.Fatalf("over-boundary projection=%v/%v, want zero and native contract refusal", got, err)
				}
				return
			}
			if err != nil || len(got) != 1 {
				t.Fatalf("admitted projection length=%d/%v, want one exact argument", len(got), err)
			}
			text, err := got[0].Value()
			if err != nil || text != value {
				t.Fatalf("argument bytes=%d/%v, want %d unchanged bytes", len(text), err, len(value))
			}
		})
	}
}

func FuzzArgumentSourceExactProjection(f *testing.F) {
	argument, err := process.NewArgument("space ü literal $()")
	if err != nil {
		f.Fatal(err)
	}
	seed, err := argument.Value()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed, uint8(3), false)
	f.Add("", uint8(0), false)
	f.Add("\x00", uint8(2), true)
	f.Fuzz(func(t *testing.T, value string, extent uint8, refused bool) {
		source := process.ArgumentSource(func(context.Context) iter.Seq2[process.Argument, error] {
			return func(yield func(process.Argument, error) bool) {
				for range extent {
					argument, err := process.NewArgument(value)
					if !yield(argument, err) || err != nil {
						return
					}
				}
				if refused {
					yield(process.Argument{}, io.ErrUnexpectedEOF)
				}
			}
		})
		got, err := process.CollectArguments(t.Context(), source)
		validAtom := !strings.ContainsRune(value, 0) && uint64(len(value)) <= process.ArgumentMaximumBytes
		validProjection := extent == 0 || validAtom && uint64(extent)*(uint64(len(value))+1) <= process.ArgumentProjectionMaximumBytes
		if !validProjection || refused {
			want := error(core.ErrProcessContract)
			if validProjection {
				want = io.ErrUnexpectedEOF
			}
			if got != nil || !errors.Is(err, want) {
				t.Fatalf("refused projection=%d/%v, want zero and %v", len(got), err, want)
			}
			return
		}
		if err != nil || len(got) != int(extent) {
			t.Fatalf("exact projection=%d/%v, want %d", len(got), err, extent)
		}
		for index, argument := range got {
			text, err := argument.Value()
			if err != nil || text != value {
				t.Fatalf("argument %d=%q/%v, want unchanged %q", index, text, err, value)
			}
		}
	})
}
