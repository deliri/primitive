package temporal_test

import (
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// numericInstantCarrier is the reason the projection exists: a wire struct
// whose members are declared with ordinary json tags and whose encoded bytes
// must be bare numbers.
type numericInstantCarrier struct {
	StartUnixNanos temporal.NumericInstant  `json:"start_unix_nanos"`
	ElapsedNanos   temporal.NumericDuration `json:"elapsed_nanos"`
}

func mustInstant(t *testing.T, nanoseconds int64) temporal.NumericInstant {
	t.Helper()
	got, err := temporal.NewNumericInstant(temporal.InstantFromNanoseconds(nanoseconds))
	if err != nil {
		t.Fatalf("NewNumericInstant(%d) error = %v, want nil", nanoseconds, err)
	}
	return got
}

func mustDuration(t *testing.T, nanoseconds int64) temporal.NumericDuration {
	t.Helper()
	value, err := temporal.DurationFromNanoseconds(nanoseconds)
	if err != nil {
		t.Fatalf("DurationFromNanoseconds(%d) error = %v, want nil", nanoseconds, err)
	}
	got, err := temporal.NewNumericDuration(value)
	if err != nil {
		t.Fatalf("NewNumericDuration(%d) error = %v, want nil", nanoseconds, err)
	}
	return got
}

// TestNumericInstantEncodesBareNumbersAcrossTheSignedDomain is the contract the
// string projection cannot satisfy: the emitted member is a JSON number, and it
// is the exact decimal strconv produces for every extreme of the domain.
func TestNumericInstantEncodesBareNumbersAcrossTheSignedDomain(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		want        string
		nanoseconds int64
	}{
		{name: "epoch is zero rather than an absent member", nanoseconds: 0, want: "0"},
		{name: "one nanosecond after the epoch", nanoseconds: 1, want: "1"},
		{name: "one nanosecond before the epoch stays signed", nanoseconds: -1, want: "-1"},
		{name: "exactly one second", nanoseconds: 1_000_000_000, want: "1000000000"},
		{name: "one nanosecond below one second", nanoseconds: 999_999_999, want: "999999999"},
		{name: "one nanosecond above one second", nanoseconds: 1_000_000_001, want: "1000000001"},
		{name: "a realistic evidence instant", nanoseconds: 1_735_689_600_000_000_000, want: "1735689600000000000"},
		{name: "maximum signed nanoseconds", nanoseconds: math.MaxInt64, want: "9223372036854775807"},
		{name: "one below maximum signed nanoseconds", nanoseconds: math.MaxInt64 - 1, want: "9223372036854775806"},
		{name: "minimum signed nanoseconds", nanoseconds: math.MinInt64, want: "-9223372036854775808"},
		{name: "one above minimum signed nanoseconds", nanoseconds: math.MinInt64 + 1, want: "-9223372036854775807"},
		{name: "a pre-epoch instant with a full second magnitude", nanoseconds: -1_000_000_000, want: "-1000000000"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, err := json.Marshal(mustInstant(t, testCase.nanoseconds))
			if err != nil {
				t.Fatalf("json.Marshal() error = %v, want nil", err)
			}
			if string(got) != testCase.want {
				t.Fatalf("json.Marshal() = %s, want %s", got, testCase.want)
			}
			if strings.ContainsRune(string(got), '"') {
				t.Fatalf("json.Marshal() = %s, want a bare number with no quotes", got)
			}
		})
	}
}

// TestNumericDurationEncodesBareNonNegativeNumbers holds the duration half of
// the same wire contract.
func TestNumericDurationEncodesBareNonNegativeNumbers(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		want        string
		nanoseconds int64
	}{
		{name: "a real zero duration is emitted, not omitted", nanoseconds: 0, want: "0"},
		{name: "one nanosecond", nanoseconds: 1, want: "1"},
		{name: "one microsecond", nanoseconds: 1_000, want: "1000"},
		{name: "one nanosecond below one microsecond", nanoseconds: 999, want: "999"},
		{name: "one nanosecond above one microsecond", nanoseconds: 1_001, want: "1001"},
		{name: "one second", nanoseconds: 1_000_000_000, want: "1000000000"},
		{name: "one hour", nanoseconds: 3_600_000_000_000, want: "3600000000000"},
		{name: "maximum bounded duration", nanoseconds: math.MaxInt64, want: "9223372036854775807"},
		{name: "one below maximum bounded duration", nanoseconds: math.MaxInt64 - 1, want: "9223372036854775806"},
		{name: "a wide but representable core-second magnitude", nanoseconds: 1_000_000_000_000_000_000, want: "1000000000000000000"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, err := json.Marshal(mustDuration(t, testCase.nanoseconds))
			if err != nil {
				t.Fatalf("json.Marshal() error = %v, want nil", err)
			}
			if string(got) != testCase.want {
				t.Fatalf("json.Marshal() = %s, want %s", got, testCase.want)
			}
		})
	}
}

