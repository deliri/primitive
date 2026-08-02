package temporal_test

import (
	"encoding/json"
	"errors"
	"math"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	maximumUint64Decimal  = "18446744073709551615"
	uint64HighLimbDecimal = "18446744073709551616"
	maximumUint128Decimal = "340282366920938463463374607431768211455"
)

func TestAggregateDurationCrossesBothLimbsExactly(t *testing.T) {
	t.Parallel()

	maximumDuration, maximumDurationErr := temporal.DurationFromNanoseconds(math.MaxInt64)
	if maximumDurationErr != nil {
		t.Fatalf("DurationFromNanoseconds(maximum) error = %v, want nil", maximumDurationErr)
	}
	fromDuration, fromDurationErr := temporal.AggregateDurationFromDuration(maximumDuration)
	if fromDurationErr != nil {
		t.Fatalf("AggregateDurationFromDuration(maximum) error = %v, want nil", fromDurationErr)
	}
	viaMethod, viaMethodErr := maximumDuration.Aggregate()
	if viaMethodErr != nil {
		t.Fatalf("Duration.Aggregate() error = %v, want nil", viaMethodErr)
	}
	fromNanoseconds := temporal.AggregateDurationFromNanoseconds(math.MaxUint64)
	if gotErr := fromDuration.Validate(); gotErr != nil ||
		fromDuration.Decimal() != "9223372036854775807" ||
		fromNanoseconds.Decimal() != maximumUint64Decimal ||
		viaMethod != fromDuration {
		t.Fatalf(
			"aggregate constructors = (duration:%q nanoseconds:%q method:%q validate:%v), want exact bounded/max-uint64/bounded/nil",
			fromDuration.Decimal(),
			fromNanoseconds.Decimal(),
			viaMethod.Decimal(),
			gotErr,
		)
	}

	cases := []struct {
		name string
		text string
	}{
		{name: "zero is canonical", text: "0"},
		{name: "one is canonical", text: "1"},
		{name: "JavaScript unsafe integer stays exact", text: "9007199254740993"},
		{name: "maximum low limb stays exact", text: maximumUint64Decimal},
		{name: "first high limb value stays exact", text: uint64HighLimbDecimal},
		{name: "both limbs carry values", text: "18446744073709551617"},
		{name: "maximum unsigned 128 value stays exact", text: maximumUint128Decimal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := temporal.ParseAggregateDuration(tc.text)
			gotWire, gotWireErr := json.Marshal(got)
			wantWire := strconv.Quote(tc.text)
			if gotErr != nil || gotWireErr != nil || got.Decimal() != tc.text ||
				string(gotWire) != wantWire {
				t.Fatalf(
					"aggregate parse/project = (%q, %s, %v, %v), want (%q, %s, nil, nil)",
					got.Decimal(),
					gotWire,
					gotErr,
					gotWireErr,
					tc.text,
					wantWire,
				)
			}
		})
	}

	lowMaximum, _ := temporal.ParseAggregateDuration(maximumUint64Decimal)
	highFloor, _ := temporal.ParseAggregateDuration(uint64HighLimbDecimal)
	if got := lowMaximum.Compare(highFloor); got != core.ComparisonLess {
		t.Fatalf("low-limb maximum Compare(high-limb floor) = %v, want %v", got, core.ComparisonLess)
	}
	if got := highFloor.Compare(lowMaximum); got != core.ComparisonGreater {
		t.Fatalf("high-limb floor Compare(low-limb maximum) = %v, want %v", got, core.ComparisonGreater)
	}
	if got := highFloor.Compare(highFloor); got != core.ComparisonEqual {
		t.Fatalf("high-limb floor Compare(itself) = %v, want %v", got, core.ComparisonEqual)
	}
	lowOne := temporal.AggregateDurationFromNanoseconds(1)
	lowTwo := temporal.AggregateDurationFromNanoseconds(2)
	if got := lowOne.Compare(lowTwo); got != core.ComparisonLess {
		t.Fatalf("low one Compare(low two) = %v, want %v", got, core.ComparisonLess)
	}
	if got := lowTwo.Compare(lowOne); got != core.ComparisonGreater {
		t.Fatalf("low two Compare(low one) = %v, want %v", got, core.ComparisonGreater)
	}
}

