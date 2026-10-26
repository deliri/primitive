package hostfacts

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestResourceCPUCounterBoundaries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, text  string
		total, idle uint64
		bad         bool
	}{
		{"ordinary counters exclude guest double counting", "cpu 1 2 3 4 5 6 7 8 999 999", 36, 9, false},
		{"one busy tick", "cpu 1 0 0 0 0 0 0 0", 1, 0, false},
		{"all idle including IO wait", "cpu 0 0 0 1 1 0 0 0", 2, 2, false},
		{"maximum total remains representable", "cpu 18446744073709551615 0 0 0 0 0 0 0", math.MaxUint64, 0, false},
		{"maximum idle remains representable", "cpu 0 0 0 18446744073709551615 0 0 0 0", math.MaxUint64, math.MaxUint64, false},
		{"empty stream is not zero usage", "", 0, 0, true},
		{"individual CPU cannot become aggregate", "cpu0 1 2 3 4 5 6 7 8", 0, 0, true},
		{"truncated counters refuse", "cpu 1 2 3 4 5 6 7", 0, 0, true},
		{"negative tick refuses", "cpu -1 0 0 0 0 0 0 0", 0, 0, true},
		{"sum overflow cannot wrap idle", "cpu 18446744073709551615 1 0 0 0 0 0 0", 0, 0, true},
		{"idle sum overflow cannot wrap", "cpu 0 0 0 18446744073709551615 1 0 0 0", 0, 0, true},
		{"zero counters are absent observation", "cpu 0 0 0 0 0 0 0 0", 0, 0, true},
		{"malformed decimal refuses", "cpu 1 2 three 4 5 6 7 8", 0, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseResourceCPU([]byte(tc.text))
			if tc.bad {
				if !errors.Is(err, core.ErrHostFactsObservation) || got != (resourceCPUCounters{}) {
					t.Fatalf("parse=(%+v,%v), want zero refusal", got, err)
				}
				return
			}
			if err != nil || got.total != tc.total || got.idle != tc.idle {
				t.Fatalf("parse=(%+v,%v), want total %d idle %d", got, err, tc.total, tc.idle)
			}
		})
	}
}

func TestResourceIntervalLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		before, after resourceCPUCounters
		cpus          int
		want          uint64
		bad           bool
	}{
		{"half busy interval", resourceCPUCounters{100, 40}, resourceCPUCounters{200, 90}, 4, 2000, false},
		{"fully idle interval", resourceCPUCounters{1, 1}, resourceCPUCounters{11, 11}, 4, 0, false},
		{"fully busy interval", resourceCPUCounters{1, 0}, resourceCPUCounters{11, 0}, 4, 4000, false},
		{"no advancing clock refuses", resourceCPUCounters{1, 0}, resourceCPUCounters{1, 0}, 4, 0, true},
		{"counter reset refuses", resourceCPUCounters{11, 5}, resourceCPUCounters{1, 0}, 4, 0, true},
		{"idle reset refuses", resourceCPUCounters{10, 5}, resourceCPUCounters{20, 4}, 4, 0, true},
		{"idle delta exceeds total delta", resourceCPUCounters{10, 0}, resourceCPUCounters{20, 11}, 4, 0, true},
		{"missing CPU count refuses", resourceCPUCounters{1, 0}, resourceCPUCounters{11, 0}, 0, 0, true},
		{"maximum tick delta stays bounded", resourceCPUCounters{1, 0}, resourceCPUCounters{math.MaxUint64, 0}, 4, 4000, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := resourceBusy(tc.before, tc.after, LogicalCPUCount{count: tc.cpus})
			if tc.bad {
				if !errors.Is(err, core.ErrHostFactsObservation) || got != 0 {
					t.Fatalf("interval=(%d,%v), want zero refusal", got, err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("interval=(%d,%v), want %d", got, err, tc.want)
			}
		})
	}
}

func TestResourcePressureBoundaries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, text string
		want       uint32
		bad        bool
	}{
		{"zero pressure remains zero", "some avg10=0.00 avg60=0.00 total=0\n", 0, false},
		{"one millipercent retained", "some avg10=0.001\n", 1, false},
		{"whole percentage retained", "some avg10=1.00\n", 1000, false},
		{"last representable percentage", "some avg10=100\n", 100000, false},
		{"full pressure cannot replace some pressure", "full avg10=100\nsome avg10=2.50\n", 2500, false},
		{"empty pressure is unavailable", "", 0, true},
		{"missing interval is unavailable", "some avg60=1\n", 0, true},
		{"negative pressure refuses", "some avg10=-0.001\n", 0, true},
		{"above full pressure refuses", "some avg10=100.001\n", 0, true},
		{"NaN refuses", "some avg10=NaN\n", 0, true},
		{"infinity refuses", "some avg10=+Inf\n", 0, true},
		{"duplicate aggregate refuses", "some avg10=1\nsome avg10=2\n", 0, true},
		{"duplicate interval refuses", "some avg10=1 avg10=2\n", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseResourcePressure(t.Context(), strings.NewReader(tc.text))
			if tc.bad {
				if !errors.Is(err, core.ErrHostFactsObservation) || got != 0 {
					t.Fatalf("pressure=(%d,%v), want zero refusal", got, err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("pressure=(%d,%v), want %d", got, err, tc.want)
			}
		})
	}
}

