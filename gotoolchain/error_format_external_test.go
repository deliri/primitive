package gotoolchain_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestErrorFormatOperandsUseNativeWrapIdentity(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, format string
		count        int
		want         []int
	}{
		{name: "empty format", format: "", count: 3},
		{name: "ordinary text", format: "literal", count: 3},
		{name: "ordinary verb grants no wrapping", format: "%v", count: 3},
		{name: "type verb grants no wrapping", format: "%T", count: 3},
		{name: "one wrapped operand", format: "%w", count: 3, want: []int{0}},
		{name: "ordinary verb advances operand", format: "%v %w", count: 3, want: []int{1}},
		{name: "escaped verb consumes no operand", format: "%%w %w", count: 3, want: []int{0}},
		{name: "explicit argument index", format: "%[2]w", count: 3, want: []int{1}},
		{name: "reordered indices follow native unwrap order", format: "%[3]w %[1]w", count: 3, want: []int{0, 2}},
		{name: "duplicate argument follows native deduplication", format: "%[2]w %[2]w", count: 3, want: []int{1}},
		{name: "native multi wrap", format: "%w %w %w", count: 3, want: []int{0, 1, 2}},
		{name: "width consumes an argument", format: "%*w", count: 3, want: []int{1}},
		{name: "precision consumes an argument", format: "%.*w", count: 3, want: []int{1}},
		{name: "width precision and verb have independent indices", format: "%[3]*.[2]*[1]w", count: 3, want: []int{0}},
		{name: "missing operand creates no marker", format: "%w", count: 0},
		{name: "trailing percent creates no marker", format: "%", count: 3},
		{name: "missing closing index creates no marker", format: "%[", count: 3},
		{name: "zero index grants no marker", format: "%[0]w", count: 3},
		{name: "negative index grants no marker", format: "%[-1]w", count: 3},
		{name: "foreign index grants no marker", format: "%[4]w", count: 3},
		{name: "malformed index retains native recovery", format: "%[99]w %w", count: 3, want: []int{0}},
		{name: "indexed width syntax follows native recovery", format: "%[1]2w %w", count: 3, want: []int{0}},
		{name: "unknown verb still advances native position", format: "%z %w", count: 3, want: []int{1}},
		{name: "unicode verb still advances native position", format: "%λ %w", count: 3, want: []int{1}},
		{name: "invalid utf8 remains native format data", format: "\xff%w", count: 3, want: []int{0}},
		{name: "nul remains native format data", format: "\x00%w", count: 3, want: []int{0}},
		{name: "argument population exceeds ordinary working window", format: "%[65537]w", count: 65537, want: []int{65536}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var got []int
			for operand, err := range gotoolchain.ErrorFormatOperands(t.Context(), gotoolchain.ErrorFormatRequest{Format: tc.format, ArgumentCount: tc.count}) {
				if err != nil || operand.Validate() != nil || operand.Index >= tc.count {
					t.Fatalf("native format loan=%+v/%v", operand, err)
				}
				got = append(got, operand.Index)
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("native wrap operands=%v want%v", got, tc.want)
			}
		})
	}
}

func TestErrorFormatOperandsRefuseRequestsAndRetainCancellationCause(t *testing.T) {
	t.Parallel()
	for _, cancelled := range [...]bool{false, true} {
		t.Run(fmt.Sprint(cancelled), func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			request := gotoolchain.ErrorFormatRequest{Format: "%w", ArgumentCount: -1}
			want := error(core.ErrGoToolchainContract)
			if cancelled {
				request.ArgumentCount = 1
				cancel(io.ErrClosedPipe)
				want = io.ErrClosedPipe
			}
			refused := 0
			for operand, err := range gotoolchain.ErrorFormatOperands(ctx, request) {
				if operand != (gotoolchain.ErrorFormatOperand{}) || !errors.Is(err, want) {
					t.Fatalf("invalid native format request=%+v/%v want zero/%v", operand, err, want)
				}
				if !cancelled && errors.Is(err, core.ErrGoToolchainOutput) {
					t.Fatalf("invalid input became native formatter output failure: %v", err)
				}
				refused++
			}
			if refused != 1 {
				t.Fatalf("native format refusals=%d want1", refused)
			}
		})
	}
	if err := (gotoolchain.ErrorFormatOperand{Index: -1}).Validate(); !errors.Is(err, core.ErrGoToolchainContract) {
		t.Fatalf("negative operand identity=%v", err)
	}
}

func TestErrorFormatOperandsStopBeforeAnotherLoan(t *testing.T) {
	t.Parallel()
	for _, stop := range [...]bool{false, true} {
		t.Run(fmt.Sprint(stop), func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			loans, refusals := 0, 0
			for operand, err := range gotoolchain.ErrorFormatOperands(ctx, gotoolchain.ErrorFormatRequest{Format: "%w %w %w", ArgumentCount: 3}) {
				if err != nil {
					if operand != (gotoolchain.ErrorFormatOperand{}) || !errors.Is(err, io.ErrClosedPipe) {
						t.Fatalf("cancelled native format loan=%+v/%v", operand, err)
					}
					refusals++
					continue
				}
				if operand.Index != 0 {
					t.Fatalf("native format yielded after cancelled first loan: %+v", operand)
				}
				loans++
				cancel(io.ErrClosedPipe)
				if stop {
					break
				}
			}
			want := 1
			if stop {
				want = 0
			}
			if loans != 1 || refusals != want {
				t.Fatalf("native format loans/refusals=%d/%d want1/%d", loans, refusals, want)
			}
		})
	}
}

func FuzzErrorFormatOperandsMatchNativeErrorIdentity(f *testing.F) {
	for _, format := range []string{"%w", "%v %w", "%%w %v", "%[2]w", "%[3]*.[2]*[1]w", "%[99]w %w", "%[1]2w %w", "%w %w %w", "%.*w", "\xff%w"} {
		f.Add(format)
	}
	f.Fuzz(func(t *testing.T, format string) {
		native := fmt.Errorf(format, core.ErrGoModuleContract, core.ErrGoToolchainContract, core.ErrLineIOContract)
		var selected [3]bool
		for operand, err := range gotoolchain.ErrorFormatOperands(t.Context(), gotoolchain.ErrorFormatRequest{Format: format, ArgumentCount: len(selected)}) {
			if err != nil || operand.Validate() != nil || operand.Index >= len(selected) {
				t.Fatalf("native format operand=%+v/%v", operand, err)
			}
			selected[operand.Index] = true
		}
		for index, identity := range [...]error{core.ErrGoModuleContract, core.ErrGoToolchainContract, core.ErrLineIOContract} {
			if got := errors.Is(native, identity); got != selected[index] {
				t.Fatalf("format%q operand%d selected%t native identity%t", format, index, selected[index], got)
			}
		}
	})
}
