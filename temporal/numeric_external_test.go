package temporal_test

import (
	json "encoding/json/v2"
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
			var decoded temporal.NumericInstant
			if err := decoded.UnmarshalJSON(got); err != nil {
				t.Fatal(err)
			}
			point, pointErr := decoded.Instant()
			nanos, nanosErr := point.Nanoseconds()
			if pointErr != nil || nanosErr != nil || nanos != testCase.nanoseconds {
				t.Fatalf("decoded instant = (%d,%v,%v), want %d", nanos, pointErr, nanosErr, testCase.nanoseconds)
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
			var decoded temporal.NumericDuration
			if err := decoded.UnmarshalJSON(got); err != nil {
				t.Fatal(err)
			}
			if nanos := decoded.Duration().Nanoseconds(); nanos != testCase.nanoseconds {
				t.Fatalf("decoded duration = %d, want %d", nanos, testCase.nanoseconds)
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
			gotInstant, gotInstantErr := got.Instant()
			gotNanoseconds, gotNanosecondsErr := gotInstant.Nanoseconds()
			if gotInstantErr != nil || gotNanosecondsErr != nil {
				t.Fatalf("numeric projection = (%v,%v), want nil", gotInstantErr, gotNanosecondsErr)
			}
			wantNanoseconds := int64(7)
			if gotNanoseconds != wantNanoseconds {
				t.Fatalf("receiver after rejection = %d, want %d unchanged", gotNanoseconds, wantNanoseconds)
			}
		})
	}
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
			point, pointErr := decoded.StartUnixNanos.Instant()
			got, nanosErr := point.Nanoseconds()
			if pointErr != nil || nanosErr != nil {
				t.Fatalf("decoded projection errors = (%v,%v), want nil", pointErr, nanosErr)
			}
			if got != testCase.start {
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

func TestNumericValueProjectionLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name           string
		start          temporal.Instant
		duration       int64
		wantInstantErr error
	}{
		{name: "unset instant remains invalid beside real zero duration", wantInstantErr: core.ErrTemporalContract},
		{name: "epoch remains set beside real zero duration", start: temporal.InstantFromNanoseconds(0)},
		{name: "negative instant stays signed beside positive elapsed", start: temporal.InstantFromNanoseconds(-1), duration: 1},
		{name: "minimum instant and maximum elapsed remain exact", start: temporal.InstantFromNanoseconds(math.MinInt64), duration: math.MaxInt64},
		{name: "maximum instant survives numeric projection", start: temporal.InstantFromNanoseconds(math.MaxInt64), duration: 9_007_199_254_740_993},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			instant, instantErr := temporal.NewNumericInstant(tc.start)
			projected, projectionErr := instant.Instant()
			wire, wireErr := instant.MarshalJSON()
			if !errors.Is(instantErr, tc.wantInstantErr) || !errors.Is(instant.Validate(), tc.wantInstantErr) || !errors.Is(projectionErr, tc.wantInstantErr) || !errors.Is(wireErr, tc.wantInstantErr) || instant.IsSet() != tc.start.IsSet() || projected != tc.start {
				t.Fatalf("numeric instant = (%v,%v,%v,%v,%v), want %v with %v", instant, instantErr, projected, projectionErr, wireErr, tc.start, tc.wantInstantErr)
			}
			if tc.wantInstantErr != nil {
				if wire != nil || instant != (temporal.NumericInstant{}) {
					t.Fatalf("unset projection = (%q,%v), want nil and zero", wire, instant)
				}
			} else {
				nanos, err := tc.start.Nanoseconds()
				if err != nil {
					t.Fatal(err)
				}
				if string(wire) != strconv.FormatInt(nanos, 10) {
					t.Fatalf("instant wire = %q, want exact bare %d", wire, nanos)
				}
			}
			duration, err := temporal.DurationFromNanoseconds(tc.duration)
			if err != nil {
				t.Fatal(err)
			}
			numeric, numericErr := temporal.NewNumericDuration(duration)
			durationWire, durationWireErr := numeric.MarshalJSON()
			if numericErr != nil || numeric.Validate() != nil || numeric.Duration() != duration || numeric.IsZero() != (tc.duration == 0) || durationWireErr != nil || string(durationWire) != strconv.FormatInt(tc.duration, 10) {
				t.Fatalf("numeric duration = (%v,%v,%q,%v), want exact %v", numeric, numericErr, durationWire, durationWireErr, duration)
			}
		})
	}
}

func TestNumericNilReceiversRefuseBeforeMutation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		decode func([]byte) error
	}{
		{name: "nil instant receiver", decode: (*temporal.NumericInstant)(nil).UnmarshalJSON},
		{name: "nil duration receiver", decode: (*temporal.NumericDuration)(nil).UnmarshalJSON},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if gotErr := tc.decode([]byte("1")); !errors.Is(gotErr, core.ErrJSONContract) || !errors.Is(gotErr, core.ErrTemporalContract) {
				t.Fatalf("nil receiver error = %v, want typed JSON and Temporal refusal", gotErr)
			}
		})
	}
}

