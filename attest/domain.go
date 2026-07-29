package attest

import (
	"encoding/json"
	"errors"
)

type domainToken struct {
	text   [SigningDomainMaximumBytes]byte
	length int
}

func newDomainToken(text []byte) (domainToken, error) {
	if !validDomainText(text) {
		return domainToken{}, contractError(errors.New(domainCanonicalErrorText))
	}
	var token domainToken
	copy(token.text[:], text)
	token.length = len(text)
	return token, nil
}

func (t domainToken) Validate() error {
	if t.length <= 0 || t.length > len(t.text) {
		return contractError(errors.New(domainCanonicalErrorText))
	}
	if !validDomainText(t.text[:t.length]) {
		return contractError(errors.New(domainCanonicalErrorText))
	}
	for _, value := range t.text[t.length:] {
		if value != 0 {
			return contractError(errors.New(domainCanonicalErrorText))
		}
	}
	return nil
}

func (t domainToken) bytes() []byte {
	if t.Validate() != nil {
		return nil
	}
	return t.text[:t.length]
}

func (t domainToken) MarshalJSON() ([]byte, error) {
	if err := t.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(string(t.text[:t.length]))
}

func (t *domainToken) UnmarshalJSON(data []byte) error {
	if t == nil {
		return contractError(errors.New(domainCanonicalErrorText))
	}
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return contractError(err)
	}
	candidate, err := newDomainToken([]byte(text))
	if err != nil {
		return err
	}
	*t = candidate
	return nil
}

func canonicalDomain[D SigningDomain[D]](domain D) (domainToken, error) {
	token, err := guardedCall(func() (domainToken, error) {
		if err := domain.Validate(); err != nil {
			return domainToken{}, err
		}
		text, marshalErr := domain.MarshalText()
		if marshalErr != nil {
			return domainToken{}, marshalErr
		}
		return newDomainToken(text)
	})
	if err != nil {
		return domainToken{}, contractError(err)
	}
	return token, nil
}

func parseCanonicalDomain[D SigningDomain[D]](token domainToken) (D, error) {
	var zero D
	if err := token.Validate(); err != nil {
		return zero, err
	}
	text := append([]byte(nil), token.text[:token.length]...)
	domain, err := guardedCall(func() (D, error) {
		return zero.ParseCanonicalText(text)
	})
	if err != nil {
		return zero, contractError(err)
	}
	projected, err := canonicalDomain(domain)
	if err != nil {
		return zero, err
	}
	if projected != token {
		return zero, contractError(errors.New(domainCanonicalErrorText))
	}
	return domain, nil
}

func validDomainText(text []byte) bool {
	if len(text) == 0 || len(text) > SigningDomainMaximumBytes {
		return false
	}
	if !domainAlphaNumeric(text[0]) || !domainAlphaNumeric(text[len(text)-1]) {
		return false
	}
	previousHyphen := false
	for _, value := range text {
		if domainAlphaNumeric(value) {
			previousHyphen = false
			continue
		}
		if value != '-' || previousHyphen {
			return false
		}
		previousHyphen = true
	}
	return true
}

func domainAlphaNumeric(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= '0' && value <= '9'
}
