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

func ParseOAuthProfileName(value string) (OAuthProfileName, error) {
	name := OAuthProfileName{value: value}
	if err := name.Validate(); err != nil {
		return OAuthProfileName{}, err
	}
	return name, nil
}

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

func (n OAuthProfileName) String() string { return n.value }

func (n OAuthProfileName) MarshalJSON() ([]byte, error) {
	if err := n.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(n.value)
}

func (n *OAuthProfileName) UnmarshalJSON(data []byte) error {
	return unmarshalOAuthJSON(n, data, ParseOAuthProfileName)
}

// OAuthProfileOutcome reports one optional observation. It is not a browser
// flow state: missing, refused, partial and failed imports remain distinct.
type OAuthProfileOutcome uint8

const (
	OAuthProfileNotRequested OAuthProfileOutcome = iota
	OAuthProfileMissing
	OAuthProfileDenied
	OAuthProfilePartial
	OAuthProfileAvailable
	OAuthProfileInvalid
	OAuthProfileUnavailable
)

func (o OAuthProfileOutcome) Validate() error {
	if o > OAuthProfileUnavailable {
		return ErrOAuthContract
	}
	return nil
}

func (o OAuthProfileOutcome) IsValid() bool { return o.Validate() == nil }

func (o OAuthProfileOutcome) String() string {
	if !o.IsValid() {
		return UnknownEnumDiagnostic
	}
	return [...]string{"not_requested", "missing", "denied", "partial", "available", "invalid", "unavailable"}[o]
}

func (o OAuthProfileOutcome) MarshalJSON() ([]byte, error) {
	if err := o.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(o.String())
}

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