// TestNumericInstantDecodeRejectsEveryNoncanonicalEncoding proves that one
// value keeps exactly one accepted encoding, and that a rejection never
// mutates the receiver.
func TestNumericInstantDecodeRejectsEveryNoncanonicalEncoding(t *testing.T) {
	t.Parallel()

	oversize := strings.Repeat("9", temporal.NumericInstantCanonicalJSONMaximumBytes+1)

	cases := []struct {
		name string
		in   string
	}{
		{name: "empty input", in: ""},
		{name: "the string projection is not accepted here", in: `"1"`},
		{name: "a quoted negative is not accepted", in: `"-1"`},
		{name: "JSON null", in: "null"},
		{name: "JSON true", in: "true"},
		{name: "JSON object", in: "{}"},
		{name: "JSON array", in: "[1]"},
		{name: "leading zero", in: "01"},
		{name: "negative leading zero", in: "-01"},
		{name: "negative zero", in: "-0"},
		{name: "explicit plus sign", in: "+1"},
		{name: "bare minus sign", in: "-"},
		{name: "fraction", in: "1.0"},
		{name: "trailing fraction point", in: "1."},
		{name: "exponent", in: "1e9"},
		{name: "capital exponent", in: "1E9"},
		{name: "leading whitespace", in: " 1"},
		{name: "trailing whitespace", in: "1 "},
		{name: "internal separator", in: "1_000"},
		{name: "hexadecimal", in: "0x10"},
		{name: "non-ASCII digit", in: "١"},
		{name: "one byte above the canonical extent", in: oversize},
		{name: "exactly at the extent but not numeric", in: strings.Repeat("a", temporal.NumericInstantCanonicalJSONMaximumBytes)},
		{name: "one above maximum signed nanoseconds", in: "9223372036854775808"},
		{name: "one below minimum signed nanoseconds", in: "-9223372036854775809"},
		{name: "double negative", in: "--1"},
		{name: "trailing sign", in: "1-"},
		{name: "duplicated digits past int64 width", in: "99999999999999999999"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			retained := mustInstant(t, 7)
			got := retained
			err := got.UnmarshalJSON([]byte(testCase.in))
			if err == nil {
				t.Fatalf("UnmarshalJSON(%q) error = nil, want a temporal rejection", testCase.in)
			}
			if !errors.Is(err, core.ErrTemporalContract) && !errors.Is(err, core.ErrTemporalOverflow) {
				t.Fatalf("UnmarshalJSON(%q) error = %v, want ErrTemporalContract or ErrTemporalOverflow", testCase.in, err)
			}
			gotNanoseconds := requireNanoseconds(t, got)
			wantNanoseconds := requireNanoseconds(t, retained)
			if gotNanoseconds != wantNanoseconds {
				t.Fatalf("receiver after rejection = %d, want %d unchanged", gotNanoseconds, wantNanoseconds)
			}
		})
	}
}

func requireNanoseconds(t *testing.T, value temporal.NumericInstant) int64 {
	t.Helper()
	instant, err := value.Instant()
	if err != nil {
		t.Fatalf("Instant() error = %v, want nil", err)
	}
	nanoseconds, err := instant.Nanoseconds()
	if err != nil {
		t.Fatalf("Nanoseconds() error = %v, want nil", err)
	}
	return nanoseconds
}

