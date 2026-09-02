package core

import (
	"errors"
	"math"
	"strconv"
	"testing"
)

// TestByteExtentJSONHostileTable is a contract ratchet over the real JSON
// boundary: accepted values must round-trip canonically and rejected values
// must preserve both receivers.
func TestByteExtentJSONHostileTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		wire         string
		wantValue    uint64
		wantLengthOK bool
		wantCountOK  bool
	}{
		{name: "zero is a valid empty length but not a positive count", wire: "0", wantLengthOK: true},
		{name: "minimum positive count", wire: "1", wantValue: 1, wantLengthOK: true, wantCountOK: true},
		{name: "single decimal digit upper edge", wire: "9", wantValue: 9, wantLengthOK: true, wantCountOK: true},
		{name: "two decimal digit lower edge", wire: "10", wantValue: 10, wantLengthOK: true, wantCountOK: true},
		{name: "uint8 upper edge", wire: "255", wantValue: 255, wantLengthOK: true, wantCountOK: true},
		{name: "uint8 one above", wire: "256", wantValue: 256, wantLengthOK: true, wantCountOK: true},
		{name: "uint16 upper edge", wire: "65535", wantValue: 65535, wantLengthOK: true, wantCountOK: true},
		{name: "uint16 one above", wire: "65536", wantValue: 65536, wantLengthOK: true, wantCountOK: true},
		{name: "uint32 one below upper edge", wire: "4294967294", wantValue: math.MaxUint32 - 1, wantLengthOK: true, wantCountOK: true},
		{name: "uint32 upper edge", wire: "4294967295", wantValue: math.MaxUint32, wantLengthOK: true, wantCountOK: true},
		{name: "uint32 one above", wire: "4294967296", wantValue: math.MaxUint32 + 1, wantLengthOK: true, wantCountOK: true},
		{name: "int64 one below upper edge", wire: "9223372036854775806", wantValue: math.MaxInt64 - 1, wantLengthOK: true, wantCountOK: true},
		{name: "int64 upper edge", wire: "9223372036854775807", wantValue: math.MaxInt64, wantLengthOK: true, wantCountOK: true},
		{name: "int64 one above", wire: "9223372036854775808", wantValue: uint64(math.MaxInt64) + 1, wantCountOK: true},
		{name: "uint64 one below upper edge", wire: "18446744073709551614", wantValue: math.MaxUint64 - 1, wantCountOK: true},
		{name: "uint64 upper edge", wire: "18446744073709551615", wantValue: math.MaxUint64, wantCountOK: true},
		{name: "empty document is rejected", wire: ""},
		{name: "ASCII space prefix is rejected", wire: " 1"},
		{name: "ASCII space suffix is rejected", wire: "1 "},
		{name: "newline prefix is rejected", wire: "\n1"},
		{name: "newline suffix is rejected", wire: "1\n"},
		{name: "tab prefix is rejected", wire: "\t1"},
		{name: "negative one is rejected", wire: "-1"},
		{name: "negative zero is rejected", wire: "-0"},
		{name: "explicit positive sign is rejected", wire: "+1"},
		{name: "single leading zero is rejected", wire: "01"},
		{name: "multiple leading zeros are rejected", wire: "0001"},
		{name: "decimal form is rejected", wire: "1.0"},
		{name: "fractional form is rejected", wire: "0.1"},
		{name: "positive exponent is rejected", wire: "1e1"},
		{name: "zero exponent is rejected", wire: "1e0"},
		{name: "uppercase exponent is rejected", wire: "1E0"},
		{name: "quoted number is rejected", wire: `"1"`},
		{name: "null is rejected", wire: "null"},
		{name: "boolean is rejected", wire: "true"},
		{name: "array is rejected", wire: "[1]"},
		{name: "object is rejected", wire: `{"value":1}`},
		{name: "uint64 one above upper edge is rejected", wire: "18446744073709551616"},
		{name: "far oversized integer is rejected", wire: "9999999999999999999999999999999999999999"},
		{name: "NUL prefix is rejected", wire: "\x001"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			beforeLength := mustByteLength(t, 7)
			gotLength := beforeLength
			gotLengthErr := gotLength.UnmarshalJSON([]byte(tc.wire))
			if (gotLengthErr == nil) != tc.wantLengthOK {
				t.Fatalf("json.Unmarshal(ByteLength, %q) error = %v, want success %t", tc.wire, gotLengthErr, tc.wantLengthOK)
			}
			if tc.wantLengthOK {
				if gotValue := gotLength.Uint64(); gotValue != tc.wantValue {
					t.Fatalf("ByteLength.Uint64() = %d, want %d", gotValue, tc.wantValue)
				}
			} else {
				if !errors.Is(gotLengthErr, ErrJSONContract) {
					t.Fatalf("json.Unmarshal(ByteLength, %q) error = %v, want %v", tc.wire, gotLengthErr, ErrJSONContract)
				}
				if gotLength != beforeLength {
					t.Fatalf("rejected ByteLength JSON mutated receiver: got %v, want %v", gotLength, beforeLength)
				}
			}

			beforeCount, setupErr := NewByteCount(7)
			if setupErr != nil {
				t.Fatalf("NewByteCount(7) error = %v, want nil", setupErr)
			}
			gotCount := beforeCount
			gotCountErr := gotCount.UnmarshalJSON([]byte(tc.wire))
			if (gotCountErr == nil) != tc.wantCountOK {
				t.Fatalf("json.Unmarshal(ByteCount, %q) error = %v, want success %t", tc.wire, gotCountErr, tc.wantCountOK)
			}
			if tc.wantCountOK {
				gotValue, gotValueErr := gotCount.Uint64()
				if gotValueErr != nil || gotValue != tc.wantValue {
					t.Fatalf("ByteCount.Uint64() = (%d, %v), want (%d, nil)", gotValue, gotValueErr, tc.wantValue)
				}
			} else {
				if !errors.Is(gotCountErr, ErrJSONContract) {
					t.Fatalf("json.Unmarshal(ByteCount, %q) error = %v, want %v", tc.wire, gotCountErr, ErrJSONContract)
				}
				if gotCount != beforeCount {
					t.Fatalf("rejected ByteCount JSON mutated receiver: got %v, want %v", gotCount, beforeCount)
				}
			}
		})
	}
}

