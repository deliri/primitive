package hostfacts

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// The producer is Linux sysfs, which has no project encoder. Render typed
// counters in its documented 17-field schema; every seed enters the real fold.
func diskStatFixture(c diskActivityCounters) string {
	return fmt.Sprintf("0 0 %d 0 0 0 %d 0 %d %d 0 0 0 0 0 0 0\n", c.readSectors, c.writtenSectors, c.inFlight, c.busyMillis)
}

func TestDiskActivityCounterIngressLayerTriad(t *testing.T) {
	t.Parallel()
	ordinary := diskActivityCounters{readSectors: 3, writtenSectors: 7, inFlight: 9, busyMillis: 10}
	for _, row := range []struct {
		name    string
		input   string
		want    diskActivityCounters
		wantErr error
	}{
		{name: "documented current kernel fields", input: diskStatFixture(ordinary), want: ordinary},
		{name: "measured zero counters are present", input: diskStatFixture(diskActivityCounters{})},
		{name: "mandatory eleven-field prefix", input: "0 0 3 0 0 0 7 0 9 10 0", want: ordinary},
		{name: "future numeric fields preserve known positions", input: strings.TrimSpace(diskStatFixture(ordinary)) + " 999 333\n", want: ordinary},
		{name: "kernel leading alignment whitespace", input: " \t" + diskStatFixture(ordinary), want: ordinary},
		{name: "trailing whitespace is not a second record", input: diskStatFixture(ordinary) + " \t\r\n", want: ordinary},
		{name: "uint64 read counter survives", input: diskStatFixture(diskActivityCounters{readSectors: math.MaxUint64}), want: diskActivityCounters{readSectors: math.MaxUint64}},
		{name: "uint64 write counter survives", input: diskStatFixture(diskActivityCounters{writtenSectors: math.MaxUint64}), want: diskActivityCounters{writtenSectors: math.MaxUint64}},
		{name: "read-only activity is independent", input: diskStatFixture(diskActivityCounters{readSectors: 512}), want: diskActivityCounters{readSectors: 512}},
		{name: "write-only activity is independent", input: diskStatFixture(diskActivityCounters{writtenSectors: 512}), want: diskActivityCounters{writtenSectors: 512}},
		{name: "empty source cannot invent counters", wantErr: core.ErrHostFactsObservation},
		{name: "missing mandatory final field", input: "0 0 3 0 0 0 7 0 9 10", wantErr: core.ErrHostFactsObservation},
		{name: "signed counter is not an unsigned observation", input: "0 0 -3 0 0 0 7 0 9 10 0", wantErr: core.ErrHostFactsObservation},
		{name: "overflow does not wrap into plausible activity", input: "0 0 18446744073709551616 0 0 0 7 0 9 10 0", wantErr: core.ErrHostFactsObservation},
		{name: "duplicate snapshot is contradictory input", input: diskStatFixture(ordinary) + diskStatFixture(ordinary), wantErr: core.ErrHostFactsObservation},
		{name: "NUL cannot disappear into whitespace", input: strings.TrimSpace(diskStatFixture(ordinary)) + "\x00", wantErr: core.ErrHostFactsObservation},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseDiskActivity(t.Context(), strings.NewReader(row.input))
			if got != row.want || row.wantErr == nil && err != nil || row.wantErr != nil && !errors.Is(err, row.wantErr) {
				t.Fatalf("counter fold=%+v/%v, want %+v/%v", got, err, row.want, row.wantErr)
			}
		})
	}
}

type diskCounterFailureReader struct {
	prefix  io.Reader
	failure error
}

func (r diskCounterFailureReader) Read(p []byte) (int, error) {
	n, err := r.prefix.Read(p)
	if errors.Is(err, io.EOF) {
		return n, r.failure
	}
	return n, err
}

func TestDiskActivityIngressDoesNotPublishPartialOrUnownedInput(t *testing.T) {
	t.Parallel()
	cause := io.ErrClosedPipe
	got, err := parseDiskActivity(t.Context(), diskCounterFailureReader{prefix: strings.NewReader(diskStatFixture(diskActivityCounters{writtenSectors: 900})), failure: cause})
	if got != (diskActivityCounters{}) || !errors.Is(err, cause) {
		t.Fatalf("partial observation=%+v/%v, want zero and native read failure", got, err)
	}
	if got, err := parseDiskActivity(t.Context(), nil); got != (diskActivityCounters{}) || !errors.Is(err, core.ErrHostFactsContract) {
		t.Fatalf("nil reader=%+v/%v, want zero/contract", got, err)
	}
	ctx, cancel, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: t.Context()})
	if err != nil {
		t.Fatal(err)
	}
	cancel(cause)
	if got, err := parseDiskActivity(ctx, strings.NewReader(diskStatFixture(diskActivityCounters{}))); got != (diskActivityCounters{}) || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled reader=%+v/%v, want zero/cancellation", got, err)
	}
}

