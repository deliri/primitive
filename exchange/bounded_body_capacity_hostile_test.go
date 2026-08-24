package exchange

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

const boundedBodyCapacityTestLimitBytes = 256 << 10

func TestBoundedBodyReservationTracksDeclaredExtent(t *testing.T) {
	t.Parallel()

	limit, err := core.NewByteCount(boundedBodyCapacityTestLimitBytes)
	if err != nil {
		t.Fatalf("core.NewByteCount() error = %v, want nil", err)
	}
	cases := []struct {
		name string
		body []byte
	}{
		{name: "empty document reserves no payload storage", body: []byte{}},
		{name: "one byte reserves one byte", body: bytes.Repeat([]byte{'a'}, 1)},
		{name: "two bytes reserve two bytes", body: bytes.Repeat([]byte{'a'}, 2)},
		{name: "fifteen bytes reserve fifteen bytes", body: bytes.Repeat([]byte{'a'}, 15)},
		{name: "one hundred twenty seven bytes reserve their extent", body: bytes.Repeat([]byte{'a'}, 127)},
		{name: "one hundred twenty eight bytes reserve their extent", body: bytes.Repeat([]byte{'a'}, 128)},
		{name: "one kilobyte minus one reserves its extent", body: bytes.Repeat([]byte{'a'}, 1023)},
		{name: "one kilobyte reserves its extent", body: bytes.Repeat([]byte{'a'}, 1024)},
		{name: "four kilobytes minus one reserves its extent", body: bytes.Repeat([]byte{'a'}, 4095)},
		{name: "four kilobytes reserves its extent", body: bytes.Repeat([]byte{'a'}, 4096)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			declared, gotDeclareErr := parseDeclaredBodyLength(int64(len(tc.body)))
			if gotDeclareErr != nil {
				t.Fatalf("parseDeclaredBodyLength() error = %v, want nil", gotDeclareErr)
			}
			got, gotErr := readBoundedBody(boundedBodyRead{
				context:  context.Background(),
				source:   bytes.NewReader(tc.body),
				declared: declared,
				limit:    limit,
			})
			if gotErr != nil || !bytes.Equal(got, tc.body) {
				t.Fatalf("readBoundedBody() = (%d bytes, %v), want exact %d-byte body and nil", len(got), gotErr, len(tc.body))
			}
			if cap(got) != len(tc.body) {
				t.Fatalf("readBoundedBody() capacity = %d, want declared extent %d", cap(got), len(tc.body))
			}
		})
	}
}

