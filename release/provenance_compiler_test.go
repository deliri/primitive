package release

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// Compiler provenance records an authenticated build fact. The compiler used
// for a new build is separately pinned by VerifiedBuildTools and BuildPlan.
func TestPublishedCompilerProvenanceLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr error
		name    string
		version string
	}{
		{name: "positive installed build retains its previous patch", version: "go1.27.1"},
		{name: "positive current build retains reviewed compiler", version: "go1.27.2"},
		{name: "positive signed next patch is readable before selector changes", version: "go1.27.3"},
		{name: "negative preview is not stable compiler provenance", version: "go1.27rc1", wantErr: core.ErrJSONContract},
		{name: "neutral absent compiler preserves populated receiver", wantErr: core.ErrJSONContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			baseline := fixtureBuildProvenance(t)
			wire, err := baseline.wire()
			if err != nil {
				t.Fatal(err)
			}
			wire.GoToolchain = tc.version
			data, err := json.Marshal(wire)
			if err != nil {
				t.Fatal(err)
			}
			got := baseline
			err = got.UnmarshalJSON(data)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || got != baseline {
					t.Fatalf("rejected provenance = (%+v, %v), want preserved receiver and %v", got, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("published compiler %q error = %v, want nil", tc.version, err)
			}
			projection, err := got.wire()
			if err != nil || projection.GoToolchain != tc.version {
				t.Fatalf("compiler projection = (%q, %v), want (%q, nil)", projection.GoToolchain, err, tc.version)
			}
			canonical, err := got.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			var second BuildProvenance
			if err := second.UnmarshalJSON(canonical); err != nil || second != got {
				t.Fatalf("round trip = (%+v, %v), want (%+v, nil)", second, err, got)
			}
			reencoded, err := second.MarshalJSON()
			if err != nil || !bytes.Equal(reencoded, canonical) {
				t.Fatalf("second canonical encoding equals first = %t, error = %v, want true and nil", bytes.Equal(reencoded, canonical), err)
			}
		})
	}
}

