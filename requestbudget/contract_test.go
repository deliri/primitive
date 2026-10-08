package requestbudget

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestContractBoundaryLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		change  func(*Request)
		wantErr error
	}{
		{"exact start is inside", func(r *Request) { r.Observed = r.Reservation.Window.Start }, nil},
		{"last nanosecond is inside", func(r *Request) { r.Observed = temporal.InstantFromNanoseconds(9) }, nil},
		{"unset identity refuses", func(r *Request) { r.Reservation.Key = Key{} }, core.ErrRequestBudgetContract},
		{"unset start refuses", func(r *Request) { r.Reservation.Window.Start = temporal.Instant{} }, core.ErrRequestBudgetContract},
		{"unset end refuses", func(r *Request) { r.Reservation.Window.End = temporal.Instant{} }, core.ErrRequestBudgetContract},
		{"equal interval bounds refuse", func(r *Request) { r.Reservation.Window.End = r.Reservation.Window.Start }, core.ErrRequestBudgetContract},
		{"reversed interval bounds refuse", func(r *Request) { r.Reservation.Window.End = temporal.InstantFromNanoseconds(-1) }, core.ErrRequestBudgetContract},
		{"unset observation refuses", func(r *Request) { r.Observed = temporal.Instant{} }, core.ErrRequestBudgetContract},
		{"zero batch refuses", func(r *Request) { r.Reservation.Batch = 0 }, core.ErrRequestBudgetContract},
		{"maximum batch is representable", func(r *Request) { r.Reservation.Batch = math.MaxUint16 }, nil},
		{"minimum signed instant stays valid", func(r *Request) {
			r.Reservation.Window.Start = temporal.InstantFromNanoseconds(math.MinInt64)
			r.Observed = r.Reservation.Window.Start
		}, nil},
		{"maximum signed end stays valid", func(r *Request) {
			r.Reservation.Window.End = temporal.InstantFromNanoseconds(math.MaxInt64)
			r.Observed = temporal.InstantFromNanoseconds(math.MaxInt64 - 1)
		}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := fixtureRequest(t, 1)
			tc.change(&r)
			if err := r.Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("Request.Validate() error = %v, want %v", err, tc.wantErr)
			}
		})
	}
	for raw := uint32(0); raw <= math.MaxUint16; raw++ {
		got := (Capacity{keys: uint16(raw)}).Validate()
		wantValid := raw != 0
		if (got == nil) != wantValid {
			t.Fatalf("Capacity(%d).Validate() error = %v, want valid %t", raw, got, wantValid)
		}
		constructed, err := NewCapacity(uint16(raw))
		if (err == nil) != wantValid || (wantValid && constructed != (Capacity{keys: uint16(raw)})) || (!wantValid && constructed != (Capacity{})) {
			t.Fatalf("NewCapacity(%d) = (%v,%v), want exact value with valid %t", raw, constructed, err, wantValid)
		}
	}
	for raw := 0; raw <= math.MaxUint8; raw++ {
		got := Decision(raw).Validate()
		wantValid := Decision(raw) == Admitted || Decision(raw) == Exhausted
		if (got == nil) != wantValid || Decision(raw).IsValid() != wantValid || (Decision(raw).String() != "") != wantValid {
			t.Fatalf("Decision(%d).Validate() error = %v, want valid %t", raw, got, wantValid)
		}
	}
	reserve := func(_ context.Context, r Reservation) (Grant, error) { return Grant{Reservation: r, Credits: 1}, nil }
	if got, err := New(Capacity{}, reserve); got != nil || !errors.Is(err, core.ErrRequestBudgetContract) {
		t.Fatalf("New(zero capacity) = (%v,%v), want nil/contract", got, err)
	}
	if got, err := New(Capacity{keys: 1}, nil); got != nil || !errors.Is(err, core.ErrRequestBudgetContract) {
		t.Fatalf("New(nil callback) = (%v,%v), want nil/contract", got, err)
	}
	var absent *Executor
	if got, err := absent.Admit(context.Background(), fixtureRequest(t, 1)); got != decisionUnknown || !errors.Is(err, core.ErrRequestBudgetContract) {
		t.Fatalf("nil Admit() = (%v,%v), want zero/contract", got, err)
	}
	e := fixtureExecutor(t, 1, reserve)
	//lint:ignore SA1012 Nil context is deliberately hostile input; executor must return a typed contract error.
	if got, err := e.Admit(nil, fixtureRequest(t, 1)); got != decisionUnknown || !errors.Is(err, core.ErrRequestBudgetContract) {
		t.Fatalf("nil context Admit() = (%v,%v), want zero/contract", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := e.Admit(ctx, fixtureRequest(t, 1)); got != decisionUnknown || !errors.Is(err, context.Canceled) || e.slots[0] != (slot{}) {
		t.Fatalf("pre-canceled Admit() = (%v,%v,%+v), want zero/canceled/untouched", got, err, e.slots[0])
	}
	if got, err := NewKey([32]byte{}); got != (Key{}) || !errors.Is(err, core.ErrRequestBudgetContract) {
		t.Fatalf("NewKey(zero) = (%v,%v), want zero/contract", got, err)
	}
	key, err := NewKey([32]byte{1})
	if err != nil {
		t.Fatal(err)
	}
	got, err := key.Bytes()
	if got != ([32]byte{1}) || err != nil {
		t.Fatalf("Key.Bytes() = (%v,%v), want exact identity/nil", got, err)
	}
	if got, err := (Key{}).Bytes(); got != ([32]byte{}) || !errors.Is(err, core.ErrRequestBudgetContract) {
		t.Fatalf("unset Key.Bytes() = (%v,%v), want zero/contract", got, err)
	}
}

func TestProductionDataFlowInventoryAndBoundedOwnership(t *testing.T) {
	t.Parallel()
	// Test-only AST inventory avoids introducing production marker interfaces.
	// Protocol facts: Capacity,Key,Window,Reservation,Request,Grant; capability: Executor;
	// internal flow: slot. New production structs must be classified here.
	want := []string{"Capacity", "Executor", "Grant", "Key", "Request", "Reservation", "Window", "slot"}
	var got []string
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir() error = %v, want nil", err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, 0)
		if err != nil {
			t.Fatalf("ParseFile(%s) error = %v, want nil", entry.Name(), err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.TypeSpec:
				if _, ok := x.Type.(*ast.StructType); ok {
					got = append(got, x.Name.Name)
				}
			case *ast.MapType, *ast.InterfaceType:
				t.Errorf("production node %T in %s, want no new map/interface", n, entry.Name())
			case *ast.Ident:
				if x.Name == "any" {
					t.Errorf("production any in %s, want no loose payload", entry.Name())
				}
			}
			return true
		})
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("production structs = %v, want classified %v", got, want)
	}
}

