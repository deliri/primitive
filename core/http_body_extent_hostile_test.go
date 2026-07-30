package core

import (
	"errors"
	"math"
	"testing"
)

// TestDeclaredBodyLengthExhaustsTheAbsenceBoundary walks the integers around the
// one value that means absence, so no neighbouring value can quietly become a
// second spelling of "unknown" and no expressible length can be refused.
func TestDeclaredBodyLengthExhaustsTheAbsenceBoundary(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		value       int64
		wantLength  uint64
		wantPresent bool
		wantErr     bool
	}{
		{name: "two below absence is not a length", value: -2, wantErr: true},
		{name: "minimum int64 is not a length", value: math.MinInt64, wantErr: true},
		{name: "absence declares no extent", value: -1},
		{
			name:        "zero declares an empty body",
			value:       0,
			wantPresent: true,
		},
		{
			name:        "one declares one byte",
			value:       1,
			wantPresent: true,
			wantLength:  1,
		},
		{
			name:        "maximum int64 declares its extent",
			value:       math.MaxInt64,
			wantPresent: true,
			wantLength:  math.MaxInt64,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := ParseDeclaredBodyLength(testCase.value)
			if testCase.wantErr {
				if gotErr == nil {
					t.Fatalf(
						"ParseDeclaredBodyLength(%d) error = nil, want an error",
						testCase.value,
					)
				}
				if !errors.Is(gotErr, ErrPrimitiveContract) {
					t.Fatalf(
						"errors.Is(ParseDeclaredBodyLength(%d) error, ErrPrimitiveContract) = false, want true",
						testCase.value,
					)
				}
				if got != (DeclaredBodyLength{}) {
					t.Fatalf(
						"ParseDeclaredBodyLength(%d) = %+v, want the zero value on refusal",
						testCase.value,
						got,
					)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf(
					"ParseDeclaredBodyLength(%d) error = %v, want nil",
					testCase.value,
					gotErr,
				)
			}
			if gotPresent := got.Present(); gotPresent != testCase.wantPresent {
				t.Fatalf(
					"ParseDeclaredBodyLength(%d).Present() = %t, want %t",
					testCase.value,
					gotPresent,
					testCase.wantPresent,
				)
			}
			if gotLength := got.Length().Uint64(); gotLength != testCase.wantLength {
				t.Fatalf(
					"ParseDeclaredBodyLength(%d).Length() = %d, want %d",
					testCase.value,
					gotLength,
					testCase.wantLength,
				)
			}
			if gotValidateErr := got.Validate(); gotValidateErr != nil {
				t.Fatalf(
					"ParseDeclaredBodyLength(%d).Validate() error = %v, want nil",
					testCase.value,
					gotValidateErr,
				)
			}
		})
	}
}

// TestDeclaredZeroIsNotAbsence pins the distinction the type exists for. A body
// declared as empty is a declaration; a body with no declaration is not. Collapsing
// the two would let an undeclared body be treated as known to be empty.
func TestDeclaredZeroIsNotAbsence(t *testing.T) {
	t.Parallel()

	declaredEmpty, err := ParseDeclaredBodyLength(0)
	if err != nil {
		t.Fatalf("ParseDeclaredBodyLength(0) error = %v, want nil", err)
	}
	absent, err := ParseDeclaredBodyLength(-1)
	if err != nil {
		t.Fatalf("ParseDeclaredBodyLength(-1) error = %v, want nil", err)
	}
	if !declaredEmpty.Present() {
		t.Fatal("ParseDeclaredBodyLength(0).Present() = false, want true")
	}
	if absent.Present() {
		t.Fatal("ParseDeclaredBodyLength(-1).Present() = true, want false")
	}
	if declaredEmpty == absent {
		t.Fatalf(
			"declared-empty %+v equals absent %+v, want distinct states",
			declaredEmpty,
			absent,
		)
	}
	if gotDeclared, gotAbsent := declaredEmpty.Length().Uint64(), absent.Length().Uint64(); gotDeclared != gotAbsent {
		t.Fatalf(
			"declared-empty and absent extents = (%d, %d), want (0, 0) with only Present distinguishing them",
			gotDeclared,
			gotAbsent,
		)
	}
}

// TestDeclaredBodyLengthLimitDecisionsSaturate proves the two decisions a bound
// drives: refusing a declaration that already exceeds the bound, and reserving no
// more memory than the bound authorized. Reservation saturates at the bound
// instead of trusting the declaration, so an inflated declaration cannot enlarge
// the memory an operation was permitted.
func TestDeclaredBodyLengthLimitDecisionsSaturate(t *testing.T) {
	t.Parallel()

	const limitBytes = 512 * 1024
	limit, err := NewByteCount(limitBytes)
	if err != nil {
		t.Fatalf("NewByteCount(%d) error = %v, want nil", limitBytes, err)
	}
	cases := []struct {
		name         string
		declared     int64
		wantExceeds  bool
		wantReserved int
	}{
		{name: "absent reserves nothing and exceeds nothing", declared: -1},
		{name: "empty declaration reserves nothing", declared: 0},
		{name: "one byte reserves one byte", declared: 1, wantReserved: 1},
		{
			name:         "exactly the limit reserves the limit",
			declared:     limitBytes,
			wantReserved: limitBytes,
		},
		{
			name:         "one byte over the limit is refused",
			declared:     limitBytes + 1,
			wantExceeds:  true,
			wantReserved: limitBytes,
		},
		{
			name:         "an inflated declaration saturates at the limit",
			declared:     math.MaxInt64,
			wantExceeds:  true,
			wantReserved: limitBytes,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			declared, parseErr := ParseDeclaredBodyLength(testCase.declared)
			if parseErr != nil {
				t.Fatalf(
					"ParseDeclaredBodyLength(%d) error = %v, want nil",
					testCase.declared,
					parseErr,
				)
			}
			gotExceeds, gotErr := declared.ExceedsLimit(limit)
			if gotErr != nil {
				t.Fatalf("ExceedsLimit() error = %v, want nil", gotErr)
			}
			if gotExceeds != testCase.wantExceeds {
				t.Fatalf(
					"ParseDeclaredBodyLength(%d).ExceedsLimit(%d) = %t, want %t",
					testCase.declared,
					limitBytes,
					gotExceeds,
					testCase.wantExceeds,
				)
			}
			gotReserved, gotErr := declared.ReservedExtent(limit)
			if gotErr != nil {
				t.Fatalf("ReservedExtent() error = %v, want nil", gotErr)
			}
			if gotReserved != testCase.wantReserved {
				t.Fatalf(
					"ParseDeclaredBodyLength(%d).ReservedExtent(%d) = %d, want %d",
					testCase.declared,
					limitBytes,
					gotReserved,
					testCase.wantReserved,
				)
			}
			if gotReserved > limitBytes {
				t.Fatalf(
					"ReservedExtent() = %d, want no more than the authorized %d",
					gotReserved,
					limitBytes,
				)
			}
		})
	}
}

