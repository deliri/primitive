package instagram

import (
	"errors"
	"fmt"
	"io"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/secretstore"
)

// Credential is one Meta Graph access token. It never formats its value.
type Credential struct{ value []byte }

func ParseCredential(value []byte) (Credential, error) {
	candidate := Credential{value: append([]byte(nil), value...)}
	if err := candidate.Validate(); err != nil {
		clear(candidate.value)
		return Credential{}, err
	}
	return candidate, nil
}

func CredentialFromSecret(value secretstore.Value) (Credential, error) {
	material, err := value.CopyBytes()
	if err != nil {
		return Credential{}, errors.Join(core.ErrInstagramContract, err)
	}
	defer clear(material)
	return ParseCredential(material)
}

// Validate admits a visible-ASCII token inside the custody bounds. Meta owns
// the token's grammar; this refuses only what could never be one.
func (c Credential) Validate() error {
	if len(c.value) < core.InstagramCredentialMinimumBytes || len(c.value) > core.InstagramCredentialCustodyMaximumBytes {
		return core.ErrInstagramContract
	}
	for _, b := range c.value {
		if b <= ' ' || b > '~' {
			return core.ErrInstagramContract
		}
	}
	return nil
}

func (c *Credential) Close() error {
	if c == nil {
		return core.ErrInstagramContract
	}
	clear(c.value)
	*c = Credential{}
	return nil
}

func (Credential) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}
