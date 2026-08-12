package filestore

import (
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func requireByteLength(t *testing.T, value uint64) core.ByteLength {
	t.Helper()
	length, err := core.NewByteLength(value)
	if err != nil {
		t.Fatalf("core.NewByteLength(%d) error = %v, want nil", value, err)
	}
	return length
}

func TestAllocationSeparatesReportedFactsFromFabrication(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		allocation   Allocation
		wantBytes    uint64
		wantErr      error
		wantReported bool
		wantBytesErr error
	}{
		{
			name:         "the zero value is an honest unreported observation",
			allocation:   Allocation{},
			wantReported: false,
			wantBytesErr: core.ErrFilestoreContract,
		},
		{
			name:         "an unreported allocation claiming one byte is a fabrication",
			allocation:   Allocation{bytes: requireByteLength(t, 1)},
			wantErr:      core.ErrFilestoreContract,
			wantBytesErr: core.ErrFilestoreContract,
		},
		{
			name:         "an unreported allocation claiming the ceiling is a fabrication",
			allocation:   Allocation{bytes: requireByteLength(t, math.MaxInt64)},
			wantErr:      core.ErrFilestoreContract,
			wantBytesErr: core.ErrFilestoreContract,
		},
		{
			name:         "a reported hole holds zero bytes",
			allocation:   Allocation{reported: true},
			wantReported: true,
			wantBytes:    0,
		},
		{
			name:         "a reported single block holds its exact bytes",
			allocation:   Allocation{bytes: requireByteLength(t, 512), reported: true},
			wantReported: true,
			wantBytes:    512,
		},
		{
			name:         "a reported ceiling allocation holds its exact bytes",
			allocation:   Allocation{bytes: requireByteLength(t, math.MaxInt64), reported: true},
			wantReported: true,
			wantBytes:    math.MaxInt64,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.allocation.Validate()
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Allocation.Validate() error = %v, want %v", err, tc.wantErr)
			}
			if got := tc.allocation.Reported(); got != tc.wantReported && tc.wantErr == nil {
				t.Fatalf("Allocation.Reported() = %t, want %t", got, tc.wantReported)
			}
			bytes, bytesErr := tc.allocation.Bytes()
			if !errors.Is(bytesErr, tc.wantBytesErr) {
				t.Fatalf("Allocation.Bytes() error = %v, want %v", bytesErr, tc.wantBytesErr)
			}
			if tc.wantBytesErr != nil {
				if bytes != (core.ByteLength{}) {
					t.Fatalf("Allocation.Bytes() = %v, want zero bytes on rejection", bytes)
				}
				return
			}
			if got := bytes.Uint64(); got != tc.wantBytes {
				t.Fatalf("Allocation.Bytes() = %d, want %d", got, tc.wantBytes)
			}
		})
	}
}
