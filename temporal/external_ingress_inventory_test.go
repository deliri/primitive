package temporal_test

import (
	"testing"

	"github.com/deliri/primitive/v2026/temporal"
)

// externalIngressFuzzContract binds one public decoder or parser to the fuzz
// target whose mutated input reaches that exact door. The reviewed public
// surface ratchet in package temporal rejects new exported doors, while these
// assignments make deletion or signature drift on either side fail at compile
// time.
type externalIngressFuzzContract[Door any] struct {
	Door Door
	Fuzz func(*testing.F)
}

var (
	_ = externalIngressFuzzContract[func(string) (temporal.Duration, error)]{
		Door: temporal.ParseDuration,
		Fuzz: FuzzParseDurationSemanticClosure,
	}
	_ = externalIngressFuzzContract[func(string) (temporal.AggregateDuration, error)]{
		Door: temporal.ParseAggregateDuration,
		Fuzz: FuzzAggregateDurationCanonicalRoundTrip,
	}
	_ = externalIngressFuzzContract[func(string) (temporal.Instant, error)]{
		Door: temporal.ParseRFC3339,
		Fuzz: FuzzParseRFC3339SemanticClosure,
	}
	_ = externalIngressFuzzContract[func(*temporal.Instant, []byte) error]{
		Door: (*temporal.Instant).UnmarshalJSON,
		Fuzz: FuzzSignedTemporalCanonicalRoundTrip,
	}
	_ = externalIngressFuzzContract[func(*temporal.Duration, []byte) error]{
		Door: (*temporal.Duration).UnmarshalJSON,
		Fuzz: FuzzSignedTemporalCanonicalRoundTrip,
	}
	_ = externalIngressFuzzContract[func(*temporal.AggregateDuration, []byte) error]{
		Door: (*temporal.AggregateDuration).UnmarshalJSON,
		Fuzz: FuzzAggregateDurationJSONSemanticClosure,
	}
	_ = externalIngressFuzzContract[func(*temporal.NumericInstant, []byte) error]{
		Door: (*temporal.NumericInstant).UnmarshalJSON,
		Fuzz: FuzzNumericInstantJSON,
	}
	_ = externalIngressFuzzContract[func(*temporal.NumericDuration, []byte) error]{
		Door: (*temporal.NumericDuration).UnmarshalJSON,
		Fuzz: FuzzNumericDurationJSON,
	}
)
