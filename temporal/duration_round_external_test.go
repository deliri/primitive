package temporal_test

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestDurationRoundUsesNativeGoAtQuantumAndRepresentationBoundaries(t *testing.T) {
	t.Parallel()
	values := []int64{0, 1, 499, 500, 501, 999, 1000, 499999, 500000, 500001, 999999, 1000000, 499999999, 500000000, 500000001, 999999999, 1000000000, math.MaxInt64 - 1000000000, math.MaxInt64 - 500000000, math.MaxInt64}
	precisions := []struct {
		value   temporal.Precision
		quantum time.Duration
	}{
		{temporal.PrecisionNanosecond, time.Nanosecond},
		{temporal.PrecisionMicrosecond, time.Microsecond},
		{temporal.PrecisionMillisecond, time.Millisecond},
		{temporal.PrecisionSecond, time.Second},
	}
	for _, ns := range values {
		for _, precision := range precisions {
			t.Run(fmt.Sprintf("%dns/%s", ns, precision.quantum), func(t *testing.T) {
				t.Parallel()
				input, err := temporal.DurationFromNanoseconds(ns)
				if err != nil {
					t.Fatal(err)
				}
				got, err := input.Round(precision.value)
				want := time.Duration(ns).Round(precision.quantum)
				if err != nil || got.Nanoseconds() != int64(want) {
					t.Fatalf("Round(%d,%v) = (%v,%v), want %d native ns", ns, precision.value, got, err, want)
				}
				text, err := got.Text()
				if err != nil || text != want.String() {
					t.Fatalf("rounded text = (%q,%v), want %q", text, err, want.String())
				}
				wire, err := json.Marshal(got)
				oracle, oracleErr := json.Marshal(fmt.Sprintf("%d", want))
				if err != nil || oracleErr != nil || string(wire) != string(oracle) {
					t.Fatalf("rounded JSON v2 = (%s,%v), want (%s,%v)", wire, err, oracle, oracleErr)
				}
				if input.Nanoseconds() != ns {
					t.Fatalf("Round changed input from %d to %d", ns, input.Nanoseconds())
				}
			})
		}
	}
}

func TestDurationRoundAdmitsOnlyCompiledPrecisionDomain(t *testing.T) {
	t.Parallel()
	input, err := temporal.DurationFromNanoseconds(1234567890)
	if err != nil {
		t.Fatal(err)
	}
	for raw := 0; raw < 256; raw++ {
		got, err := input.Round(temporal.Precision(raw))
		admitted := raw >= 1 && raw <= 4
		if admitted && err != nil {
			t.Fatalf("Round precision %d = %v, want nil", raw, err)
		}
		if !admitted && (!errors.Is(err, core.ErrTemporalContract) || got.Nanoseconds() != 0) {
			t.Fatalf("Round precision %d = (%v,%v), want zero and temporal contract", raw, got, err)
		}
	}
}

func FuzzDurationRoundMatchesNativeGo(f *testing.F) {
	for _, ns := range []int64{-1, 0, 1, 500, 500000, 500000000, math.MaxInt64} {
		for _, precision := range []uint8{0, 1, 2, 3, 4, 5, 255} {
			f.Add(ns, precision)
		}
	}
	f.Fuzz(func(t *testing.T, ns int64, precision uint8) {
		input, err := temporal.DurationFromNanoseconds(ns)
		if ns < 0 {
			if !errors.Is(err, core.ErrTemporalContract) {
				t.Fatalf("negative duration admission = %v, want temporal contract", err)
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		got, err := input.Round(temporal.Precision(precision))
		if precision < 1 || precision > 4 {
			if !errors.Is(err, core.ErrTemporalContract) || got.Nanoseconds() != 0 {
				t.Fatalf("unadmitted precision = (%v,%v)", got, err)
			}
			return
		}
		quantum := [...]time.Duration{0, time.Nanosecond, time.Microsecond, time.Millisecond, time.Second}[precision]
		want := time.Duration(ns).Round(quantum)
		if err != nil || got.Nanoseconds() != int64(want) {
			t.Fatalf("Round(%d,%d) = (%v,%v), want native %d", ns, precision, got, err, want)
		}
	})
}

func BenchmarkDurationRoundNative(b *testing.B) {
	values := [...]int64{499999999, 500000000, 500000001, math.MaxInt64}
	var observed temporal.Duration
	var count uint64
	b.ReportAllocs()
	for b.Loop() {
		input, err := temporal.DurationFromNanoseconds(values[count%uint64(len(values))])
		if err != nil {
			b.Fatal(err)
		}
		observed, err = input.Round(temporal.PrecisionSecond)
		if err != nil {
			b.Fatal(err)
		}
		count++
	}
	if count == 0 {
		b.Fatal("no observed rounding")
	}
	want := time.Duration(values[(count-1)%uint64(len(values))]).Round(time.Second)
	if observed.Nanoseconds() != int64(want) {
		b.Fatalf("observed rounded value = %d, want native %d after %d operations", observed.Nanoseconds(), want, count)
	}
}