func TestBoundedBodyGrowthTracksRealBytesAndRefusesTheCeiling(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr            error
		name               string
		bodyBytes          int
		declaredBytes      int64
		limitBytes         uint64
		wantCapacityAtMost int
	}{
		{name: "absent declaration and empty body retain zero capacity", declaredBytes: declaredBodyLengthAbsent, limitBytes: 128, wantCapacityAtMost: 0},
		{name: "absent declaration grows to one received byte", bodyBytes: 1, declaredBytes: declaredBodyLengthAbsent, limitBytes: 128, wantCapacityAtMost: 1},
		{name: "absent declaration grows to one below limit", bodyBytes: 127, declaredBytes: declaredBodyLengthAbsent, limitBytes: 128, wantCapacityAtMost: 127},
		{name: "absent declaration grows exactly to limit", bodyBytes: 128, declaredBytes: declaredBodyLengthAbsent, limitBytes: 128, wantCapacityAtMost: 128},
		{name: "absent declaration refuses one byte above limit", bodyBytes: 129, declaredBytes: declaredBodyLengthAbsent, limitBytes: 128, wantErr: core.ErrExchangeBodyLimit},
		{name: "zero declaration and empty body retain zero capacity", declaredBytes: 0, limitBytes: 128, wantCapacityAtMost: 0},
		{name: "zero declaration grows when one real byte arrives", bodyBytes: 1, declaredBytes: 0, limitBytes: 128, wantCapacityAtMost: 1},
		{name: "one-byte declaration grows for two real bytes", bodyBytes: 2, declaredBytes: 1, limitBytes: 128, wantCapacityAtMost: 2},
		{name: "one-below declaration grows exactly to limit", bodyBytes: 128, declaredBytes: 127, limitBytes: 128, wantCapacityAtMost: 128},
		{name: "overstated small declaration never pads returned body", bodyBytes: 1, declaredBytes: 128, limitBytes: 128, wantCapacityAtMost: 128},
		{name: "exact declaration at limit admits complete body", bodyBytes: 128, declaredBytes: 128, limitBytes: 128, wantCapacityAtMost: 128},
		{name: "declaration at limit refuses one real excess byte", bodyBytes: 129, declaredBytes: 128, limitBytes: 128, wantErr: core.ErrExchangeBodyLimit},
		{name: "declaration above limit is refused before allocation", bodyBytes: 1, declaredBytes: 129, limitBytes: 128, wantErr: core.ErrExchangeBodyLimit},
		{name: "large overstated declaration reserves only initial ceiling", bodyBytes: 1, declaredBytes: boundedBodyCapacityTestLimitBytes, limitBytes: boundedBodyCapacityTestLimitBytes, wantCapacityAtMost: boundedBodyInitialReservationMaximumBytes},
		{name: "initial ceiling minus one declaration stays below ceiling", bodyBytes: 1, declaredBytes: boundedBodyInitialReservationMaximumBytes - 1, limitBytes: boundedBodyCapacityTestLimitBytes, wantCapacityAtMost: boundedBodyInitialReservationMaximumBytes - 1},
		{name: "initial ceiling declaration reserves exact ceiling", bodyBytes: 1, declaredBytes: boundedBodyInitialReservationMaximumBytes, limitBytes: boundedBodyCapacityTestLimitBytes, wantCapacityAtMost: boundedBodyInitialReservationMaximumBytes},
		{name: "initial ceiling plus one declaration remains capped", bodyBytes: 1, declaredBytes: boundedBodyInitialReservationMaximumBytes + 1, limitBytes: boundedBodyCapacityTestLimitBytes, wantCapacityAtMost: boundedBodyInitialReservationMaximumBytes},
		{name: "real bytes one above initial ceiling trigger bounded growth", bodyBytes: boundedBodyInitialReservationMaximumBytes + 1, declaredBytes: boundedBodyInitialReservationMaximumBytes + 1, limitBytes: 2 * boundedBodyInitialReservationMaximumBytes, wantCapacityAtMost: 2 * boundedBodyInitialReservationMaximumBytes},
		{name: "real bytes reach doubled growth boundary", bodyBytes: 2 * boundedBodyInitialReservationMaximumBytes, declaredBytes: boundedBodyInitialReservationMaximumBytes + 1, limitBytes: 2 * boundedBodyInitialReservationMaximumBytes, wantCapacityAtMost: 2 * boundedBodyInitialReservationMaximumBytes},
		{name: "real bytes one above doubled limit are refused", bodyBytes: 2*boundedBodyInitialReservationMaximumBytes + 1, declaredBytes: boundedBodyInitialReservationMaximumBytes + 1, limitBytes: 2 * boundedBodyInitialReservationMaximumBytes, wantErr: core.ErrExchangeBodyLimit},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			limit, gotLimitErr := core.NewByteCount(tc.limitBytes)
			if gotLimitErr != nil {
				t.Fatalf("core.NewByteCount() error = %v, want nil", gotLimitErr)
			}
			declared, gotAdmissionErr := admittedBodyLength(tc.declaredBytes, limit)
			if gotAdmissionErr != nil {
				if tc.wantErr != nil && errors.Is(gotAdmissionErr, tc.wantErr) {
					return
				}
				t.Fatalf("admittedBodyLength() error = %v, want nil", gotAdmissionErr)
			}
			body := bytes.Repeat([]byte{'b'}, tc.bodyBytes)
			got, gotErr := readBoundedBody(boundedBodyRead{
				context: context.Background(), source: bytes.NewReader(body),
				declared: declared, limit: limit,
			})
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || got != nil {
					t.Fatalf("readBoundedBody() = (%v, %v), want nil and errors.Is(..., %v)", got, gotErr, tc.wantErr)
				}
				return
			}
			if gotErr != nil || !bytes.Equal(got, body) {
				t.Fatalf("readBoundedBody() = (%d bytes, %v), want exact %d-byte body and nil", len(got), gotErr, len(body))
			}
			if cap(got) > tc.wantCapacityAtMost {
				t.Fatalf("readBoundedBody() capacity = %d, want at most %d", cap(got), tc.wantCapacityAtMost)
			}
		})
	}
}
