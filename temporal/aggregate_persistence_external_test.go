package temporal_test

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"math"
	"math/big"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"encoding/json/jsontext"

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
				var instantSyntax *jsontext.SyntacticError
				var durationSyntax *jsontext.SyntacticError
				var aggregateSyntax *jsontext.SyntacticError
				if !errors.As(instantErr, &instantSyntax) ||
					!errors.As(durationErr, &durationSyntax) ||
					!errors.As(aggregateErr, &aggregateSyntax) {
					t.Fatalf(
						"native syntax errors = (instant:%v duration:%v aggregate:%v), want *jsontext.SyntacticError",
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

	instantCases := []struct {
		name string
		want int64
	}{
		{name: "minimum signed instant", want: math.MinInt64},
		{name: "one nanosecond before epoch", want: -1},
		{name: "neutral epoch", want: 0},
		{name: "one nanosecond after epoch", want: 1},
		{name: "first integer beyond JavaScript exact range", want: 9_007_199_254_740_993},
		{name: "maximum signed instant", want: math.MaxInt64},
	}
	for _, tc := range instantCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			value := temporal.InstantFromNanoseconds(tc.want)
			wire, marshalErr := json.Marshal(value)
			var got temporal.Instant
			unmarshalErr := json.Unmarshal(wire, &got)
			gotNanoseconds, gotNanosecondsErr := got.Nanoseconds()
			if marshalErr != nil || unmarshalErr != nil || gotNanosecondsErr != nil ||
				gotNanoseconds != tc.want {
				t.Fatalf(
					"Instant JSON round trip = (%s, %d, %v, %v, %v), want exact %d with nil errors",
					wire,
					gotNanoseconds,
					marshalErr,
					unmarshalErr,
					gotNanosecondsErr,
					tc.want,
				)
			}
		})
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

func TestTemporalStringJSONNilReceiversRejectBeforeMutation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		decode func([]byte) error
		name   string
	}{
		{
			name: "nil instant receiver",
			decode: func(data []byte) error {
				var value *temporal.Instant
				return value.UnmarshalJSON(data)
			},
		},
		{
			name: "nil duration receiver",
			decode: func(data []byte) error {
				var value *temporal.Duration
				return value.UnmarshalJSON(data)
			},
		},
		{
			name: "nil aggregate duration receiver",
			decode: func(data []byte) error {
				var value *temporal.AggregateDuration
				return value.UnmarshalJSON(data)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.decode([]byte(`"0"`))
			if !errors.Is(gotErr, core.ErrJSONContract) || !errors.Is(gotErr, core.ErrTemporalContract) {
				t.Fatalf("UnmarshalJSON(nil receiver) error = %v, want %v and %v", gotErr, core.ErrJSONContract, core.ErrTemporalContract)
			}
		})
	}
}

