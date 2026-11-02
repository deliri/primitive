package core

import "errors"

// OAuth nominal codecs preserve the validated wire text. Secret proof fields
// belong only inside caller-owned authenticated encryption, never public logs.
// ClientSecret refuses JSON entirely; explicit provider code exchange owns it.
func unmarshalOAuthJSON[Value Validatable](destination *Value, data []byte, parse func(string) (Value, error)) error {
	if destination == nil {
		return errors.Join(ErrOAuthContract, ErrJSONContract)
	}
	text, err := DecodeJSONStringToken(data)
	if err != nil {
		return errors.Join(ErrOAuthContract, ErrJSONContract, err)
	}
	value, err := parse(text)
	if err != nil {
		return errors.Join(ErrOAuthContract, ErrJSONContract, err)
	}
	*destination = value
	return nil
}

// MarshalJSON writes the validated value as a canonical JSON string.
func (value OAuthClientID) MarshalJSON() ([]byte, error) {
	if err := value.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(value.String())
}

// UnmarshalJSON admits only a JSON string that OAuthClientID.Validate accepts.
func (value *OAuthClientID) UnmarshalJSON(data []byte) error {
	return unmarshalOAuthJSON(value, data, ParseOAuthClientID)
}

// MarshalJSON writes the validated value as a canonical JSON string.
func (value OAuthAuthorizationCode) MarshalJSON() ([]byte, error) {
	if err := value.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(value.String())
}

// UnmarshalJSON admits only a JSON string that OAuthAuthorizationCode.Validate accepts.
func (value *OAuthAuthorizationCode) UnmarshalJSON(data []byte) error {
	return unmarshalOAuthJSON(value, data, ParseOAuthAuthorizationCode)
}

// MarshalJSON writes the validated value as a canonical JSON string.
func (value OAuthRedirectURI) MarshalJSON() ([]byte, error) {
	if err := value.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(value.String())
}

// UnmarshalJSON admits only a JSON string that OAuthRedirectURI.Validate accepts.
func (value *OAuthRedirectURI) UnmarshalJSON(data []byte) error {
	return unmarshalOAuthJSON(value, data, ParseOAuthRedirectURI)
}

// MarshalJSON writes the validated value as a canonical JSON string.
func (value OAuthState) MarshalJSON() ([]byte, error) {
	if err := value.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(value.String())
}

// UnmarshalJSON admits only a JSON string that OAuthState.Validate accepts.
func (value *OAuthState) UnmarshalJSON(data []byte) error {
	return unmarshalOAuthJSON(value, data, ParseOAuthState)
}

// MarshalJSON writes the validated value as a canonical JSON string.
func (value OAuthPKCEVerifier) MarshalJSON() ([]byte, error) {
	if err := value.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(value.String())
}

// UnmarshalJSON admits only a JSON string that OAuthPKCEVerifier.Validate accepts.
func (value *OAuthPKCEVerifier) UnmarshalJSON(data []byte) error {
	return unmarshalOAuthJSON(value, data, ParseOAuthPKCEVerifier)
}

// MarshalJSON writes the validated value as a canonical JSON string.
func (value OAuthPKCEChallenge) MarshalJSON() ([]byte, error) {
	if err := value.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(value.String())
}

// UnmarshalJSON admits only a JSON string that OAuthPKCEChallenge.Validate accepts.
func (value *OAuthPKCEChallenge) UnmarshalJSON(data []byte) error {
	return unmarshalOAuthJSON(value, data, parseOAuthPKCEChallenge)
}

// MarshalJSON writes the validated value as a canonical JSON string.
func (value OAuthNonce) MarshalJSON() ([]byte, error) {
	if err := value.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(value.String())
}

// UnmarshalJSON admits only a JSON string that OAuthNonce.Validate accepts.
func (value *OAuthNonce) UnmarshalJSON(data []byte) error {
	return unmarshalOAuthJSON(value, data, ParseOAuthNonce)
}

// MarshalJSON writes the validated value as a canonical JSON string.
func (value OAuthSubject) MarshalJSON() ([]byte, error) {
	if err := value.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(value.String())
}

// UnmarshalJSON admits only a JSON string that OAuthSubject.Validate accepts.
func (value *OAuthSubject) UnmarshalJSON(data []byte) error {
	return unmarshalOAuthJSON(value, data, ParseOAuthSubject)
}

// MarshalJSON writes the validated value as a canonical JSON string.
func (value OAuthEmail) MarshalJSON() ([]byte, error) {
	if err := value.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(value.String())
}

// UnmarshalJSON admits only a JSON string that OAuthEmail.Validate accepts.
func (value *OAuthEmail) UnmarshalJSON(data []byte) error {
	return unmarshalOAuthJSON(value, data, ParseOAuthEmail)
}