// TestByteExtentJSONReencodesToTheAcceptedBytes closes the emit half of the
// boundary. The table above proves only which documents are accepted; without a
// re-encode ratchet an emitter could pad, quote, or otherwise leave the
// canonical form and every decode test would stay green. Decode followed by
// encode must reproduce the exact accepted bytes, because those bytes are the
// wire contract.
func TestByteExtentJSONReencodesToTheAcceptedBytes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		wire         string
		countRejects bool
	}{
		{name: "zero is a length but not a count", wire: "0", countRejects: true},
		{name: "minimum positive count", wire: "1"},
		{name: "single decimal digit upper edge", wire: "9"},
		{name: "two decimal digit lower edge", wire: "10"},
		{name: "uint8 upper edge", wire: "255"},
		{name: "uint8 one above", wire: "256"},
		{name: "uint16 upper edge", wire: "65535"},
		{name: "uint32 upper edge", wire: "4294967295"},
		{name: "uint32 one above", wire: "4294967296"},
		{name: "int64 one below upper edge", wire: "9223372036854775806"},
		{name: "int64 upper edge", wire: "9223372036854775807"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var gotLength ByteLength
			if gotErr := gotLength.UnmarshalJSON([]byte(tc.wire)); gotErr != nil {
				t.Fatalf("ByteLength.UnmarshalJSON(%q) error = %v, want nil", tc.wire, gotErr)
			}
			gotLengthWire, gotLengthErr := gotLength.MarshalJSON()
			if gotLengthErr != nil || string(gotLengthWire) != tc.wire {
				t.Fatalf(
					"ByteLength.MarshalJSON() = (%s, %v), want (%s, nil)",
					gotLengthWire,
					gotLengthErr,
					tc.wire,
				)
			}

			if tc.countRejects {
				return
			}
			var gotCount ByteCount
			if gotErr := gotCount.UnmarshalJSON([]byte(tc.wire)); gotErr != nil {
				t.Fatalf("ByteCount.UnmarshalJSON(%q) error = %v, want nil", tc.wire, gotErr)
			}
			gotCountWire, gotCountErr := gotCount.MarshalJSON()
			if gotCountErr != nil || string(gotCountWire) != tc.wire {
				t.Fatalf(
					"ByteCount.MarshalJSON() = (%s, %v), want (%s, nil)",
					gotCountWire,
					gotCountErr,
					tc.wire,
				)
			}
		})
	}
}

