package fuzzfinder

import (
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestObservationSchemaLayerTriad(t *testing.T) {
	t.Parallel()

	limit := mustRetentionLimit(t, 2)
	first := generatedNameForPosition(t, 1)
	second := generatedNameForPosition(t, 2)
	valid := Observation{
		limit:    limit,
		retained: 2,
		names:    [MaximumRetainedEntries]GeneratedName{first, second},
		kind:     ArtifactCorpus,
		state:    ObservationComplete,
	}
	cases := []struct {
		name        string
		observation Observation
		wantValid   bool
	}{
		{name: "positive complete canonical observation is valid", observation: valid, wantValid: true},
		{name: "neutral complete empty observation is valid", observation: Observation{limit: limit, kind: ArtifactCorpus, state: ObservationComplete}, wantValid: true},
		{name: "neutral partial observation may retain facts", observation: Observation{limit: limit, retained: 1, names: [MaximumRetainedEntries]GeneratedName{first}, kind: ArtifactCorpus, state: ObservationPartial}, wantValid: true},
		{name: "neutral partial observation may carry unsupported entries", observation: Observation{limit: limit, unsupportedRegular: 1, kind: ArtifactCorpus, state: ObservationPartial}, wantValid: true},
		{name: "unsupported state binds a positive unsupported count", observation: Observation{limit: limit, unsupportedRegular: 1, kind: ArtifactCorpus, state: ObservationUnsupportedFormat}, wantValid: true},
		{name: "failed state with no directory facts is valid", observation: Observation{limit: limit, kind: ArtifactCorpus, state: ObservationFailed}, wantValid: true},
		{name: "retained count at the exact limit is valid", observation: Observation{limit: mustRetentionLimit(t, 2), retained: 2, names: [MaximumRetainedEntries]GeneratedName{first, second}, kind: ArtifactCrasher, state: ObservationComplete}, wantValid: true},
		{name: "retained count one below the limit is valid", observation: Observation{limit: limit, retained: 1, names: [MaximumRetainedEntries]GeneratedName{first}, kind: ArtifactCrasher, state: ObservationComplete}, wantValid: true},
		{name: "saturated accounting with a full canonical prefix is valid", observation: Observation{limit: limit, retained: 2, names: [MaximumRetainedEntries]GeneratedName{first, second}, overLimit: math.MaxUint64, ignoredDirectories: math.MaxUint64, kind: ArtifactCorpus, state: ObservationComplete}, wantValid: true},
		{name: "zero state is rejected", observation: Observation{limit: limit, kind: ArtifactCorpus}},
		{name: "future state is rejected", observation: Observation{limit: limit, kind: ArtifactCorpus, state: ObservationState(255)}},
		{name: "undeclared artifact kind is rejected", observation: Observation{limit: limit, state: ObservationComplete}},
		{name: "future artifact kind is rejected", observation: Observation{limit: limit, kind: ArtifactKind(255), state: ObservationComplete}},
		{name: "zero limit is rejected", observation: Observation{kind: ArtifactCorpus, state: ObservationComplete}},
		{name: "retained count above request limit is rejected", observation: Observation{limit: mustRetentionLimit(t, 1), retained: 2, names: [MaximumRetainedEntries]GeneratedName{first, second}, kind: ArtifactCorpus, state: ObservationComplete}},
		{name: "retained count above the shared ceiling is rejected", observation: Observation{limit: mustRetentionLimit(t, MaximumRetainedEntries), retained: MaximumRetainedEntries + 1, kind: ArtifactCorpus, state: ObservationComplete}},
		{name: "one over-limit observation with an empty retained prefix is rejected", observation: Observation{limit: limit, overLimit: 1, kind: ArtifactCorpus, state: ObservationComplete}},
		{name: "one over-limit observation with a one-below-limit retained prefix is rejected", observation: Observation{limit: limit, retained: 1, names: [MaximumRetainedEntries]GeneratedName{first}, overLimit: 1, kind: ArtifactCorpus, state: ObservationComplete}},
		{name: "retained zero name is rejected", observation: Observation{limit: limit, retained: 1, kind: ArtifactCorpus, state: ObservationComplete}},
		{name: "retained duplicate name is rejected", observation: Observation{limit: limit, retained: 2, names: [MaximumRetainedEntries]GeneratedName{first, first}, kind: ArtifactCorpus, state: ObservationComplete}},
		{name: "retained descending names are rejected", observation: Observation{limit: limit, retained: 2, names: [MaximumRetainedEntries]GeneratedName{second, first}, kind: ArtifactCorpus, state: ObservationComplete}},
		{name: "complete state with unsupported regular is rejected", observation: Observation{limit: limit, unsupportedRegular: 1, kind: ArtifactCorpus, state: ObservationComplete}},
		{name: "unsupported state without unsupported regular is rejected", observation: Observation{limit: limit, kind: ArtifactCorpus, state: ObservationUnsupportedFormat}},
		{name: "failed state with retained name is rejected", observation: Observation{limit: limit, retained: 1, names: [MaximumRetainedEntries]GeneratedName{first}, kind: ArtifactCorpus, state: ObservationFailed}},
		{name: "failed state with ignored directory is rejected", observation: Observation{limit: limit, ignoredDirectories: 1, kind: ArtifactCorpus, state: ObservationFailed}},
		{name: "failed state with non-regular entry is rejected", observation: Observation{limit: limit, nonRegular: 1, kind: ArtifactCorpus, state: ObservationFailed}},
		{name: "failed state with over-limit entry is rejected", observation: Observation{limit: limit, overLimit: 1, kind: ArtifactCorpus, state: ObservationFailed}},
		{name: "failed state with unsupported entry is rejected", observation: Observation{limit: limit, unsupportedRegular: 1, kind: ArtifactCorpus, state: ObservationFailed}},
		{name: "unretained hidden name is rejected", observation: Observation{limit: limit, names: [MaximumRetainedEntries]GeneratedName{1: first}, kind: ArtifactCorpus, state: ObservationComplete}},
		{name: "name beyond the retained prefix at the storage ceiling is rejected", observation: Observation{limit: limit, names: [MaximumRetainedEntries]GeneratedName{MaximumRetainedEntries - 1: first}, kind: ArtifactCorpus, state: ObservationComplete}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.observation.Validate()
			if tc.wantValid {
				if gotErr != nil {
					t.Fatalf("Observation.Validate() error = %v, want nil", gotErr)
				}
				return
			}
			// Every owned contradiction is a contract violation of this type, even
			// the noncanonical-name rows whose cause is a format error: the
			// observation is what failed its own rule, so the identity a caller
			// switches on must not depend on which field was wrong.
			if !errors.Is(gotErr, core.ErrFuzzFinderContract) {
				t.Fatalf("Observation.Validate() error = %v, want %v", gotErr, core.ErrFuzzFinderContract)
			}
		})
	}
}

