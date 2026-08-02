package exchange

import (
	"errors"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

const (
	identityContentCodingText = "identity"
	retryAfterHeaderName      = "Retry-After"
)

type httpContentCoding struct {
	value string
}

func parseHTTPContentCoding(value string) (httpContentCoding, error) {
	if _, err := core.ParseHTTPHeaderName(value); err != nil {
		return httpContentCoding{}, errors.Join(core.ErrExchangeContract, err)
	}
	return httpContentCoding{value: strings.ToLower(value)}, nil
}

func (c httpContentCoding) Validate() error {
	parsed, err := parseHTTPContentCoding(c.value)
	if err != nil || parsed != c {
		return errors.Join(core.ErrExchangeContract, err)
	}
	return nil
}

func (c httpContentCoding) String() string { return c.value }

func identityContentCoding() httpContentCoding {
	return httpContentCoding{value: identityContentCodingText}
}
