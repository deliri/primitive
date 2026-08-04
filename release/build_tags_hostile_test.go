package release_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/garble"
	"github.com/deliri/primitive/v2026/release"
)

const buildTagMaximumBytesForTest = 64

// TestParseBuildTagPressuresConstraintWordGrammar attacks the one grammar that
// decides whether a release build compiles the files the operator asked for. A
// tag reaches cmd/go and Garble inside one comma-separated argument, so every
// separator, flag prefix, negation, and encoding hazard must be rejected before
// argv is built rather than silently splitting or inverting a constraint.
func TestParseBuildTagPressuresConstraintWordGrammar(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr error
		name    string
		in      string
	}{
		{name: "witness production constraint is accepted", in: "witness_production"},
		{name: "netgo resolver constraint is accepted", in: "netgo"},
		{name: "osusergo resolver constraint is accepted", in: "osusergo"},
		{name: "single lowercase letter is accepted", in: "a"},
		{name: "single uppercase letter is accepted", in: "A"},
		{name: "single underscore is accepted", in: "_"},
		{name: "underscore initial with body is accepted", in: "_internal"},
		{name: "mixed case body is accepted", in: "WitnessProduction"},
		{name: "digit after initial letter is accepted", in: "go1"},
		{name: "architecture tag beginning with digits is accepted", in: "386"},
		{name: "digit initial custom tag is accepted", in: "1go"},
		{name: "dot initial constraint word is accepted", in: ".netgo"},
		{name: "unicode letter is accepted by Go constraint grammar", in: "nétgo"},
		{name: "unicode digit is accepted by Go constraint grammar", in: "go١"},
		{name: "dotted release constraint is accepted", in: "go1.26"},
		{name: "every admitted class in one tag is accepted", in: "a_B9.z"},
		{name: "exact maximum length is accepted", in: strings.Repeat("t", buildTagMaximumBytesForTest)},
		{name: "one below maximum length is accepted", in: strings.Repeat("t", buildTagMaximumBytesForTest-1)},

		{name: "empty tag is rejected", wantErr: core.ErrReleaseContract},
		{name: "one above maximum length is rejected", in: strings.Repeat("t", buildTagMaximumBytesForTest+1), wantErr: core.ErrReleaseContract},
		{name: "flag prefix initial is rejected", in: "-netgo", wantErr: core.ErrReleaseContract},
		{name: "negation prefix is rejected", in: "!netgo", wantErr: core.ErrReleaseContract},
		{name: "embedded separator is rejected", in: "netgo,osusergo", wantErr: core.ErrReleaseContract},
		{name: "trailing separator is rejected", in: "netgo,", wantErr: core.ErrReleaseContract},
		{name: "embedded space is rejected", in: "netgo osusergo", wantErr: core.ErrReleaseContract},
		{name: "leading space is rejected", in: " netgo", wantErr: core.ErrReleaseContract},
		{name: "embedded assignment is rejected", in: "netgo=1", wantErr: core.ErrReleaseContract},
		{name: "embedded hyphen is rejected", in: "net-go", wantErr: core.ErrReleaseContract},
		{name: "embedded slash is rejected", in: "net/go", wantErr: core.ErrReleaseContract},
		{name: "embedded newline is rejected", in: "netgo\nosusergo", wantErr: core.ErrReleaseContract},
		{name: "embedded NUL is rejected", in: "netgo\x00", wantErr: core.ErrReleaseContract},
		{name: "invalid UTF8 is rejected", in: "netgo\xff", wantErr: core.ErrReleaseContract},
		{name: "build constraint expression is rejected", in: "netgo&&osusergo", wantErr: core.ErrReleaseContract},
		{name: "parenthesised expression is rejected", in: "(netgo)", wantErr: core.ErrReleaseContract},
		{name: "tab separator is rejected", in: "netgo\tosusergo", wantErr: core.ErrReleaseContract},
		{name: "quoted tag is rejected", in: `"netgo"`, wantErr: core.ErrReleaseContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := release.ParseBuildTag(tc.in)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("release.ParseBuildTag(%q) error = %v, want errors.Is(..., %v)", tc.in, gotErr, tc.wantErr)
				}
				if got != (release.BuildTag{}) {
					t.Fatalf("release.ParseBuildTag(%q) = %q, want zero tag on rejection", tc.in, got.String())
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("release.ParseBuildTag(%q) error = %v, want nil", tc.in, gotErr)
			}
			if got.String() != tc.in {
				t.Fatalf("release.ParseBuildTag(%q).String() = %q, want %q", tc.in, got.String(), tc.in)
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("release.BuildTag.Validate() error = %v, want nil", err)
			}
		})
	}
}