func TestCompilerVersionCanonicalNativeBoundaries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr error
		name    string
		input   string
		want    GoCompilerVersion
	}{
		{name: "minimum stable components", input: "go1.0.0", want: GoCompilerVersion{major: 1}},
		{name: "historical patch", input: "go1.27.1", want: GoCompilerVersion{major: 1, minor: 27, patch: 1}},
		{name: "current patch", input: "go1.27.2", want: GoCompilerVersion{major: 1, minor: 27, patch: 2}},
		{name: "next patch", input: "go1.27.3", want: GoCompilerVersion{major: 1, minor: 27, patch: 3}},
		{name: "next minor initial patch", input: "go1.28.0", want: GoCompilerVersion{major: 1, minor: 28}},
		{name: "future major evidence", input: "go2.0.0", want: GoCompilerVersion{major: 2}},
		{name: "patch below native maximum", input: "go1.27.4294967294", want: GoCompilerVersion{major: 1, minor: 27, patch: 4294967294}},
		{name: "patch at native maximum", input: "go1.27.4294967295", want: GoCompilerVersion{major: 1, minor: 27, patch: 4294967295}},
		{name: "minor at native maximum", input: "go1.4294967295.0", want: GoCompilerVersion{major: 1, minor: 4294967295}},
		{name: "major at native maximum", input: "go4294967295.0.0", want: GoCompilerVersion{major: 4294967295}},
		{name: "all components at native maximum", input: "go4294967295.4294967295.4294967295", want: GoCompilerVersion{major: 4294967295, minor: 4294967295, patch: 4294967295}},
		{name: "zero major is unset", input: "go0.27.2", wantErr: core.ErrReleaseManifest},
		{name: "missing prefix", input: "1.27.2", wantErr: core.ErrReleaseManifest},
		{name: "foreign prefix", input: "GO1.27.2", wantErr: core.ErrReleaseManifest},
		{name: "missing major separator", input: "go1272", wantErr: core.ErrReleaseManifest},
		{name: "missing patch separator", input: "go1.27", wantErr: core.ErrReleaseManifest},
		{name: "empty major", input: "go.27.2", wantErr: core.ErrReleaseManifest},
		{name: "empty minor", input: "go1..2", wantErr: core.ErrReleaseManifest},
		{name: "empty patch", input: "go1.27.", wantErr: core.ErrReleaseManifest},
		{name: "extra component", input: "go1.27.2.0", wantErr: core.ErrReleaseManifest},
		{name: "major above native maximum", input: "go4294967296.27.2", wantErr: core.ErrReleaseManifest},
		{name: "minor above native maximum", input: "go1.4294967296.2", wantErr: core.ErrReleaseManifest},
		{name: "patch above native maximum", input: "go1.27.4294967296", wantErr: core.ErrReleaseManifest},
		{name: "extent above complete native representation", input: "go4294967295.4294967295.42949672950", wantErr: core.ErrReleaseManifest},
		{name: "major leading zero changes canonical identity", input: "go01.27.2", wantErr: core.ErrReleaseManifest},
		{name: "minor leading zero changes canonical identity", input: "go1.027.2", wantErr: core.ErrReleaseManifest},
		{name: "patch leading zero changes canonical identity", input: "go1.27.02", wantErr: core.ErrReleaseManifest},
		{name: "major signed decimal", input: "go+1.27.2", wantErr: core.ErrReleaseManifest},
		{name: "negative minor", input: "go1.-27.2", wantErr: core.ErrReleaseManifest},
		{name: "patch signed decimal", input: "go1.27.+2", wantErr: core.ErrReleaseManifest},
		{name: "release candidate", input: "go1.27rc1", wantErr: core.ErrReleaseManifest},
		{name: "development compiler", input: "devel go1.27.2", wantErr: core.ErrReleaseManifest},
		{name: "suffixed custom compiler", input: "go1.27.2-custom", wantErr: core.ErrReleaseManifest},
		{name: "leading whitespace", input: " go1.27.2", wantErr: core.ErrReleaseManifest},
		{name: "trailing newline", input: "go1.27.2\n", wantErr: core.ErrReleaseManifest},
		{name: "embedded NUL", input: "go1.27.2\x00", wantErr: core.ErrReleaseManifest},
		{name: "non ASCII digit", input: "go1.27.２", wantErr: core.ErrReleaseManifest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseGoCompilerVersion(tc.input)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || got != (GoCompilerVersion{}) {
					t.Fatalf("parse = (%+v, %v), want zero and %v", got, err, tc.wantErr)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("parse = (%+v, %v), want (%+v, nil)", got, err, tc.want)
			}
			text, err := got.Version()
			if err != nil || text != tc.input {
				t.Fatalf("compiler token = (%q, %v), want (%q, nil)", text, err, tc.input)
			}
		})
	}
}

func TestSignedCompilerProvenanceLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr  error
		name     string
		compiler GoCompilerVersion
		tamper   bool
	}{
		{name: "positive historical compiler remains authenticated", compiler: GoCompilerVersion{major: 1, minor: 27, patch: 1}},
		{name: "positive future patch remains authenticated", compiler: GoCompilerVersion{major: 1, minor: 27, patch: 3}},
		{name: "negative compiler substitution breaks signature", compiler: GoCompilerVersion{major: 1, minor: 27, patch: 1}, tamper: true, wantErr: core.ErrReleaseVerification},
		{name: "neutral missing compiler cannot issue proof", wantErr: core.ErrReleaseManifest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fixture := newReleaseFixture(t, core.NewReleaseVersion(2026, 1, 38), 1)
			fact := fixture.manifest.Fact
			fact.provenance.goToolchain = tc.compiler
			if tc.compiler != (GoCompilerVersion{}) {
				fact = fixtureCompilerManifestFact(t, fact)
			}
			doc, err := IssueManifest(IssueManifestRequest{Fact: fact, Signer: fixture.manifestKey})
			if tc.compiler == (GoCompilerVersion{}) {
				if !errors.Is(err, tc.wantErr) || doc != (ManifestDocument{}) {
					t.Fatalf("issue absent compiler = (%+v, %v), want zero proof and %v", doc, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.tamper {
				doc.Fact.provenance.goToolchain = fixtureGoCompilerVersion(t)
				doc.Fact = fixtureCompilerManifestFact(t, doc.Fact)
			}
			encoded, err := doc.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			var decoded ManifestDocument
			if err := decoded.UnmarshalJSON(encoded); err != nil {
				t.Fatal(err)
			}
			got, err := VerifyManifest(VerifyManifestRequest{Document: decoded, TrustedKeys: fixture.manifestTrust, ExpectedOffering: fact.Offering()})
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || got != (VerifiedManifest{}) {
					t.Fatalf("verify substituted compiler = (%+v, %v), want zero proof and %v", got, err, tc.wantErr)
				}
				return
			}
			if err != nil || got.Provenance().goToolchain != tc.compiler {
				t.Fatalf("verified compiler = (%+v, %v), want (%+v, nil)", got.Provenance().goToolchain, err, tc.compiler)
			}
		})
	}
}

