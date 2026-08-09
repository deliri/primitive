package id_test

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/id"
)

const (
	// ulidOneAfterEpochText is millisecond one beside the low entropy bit:
	// the value 2^80+1, whose only set groups are position nine and the tail.
	ulidOneAfterEpochText = "000000000" + "1" + "000000000000000" + "1"
	// ulidEpochLowBitText is the epoch stamp beside the low entropy bit.
	ulidEpochLowBitText = "0000000000000000000000000" + "1"
)

func TestNewULIDPinsExactCanonicalSpellings(t *testing.T) {
	t.Parallel()

	cases := []struct {
		setup    func(t *testing.T) id.Request
		name     string
		wantText string
	}{
		{
			name: "epoch itself is the smallest stamp",
			setup: func(t *testing.T) id.Request {
				t.Helper()
				return testRequest(t, 0, testEntropy())
			},
			wantText: ulidEpochLowBitText,
		},
		{
			name: "one millisecond after the epoch",
			setup: func(t *testing.T) id.Request {
				t.Helper()
				return testRequest(t, 1, testEntropy())
			},
			wantText: ulidOneAfterEpochText,
		},
		{
			name: "saturated entropy keeps all eighty bits",
			setup: func(t *testing.T) id.Request {
				t.Helper()
				saturated := make([]byte, 16)
				for index := range 10 {
					saturated[index] = 0xff
				}
				return testRequest(t, 0, saturated)
			},
			wantText: strings.Repeat("0", 10) + strings.Repeat("Z", 16),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := id.NewULID(tc.setup(t))
			if err != nil {
				t.Fatalf("NewULID() error = %v, want nil", err)
			}
			if got.String() != tc.wantText {
				t.Fatalf("NewULID().String() = %q, want %q", got.String(), tc.wantText)
			}
			parsed, err := id.ParseULID(tc.wantText)
			if err != nil || parsed != got {
				t.Fatalf("ParseULID(%q) = (%v, %v), want the constructed value back", tc.wantText, parsed, err)
			}
		})
	}
}

func TestNewULIDIsPureAndConsumesExactlyTenEntropyBytes(t *testing.T) {
	t.Parallel()

	first, err := id.NewULID(testRequest(t, 1, testEntropy()))
	if err != nil {
		t.Fatalf("NewULID(first) error = %v, want nil", err)
	}
	second, err := id.NewULID(testRequest(t, 1, testEntropy()))
	if err != nil {
		t.Fatalf("NewULID(second) error = %v, want nil", err)
	}
	if first != second {
		t.Fatalf("NewULID minted %v then %v from one request, want pure construction", first, second)
	}
	tailDiffers := testEntropy()
	for index := 10; index < len(tailDiffers); index++ {
		tailDiffers[index] = 0x99
	}
	third, err := id.NewULID(testRequest(t, 1, tailDiffers))
	if err != nil {
		t.Fatalf("NewULID(differing tail) error = %v, want nil", err)
	}
	if third != first {
		t.Fatalf("NewULID(differing tail) = %v, want %v: only the first ten entropy bytes are consumed", third, first)
	}
}

func TestNewULIDRefusesHostileRequests(t *testing.T) {
	t.Parallel()

	cases := []struct {
		setup func(t *testing.T) id.Request
		name  string
	}{
		{
			name: "instant one millisecond before the epoch",
			setup: func(t *testing.T) id.Request {
				t.Helper()
				return testRequest(t, -1, testEntropy())
			},
		},
		{
			name: "epoch stamp beside an all zero entropy head builds the unset value",
			setup: func(t *testing.T) id.Request {
				t.Helper()
				zeroHead := make([]byte, 16)
				zeroHead[15] = 1
				return testRequest(t, 0, zeroHead)
			},
		},
		{
			name: "zero request carries no observation",
			setup: func(t *testing.T) id.Request {
				t.Helper()
				return id.Request{}
			},
		},
		{
			name: "entropy one byte above the exact minimum extent",
			setup: func(t *testing.T) id.Request {
				t.Helper()
				return testRequest(t, 1, append(testEntropy(), 0x01))
			},
		},
		{
			name: "entropy destroyed before construction",
			setup: func(t *testing.T) id.Request {
				t.Helper()
				request := testRequest(t, 1, testEntropy())
				if err := request.Entropy.Destroy(); err != nil {
					t.Fatalf("Entropy.Destroy() error = %v, want nil", err)
				}
				return request
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := id.NewULID(tc.setup(t))
			requireIDContract(t, "NewULID(hostile request)", err)
			if !got.IsZero() {
				t.Fatalf("NewULID(hostile request) value = %v, want the zero value", got)
			}
		})
	}
}