// TestBuildTagsLayerTriadCanonicalizesTheConstraintSet proves the set contract
// that the -tags argument depends on: bounded cardinality, distinctness,
// canonical order, and an empty set that requests cmd/go's own default rather
// than an empty constraint list.
func TestBuildTagsLayerTriadCanonicalizesTheConstraintSet(t *testing.T) {
	t.Parallel()

	maximum := make([]string, 0, 16)
	for index := range 16 {
		maximum = append(maximum, "tag"+string(rune('a'+index)))
	}
	cases := []struct {
		wantErr      error
		name         string
		wantArgument string
		in           []string
		wantOrder    []string
	}{
		{
			name: "positive witness production set sorts into one argument",
			in:   []string{"witness_production", "netgo", "osusergo"},
			// The Witness release constraint set is the exact reason build tags
			// are compiler-owned; its canonical argument is pinned here.
			wantOrder:    []string{"netgo", "osusergo", "witness_production"},
			wantArgument: "-tags=netgo,osusergo,witness_production",
		},
		{
			name: "positive already sorted set is unchanged", in: []string{"a", "b", "c"},
			wantOrder: []string{"a", "b", "c"}, wantArgument: "-tags=a,b,c",
		},
		{
			name: "positive reverse sorted set is canonicalized", in: []string{"c", "b", "a"},
			wantOrder: []string{"a", "b", "c"}, wantArgument: "-tags=a,b,c",
		},
		{
			name: "positive single tag set", in: []string{"netgo"},
			wantOrder: []string{"netgo"}, wantArgument: "-tags=netgo",
		},
		{
			name: "positive uppercase sorts before lowercase by exact byte order",
			in:   []string{"beta", "Alpha"},
			// Ordering is byte order, not locale order; the argument must be
			// reproducible on every host.
			wantOrder: []string{"Alpha", "beta"}, wantArgument: "-tags=Alpha,beta",
		},
		{
			name: "positive dotted and underscored tags coexist", in: []string{"go1.26", "_x"},
			wantOrder: []string{"_x", "go1.26"}, wantArgument: "-tags=_x,go1.26",
		},
		{
			name: "positive exact maximum cardinality is accepted", in: maximum,
			wantOrder: slices.Sorted(slices.Values(maximum)),
			wantArgument: "-tags=" + strings.Join(
				slices.Sorted(slices.Values(maximum)), ","),
		},
		{
			name: "positive one below maximum cardinality is accepted", in: maximum[:15],
			wantOrder: slices.Sorted(slices.Values(maximum[:15])),
			wantArgument: "-tags=" + strings.Join(
				slices.Sorted(slices.Values(maximum[:15])), ","),
		},
		{
			name: "positive prefix tag and its extension are distinct",
			in:   []string{"netgoextra", "netgo"},
			// A separator-joined argument must not let one tag absorb another.
			wantOrder: []string{"netgo", "netgoextra"}, wantArgument: "-tags=netgo,netgoextra",
		},
		{
			name: "positive two tags differing only in case are distinct",
			in:   []string{"netgo", "Netgo"},
			// Go build constraints are case sensitive; collapsing them would
			// silently drop a constraint.
			wantOrder: []string{"Netgo", "netgo"}, wantArgument: "-tags=Netgo,netgo",
		},
		{
			name: "neutral empty set requests the cmd/go default and adds no argument",
			// A zero-tag build must produce the exact argv it produced before
			// tags existed, not an empty -tags= constraint list.
			wantOrder: nil, wantArgument: "",
		},
		{
			name: "neutral nil slice is the same contract as an empty slice",
			in:   nil, wantOrder: nil, wantArgument: "",
		},

		{name: "negative one above maximum cardinality", in: append(slices.Clone(maximum), "overflow"), wantErr: core.ErrReleaseContract},
		{name: "negative exact duplicate tag", in: []string{"netgo", "netgo"}, wantErr: core.ErrReleaseContract},
		{name: "negative duplicate after canonical sorting", in: []string{"b", "a", "b"}, wantErr: core.ErrReleaseContract},
		{name: "negative empty member", in: []string{"netgo", ""}, wantErr: core.ErrReleaseContract},
		{name: "negative separator inside a member", in: []string{"netgo,osusergo"}, wantErr: core.ErrReleaseContract},
		{name: "negative flag prefix member", in: []string{"-netgo"}, wantErr: core.ErrReleaseContract},
		{name: "negative negation member", in: []string{"!netgo"}, wantErr: core.ErrReleaseContract},
		{name: "negative oversized member", in: []string{strings.Repeat("t", buildTagMaximumBytesForTest+1)}, wantErr: core.ErrReleaseContract},
		{name: "negative invalid UTF8 member", in: []string{"netgo\xff"}, wantErr: core.ErrReleaseContract},
		{name: "negative space inside a member", in: []string{"netgo osusergo"}, wantErr: core.ErrReleaseContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tags, gotErr := release.NewBuildTags(buildTagsForTest(t, tc.in))
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) {
					t.Fatalf("release.NewBuildTags(%q) error = %v, want errors.Is(..., %v)", tc.in, gotErr, tc.wantErr)
				}
				if tags != (release.BuildTags{}) {
					t.Fatalf("release.NewBuildTags(%q) = %v, want zero set on rejection", tc.in, tags)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("release.NewBuildTags(%q) error = %v, want nil", tc.in, gotErr)
			}
			if err := tags.Validate(); err != nil {
				t.Fatalf("release.BuildTags.Validate() error = %v, want nil", err)
			}
			if got := buildTagStrings(t, tags); !slices.Equal(got, tc.wantOrder) {
				t.Fatalf("release.BuildTags order = %q, want %q", got, tc.wantOrder)
			}
			argument, err := tags.Argument()
			if err != nil {
				t.Fatalf("release.BuildTags.Argument() error = %v, want nil", err)
			}
			if argument != tc.wantArgument {
				t.Fatalf("release.BuildTags.Argument() = %q, want %q", argument, tc.wantArgument)
			}
			if _, ok := tags.At(tags.Count()); ok {
				t.Fatalf("release.BuildTags.At(%d) returned a tag past the admitted count", tags.Count())
			}
			if _, ok := tags.At(-1); ok {
				t.Fatalf("release.BuildTags.At(-1) = (_, %t), want (_, false)", ok)
			}
		})
	}
}

