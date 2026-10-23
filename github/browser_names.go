package github

import (
	json "encoding/json/v2"

	"github.com/deliri/primitive/v2026/core"
)

// GitHub's authenticated user contains one optional full name, not structured
// given/family names. A malformed optional field does not invalidate its ID.
// Source: https://docs.github.com/en/rest/users/users#get-the-authenticated-user
type browserProfileNameWire struct {
	name    core.OAuthProfileName
	outcome core.OAuthProfileOutcome
}

func (n *browserProfileNameWire) UnmarshalJSON(data []byte) error {
	var text *string
	if err := json.Unmarshal(data, &text); err != nil {
		*n = browserProfileNameWire{outcome: core.OAuthProfileInvalid}
		return nil
	}
	if text == nil {
		*n = browserProfileNameWire{outcome: core.OAuthProfileMissing}
		return nil
	}
	name, err := core.ParseOAuthProfileName(*text)
	if err != nil {
		*n = browserProfileNameWire{outcome: core.OAuthProfileInvalid}
		return nil
	}
	*n = browserProfileNameWire{name: name, outcome: core.OAuthProfileAvailable}
	return nil
}

type browserUserObservation struct {
	id          BrowserUserID
	fullName    core.OAuthProfileName
	nameOutcome core.OAuthProfileOutcome
}

func validateBrowserName(name core.OAuthProfileName, outcome core.OAuthProfileOutcome) error {
	switch outcome {
	case core.OAuthProfileAvailable:
		return name.Validate()
	case core.OAuthProfileNotRequested, core.OAuthProfileMissing, core.OAuthProfileInvalid:
		if name != (core.OAuthProfileName{}) {
			return core.ErrGitHubResponse
		}
		return nil
	default:
		return core.ErrGitHubResponse
	}
}

func (browserProfileNameWire) githubInternalFlow() {}
func (browserUserObservation) githubInternalFlow() {}