// TestZeroByteCountRefusesToEmitAnInvalidCount proves the emit gate. A zero
// ByteCount is the invalid Go zero value, and emitting it would put a quantity
// on the wire that the decoder is required to reject, leaving the two halves of
// one boundary disagreeing about the same document.
func TestZeroByteCountRefusesToEmitAnInvalidCount(t *testing.T) {
	t.Parallel()

	got, gotErr := ByteCount{}.MarshalJSON()
	if !errors.Is(gotErr, ErrJSONContract) || !errors.Is(gotErr, ErrPrimitiveContract) {
		t.Fatalf(
			"ByteCount{}.MarshalJSON() error = %v, want %v and %v",
			gotErr,
			ErrJSONContract,
			ErrPrimitiveContract,
		)
	}
	if got != nil {
		t.Fatalf("ByteCount{}.MarshalJSON() = %s, want no document", got)
	}

	// A zero length is a meaningful quantity, unlike a zero count, so it must
	// still emit. Collapsing both types onto one rule is the failure this
	// pairing pins.
	zeroLength := mustByteLength(t, 0)
	gotLength, gotLengthErr := zeroLength.MarshalJSON()
	if gotLengthErr != nil || string(gotLength) != "0" {
		t.Fatalf("NewByteLength(0).MarshalJSON() = (%s, %v), want (0, nil)", gotLength, gotLengthErr)
	}
}

