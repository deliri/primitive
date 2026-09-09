package googleidentity

import "github.com/deliri/primitive/v2026/core"

// MarshalText preserves an audience's exact spelling at configuration boundaries.
// An OIDC audience is an identifier, not necessarily an HTTPS URL.
func (a Audience) MarshalText() ([]byte, error) {
	if err := a.Validate(); err != nil {
		return nil, err
	}
	return []byte(a.value), nil
}

// UnmarshalText admits bounded UTF-8 without changing the receiver on refusal.
func (a *Audience) UnmarshalText(data []byte) error {
	if a == nil || len(data) > AudienceMaximumBytes {
		return core.ErrGoogleIdentityContract
	}
	admitted, err := ParseAudience(string(data))
	if err != nil {
		return err
	}
	*a = admitted
	return nil
}
