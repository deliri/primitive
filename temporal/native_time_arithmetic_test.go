package temporal

import (
	"fmt"
	"math"
	"testing"
	"time"
)

func TestNativeTimeDifferencePreservesSignedSaturationAndMonotonicReadings(t *testing.T) {
	t.Parallel()
	values := nativeTimeCarriersForTest(t)
	values = append(values, time.Unix(math.MinInt64, 0), time.Unix(math.MaxInt64, 999999999))
	for index, first := range values {
		for secondIndex, second := range values {
			t.Run(fmt.Sprintf("%d/%d", index, secondIndex), func(t *testing.T) {
				t.Parallel()
				request := NativeTimePair{First: first, Second: second}
				before := request
				want := first.Sub(second)
				got := DifferenceNativeTimes(request)
				if got != want || request != before {
					t.Fatalf("native difference=%d,want%d; carriers changed=%t", got, want, request != before)
				}
				// Signed saturation is native authority. The subtraction must
				// never wrap when endpoints are farther apart than Duration.
				if first.After(second) && got <= 0 || first.Before(second) && got >= 0 {
					t.Fatalf("difference lost native order=%d", got)
				}
			})
		}
	}
}

func TestNativeTimeShiftPreservesSignedOffsetsLocationAndCalendarRange(t *testing.T) {
	t.Parallel()
	values := nativeTimeCarriersForTest(t)
	values = append(values, time.Unix(math.MinInt64, 0), time.Unix(math.MaxInt64, 999999999))
	for index, value := range values {
		for _, offset := range []time.Duration{math.MinInt64, math.MinInt64 + 1, -24 * time.Hour, -time.Second, -1, 0, 1, time.Second, 24 * time.Hour, math.MaxInt64 - 1, math.MaxInt64} {
			t.Run(fmt.Sprintf("%d/%d", index, offset), func(t *testing.T) {
				t.Parallel()
				request := NativeTimeOffset{Time: value, Offset: offset}
				before := request
				want := value.Add(offset)
				got := ShiftNativeTime(request)
				if got != want || request != before {
					t.Fatalf("native shift=%#v,want%#v; carriers changed=%t", got, want, request != before)
				}
				if got.Location() != value.Location() {
					t.Fatal("shift changed native location identity")
				}
			})
		}
	}
}

func FuzzNativeTimeArithmeticMatchesGoCalendarAndSaturation(f *testing.F) {
	for _, seconds := range []int64{math.MinInt64, -62167219200, -1, 0, 1, 253402300799, math.MaxInt64} {
		for _, offset := range []int64{math.MinInt64, -1, 0, 1, math.MaxInt64} {
			f.Add(seconds, int64(123456789), offset, int32(-18000))
		}
	}
	f.Fuzz(func(t *testing.T, seconds, nanos, offset int64, zoneOffset int32) {
		location := time.FixedZone("native", int(zoneOffset))
		value := time.Unix(seconds, nanos).In(location)
		shifted := ShiftNativeTime(NativeTimeOffset{Time: value, Offset: time.Duration(offset)})
		want := value.Add(time.Duration(offset))
		if shifted != want || shifted.Location() != location {
			t.Fatalf("native shift=%#v,want%#v", shifted, want)
		}
		pair := NativeTimePair{First: shifted, Second: value}
		got := DifferenceNativeTimes(pair)
		if got != shifted.Sub(value) {
			t.Fatalf("native signed difference=%d,want%d", got, shifted.Sub(value))
		}
		// Comparing arbitrary calendar endpoints also attacks the duration
		// saturation path, rather than only testing add/subtract round trips.
		other := time.Unix(offset, seconds).UTC()
		pair = NativeTimePair{First: value, Second: other}
		if got := DifferenceNativeTimes(pair); got != value.Sub(other) {
			t.Fatalf("native saturated difference=%d,want%d", got, value.Sub(other))
		}
	})
}
