package googleidentity

import (
	"errors"
	"fmt"
	"github.com/deliri/primitive/v2026/core"
	"sync/atomic"
	"testing"
	"testing/synctest"
)

func TestBrowserOAuthOptionalPicturePreservesPrincipal(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name      string
		text      *string
		wrongType bool
		want      core.OAuthProfileOutcome
	}{
		{name: "signed HTTPS picture observed", text: browserString("https://lh3.googleusercontent.com/profile/photo=s96-c"), want: core.OAuthProfileAvailable},
		{name: "missing picture stays missing", want: core.OAuthProfileMissing},
		{name: "wrong type cannot become picture", wrongType: true, want: core.OAuthProfileInvalid},
		{name: "HTTP picture is not admitted", text: browserString("http://images.example/profile.jpg"), want: core.OAuthProfileInvalid},
		{name: "embedded credentials are refused", text: browserString("https://user:secret@images.example/profile.jpg"), want: core.OAuthProfileInvalid},
		{name: "empty picture is invalid observation", text: browserString(""), want: core.OAuthProfileInvalid},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				claims := browserClaimsSeed()
				claims.Picture = row.text
				token := signBrowserFixture(t, claims, false)
				if row.wrongType {
					token = signBrowserFixture(t, struct {
						browserClaimsFixture
						Picture bool `json:"picture"`
					}{claims, true}, false)
				}
				var calls atomic.Uint32
				got, err := CompleteBrowserAuthorization(t.Context(), browserRequestFixture(t, browserLocalClient(t, token, 200, &calls)))
				if err != nil || got.Subject.String() != claims.Subject || !got.EmailVerified || got.Profile.PictureOutcome != row.want || got.Validate() != nil || calls.Load() != 2 {
					t.Fatalf("signed picture=(%+v,%v,%dcalls),want preservedsubject/outcome%v/2calls", got, err, calls.Load(), row.want)
				}
				if row.want == core.OAuthProfileAvailable {
					if got.Profile.PictureURL.String() != *row.text {
						t.Fatalf("pictureURL=%q,wantexactsignedURL%q", got.Profile.PictureURL.String(), *row.text)
					}
				} else if got.Profile.PictureURL != (core.HTTPEndpoint{}) {
					t.Fatalf("unusablepicture=%+v,wantzeroURL", got.Profile.PictureURL)
				}
			})
		})
	}
}

func TestBrowserOAuthPictureValidationLayerTriad(t *testing.T) {
	t.Parallel()
	https, e1 := core.ParseHTTPEndpoint("https://images.example/profile.jpg")
	http, e2 := core.ParseHTTPEndpoint("http://images.example/profile.jpg")
	if err := errors.Join(e1, e2); err != nil {
		t.Fatal(err)
	}
	for raw := range 256 {
		for index, endpoint := range [...]core.HTTPEndpoint{{}, https, http} {
			t.Run(fmt.Sprintf("outcome%d_endpoint%d", raw, index), func(t *testing.T) {
				t.Parallel()
				value := BrowserProfile{PictureURL: endpoint, PictureOutcome: core.OAuthProfileOutcome(raw)}
				got := value.Validate()
				want := raw <= int(core.OAuthProfileUnavailable) && ((raw == int(core.OAuthProfileAvailable) && index == 1) || (raw != int(core.OAuthProfileAvailable) && index == 0))
				if (got == nil) != want || (!want && !errors.Is(got, core.ErrGoogleIdentityContract)) {
					t.Fatalf("picturevalidation(%d,%d)=%v,wantadmitted%t/owningrefusal", raw, index, got, want)
				}
			})
		}
	}
}
