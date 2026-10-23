package googleidentity

import (
	"github.com/deliri/primitive/v2026/core"
	"google.golang.org/api/idtoken"
)

// Basic OIDC profile scope can supply these optional signed name claims. An
// unavailable or malformed optional name never changes the verified subject.
// Source: https://developers.google.com/identity/openid-connect/openid-connect#an-id-tokens-payload
func browserAuthorizationScopes(request BrowserProfileRequest) []string {
	scopes := []string{"openid", "email", "profile"}
	if request.PhoneNumber {
		scopes = append(scopes, BrowserPhoneScope)
	}
	if request.Birthday {
		scopes = append(scopes, BrowserBirthdayScope)
	}
	return scopes
}

func browserClaimProfile(payload *idtoken.Payload) BrowserProfile {
	full, fullOutcome := browserNameClaim(payload, "name")
	given, givenOutcome := browserNameClaim(payload, "given_name")
	family, familyOutcome := browserNameClaim(payload, "family_name")
	picture, pictureOutcome := browserPictureClaim(payload)
	result := BrowserProfile{PictureURL: picture, PictureOutcome: pictureOutcome, FullName: full, GivenName: given, FamilyName: family, NameOutcome: core.OAuthProfileMissing, nameIssues: browserNameIssues{Full: fullOutcome == core.OAuthProfileInvalid, Given: givenOutcome == core.OAuthProfileInvalid, Family: familyOutcome == core.OAuthProfileInvalid}}
	switch {
	case fullOutcome == core.OAuthProfileInvalid || givenOutcome == core.OAuthProfileInvalid || familyOutcome == core.OAuthProfileInvalid:
		result.NameOutcome = core.OAuthProfileInvalid
	case fullOutcome == core.OAuthProfileAvailable:
		result.NameOutcome = core.OAuthProfileAvailable
	case givenOutcome == core.OAuthProfileAvailable || familyOutcome == core.OAuthProfileAvailable:
		result.NameOutcome = core.OAuthProfilePartial
	default:
		return result
	}
	return result
}

func browserNameClaim(payload *idtoken.Payload, member string) (core.OAuthProfileName, core.OAuthProfileOutcome) {
	observed, present := payload.Claims[member]
	if !present {
		return core.OAuthProfileName{}, core.OAuthProfileMissing
	}
	text, isText := observed.(string)
	if !isText {
		return core.OAuthProfileName{}, core.OAuthProfileInvalid
	}
	name, err := core.ParseOAuthProfileName(text)
	if err != nil {
		return core.OAuthProfileName{}, core.OAuthProfileInvalid
	}
	return name, core.OAuthProfileAvailable
}

// browserNameIssues retains only invalid-field positions, never rejected PII.
// A later validated observation can repair exactly its own absent field.
type browserNameIssues struct{ Full, Given, Family bool }

func (browserNameIssues) googleIdentityInternalFlow() {}

func mergeBrowserNames(current, observed BrowserProfile) BrowserProfile {
	for _, part := range [...]struct {
		value     *core.OAuthProfileName
		issue     *bool
		candidate core.OAuthProfileName
		invalid   bool
	}{
		{&current.FullName, &current.nameIssues.Full, observed.FullName, observed.nameIssues.Full},
		{&current.GivenName, &current.nameIssues.Given, observed.GivenName, observed.nameIssues.Given},
		{&current.FamilyName, &current.nameIssues.Family, observed.FamilyName, observed.nameIssues.Family},
	} {
		if *part.value != (core.OAuthProfileName{}) {
			continue
		}
		if part.candidate != (core.OAuthProfileName{}) {
			*part.value, *part.issue = part.candidate, false
		} else {
			*part.issue = *part.issue || part.invalid
		}
	}
	switch {
	case current.nameIssues.Full || current.nameIssues.Given || current.nameIssues.Family:
		current.NameOutcome = core.OAuthProfileInvalid
	case current.FullName != (core.OAuthProfileName{}):
		current.NameOutcome = core.OAuthProfileAvailable
	case current.GivenName != (core.OAuthProfileName{}) || current.FamilyName != (core.OAuthProfileName{}):
		current.NameOutcome = core.OAuthProfilePartial
	case observed.NameOutcome != core.OAuthProfileNotRequested:
		current.NameOutcome = observed.NameOutcome
	}
	return current
}

// The optional OIDC picture is authenticated data, not a token or an image
// fetch. Reuse the canonical URL contract and require encrypted transport.
// Source: https://developers.google.com/identity/openid-connect/openid-connect#an-id-tokens-payload
func browserPictureClaim(payload *idtoken.Payload) (core.HTTPEndpoint, core.OAuthProfileOutcome) {
	observed, present := payload.Claims["picture"]
	if !present {
		return core.HTTPEndpoint{}, core.OAuthProfileMissing
	}
	text, isText := observed.(string)
	if !isText {
		return core.HTTPEndpoint{}, core.OAuthProfileInvalid
	}
	endpoint, err := core.ParseHTTPEndpoint(text)
	if err != nil || endpoint.HTTPURL().Scheme != core.SchemeHTTPS {
		return core.HTTPEndpoint{}, core.OAuthProfileInvalid
	}
	return endpoint, core.OAuthProfileAvailable
}
