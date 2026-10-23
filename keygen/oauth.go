package keygen

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"github.com/deliri/primitive/v2026/core"
)

// OAuthBrowserProof holds one request's independent anti-forgery, proof-key and
// OpenID nonce values. The caller owns encrypted cookie custody and lifetime.
// RFC7636 recommends32random octets before base64URL encoding the verifier.
// Source: https://www.rfc-editor.org/rfc/rfc7636#section-4.1
type OAuthBrowserProof struct {
	State    core.OAuthState
	Verifier core.OAuthPKCEVerifier
	Nonce    core.OAuthNonce
}

func (p OAuthBrowserProof) Validate() error {
	if err := errors.Join(p.State.Validate(), p.Verifier.Validate(), p.Nonce.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}
func (OAuthBrowserProof) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

// GenerateOAuthBrowserProof makes three separate native CSPRNG draws. It owns
// no flow registry, account, cookie, cache, goroutine, or provider request.
func GenerateOAuthBrowserProof() (OAuthBrowserProof, error) {
	state, err := oauthProofText()
	if err != nil {
		return OAuthBrowserProof{}, err
	}
	verifier, err := oauthProofText()
	if err != nil {
		return OAuthBrowserProof{}, err
	}
	nonce, err := oauthProofText()
	if err != nil {
		return OAuthBrowserProof{}, err
	}
	stateValue, stateErr := core.ParseOAuthState(state)
	verifierValue, verifierErr := core.ParseOAuthPKCEVerifier(verifier)
	nonceValue, nonceErr := core.ParseOAuthNonce(nonce)
	if err := errors.Join(stateErr, verifierErr, nonceErr); err != nil {
		return OAuthBrowserProof{}, contractError(err)
	}
	return OAuthBrowserProof{State: stateValue, Verifier: verifierValue, Nonce: nonceValue}, nil
}
func oauthProofText() (string, error) {
	size, err := core.NewByteCount(32)
	if err != nil {
		return "", contractError(err)
	}
	token, err := RandomToken(RandomTokenRequest{Size: size})
	if err != nil {
		return "", err
	}
	value, err := token.Bytes()
	if err != nil {
		return "", err
	}
	defer clear(value)
	return base64.RawURLEncoding.EncodeToString(value), nil
}