func TestAggregateDurationArithmeticAttacksCarryAndOverflow(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr    error
		name       string
		input      string
		add        string
		want       string
		multiplier uint64
	}{
		{name: "low limb plus one carries into high limb", input: maximumUint64Decimal, add: "1", want: uint64HighLimbDecimal},
		{name: "both limbs preserve carry", input: uint64HighLimbDecimal, add: "1", want: "18446744073709551617"},
		{name: "maximum plus zero remains maximum", input: maximumUint128Decimal, add: "0", want: maximumUint128Decimal},
		{name: "maximum plus one overflows", input: maximumUint128Decimal, add: "1", wantErr: core.ErrTemporalOverflow},
		{name: "high limb times maximum fits exactly", input: uint64HighLimbDecimal, multiplier: math.MaxUint64, want: "340282366920938463444927863358058659840"},
		{name: "maximum times zero is neutral", input: maximumUint128Decimal, multiplier: 0, want: "0"},
		{name: "maximum times one remains maximum", input: maximumUint128Decimal, multiplier: 1, want: maximumUint128Decimal},
		{name: "maximum times two overflows", input: maximumUint128Decimal, multiplier: 2, wantErr: core.ErrTemporalOverflow},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			input, inputErr := temporal.ParseAggregateDuration(tc.input)
			if inputErr != nil {
				t.Fatalf("ParseAggregateDuration(%q) error = %v, want nil", tc.input, inputErr)
			}
			var got temporal.AggregateDuration
			var gotErr error
			if tc.add != "" {
				addend, addendErr := temporal.ParseAggregateDuration(tc.add)
				if addendErr != nil {
					t.Fatalf("ParseAggregateDuration(%q) error = %v, want nil", tc.add, addendErr)
				}
				got, gotErr = input.Add(addend)
			} else {
				got, gotErr = input.Multiply(tc.multiplier)
			}
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) ||
					!errors.Is(gotErr, core.ErrNumericOverflow) {
					t.Fatalf("aggregate arithmetic error = %v, want %v and %v", gotErr, tc.wantErr, core.ErrNumericOverflow)
				}
				return
			}
			if gotErr != nil || got.Decimal() != tc.want {
				t.Fatalf("aggregate arithmetic = (%q, %v), want (%q, nil)", got.Decimal(), gotErr, tc.want)
			}
		})
	}
}

func TestAggregateDurationNarrowingBoundary(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		name    string
		text    string
		want    int64
	}{
		{name: "zero narrows to zero", text: "0"},
		{name: "maximum duration narrows exactly", text: "9223372036854775807", want: math.MaxInt64},
		{name: "one beyond duration maximum refuses narrowing", text: "9223372036854775808", wantErr: core.ErrTemporalOverflow},
		{name: "high limb refuses narrowing", text: uint64HighLimbDecimal, wantErr: core.ErrTemporalOverflow},
		{name: "maximum aggregate refuses narrowing", text: maximumUint128Decimal, wantErr: core.ErrTemporalOverflow},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			value, parseErr := temporal.ParseAggregateDuration(tc.text)
			if parseErr != nil {
				t.Fatalf("ParseAggregateDuration(%q) error = %v, want nil", tc.text, parseErr)
			}
			got, gotErr := value.Duration()
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || got != (temporal.Duration{}) {
					t.Fatalf("AggregateDuration.Duration() = (%v, %v), want zero/%v", got, gotErr, tc.wantErr)
				}
				return
			}
			if gotErr != nil || got.Nanoseconds() != tc.want {
				t.Fatalf("AggregateDuration.Duration() = (%d, %v), want (%d, nil)", got.Nanoseconds(), gotErr, tc.want)
			}
		})
	}
}

