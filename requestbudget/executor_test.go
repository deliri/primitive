package requestbudget

import (
	"context"
	"github.com/deliri/primitive/v2026/temporal"
	"testing"
)

func TestReservationCreditsLayerTriad(t *testing.T) {
	t.Parallel()
	key, err := NewKey([32]byte{1})
	if err != nil {
		t.Fatalf("NewKey() error = %v, want nil", err)
	}
	calls := 0
	capacity, err := NewCapacity(1)
	if err != nil {
		t.Fatal(err)
	}
	executor, err := New(capacity, func(_ context.Context, reservation Reservation) (Grant, error) {
		calls++
		credits := uint16(2)
		if calls > 1 {
			credits = 0
		}
		return Grant{Reservation: reservation, Credits: credits}, nil
	})
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	request := Request{Reservation: Reservation{Key: key, Window: Window{Start: temporal.InstantFromNanoseconds(0), End: temporal.InstantFromNanoseconds(10)}, Batch: 2}, Observed: temporal.InstantFromNanoseconds(1)}
	for _, want := range []Decision{Admitted, Admitted, Exhausted, Exhausted} {
		got, err := executor.Admit(context.Background(), request)
		if err != nil || got != want {
			t.Fatalf("Admit() = (%v, %v), want (%v, nil)", got, err, want)
		}
	}
	if calls != 2 {
		t.Fatalf("reservation calls = %d, want 2", calls)
	}
}