func TestNumericProjectionPreservesInstantValueSemantics(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name              string
		start, step, want int64
		wantOrder         core.Comparison
		wantErr           error
	}{
		{name: "zero displacement is neutral", start: -1, want: -1, wantOrder: core.ComparisonEqual},
		{name: "advance crosses epoch exactly", start: -1, step: 1, want: 0, wantOrder: core.ComparisonGreater},
		{name: "largest elapsed spans minimum to negative one", start: math.MinInt64, step: math.MaxInt64, want: -1, wantOrder: core.ComparisonGreater},
		{name: "maximum instant refuses further displacement", start: math.MaxInt64, step: 1, wantErr: core.ErrTemporalOverflow},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			numeric := mustInstant(t, tc.start)
			start, startErr := numeric.Instant()
			if startErr != nil {
				t.Fatal(startErr)
			}
			step, stepErr := temporal.DurationFromNanoseconds(tc.step)
			if stepErr != nil {
				t.Fatal(stepErr)
			}
			advanced, gotErr := start.Add(step)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("advance error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if advanced != (temporal.Instant{}) {
					t.Fatalf("refused advance = %v, want zero", advanced)
				}
				return
			}
			projected, projectErr := temporal.NewNumericInstant(advanced)
			got, gotProjectionErr := projected.Instant()
			nanos, nanosErr := got.Nanoseconds()
			order, orderErr := got.Compare(start)
			if projectErr != nil || gotProjectionErr != nil || nanosErr != nil || orderErr != nil || nanos != tc.want || order != tc.wantOrder {
				t.Fatalf("advance facts = (%d,%v,%v,%v,%v,%v), want %d and %v", nanos, order, projectErr, gotProjectionErr, nanosErr, orderErr, tc.want, tc.wantOrder)
			}
		})
	}
}

