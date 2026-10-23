package googleidentity

import (
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/deliri/primitive/v2026/core"
)

func TestBrowserOAuthOptionalNamesPreserveVerifiedPrincipal(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name      string
		full      *string
		given     *string
		family    *string
		wrongType bool
		want      core.OAuthProfileOutcome
	}{
		{name: "signed structured names preserved", full: browserString("Ada Lovelace"), given: browserString("Ada"), family: browserString("Lovelace"), want: core.OAuthProfileAvailable},
		{name: "signed full name requires no fabricated split", full: browserString("Ada Middle Lovelace"), want: core.OAuthProfileAvailable},
		{name: "missing names preserve subject", want: core.OAuthProfileMissing},
		{name: "given name alone is partial", given: browserString("Ada"), want: core.OAuthProfilePartial},
		{name: "valid full name survives invalid given name", full: browserString("Ada Lovelace"), given: browserString("Ada\x00"), want: core.OAuthProfileInvalid},
		{name: "empty optional name is invalid observation", full: browserString(""), want: core.OAuthProfileInvalid},
		{name: "wrong-type optional name preserves principal", wrongType: true, want: core.OAuthProfileInvalid},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				claims := browserClaimsSeed()
				claims.Name, claims.GivenName, claims.FamilyName = row.full, row.given, row.family
				raw := signBrowserFixture(t, claims, false)
				if row.wrongType {
					raw = signBrowserFixture(t, struct {
						browserClaimsFixture
						Name int `json:"name"`
					}{browserClaimsFixture: claims, Name: 42}, false)
				}
				var calls atomic.Uint32
				request := browserRequestFixture(t, browserLocalClient(t, raw, 200, &calls))
				got, err := CompleteBrowserAuthorization(t.Context(), request)
				if err != nil || got.Subject.String() != claims.Subject || got.Profile.NameOutcome != row.want || !got.EmailVerified || calls.Load() != 2 {
					t.Fatalf("optional Google names = (%+v,%v,%dcalls), want verified subject/outcome%v/2calls", got, err, calls.Load(), row.want)
				}
				wantFull := ""
				if row.full != nil && *row.full != "" {
					wantFull = *row.full
				}
				if got.Profile.FullName.String() != wantFull || (row.want == core.OAuthProfileInvalid && got.Profile.GivenName != (core.OAuthProfileName{})) {
					t.Fatalf("Google name projection = %+v, want only admitted source fields and full name%q", got.Profile, wantFull)
				}
				if row.family == nil && got.Profile.FamilyName != (core.OAuthProfileName{}) {
					t.Fatalf("Google family name = %q, want no invented split", got.Profile.FamilyName.String())
				}
			})
		})
	}
}
