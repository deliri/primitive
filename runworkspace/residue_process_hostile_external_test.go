package runworkspace_test

import (
	"errors"
	"math"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runworkspace"
)

func TestParseResidueCountHostileEvidenceFloor(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   string
		want    uint32
		wantErr error
	}{
		{name: "valid zero is the neutral residue count", input: "0", want: 0},
		{name: "valid one is the first non-clean residue count", input: "1", want: 1},
		{name: "valid largest one-digit count", input: "9", want: 9},
		{name: "valid first two-digit count", input: "10", want: 10},
		{name: "valid largest two-digit count", input: "99", want: 99},
		{name: "valid first three-digit count", input: "100", want: 100},
		{name: "valid ordinary host count", input: "32768", want: 32768},
		{name: "valid billion-scale count", input: "1000000000", want: 1000000000},
		{name: "valid one below uint32 ceiling", input: "4294967294", want: math.MaxUint32 - 1},
		{name: "valid uint32 ceiling with command newline", input: "4294967295\n", want: math.MaxUint32},

		{name: "reject empty output because the probe reported no fact", input: "", wantErr: core.ErrPrimitiveContract},
		{name: "reject newline without digits", input: "\n", wantErr: core.ErrPrimitiveContract},
		{name: "reject leading zero because two spellings cannot name one fact", input: "00", wantErr: core.ErrPrimitiveContract},
		{name: "reject explicit plus sign", input: "+1", wantErr: core.ErrPrimitiveContract},
		{name: "reject negative count", input: "-1", wantErr: core.ErrPrimitiveContract},
		{name: "reject leading space", input: " 1", wantErr: core.ErrPrimitiveContract},
		{name: "reject trailing space", input: "1 ", wantErr: core.ErrPrimitiveContract},
		{name: "reject second line", input: "1\n2", wantErr: core.ErrPrimitiveContract},
		{name: "reject alphabetic output", input: "one", wantErr: core.ErrPrimitiveContract},
		{name: "reject decimal fraction", input: "1.0", wantErr: core.ErrPrimitiveContract},

		{name: "boundary newline is optional for zero", input: "0\n", want: 0},
		{name: "boundary newline is optional for one", input: "1\n", want: 1},
		{name: "boundary first four-digit count", input: "1000", want: 1000},
		{name: "boundary largest four-digit count", input: "9999", want: 9999},
		{name: "boundary first five-digit count", input: "10000", want: 10000},
		{name: "boundary largest five-digit count", input: "99999", want: 99999},
		{name: "boundary first nine-digit count", input: "100000000", want: 100000000},
		{name: "boundary largest nine-digit count", input: "999999999", want: 999999999},
		{name: "boundary first ten-digit count", input: "1000000000\n", want: 1000000000},
		{name: "boundary exact uint32 ceiling without newline", input: "4294967295", want: math.MaxUint32},
		{name: "boundary one above uint32 ceiling is refused", input: "4294967296", wantErr: core.ErrPrimitiveContract},
		{name: "boundary eleven digits are refused before conversion", input: "10000000000", wantErr: core.ErrPrimitiveContract},
		{name: "boundary twelve bytes including newline are refused", input: "10000000000\n", wantErr: core.ErrPrimitiveContract},
		{name: "boundary carriage-return newline is not canonical", input: "1\r\n", wantErr: core.ErrPrimitiveContract},
		{name: "boundary tab prefix is refused", input: "\t1", wantErr: core.ErrPrimitiveContract},
		{name: "boundary tab suffix is refused", input: "1\t", wantErr: core.ErrPrimitiveContract},
		{name: "boundary embedded NUL is refused", input: "1\x00", wantErr: core.ErrPrimitiveContract},
		{name: "boundary Unicode digit is refused", input: "１", wantErr: core.ErrPrimitiveContract},
		{name: "boundary two trailing newlines are refused", input: "1\n\n", wantErr: core.ErrPrimitiveContract},
		{name: "boundary leading-zero ceiling is refused", input: "04294967295", wantErr: core.ErrPrimitiveContract},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := runworkspace.ParseResidueCount(tc.input)
			if tc.wantErr != nil {
				if got != 0 || !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("ParseResidueCount(%q) = (%d, %v), want (0, errors.Is(..., %v))", tc.input, got, gotErr, tc.wantErr)
				}
				return
			}
			if gotErr != nil || got != tc.want {
				t.Fatalf("ParseResidueCount(%q) = (%d, %v), want (%d, nil)", tc.input, got, gotErr, tc.want)
			}
		})
	}
}

func FuzzParseResidueCountSemanticClosure(f *testing.F) {
	for _, value := range []uint32{0, 1, 9, 10, math.MaxUint32} {
		f.Add(strconv.FormatUint(uint64(value), 10))
	}
	for _, malformed := range []string{"", "-1", "+1", "00", "4294967296", "1\r\n", "one"} {
		f.Add(malformed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		got, gotErr := runworkspace.ParseResidueCount(input)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrPrimitiveContract) {
				t.Fatalf("ParseResidueCount(%q) error = %v, want errors.Is(..., %v)", input, gotErr, core.ErrPrimitiveContract)
			}
			return
		}
		canonical := strconv.FormatUint(uint64(got), 10)
		roundTrip, roundTripErr := runworkspace.ParseResidueCount(canonical)
		if roundTripErr != nil || roundTrip != got {
			t.Fatalf("ParseResidueCount canonical closure for %q = (%d, %v), want (%d, nil)", input, roundTrip, roundTripErr, got)
		}
	})
}
