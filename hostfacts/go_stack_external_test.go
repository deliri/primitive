package hostfacts_test

import (
	"context"
	"errors"
	"reflect"
	"runtime"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestResolveGoProgramCounterMatchesRecordedGoFrame(t *testing.T) {
	t.Parallel()
	var pcs [1]uintptr
	if runtime.Callers(1, pcs[:]) != 1 {
		t.Fatal("recorded program counter is unavailable")
	}
	want, _ := runtime.CallersFrames(pcs[:]).Next()
	got, err := hostfacts.ResolveGoProgramCounter(t.Context(), hostfacts.GoProgramCounter(pcs[0]))
	if err != nil || got.Validate() != nil || uintptr(got.PC) != want.PC || string(got.Function) != want.Function || string(got.File) != want.File || got.Line != want.Line {
		t.Fatalf("recorded frame = %+v/%v, want Go observation %+v", got, err, want)
	}
	if frame, err := hostfacts.ResolveGoProgramCounter(t.Context(), 0); frame != (hostfacts.GoStackFrame{}) || !errors.Is(err, core.ErrHostFactsObservation) {
		t.Fatalf("zero coordinate = %+v/%v, want typed refusal", frame, err)
	}
	if frame, err := hostfacts.ResolveGoProgramCounter(nil, hostfacts.GoProgramCounter(pcs[0])); frame != (hostfacts.GoStackFrame{}) || !errors.Is(err, core.ErrNilContext) {
		t.Fatalf("nil context = %+v/%v, want typed refusal", frame, err)
	}
	ctx, cancel, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: t.Context()})
	if err != nil {
		t.Fatal(err)
	}
	defer cancel(nil)
	cancel(nil)
	if frame, err := hostfacts.ResolveGoProgramCounter(ctx, hostfacts.GoProgramCounter(pcs[0])); frame != (hostfacts.GoStackFrame{}) || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled observation = %+v/%v, want no frame", frame, err)
	}
}

func FuzzResolveGoProgramCounterMatchesRuntime(f *testing.F) {
	f.Add(uint8(0))
	f.Add(uint8(1))
	f.Add(uint8(63))
	f.Fuzz(func(t *testing.T, index uint8) {
		var pcs [64]uintptr
		count := runtime.Callers(1, pcs[:])
		pc := hostfacts.GoProgramCounter(pcs[int(index)%len(pcs)])
		got, err := hostfacts.ResolveGoProgramCounter(t.Context(), pc)
		if int(index)%len(pcs) >= count {
			if got != (hostfacts.GoStackFrame{}) || !errors.Is(err, core.ErrHostFactsObservation) {
				t.Fatalf("absent coordinate = %+v/%v", got, err)
			}
			return
		}
		want, _ := runtime.CallersFrames([]uintptr{uintptr(pc)}).Next()
		if err != nil || got.Validate() != nil || uintptr(got.PC) != want.PC || string(got.Function) != want.Function || string(got.File) != want.File || got.Line != want.Line {
			t.Fatalf("resolved recorded PC = %+v/%v, want Go frame %+v", got, err, want)
		}
	})
}

func TestCurrentGoStackFramesCrossesEveryWorkingWindow(t *testing.T) {
	t.Parallel()
	for _, depth := range []int{0, 1, 31, 32, 33, 64, 127, 256} {
		t.Run(stringDepth(depth), func(t *testing.T) {
			t.Parallel()
			compareRecursiveStack(t, depth)
		})
	}
}

func stringDepth(depth int) string {
	// A test name is a diagnostic label, not an effect or source identity.
	return "depth=" + strconv.Itoa(depth)
}

// This bounded hostile fixture compares against Go's independent full-stack
// observation. The production iterator must cross many fixed working windows.
func compareRecursiveStack(t *testing.T, depth int) {
	if depth > 0 {
		compareRecursiveStack(t, depth-1)
		return
	}
	target := runtime.FuncForPC(reflect.ValueOf(compareRecursiveStack).Pointer()).Name()
	var expectedPCs [512]uintptr
	count := runtime.Callers(0, expectedPCs[:])
	if count == len(expectedPCs) {
		t.Fatal("independent oracle buffer saturated")
	}
	oracle := runtime.CallersFrames(expectedPCs[:count])
	var want []runtime.Frame
	for {
		frame, more := oracle.Next()
		if frame.Function == target {
			want = append(want, frame)
		}
		if !more {
			break
		}
	}
	index := 0
	for frame, err := range hostfacts.CurrentGoStackFrames(t.Context()) {
		if err != nil {
			t.Fatal(err)
		}
		if err := frame.Validate(); err != nil {
			t.Fatal(err)
		}
		if string(frame.Function) != target {
			continue
		}
		if index >= len(want) || string(frame.File) != want[index].File || frame.Line <= 0 {
			t.Fatalf("recursive frame %d = %+v, want oracle source coordinate", index, frame)
		}
		index++
	}
	if index != len(want) || index == 0 {
		t.Fatalf("recursive frames = %d, want %d independent Go frames", index, len(want))
	}
}

func TestCurrentGoStackFramesLifetimeAndRefusal(t *testing.T) {
	t.Parallel()
	parent, cancel, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: t.Context()})
	if err != nil {
		t.Fatal(err)
	}
	defer cancel(nil)
	observations, refusals := 0, 0
	for frame, err := range hostfacts.CurrentGoStackFrames(parent) {
		if err != nil {
			refusals++
			if !errors.Is(err, context.Canceled) || frame != (hostfacts.GoStackFrame{}) {
				t.Fatalf("terminal observation = %+v/%v", frame, err)
			}
			continue
		}
		observations++
		cancel(nil)
	}
	if observations != 1 || refusals != 1 {
		t.Fatalf("canceled iterator = %d observations/%d refusals, want 1/1", observations, refusals)
	}
	stopped := 0
	for _, err := range hostfacts.CurrentGoStackFrames(t.Context()) {
		if err != nil {
			t.Fatal(err)
		}
		stopped++
		break
	}
	if stopped != 1 {
		t.Fatalf("consumer stop = %d observations, want 1", stopped)
	}
	for frame, err := range hostfacts.CurrentGoStackFrames(nil) {
		if !errors.Is(err, core.ErrNilContext) || frame != (hostfacts.GoStackFrame{}) {
			t.Fatalf("nil context = %+v/%v, want typed refusal", frame, err)
		}
	}
	for _, frame := range []hostfacts.GoStackFrame{{}, {PC: 1, Line: -1}} {
		if !errors.Is(frame.Validate(), core.ErrHostFactsObservation) {
			t.Fatalf("fabricated frame %+v admitted", frame)
		}
	}
	if err := (hostfacts.GoStackFrame{PC: 1}).Validate(); err != nil {
		t.Fatalf("Go unavailable source coordinates = %v, want valid observation", err)
	}
}

func FuzzCurrentGoStackFramesMatchesRuntime(f *testing.F) {
	for _, depth := range []uint8{0, 1, 31, 32, 33, 64, 127, 255} {
		f.Add(depth)
	}
	f.Fuzz(func(t *testing.T, depth uint8) { compareRecursiveStack(t, int(depth)) })
}