// TestDeclaredBodyLengthRefusesAnUnauthorizedLimit proves both bound-driven
// decisions refuse an invalid bound rather than defaulting to a permissive answer.
// A zero ByteCount is the unset bound, and reading it as "no limit" would remove
// the ceiling entirely.
func TestDeclaredBodyLengthRefusesAnUnauthorizedLimit(t *testing.T) {
	t.Parallel()

	declared, err := ParseDeclaredBodyLength(1)
	if err != nil {
		t.Fatalf("ParseDeclaredBodyLength(1) error = %v, want nil", err)
	}
	if _, gotErr := declared.ExceedsLimit(ByteCount{}); gotErr == nil {
		t.Fatal("ExceedsLimit(ByteCount{}) error = nil, want a refusal")
	}
	if _, gotErr := declared.ReservedExtent(ByteCount{}); gotErr == nil {
		t.Fatal("ReservedExtent(ByteCount{}) error = nil, want a refusal")
	}
}

// TestForgedDeclaredBodyLengthIsRefused pins the one state no constructor can
// produce: absence carrying an extent. A value assembled outside
// ParseDeclaredBodyLength must fail Validate and must not answer either
// bound-driven decision, so no caller can act on an extent that was never
// declared.
func TestForgedDeclaredBodyLengthIsRefused(t *testing.T) {
	t.Parallel()

	limit, err := NewByteCount(1024)
	if err != nil {
		t.Fatalf("NewByteCount(1024) error = %v, want nil", err)
	}
	forged := DeclaredBodyLength{length: NewByteLength(4096), present: false}
	if gotErr := forged.Validate(); gotErr == nil {
		t.Fatal("forged DeclaredBodyLength Validate() error = nil, want a refusal")
	}
	if _, gotErr := forged.ExceedsLimit(limit); gotErr == nil {
		t.Fatal("forged DeclaredBodyLength ExceedsLimit() error = nil, want a refusal")
	}
	if _, gotErr := forged.ReservedExtent(limit); gotErr == nil {
		t.Fatal("forged DeclaredBodyLength ReservedExtent() error = nil, want a refusal")
	}
}

// FuzzParseDeclaredBodyLength proves the oracle every caller relies on: an
// accepted value round trips to the extent it declared, absence never carries an
// extent, and a reservation never exceeds the authorized bound for any input.
func FuzzParseDeclaredBodyLength(f *testing.F) {
	for _, seed := range []int64{math.MinInt64, -2, -1, 0, 1, 4096, math.MaxInt64} {
		f.Add(seed)
	}
	limit, err := NewByteCount(512 * 1024)
	if err != nil {
		f.Fatalf("NewByteCount() error = %v, want nil", err)
	}
	allowed, err := limit.Uint64()
	if err != nil {
		f.Fatalf("ByteCount.Uint64() error = %v, want nil", err)
	}

	f.Fuzz(func(t *testing.T, value int64) {
		declared, parseErr := ParseDeclaredBodyLength(value)
		if parseErr != nil {
			if declared != (DeclaredBodyLength{}) {
				t.Fatalf(
					"ParseDeclaredBodyLength(%d) refused but returned %+v, want the zero value",
					value,
					declared,
				)
			}
			return
		}
		if gotErr := declared.Validate(); gotErr != nil {
			t.Fatalf(
				"ParseDeclaredBodyLength(%d).Validate() error = %v, want nil",
				value,
				gotErr,
			)
		}
		if declared.Present() != (value >= 0) {
			t.Fatalf(
				"ParseDeclaredBodyLength(%d).Present() = %t, want %t",
				value,
				declared.Present(),
				value >= 0,
			)
		}
		if declared.Present() && declared.Length().Uint64() != uint64(value) {
			t.Fatalf(
				"ParseDeclaredBodyLength(%d).Length() = %d, want %d",
				value,
				declared.Length().Uint64(),
				value,
			)
		}
		if !declared.Present() && declared.Length().Uint64() != 0 {
			t.Fatalf(
				"absent ParseDeclaredBodyLength(%d).Length() = %d, want 0",
				value,
				declared.Length().Uint64(),
			)
		}
		reserved, gotErr := declared.ReservedExtent(limit)
		if gotErr != nil {
			t.Fatalf("ReservedExtent() error = %v, want nil", gotErr)
		}
		if reserved < 0 || uint64(reserved) > allowed {
			t.Fatalf(
				"ParseDeclaredBodyLength(%d).ReservedExtent() = %d, want within [0, %d]",
				value,
				reserved,
				allowed,
			)
		}
	})
}