// TestBuildPlanLowersOneTagProjectionToEveryTargetAndObservation is the ratchet
// for the defect class this slice exists to prevent: cmd/go must enumerate the
// same package closure the Garble build compiles. The plan owns the tags, so
// every one of the four target commands must carry the identical -tags argument
// and it must sit on the Go side of the invocation.
func TestBuildPlanLowersOneTagProjectionToEveryTargetAndObservation(t *testing.T) {
	t.Parallel()

	tags, err := release.NewBuildTags(buildTagsForTest(t,
		[]string{"witness_production", "netgo", "osusergo"}))
	if err != nil {
		t.Fatalf("release.NewBuildTags() error = %v, want nil", err)
	}
	request := buildPlanRequestForHostileTest(t)
	request.BuildTags = tags
	plan, err := release.PrepareBuildPlan(request)
	if err != nil {
		t.Fatalf("release.PrepareBuildPlan() error = %v, want nil", err)
	}
	const wantArgument = "-tags=netgo,osusergo,witness_production"
	for index := range 4 {
		command, ok := plan.At(index)
		if !ok {
			t.Fatalf("release.BuildPlan.At(%d) = _, false, want a command", index)
		}
		values, err := command.ArgumentValues()
		if err != nil {
			t.Fatalf("release.BuildCommand.ArgumentValues() error = %v, want nil", err)
		}
		got := slices.Index(values, wantArgument)
		if got < 0 {
			t.Fatalf("target %d arguments = %q, want one %q element", index, values, wantArgument)
		}
		if slices.Contains(values[got+1:], wantArgument) {
			t.Fatalf("target %d arguments = %q, want exactly one %q element", index, values, wantArgument)
		}
		mod := slices.Index(values, "-mod=readonly")
		if mod != got+1 {
			t.Fatalf("target %d arguments = %q, want the tag argument immediately before -mod", index, values)
		}
		if output := slices.Index(values, "-o"); output < mod {
			t.Fatalf("target %d arguments = %q, want the closure selectors before the output flag", index, values)
		}
	}
}

