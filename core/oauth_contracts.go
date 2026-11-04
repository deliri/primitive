package core

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"strings"
	"sync"
	"unicode/utf8"
)

// OAuthClockSkewMaximumSeconds bounds the explicit browser-token issued-at
// tolerance. Callers choose a value from zero through60seconds. Expiry remains
// the official SDK's strict authenticated boundary.
const OAuthClockSkewMaximumSeconds = 60

// OAuth protocol text follows RFC6749 Appendix A; PKCE follows RFC7636.
// These are nominal protocol values, not session, account or provider policy.
// Sources: https://www.rfc-editor.org/rfc/rfc6749#appendix-A
// https://www.rfc-editor.org/rfc/rfc7636#section-4.1

// OAuthClientID is a provider-issued client identifier (RFC 6749 client_id): non-empty visible ASCII.
type OAuthClientID struct{ value string }

// OAuthAuthorizationCode is a one-time code from the provider redirect (RFC 6749 code). It is a credential: formatting redacts it.
type OAuthAuthorizationCode struct{ value string }

// OAuthState binds a provider redirect to the request that began it (RFC 6749 state). Formatting redacts it.
type OAuthState struct{ value string }

// OAuthNonce binds an identity token to one authorization request. Formatting redacts it.
type OAuthNonce struct{ value string }

// OAuthPKCEVerifier is an RFC 7636 code verifier: 43 to 128 unreserved characters. Formatting redacts it.
type OAuthPKCEVerifier struct{ value string }

// OAuthPKCEChallenge is the RFC 7636 S256 challenge: unpadded base64url SHA-256 of a verifier.
type OAuthPKCEChallenge struct{ value string }

// OAuthRedirectURI is the registered redirection endpoint (RFC 6749 redirect_uri).
type OAuthRedirectURI struct{ value HTTPEndpoint }

// OAuthSubject is the provider's stable identifier for the signed-in account.
type OAuthSubject struct{ value string }

// OAuthEmail is a bare email address as the provider reports it, with no display name.
type OAuthEmail struct{ value string }

func validOAuthText(value string) bool {
	if value == "" {
		return false
	}
	for i := range len(value) {
		if value[i] < 0x20 || value[i] > 0x7e {
			return false
		}
	}
	return true
}
func validOAuthUnreserved(value string) bool {
	for i := range len(value) {
		b := value[i]
		if !(b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || strings.ContainsRune("-._~", rune(b))) {
			return false
		}
	}
	return true
}

// ParseOAuthClientID admits only a value that OAuthClientID.Validate accepts.
func ParseOAuthClientID(value string) (OAuthClientID, error) {
	v := OAuthClientID{value}
	if err := v.Validate(); err != nil {
		return OAuthClientID{}, err
	}
	return v, nil
}

// Validate refuses a value outside the OAuthClientID grammar.
func (v OAuthClientID) Validate() error {
	if !validOAuthText(v.value) {
		return ErrOAuthContract
	}
	return nil
}

// String returns the exact validated value.
func (v OAuthClientID) String() string { return v.value }

// ParseOAuthAuthorizationCode admits only a value that OAuthAuthorizationCode.Validate accepts.
func ParseOAuthAuthorizationCode(value string) (OAuthAuthorizationCode, error) {
	v := OAuthAuthorizationCode{value}
	if err := v.Validate(); err != nil {
		return OAuthAuthorizationCode{}, err
	}
	return v, nil
}

// Validate refuses a value outside the OAuthAuthorizationCode grammar.
func (v OAuthAuthorizationCode) Validate() error {
	if !validOAuthText(v.value) {
		return ErrOAuthContract
	}
	return nil
}

// String explicitly discloses the code for the provider exchange. Formatting redacts it.
func (v OAuthAuthorizationCode) String() string { return v.value }

// Format redacts the value under every fmt verb.
func (OAuthAuthorizationCode) Format(s fmt.State, _ rune) {
	_, _ = io.WriteString(s, RedactedValueText)
}

// ParseOAuthState admits only a value that OAuthState.Validate accepts.
func ParseOAuthState(value string) (OAuthState, error) {
	v := OAuthState{value}
	if err := v.Validate(); err != nil {
		return OAuthState{}, err
	}
	return v, nil
}