// TestNumericDurationDecodeRejectsNegativeAndNoncanonicalInput adds the
// nonnegative boundary Instant does not have.
func TestNumericDurationDecodeRejectsNegativeAndNoncanonicalInput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
	}{
		{name: "one nanosecond below zero", in: "-1"},
		{name: "a large negative duration", in: "-1000000000"},
		{name: "minimum signed nanoseconds", in: "-9223372036854775808"},
		{name: "the string projection is not accepted here", in: `"0"`},
		{name: "JSON null", in: "null"},
		{name: "leading zero", in: "00"},
		{name: "negative zero", in: "-0"},
		{name: "fraction", in: "0.5"},
		{name: "exponent", in: "1e3"},
		{name: "empty input", in: ""},
		{name: "one above maximum bounded duration", in: "9223372036854775808"},
		{name: "one byte above the canonical extent", in: strings.Repeat("9", temporal.NumericDurationCanonicalJSONMaximumBytes+1)},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			retained := mustDuration(t, 42)
			got := retained
			err := got.UnmarshalJSON([]byte(testCase.in))
			if err == nil {
				t.Fatalf("UnmarshalJSON(%q) error = nil, want a temporal rejection", testCase.in)
			}
			if !errors.Is(err, core.ErrTemporalContract) && !errors.Is(err, core.ErrTemporalOverflow) {
				t.Fatalf("UnmarshalJSON(%q) error = %v, want ErrTemporalContract or ErrTemporalOverflow", testCase.in, err)
			}
			if got.Duration().Nanoseconds() != retained.Duration().Nanoseconds() {
				t.Fatalf("receiver after rejection = %d, want %d unchanged", got.Duration().Nanoseconds(), retained.Duration().Nanoseconds())
			}
		})
	}
}

// TestNumericValuesAcceptTheirExactBoundaryEncodings holds the accepted side of
// the extent boundary that the rejection tables hold from above.
func TestNumericValuesAcceptTheirExactBoundaryEncodings(t *testing.T) {
	t.Parallel()

	minimum := strconv.FormatInt(math.MinInt64, 10)
	if len(minimum) != temporal.NumericInstantCanonicalJSONMaximumBytes {
		t.Fatalf("minimum instant encoding = %d bytes, want exactly the canonical bound %d", len(minimum), temporal.NumericInstantCanonicalJSONMaximumBytes)
	}
	maximum := strconv.FormatInt(math.MaxInt64, 10)
	if len(maximum) != temporal.NumericDurationCanonicalJSONMaximumBytes {
		t.Fatalf("maximum duration encoding = %d bytes, want exactly the canonical bound %d", len(maximum), temporal.NumericDurationCanonicalJSONMaximumBytes)
	}

	var instant temporal.NumericInstant
	if err := instant.UnmarshalJSON([]byte(minimum)); err != nil {
		t.Fatalf("UnmarshalJSON(%s) error = %v, want nil at the exact extent", minimum, err)
	}
	if got := requireNanoseconds(t, instant); got != math.MinInt64 {
		t.Fatalf("decoded instant = %d, want %d", got, int64(math.MinInt64))
	}

	var duration temporal.NumericDuration
	if err := duration.UnmarshalJSON([]byte(maximum)); err != nil {
		t.Fatalf("UnmarshalJSON(%s) error = %v, want nil at the exact extent", maximum, err)
	}
	if got := duration.Duration().Nanoseconds(); got != math.MaxInt64 {
		t.Fatalf("decoded duration = %d, want %d", got, int64(math.MaxInt64))
	}
}

// TestNumericValuesRoundTripThroughARealWireStruct proves the production
// shape: struct tags, encoding/json, and stable re-encoding.
func TestNumericValuesRoundTripThroughARealWireStruct(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		wire  string
		start int64
	}{
		{name: "epoch start with a zero elapsed observation", start: 0, wire: `{"start_unix_nanos":0,"elapsed_nanos":0}`},
		{name: "a realistic evidence pair", start: 1_735_689_600_000_000_000, wire: `{"start_unix_nanos":1735689600000000000,"elapsed_nanos":1735689600000000000}`},
		{name: "a pre-epoch start", start: -1, wire: `{"start_unix_nanos":-1,"elapsed_nanos":0}`},
		{name: "minimum start with maximum elapsed", start: math.MinInt64, wire: `{"start_unix_nanos":-9223372036854775808,"elapsed_nanos":9223372036854775807}`},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			elapsed := int64(0)
			if testCase.start == 1_735_689_600_000_000_000 {
				elapsed = testCase.start
			}
			if testCase.start == math.MinInt64 {
				elapsed = math.MaxInt64
			}
			original := numericInstantCarrier{
				StartUnixNanos: mustInstant(t, testCase.start),
				ElapsedNanos:   mustDuration(t, elapsed),
			}

			encoded, err := json.Marshal(original)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v, want nil", err)
			}
			if string(encoded) != testCase.wire {
				t.Fatalf("json.Marshal() = %s, want %s", encoded, testCase.wire)
			}

			var decoded numericInstantCarrier
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatalf("json.Unmarshal() error = %v, want nil", err)
			}
			if got := requireNanoseconds(t, decoded.StartUnixNanos); got != testCase.start {
				t.Fatalf("decoded start = %d, want %d", got, testCase.start)
			}
			if got := decoded.ElapsedNanos.Duration().Nanoseconds(); got != elapsed {
				t.Fatalf("decoded elapsed = %d, want %d", got, elapsed)
			}

			reencoded, err := json.Marshal(decoded)
			if err != nil {
				t.Fatalf("second json.Marshal() error = %v, want nil", err)
			}
			if string(reencoded) != string(encoded) {
				t.Fatalf("re-encoded = %s, want byte-stable %s", reencoded, encoded)
			}
		})
	}
}

