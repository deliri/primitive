package reviewcontrol

import (
	json "encoding/json/v2"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type closedReviewEnum interface {
	~uint8
	core.ValidatedJSONMarshaler
	IsValid() bool
	String() string
}

func proveReviewEnumDomain[E closedReviewEnum](t *testing.T, labels []string) {
	t.Helper()
	for raw := 0; raw <= 255; raw++ {
		got := E(raw)
		wantValid := raw > 0 && raw < len(labels) && labels[raw] != ""
		gotErr := got.Validate()
		if got.IsValid() != wantValid || (gotErr == nil) != wantValid {
			t.Fatalf("enum(%d) = (valid=%t, error=%v), want valid=%t", raw, got.IsValid(), gotErr, wantValid)
		}
		if !wantValid {
			if got.String() != "" || !errors.Is(gotErr, core.ErrReviewControlContract) {
				t.Fatalf("enum(%d) = (text=%q, error=%v), want empty and %v", raw, got.String(), gotErr, core.ErrReviewControlContract)
			}
			continue
		}
		if got.String() != labels[raw] {
			t.Fatalf("enum(%d).String() = %q, want %q", raw, got.String(), labels[raw])
		}
		encoded, err := got.MarshalJSON()
		if err != nil {
			t.Fatalf("enum(%d).MarshalJSON() error = %v, want nil", raw, err)
		}
		var roundTrip E
		if err := json.Unmarshal(encoded, &roundTrip); err != nil || roundTrip != got {
			t.Fatalf("enum(%d) round trip = (%d, %v), want (%d, nil)", raw, roundTrip, err, got)
		}
	}
}

func TestReviewControlEnumsExhaustTheirCompleteUint8Domains(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "reviewer provenance", run: func(t *testing.T) { proveReviewEnumDomain[ReviewerKind](t, reviewerLabels()) }},
		{name: "advisory verdict", run: func(t *testing.T) { proveReviewEnumDomain[Verdict](t, verdictLabels()) }},
		{name: "finding severity", run: func(t *testing.T) { proveReviewEnumDomain[FindingSeverity](t, severityLabels()) }},
		{name: "required check", run: func(t *testing.T) { proveReviewEnumDomain[CheckKind](t, checkLabels()) }},
		{name: "required proof", run: func(t *testing.T) { proveReviewEnumDomain[ProofKind](t, proofLabels()) }},
		{name: "human decision", run: func(t *testing.T) { proveReviewEnumDomain[DecisionKind](t, decisionLabels()) }},
		{name: "review event", run: func(t *testing.T) { proveReviewEnumDomain[EventKind](t, eventLabels()) }},
		{name: "authority provenance", run: func(t *testing.T) { proveReviewEnumDomain[AuthorityKind](t, authorityLabels()) }},
		{name: "review operation", run: func(t *testing.T) {
			proveReviewEnumDomain[Operation](t, []string{"", "issue_review", "read_review", "record_observation", "record_decision", "read_events", "read_projection"})
		}},
		{name: "human authority signing domain", run: func(t *testing.T) {
			proveReviewEnumDomain[HumanAuthoritySigningDomain](t, []string{"", HumanAuthoritySigningDomainV1Token})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name+" admits only published values", func(t *testing.T) {
			t.Parallel()
			tc.run(t)
		})
	}
}

func TestReviewControlEnumJSONRejectsTwentyDistinctHostileRepresentations(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		data []byte
	}{
		{name: "empty document", data: []byte{}},
		{name: "whitespace only", data: []byte(" \n\t")},
		{name: "null token", data: []byte(`null`)},
		{name: "true token", data: []byte(`true`)},
		{name: "false token", data: []byte(`false`)},
		{name: "zero number", data: []byte(`0`)},
		{name: "negative number", data: []byte(`-1`)},
		{name: "object token", data: []byte(`{}`)},
		{name: "array token", data: []byte(`[]`)},
		{name: "empty string", data: []byte(`""`)},
		{name: "unknown future token", data: []byte(`"future"`)},
		{name: "uppercase published token", data: []byte(`"PASS"`)},
		{name: "leading internal whitespace", data: []byte(`" pass"`)},
		{name: "trailing internal whitespace", data: []byte(`"pass "`)},
		{name: "published prefix", data: []byte(`"pas"`)},
		{name: "published suffix", data: []byte(`"passx"`)},
		{name: "unterminated string", data: []byte(`"pass`)},
		{name: "invalid escape", data: []byte(`"\x"`)},
		{name: "two string values", data: []byte(`"pass" "pass"`)},
		{name: "trailing object", data: []byte(`"pass"{}`)},
	}
	for _, tc := range cases {
		t.Run(tc.name+" is refused without receiver mutation", func(t *testing.T) {
			t.Parallel()
			got := VerdictPass
			gotErr := got.UnmarshalJSON(tc.data)
			if !errors.Is(gotErr, core.ErrJSONContract) || got != VerdictPass {
				t.Fatalf("Verdict.UnmarshalJSON() = (%v, %v), want (%v, %v)", got, gotErr, VerdictPass, core.ErrJSONContract)
			}
		})
	}
}