func TestTemporalPersistenceRejectsMalformedInputWithoutMutation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                  string
		raw                   string
		wantInstantError      bool
		wantDurationError     bool
		wantAggregateError    bool
		wantNativeSyntaxError bool
	}{
		{name: "empty document is native syntax failure", raw: "", wantInstantError: true, wantDurationError: true, wantAggregateError: true, wantNativeSyntaxError: true},
		{name: "trailing document is native syntax failure", raw: `"7" "8"`, wantInstantError: true, wantDurationError: true, wantAggregateError: true, wantNativeSyntaxError: true},
		{name: "empty string violates every decimal domain", raw: `""`, wantInstantError: true, wantDurationError: true, wantAggregateError: true},
		{name: "bare number violates every string projection", raw: `7`, wantInstantError: true, wantDurationError: true, wantAggregateError: true},
		{name: "null violates every required value", raw: `null`, wantInstantError: true, wantDurationError: true, wantAggregateError: true},
		{name: "object violates every string projection", raw: `{}`, wantInstantError: true, wantDurationError: true, wantAggregateError: true},
		{name: "array violates every string projection", raw: `[]`, wantInstantError: true, wantDurationError: true, wantAggregateError: true},
		{name: "leading zero is not canonical", raw: `"07"`, wantInstantError: true, wantDurationError: true, wantAggregateError: true},
		{name: "negative zero is not canonical", raw: `"-0"`, wantInstantError: true, wantDurationError: true, wantAggregateError: true},
		{name: "positive sign is not canonical", raw: `"+7"`, wantInstantError: true, wantDurationError: true, wantAggregateError: true},
		{name: "fraction is not nanoseconds", raw: `"7.0"`, wantInstantError: true, wantDurationError: true, wantAggregateError: true},
		{name: "exponent is not canonical", raw: `"7e0"`, wantInstantError: true, wantDurationError: true, wantAggregateError: true},
		{name: "embedded whitespace is rejected", raw: `" 7"`, wantInstantError: true, wantDurationError: true, wantAggregateError: true},
		{name: "escaped canonical digit is a second spelling", raw: `"\u0037"`, wantInstantError: true, wantDurationError: true, wantAggregateError: true},
		{name: "non ASCII digit is rejected", raw: `"７"`, wantInstantError: true, wantDurationError: true, wantAggregateError: true},
		{name: "signed maximum plus one is only a valid aggregate", raw: `"9223372036854775808"`, wantInstantError: true, wantDurationError: true},
		{name: "negative one is only a valid instant", raw: `"-1"`, wantDurationError: true, wantAggregateError: true},
		{name: "unsigned 128 maximum plus one overflows every domain", raw: `"340282366920938463463374607431768211456"`, wantInstantError: true, wantDurationError: true, wantAggregateError: true},
		{name: "oversized valid string is bounded before parsing", raw: `"` + strings.Repeat("1", temporal.AggregateDurationJSONMaximumBytes) + `"`, wantInstantError: true, wantDurationError: true, wantAggregateError: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			instant := temporal.InstantFromNanoseconds(7)
			duration, _ := temporal.DurationFromNanoseconds(7)
			aggregate, _ := temporal.ParseAggregateDuration("7")

			instantErr := json.Unmarshal([]byte(tc.raw), &instant)
			durationErr := json.Unmarshal([]byte(tc.raw), &duration)
			aggregateErr := json.Unmarshal([]byte(tc.raw), &aggregate)
			if tc.wantNativeSyntaxError {
				var instantSyntax *json.SyntaxError
				var durationSyntax *json.SyntaxError
				var aggregateSyntax *json.SyntaxError
				if !errors.As(instantErr, &instantSyntax) ||
					!errors.As(durationErr, &durationSyntax) ||
					!errors.As(aggregateErr, &aggregateSyntax) {
					t.Fatalf(
						"native syntax errors = (instant:%v duration:%v aggregate:%v), want *json.SyntaxError",
						instantErr,
						durationErr,
						aggregateErr,
					)
				}
			} else if (errors.Is(instantErr, core.ErrTemporalContract) != tc.wantInstantError) ||
				(errors.Is(durationErr, core.ErrTemporalContract) != tc.wantDurationError) ||
				(errors.Is(aggregateErr, core.ErrTemporalContract) != tc.wantAggregateError) {
				t.Fatalf(
					"temporal decode identities = (instant:%v duration:%v aggregate:%v), want (%t, %t, %t)",
					instantErr,
					durationErr,
					aggregateErr,
					tc.wantInstantError,
					tc.wantDurationError,
					tc.wantAggregateError,
				)
			}
			gotInstant, gotInstantErr := instant.Nanoseconds()
			wantInstant := int64(7)
			if !tc.wantInstantError {
				wantInstant, _ = strconv.ParseInt(strings.Trim(tc.raw, `"`), 10, 64)
			}
			wantDuration := int64(7)
			if !tc.wantDurationError {
				wantDuration, _ = strconv.ParseInt(strings.Trim(tc.raw, `"`), 10, 64)
			}
			wantAggregate := "7"
			if !tc.wantAggregateError {
				wantAggregate = strings.Trim(tc.raw, `"`)
			}
			if gotInstantErr != nil || gotInstant != wantInstant ||
				duration.Nanoseconds() != wantDuration || aggregate.Decimal() != wantAggregate {
				t.Fatalf(
					"decode mutation/result = (instant:%d/%v duration:%d aggregate:%q), want %d/nil/%d/%q",
					gotInstant,
					gotInstantErr,
					duration.Nanoseconds(),
					aggregate.Decimal(),
					wantInstant,
					wantDuration,
					wantAggregate,
				)
			}
		})
	}
}