// FuzzNumericInstantJSON pressures the decode boundary with arbitrary bytes.
// Its oracle is independent of the decoder: an accepted document must re-encode
// to exactly the bytes that were accepted, and a rejection must carry a stable
// typed identity and leave the receiver untouched.
func FuzzNumericInstantJSON(f *testing.F) {
	for _, nanoseconds := range []int64{math.MinInt64, -1, 0, 1, math.MaxInt64} {
		value, err := temporal.NewNumericInstant(temporal.InstantFromNanoseconds(nanoseconds))
		if err != nil {
			f.Fatalf("NewNumericInstant(%d) seed error = %v, want nil", nanoseconds, err)
		}
		encoded, err := value.MarshalJSON()
		if err != nil {
			f.Fatalf("NumericInstant.MarshalJSON(%d) seed error = %v, want nil", nanoseconds, err)
		}
		f.Add(encoded)
	}
	for _, malformed := range []string{"01", "-0", "1e9", "1.0", `"1"`, "", "null", "+1", "9223372036854775808"} {
		f.Add([]byte(malformed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > temporal.NumericInstantCanonicalJSONMaximumBytes {
			retained, retainedErr := temporal.NewNumericInstant(temporal.InstantFromNanoseconds(-77))
			if retainedErr != nil {
				t.Fatalf("NewNumericInstant(retained) error = %v, want nil", retainedErr)
			}
			got := retained
			gotErr := got.UnmarshalJSON(data)
			if !errors.Is(gotErr, core.ErrJSONContract) || !errors.Is(gotErr, core.ErrTemporalContract) || got != retained {
				t.Fatalf("NumericInstant.UnmarshalJSON(%d-byte oversized input) = (%v, %v), want retained/%v/%v", len(data), got, gotErr, core.ErrJSONContract, core.ErrTemporalContract)
			}
			return
		}
		independent, independentErr := strconv.ParseInt(string(data), 10, 64)
		wantAccepted := independentErr == nil && strconv.FormatInt(independent, 10) == string(data)
		sentinel := temporal.InstantFromNanoseconds(-77)
		retained, err := temporal.NewNumericInstant(sentinel)
		if err != nil {
			t.Fatalf("NewNumericInstant() error = %v, want nil", err)
		}
		got := retained

		if err := got.UnmarshalJSON(data); err != nil {
			if wantAccepted || (!errors.Is(err, core.ErrTemporalContract) || !errors.Is(err, core.ErrJSONContract)) {
				t.Fatalf("UnmarshalJSON(%q) error = %v, want accepted=%t or a stable temporal identity", data, err, wantAccepted)
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

		gotInstant, gotInstantErr := got.Instant()
		gotNanoseconds, gotNanosecondsErr := gotInstant.Nanoseconds()
		if gotInstantErr != nil || gotNanosecondsErr != nil {
			t.Fatalf("numeric projection = (%v,%v), want nil", gotInstantErr, gotNanosecondsErr)
		}
		if gotErr := got.Validate(); !wantAccepted || gotErr != nil || gotNanoseconds != independent {
			t.Fatalf("UnmarshalJSON(%q) = (value:%d validate:%v), want accepted=%t value=%d", data, gotNanoseconds, gotErr, wantAccepted, independent)
		}
		encoded, err := got.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON() after accepted decode error = %v, want nil", err)
		}
		if string(encoded) != string(data) {
			t.Fatalf("re-encoded = %s, want the accepted bytes %s", encoded, data)
		}
		var roundTrip temporal.NumericInstant
		if err := roundTrip.UnmarshalJSON(encoded); err != nil {
			t.Fatalf("second UnmarshalJSON(%s) error = %v, want nil", encoded, err)
		}
		second, err := roundTrip.MarshalJSON()
		roundTripInstant, roundTripInstantErr := roundTrip.Instant()
		roundTripNanoseconds, roundTripNanosErr := roundTripInstant.Nanoseconds()
		if roundTripInstantErr != nil || roundTripNanosErr != nil {
			t.Fatalf("round trip projection errors = (%v,%v), want nil", roundTripInstantErr, roundTripNanosErr)
		}
		if err != nil || roundTripNanoseconds != gotNanoseconds || string(second) != string(encoded) {
			t.Fatalf("numeric instant closure = (%s, %v, %d), want (%s, nil, %d)", second, err, roundTripNanoseconds, encoded, gotNanoseconds)
		}
	})
}

// FuzzNumericDurationJSON holds the same boundary for the nonnegative half.
func FuzzNumericDurationJSON(f *testing.F) {
	for _, nanoseconds := range []int64{0, 1, 1_000, math.MaxInt64} {
		duration, err := temporal.DurationFromNanoseconds(nanoseconds)
		if err != nil {
			f.Fatalf("DurationFromNanoseconds(%d) seed error = %v, want nil", nanoseconds, err)
		}
		value, err := temporal.NewNumericDuration(duration)
		if err != nil {
			f.Fatalf("NewNumericDuration(%d) seed error = %v, want nil", nanoseconds, err)
		}
		encoded, err := value.MarshalJSON()
		if err != nil {
			f.Fatalf("NumericDuration.MarshalJSON(%d) seed error = %v, want nil", nanoseconds, err)
		}
		f.Add(encoded)
	}
	for _, malformed := range []string{"-1", "-0", "00", "1e3", `"0"`, ""} {
		f.Add([]byte(malformed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > temporal.NumericDurationCanonicalJSONMaximumBytes {
			retainedDuration, retainedDurationErr := temporal.DurationFromNanoseconds(13)
			if retainedDurationErr != nil {
				t.Fatalf("DurationFromNanoseconds(retained) error = %v, want nil", retainedDurationErr)
			}
			retained, retainedErr := temporal.NewNumericDuration(retainedDuration)
			if retainedErr != nil {
				t.Fatalf("NewNumericDuration(retained) error = %v, want nil", retainedErr)
			}
			got := retained
			gotErr := got.UnmarshalJSON(data)
			if !errors.Is(gotErr, core.ErrJSONContract) || !errors.Is(gotErr, core.ErrTemporalContract) || got != retained {
				t.Fatalf("NumericDuration.UnmarshalJSON(%d-byte oversized input) = (%v, %v), want retained/%v/%v", len(data), got, gotErr, core.ErrJSONContract, core.ErrTemporalContract)
			}
			return
		}
		independent, independentErr := strconv.ParseInt(string(data), 10, 64)
		wantAccepted := independentErr == nil && independent >= 0 && strconv.FormatInt(independent, 10) == string(data)
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
			if wantAccepted || (!errors.Is(err, core.ErrTemporalContract) || !errors.Is(err, core.ErrJSONContract)) {
				t.Fatalf("UnmarshalJSON(%q) error = %v, want accepted=%t or a stable temporal identity", data, err, wantAccepted)
			}
			if got.Duration().Nanoseconds() != 13 {
				t.Fatalf("receiver after rejection = %d, want 13 unchanged", got.Duration().Nanoseconds())
			}
			return
		}

		if gotErr := got.Validate(); !wantAccepted || gotErr != nil || got.Duration().Nanoseconds() != independent {
			t.Fatalf("UnmarshalJSON(%q) accepted duration = (%d, %v), want accepted=%t value=%d", data, got.Duration().Nanoseconds(), gotErr, wantAccepted, independent)
		}
		encoded, err := got.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON() after accepted decode error = %v, want nil", err)
		}
		if string(encoded) != string(data) {
			t.Fatalf("re-encoded = %s, want the accepted bytes %s", encoded, data)
		}
		var roundTrip temporal.NumericDuration
		if err := roundTrip.UnmarshalJSON(encoded); err != nil {
			t.Fatalf("second UnmarshalJSON(%s) error = %v, want nil", encoded, err)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || roundTrip.Duration() != got.Duration() || string(second) != string(encoded) {
			t.Fatalf("numeric duration closure = (%s, %v, %v), want (%s, nil, %v)", second, err, roundTrip.Duration(), encoded, got.Duration())
		}
	})
}
