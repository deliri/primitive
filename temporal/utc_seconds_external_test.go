package temporal_test

import (
	"fmt"
	"github.com/deliri/primitive/v2026/temporal"
	"math"
	"testing"
	"time"
)

func TestCompactUTCSecondsPreservesNativeProjectionAtRepresentationEdges(t *testing.T) {
	t.Parallel()
	for _, base := range []int64{math.MinInt64, -1000000001, -1000000000, -1, 0, 1, 1000000000, math.MaxInt64} {
		for offset := range 8 {
			nanos := base + int64(offset)
			t.Run(fmt.Sprint(nanos), func(t *testing.T) {
				instant := temporal.InstantFromNanoseconds(nanos)
				got, err := instant.CompactUTCSeconds()
				if err != nil {
					t.Fatal(err)
				}
				if want := time.Unix(0, nanos).UTC().Format("20060102T150405Z"); got != want {
					t.Fatalf("second projection=%q want=%q", got, want)
				}
				after, err := instant.Nanoseconds()
				if err != nil || after != nanos {
					t.Fatal("text projection altered instant")
				}
			})
		}
	}
	if _, err := (temporal.Instant{}).CompactUTCSeconds(); err == nil {
		t.Fatal("unset instant has no refusal")
	}
}

func FuzzCompactUTCSecondsUsesNativeSecondProjection(f *testing.F) {
	for _, nanos := range []int64{math.MinInt64, -1000000001, -1, 0, 1, 1000000001, math.MaxInt64} {
		f.Add(nanos)
	}
	f.Fuzz(func(t *testing.T, nanos int64) {
		got, err := temporal.InstantFromNanoseconds(nanos).CompactUTCSeconds()
		if err != nil {
			t.Fatal(err)
		}
		if want := time.Unix(0, nanos).UTC().Format("20060102T150405Z"); got != want {
			t.Fatalf("second projection=%q want=%q", got, want)
		}
	})
}
