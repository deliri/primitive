package core

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

// OAuthProfileName is provider-observed human name data, never account identity.
// It preserves the provider's valid Unicode spelling and does not split names.
// Optional absence is represented by the zero value in the containing profile.
type OAuthProfileName struct{ value string }

// ParseOAuthProfileName admits only a value that OAuthProfileName.Validate accepts.
func ParseOAuthProfileName(value string) (OAuthProfileName, error) {
	name := OAuthProfileName{value: value}
	if err := name.Validate(); err != nil {
		return OAuthProfileName{}, err
	}
	return name, nil
}

// Validate refuses a value outside the OAuthProfileName grammar.
func (n OAuthProfileName) Validate() error {
	if !utf8.ValidString(n.value) || strings.TrimSpace(n.value) == "" {
		return ErrOAuthContract
	}
	for _, character := range n.value {
		if unicode.IsControl(character) {
			return ErrOAuthContract
		}
	}
	return nil
}

// String returns the exact validated value.
func (n OAuthProfileName) String() string { return n.value }

// MarshalJSON writes the validated value as a canonical JSON string.
func (n OAuthProfileName) MarshalJSON() ([]byte, error) {
	if err := n.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(n.value)
}

// UnmarshalJSON admits only a JSON string that OAuthProfileName.Validate accepts.
func (n *OAuthProfileName) UnmarshalJSON(data []byte) error {
	return unmarshalOAuthJSON(n, data, ParseOAuthProfileName)
}

// OAuthProfileOutcome reports one optional observation. It is not a browser
// flow state: missing, refused, partial and failed imports remain distinct.
type OAuthProfileOutcome uint8

const (
	// OAuthProfileNotRequested means the caller did not ask for this fact.
	OAuthProfileNotRequested OAuthProfileOutcome = iota
	// OAuthProfileMissing means the fact was requested and the provider sent none.
	OAuthProfileMissing
	// OAuthProfileDenied means the account holder refused consent for the fact.
	OAuthProfileDenied
	// OAuthProfilePartial means the provider sent only part of the requested fact.
	OAuthProfilePartial
	// OAuthProfileAvailable means the provider sent a valid fact.
	OAuthProfileAvailable
	// OAuthProfileInvalid means the provider sent a fact outside the contract.
	OAuthProfileInvalid
	// OAuthProfileUnavailable means the provider could not be asked or failed to answer.
	OAuthProfileUnavailable
)

// Validate refuses an outcome outside the closed domain.
func (o OAuthProfileOutcome) Validate() error {
	if o > OAuthProfileUnavailable {
		return ErrOAuthContract
	}
	return nil
}

// IsValid reports whether Validate accepts the outcome.
func (o OAuthProfileOutcome) IsValid() bool { return o.Validate() == nil }

// String names a valid outcome, and an invalid one as empty.
func (o OAuthProfileOutcome) String() string {
	if !o.IsValid() {
		return UnknownEnumDiagnostic
	}
	return [...]string{"not_requested", "missing", "denied", "partial", "available", "invalid", "unavailable"}[o]
}

// MarshalJSON writes the validated value as a canonical JSON string.
func (o OAuthProfileOutcome) MarshalJSON() ([]byte, error) {
	if err := o.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(o.String())
}

// UnmarshalJSON admits only a JSON string that OAuthProfileOutcome.Validate accepts.
func (o *OAuthProfileOutcome) UnmarshalJSON(data []byte) error {
	if o == nil {
		return errors.Join(ErrOAuthContract, ErrJSONContract)
	}
	text, err := DecodeJSONStringToken(data)
	if err != nil {
		return errors.Join(ErrOAuthContract, ErrJSONContract, err)
	}
	for value := OAuthProfileNotRequested; value <= OAuthProfileUnavailable; value++ {
		if text == value.String() {
			*o = value
			return nil
		}
	}
	return errors.Join(ErrOAuthContract, ErrJSONContract)
}