// TestNumericZeroValuesFollowTheirProjectedTypes holds the neutral case: an
// unset instant refuses to encode, while a zero duration is a real observation.
func TestNumericZeroValuesFollowTheirProjectedTypes(t *testing.T) {
	t.Parallel()

	var instant temporal.NumericInstant
	if instant.IsSet() {
		t.Fatal("zero NumericInstant IsSet() = true, want false")
	}
	if err := instant.Validate(); !errors.Is(err, core.ErrTemporalContract) {
		t.Fatalf("zero NumericInstant Validate() error = %v, want ErrTemporalContract", err)
	}
	if _, err := instant.Instant(); !errors.Is(err, core.ErrTemporalContract) {
		t.Fatalf("zero NumericInstant Instant() error = %v, want ErrTemporalContract", err)
	}
	if _, err := json.Marshal(instant); !errors.Is(err, core.ErrTemporalContract) {
		t.Fatalf("json.Marshal(zero NumericInstant) error = %v, want errors.Is %v", err, core.ErrTemporalContract)
	}

	var duration temporal.NumericDuration
	if !duration.IsZero() {
		t.Fatal("zero NumericDuration IsZero() = false, want true")
	}
	if err := duration.Validate(); err != nil {
		t.Fatalf("zero NumericDuration Validate() error = %v, want nil", err)
	}
	encoded, err := json.Marshal(duration)
	if err != nil {
		t.Fatalf("json.Marshal(zero NumericDuration) error = %v, want nil", err)
	}
	if string(encoded) != "0" {
		t.Fatalf("json.Marshal(zero NumericDuration) = %s, want 0", encoded)
	}
}

// TestNumericConstructorsRejectUnsetAndTypedNilReceivers closes the two
// remaining ingress boundaries.
func TestNumericConstructorsRejectUnsetAndTypedNilReceivers(t *testing.T) {
	t.Parallel()

	if _, err := temporal.NewNumericInstant(temporal.Instant{}); !errors.Is(err, core.ErrTemporalContract) {
		t.Fatalf("NewNumericInstant(unset) error = %v, want ErrTemporalContract", err)
	}
	if _, err := temporal.NewNumericDuration(temporal.Duration{}); err != nil {
		t.Fatalf("NewNumericDuration(zero) error = %v, want nil because a zero duration is real", err)
	}

	var nilInstant *temporal.NumericInstant
	if err := nilInstant.UnmarshalJSON([]byte("1")); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil NumericInstant UnmarshalJSON() error = %v, want ErrJSONContract", err)
	}
	var nilDuration *temporal.NumericDuration
	if err := nilDuration.UnmarshalJSON([]byte("1")); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil NumericDuration UnmarshalJSON() error = %v, want ErrJSONContract", err)
	}
}

