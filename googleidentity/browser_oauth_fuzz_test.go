package googleidentity

import (
	"encoding/base64"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzBrowserOAuthSignedTokenSemanticClosure(f *testing.F) {
	name, err := core.ParseOAuthProfileName("Ada Lovelace")
	if err != nil {
		f.Fatal(err)
	}
	picture, err := core.ParseHTTPEndpoint("https://lh3.googleusercontent.com/profile/photo=s96-c")
	if err != nil {
		f.Fatal(err)
	}
	claims := browserClaimsSeed()
	claims.Picture = browserString(picture.String())
	claims.Name, claims.GivenName, claims.FamilyName = browserString(name.String()), browserString("Ada"), browserString("Lovelace")
	seed := signBrowserFixture(f, claims, false)
	parts := strings.Split(seed, ".")
	for index, part := range parts {
		raw, err := base64.RawURLEncoding.DecodeString(part)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(raw, uint8(index))
	}
	f.Add([]byte{}, uint8(0))
	f.Add([]byte{1}, uint8(2))
	f.Fuzz(func(t *testing.T, data []byte, selector uint8) {
		// Mutate one actual JOSE source segment, then use its native wire encoder.
		// The surrounding JSON string is always representable; malformed segment
		// bytes remain malformed after decode and reach the real SDK admission.
		candidate := append([]string(nil), parts...)
		candidate[int(selector)%3] = base64.RawURLEncoding.EncodeToString(data)
		raw := strings.Join(candidate, ".")
		synctest.Test(t, func(t *testing.T) {
			var calls atomic.Uint32
			client := browserLocalClient(t, raw, 200, &calls)
			got, err := CompleteBrowserAuthorization(t.Context(), browserRequestFixture(t, client))
			genuine := sameSignedDocument(googleCloudIdentityBearerPrefix+raw, googleCloudIdentityBearerPrefix+seed)
			if genuine && err != nil {
				t.Fatalf("genuinely signed source refused = %v", err)
			}
			if err != nil {
				if !errors.Is(err, core.ErrGoogleIdentityContract) || got != (BrowserVerifiedIdentity{}) {
					t.Fatalf("mutated token refusal = (%+v,%v), want zero and owning identity", got, err)
				}
				return
			}
			if !genuine || got.Subject.String() != "stable-google-subject" || !got.EmailVerified || !got.AuthoritativeEmail() {
				t.Fatalf("accepted token changed signed source facts: genuine=%t identity=%+v", genuine, got)
			}
			if got.Profile.PictureURL != picture || got.Profile.PictureOutcome != core.OAuthProfileAvailable || got.Profile.FullName != name || got.Profile.GivenName.String() != "Ada" || got.Profile.FamilyName.String() != "Lovelace" || got.Profile.NameOutcome != core.OAuthProfileAvailable || got.Profile.Validate() != nil {
				t.Fatalf("accepted signed profile = %+v, want exact canonical name facts", got.Profile)
			}
			if calls.Load() != 2 {
				t.Fatalf("accepted provider calls = %d, want token plus JWKS", calls.Load())
			}
		})
	})
}
func FuzzBrowserHostedDomainSemanticClosure(f *testing.F) {
	seed, err := ParseHostedDomain("team.example")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed.String())
	f.Add("")
	f.Add("https://wrong.example")
	f.Fuzz(func(t *testing.T, text string) {
		got, err := ParseHostedDomain(text)
		if err != nil {
			if !errors.Is(err, core.ErrGoogleIdentityContract) || got != (HostedDomain{}) {
				t.Fatalf("hosted domain refusal = (%+v,%v), want zero and owning refusal", got, err)
			}
			return
		}
		if got.String() != text || got.Present() != (text != "") || got.Validate() != nil {
			t.Fatalf("hosted domain admitted altered facts = %+v, want exact %q", got, text)
		}
		again, err := ParseHostedDomain(got.String())
		if err != nil || again != got {
			t.Fatalf("hosted domain fixed point = (%+v,%v), want %+v", again, err, got)
		}
	})
}