func TestParseULIDAdmitsOnlyCanonicalText(t *testing.T) {
	t.Parallel()

	valid := []struct {
		name string
		text string
	}{
		{name: "smallest nonzero value", text: ulidEpochLowBitText},
		{name: "one millisecond stamp beside the low bit", text: ulidOneAfterEpochText},
		{name: "saturated entropy under the epoch stamp", text: strings.Repeat("0", 10) + strings.Repeat("Z", 16)},
		{name: "largest admissible value", text: "7" + strings.Repeat("Z", 25)},
		{name: "first character exactly at its ceiling", text: "7" + strings.Repeat("0", 24) + "1"},
		{name: "specification shaped spelling", text: "01ARZ3NDEKTSV4RRFFQ69G5FAV"},
		{name: "every digit of the alphabet's first half", text: "0123456789ABCDEFGH00000001"},
		{name: "every digit of the alphabet's second half", text: "0JKMNPQRSTVWXYZ00000000001"},
		{name: "single high bit in the tail group", text: strings.Repeat("0", 25) + "G"},
		{name: "alternating groups across both halves", text: "0Z0Z0Z0Z0Z0Z0Z0Z0Z0Z0Z0Z0Z"},
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := id.ParseULID(tc.text)
			if err != nil {
				t.Fatalf("ParseULID(%q) error = %v, want nil", tc.text, err)
			}
			if got.String() != tc.text {
				t.Fatalf("ParseULID(%q).String() = %q, want the admitted text back", tc.text, got.String())
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("ParseULID(%q).Validate() error = %v, want nil", tc.text, err)
			}
		})
	}

	invalid := []struct {
		name string
		text string
	}{
		{name: "empty text", text: ""},
		{name: "one byte below the canonical extent", text: strings.Repeat("0", 24) + "1"},
		{name: "one byte above the canonical extent", text: strings.Repeat("0", 26) + "1"},
		{name: "lowercase spelling of a valid value", text: "01arz3ndektsv4rrffq69g5fav"},
		{name: "single lowercase byte in the tail", text: strings.Repeat("0", 25) + "z"},
		{name: "excluded letter i", text: strings.Repeat("0", 25) + "I"},
		{name: "excluded letter l", text: strings.Repeat("0", 25) + "L"},
		{name: "excluded letter o", text: strings.Repeat("0", 25) + "O"},
		{name: "excluded letter u", text: strings.Repeat("0", 25) + "U"},
		{name: "first character one above its ceiling", text: "8" + strings.Repeat("0", 24) + "1"},
		{name: "first character at the alphabet ceiling", text: "Z" + strings.Repeat("0", 24) + "1"},
		{name: "separator inside the spelling", text: "0000000000000-000000000001"},
		{name: "space padded tail", text: strings.Repeat("0", 25) + " "},
		{name: "multibyte rune at the exact extent", text: strings.Repeat("0", 24) + "é"},
		{name: "all zero spelling decodes to the unset value", text: strings.Repeat("0", 26)},
		{name: "uuid spelling offered to the ulid door", text: "00000000-0001-7000-8000-000000000001"},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := id.ParseULID(tc.text)
			requireIDContract(t, "ParseULID(hostile text)", err)
			if !got.IsZero() {
				t.Fatalf("ParseULID(%q) value = %v, want the zero value", tc.text, got)
			}
		})
	}
}

func TestULIDZeroValueNeverValidates(t *testing.T) {
	t.Parallel()

	var zero id.ULID
	if !zero.IsZero() || zero.IsValid() {
		t.Fatalf("zero ULID IsZero() = %t IsValid() = %t, want true and false", zero.IsZero(), zero.IsValid())
	}
	requireIDContract(t, "zero ULID Validate()", zero.Validate())
	if zero.String() != "" {
		t.Fatalf("zero ULID String() = %q, want empty", zero.String())
	}
}

func TestULIDStringOrderIsTimeOrder(t *testing.T) {
	t.Parallel()

	milliseconds := []int64{0, 1, 4294967295, 4294967296, time.Unix(0, math.MaxInt64).UnixMilli()}
	previous := ""
	for _, stamp := range milliseconds {
		value, err := id.NewULID(testRequest(t, stamp, testEntropy()))
		if err != nil {
			t.Fatalf("NewULID(%d ms) error = %v, want nil", stamp, err)
		}
		if got := value.String(); got <= previous {
			t.Fatalf("NewULID(%d ms).String() = %q, want lexicographically after %q", stamp, got, previous)
		} else {
			previous = got
		}
	}
}