func TestDiskActivityArithmeticLayerTriad(t *testing.T) {
	t.Parallel()
	device, err := core.ParseAbsolutePath("/sys/devices/fixture/block/disk")
	if err != nil {
		t.Fatal(err)
	}
	window, err := temporal.DurationFromSeconds(1)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		name                  string
		before, after         diskActivityCounters
		window                temporal.Duration
		wantRead, wantWritten uint64
		wantBusy              uint32
		wantErr               error
	}{
		{name: "idle device is measured zero", window: window},
		{name: "512-byte sectors are not hardware block size", after: diskActivityCounters{readSectors: 1, writtenSectors: 2}, window: window, wantRead: 512, wantWritten: 1024},
		{name: "counter origins are subtracted", before: diskActivityCounters{readSectors: 100, writtenSectors: 200, busyMillis: 500}, after: diskActivityCounters{readSectors: 101, writtenSectors: 202, busyMillis: 1400}, window: window, wantRead: 512, wantWritten: 1024, wantBusy: 90000},
		{name: "one millisecond is not one percent", after: diskActivityCounters{busyMillis: 1}, window: window, wantBusy: 100},
		{name: "below ninety percent", after: diskActivityCounters{busyMillis: 899}, window: window, wantBusy: 89900},
		{name: "exact ninety percent", after: diskActivityCounters{busyMillis: 900}, window: window, wantBusy: 90000},
		{name: "above ninety percent", after: diskActivityCounters{busyMillis: 901}, window: window, wantBusy: 90100},
		{name: "exact full device window", after: diskActivityCounters{busyMillis: 1000}, window: window, wantBusy: 100000},
		{name: "jiffy overlap saturates the bounded busy fraction", after: diskActivityCounters{busyMillis: 1001}, window: window, wantBusy: 100000},
		{name: "extreme busy delta never wraps", after: diskActivityCounters{busyMillis: math.MaxUint64}, window: window, wantBusy: 100000},
		{name: "read counter reset has no published delta", before: diskActivityCounters{readSectors: 1}, window: window, wantErr: core.ErrHostFactsObservation},
		{name: "write counter reset has no published delta", before: diskActivityCounters{writtenSectors: 1}, window: window, wantErr: core.ErrHostFactsObservation},
		{name: "busy counter reset has no published percentage", before: diskActivityCounters{busyMillis: 1}, window: window, wantErr: core.ErrHostFactsObservation},
		{name: "sector multiplication overflow refuses", after: diskActivityCounters{readSectors: math.MaxUint64}, window: window, wantErr: core.ErrNumericOverflow},
		{name: "zero window has no rate", wantErr: core.ErrHostFactsObservation},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			// Both samples pass the actual kernel-text producer first.
			before, e := parseDiskActivity(t.Context(), strings.NewReader(diskStatFixture(row.before)))
			if e != nil {
				t.Fatal(e)
			}
			after, e := parseDiskActivity(t.Context(), strings.NewReader(diskStatFixture(row.after)))
			if e != nil {
				t.Fatal(e)
			}
			got, err := before.delta(device, after, row.window)
			if row.wantErr != nil {
				if !errors.Is(err, row.wantErr) || got != (DiskActivity{}) {
					t.Fatalf("delta=%+v/%v, want zero/%v", got, err, row.wantErr)
				}
				return
			}
			if err != nil || got.Validate() != nil || got.ReadBytes().Uint64() != row.wantRead || got.WrittenBytes().Uint64() != row.wantWritten || got.BusyMilli() != row.wantBusy || got.Device() != device || got.Window() != row.window || got.InFlight() != row.after.inFlight {
				t.Fatalf("delta=%+v/%v, want reads=%d writes=%d busy=%d on the observed device/window", got, err, row.wantRead, row.wantWritten, row.wantBusy)
			}
		})
	}
}

