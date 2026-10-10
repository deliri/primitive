package temporal

import (
	"fmt"
	"github.com/deliri/primitive/v2026/core"
	"math"
	"strings"
	"testing"
	"time"
)

func nativeTimeCarriersForTest(t *testing.T) []time.Time {
	t.Helper()
	observed, err := Observe()
	if err != nil {
		t.Fatal(err)
	}
	// The admitted clock only supplies a native monotonic carrier. Every expected
	// result below is computed independently by Go, without waiting or timing assertions.
	return []time.Time{
		{}, time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(-1, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(1000, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC),
		time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC), time.Unix(0, math.MinInt64), time.Unix(0, math.MaxInt64),
		time.Unix(0, -1), time.Unix(0, 0), time.Unix(0, 1),
		time.Date(2024, 2, 29, 23, 59, 59, 999999999, time.UTC),
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.FixedZone("west", -18000)),
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.FixedZone("east", 20700)),
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.FixedZone("", 0)),
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.FixedZone(" zone\x00\n", -1)),
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.FixedZone(strings.Repeat("z", 65536), 1)),
		observed.value, observed.value.Add(time.Nanosecond), observed.value.Add(-time.Nanosecond),
	}
}

func TestNativeTimePairMatchesGoSignedSaturationOrderingAndMonotonicCarriers(t *testing.T) {
	t.Parallel()
	values := nativeTimeCarriersForTest(t)
	for i, first := range values {
		for _, second := range []struct {
			name  string
			value time.Time
		}{
			{name: "same", value: first}, {name: "next", value: values[(i+1)%len(values)]}, {name: "opposite extent", value: values[(i+len(values)/2)%len(values)]},
		} {
			t.Run(fmt.Sprint(i)+"/"+second.name, func(t *testing.T) {
				t.Parallel()
				request := NativeTimePair{First: first, Second: second.value}
				original := request
				if got, want := DifferenceNativeTimes(request), first.Sub(second.value); got != want {
					t.Fatalf("difference = %v, want native %v", got, want)
				}
				want := core.ComparisonEqual
				if first.Before(second.value) {
					want = core.ComparisonLess
				} else if first.After(second.value) {
					want = core.ComparisonGreater
				}
				if got := CompareNativeTimes(request); got != want || !got.IsValid() {
					t.Fatalf("comparison = %v, want native order %v", got, want)
				}
				if request != original {
					t.Fatal("pair operation changed the borrowed carriers")
				}
			})
		}
	}
}

func TestNativeTimeOffsetPreservesGoFullCalendarAndSignedOffsetDomain(t *testing.T) {
	t.Parallel()
	values := nativeTimeCarriersForTest(t)
	for i, value := range values {
		for _, offset := range []time.Duration{math.MinInt64, -1, 0, 1, math.MaxInt64} {
			t.Run(fmt.Sprint(i)+"/"+fmt.Sprint(int64(offset)), func(t *testing.T) {
				t.Parallel()
				request := NativeTimeOffsetRequest{Time: value, Offset: offset}
				original := request
				if got, want := OffsetNativeTime(request), value.Add(offset); got != want {
					t.Fatalf("offset = %v, want exact native carrier %v", got, want)
				}
				if request != original {
					t.Fatal("offset operation changed the borrowed carrier")
				}
			})
		}
	}
}

func TestNativeTimeFactsPreserveGoZonesEmptyNamesAndZeroValue(t *testing.T) {
	t.Parallel()
	for i, value := range nativeTimeCarriersForTest(t) {
		for _, carrier := range []struct {
			name  string
			value time.Time
		}{
			{name: "native zone", value: value}, {name: "same instant UTC", value: value.UTC()}, {name: "same instant empty zone", value: value.In(time.FixedZone("", 0))},
		} {
			t.Run(fmt.Sprint(i)+"/"+carrier.name, func(t *testing.T) {
				t.Parallel()
				name, offset := carrier.value.Zone()
				want := NativeTimeFacts{ZoneName: name, ZoneOffsetSeconds: offset, Zero: carrier.value.IsZero()}
				if got := IsNativeTimeZero(carrier.value); got != want.Zero {
					t.Fatalf("zero predicate = %t, want Go %t", got, want.Zero)
				}
				if got := InspectNativeTime(carrier.value); got != want {
					t.Fatalf("native facts = %+v, want Go %+v", got, want)
				}
			})
		}
	}
}

func FuzzNativeTimePairAndOffsetMatchNativeGo(f *testing.F) {
	for _, first := range []int64{math.MinInt64, -1, 0, 1, math.MaxInt64} {
		for _, second := range []int64{math.MinInt64, -1, 0, 1, math.MaxInt64} {
			f.Add(first, second, int64(0))
			f.Add(first, second, int64(math.MinInt64))
			f.Add(first, second, int64(math.MaxInt64))
		}
	}
	f.Fuzz(func(t *testing.T, first, second, offset int64) {
		left, right := time.Unix(0, first).UTC(), time.Unix(0, second).UTC()
		request := NativeTimePair{First: left, Second: right}
		if got, want := DifferenceNativeTimes(request), left.Sub(right); got != want {
			t.Fatalf("difference = %v, want Go %v", got, want)
		}
		want := core.ComparisonEqual
		if first < second {
			want = core.ComparisonLess
		} else if first > second {
			want = core.ComparisonGreater
		}
		if got := CompareNativeTimes(request); got != want {
			t.Fatalf("comparison = %v, want integer oracle %v", got, want)
		}
		if got, want := OffsetNativeTime(NativeTimeOffsetRequest{Time: left, Offset: time.Duration(offset)}), left.Add(time.Duration(offset)); got != want {
			t.Fatalf("offset = %v, want exact Go %v", got, want)
		}
	})
}

func BenchmarkNativeTimePairObservableOperations(b *testing.B) {
	first, second := time.Unix(0, 1), time.Unix(0, 0)
	var difference time.Duration
	var comparison core.Comparison
	var shifted time.Time
	var facts NativeTimeFacts
	var operations uint64
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		request := NativeTimePair{First: first, Second: second}
		difference = DifferenceNativeTimes(request)
		comparison = CompareNativeTimes(request)
		shifted = OffsetNativeTime(NativeTimeOffsetRequest{Time: second, Offset: difference})
		facts = InspectNativeTime(shifted)
		operations++
	}
	b.StopTimer()
	if operations != uint64(b.N) || difference != time.Nanosecond || comparison != core.ComparisonGreater || shifted != first || facts.Zero {
		b.Fatalf("observed operations %d, outputs %v/%v/%v/%+v", operations, difference, comparison, shifted, facts)
	}
}
