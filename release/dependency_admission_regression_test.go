package release

import (
	"bytes"
	"encoding/base64"
	json "encoding/json/v2"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestDependencyChecksumJSONRefusesAlternateRepresentations(t *testing.T) {
	t.Parallel()
	canonical := goModuleSumPrefix + base64.StdEncoding.EncodeToString(make([]byte, core.SHA256DigestBytes))
	cases := []struct {
		name    string
		sum     string
		wantErr error
	}{
		{name: "canonical zero digest retains every byte", sum: canonical},
		{name: "canonical all bits set retains alphabet endpoints", sum: goModuleSumPrefix + base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{255}, core.SHA256DigestBytes))},
		{name: "low unused pad bit cannot alias zero digest", sum: canonical[:len(canonical)-2] + "B=", wantErr: core.ErrReleaseContract},
		{name: "high unused pad bit cannot alias zero digest", sum: canonical[:len(canonical)-2] + "C=", wantErr: core.ErrReleaseContract},
		{name: "both unused pad bits cannot alias zero digest", sum: canonical[:len(canonical)-2] + "D=", wantErr: core.ErrReleaseContract},
		{name: "line feed inside encoding cannot disappear", sum: canonical[:10] + "\n" + canonical[10:], wantErr: core.ErrReleaseContract},
		{name: "carriage return after padding cannot disappear", sum: canonical + "\r", wantErr: core.ErrReleaseContract},
		{name: "unknown algorithm cannot acquire sha256 identity", sum: "h2:" + canonical[len(goModuleSumPrefix):], wantErr: core.ErrReleaseContract},
		{name: "one encoded byte below exact width refuses", sum: canonical[:len(canonical)-1], wantErr: core.ErrReleaseContract},
		{name: "one encoded byte above exact width refuses", sum: canonical + "=", wantErr: core.ErrReleaseContract},
		{name: "exact width nonalphabet byte refuses", sum: canonical[:10] + "!" + canonical[11:], wantErr: core.ErrReleaseContract},
		{name: "absent checksum cannot create zero digest", wantErr: core.ErrReleaseContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			before, err := newBuildDependencies(mustModulePath(t, testMainModule), CurrentGoToolchain(), moduleFixtures(t, "example.com/original"))
			if err != nil {
				t.Fatalf("newBuildDependencies() error = %v, want nil", err)
			}
			original, err := before.MarshalJSON()
			if err != nil {
				t.Fatalf("BuildDependencies.MarshalJSON() error = %v, want nil", err)
			}
			var wire buildDependenciesWire
			if err := json.Unmarshal(original, &wire); err != nil {
				t.Fatalf("json.Unmarshal(fixture) error = %v, want nil", err)
			}
			wire.Modules[0].Sum = tc.sum
			input, err := json.Marshal(wire)
			if err != nil {
				t.Fatalf("json.Marshal(mutated wire) error = %v, want nil", err)
			}
			got := before
			gotErr := got.UnmarshalJSON(input)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("BuildDependencies.UnmarshalJSON() error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if !errors.Is(gotErr, core.ErrJSONContract) || got != before {
					t.Fatalf("refused dependency document = (%v, %v), want preserved receiver and JSON refusal", got, gotErr)
				}
				preserved, err := got.MarshalJSON()
				if err != nil || !bytes.Equal(preserved, original) {
					t.Fatalf("refused dependency custody = (%q, %v), want original facts %q", preserved, err, original)
				}
				return
			}
			module, ok := got.At(0)
			if err := got.Validate(); err != nil || !ok || got.Count() != 1 || module.Sum().String() != tc.sum || got.MainModule() != before.MainModule() || got.GoToolchain() != before.GoToolchain() {
				t.Fatalf("admitted dependency = (%v, %v, %v), want exact checksum and bound fixture facts", got, module, err)
			}
			encoded, err := got.MarshalJSON()
			if err != nil || !bytes.Equal(encoded, input) {
				t.Fatalf("dependency projection = (%q, %v), want exact admitted bytes %q", encoded, err, input)
			}
		})
	}
}

// Repeated standard-library packages make the count probe independent of the
// distinct-module ceiling. Only the first record contributes a main identity.
func TestDependencyStreamCountIncludesTheExactCeiling(t *testing.T) {
	t.Parallel()
	main := goListPackageWire{ImportPath: testMainModule, Module: &goListModuleWire{Path: testMainModule, Main: true}}
	standard := goListPackageWire{ImportPath: "io", Standard: true}
	first, err := json.Marshal(main)
	if err != nil {
		t.Fatalf("json.Marshal(main) error = %v, want nil", err)
	}
	repeated, err := json.Marshal(standard)
	if err != nil {
		t.Fatalf("json.Marshal(standard) error = %v, want nil", err)
	}
	cases := []struct {
		name    string
		count   int
		suffix  string
		wantErr error
	}{
		{name: "no packages cannot manufacture a main module", wantErr: core.ErrReleaseContract},
		{name: "one main package retains an empty dependency set", count: 1},
		{name: "one below package ceiling admits the complete stream", count: buildPackageObservationMaximumCount - 1},
		{name: "exact package ceiling still permits the EOF probe", count: buildPackageObservationMaximumCount},
		{name: "whitespace after exact ceiling is still EOF", count: buildPackageObservationMaximumCount, suffix: " \n\t"},
		{name: "one package above ceiling refuses all prior facts", count: buildPackageObservationMaximumCount + 1, wantErr: core.ErrReleaseContract},
		{name: "truncated record after ceiling refuses all prior facts", count: buildPackageObservationMaximumCount, suffix: "{", wantErr: core.ErrReleaseContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var body bytes.Buffer
			if tc.count > 0 {
				body.Write(first)
			}
			for range max(0, tc.count-1) {
				body.Write(repeated)
			}
			source := io.MultiReader(&body, strings.NewReader(tc.suffix))
			got := dependencyObservation{main: mustModulePath(t, "example.com/stale"), modules: moduleFixtures(t, "example.com/stale-dependency")}
			gotErr := decodeBuildDependencies(source, &got)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("decodeBuildDependencies(count %d) error = %v, want %v", tc.count, gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got.main != (GoModulePath{}) || got.modules != nil {
					t.Fatalf("refused stream = %v, want exact zero observation", got)
				}
			} else if got.main != mustModulePath(t, testMainModule) || len(got.modules) != 0 {
				t.Fatalf("admitted stream = %v, want exact main identity and no dependencies", got)
			}
		})
	}
}