func FuzzDiskActivityKernelRecordSemantic(f *testing.F) {
	for _, c := range []diskActivityCounters{{}, {readSectors: 1, writtenSectors: 2, inFlight: 3, busyMillis: 900}, {readSectors: math.MaxUint64}} {
		wire := diskStatFixture(c)
		got, err := parseDiskActivity(f.Context(), strings.NewReader(wire))
		if err != nil || got != c {
			f.Fatalf("seed producer=%+v/%v, want %+v", got, err, c)
		}
		f.Add([]byte(wire))
	}
	f.Add([]byte{})
	f.Add([]byte("0 0 -1\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := parseDiskActivity(t.Context(), bytes.NewReader(data))
		if err != nil {
			if !errors.Is(err, core.ErrHostFactsObservation) || got != (diskActivityCounters{}) {
				t.Fatalf("refused kernel input=%+v/%v, want zero/observation refusal", got, err)
			}
			return
		}
		// Independent whitespace/token oracle, bounded by the fuzzer's owned
		// input; production itself never retains the token collection.
		fields := strings.Fields(string(data))
		if len(fields) < 11 {
			t.Fatalf("accepted %d fields, want mandatory counters", len(fields))
		}
		var want diskActivityCounters
		for index, field := range fields {
			value, err := strconv.ParseUint(field, 10, 64)
			if err != nil {
				t.Fatalf("accepted field %d=%q, independent decimal parser refuses: %v", index, field, err)
			}
			switch index {
			case 2:
				want.readSectors = value
			case 6:
				want.writtenSectors = value
			case 8:
				want.inFlight = value
			case 9:
				want.busyMillis = value
			}
		}
		if got != want {
			t.Fatalf("kernel fields=%+v, want independent numeric projection %+v", got, want)
		}
		canonical := diskStatFixture(got)
		again, err := parseDiskActivity(t.Context(), strings.NewReader(canonical))
		if err != nil || again != got {
			t.Fatalf("canonical fold=%+v/%v, want %+v", again, err, got)
		}
		if diskStatFixture(again) != canonical {
			t.Fatalf("second canonical rendering=%q, want %q", diskStatFixture(again), canonical)
		}
	})
}

type diskZeroReader struct{}

func (diskZeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = '0'
	}
	return len(p), nil
}

func TestDiskActivityFoldStreamsBeyondItsWindow(t *testing.T) {
	t.Parallel()
	for _, extent := range []int64{4095, 4096, 4097, 4*4096 + 13, 1 << 20} {
		t.Run(fmt.Sprint(extent), func(t *testing.T) {
			t.Parallel()
			input := io.MultiReader(strings.NewReader("0 0 "), io.LimitReader(diskZeroReader{}, extent), strings.NewReader("7 0 0 0 9 0 0 10 0\n"))
			got, err := parseDiskActivity(t.Context(), input)
			want := diskActivityCounters{readSectors: 7, writtenSectors: 9, busyMillis: 10}
			if err != nil || got != want {
				t.Fatalf("%d-byte token across reader windows=%+v/%v, want %+v/nil", extent, got, err, want)
			}
		})
	}
}

func TestDiskActivityPublicNativeLayerTriad(t *testing.T) {
	t.Parallel()
	root := mustAbsolutePathForHostfactsTest(t, t.TempDir())
	interval, err := temporal.DurationFromMilliseconds(1)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ObserveDiskActivity(t.Context(), DiskActivityRequest{Directory: root, Interval: interval})
	if errors.Is(err, core.ErrHostFactsUnsupported) {
		if got != (DiskActivity{}) {
			t.Fatalf("unsupported host=%+v, want no fabricated sample", got)
		}
	} else if err != nil || got.Validate() != nil || got.Window().IsZero() {
		t.Fatalf("native sample=%+v/%v, want an observed device interval", got, err)
	}
	if got, err := ObserveDiskActivity(t.Context(), DiskActivityRequest{Directory: root}); got != (DiskActivity{}) || !errors.Is(err, core.ErrHostFactsContract) {
		t.Fatalf("zero interval=%+v/%v, want zero/contract", got, err)
	}
	ctx, cancel, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: t.Context()})
	if err != nil {
		t.Fatal(err)
	}
	cancel(context.Canceled)
	if got, err := ObserveDiskActivity(ctx, DiskActivityRequest{Directory: root, Interval: interval}); got != (DiskActivity{}) || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled sample=%+v/%v, want zero/cancellation", got, err)
	}
}
