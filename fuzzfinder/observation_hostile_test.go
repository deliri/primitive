package fuzzfinder

import (
	json "encoding/json/v2"
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestObservationSchemaLayerTriad(t *testing.T) {
	t.Parallel()

	type caseClass uint8
	const (
		caseValid caseClass = iota + 1
		caseReject
		caseBoundary
	)
	limit := mustRetentionLimit(t, 2)
	first := generatedNameForPosition(t, ArtifactCorpus, 1)
	second := generatedNameForPosition(t, ArtifactCorpus, 2)
	crasherFirst := generatedNameForPosition(t, ArtifactCrasher, 1)
	crasherSecond := generatedNameForPosition(t, ArtifactCrasher, 2)
	valid := Observation{
		limit:    limit,
		retained: 2,
		names:    [MaximumRetainedEntries]GeneratedName{first, second},
		kind:     ArtifactCorpus,
		format:   CacheFormatGo1_27,
		state:    ObservationComplete,
	}
	cases := []struct {
		wantErr     error
		name        string
		observation Observation
		class       caseClass
	}{
		{name: "positive complete canonical observation is valid", class: caseValid, observation: valid},
		{name: "neutral complete empty observation is valid", class: caseValid, observation: Observation{limit: limit, kind: ArtifactCorpus, format: CacheFormatGo1_27, state: ObservationComplete}},
		{name: "neutral partial observation may retain facts", class: caseValid, observation: Observation{limit: limit, retained: 1, names: [MaximumRetainedEntries]GeneratedName{first}, kind: ArtifactCorpus, format: CacheFormatGo1_27, state: ObservationPartial}},
		{name: "neutral partial observation may carry unsupported entries", class: caseValid, observation: Observation{limit: limit, unsupportedRegular: 1, kind: ArtifactCorpus, format: CacheFormatGo1_27, state: ObservationPartial}},
		{name: "unsupported state binds a positive unsupported count", class: caseValid, observation: Observation{limit: limit, unsupportedRegular: 1, kind: ArtifactCorpus, format: CacheFormatGo1_27, state: ObservationUnsupportedFormat}},
		{name: "failed state with no directory facts is valid", class: caseValid, observation: Observation{limit: limit, kind: ArtifactCorpus, format: CacheFormatGo1_27, state: ObservationFailed}},
		{name: "retained count at the exact limit is valid", class: caseValid, observation: Observation{limit: mustRetentionLimit(t, 2), retained: 2, names: [MaximumRetainedEntries]GeneratedName{crasherFirst, crasherSecond}, kind: ArtifactCrasher, format: CacheFormatGo1_27, state: ObservationComplete}},
		{name: "retained count one below the limit is valid", class: caseValid, observation: Observation{limit: limit, retained: 1, names: [MaximumRetainedEntries]GeneratedName{crasherFirst}, kind: ArtifactCrasher, format: CacheFormatGo1_27, state: ObservationComplete}},
		{name: "saturated accounting with a full canonical prefix is valid", class: caseValid, observation: Observation{limit: limit, retained: 2, names: [MaximumRetainedEntries]GeneratedName{first, second}, overLimit: math.MaxUint64, ignoredDirectories: math.MaxUint64, kind: ArtifactCorpus, format: CacheFormatGo1_27, state: ObservationComplete}},
		{name: "crasher partial empty observation is valid", class: caseValid, observation: Observation{limit: limit, kind: ArtifactCrasher, format: CacheFormatGo1_27, state: ObservationPartial}},

		{name: "zero state is rejected", class: caseReject, observation: Observation{limit: limit, kind: ArtifactCorpus, format: CacheFormatGo1_27}, wantErr: core.ErrFuzzFinderContract},
		{name: "future state is rejected", class: caseReject, observation: Observation{limit: limit, kind: ArtifactCorpus, format: CacheFormatGo1_27, state: ObservationState(255)}, wantErr: core.ErrFuzzFinderContract},
		{name: "undeclared artifact kind is rejected", class: caseReject, observation: Observation{limit: limit, format: CacheFormatGo1_27, state: ObservationComplete}, wantErr: core.ErrFuzzFinderContract},
		{name: "future artifact kind is rejected", class: caseReject, observation: Observation{limit: limit, kind: ArtifactKind(255), format: CacheFormatGo1_27, state: ObservationComplete}, wantErr: core.ErrFuzzFinderContract},
		{name: "undeclared format is rejected", class: caseReject, observation: Observation{limit: limit, kind: ArtifactCorpus, state: ObservationComplete}, wantErr: core.ErrFuzzFinderContract},
		{name: "future format is rejected", class: caseReject, observation: Observation{limit: limit, kind: ArtifactCorpus, format: CacheFormat(255), state: ObservationComplete}, wantErr: core.ErrFuzzFinderContract},
		{name: "zero limit is rejected", class: caseReject, observation: Observation{kind: ArtifactCorpus, format: CacheFormatGo1_27, state: ObservationComplete}, wantErr: core.ErrFuzzFinderContract},
		{name: "retained count above request limit is rejected", class: caseReject, observation: Observation{limit: mustRetentionLimit(t, 1), retained: 2, names: [MaximumRetainedEntries]GeneratedName{first, second}, kind: ArtifactCorpus, format: CacheFormatGo1_27, state: ObservationComplete}, wantErr: core.ErrFuzzFinderContract},
		{name: "retained count above shared ceiling is rejected", class: caseReject, observation: Observation{limit: mustRetentionLimit(t, MaximumRetainedEntries), retained: MaximumRetainedEntries + 1, kind: ArtifactCorpus, format: CacheFormatGo1_27, state: ObservationComplete}, wantErr: core.ErrFuzzFinderContract},
		{name: "over-limit with empty retained prefix is rejected", class: caseReject, observation: Observation{limit: limit, overLimit: 1, kind: ArtifactCorpus, format: CacheFormatGo1_27, state: ObservationComplete}, wantErr: core.ErrFuzzFinderContract},

		{name: "boundary over-limit one below full prefix", class: caseBoundary, observation: Observation{limit: limit, retained: 1, names: [MaximumRetainedEntries]GeneratedName{first}, overLimit: 1, kind: ArtifactCorpus, format: CacheFormatGo1_27, state: ObservationComplete}, wantErr: core.ErrFuzzFinderContract},
		{name: "boundary retained zero name", class: caseBoundary, observation: Observation{limit: limit, retained: 1, kind: ArtifactCorpus, format: CacheFormatGo1_27, state: ObservationComplete}, wantErr: core.ErrFuzzFinderContract},
		{name: "boundary retained cross-kind name", class: caseBoundary, observation: Observation{limit: limit, retained: 1, names: [MaximumRetainedEntries]GeneratedName{crasherFirst}, kind: ArtifactCorpus, format: CacheFormatGo1_27, state: ObservationComplete}, wantErr: core.ErrFuzzFinderContract},
		{name: "boundary retained duplicate name", class: caseBoundary, observation: Observation{limit: limit, retained: 2, names: [MaximumRetainedEntries]GeneratedName{first, first}, kind: ArtifactCorpus, format: CacheFormatGo1_27, state: ObservationComplete}, wantErr: core.ErrFuzzFinderContract},
		{name: "boundary retained descending names", class: caseBoundary, observation: Observation{limit: limit, retained: 2, names: [MaximumRetainedEntries]GeneratedName{second, first}, kind: ArtifactCorpus, format: CacheFormatGo1_27, state: ObservationComplete}, wantErr: core.ErrFuzzFinderContract},
		{name: "boundary complete with unsupported regular", class: caseBoundary, observation: Observation{limit: limit, unsupportedRegular: 1, kind: ArtifactCorpus, format: CacheFormatGo1_27, state: ObservationComplete}, wantErr: core.ErrFuzzFinderContract},
		{name: "boundary unsupported without unsupported regular", class: caseBoundary, observation: Observation{limit: limit, kind: ArtifactCorpus, format: CacheFormatGo1_27, state: ObservationUnsupportedFormat}, wantErr: core.ErrFuzzFinderContract},
		{name: "boundary failed with retained name", class: caseBoundary, observation: Observation{limit: limit, retained: 1, names: [MaximumRetainedEntries]GeneratedName{first}, kind: ArtifactCorpus, format: CacheFormatGo1_27, state: ObservationFailed}, wantErr: core.ErrFuzzFinderContract},
		{name: "boundary failed with ignored directory", class: caseBoundary, observation: Observation{limit: limit, ignoredDirectories: 1, kind: ArtifactCorpus, format: CacheFormatGo1_27, state: ObservationFailed}, wantErr: core.ErrFuzzFinderContract},
		{name: "boundary failed with non-regular entry", class: caseBoundary, observation: Observation{limit: limit, nonRegular: 1, kind: ArtifactCorpus, format: CacheFormatGo1_27, state: ObservationFailed}, wantErr: core.ErrFuzzFinderContract},
		{name: "boundary failed with over-limit entry", class: caseBoundary, observation: Observation{limit: limit, overLimit: 1, kind: ArtifactCorpus, format: CacheFormatGo1_27, state: ObservationFailed}, wantErr: core.ErrFuzzFinderContract},
		{name: "boundary failed with unsupported entry", class: caseBoundary, observation: Observation{limit: limit, unsupportedRegular: 1, kind: ArtifactCorpus, format: CacheFormatGo1_27, state: ObservationFailed}, wantErr: core.ErrFuzzFinderContract},
		{name: "boundary unretained hidden name", class: caseBoundary, observation: Observation{limit: limit, names: [MaximumRetainedEntries]GeneratedName{1: first}, kind: ArtifactCorpus, format: CacheFormatGo1_27, state: ObservationComplete}, wantErr: core.ErrFuzzFinderContract},
		{name: "boundary name at storage ceiling", class: caseBoundary, observation: Observation{limit: limit, names: [MaximumRetainedEntries]GeneratedName{MaximumRetainedEntries - 1: first}, kind: ArtifactCorpus, format: CacheFormatGo1_27, state: ObservationComplete}, wantErr: core.ErrFuzzFinderContract},
		{name: "boundary full prefix admits one over-limit", class: caseBoundary, observation: Observation{limit: limit, retained: 2, names: [MaximumRetainedEntries]GeneratedName{first, second}, overLimit: 1, kind: ArtifactCorpus, format: CacheFormatGo1_27, state: ObservationComplete}},
		{name: "boundary unsupported maximum count is valid", class: caseBoundary, observation: Observation{limit: limit, unsupportedRegular: math.MaxUint64, kind: ArtifactCrasher, format: CacheFormatGo1_27, state: ObservationUnsupportedFormat}},
		{name: "boundary partial saturated counters are valid", class: caseBoundary, observation: Observation{limit: limit, ignoredDirectories: math.MaxUint64, nonRegular: math.MaxUint64, unsupportedRegular: math.MaxUint64, kind: ArtifactCorpus, format: CacheFormatGo1_27, state: ObservationPartial}},
		{name: "boundary crasher failed empty is valid", class: caseBoundary, observation: Observation{limit: limit, kind: ArtifactCrasher, format: CacheFormatGo1_27, state: ObservationFailed}},
		{name: "boundary complete maximum non-regular is valid", class: caseBoundary, observation: Observation{limit: limit, nonRegular: math.MaxUint64, kind: ArtifactCorpus, format: CacheFormatGo1_27, state: ObservationComplete}},
		{name: "boundary exact one-entry limit is valid", class: caseBoundary, observation: Observation{limit: mustRetentionLimit(t, 1), retained: 1, names: [MaximumRetainedEntries]GeneratedName{first}, kind: ArtifactCorpus, format: CacheFormatGo1_27, state: ObservationComplete}},
	}
	counts := [4]int{}
	for _, tc := range cases {
		counts[tc.class]++
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.observation.Validate()
			if tc.wantErr == nil {
				if gotErr != nil {
					t.Fatalf("Observation.Validate() error = %v, want nil", gotErr)
				}
				return
			}
			// Every owned contradiction is a contract violation of this type, even
			// the noncanonical-name rows whose cause is a format error: the
			// observation is what failed its own rule, so the identity a caller
			// switches on must not depend on which field was wrong.
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Observation.Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
	if counts[caseValid] != 10 || counts[caseReject] != 10 || counts[caseBoundary] != 20 {
		t.Fatalf("hostile case counts = valid:%d reject:%d boundary:%d, want 10/10/20", counts[caseValid], counts[caseReject], counts[caseBoundary])
	}
}

func TestObservationStateExhaustsClosedDomain(t *testing.T) {
	t.Parallel()

	unknownLabel := ObservationUnknown.String()
	labels := make(map[string]ObservationState, int(observationStateLimit))
	for raw := range 256 {
		state := ObservationState(raw)
		gotErr := state.Validate()
		wantValid := state >= ObservationComplete && state <= ObservationFailed
		if (gotErr == nil) != wantValid || state.IsValid() != wantValid {
			t.Fatalf("ObservationState(%d) validity = Validate:%v IsValid:%t, want %t", raw, gotErr, state.IsValid(), wantValid)
		}
		if !wantValid {
			if !errors.Is(gotErr, core.ErrFuzzFinderContract) {
				t.Fatalf("ObservationState(%d).Validate() error = %v, want %v", raw, gotErr, core.ErrFuzzFinderContract)
			}
			if state.String() != unknownLabel {
				t.Fatalf("ObservationState(%d).String() = %q, want unknown label %q", raw, state.String(), unknownLabel)
			}
			continue
		}
		if label := state.String(); label == "" || label == unknownLabel {
			t.Fatalf("ObservationState(%d).String() = %q, want an admitted diagnostic", raw, label)
		} else if prior, exists := labels[label]; exists {
			t.Fatalf("ObservationState values %d and %d share label %q, want unique labels", prior, state, label)
		} else {
			labels[label] = state
		}
	}
	if _, implemented := any(ObservationComplete).(json.Marshaler); implemented {
		t.Fatalf("%T implements json.Marshaler, want an off-wire enum", ObservationComplete)
	}
	state := ObservationComplete
	if _, implemented := any(&state).(json.Unmarshaler); implemented {
		t.Fatalf("%T implements json.Unmarshaler, want an off-wire enum", &state)
	}
}

func TestObservationNamesAreDefensiveAndCountersSaturate(t *testing.T) {
	t.Parallel()

	limit := mustRetentionLimit(t, 1)
	name := generatedNameForPosition(t, ArtifactCorpus, 1)
	got := Observation{
		limit:              limit,
		retained:           1,
		names:              [MaximumRetainedEntries]GeneratedName{name},
		ignoredDirectories: math.MaxUint64,
		nonRegular:         math.MaxUint64,
		overLimit:          math.MaxUint64,
		unsupportedRegular: math.MaxUint64,
		kind:               ArtifactCorpus,
		format:             CacheFormatGo1_27,
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
