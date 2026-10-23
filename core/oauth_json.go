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

func (value OAuthClientID) MarshalJSON() ([]byte, error) {
	if err := value.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(value.String())
}
func (value *OAuthClientID) UnmarshalJSON(data []byte) error {
	return unmarshalOAuthJSON(value, data, ParseOAuthClientID)
}

func (value OAuthAuthorizationCode) MarshalJSON() ([]byte, error) {
	if err := value.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(value.String())
}
func (value *OAuthAuthorizationCode) UnmarshalJSON(data []byte) error {
	return unmarshalOAuthJSON(value, data, ParseOAuthAuthorizationCode)
}

func (value OAuthRedirectURI) MarshalJSON() ([]byte, error) {
	if err := value.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(value.String())
}
func (value *OAuthRedirectURI) UnmarshalJSON(data []byte) error {
	return unmarshalOAuthJSON(value, data, ParseOAuthRedirectURI)
}

func (value OAuthState) MarshalJSON() ([]byte, error) {
	if err := value.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(value.String())
}
func (value *OAuthState) UnmarshalJSON(data []byte) error {
	return unmarshalOAuthJSON(value, data, ParseOAuthState)
}

func (value OAuthPKCEVerifier) MarshalJSON() ([]byte, error) {
	if err := value.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(value.String())
}
func (value *OAuthPKCEVerifier) UnmarshalJSON(data []byte) error {
	return unmarshalOAuthJSON(value, data, ParseOAuthPKCEVerifier)
}

func (value OAuthPKCEChallenge) MarshalJSON() ([]byte, error) {
	if err := value.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(value.String())
}
func (value *OAuthPKCEChallenge) UnmarshalJSON(data []byte) error {
	return unmarshalOAuthJSON(value, data, parseOAuthPKCEChallenge)
}

func (value OAuthNonce) MarshalJSON() ([]byte, error) {
	if err := value.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(value.String())
}
func (value *OAuthNonce) UnmarshalJSON(data []byte) error {
	return unmarshalOAuthJSON(value, data, ParseOAuthNonce)
}

func (value OAuthSubject) MarshalJSON() ([]byte, error) {
	if err := value.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(value.String())
}
func (value *OAuthSubject) UnmarshalJSON(data []byte) error {
	return unmarshalOAuthJSON(value, data, ParseOAuthSubject)
}

func (value OAuthEmail) MarshalJSON() ([]byte, error) {
	if err := value.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(value.String())
}
func (value *OAuthEmail) UnmarshalJSON(data []byte) error {
	return unmarshalOAuthJSON(value, data, ParseOAuthEmail)
}
