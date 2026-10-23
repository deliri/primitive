package googleidentity

import (
	"fmt"
	"github.com/deliri/primitive/v2026/core"
	"testing"
)

func TestBrowserOAuthNameMergeLayerTriad(t *testing.T) {
	t.Parallel()
	signed, err := core.ParseOAuthProfileName("Signed name")
	if err != nil {
		t.Fatal(err)
	}
	people, err := core.ParseOAuthProfileName("People name")
	if err != nil {
		t.Fatal(err)
	}
	// Each of three field positions exhausts absent/valid/invalid observations
	// from both independently verified owners. No malformed text is retained.
	for field := range 3 {
		for before := range 3 {
			for after := range 3 {
				t.Run(fmt.Sprintf("field%d_signed%d_people%d", field, before, after), func(t *testing.T) {
					t.Parallel()
					current := BrowserProfile{NameOutcome: core.OAuthProfileMissing}
					observed := BrowserProfile{NameOutcome: core.OAuthProfileMissing}
					values := [...]*core.OAuthProfileName{&current.FullName, &current.GivenName, &current.FamilyName}
					issues := [...]*bool{&current.nameIssues.Full, &current.nameIssues.Given, &current.nameIssues.Family}
					candidates := [...]*core.OAuthProfileName{&observed.FullName, &observed.GivenName, &observed.FamilyName}
					candidateIssues := [...]*bool{&observed.nameIssues.Full, &observed.nameIssues.Given, &observed.nameIssues.Family}
					if before == 1 {
						*values[field] = signed
					}
					if before == 2 {
						*issues[field] = true
						current.NameOutcome = core.OAuthProfileInvalid
					}
					if after == 1 {
						*candidates[field] = people
					}
					if after == 2 {
						*candidateIssues[field] = true
						observed.NameOutcome = core.OAuthProfileInvalid
					}
					got := mergeBrowserNames(current, observed)
					gotValues := [...]core.OAuthProfileName{got.FullName, got.GivenName, got.FamilyName}
					want := core.OAuthProfileName{}
					if before == 1 {
						want = signed
					} else if after == 1 {
						want = people
					}
					wantInvalid := (before == 2 || after == 2) && want == (core.OAuthProfileName{})
					wantOutcome := core.OAuthProfileMissing
					if wantInvalid {
						wantOutcome = core.OAuthProfileInvalid
					} else if want != (core.OAuthProfileName{}) {
						if field == 0 {
							wantOutcome = core.OAuthProfileAvailable
						} else {
							wantOutcome = core.OAuthProfilePartial
						}
					}
					if gotValues[field] != want || got.NameOutcome != wantOutcome || got.Validate() != nil {
						t.Fatalf("merged field%d = (%q,%v,%v), want (%q,%v,nil)", field, gotValues[field].String(), got.NameOutcome, got.Validate(), want.String(), wantOutcome)
					}
					for index, value := range gotValues {
						if index != field && value != (core.OAuthProfileName{}) {
							t.Fatalf("sibling%d=%q, want no invented name", index, value.String())
						}
					}
				})
			}
		}
	}
	for _, outcome := range [...]core.OAuthProfileOutcome{core.OAuthProfileNotRequested, core.OAuthProfileMissing, core.OAuthProfileDenied, core.OAuthProfilePartial, core.OAuthProfileInvalid, core.OAuthProfileUnavailable} {
		t.Run("empty_observation_"+outcome.String(), func(t *testing.T) {
			t.Parallel()
			got := mergeBrowserNames(BrowserProfile{NameOutcome: core.OAuthProfileMissing}, BrowserProfile{NameOutcome: outcome})
			want := outcome
			if outcome == core.OAuthProfileNotRequested {
				want = core.OAuthProfileMissing
			}
			if got.NameOutcome != want {
				t.Fatalf("absent merge=%v,want%v", got.NameOutcome, want)
			}
		})
	}
}