func fixtureCompilerManifestFact(t testing.TB, fact ManifestFact) ManifestFact {
	t.Helper()
	value, err := NewManifestFact(ManifestFactRequest{
		Revision: fact.Revision(), Offering: fact.Offering(), Version: fact.Version(),
		Commit: fact.Commit(), CreatedAt: fact.CreatedAt(), Artifacts: fact.Artifacts(),
		Provenance: fact.Provenance(), Metadata: fact.Metadata(),
	})
	if err != nil {
		t.Fatalf("NewManifestFact(compiler evidence) error = %v, want nil", err)
	}
	return value
}

func TestPublishedDependencyCompilerLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr  error
		name     string
		compiler string
	}{
		{name: "positive prior compiler facts survive patch upgrade", compiler: "go1.27.1"},
		{name: "positive future compiler facts are readable", compiler: "go1.27.3"},
		{name: "negative prerelease is not stable evidence", compiler: "go1.27rc1", wantErr: core.ErrJSONContract},
		{name: "neutral absent compiler preserves prior document", wantErr: core.ErrJSONContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			baseline, err := newBuildDependencies(mustModulePath(t, testMainModule), fixtureGoCompilerVersion(t), numberedModules(t, 1))
			if err != nil {
				t.Fatal(err)
			}
			original, err := baseline.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			var wire buildDependenciesWire
			if err := json.Unmarshal(original, &wire); err != nil {
				t.Fatal(err)
			}
			wire.GoToolchain = tc.compiler
			input, err := json.Marshal(wire)
			if err != nil {
				t.Fatal(err)
			}
			got := baseline
			err = got.UnmarshalJSON(input)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || !errors.Is(err, core.ErrReleaseContract) || got != baseline {
					t.Fatalf("dependency refusal = (%+v, %v), want preserved value and typed refusal", got, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			compiler, err := got.GoToolchain().Version()
			if err != nil || compiler != tc.compiler || got.Count() != baseline.Count() || got.MainModule() != baseline.MainModule() {
				t.Fatalf("compiler custody = (%q, %v, %d), want (%q, nil, %d)", compiler, err, got.Count(), tc.compiler, baseline.Count())
			}
			canonical, err := got.MarshalJSON()
			if err != nil || !bytes.Equal(canonical, input) {
				t.Fatalf("dependency canonical equality = %t, error = %v, want true and nil", bytes.Equal(canonical, input), err)
			}
		})
	}
}

func FuzzPublishedCompilerProvenanceSemanticClosure(f *testing.F) {
	baseline := fixtureBuildProvenance(f)
	for _, compiler := range []GoCompilerVersion{{major: 1, minor: 27, patch: 1}, fixtureGoCompilerVersion(f), {major: 1, minor: 27, patch: 3}} {
		seed := baseline
		seed.goToolchain = compiler
		encoded, err := seed.MarshalJSON()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(encoded)
	}
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		got := baseline
		err := got.UnmarshalJSON(data)
		if err != nil {
			if !errors.Is(err, core.ErrJSONContract) || got != baseline {
				t.Fatalf("refusal = (%+v, %v), want preserved receiver and typed error", got, err)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatal(err)
		}
		var input buildProvenanceWire
		if err := json.Unmarshal(data, &input); err != nil {
			t.Fatal(err)
		}
		version, err := got.goToolchain.Version()
		if err != nil || version != input.GoToolchain {
			t.Fatalf("compiler fact = (%q, %v), want (%q, nil)", version, err, input.GoToolchain)
		}
		canonical, err := got.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		var second BuildProvenance
		if err := second.UnmarshalJSON(canonical); err != nil || second != got {
			t.Fatalf("round trip = (%+v, %v), want (%+v, nil)", second, err, got)
		}
		reencoded, err := second.MarshalJSON()
		if err != nil || !bytes.Equal(reencoded, canonical) {
			t.Fatalf("canonical equality = %t, error = %v, want true and nil", bytes.Equal(reencoded, canonical), err)
		}
	})
}
