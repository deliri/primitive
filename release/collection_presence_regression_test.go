package release

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// Presence has three distinct wire states. An explicit empty set is a fact;
// absence and null must not manufacture the same fact in a closed document.
func TestDependencyDocumentRequiresAnExplicitModuleCollection(t *testing.T) {
	t.Parallel()
	baseline, err := newBuildDependencies(mustModulePath(t, testMainModule), CurrentGoToolchain(), numberedModules(t, 1))
	if err != nil {
		t.Fatalf("newBuildDependencies() error = %v, want nil", err)
	}
	before, err := baseline.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON(baseline) error = %v, want nil", err)
	}
	for _, tc := range []struct {
		name    string
		modules []buildDependencyWire
		options json.Options
		wantErr error
	}{
		{name: "explicit empty module set remains an observed empty set", modules: []buildDependencyWire{}},
		{name: "null module set cannot become observed absence", options: json.FormatNilSliceAsNull(true), wantErr: core.ErrReleaseContract},
		{name: "missing module set cannot become observed absence", options: json.OmitZeroStructFields(true), wantErr: core.ErrReleaseContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var wire buildDependenciesWire
			if err := json.Unmarshal(before, &wire); err != nil {
				t.Fatalf("Go Unmarshal(baseline) error = %v, want nil", err)
			}
			wire.Modules = tc.modules
			data, err := json.Marshal(wire, tc.options)
			if err != nil {
				t.Fatalf("Go Marshal(presence input) error = %v, want nil", err)
			}
			got := baseline
			gotErr := got.UnmarshalJSON(data)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("UnmarshalJSON(module presence) error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				preserved, err := got.MarshalJSON()
				if !errors.Is(gotErr, core.ErrJSONContract) || err != nil || got != baseline || !bytes.Equal(preserved, before) {
					t.Fatalf("refused module presence = (%v, %v), want unchanged facts and typed JSON refusal", got, gotErr)
				}
				return
			}
			encoded, err := got.MarshalJSON()
			if err != nil || got.Count() != 0 || !bytes.Equal(encoded, data) {
				t.Fatalf("explicit empty module document = (%q, %v), want exact input %q", encoded, err, data)
			}
		})
	}
}

func TestBuildProvenanceRequiresExplicitSelectorCollections(t *testing.T) {
	t.Parallel()
	fixture := newReleaseFixture(t, core.NewReleaseVersion(2026, 7, 30), 1)
	baseline := fixture.manifest.Fact.Provenance()
	before, err := baseline.MarshalJSON()
	if err != nil {
		t.Fatalf("BuildProvenance.MarshalJSON(baseline) error = %v, want nil", err)
	}
	for _, tc := range []struct {
		name    string
		mutate  func(*buildProvenanceWire)
		options json.Options
		wantErr error
	}{
		{name: "explicit empty build tags stay empty", mutate: func(w *buildProvenanceWire) { w.BuildTags = []string{} }},
		{name: "null build tags cannot become observed absence", mutate: func(w *buildProvenanceWire) { w.BuildTags = nil }, options: json.FormatNilSliceAsNull(true), wantErr: core.ErrReleaseContract},
		{name: "missing build tags cannot become observed absence", mutate: func(w *buildProvenanceWire) { w.BuildTags = nil }, options: json.OmitZeroStructFields(true), wantErr: core.ErrReleaseContract},
		{name: "explicit empty linker assignments stay empty", mutate: func(w *buildProvenanceWire) { w.LinkerAssignments = []linkerAssignmentWire{} }},
		{name: "null linker assignments cannot become observed absence", mutate: func(w *buildProvenanceWire) { w.LinkerAssignments = nil }, options: json.FormatNilSliceAsNull(true), wantErr: core.ErrReleaseContract},
		{name: "missing linker assignments cannot become observed absence", mutate: func(w *buildProvenanceWire) { w.LinkerAssignments = nil }, options: json.OmitZeroStructFields(true), wantErr: core.ErrReleaseContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var wire buildProvenanceWire
			if err := json.Unmarshal(before, &wire); err != nil {
				t.Fatalf("Go Unmarshal(baseline) error = %v, want nil", err)
			}
			tc.mutate(&wire)
			data, err := json.Marshal(wire, tc.options)
			if err != nil {
				t.Fatalf("Go Marshal(selector presence) error = %v, want nil", err)
			}
			got := baseline
			gotErr := got.UnmarshalJSON(data)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("BuildProvenance.UnmarshalJSON() error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				preserved, err := got.MarshalJSON()
				if !errors.Is(gotErr, core.ErrJSONContract) || err != nil || got != baseline || !bytes.Equal(preserved, before) {
					t.Fatalf("refused selector presence = (%v, %v), want unchanged facts and typed JSON refusal", got, gotErr)
				}
				return
			}
			encoded, err := got.MarshalJSON()
			if err != nil || !bytes.Equal(encoded, data) {
				t.Fatalf("explicit empty selector document = (%q, %v), want exact input %q", encoded, err, data)
			}
		})
	}
}