// TestCheckedNumericConversionsPinBothSidesOfEveryBoundary exercises the
// exported checked conversions directly. They are the single owner of the
// signed and 32-bit narrowing rules that ByteCount, ByteLength, and
// SecretMaterial delegate to, so an off-by-one in either would silently widen a
// bound everywhere those types are used.
func TestCheckedNumericConversionsPinBothSidesOfEveryBoundary(t *testing.T) {
	t.Parallel()

	int64Cases := []struct {
		wantErr error
		name    string
		value   uint64
		want    int64
	}{
		{name: "zero converts", value: 0},
		{name: "one converts", value: 1, want: 1},
		{name: "int32 upper edge converts", value: math.MaxInt32, want: math.MaxInt32},
		{name: "uint32 upper edge converts", value: math.MaxUint32, want: math.MaxUint32},
		{name: "one below int64 upper edge converts", value: math.MaxInt64 - 1, want: math.MaxInt64 - 1},
		{name: "int64 upper edge converts", value: math.MaxInt64, want: math.MaxInt64},
		{name: "one above int64 upper edge overflows", value: uint64(math.MaxInt64) + 1, wantErr: ErrNumericOverflow},
		{name: "two above int64 upper edge overflows", value: uint64(math.MaxInt64) + 2, wantErr: ErrNumericOverflow},
		{name: "uint64 upper edge overflows", value: math.MaxUint64, wantErr: ErrNumericOverflow},
	}
	for _, tc := range int64Cases {
		t.Run("CheckedInt64FromUint64 "+tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := CheckedInt64FromUint64(tc.value)
			length, lengthErr := NewByteLength(tc.value)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || got != 0 {
					t.Fatalf(
						"CheckedInt64FromUint64(%d) = (%d, %v), want (0, %v)",
						tc.value,
						got,
						gotErr,
						ErrNumericOverflow,
					)
				}
				// ByteLength delegates its signed projection to the same rule,
				// so the two must agree at every boundary rather than drift.
				if !errors.Is(lengthErr, ErrNumericOverflow) || length != (ByteLength{}) {
					t.Fatalf(
						"NewByteLength(%d) = (%v, %v), want (zero, %v)",
						tc.value,
						length,
						lengthErr,
						ErrNumericOverflow,
					)
				}
				forgedLength, forgedLengthErr := (ByteLength{value: tc.value}).Int64()
				if !errors.Is(forgedLengthErr, ErrNumericOverflow) || forgedLength != 0 {
					t.Fatalf("forged ByteLength.Int64() = (%d, %v), want (0, %v)", forgedLength, forgedLengthErr, ErrNumericOverflow)
				}
				return
			}
			if gotErr != nil || got != tc.want {
				t.Fatalf("CheckedInt64FromUint64(%d) = (%d, %v), want (%d, nil)", tc.value, got, gotErr, tc.want)
			}
			if lengthErr != nil {
				t.Fatalf("NewByteLength(%d) error = %v, want nil", tc.value, lengthErr)
			}
			gotLength, gotLengthErr := length.Int64()
			if gotLengthErr != nil || gotLength != tc.want {
				t.Fatalf(
					"NewByteLength(%d).Int64() = (%d, %v), want (%d, nil)",
					tc.value,
					gotLength,
					gotLengthErr,
					tc.want,
				)
			}
		})
	}

	uint64Cases := []struct {
		wantErr error
		name    string
		value   int64
		want    uint64
	}{
		{name: "minimum int64 overflows", value: math.MinInt64, wantErr: ErrNumericOverflow},
		{name: "negative two overflows", value: -2, wantErr: ErrNumericOverflow},
		{name: "negative one overflows", value: -1, wantErr: ErrNumericOverflow},
		{name: "zero converts", value: 0},
		{name: "one converts", value: 1, want: 1},
		{name: "int32 upper edge converts", value: math.MaxInt32, want: math.MaxInt32},
		{name: "uint32 upper edge converts", value: math.MaxUint32, want: math.MaxUint32},
		{name: "one below int64 upper edge converts", value: math.MaxInt64 - 1, want: math.MaxInt64 - 1},
		{name: "int64 upper edge converts", value: math.MaxInt64, want: math.MaxInt64},
	}
	for _, tc := range uint64Cases {
		t.Run("CheckedUint64FromInt64 "+tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := CheckedUint64FromInt64(tc.value)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || got != 0 {
					t.Fatalf(
						"CheckedUint64FromInt64(%d) = (%d, %v), want (0, %v)",
						tc.value,
						got,
						gotErr,
						ErrNumericOverflow,
					)
				}
				return
			}
			if gotErr != nil || got != tc.want {
				t.Fatalf(
					"CheckedUint64FromInt64(%d) = (%d, %v), want (%d, nil)",
					tc.value,
					got,
					gotErr,
					tc.want,
				)
			}
		})
	}

	type checkedUint32Case struct {
		wantErr error
		name    string
		value   int
		want    uint32
	}
	uint32Cases := []checkedUint32Case{
		{name: "negative one overflows", value: -1, wantErr: ErrNumericOverflow},
		{name: "minimum int overflows", value: math.MinInt, wantErr: ErrNumericOverflow},
		{name: "zero converts", value: 0},
		{name: "one converts", value: 1, want: 1},
	}
	if strconv.IntSize == 32 {
		intUpper := math.MaxInt
		uint32Cases = append(
			uint32Cases,
			checkedUint32Case{name: "one below 32-bit int upper edge converts", value: intUpper - 1, want: uint32(intUpper - 1)},
			checkedUint32Case{name: "32-bit int upper edge converts", value: intUpper, want: uint32(intUpper)},
		)
	} else {
		uint32Upper := uint64(math.MaxUint32)
		uint32Cases = append(
			uint32Cases,
			checkedUint32Case{name: "one below uint32 upper edge converts", value: int(uint32Upper - 1), want: math.MaxUint32 - 1},
			checkedUint32Case{name: "uint32 upper edge converts", value: int(uint32Upper), want: math.MaxUint32},
			checkedUint32Case{name: "one above uint32 upper edge overflows", value: int(uint32Upper + 1), wantErr: ErrNumericOverflow},
			checkedUint32Case{name: "maximum int overflows", value: math.MaxInt, wantErr: ErrNumericOverflow},
		)
	}
	for _, tc := range uint32Cases {
		t.Run("CheckedUint32FromInt "+tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := CheckedUint32FromInt(tc.value)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || got != 0 {
					t.Fatalf(
						"CheckedUint32FromInt(%d) = (%d, %v), want (0, %v)",
						tc.value,
						got,
						gotErr,
						ErrNumericOverflow,
					)
				}
				return
			}
			if gotErr != nil || got != tc.want {
				t.Fatalf("CheckedUint32FromInt(%d) = (%d, %v), want (%d, nil)", tc.value, got, gotErr, tc.want)
			}
		})
	}

	uint16FromIntCases := []struct {
		wantErr error
		name    string
		value   int
		want    uint16
	}{
		{name: "minimum int overflows", value: math.MinInt, wantErr: ErrNumericOverflow},
		{name: "negative one overflows", value: -1, wantErr: ErrNumericOverflow},
		{name: "zero converts", value: 0},
		{name: "one converts", value: 1, want: 1},
		{name: "one below uint16 upper edge converts", value: math.MaxUint16 - 1, want: math.MaxUint16 - 1},
		{name: "uint16 upper edge converts", value: math.MaxUint16, want: math.MaxUint16},
		{name: "one above uint16 upper edge overflows", value: math.MaxUint16 + 1, wantErr: ErrNumericOverflow},
		{name: "maximum int overflows", value: math.MaxInt, wantErr: ErrNumericOverflow},
	}
	for _, tc := range uint16FromIntCases {
		t.Run("CheckedUint16FromInt "+tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := CheckedUint16FromInt(tc.value)
			if !errors.Is(gotErr, tc.wantErr) || got != tc.want {
				t.Fatalf("CheckedUint16FromInt(%d) = (%d, %v), want (%d, %v)", tc.value, got, gotErr, tc.want, tc.wantErr)
			}
		})
	}

	uint8Cases := []struct {
		wantErr error
		name    string
		value   int
		want    uint8
	}{
		{name: "minimum int overflows", value: math.MinInt, wantErr: ErrNumericOverflow},
		{name: "negative one overflows", value: -1, wantErr: ErrNumericOverflow},
		{name: "zero converts", value: 0},
		{name: "one converts", value: 1, want: 1},
		{name: "one below uint8 upper edge converts", value: math.MaxUint8 - 1, want: math.MaxUint8 - 1},
		{name: "uint8 upper edge converts", value: math.MaxUint8, want: math.MaxUint8},
		{name: "one above uint8 upper edge overflows", value: math.MaxUint8 + 1, wantErr: ErrNumericOverflow},
		{name: "maximum int overflows", value: math.MaxInt, wantErr: ErrNumericOverflow},
	}
	for _, tc := range uint8Cases {
		t.Run("CheckedUint8FromInt "+tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := CheckedUint8FromInt(tc.value)
			if !errors.Is(gotErr, tc.wantErr) || got != tc.want {
				t.Fatalf("CheckedUint8FromInt(%d) = (%d, %v), want (%d, %v)", tc.value, got, gotErr, tc.want, tc.wantErr)
			}
		})
	}

	int32Cases := []struct {
		wantErr error
		name    string
		value   int
		want    int32
	}{
		{name: "int32 minimum converts", value: math.MinInt32, want: math.MinInt32},
		{name: "int32 minimum plus one converts", value: math.MinInt32 + 1, want: math.MinInt32 + 1},
		{name: "negative one converts", value: -1, want: -1},
		{name: "zero converts", value: 0},
		{name: "one converts", value: 1, want: 1},
		{name: "int32 upper edge minus one converts", value: math.MaxInt32 - 1, want: math.MaxInt32 - 1},
		{name: "int32 upper edge converts", value: math.MaxInt32, want: math.MaxInt32},
	}
	if strconv.IntSize == 64 {
		int32Cases = append(int32Cases,
			struct {
				wantErr error
				name    string
				value   int
				want    int32
			}{name: "one below int32 minimum overflows", value: math.MinInt32 - 1, wantErr: ErrNumericOverflow},
			struct {
				wantErr error
				name    string
				value   int
				want    int32
			}{name: "one above int32 upper edge overflows", value: math.MaxInt32 + 1, wantErr: ErrNumericOverflow},
		)
	}
	for _, tc := range int32Cases {
		t.Run("CheckedInt32FromInt "+tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := CheckedInt32FromInt(tc.value)
			if !errors.Is(gotErr, tc.wantErr) || got != tc.want {
				t.Fatalf("CheckedInt32FromInt(%d) = (%d, %v), want (%d, %v)", tc.value, got, gotErr, tc.want, tc.wantErr)
			}
		})
	}
}

