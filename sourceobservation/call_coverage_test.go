package sourceobservation

import (
	"bytes"
	"errors"
	"github.com/deliri/primitive/v2026/capabilities"
	"github.com/deliri/primitive/v2026/core"
	"math"
	"testing"
)

func TestCallCoverageStateJSONRefusalIdentity(t *testing.T) {
	t.Parallel()
	for _, data := range [][]byte{[]byte(`"future"`), []byte(`""`), []byte(`null`), []byte(`0`), []byte(`"partial`)} {
		got := CallCoverageObserved
		err := got.UnmarshalJSON(data)
		if !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrSourceObservationContract) || got != CallCoverageObserved {
			t.Fatalf("UnmarshalJSON(%q) = (%v,%v), want preserved observed state and both boundary identities", data, got, err)
		}
	}
	var destination *CallCoverageState
	if err := destination.UnmarshalJSON(nil); !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrSourceObservationContract) {
		t.Fatalf("nil receiver error = %v, want JSON and source observation contracts", err)
	}
	for raw := range 256 {
		state := CallCoverageState(raw)
		data, err := state.MarshalJSON()
		if !state.IsValid() {
			if !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrSourceObservationContract) || data != nil {
				t.Fatalf("MarshalJSON(%d) = (%q,%v), want no bytes and both boundary identities", raw, data, err)
			}
			continue
		}
		var got CallCoverageState
		if err != nil {
			t.Fatalf("MarshalJSON(%v) error = %v, want nil", state, err)
		}
		if err := got.UnmarshalJSON(data); err != nil || got != state {
			t.Fatalf("round trip = (%v,%v), want (%v,nil)", got, err, state)
		}
		second, err := got.MarshalJSON()
		if err != nil || !bytes.Equal(second, data) {
			t.Fatalf("canonical bytes = (%q,%v), want (%q,nil)", second, err, data)
		}
	}
}

func TestCallCoverageAccountingLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		wantErr error
		name    string
		fact    capabilities.Classification
		before  CallCoverage
		want    CallCoverage
	}{
		{name: "observed pure call increments only pure", before: CallCoverage{State: CallCoverageObserved}, fact: capabilities.Classification{Disposition: capabilities.StandardSymbolPure}, want: CallCoverage{State: CallCoverageObserved, Pure: 1}, wantErr: nil},
		{name: "effect retains effect count", before: CallCoverage{State: CallCoverageObserved}, fact: capabilities.Classification{Disposition: capabilities.StandardSymbolEffect, Effect: capabilities.EffectTransport}, want: CallCoverage{State: CallCoverageObserved, Effects: 1}, wantErr: nil},
		{name: "context remains incomplete", before: CallCoverage{State: CallCoverageObserved}, fact: capabilities.Classification{Disposition: capabilities.StandardSymbolContextual}, want: CallCoverage{State: CallCoverageObserved, Contextual: 1}, wantErr: nil},
		{name: "unresolved remains incomplete", before: CallCoverage{State: CallCoverageObserved}, fact: capabilities.Classification{Disposition: capabilities.StandardSymbolUnresolved}, want: CallCoverage{State: CallCoverageObserved, Unresolved: 1}, wantErr: nil},
		{name: "unobserved refuses invented evidence", before: CallCoverage{}, fact: capabilities.Classification{Disposition: capabilities.StandardSymbolPure}, want: CallCoverage{}, wantErr: core.ErrSourceObservationConflict},
		{name: "zero classification preserves counters", before: CallCoverage{State: CallCoverageObserved}, fact: capabilities.Classification{}, want: CallCoverage{State: CallCoverageObserved}, wantErr: core.ErrCapabilitiesContract},
		{name: "counter at ceiling refuses wrap", before: CallCoverage{State: CallCoverageObserved, Pure: math.MaxUint64}, fact: capabilities.Classification{Disposition: capabilities.StandardSymbolPure}, want: CallCoverage{State: CallCoverageObserved, Pure: math.MaxUint64}, wantErr: core.ErrSourceObservationConflict},
		{name: "aggregate ceiling refuses another owner", before: CallCoverage{State: CallCoverageObserved, Pure: math.MaxUint64}, fact: capabilities.Classification{Disposition: capabilities.StandardSymbolUnresolved}, want: CallCoverage{State: CallCoverageObserved, Pure: math.MaxUint64}, wantErr: core.ErrSourceObservationConflict},
		{name: "exact ceiling succeeds", before: CallCoverage{State: CallCoverageObserved, Pure: math.MaxUint64 - 1}, fact: capabilities.Classification{Disposition: capabilities.StandardSymbolPure}, want: CallCoverage{State: CallCoverageObserved, Pure: math.MaxUint64}, wantErr: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.before
			err := got.Add(tc.fact)
			if !errors.Is(err, tc.wantErr) || got != tc.want {
				t.Fatalf("Add = (%+v,%v), want (%+v,%v)", got, err, tc.want, tc.wantErr)
			}
		})
	}
	partial := CallCoverage{State: CallCoveragePartial}
	if err := partial.Add(capabilities.Classification{Disposition: capabilities.StandardSymbolPure}); err != nil || partial.Pure != 1 || partial.Complete() {
		t.Fatalf("partial census = (%+v,%v), want retained pure call and incomplete", partial, err)
	}
	if (CallCoverage{}).Complete() {
		t.Fatal("unobserved Complete = true, want false")
	}
	if !(CallCoverage{State: CallCoverageObserved}).Complete() {
		t.Fatal("observed empty Complete = false, want true for empty call scope")
	}
	if (CallCoverage{State: CallCoverageObserved, Unresolved: 1}).Complete() || (CallCoverage{State: CallCoverageObserved, Contextual: 1}).Complete() {
		t.Fatal("incomplete Complete = true, want false")
	}
}