func TestTemporalPersistencePreservesSignedAndWideExtremes(t *testing.T) {
	t.Parallel()

	instantCases := []int64{math.MinInt64, -1, 0, 1, 9_007_199_254_740_993, math.MaxInt64}
	for _, want := range instantCases {
		value := temporal.InstantFromNanoseconds(want)
		wire, marshalErr := json.Marshal(value)
		var got temporal.Instant
		unmarshalErr := json.Unmarshal(wire, &got)
		gotNanoseconds, gotNanosecondsErr := got.Nanoseconds()
		if marshalErr != nil || unmarshalErr != nil || gotNanosecondsErr != nil ||
			gotNanoseconds != want {
			t.Fatalf(
				"Instant JSON round trip %d = (%s, %d, %v, %v, %v)",
				want,
				wire,
				gotNanoseconds,
				marshalErr,
				unmarshalErr,
				gotNanosecondsErr,
			)
		}
	}

	aggregate, parseErr := temporal.ParseAggregateDuration(maximumUint128Decimal)
	wire, marshalErr := json.Marshal(aggregate)
	var got temporal.AggregateDuration
	unmarshalErr := json.Unmarshal(wire, &got)
	if parseErr != nil || marshalErr != nil || unmarshalErr != nil ||
		got.Decimal() != maximumUint128Decimal {
		t.Fatalf(
			"Aggregate JSON maximum round trip = (%q, %s, %v, %v, %v), want exact maximum",
			got.Decimal(),
			wire,
			parseErr,
			marshalErr,
			unmarshalErr,
		)
	}
}

func TestTemporalJSONMethodsEnforceExactDocumentBounds(t *testing.T) {
	t.Parallel()

	cases := []struct {
		decode       func([]byte) error
		name         string
		maximumBytes int
	}{
		{
			name:         "instant",
			maximumBytes: temporal.InstantJSONMaximumBytes,
			decode: func(data []byte) error {
				var value temporal.Instant
				return value.UnmarshalJSON(data)
			},
		},
		{
			name:         "duration",
			maximumBytes: temporal.DurationJSONMaximumBytes,
			decode: func(data []byte) error {
				var value temporal.Duration
				return value.UnmarshalJSON(data)
			},
		},
		{
			name:         "aggregate duration",
			maximumBytes: temporal.AggregateDurationJSONMaximumBytes,
			decode: func(data []byte) error {
				var value temporal.AggregateDuration
				return value.UnmarshalJSON(data)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			const canonicalZero = `"0"`
			exact := []byte(canonicalZero + strings.Repeat(" ", tc.maximumBytes-len(canonicalZero)))
			if gotErr := tc.decode(exact); gotErr != nil {
				t.Fatalf("UnmarshalJSON(exact %d-byte bound) error = %v, want nil", len(exact), gotErr)
			}
			oneAbove := append(slices.Clone(exact), ' ')
			if gotErr := tc.decode(oneAbove); !errors.Is(gotErr, core.ErrJSONContract) ||
				!errors.Is(gotErr, core.ErrTemporalContract) {
				t.Fatalf(
					"UnmarshalJSON(%d bytes) error = %v, want %v and %v",
					len(oneAbove),
					gotErr,
					core.ErrJSONContract,
					core.ErrTemporalContract,
				)
			}
		})
	}
}