func TestResourceUnknownRecordsStreamAcrossWindows(t *testing.T) {
	t.Parallel()
	for _, extent := range []int{resourceRecordBytes - 1, resourceRecordBytes, resourceRecordBytes + 1, resourceRecordBytes*32 + 7} {
		t.Run(strconv.Itoa(extent), func(t *testing.T) {
			t.Parallel()
			source := io.MultiReader(strings.NewReader(strings.Repeat("x", extent)), strings.NewReader("\nsome avg10=2.50\n"))
			got, err := parseResourcePressure(t.Context(), source)
			if err != nil || got != 2500 {
				t.Fatalf("multi-window pressure=(%d,%v), want 2500", got, err)
			}
		})
	}
}

func TestResourceUsageNativeLayerTriad(t *testing.T) {
	t.Parallel()
	platform, err := CurrentPlatform()
	if err != nil {
		t.Fatal(err)
	}
	if platform.OperatingSystem != core.OperatingSystemLinux && platform.OperatingSystem != core.OperatingSystemDarwin {
		t.Skip("native sample requires Linux or Darwin")
	}
	got, err := ObserveResourceUsage(t.Context(), ResourceUsageRequest{IntervalSeconds: 1})
	if err != nil || got.Validate() != nil || got.TotalMemoryBytes().Uint64() == 0 ||
		got.ProcessMemoryBytes().Uint64() == 0 || got.ProcessMemoryBytes().Uint64() > got.TotalMemoryBytes().Uint64() || got.BusyCPUMilli() > uint64(got.CPUCount().Int())*1000 {
		t.Fatalf("native resources=(%+v,%v), want admitted OS sample", got, err)
	}
	zero, err := ObserveResourceUsage(t.Context(), ResourceUsageRequest{})
	if !errors.Is(err, core.ErrHostFactsContract) || !emptyResourceUsage(zero) {
		t.Fatalf("zero request=(%+v,%v), want zero contract refusal", zero, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	zero, err = ObserveResourceUsage(ctx, ResourceUsageRequest{IntervalSeconds: 1})
	if !errors.Is(err, context.Canceled) || !emptyResourceUsage(zero) {
		t.Fatalf("cancelled sample=(%+v,%v), want no sample", zero, err)
	}
}

func FuzzResourcePressureSemanticClosure(f *testing.F) {
	f.Add([]byte("some avg10=0.00 avg60=0.00 total=0\n"))
	f.Add([]byte("some avg10=100\n"))
	f.Add([]byte(""))
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := parseResourcePressure(t.Context(), bytes.NewReader(data))
		if err != nil {
			if !errors.Is(err, core.ErrHostFactsObservation) || got != 0 {
				t.Fatalf("rejected pressure=(%d,%v), want zero refusal", got, err)
			}
			return
		}
		if got > resourcePercentMilli {
			t.Fatalf("accepted pressure=%d, want at most 100000", got)
		}
		canonical := []byte("some avg10=" + strconv.FormatFloat(float64(got)/1000, 'f', 3, 64) + "\n")
		again, err := parseResourcePressure(t.Context(), bytes.NewReader(canonical))
		if err != nil || again != got {
			t.Fatalf("pressure round trip=(%d,%v), want %d", again, err, got)
		}
	})
}

func FuzzResourceCPUSemanticClosure(f *testing.F) {
	f.Add([]byte("cpu 1 2 3 4 5 6 7 8"))
	f.Add([]byte("cpu 1 0 0 0 0 0 0 0"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := parseResourceCPU(data)
		if err != nil {
			if !errors.Is(err, core.ErrHostFactsObservation) || got != (resourceCPUCounters{}) {
				t.Fatalf("rejected CPU=(%+v,%v)", got, err)
			}
			return
		}
		if got.total == 0 || got.idle > got.total {
			t.Fatalf("accepted CPU violates conservation: %+v", got)
		}
		canonical := "cpu " + strconv.FormatUint(got.total-got.idle, 10) + " 0 0 " + strconv.FormatUint(got.idle, 10) + " 0 0 0 0"
		again, err := parseResourceCPU([]byte(canonical))
		if err != nil || again != got {
			t.Fatalf("CPU round trip=(%+v,%v), want %+v", again, err, got)
		}
	})
}

// emptyResourceUsage proves a refusal returned no partial sample, including
// no scope samples.
func emptyResourceUsage(u ResourceUsage) bool {
	return u.cpus.Int() == 0 && u.busy == 0 && u.total.Uint64() == 0 && u.available.Uint64() == 0 &&
		u.process.Uint64() == 0 && u.ioPressure == 0 && u.memoryPressure == 0 && u.scopes == nil
}