func TestObservationStateExhaustsClosedDomain(t *testing.T) {
	t.Parallel()

	for raw := range 256 {
		state := ObservationState(raw)
		gotErr := state.Validate()
		wantValid := state >= ObservationComplete && state <= ObservationFailed
		if (gotErr == nil) != wantValid || state.IsValid() != wantValid {
			t.Fatalf("ObservationState(%d) validity = Validate:%v IsValid:%t, want %t", raw, gotErr, state.IsValid(), wantValid)
		}
		if !wantValid && !errors.Is(gotErr, core.ErrFuzzFinderContract) {
			t.Fatalf("ObservationState(%d).Validate() error = %v, want %v", raw, gotErr, core.ErrFuzzFinderContract)
		}
	}
}

func TestObservationNamesAreDefensiveAndCountersSaturate(t *testing.T) {
	t.Parallel()

	limit := mustRetentionLimit(t, 1)
	name := generatedNameForPosition(t, 1)
	got := Observation{
		limit:              limit,
		retained:           1,
		names:              [MaximumRetainedEntries]GeneratedName{name},
		ignoredDirectories: math.MaxUint64,
		nonRegular:         math.MaxUint64,
		overLimit:          math.MaxUint64,
		unsupportedRegular: math.MaxUint64,
		kind:               ArtifactCorpus,
		state:              ObservationPartial,
	}
	names := got.Names()
	names[0] = GeneratedName{}
	if got.Names()[0] != name {
		t.Fatalf("Observation.Names() after caller mutation = %q, want %q", got.Names()[0].String(), name.String())
	}
	incrementSaturating(&got.ignoredDirectories)
	incrementSaturating(&got.nonRegular)
	incrementSaturating(&got.overLimit)
	incrementSaturating(&got.unsupportedRegular)
	if got.IgnoredDirectories().Uint64() != math.MaxUint64 ||
		got.NonRegular().Uint64() != math.MaxUint64 ||
		got.OverLimitObservations().Uint64() != math.MaxUint64 ||
		got.UnsupportedRegular().Uint64() != math.MaxUint64 {
		t.Fatalf("saturated counts = %d/%d/%d/%d, want MaxUint64 each", got.IgnoredDirectories().Uint64(), got.NonRegular().Uint64(), got.OverLimitObservations().Uint64(), got.UnsupportedRegular().Uint64())
	}
}