func FuzzAggregateDurationCanonicalRoundTrip(f *testing.F) {
	for _, seed := range []string{
		"0",
		"1",
		"9007199254740993",
		maximumUint64Decimal,
		uint64HighLimbDecimal,
		maximumUint128Decimal,
		"",
		"00",
		"-1",
		"340282366920938463463374607431768211456",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		value, err := temporal.ParseAggregateDuration(input)
		if err != nil {
			if !errors.Is(err, core.ErrTemporalContract) {
				t.Fatalf("ParseAggregateDuration(%q) error = %v, want %v", input, err, core.ErrTemporalContract)
			}
			return
		}
		if value.Decimal() != input {
			t.Fatalf("ParseAggregateDuration(%q).Decimal() = %q, want identical canonical input", input, value.Decimal())
		}
		wire, marshalErr := json.Marshal(value)
		var got temporal.AggregateDuration
		unmarshalErr := json.Unmarshal(wire, &got)
		if marshalErr != nil || unmarshalErr != nil || got != value {
			t.Fatalf("aggregate round trip = (%q, %v, %v), want (%q, nil, nil)", got.Decimal(), marshalErr, unmarshalErr, value.Decimal())
		}
	})
}

func FuzzSignedTemporalCanonicalRoundTrip(f *testing.F) {
	for _, seed := range []string{
		strconv.FormatInt(math.MinInt64, 10),
		"-1",
		"0",
		"1",
		"9007199254740993",
		strconv.FormatInt(math.MaxInt64, 10),
		"",
		"00",
		"-0",
		"+1",
		"1e0",
		"９",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		if !utf8.ValidString(input) {
			return
		}
		wire, wireErr := json.Marshal(input)
		if wireErr != nil {
			t.Fatalf("json.Marshal(%q) error = %v, want nil", input, wireErr)
		}
		parsed, parseErr := strconv.ParseInt(input, 10, 64)
		canonical := parseErr == nil && strconv.FormatInt(parsed, 10) == input

		var instant temporal.Instant
		instantErr := json.Unmarshal(wire, &instant)
		if canonical {
			got, gotErr := instant.Nanoseconds()
			if instantErr != nil || gotErr != nil || got != parsed {
				t.Fatalf("Instant decode %q = (%d, %v, %v), want (%d, nil, nil)", input, got, instantErr, gotErr, parsed)
			}
			gotWire, gotWireErr := json.Marshal(instant)
			if gotWireErr != nil || string(gotWire) != string(wire) {
				t.Fatalf("Instant round trip %q = (%s, %v), want (%s, nil)", input, gotWire, gotWireErr, wire)
			}
		} else if !errors.Is(instantErr, core.ErrTemporalContract) {
			t.Fatalf("Instant rejected %q with %v, want %v", input, instantErr, core.ErrTemporalContract)
		}

		var duration temporal.Duration
		durationErr := json.Unmarshal(wire, &duration)
		durationCanonical := canonical && parsed >= 0
		if durationCanonical {
			if durationErr != nil || duration.Nanoseconds() != parsed {
				t.Fatalf("Duration decode %q = (%d, %v), want (%d, nil)", input, duration.Nanoseconds(), durationErr, parsed)
			}
			gotWire, gotWireErr := json.Marshal(duration)
			if gotWireErr != nil || string(gotWire) != string(wire) {
				t.Fatalf("Duration round trip %q = (%s, %v), want (%s, nil)", input, gotWire, gotWireErr, wire)
			}
		} else if !errors.Is(durationErr, core.ErrTemporalContract) {
			t.Fatalf("Duration rejected %q with %v, want %v", input, durationErr, core.ErrTemporalContract)
		}
	})
}
