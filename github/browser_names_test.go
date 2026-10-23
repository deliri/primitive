package github

import (
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestBrowserOAuthOptionalNameKeepsStablePrincipal(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name        string
		text        string
		present     bool
		invalidType bool
		want        core.OAuthProfileOutcome
	}{
		{name: "full display name stays whole", text: "Ada Middle Lovelace", present: true, want: core.OAuthProfileAvailable},
		{name: "Unicode spelling stays exact", text: "李小龍", present: true, want: core.OAuthProfileAvailable},
		{name: "missing optional name remains missing", want: core.OAuthProfileMissing},
		{name: "empty optional name is unusable data", present: true, want: core.OAuthProfileInvalid},
		{name: "control-bearing name cannot escape", text: "Ada\x00", present: true, want: core.OAuthProfileInvalid},
		{name: "wrong-type optional name never revokes user ID", invalidType: true, want: core.OAuthProfileInvalid},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			fixture := browserProviderFixture{userID: 42, pages: 1, invalidName: row.invalidType}
			if row.present {
				fixture.fullName = &row.text
			}
			var calls atomic.Uint64
			request := browserOAuthRequest(t, browserOAuthClient(t, fixture, &calls))
			got, err := CompleteBrowserAuthorization(t.Context(), request)
			id, idErr := got.UserID.Uint64()
			if err != nil || idErr != nil || id != 42 || got.NameOutcome != row.want || calls.Load() != 3 || !got.EmailVerified {
				t.Fatalf("optional GitHub name = (%+v,%v,%v,%dcalls), want subject42/outcome%v/verifiedemail/3calls", got, err, idErr, calls.Load(), row.want)
			}
			if row.want == core.OAuthProfileAvailable {
				if got.FullName.String() != row.text {
					t.Fatalf("GitHub full name = %q, want exact unsplit%q", got.FullName.String(), row.text)
				}
			} else if got.FullName != (core.OAuthProfileName{}) {
				t.Fatalf("unusable GitHub name = %q, want no raw rejected text", got.FullName.String())
			}
			principal, principalErr := got.PrincipalIdentity()
			baseline, baselineErr := (BrowserVerifiedIdentity{UserID: got.UserID}).PrincipalIdentity()
			if principalErr != nil || baselineErr != nil || principal != baseline {
				t.Fatalf("GitHub principal = (%v,%v), want stable ID-only principal%v/%v", principal, principalErr, baseline, baselineErr)
			}
		})
	}
}

func TestBrowserOAuthNameValidationExhaustsOutcomes(t *testing.T) {
	t.Parallel()
	name, err := core.ParseOAuthProfileName("Ada Lovelace")
	if err != nil {
		t.Fatal(err)
	}
	id, err := NewBrowserUserID(42)
	if err != nil {
		t.Fatal(err)
	}
	for raw := range 256 {
		for _, present := range [...]bool{false, true} {
			t.Run(fmt.Sprintf("outcome%d_name%t", raw, present), func(t *testing.T) {
				t.Parallel()
				value := BrowserVerifiedIdentity{UserID: id, NameOutcome: core.OAuthProfileOutcome(raw)}
				if present {
					value.FullName = name
				}
				got := value.Validate()
				want := (raw == int(core.OAuthProfileAvailable) && present) || (!present && (raw == int(core.OAuthProfileNotRequested) || raw == int(core.OAuthProfileMissing) || raw == int(core.OAuthProfileInvalid)))
				if (got == nil) != want || (!want && !errors.Is(got, core.ErrGitHubContract)) {
					t.Fatalf("name validation(%d,%t)=%v,want admission%t and owning refusal", raw, present, got, want)
				}
			})
		}
	}
}
