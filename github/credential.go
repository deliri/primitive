package github

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

// AppID is one positive GitHub App identifier.
type AppID struct{ value uint64 }

// NewAppID admits a positive provider identity.
func NewAppID(value uint64) (AppID, error) {
	if value == 0 {
		return AppID{}, core.ErrGitHubContract
	}
	return AppID{value: value}, nil
}

// Validate rejects the unset zero value.
func (i AppID) Validate() error {
	if i.value == 0 {
		return core.ErrGitHubContract
	}
	return nil
}

// Uint64 returns the validated provider identifier.
func (i AppID) Uint64() (uint64, error) {
	if err := i.Validate(); err != nil {
		return 0, err
	}
	return i.value, nil
}

// InstallationID is one positive GitHub App installation identifier.
type InstallationID struct{ value uint64 }

// NewInstallationID admits a positive provider identity.
func NewInstallationID(value uint64) (InstallationID, error) {
	if value == 0 {
		return InstallationID{}, core.ErrGitHubContract
	}
	return InstallationID{value: value}, nil
}

// Validate rejects the unset zero value.
func (i InstallationID) Validate() error {
	if i.value == 0 {
		return core.ErrGitHubContract
	}
	return nil
}

// Uint64 returns the validated provider identifier.
func (i InstallationID) Uint64() (uint64, error) {
	if err := i.Validate(); err != nil {
		return 0, err
	}
	return i.value, nil
}

type credentialState struct {
	privateKey   []byte
	app          AppID
	installation InstallationID
}

// AppCredential owns copied GitHub App RSA private-key material. Diagnostics
// are always redacted and Close destroys Primitive's retained byte copy.
type AppCredential struct{ state *credentialState }

// NewAppCredential validates and takes an owned copy of one GitHub App key.
func NewAppCredential(app AppID, installation InstallationID, privateKeyPEM []byte) (AppCredential, error) {
	if err := errors.Join(app.Validate(), installation.Validate()); err != nil {
		return AppCredential{}, authenticationError(err)
	}
	if len(privateKeyPEM) == 0 || len(privateKeyPEM) > core.GitHubAppPrivateKeyCustodyMaximumBytes {
		return AppCredential{}, core.ErrGitHubAuthentication
	}
	owned := append([]byte(nil), privateKeyPEM...)
	if _, err := parsePrivateKey(owned); err != nil {
		clear(owned)
		return AppCredential{}, authenticationError(err)
	}
	return AppCredential{state: &credentialState{app: app, installation: installation, privateKey: owned}}, nil
}

// Validate proves identities, key custody, and key parseability.
func (c AppCredential) Validate() error {
	if c.state == nil || len(c.state.privateKey) == 0 || len(c.state.privateKey) > core.GitHubAppPrivateKeyCustodyMaximumBytes {
		return core.ErrGitHubAuthentication
	}
	if err := errors.Join(c.state.app.Validate(), c.state.installation.Validate()); err != nil {
		return authenticationError(err)
	}
	if _, err := parsePrivateKey(c.state.privateKey); err != nil {
		return authenticationError(err)
	}
	return nil
}

func (c AppCredential) clone() (AppCredential, error) {
	if err := c.Validate(); err != nil {
		return AppCredential{}, err
	}
	return NewAppCredential(c.state.app, c.state.installation, c.state.privateKey)
}

// Close destroys this credential's retained key bytes.
func (c *AppCredential) Close() error {
	if c == nil || c.state == nil {
		return core.ErrGitHubAuthentication
	}
	clear(c.state.privateKey)
	c.state = nil
	return nil
}

// Format prevents ordinary diagnostics from disclosing credential material.
func (AppCredential) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

func parsePrivateKey(payload []byte) (*rsa.PrivateKey, error) {
	block, rest := pem.Decode(payload)
	if block == nil || len(strings.TrimSpace(string(rest))) != 0 {
		return nil, core.ErrGitHubAuthentication
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, key.Validate()
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, core.ErrGitHubAuthentication
	}
	return key, key.Validate()
}

var (
	_ core.Validatable = AppID{}
	_ core.Validatable = InstallationID{}
	_ core.Validatable = AppCredential{}
)