// Validate refuses a value outside the OAuthState grammar.
func (v OAuthState) Validate() error {
	if !validOAuthText(v.value) {
		return ErrOAuthContract
	}
	return nil
}

// String discloses the value for the provider request; Format redacts it.
func (v OAuthState) String() string { return v.value }

// Format redacts the value under every fmt verb.
func (OAuthState) Format(s fmt.State, _ rune) { _, _ = io.WriteString(s, RedactedValueText) }

// ParseOAuthNonce admits only a value that OAuthNonce.Validate accepts.
func ParseOAuthNonce(value string) (OAuthNonce, error) {
	v := OAuthNonce{value}
	if err := v.Validate(); err != nil {
		return OAuthNonce{}, err
	}
	return v, nil
}

// Validate refuses a value outside the OAuthNonce grammar.
func (v OAuthNonce) Validate() error {
	if !validOAuthText(v.value) {
		return ErrOAuthContract
	}
	return nil
}

// String discloses the value for the provider request; Format redacts it.
func (v OAuthNonce) String() string { return v.value }

// Format redacts the value under every fmt verb.
func (OAuthNonce) Format(s fmt.State, _ rune) { _, _ = io.WriteString(s, RedactedValueText) }

// ParseOAuthPKCEVerifier admits only a value that OAuthPKCEVerifier.Validate accepts.
func ParseOAuthPKCEVerifier(value string) (OAuthPKCEVerifier, error) {
	v := OAuthPKCEVerifier{value}
	if err := v.Validate(); err != nil {
		return OAuthPKCEVerifier{}, err
	}
	return v, nil
}

// Validate refuses a value outside the OAuthPKCEVerifier grammar.
func (v OAuthPKCEVerifier) Validate() error {
	if len(v.value) < 43 || len(v.value) > 128 || !validOAuthUnreserved(v.value) {
		return ErrOAuthContract
	}
	return nil
}

// String discloses the value for the provider request; Format redacts it.
func (v OAuthPKCEVerifier) String() string { return v.value }

// Format redacts the value under every fmt verb.
func (OAuthPKCEVerifier) Format(s fmt.State, _ rune) { _, _ = io.WriteString(s, RedactedValueText) }

// Challenge derives exactly the RFC7636 S256 challenge through Go SHA256.
func (v OAuthPKCEVerifier) Challenge() (OAuthPKCEChallenge, error) {
	if err := v.Validate(); err != nil {
		return OAuthPKCEChallenge{}, err
	}
	digest := sha256.Sum256([]byte(v.value))
	return parseOAuthPKCEChallenge(base64.RawURLEncoding.EncodeToString(digest[:]))
}
func parseOAuthPKCEChallenge(value string) (OAuthPKCEChallenge, error) {
	v := OAuthPKCEChallenge{value}
	if err := v.Validate(); err != nil {
		return OAuthPKCEChallenge{}, err
	}
	return v, nil
}

// Validate refuses a value outside the OAuthPKCEChallenge grammar.
func (v OAuthPKCEChallenge) Validate() error {
	raw, err := base64.RawURLEncoding.Strict().DecodeString(v.value)
	if err != nil {
		return errors.Join(ErrOAuthContract, err)
	}
	if len(raw) != sha256.Size || base64.RawURLEncoding.EncodeToString(raw) != v.value {
		return ErrOAuthContract
	}
	return nil
}

// String returns the exact validated value.
func (v OAuthPKCEChallenge) String() string { return v.value }

// ParseOAuthRedirectURI admits only a value that OAuthRedirectURI.Validate accepts.
func ParseOAuthRedirectURI(value string) (OAuthRedirectURI, error) {
	endpoint, err := ParseHTTPEndpoint(value)
	if err != nil {
		return OAuthRedirectURI{}, errors.Join(ErrOAuthContract, err)
	}
	v := OAuthRedirectURI{endpoint}
	if err := v.Validate(); err != nil {
		return OAuthRedirectURI{}, err
	}
	return v, nil
}

