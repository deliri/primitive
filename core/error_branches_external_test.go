package core_test

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/testserial"
)

type nativeErrorJoin []error

func (nativeErrorJoin) Error() string     { return "native joined causes" }
func (e nativeErrorJoin) Unwrap() []error { return e }

type ownedAtomicError struct{ cause error }

func (ownedAtomicError) Error() string   { return "owned atomic cause" }
func (e ownedAtomicError) Unwrap() error { return e.cause }

func TestErrorBranchesRequireEveryNativeCause(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		cause  error
		atomic bool
		want   bool
	}{
		{name: "nil error is absence"},
		{name: "matching terminal", cause: io.EOF, want: true},
		{name: "unrelated terminal", cause: io.ErrClosedPipe},
		{name: "linear native wrapping", cause: fmt.Errorf("outer: %w", io.EOF), want: true},
		{name: "linear unrelated cause", cause: fmt.Errorf("outer: %w", io.ErrClosedPipe)},
		{name: "all joined branches match", cause: errors.Join(io.EOF, fmt.Errorf("inner: %w", io.EOF)), want: true},
		{name: "matching branch cannot hide native failure", cause: errors.Join(io.EOF, io.ErrClosedPipe)},
		{name: "wrapper cannot hide mixed join", cause: fmt.Errorf("outer: %w", errors.Join(io.EOF, io.ErrClosedPipe))},
		{name: "late nested failure remains visible", cause: errors.Join(io.EOF, errors.Join(io.EOF, io.ErrClosedPipe))},
		{name: "empty join has no authority", cause: nativeErrorJoin{}},
		{name: "nil branch has no authority", cause: nativeErrorJoin{io.EOF, nil}},
		{name: "typed nil join has no authority", cause: nativeErrorJoin(nil)},
		{name: "policy can own atomic wrapper", cause: ownedAtomicError{cause: io.ErrClosedPipe}, atomic: true, want: true},
		{name: "unowned wrapper still exposes cause", cause: ownedAtomicError{cause: io.ErrClosedPipe}},
		{name: "atomic wrapper cannot lend sibling authority", cause: errors.Join(ownedAtomicError{cause: io.ErrClosedPipe}, io.ErrClosedPipe), atomic: true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			got := core.ErrorBranchesMatch(core.ErrorBranchesRequest{Error: row.cause, Accept: func(node core.ErrorBranchObservation) bool {
				if err := node.Validate(); err != nil {
					t.Fatalf("borrowed native cause error=%v, want valid observation", err)
				}
				if _, owned := node.Error.(ownedAtomicError); owned && row.atomic {
					return true
				}
				return node.Unwrapped == nil && errors.Is(node.Error, io.EOF)
			}})
			if got != row.want {
				t.Fatalf("native error branches matched=%t for %v, want %t", got, row.cause, row.want)
			}
		})
	}
}

func TestErrorBranchesStopBeforeBorrowingLaterCauses(t *testing.T) {
	t.Parallel()
	seen := 0
	matched := core.ErrorBranchesMatch(core.ErrorBranchesRequest{Error: errors.Join(io.ErrClosedPipe, io.EOF), Accept: func(core.ErrorBranchObservation) bool { seen++; return false }})
	if matched || seen != 1 {
		t.Fatalf("refused native branches matched=%t visits=%d, want false/1", matched, seen)
	}
	if core.ErrorBranchesMatch(core.ErrorBranchesRequest{Error: io.EOF}) {
		t.Fatal("missing predicate matched native cause, want false")
	}
	if err := (core.ErrorBranchesRequest{}).Validate(); !errors.Is(err, core.ErrPrimitiveContract) {
		t.Fatalf("missing native predicate error=%v, want %v", err, core.ErrPrimitiveContract)
	}
	if err := (core.ErrorBranchObservation{}).Validate(); !errors.Is(err, core.ErrPrimitiveContract) {
		t.Fatalf("absent native cause error=%v, want %v", err, core.ErrPrimitiveContract)
	}
}

func FuzzErrorBranchesConserveUnrelatedNativeCauses(f *testing.F) {
	for _, seed := range [][]byte{{}, {0}, {1}, {0, 1}, {0, 0}, {254, 0}, {255, 0}} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		branches := make([]error, len(data))
		want := len(data) > 0
		for index, value := range data {
			cause := io.EOF
			if value&1 != 0 {
				cause = io.ErrClosedPipe
				want = false
			}
			for range value >> 5 {
				cause = fmt.Errorf("native: %w", cause)
			}
			branches[index] = cause
		}
		got := core.ErrorBranchesMatch(core.ErrorBranchesRequest{Error: errors.Join(branches...), Accept: func(node core.ErrorBranchObservation) bool {
			return node.Unwrapped == nil && errors.Is(node.Error, io.EOF)
		}})
		if got != want {
			t.Fatalf("native input=%x matched=%t, want %t from independent branch bits", data, got, want)
		}
	})
}

func TestErrorBranchesRetainNoJoinedPopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	var small float64
	for _, width := range []int{32, 16385} {
		branches := make([]error, width)
		for index := range branches {
			branches[index] = io.EOF
		}
		request := core.ErrorBranchesRequest{Error: errors.Join(branches...), Accept: func(node core.ErrorBranchObservation) bool {
			return node.Unwrapped == nil && errors.Is(node.Error, io.EOF)
		}}
		allocated := testing.AllocsPerRun(16, func() {
			if !core.ErrorBranchesMatch(request) {
				t.Fatalf("native width=%d refused, want every EOF branch admitted", width)
			}
		})
		if width == 32 {
			small = allocated
		}
		if allocated != small || allocated != 0 {
			t.Fatalf("native width=%d allocations=%v, want small width=%v and no copied population", width, allocated, small)
		}
	}
}

func BenchmarkErrorBranchesNativeJoin(b *testing.B) {
	b.ReportAllocs()
	branches := make([]error, 16385)
	for index := range branches {
		branches[index] = io.EOF
	}
	request := core.ErrorBranchesRequest{Error: errors.Join(branches...), Accept: func(node core.ErrorBranchObservation) bool {
		return node.Unwrapped == nil && errors.Is(node.Error, io.EOF)
	}}
	for b.Loop() {
		if !core.ErrorBranchesMatch(request) {
			b.Fatalf("native join width=%d refused, want all branches matched", len(branches))
		}
	}
}