func TestULIDJSONRoundTripIsCanonical(t *testing.T) {
	t.Parallel()

	value, err := id.NewULID(testRequest(t, 1, testEntropy()))
	if err != nil {
		t.Fatalf("NewULID(1 ms) error = %v, want nil", err)
	}
	gotJSON, err := value.MarshalJSON()
	if err != nil {
		t.Fatalf("ULID.MarshalJSON() error = %v, want nil", err)
	}
	wantJSON := `"` + ulidOneAfterEpochText + `"`
	if string(gotJSON) != wantJSON {
		t.Fatalf("ULID.MarshalJSON() = %s, want %s", gotJSON, wantJSON)
	}
	var decoded id.ULID
	if err := decoded.UnmarshalJSON(gotJSON); err != nil {
		t.Fatalf("ULID.UnmarshalJSON(canonical) error = %v, want nil", err)
	}
	if decoded != value {
		t.Fatalf("ULID JSON round trip = %v, want %v", decoded, value)
	}
}

func TestULIDJSONRefusesHostileTokensAndReceivers(t *testing.T) {
	t.Parallel()

	var zero id.ULID
	if _, err := zero.MarshalJSON(); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("zero ULID MarshalJSON() error = %v, want errors.Is %v", err, core.ErrJSONContract)
	}
	survivor, err := id.NewULID(testRequest(t, 1, testEntropy()))
	if err != nil {
		t.Fatalf("NewULID(1 ms) error = %v, want nil", err)
	}
	cases := []struct {
		name string
		data string
	}{
		{name: "numeric token", data: "123"},
		{name: "array token", data: "[]"},
		{name: "null token", data: "null"},
		{name: "lowercase spelling inside a string", data: `"01arz3ndektsv4rrffq69g5fav"`},
		{name: "overflowing first character inside a string", data: `"8` + strings.Repeat("0", 24) + `1"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			decoded := survivor
			if err := decoded.UnmarshalJSON([]byte(tc.data)); !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("ULID.UnmarshalJSON(%s) error = %v, want errors.Is %v", tc.data, err, core.ErrJSONContract)
			}
			if decoded != survivor {
				t.Fatalf("ULID.UnmarshalJSON(%s) mutated the receiver to %v, want %v unchanged", tc.data, decoded, survivor)
			}
		})
	}
	var absent *id.ULID
	if err := absent.UnmarshalJSON([]byte(`"` + ulidOneAfterEpochText + `"`)); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil ULID receiver UnmarshalJSON() error = %v, want errors.Is %v", err, core.ErrJSONContract)
	}
}

func TestULIDAppendTextSpellsTheOneCanonicalForm(t *testing.T) {
	t.Parallel()

	value, err := id.NewULID(testRequest(t, 1, testEntropy()))
	if err != nil {
		t.Fatalf("NewULID(1 ms) error = %v, want nil", err)
	}
	cases := []struct {
		name        string
		destination func() []byte
		wantPrefix  string
	}{
		{name: "nil destination receives exactly the spelling", destination: func() []byte { return nil }},
		{name: "empty destination with capacity receives exactly the spelling", destination: func() []byte { return make([]byte, 0, 64) }},
		{name: "existing prefix survives in front of the spelling", destination: func() []byte { return []byte(`"actor":"`) }, wantPrefix: `"actor":"`},
		{name: "full destination grows past its capacity", destination: func() []byte { return append(make([]byte, 0, 2), 'x', 'y') }, wantPrefix: "xy"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := value.AppendText(tc.destination())
			if err != nil {
				t.Fatalf("ULID.AppendText() error = %v, want nil", err)
			}
			want := tc.wantPrefix + value.String()
			if string(got) != want {
				t.Fatalf("ULID.AppendText() = %q, want %q", got, want)
			}
			parsed, err := id.ParseULID(string(got[len(tc.wantPrefix):]))
			if err != nil || parsed != value {
				t.Fatalf("ParseULID(appended spelling) = (%v, %v), want (%v, nil)", parsed, err, value)
			}
		})
	}
}

func TestULIDAppendTextIntoSufficientCapacityDoesNotAllocate(t *testing.T) {
	t.Parallel()

	value, err := id.NewULID(testRequest(t, 1, testEntropy()))
	if err != nil {
		t.Fatalf("NewULID(1 ms) error = %v, want nil", err)
	}
	destination := make([]byte, 0, 64)
	result := testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			appended, appendErr := value.AppendText(destination[:0])
			if appendErr != nil || len(appended) == 0 {
				b.Fatalf("ULID.AppendText() = (%d bytes, %v), want the spelling and nil", len(appended), appendErr)
			}
		}
	})
	if got := result.AllocsPerOp(); got != 0 {
		t.Fatalf("ULID.AppendText() into sufficient capacity allocs/op = %d, want 0", got)
	}
}

func TestULIDAppendTextRefusesTheUnsetValue(t *testing.T) {
	t.Parallel()

	var zero id.ULID
	got, err := zero.AppendText([]byte("prefix"))
	requireIDContract(t, "zero ULID AppendText()", err)
	if got != nil {
		t.Fatalf("zero ULID AppendText() = %q, want nil", got)
	}
}

func TestULIDBytesReturnsTheIdentityAndRefusesTheUnsetValue(t *testing.T) {
	t.Parallel()

	value, err := id.NewULID(testRequest(t, 1, testEntropy()))
	if err != nil {
		t.Fatalf("NewULID(1 ms) error = %v, want nil", err)
	}
	got, err := value.Bytes()
	if err != nil {
		t.Fatalf("ULID.Bytes() error = %v, want nil", err)
	}
	// The bytes are the value the canonical spelling renders, so parsing that
	// spelling back must reproduce them exactly.
	reparsed, err := id.ParseULID(value.String())
	if err != nil {
		t.Fatalf("ParseULID(%q) error = %v, want nil", value.String(), err)
	}
	same, err := reparsed.Bytes()
	if err != nil {
		t.Fatalf("reparsed ULID.Bytes() error = %v, want nil", err)
	}
	if got != same {
		t.Fatalf("ULID.Bytes() = %x, want %x after a spelling round trip", got, same)
	}
	// The leading six bytes are the big-endian millisecond stamp, so byte
	// order is time order in the bytes exactly as it is in the spelling.
	wantStamp := [6]byte{0, 0, 0, 0, 0, 1}
	if [6]byte(got[:6]) != wantStamp {
		t.Fatalf("ULID.Bytes() stamp = %x, want %x", got[:6], wantStamp)
	}

	var zero id.ULID
	empty, err := zero.Bytes()
	requireIDContract(t, "zero ULID Bytes()", err)
	if empty != [16]byte{} {
		t.Fatalf("zero ULID Bytes() = %x, want the zero array", empty)
	}
}

func TestULIDBytesHandsBackACopyTheCallerCannotWriteThrough(t *testing.T) {
	t.Parallel()

	value, err := id.NewULID(testRequest(t, 7, testEntropy()))
	if err != nil {
		t.Fatalf("NewULID(7 ms) error = %v, want nil", err)
	}
	first, err := value.Bytes()
	if err != nil {
		t.Fatalf("ULID.Bytes() error = %v, want nil", err)
	}
	spelling := value.String()
	for i := range first {
		first[i] ^= 0xff
	}
	second, err := value.Bytes()
	if err != nil {
		t.Fatalf("ULID.Bytes() error = %v, want nil", err)
	}
	if first == second {
		t.Fatalf("ULID.Bytes() handed back the same array the caller mutated")
	}
	if value.String() != spelling {
		t.Fatalf("ULID spelling = %q after caller mutation, want %q", value.String(), spelling)
	}
}

func TestNewULIDFromBytesRoundTripsAndRefusesTheUnsetArray(t *testing.T) {
	t.Parallel()

	minted, err := id.NewULID(testRequest(t, 3, testEntropy()))
	if err != nil {
		t.Fatalf("NewULID(3 ms) error = %v, want nil", err)
	}
	raw, err := minted.Bytes()
	if err != nil {
		t.Fatalf("ULID.Bytes() error = %v, want nil", err)
	}
	rebuilt, err := id.NewULIDFromBytes(raw)
	if err != nil {
		t.Fatalf("NewULIDFromBytes() error = %v, want nil", err)
	}
	if rebuilt != minted {
		t.Fatalf("NewULIDFromBytes(Bytes()) = %v, want %v", rebuilt, minted)
	}
	if rebuilt.String() != minted.String() {
		t.Fatalf("rebuilt spelling = %q, want %q", rebuilt.String(), minted.String())
	}

	// The unset array is the absent value, not an identity, so it must not be
	// admittable as one.
	absent, err := id.NewULIDFromBytes([16]byte{})
	requireIDContract(t, "NewULIDFromBytes(zero array)", err)
	if !absent.IsZero() {
		t.Fatalf("NewULIDFromBytes(zero array) = %v, want the unset value", absent)
	}

	// A single set bit anywhere is a legal identity: only all zero is absent.
	for _, position := range []int{0, 5, 6, 15} {
		var single [16]byte
		single[position] = 0x01
		value, err := id.NewULIDFromBytes(single)
		if err != nil {
			t.Fatalf("NewULIDFromBytes(bit at %d) error = %v, want nil", position, err)
		}
		back, err := value.Bytes()
		if err != nil || back != single {
			t.Fatalf("NewULIDFromBytes(bit at %d).Bytes() = (%x, %v), want %x", position, back, err, single)
		}
	}
}