func mustByteLength(t *testing.T, value uint64) ByteLength {
	t.Helper()
	length, err := NewByteLength(value)
	if err != nil {
		t.Fatalf("NewByteLength(%d) error = %v, want nil", value, err)
	}
	return length
}

// TestByteCountInt64DelegatesTheSignedBoundAndRejectsTheZeroValue proves the
// count projection routes through the shared conversion rather than repeating
// it, and that the invalid zero value is refused before any conversion runs.
func TestByteCountInt64DelegatesTheSignedBoundAndRejectsTheZeroValue(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		name    string
		value   uint64
		want    int64
	}{
		{name: "minimum positive count converts", value: 1, want: 1},
		{name: "one below int64 upper edge converts", value: math.MaxInt64 - 1, want: math.MaxInt64 - 1},
		{name: "int64 upper edge converts", value: math.MaxInt64, want: math.MaxInt64},
		{name: "one above int64 upper edge overflows", value: uint64(math.MaxInt64) + 1, wantErr: ErrNumericOverflow},
		{name: "uint64 upper edge overflows", value: math.MaxUint64, wantErr: ErrNumericOverflow},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			count, gotNewErr := NewByteCount(tc.value)
			if gotNewErr != nil {
				t.Fatalf("NewByteCount(%d) error = %v, want nil", tc.value, gotNewErr)
			}
			got, gotErr := count.Int64()
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || got != 0 {
					t.Fatalf("NewByteCount(%d).Int64() = (%d, %v), want (0, %v)", tc.value, got, gotErr, tc.wantErr)
				}
				return
			}
			if gotErr != nil || got != tc.want {
				t.Fatalf("NewByteCount(%d).Int64() = (%d, %v), want (%d, nil)", tc.value, got, gotErr, tc.want)
			}
		})
	}

	gotZero, gotZeroErr := ByteCount{}.Int64()
	if !errors.Is(gotZeroErr, ErrPrimitiveContract) || gotZero != 0 {
		t.Fatalf("ByteCount{}.Int64() = (%d, %v), want (0, %v)", gotZero, gotZeroErr, ErrPrimitiveContract)
	}
}