func FuzzReservationCreditProviderBoundary(f *testing.F) {
	// Seeds come from validated typed production requests/grants, not JSON maps.
	key, err := NewKey([32]byte{1})
	if err != nil {
		f.Fatal(err)
	}
	res := Reservation{Key: key, Window: Window{Start: temporal.InstantFromNanoseconds(0), End: temporal.InstantFromNanoseconds(10)}, Batch: 2}
	for _, credits := range []uint16{0, 1, 2} {
		g := Grant{Reservation: res, Credits: credits}
		if err := g.Validate(); err != nil {
			f.Fatal(err)
		}
		f.Add(uint64(g.Credits), byte(0))
	}
	failure := errors.New("test provider refusal")
	f.Fuzz(func(t *testing.T, raw uint64, mutation byte) {
		r := fixtureRequest(t, 1)
		credits := uint16(raw)
		r.Reservation.Batch = uint16(raw >> 16)
		if r.Reservation.Batch == 0 {
			r.Reservation.Batch = 1
		}
		calls := 0
		e := fixtureExecutor(t, 1, func(_ context.Context, res Reservation) (Grant, error) {
			calls++
			if mutation&1 != 0 {
				res.Key, _ = NewKey([32]byte{2})
			}
			g := Grant{Reservation: res, Credits: credits}
			if mutation&2 != 0 {
				return g, failure
			}
			return g, nil
		})
		got, err := e.Admit(context.Background(), r)
		want := decisionUnknown
		var wantErr error
		switch {
		case mutation&2 != 0:
			wantErr = failure
		case credits > r.Reservation.Batch:
			wantErr = core.ErrRequestBudgetContract
		case mutation&1 != 0:
			wantErr = core.ErrRequestBudgetBinding
		case credits == 0:
			want = Exhausted
		default:
			want = Admitted
		}
		if got != want || !errors.Is(err, wantErr) || calls != 1 {
			t.Fatalf("provider Admit() = (%v,%v,%d calls), want (%v,%v,1)", got, err, calls, want, wantErr)
		}
		if wantErr != nil {
			if e.slots[0] != (slot{}) {
				t.Fatalf("rejected slot = %+v, want zero", e.slots[0])
			}
			return
		}
		wantCredits := credits
		if credits > 0 {
			wantCredits--
		}
		if e.slots[0].credits != wantCredits || e.slots[0].exhausted != (credits == 0) {
			t.Fatalf("retained grant = %+v, want credits %d/exhausted %t", e.slots[0], wantCredits, credits == 0)
		}
	})
}