// Validate refuses a value outside the OAuthRedirectURI grammar.
func (v OAuthRedirectURI) Validate() error {
	if err := v.value.Validate(); err != nil {
		return errors.Join(ErrOAuthContract, err)
	}
	u := v.value.HTTPURL()
	if u.Scheme != SchemeHTTPS && !(u.Scheme == "http" && (u.Hostname() == "localhost" || strings.HasSuffix(u.Hostname(), ".localhost") || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1")) {
		return ErrOAuthContract
	}
	return nil
}

// String returns the exact validated value.
func (v OAuthRedirectURI) String() string { return v.value.String() }

// OAuthSubject is the case-sensitive OIDC subject, at most255 ASCII characters.
// Source: https://openid.net/specs/openid-connect-core-1_0.html#IDToken
func ParseOAuthSubject(value string) (OAuthSubject, error) {
	v := OAuthSubject{value}
	if err := v.Validate(); err != nil {
		return OAuthSubject{}, err
	}
	return v, nil
}

// Validate refuses a value outside the OAuthSubject grammar.
func (v OAuthSubject) Validate() error {
	if len(v.value) > 255 || !validOAuthText(v.value) {
		return ErrOAuthContract
	}
	return nil
}

// String returns the exact validated value.
func (v OAuthSubject) String() string { return v.value }

// ParseOAuthEmail admits only a value that OAuthEmail.Validate accepts.
func ParseOAuthEmail(value string) (OAuthEmail, error) {
	v := OAuthEmail{value}
	if err := v.Validate(); err != nil {
		return OAuthEmail{}, err
	}
	return v, nil
}

// Validate refuses a value outside the OAuthEmail grammar.
func (v OAuthEmail) Validate() error {
	address, err := mail.ParseAddress(v.value)
	if err != nil {
		return errors.Join(ErrOAuthContract, err)
	}
	if address.Name != "" || address.Address != v.value || !utf8.ValidString(v.value) {
		return ErrOAuthContract
	}
	return nil
}

// String returns the exact validated value.
func (v OAuthEmail) String() string { return v.value }

// OAuthClientSecret owns an independently copied confidential-client secret.
// Copies share destruction. CopyBytes lends an independent caller-owned copy;
// closing this handle cannot erase copies already handed to an active SDK call.
type OAuthClientSecret struct{ state *oauthClientSecretState }
type oauthClientSecretState struct {
	mutex sync.RWMutex
	value []byte
}

// NewOAuthClientSecret copies a non-empty secret into owned custody.
func NewOAuthClientSecret(value []byte) (OAuthClientSecret, error) {
	if len(value) == 0 {
		return OAuthClientSecret{}, ErrOAuthContract
	}
	return OAuthClientSecret{&oauthClientSecretState{value: append([]byte(nil), value...)}}, nil
}

// Validate refuses a missing or closed secret.
func (v OAuthClientSecret) Validate() error {
	if v.state == nil {
		return ErrOAuthContract
	}
	v.state.mutex.RLock()
	defer v.state.mutex.RUnlock()
	if len(v.state.value) == 0 {
		return ErrOAuthContract
	}
	return nil
}

// CopyBytes lends an independent copy for one provider call; the caller clears it.
func (v OAuthClientSecret) CopyBytes() ([]byte, error) {
	if v.state == nil {
		return nil, ErrOAuthContract
	}
	v.state.mutex.RLock()
	defer v.state.mutex.RUnlock()
	if len(v.state.value) == 0 {
		return nil, ErrOAuthContract
	}
	return append([]byte(nil), v.state.value...), nil
}

// Close clears the secret for every copy of this handle.
func (v *OAuthClientSecret) Close() error {
	if v == nil || v.state == nil {
		return ErrOAuthContract
	}
	v.state.mutex.Lock()
	defer v.state.mutex.Unlock()
	clear(v.state.value)
	v.state.value = nil
	return nil
}

// Format redacts the value under every fmt verb.
func (OAuthClientSecret) Format(s fmt.State, _ rune) { _, _ = io.WriteString(s, RedactedValueText) }

// MarshalJSON always refuses: a client secret never becomes JSON.
func (OAuthClientSecret) MarshalJSON() ([]byte, error) {
	return nil, errors.Join(ErrOAuthContract, ErrJSONContract)
}
