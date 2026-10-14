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

func FuzzNativeTimePairMatchesNativeGo(f *testing.F) {
	for _, first := range []int64{math.MinInt64, -1, 0, 1, math.MaxInt64} {
		for _, second := range []int64{math.MinInt64, -1, 0, 1, math.MaxInt64} {
			f.Add(first, second)
		}
	}
	f.Fuzz(func(t *testing.T, first, second int64) {
		request := NativeTimePair{First: time.Unix(0, first).UTC(), Second: time.Unix(0, second).UTC()}
		want := core.ComparisonEqual
		if first < second {
			want = core.ComparisonLess
		} else if first > second {
			want = core.ComparisonGreater
		}
		if got := CompareNativeTimes(request); got != want {
			t.Fatalf("native comparison=%v, want integer order %v", got, want)
		}
	})
}

func BenchmarkNativeTimePairObservableComparison(b *testing.B) {
	request := NativeTimePair{First: time.Unix(0, 1), Second: time.Unix(0, 0)}
	var got core.Comparison
	var operations uint64
	b.ReportAllocs()
	for b.Loop() {
		got = CompareNativeTimes(request)
		operations++
	}
	if operations != uint64(b.N) || got != core.ComparisonGreater {
		b.Fatalf("native comparison=%v after %d operations, want greater/%d", got, operations, b.N)
	}
}