func FuzzAggregateDurationCanonicalRoundTrip(f *testing.F) {
	one := temporal.AggregateDurationFromNanoseconds(1)
	lowMaximum := temporal.AggregateDurationFromNanoseconds(math.MaxUint64)
	highFloor, highFloorErr := lowMaximum.Add(one)
	if highFloorErr != nil {
		f.Fatalf("low maximum plus one seed error = %v, want nil", highFloorErr)
	}
	maximumHigh, maximumHighErr := highFloor.Multiply(math.MaxUint64)
	if maximumHighErr != nil {
		f.Fatalf("high floor times maximum seed error = %v, want nil", maximumHighErr)
	}
	maximum, maximumErr := maximumHigh.Add(lowMaximum)
	if maximumErr != nil {
		f.Fatalf("unsigned 128 maximum seed error = %v, want nil", maximumErr)
	}
	for _, value := range []temporal.AggregateDuration{
		temporal.AggregateDurationFromNanoseconds(0),
		one,
		temporal.AggregateDurationFromNanoseconds(9_007_199_254_740_993),
		lowMaximum,
		highFloor,
		maximum,
	} {
		f.Add(value.Decimal())
	}
	for _, malformed := range []string{"", "00", "-1", "340282366920938463463374607431768211456"} {
		f.Add(malformed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > temporal.AggregateDurationMaximumDecimalDigits {
			value, err := temporal.ParseAggregateDuration(input)
			if !errors.Is(err, core.ErrTemporalContract) || !value.IsZero() {
				t.Fatalf("ParseAggregateDuration(%d-byte oversized input) = (%q, %v), want zero/%v", len(input), value.Decimal(), err, core.ErrTemporalContract)
			}
			return
		}
		independent, independentOK := new(big.Int).SetString(input, 10)
		wantAccepted := independentOK && independent.Sign() >= 0 && independent.BitLen() <= 128 && independent.String() == input

		value, err := temporal.ParseAggregateDuration(input)
		if err != nil {
			if wantAccepted ||
				(!errors.Is(err, core.ErrTemporalContract) && !errors.Is(err, core.ErrTemporalOverflow)) ||
				!value.IsZero() {
				t.Fatalf(
					"ParseAggregateDuration(%q) = (%q, %v), want zero/typed refusal with independent accepted=%t",
					input,
					value.Decimal(),
					err,
					wantAccepted,
				)
			}
			return
		}
		if !wantAccepted || value.Decimal() != independent.String() {
			t.Fatalf(
				"ParseAggregateDuration(%q) = %q, want independent accepted=%t and decimal %q",
				input,
				value.Decimal(),
				wantAccepted,
				independent.String(),
			)
		}
		if gotErr := value.Validate(); gotErr != nil {
			t.Fatalf("ParseAggregateDuration(%q).Validate() error = %v, want nil", input, gotErr)
		}
		wire, marshalErr := json.Marshal(value)
		var got temporal.AggregateDuration
		unmarshalErr := json.Unmarshal(wire, &got)
		secondWire, secondMarshalErr := json.Marshal(got)
		if marshalErr != nil || unmarshalErr != nil || secondMarshalErr != nil || got != value || string(secondWire) != string(wire) {
			t.Fatalf(
				"aggregate round trip = (%q, %v, %v, %v, %s), want (%q, nil, nil, nil, %s)",
				got.Decimal(),
				marshalErr,
				unmarshalErr,
				secondMarshalErr,
				secondWire,
				value.Decimal(),
				wire,
			)
		}
	})
}

