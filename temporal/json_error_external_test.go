package temporal_test

import (
	"errors"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// Every row crosses all five real persistence doors. A rejected decimal must
// retain both the JSON boundary and its lower-level reason, without overwriting
// a previously populated receiver.
func TestTemporalJSONRefusalLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, decimal                                                    string
		wantSigned                                                       int64
		wantWide                                                         string
		wantInstantErr, wantDurationErr, wantAggregateErr, wantNumberErr error
	}{
		{name: "zero replaces an existing value", decimal: "0", wantWide: "0"},
		{name: "one preserves the smallest positive value", decimal: "1", wantSigned: 1, wantWide: "1"},
		{name: "negative one is an instant but cannot become elapsed time", decimal: "-1", wantSigned: -1, wantDurationErr: core.ErrTemporalContract, wantAggregateErr: core.ErrTemporalContract},
		{name: "signed minimum remains exact", decimal: "-9223372036854775808", wantSigned: -9223372036854775808, wantDurationErr: core.ErrTemporalContract, wantAggregateErr: core.ErrTemporalContract},
		{name: "signed maximum remains exact", decimal: "9223372036854775807", wantSigned: 9223372036854775807, wantWide: "9223372036854775807"},
		{name: "integer beyond floating point precision remains exact", decimal: "9007199254740993", wantSigned: 9007199254740993, wantWide: "9007199254740993"},
		{name: "signed overflow preserves strconv range identity", decimal: "9223372036854775808", wantWide: "9223372036854775808", wantInstantErr: core.ErrTemporalContract, wantDurationErr: core.ErrTemporalContract, wantNumberErr: strconv.ErrRange},
		{name: "signed underflow preserves strconv range identity", decimal: "-9223372036854775809", wantInstantErr: core.ErrTemporalContract, wantDurationErr: core.ErrTemporalContract, wantAggregateErr: core.ErrTemporalContract, wantNumberErr: strconv.ErrRange},
		{name: "aggregate overflow preserves numeric overflow identity", decimal: "340282366920938463463374607431768211456", wantInstantErr: core.ErrTemporalContract, wantDurationErr: core.ErrTemporalContract, wantAggregateErr: core.ErrTemporalOverflow, wantNumberErr: strconv.ErrRange},
		{name: "missing digits preserve strconv syntax identity", decimal: "", wantInstantErr: core.ErrTemporalContract, wantDurationErr: core.ErrTemporalContract, wantAggregateErr: core.ErrTemporalContract, wantNumberErr: strconv.ErrSyntax},
		{name: "unknown decimal digit preserves syntax identity", decimal: "x", wantInstantErr: core.ErrTemporalContract, wantDurationErr: core.ErrTemporalContract, wantAggregateErr: core.ErrTemporalContract, wantNumberErr: strconv.ErrSyntax},
		{name: "fraction cannot silently round to nanoseconds", decimal: "1.5", wantInstantErr: core.ErrTemporalContract, wantDurationErr: core.ErrTemporalContract, wantAggregateErr: core.ErrTemporalContract, wantNumberErr: strconv.ErrSyntax},
		{name: "exponent cannot become an alternate integer spelling", decimal: "1e3", wantInstantErr: core.ErrTemporalContract, wantDurationErr: core.ErrTemporalContract, wantAggregateErr: core.ErrTemporalContract, wantNumberErr: strconv.ErrSyntax},
		{name: "leading zero refuses a second spelling", decimal: "01", wantInstantErr: core.ErrTemporalContract, wantDurationErr: core.ErrTemporalContract, wantAggregateErr: core.ErrTemporalContract},
		{name: "negative zero refuses a second zero spelling", decimal: "-0", wantInstantErr: core.ErrTemporalContract, wantDurationErr: core.ErrTemporalContract, wantAggregateErr: core.ErrTemporalContract},
		{name: "positive sign refuses a second positive spelling", decimal: "+1", wantInstantErr: core.ErrTemporalContract, wantDurationErr: core.ErrTemporalContract, wantAggregateErr: core.ErrTemporalContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			retainedInstant := temporal.InstantFromNanoseconds(7)
			retainedDuration, err := temporal.DurationFromNanoseconds(7)
			if err != nil {
				t.Fatal(err)
			}
			retainedAggregate := temporal.AggregateDurationFromNanoseconds(7)
			instant, duration, aggregate := retainedInstant, retainedDuration, retainedAggregate
			quoted := []byte(strconv.Quote(tc.decimal))
			instantErr := instant.UnmarshalJSON(quoted)
			durationErr := duration.UnmarshalJSON(quoted)
			aggregateErr := aggregate.UnmarshalJSON(quoted)
			for _, result := range []struct {
				name            string
				gotErr, wantErr error
			}{
				{"instant", instantErr, tc.wantInstantErr},
				{"duration", durationErr, tc.wantDurationErr},
				{"aggregate", aggregateErr, tc.wantAggregateErr},
			} {
				if !errors.Is(result.gotErr, result.wantErr) || (result.wantErr != nil && (!errors.Is(result.gotErr, core.ErrJSONContract) || !errors.Is(result.gotErr, core.ErrTemporalContract))) {
					t.Fatalf("%s decode = %v, want %v with JSON and Temporal identities", result.name, result.gotErr, result.wantErr)
				}
			}
			if tc.wantNumberErr != nil {
				var instantNumber, durationNumber *strconv.NumError
				if !errors.Is(instantErr, tc.wantNumberErr) || !errors.Is(durationErr, tc.wantNumberErr) || !errors.As(instantErr, &instantNumber) || !errors.As(durationErr, &durationNumber) {
					t.Fatalf("numeric causes = (%v,%v), want %v and *strconv.NumError", instantErr, durationErr, tc.wantNumberErr)
				}
			}
			wantInstant, wantDuration, wantAggregate := retainedInstant, retainedDuration, retainedAggregate
			if tc.wantInstantErr == nil {
				wantInstant = temporal.InstantFromNanoseconds(tc.wantSigned)
			}
			if tc.wantDurationErr == nil {
				wantDuration, err = temporal.DurationFromNanoseconds(tc.wantSigned)
				if err != nil {
					t.Fatal(err)
				}
			}
			if tc.wantAggregateErr == nil {
				wantAggregate, err = temporal.ParseAggregateDuration(tc.wantWide)
				if err != nil {
					t.Fatal(err)
				}
			}
			if instant != wantInstant || duration != wantDuration || aggregate != wantAggregate {
				t.Fatalf("decoded facts = (%v,%v,%v), want (%v,%v,%v)", instant, duration, aggregate, wantInstant, wantDuration, wantAggregate)
			}
			numericInstant, err := temporal.NewNumericInstant(retainedInstant)
			if err != nil {
				t.Fatal(err)
			}
			numericDuration, err := temporal.NewNumericDuration(retainedDuration)
			if err != nil {
				t.Fatal(err)
			}
			instantErr = numericInstant.UnmarshalJSON([]byte(tc.decimal))
			durationErr = numericDuration.UnmarshalJSON([]byte(tc.decimal))
			for _, result := range []struct {
				name            string
				gotErr, wantErr error
			}{
				{"numeric instant", instantErr, tc.wantInstantErr},
				{"numeric duration", durationErr, tc.wantDurationErr},
			} {
				if !errors.Is(result.gotErr, result.wantErr) || (result.wantErr != nil && (!errors.Is(result.gotErr, core.ErrJSONContract) || !errors.Is(result.gotErr, core.ErrTemporalContract))) {
					t.Fatalf("%s decode = %v, want %v with JSON and Temporal identities", result.name, result.gotErr, result.wantErr)
				}
			}
			gotInstant, projectionErr := numericInstant.Instant()
			if projectionErr != nil || gotInstant != wantInstant || numericDuration.Duration() != wantDuration {
				t.Fatalf("numeric facts = (%v,%v,%v), want (%v,%v,nil)", gotInstant, numericDuration.Duration(), projectionErr, wantInstant, wantDuration)
			}
		})
	}
}
