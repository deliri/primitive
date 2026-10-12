package hostfacts

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
)

func TestKernelObservationLayerTriadRequiresEveryNativeField(t *testing.T) {
	t.Parallel()
	for shape := range 8 {
		t.Run(fmt.Sprintf("field_presence_%03b", shape), func(t *testing.T) {
			t.Parallel()
			var got KernelObservation
			if shape&1 != 0 {
				got.System = KernelText{value: "Linux"}
			}
			if shape&2 != 0 {
				got.Release = KernelText{value: "7.2.8-build"}
			}
			if shape&4 != 0 {
				got.Machine = KernelText{value: "x86_64"}
			}
			var wantErr error
			if shape != 7 {
				wantErr = core.ErrHostFactsContract
			}
			if err := got.Validate(); !errors.Is(err, wantErr) {
				t.Fatalf("observation=%+v validation=%v, want %v", got, err, wantErr)
			}
		})
	}
}

func TestKernelFieldLayerTriadExhaustsByteAndNativeTerminationDomain(t *testing.T) {
	t.Parallel()
	for value := range 256 {
		t.Run(fmt.Sprintf("byte_%02x", value), func(t *testing.T) {
			t.Parallel()
			field := []byte{byte(value), 0}
			got, err := admitKernelField(field)
			admitted := value > 32 && value < 127
			if admitted {
				if err != nil || got.String() != string(field[:1]) || got.Validate() != nil {
					t.Fatalf("field=%x observation=%q/%v, want exact native text", field, got.String(), err)
				}
			} else if !errors.Is(err, core.ErrHostFactsObservation) || got != (KernelText{}) {
				t.Fatalf("field=%x observation=%q/%v, want zero and typed refusal", field, got.String(), err)
			}
		})
	}
	for position := range 66 {
		t.Run(fmt.Sprintf("native_termination_%d", position), func(t *testing.T) {
			t.Parallel()
			var field [65]byte
			for i := range field {
				field[i] = 'a'
			}
			if position < len(field) {
				field[position] = 0
			}
			got, err := admitKernelField(field[:])
			if position > 0 && position < len(field) {
				if err != nil || len(got.String()) != position || got.Validate() != nil {
					t.Fatalf("termination=%d observation=%q/%v, want exact prefix", position, got.String(), err)
				}
			} else if !errors.Is(err, core.ErrHostFactsObservation) || got != (KernelText{}) {
				t.Fatalf("termination=%d observation=%q/%v, want zero typed refusal", position, got.String(), err)
			}
		})
	}
	for _, row := range []struct {
		name  string
		field []byte
		want  string
	}{
		{name: "unicode native label", field: []byte("noyau-é\x00"), want: "noyau-é"},
		{name: "padding after native terminator is excluded", field: []byte("Linux\x00\xff\n"), want: "Linux"},
		{name: "unicode separator is refused", field: []byte("Linux\u2028next\x00")},
		{name: "interior control is refused", field: []byte("7.2\x01build\x00")},
		{name: "missing terminator is refused", field: []byte("Linux")},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			got, err := admitKernelField(row.field)
			if row.want != "" {
				if err != nil || got.String() != row.want {
					t.Fatalf("field=%x observation=%q/%v, want %q", row.field, got.String(), err, row.want)
				}
			} else if !errors.Is(err, core.ErrHostFactsObservation) || got != (KernelText{}) {
				t.Fatalf("field=%x observation=%q/%v, want zero typed refusal", row.field, got.String(), err)
			}
		})
	}
}

func TestKernelObservationWithholdsFactsOnTerminalContext(t *testing.T) {
	t.Parallel()
	for _, expired := range []bool{false, true} {
		t.Run(fmt.Sprintf("deadline_%v", expired), func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			var wantErr error = context.Canceled
			if expired {
				ctx, cancel = context.WithDeadline(t.Context(), time.Unix(0, 0))
				defer cancel()
				wantErr = context.DeadlineExceeded
			}
			got, err := ObserveKernel(ctx)
			if got != (KernelObservation{}) || !errors.Is(err, wantErr) {
				t.Fatalf("observation=%+v error=%v, want zero and %v", got, err, wantErr)
			}
		})
	}
}