func FuzzSignedTemporalCanonicalRoundTrip(f *testing.F) {
	for _, nanoseconds := range []int64{math.MinInt64, -1, 0, 1, 9_007_199_254_740_993, math.MaxInt64} {
		wire, err := json.Marshal(temporal.InstantFromNanoseconds(nanoseconds))
		if err != nil {
			f.Fatalf("Instant.MarshalJSON(%d) seed error = %v, want nil", nanoseconds, err)
		}
		f.Add(wire)
	}
	for _, nanoseconds := range []int64{0, 1, 9_007_199_254_740_993, math.MaxInt64} {
		value, err := temporal.DurationFromNanoseconds(nanoseconds)
		if err != nil {
			f.Fatalf("DurationFromNanoseconds(%d) seed error = %v, want nil", nanoseconds, err)
		}
		wire, err := json.Marshal(value)
		if err != nil {
			f.Fatalf("Duration.MarshalJSON(%d) seed error = %v, want nil", nanoseconds, err)
		}
		f.Add(wire)
	}
	for _, malformed := range []string{"", `"`, `"00"`, `"-0"`, `"+1"`, `"1e0"`, `"９"`, `null`, `0`, `[]`, `{} `} {
		f.Add([]byte(malformed))
	}
	f.Fuzz(func(t *testing.T, wire []byte) {
		var decimal string
		var decodeErr error
		canonicalWire := false
		if len(wire) > 0 && len(wire) <= temporal.InstantJSONMaximumBytes {
			decodeErr = json.Unmarshal(wire, &decimal)
		}
		if decodeErr == nil && len(wire) > 0 && len(wire) <= temporal.InstantJSONMaximumBytes && utf8.ValidString(decimal) {
			projected, projectErr := json.Marshal(decimal)
			canonicalWire = projectErr == nil && bytes.Equal(bytes.TrimSpace(wire), projected)
		}
		parsed, parseErr := strconv.ParseInt(decimal, 10, 64)
		canonicalDecimal := parseErr == nil && strconv.FormatInt(parsed, 10) == decimal

		retainedInstant := temporal.InstantFromNanoseconds(7)
		instant := retainedInstant
		instantErr := instant.UnmarshalJSON(wire)
		wantInstantAccepted := len(wire) > 0 && len(wire) <= temporal.InstantJSONMaximumBytes && canonicalWire && canonicalDecimal
		if instantErr != nil {
			retained, retainedErr := instant.Nanoseconds()
			if wantInstantAccepted ||
				(!errors.Is(instantErr, core.ErrTemporalContract) && !errors.Is(instantErr, core.ErrTemporalOverflow)) ||
				retainedErr != nil || retained != 7 {
				t.Fatalf("Instant.UnmarshalJSON(%q) = (retained:%d/%v error:%v), want accepted=%t or retained 7/typed refusal", wire, retained, retainedErr, instantErr, wantInstantAccepted)
			}
		} else {
			got, gotErr := instant.Nanoseconds()
			gotWire, gotWireErr := instant.MarshalJSON()
			var roundTrip temporal.Instant
			roundTripErr := roundTrip.UnmarshalJSON(gotWire)
			secondWire, secondWireErr := roundTrip.MarshalJSON()
			if !wantInstantAccepted || gotErr != nil || got != parsed || gotWireErr != nil ||
				roundTripErr != nil || roundTrip != instant || secondWireErr != nil || !bytes.Equal(secondWire, gotWire) {
				t.Fatalf("Instant.UnmarshalJSON(%q) closure = (value:%d errors:%v/%v/%v/%v wire:%s/%s), want accepted=%t value=%d stable", wire, got, gotErr, gotWireErr, roundTripErr, secondWireErr, gotWire, secondWire, wantInstantAccepted, parsed)
			}
		}

		retainedDuration, retainedDurationErr := temporal.DurationFromNanoseconds(7)
		if retainedDurationErr != nil {
			t.Fatalf("DurationFromNanoseconds(retained) error = %v, want nil", retainedDurationErr)
		}
		duration := retainedDuration
		durationErr := duration.UnmarshalJSON(wire)
		wantDurationAccepted := len(wire) > 0 && len(wire) <= temporal.DurationJSONMaximumBytes && canonicalWire && canonicalDecimal && parsed >= 0
		if durationErr != nil {
			if wantDurationAccepted ||
				(!errors.Is(durationErr, core.ErrTemporalContract) && !errors.Is(durationErr, core.ErrTemporalOverflow)) ||
				duration != retainedDuration {
				t.Fatalf("Duration.UnmarshalJSON(%q) = (retained:%v error:%v), want accepted=%t or retained %v/typed refusal", wire, duration, durationErr, wantDurationAccepted, retainedDuration)
			}
			return
		}
		gotWire, gotWireErr := duration.MarshalJSON()
		var roundTrip temporal.Duration
		roundTripErr := roundTrip.UnmarshalJSON(gotWire)
		secondWire, secondWireErr := roundTrip.MarshalJSON()
		if !wantDurationAccepted || duration.Nanoseconds() != parsed || gotWireErr != nil ||
			roundTripErr != nil || roundTrip != duration || secondWireErr != nil || !bytes.Equal(secondWire, gotWire) {
			t.Fatalf("Duration.UnmarshalJSON(%q) closure = (value:%d errors:%v/%v/%v wire:%s/%s), want accepted=%t value=%d stable", wire, duration.Nanoseconds(), gotWireErr, roundTripErr, secondWireErr, gotWire, secondWire, wantDurationAccepted, parsed)
		}
	})
}

