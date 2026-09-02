package twilio

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/secretstore"
)

const (
	accountSIDPrefix = "AC"
	apiKeySIDPrefix  = "SK"
	sidHexBytes      = 32
	sidBytes         = len(accountSIDPrefix) + sidHexBytes
)

// AccountSID follows Twilio's published two-letter prefix plus 32-hex SID grammar.
// Source: https://www.twilio.com/docs/glossary/what-is-a-sid
type AccountSID string

func ParseAccountSID(value string) (AccountSID, error) {
	candidate := AccountSID(value)
	if err := candidate.Validate(); err != nil {
		return "", err
	}
	return candidate, nil
}
func (s AccountSID) Validate() error {
	if !validSID(string(s), accountSIDPrefix) {
		return core.ErrTwilioContract
	}
	return nil
}
func (s AccountSID) String() string { return string(s) }

// APIKeySID follows Twilio's published SK-prefixed 34-character SID grammar.
// Source: https://www.twilio.com/docs/glossary/what-is-a-sid
type APIKeySID string

func ParseAPIKeySID(value string) (APIKeySID, error) {
	candidate := APIKeySID(value)
	if err := candidate.Validate(); err != nil {
		return "", err
	}
	return candidate, nil
}
func (s APIKeySID) Validate() error {
	if !validSID(string(s), apiKeySIDPrefix) {
		return core.ErrTwilioContract
	}
	return nil
}
func (s APIKeySID) String() string { return string(s) }

type Credential struct {
	AccountSID AccountSID
	APIKeySID  APIKeySID
	secret     []byte
}

func NewCredential(account AccountSID, key APIKeySID, secret []byte) (Credential, error) {
	candidate := Credential{AccountSID: account, APIKeySID: key, secret: append([]byte(nil), secret...)}
	if err := candidate.Validate(); err != nil {
		clear(candidate.secret)
		return Credential{}, err
	}
	return candidate, nil
}

func CredentialFromSecret(account AccountSID, key APIKeySID, value secretstore.Value) (Credential, error) {
	material, err := value.CopyBytes()
	if err != nil {
		return Credential{}, contractError(err)
	}
	defer clear(material)
	return NewCredential(account, key, material)
}

func (c Credential) Validate() error {
	if err := errors.Join(c.AccountSID.Validate(), c.APIKeySID.Validate()); err != nil || !validSecret(c.secret, core.TwilioAPIKeySecretCustodyMaximumBytes) {
		return core.ErrTwilioContract
	}
	return nil
}

func (c *Credential) Close() error {
	if c == nil {
		return core.ErrTwilioContract
	}
	clear(c.secret)
	*c = Credential{}
	return nil
}
func (Credential) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

type AuthToken struct{ value []byte }

func ParseAuthToken(value []byte) (AuthToken, error) {
	candidate := AuthToken{value: append([]byte(nil), value...)}
	if err := candidate.Validate(); err != nil {
		clear(candidate.value)
		return AuthToken{}, err
	}
	return candidate, nil
}

func AuthTokenFromSecret(value secretstore.Value) (AuthToken, error) {
	material, err := value.CopyBytes()
	if err != nil {
		return AuthToken{}, contractError(err)
	}
	defer clear(material)
	return ParseAuthToken(material)
}

func (t AuthToken) Validate() error {
	if !validSecret(t.value, core.TwilioAuthTokenCustodyMaximumBytes) {
		return core.ErrTwilioContract
	}
	return nil
}

func validSecret(value []byte, maximum int) bool {
	if len(value) == 0 || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if character < 0x21 || character > 0x7e {
			return false
		}
	}
	return true
}
func (t *AuthToken) Close() error {
	if t == nil {
		return core.ErrTwilioContract
	}
	clear(t.value)
	*t = AuthToken{}
	return nil
}
func (AuthToken) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

func validSID(value, prefix string) bool {
	if len(value) != sidBytes || !strings.HasPrefix(value, prefix) {
		return false
	}
	for index := len(prefix); index < len(value); index++ {
		character := value[index]
		if character < '0' || character > '9' {
			if character < 'a' || character > 'f' {
				if character < 'A' || character > 'F' {
					return false
				}
			}
		}
	}
	return true
}

var (
	_ core.Validatable = AccountSID("")
	_ core.Validatable = APIKeySID("")
	_ core.Validatable = Credential{}
	_ core.Validatable = AuthToken{}
)