// TestBuildPlanWithoutTagsProducesTheExactUntaggedArgv proves the neutral half
// of the contract: adding the tag surface must not change the argv of a build
// that declares no tags.
func TestBuildPlanWithoutTagsProducesTheExactUntaggedArgv(t *testing.T) {
	t.Parallel()

	plan, err := release.PrepareBuildPlan(buildPlanRequestForHostileTest(t))
	if err != nil {
		t.Fatalf("release.PrepareBuildPlan() error = %v, want nil", err)
	}
	command, ok := plan.At(0)
	if !ok {
		t.Fatal("release.BuildPlan.At(0) = _, false, want a command")
	}
	values, err := command.ArgumentValues()
	if err != nil {
		t.Fatalf("release.BuildCommand.ArgumentValues() error = %v, want nil", err)
	}
	for _, value := range values {
		if strings.HasPrefix(value, "-tags") {
			t.Fatalf("untagged arguments = %q, want no -tags element", values)
		}
	}
	if got := slices.Index(values, "-mod=readonly"); got < 0 {
		t.Fatalf("untagged arguments = %q, want a -mod element", values)
	}
}

// TestBuildProvenanceSealsTheExactTagSet proves the tags survive the signed
// provenance round trip. Reproducing a release requires the constraint set, so
// a provenance document that drops or reorders it is not reproducible.
func TestBuildProvenanceSealsTheExactTagSet(t *testing.T) {
	t.Parallel()

	tools := verifiedBuildToolsForLiveTest(t)
	cases := []struct {
		name string
		want string
		in   []string
	}{
		{name: "positive witness constraint set survives the round trip", in: []string{"witness_production", "netgo", "osusergo"}, want: `"build_tags":["netgo","osusergo","witness_production"]`},
		{name: "positive single constraint survives the round trip", in: []string{"netgo"}, want: `"build_tags":["netgo"]`},
		{name: "neutral empty constraint set is sealed as an empty list", in: nil, want: `"build_tags":[]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tags, err := release.NewBuildTags(buildTagsForTest(t, tc.in))
			if err != nil {
				t.Fatalf("release.NewBuildTags() error = %v, want nil", err)
			}
			request := buildPlanRequestForHostileTest(t)
			request.BuildTags = tags
			plan, err := release.PrepareBuildPlan(request)
			if err != nil {
				t.Fatalf("release.PrepareBuildPlan() error = %v, want nil", err)
			}
			provenance, err := release.NewBuildProvenance(release.BuildProvenanceRequest{
				Tools:                tools,
				Plan:                 plan,
				DerivationGeneration: garble.CurrentDerivationGeneration(),
			})
			if err != nil {
				t.Fatalf("release.NewBuildProvenance() error = %v, want nil", err)
			}
			encoded, err := provenance.MarshalJSON()
			if err != nil {
				t.Fatalf("release.BuildProvenance.MarshalJSON() error = %v, want nil", err)
			}
			if !strings.Contains(string(encoded), tc.want) {
				t.Fatalf("provenance document = %s, want it to contain %s", encoded, tc.want)
			}
			var decoded release.BuildProvenance
			if err := decoded.UnmarshalJSON(encoded); err != nil {
				t.Fatalf("release.BuildProvenance.UnmarshalJSON() error = %v, want nil", err)
			}
			second, err := decoded.MarshalJSON()
			if err != nil {
				t.Fatalf("re-encoded release.BuildProvenance error = %v, want nil", err)
			}
			if string(second) != string(encoded) {
				t.Fatalf("re-encoded provenance = %s, want %s", second, encoded)
			}
		})
	}
}

// TestBuildProvenanceRejectsNoncanonicalSelectorDocuments proves the decode side
// refuses a document whose selector sets are reordered or duplicated. Silently
// canonicalizing them would let two different signed byte sequences project to
// one provenance value.
func TestBuildProvenanceRejectsNoncanonicalSelectorDocuments(t *testing.T) {
	t.Parallel()

	tags, err := release.NewBuildTags(buildTagsForTest(t, []string{"netgo", "osusergo"}))
	if err != nil {
		t.Fatalf("release.NewBuildTags() error = %v, want nil", err)
	}
	request := buildPlanRequestForHostileTest(t)
	request.BuildTags = tags
	plan, err := release.PrepareBuildPlan(request)
	if err != nil {
		t.Fatalf("release.PrepareBuildPlan() error = %v, want nil", err)
	}
	provenance, err := release.NewBuildProvenance(release.BuildProvenanceRequest{
		Tools:                verifiedBuildToolsForLiveTest(t),
		Plan:                 plan,
		DerivationGeneration: garble.CurrentDerivationGeneration(),
	})
	if err != nil {
		t.Fatalf("release.NewBuildProvenance() error = %v, want nil", err)
	}
	encoded, err := provenance.MarshalJSON()
	if err != nil {
		t.Fatalf("release.BuildProvenance.MarshalJSON() error = %v, want nil", err)
	}
	cases := []struct {
		name string
		from string
		to   string
	}{
		{name: "reversed tag order is rejected", from: `["netgo","osusergo"]`, to: `["osusergo","netgo"]`},
		{name: "duplicated tag is rejected", from: `["netgo","osusergo"]`, to: `["netgo","netgo"]`},
		{name: "unparsable tag is rejected", from: `["netgo","osusergo"]`, to: `["netgo","!osusergo"]`},
		{name: "separator smuggled into one tag is rejected", from: `["netgo","osusergo"]`, to: `["netgo,osusergo"]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mutated := strings.Replace(string(encoded), tc.from, tc.to, 1)
			if mutated == string(encoded) {
				t.Fatalf("fixture document does not contain %s; the test would pass vacuously", tc.from)
			}
			var decoded release.BuildProvenance
			err := decoded.UnmarshalJSON([]byte(mutated))
			if !errors.Is(err, core.ErrReleaseContract) && !errors.Is(err, core.ErrReleaseManifest) {
				t.Fatalf("release.BuildProvenance.UnmarshalJSON() error = %v, want a typed release rejection", err)
			}
			if decoded != (release.BuildProvenance{}) {
				t.Fatalf("release.BuildProvenance.UnmarshalJSON() receiver = %v, want zero after rejection", decoded)
			}
		})
	}
}

func buildTagsForTest(t *testing.T, values []string) []release.BuildTag {
	t.Helper()

	if values == nil {
		return nil
	}
	tags := make([]release.BuildTag, 0, len(values))
	for _, value := range values {
		tag, err := release.ParseBuildTag(value)
		if err != nil {
			// Rejection tables intentionally supply unparsable members; the set
			// constructor must reject the zero tag rather than the parse step
			// hiding the case.
			tags = append(tags, release.BuildTag{})
			continue
		}
		tags = append(tags, tag)
	}
	return tags
}

func buildTagStrings(t *testing.T, tags release.BuildTags) []string {
	t.Helper()

	if tags.Count() == 0 {
		return nil
	}
	values := make([]string, 0, tags.Count())
	for index := range tags.Count() {
		tag, ok := tags.At(index)
		if !ok {
			t.Fatalf("release.BuildTags.At(%d) = _, false, want a tag below the admitted count", index)
		}
		values = append(values, tag.String())
	}
	return values
}