func FuzzAggregateDurationJSONSemanticClosure(f *testing.F) {
	maximum, maximumErr := temporal.ParseAggregateDuration(maximumUint128Decimal)
	if maximumErr != nil {
		f.Fatalf("ParseAggregateDuration(maximum seed) error = %v, want nil", maximumErr)
	}
	for _, value := range []temporal.AggregateDuration{
		temporal.AggregateDurationFromNanoseconds(0),
		temporal.AggregateDurationFromNanoseconds(1),
		temporal.AggregateDurationFromNanoseconds(math.MaxUint64),
		maximum,
	} {
		wire, err := value.MarshalJSON()
		if err != nil {
			f.Fatalf("AggregateDuration.MarshalJSON(%q) seed error = %v, want nil", value.Decimal(), err)
		}
		f.Add(wire)
	}
	for _, malformed := range []string{"", `"`, `"00"`, `"-1"`, `"340282366920938463463374607431768211456"`, `null`, `0`, `[]`, `{}`} {
		f.Add([]byte(malformed))
	}

	f.Fuzz(func(t *testing.T, wire []byte) {
		retained := temporal.AggregateDurationFromNanoseconds(7)
		got := retained

		if len(wire) > temporal.AggregateDurationJSONMaximumBytes {
			gotErr := got.UnmarshalJSON(wire)
			if !errors.Is(gotErr, core.ErrJSONContract) || !errors.Is(gotErr, core.ErrTemporalContract) || got != retained {
				t.Fatalf("AggregateDuration.UnmarshalJSON(%d-byte oversized input) = (%q, %v), want retained %q and %v/%v", len(wire), got.Decimal(), gotErr, retained.Decimal(), core.ErrJSONContract, core.ErrTemporalContract)
			}
			return
		}

		var decimal string
		decodeErr := json.Unmarshal(wire, &decimal)
		canonicalWire := false
		if decodeErr == nil {
			projected, projectErr := json.Marshal(decimal)
			canonicalWire = projectErr == nil && bytes.Equal(bytes.TrimSpace(wire), projected)
		}
		var independent *big.Int
		independentOK := false
		if len(decimal) <= temporal.AggregateDurationMaximumDecimalDigits {
			independent, independentOK = new(big.Int).SetString(decimal, 10)
		}
		wantAccepted := len(wire) > 0 && canonicalWire && independentOK && independent.Sign() >= 0 && independent.BitLen() <= 128 && independent.String() == decimal

		gotErr := got.UnmarshalJSON(wire)
		if gotErr != nil {
			if wantAccepted ||
				(!errors.Is(gotErr, core.ErrTemporalContract) && !errors.Is(gotErr, core.ErrTemporalOverflow)) ||
				got != retained {
				t.Fatalf("AggregateDuration.UnmarshalJSON(%q) = (retained:%q error:%v), want accepted=%t or retained %q/typed refusal", wire, got.Decimal(), gotErr, wantAccepted, retained.Decimal())
			}
			return
		}

		gotWire, gotWireErr := got.MarshalJSON()
		var roundTrip temporal.AggregateDuration
		roundTripErr := roundTrip.UnmarshalJSON(gotWire)
		secondWire, secondWireErr := roundTrip.MarshalJSON()
		if !wantAccepted || got.Validate() != nil || got.Decimal() != independent.String() || gotWireErr != nil ||
			roundTripErr != nil || roundTrip != got || secondWireErr != nil || !bytes.Equal(secondWire, gotWire) {
			t.Fatalf("AggregateDuration.UnmarshalJSON(%q) closure = (value:%q errors:%v/%v/%v wire:%s/%s), want accepted=%t value=%q stable", wire, got.Decimal(), gotWireErr, roundTripErr, secondWireErr, gotWire, secondWire, wantAccepted, independent.String())
		}
	})
}
