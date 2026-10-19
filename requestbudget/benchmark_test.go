package requestbudget

import (
	"context"
	"testing"

	"github.com/deliri/primitive/v2026/temporal"
)

// benchmarkBudgetKeys fills a 256-key executor. Admission places each new key
// in the last reusable slot, so the first fixture key lands in the final slot
// and every measured admission scans the whole table before reaching it.
const benchmarkBudgetKeys = 256

func benchmarkBudgetRequest(index int) Request {
	return Request{
		Reservation: Reservation{
			Key:    Key{value: [32]byte{byte(index), byte(index >> 8), 1}},
			Window: Window{Start: temporal.InstantFromNanoseconds(0), End: temporal.InstantFromNanoseconds(1 << 40)},
			Batch:  1,
		},
		Observed: temporal.InstantFromNanoseconds(1),
	}
}

// BenchmarkAdmitFullTable measures one per-request admission against a full
// slot table: validation, high-water observation, the slot scan, and one
// durable reservation callback (Batch 1, so every admission reserves). The
// callback is an in-memory grant; no durable store is contacted.
func BenchmarkAdmitFullTable(b *testing.B) {
	capacity, err := NewCapacity(benchmarkBudgetKeys)
	if err != nil {
		b.Fatal(err)
	}
	var reservations uint64
	executor, err := New(capacity, func(_ context.Context, reservation Reservation) (Grant, error) {
		reservations++
		return Grant{Reservation: reservation, Credits: reservation.Batch}, nil
	})
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	for index := range benchmarkBudgetKeys {
		if got, err := executor.Admit(ctx, benchmarkBudgetRequest(index)); err != nil || got != Admitted {
			b.Fatalf("fixture Admit(%d) = (%v,%v), want admitted", index, got, err)
		}
	}
	request := benchmarkBudgetRequest(0)
	if executor.slots[benchmarkBudgetKeys-1].reservation.Key != request.Reservation.Key {
		b.Fatal("measured key is not in the final slot, want a full-table scan")
	}
	if err := request.Validate(); err != nil {
		b.Fatal(err)
	}
	reservations = 0
	b.ReportAllocs()
	for b.Loop() {
		if got, err := executor.Admit(ctx, request); err != nil || got != Admitted {
			b.Fatalf("Admit() = (%v,%v), want admitted", got, err)
		}
	}
	if reservations != uint64(b.N) {
		b.Fatalf("reservations = %d, want one per admission (%d)", reservations, b.N)
	}
}