// TestNumericProjectionPreservesInstantValueSemantics proves the projection
// adds encoding only: the carried Instant remains the same Primitive value and
// still routes through Primitive arithmetic.
func TestNumericProjectionPreservesInstantValueSemantics(t *testing.T) {
	t.Parallel()

	start := mustInstant(t, 1_000)
	instant, err := start.Instant()
	if err != nil {
		t.Fatalf("Instant() error = %v, want nil", err)
	}
	step, err := temporal.DurationFromNanoseconds(500)
	if err != nil {
		t.Fatalf("DurationFromNanoseconds() error = %v, want nil", err)
	}
	advanced, err := instant.Add(step)
	if err != nil {
		t.Fatalf("Add() error = %v, want nil", err)
	}
	projected, err := temporal.NewNumericInstant(advanced)
	if err != nil {
		t.Fatalf("NewNumericInstant() error = %v, want nil", err)
	}
	if got := requireNanoseconds(t, projected); got != 1_500 {
		t.Fatalf("projected instant = %d, want 1500", got)
	}

	comparison, err := advanced.Compare(instant)
	if err != nil {
		t.Fatalf("Compare() error = %v, want nil", err)
	}
	if comparison != core.ComparisonGreater {
		t.Fatalf("Compare() = %v, want %v", comparison, core.ComparisonGreater)
	}
}

// FuzzNumericInstantJSON pressures the decode boundary with arbitrary bytes.
// Its oracle is independent of the decoder: an accepted document must re-encode
// to exactly the bytes that were accepted, and a rejection must carry a stable
// typed identity and leave the receiver untouched.
func FuzzNumericInstantJSON(f *testing.F) {
	seeds := []string{
		"0", "1", "-1", "9223372036854775807", "-9223372036854775808",
		"01", "-0", "1e9", "1.0", `"1"`, "", "null", "+1", "9223372036854775808",
	}
	for _, seed := range seeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		sentinel := temporal.InstantFromNanoseconds(-77)
		retained, err := temporal.NewNumericInstant(sentinel)
		if err != nil {
			t.Fatalf("NewNumericInstant() error = %v, want nil", err)
		}
		got := retained

		if err := got.UnmarshalJSON(data); err != nil {
			if !errors.Is(err, core.ErrTemporalContract) && !errors.Is(err, core.ErrTemporalOverflow) {
				t.Fatalf("UnmarshalJSON(%q) error = %v, want a stable temporal identity", data, err)
			}
			instant, instantErr := got.Instant()
			if instantErr != nil {
				t.Fatalf("Instant() after rejection error = %v, want nil", instantErr)
			}
			nanoseconds, nanosecondErr := instant.Nanoseconds()
			if nanosecondErr != nil {
				t.Fatalf("Nanoseconds() after rejection error = %v, want nil", nanosecondErr)
			}
			if nanoseconds != -77 {
				t.Fatalf("receiver after rejection = %d, want -77 unchanged", nanoseconds)
			}
			return
		}

		if err := got.Validate(); err != nil {
			t.Fatalf("UnmarshalJSON accepted a value that fails Validate: %v", err)
		}
		encoded, err := got.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON() after accepted decode error = %v, want nil", err)
		}
		if string(encoded) != string(data) {
			t.Fatalf("re-encoded = %s, want the accepted bytes %s", encoded, data)
		}
	})
}

// FuzzNumericDurationJSON holds the same boundary for the nonnegative half.
func FuzzNumericDurationJSON(f *testing.F) {
	seeds := []string{"0", "1", "9223372036854775807", "-1", "-0", "00", "1e3", `"0"`, ""}
	for _, seed := range seeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		base, err := temporal.DurationFromNanoseconds(13)
		if err != nil {
			t.Fatalf("DurationFromNanoseconds() error = %v, want nil", err)
		}
		retained, err := temporal.NewNumericDuration(base)
		if err != nil {
			t.Fatalf("NewNumericDuration() error = %v, want nil", err)
		}
		got := retained

		if err := got.UnmarshalJSON(data); err != nil {
			if !errors.Is(err, core.ErrTemporalContract) && !errors.Is(err, core.ErrTemporalOverflow) {
				t.Fatalf("UnmarshalJSON(%q) error = %v, want a stable temporal identity", data, err)
			}
			if got.Duration().Nanoseconds() != 13 {
				t.Fatalf("receiver after rejection = %d, want 13 unchanged", got.Duration().Nanoseconds())
			}
			return
		}

		if got.Duration().Nanoseconds() < 0 {
			t.Fatalf("UnmarshalJSON accepted a negative duration: %d", got.Duration().Nanoseconds())
		}
		encoded, err := got.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON() after accepted decode error = %v, want nil", err)
		}
		if string(encoded) != string(data) {
			t.Fatalf("re-encoded = %s, want the accepted bytes %s", encoded, data)
		}
	})
}
