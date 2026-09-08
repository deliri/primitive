package hostfacts

import (
	"bytes"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestDiskAssessmentAdmissionLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                    string
		available, total, floor uint64
		state                   DiskPressureState
		wantErr                 error
	}{
		{name: "disabled pressure retains exhausted capacity", total: 1, state: DiskPressureDisabled},
		{name: "one below device ceiling admits exact pressure", available: 1, total: 2, floor: 1, state: DiskPressureReached},
		{name: "full availability above floor admits healthy", available: 2, total: 2, floor: 1, state: DiskPressureHealthy},
		{name: "exact device ceiling cannot seal perpetual pressure", available: 2, total: 2, floor: 2, state: DiskPressureReached, wantErr: core.ErrHostFactsContract},
		{name: "one above device ceiling cannot seal perpetual pressure", available: 2, total: 2, floor: 3, state: DiskPressureReached, wantErr: core.ErrHostFactsContract},
		{name: "signed maximum floor cannot hide behind zero availability", total: 1, floor: math.MaxInt64, state: DiskPressureReached, wantErr: core.ErrHostFactsContract},
		{name: "unsigned total does not narrow a signed maximum floor", available: math.MaxInt64, total: math.MaxUint64, floor: math.MaxInt64, state: DiskPressureReached},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			capacity := mustDiskCapacityForHostfactsTest(t, tc.available, tc.total)
			policy := DiskPressurePolicy{FreeSpaceFloor: mustByteLength(t, tc.floor)}
			candidate := DiskAssessment{capacity: capacity, policy: policy, state: tc.state}
			if err := candidate.Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("assessment Validate = %v, want %v", err, tc.wantErr)
			}
			got, err := assessDiskCapacity(capacity, policy)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || errors.Is(err, core.ErrHostFactsPressure) || got != (DiskAssessment{}) {
					t.Fatalf("assessment = %+v/%v, want zero contract refusal without pressure", got, err)
				}
				return
			}
			var wantErr error
			if tc.state == DiskPressureReached {
				wantErr = core.ErrDiskFloorReached
			}
			if !errors.Is(err, wantErr) || got != candidate {
				t.Fatalf("assessment = %+v/%v, want %+v/%v", got, err, candidate, wantErr)
			}
		})
	}
}

func TestHostnameValidationClosesAdmission(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, value string
		wantErr     error
	}{
		{name: "unset observation refuses", wantErr: core.ErrHostFactsContract},
		{name: "smallest printable host retains exact byte", value: "h"},
		{name: "exact byte ceiling is admitted", value: strings.Repeat("h", hostnameMaximumBytes)},
		{name: "one above byte ceiling cannot validate", value: strings.Repeat("h", hostnameMaximumBytes+1), wantErr: core.ErrHostFactsContract},
		{name: "embedded C0 cannot validate", value: "h\x00ost", wantErr: core.ErrHostFactsContract},
		{name: "DEL cannot validate", value: "host\x7f", wantErr: core.ErrHostFactsContract},
		{name: "truncated UTF8 cannot validate", value: "host\xc3", wantErr: core.ErrHostFactsContract},
		{name: "multibyte boundary counts bytes", value: strings.Repeat("é", hostnameMaximumBytes/2) + "h"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := (Hostname{value: tc.value}).Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("hostname Validate = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestBoundedValueMaximumRejectsBeforeReading(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		maximum uint64
		wantErr error
	}{
		{name: "zero ceiling accepts empty value", maximum: 0},
		{name: "package ceiling accepts empty value", maximum: virtualFileMaximumBytes},
		{name: "first above package ceiling refuses", maximum: virtualFileMaximumBytes + 1, wantErr: core.ErrHostFactsObservation},
		{name: "signed maximum cannot overflow spare byte", maximum: math.MaxInt64, wantErr: core.ErrHostFactsObservation},
		{name: "unsigned sign bit cannot wrap allocation size", maximum: math.MaxInt64 + 1, wantErr: core.ErrHostFactsObservation},
		{name: "unsigned maximum cannot wrap to negative int", maximum: math.MaxUint64, wantErr: core.ErrHostFactsObservation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source := &countedHostfactsReader{Reader: bytes.NewReader(nil)}
			// A panic is reported as the broken admission fact so every row still runs.
			defer func() {
				if value := recover(); value != nil {
					t.Errorf("bounded read panic = %v, want typed size refusal", value)
				}
			}()
			got, err := readBoundedValue(t.Context(), source, tc.maximum)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("bounded read = %q/%v, want %v", got, err, tc.wantErr)
			}
			if tc.wantErr != nil && (got != nil || source.calls != 0) {
				t.Fatalf("refused read bytes=%d calls=%d, want nil and zero", len(got), source.calls)
			}
			if tc.wantErr == nil && (len(got) != 0 || source.calls != 1) {
				t.Fatalf("empty read bytes=%d calls=%d, want empty and one", len(got), source.calls)
			}
		})
	}
}

type countedHostfactsReader struct {
	*bytes.Reader
	calls int
}

func (r *countedHostfactsReader) Read(p []byte) (int, error) { r.calls++; return r.Reader.Read(p) }
