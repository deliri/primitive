package exchange

import (
	"errors"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

// CookieName is one case-sensitive request-cookie selector.
type CookieName struct{ value string }

// ParseCookieName admits the standard library's cookie-name grammar.
func ParseCookieName(value string) (CookieName, error) {
	name := CookieName{value: value}
	if err := name.Validate(); err != nil {
		return CookieName{}, err
	}
	return name, nil
}

// String returns the exact case-sensitive selector.
func (n CookieName) String() string { return n.value }

// Validate rejects an unset or malformed cookie selector.
func (n CookieName) Validate() error {
	if err := (&http.Cookie{Name: n.value}).Valid(); err != nil {
		return errors.Join(core.ErrExchangeContract, err)
	}
	return nil
}

// QueryParameterName selects one decoded, case-sensitive query parameter.
type QueryParameterName struct{ value string }

// ParseQueryParameterName admits a decoded UTF-8 name without control bytes.
func ParseQueryParameterName(value string) (QueryParameterName, error) {
	name := QueryParameterName{value: value}
	if err := name.Validate(); err != nil {
		return QueryParameterName{}, err
	}
	return name, nil
}

// String returns the exact decoded selector.
func (n QueryParameterName) String() string { return n.value }

// Validate rejects an unset or malformed query selector.
func (n QueryParameterName) Validate() error {
	if n.value == "" || !utf8.ValidString(n.value) {
		return core.ErrExchangeContract
	}
	for _, character := range n.value {
		if character < 0x20 || character == 0x7f {
			return core.ErrExchangeContract
		}
	}
	return nil
}

// ScalarValue preserves the difference between absence and a present empty
// value. A refusal always returns its zero value, never a partial observation.
type ScalarValue struct {
	value   string
	present bool
}

// Present distinguishes an observed empty scalar from absence.
func (v ScalarValue) Present() bool { return v.present }

// Value returns the selected decoded data; absence returns an empty string.
func (v ScalarValue) Value() string { return v.value }

func (c SocketServerCall) scalarLimit(maximum core.ByteCount) (int64, error) {
	if err := c.Validate(); err != nil {
		return 0, err
	}
	if err := exchangeContextError(c.request.Context()); err != nil {
		return 0, err
	}
	limit, err := maximum.Int64()
	if err != nil {
		return 0, errors.Join(core.ErrExchangeContract, err)
	}
	return limit, nil
}

// UniqueCookie scans every Cookie field forward, retaining only the selected
// scalar. Parsing one bounded pair at a time keeps Go's native cookie grammar
// without materializing a request-sized cookie collection. Unrelated cookies
// do not become domain inputs; duplicates of the selected name are refused.
func (c SocketServerCall) UniqueCookie(name CookieName, maximum core.ByteCount) (ScalarValue, error) {
	limit, err := c.scalarLimit(maximum)
	if err != nil {
		return ScalarValue{}, err
	}
	if err := name.Validate(); err != nil {
		return ScalarValue{}, err
	}
	var selected ScalarValue
	for _, line := range c.request.Header.Values("Cookie") {
		for pair := range strings.SplitSeq(line, ";") {
			if err := exchangeContextError(c.request.Context()); err != nil {
				return ScalarValue{}, err
			}
			pair = textproto.TrimString(pair)
			key, encoded, _ := strings.Cut(pair, "=")
			if textproto.TrimString(key) != name.value {
				continue
			}
			valueBytes := len(encoded)
			if valueBytes >= 2 && encoded[0] == '"' && encoded[valueBytes-1] == '"' {
				valueBytes -= 2
			}
			if selected.present || int64(valueBytes) > limit {
				return ScalarValue{}, core.ErrExchangeContract
			}
			cookies, parseErr := http.ParseCookie(pair)
			if parseErr != nil {
				return ScalarValue{}, errors.Join(core.ErrExchangeContract, parseErr)
			}
			if len(cookies) != 1 || int64(len(cookies[0].Value)) > limit {
				return ScalarValue{}, core.ErrExchangeContract
			}
			selected = ScalarValue{value: cookies[0].Value, present: true}
		}
	}
	if err := exchangeContextError(c.request.Context()); err != nil {
		return ScalarValue{}, err
	}
	// Native scalar parsers can return a substring of the request storage.
	// Retain only the bounded selected bytes after the request is released.
	selected.value = strings.Clone(selected.value)
	return selected, nil
}

// UniqueQuery scans the already-bound request target without url.Values or a
// query map. Only a possibly matching name and the selected value are decoded;
// their encoded extents are checked before the native decoder can allocate.
func (c SocketServerCall) UniqueQuery(name QueryParameterName, maximum core.ByteCount) (ScalarValue, error) {
	limit, err := c.scalarLimit(maximum)
	if err != nil {
		return ScalarValue{}, err
	}
	if err := name.Validate(); err != nil {
		return ScalarValue{}, err
	}
	if c.request.URL == nil {
		return ScalarValue{}, core.ErrExchangeContract
	}
	var selected ScalarValue
	for field := range strings.SplitSeq(c.request.URL.RawQuery, "&") {
		if err := exchangeContextError(c.request.Context()); err != nil {
			return ScalarValue{}, err
		}
		key, encoded, _ := strings.Cut(field, "=")
		// A percent-encoded name has at most three wire bytes per decoded byte.
		// Division avoids overflow for a caller-owned, unusually long selector.
		if !queryEncodedExtentFits(len(key), int64(len(name.value))) {
			continue
		}
		decoded, nameErr := url.QueryUnescape(key)
		if nameErr != nil {
			return ScalarValue{}, errors.Join(core.ErrExchangeContract, nameErr)
		}
		if decoded != name.value {
			continue
		}
		if selected.present || strings.Contains(field, ";") || !queryEncodedExtentFits(len(encoded), limit) {
			return ScalarValue{}, core.ErrExchangeContract
		}
		value, valueErr := url.QueryUnescape(encoded)
		if valueErr != nil {
			return ScalarValue{}, errors.Join(core.ErrExchangeContract, valueErr)
		}
		if int64(len(value)) > limit {
			return ScalarValue{}, core.ErrExchangeContract
		}
		selected = ScalarValue{value: value, present: true}
	}
	if err := exchangeContextError(c.request.Context()); err != nil {
		return ScalarValue{}, err
	}
	selected.value = strings.Clone(selected.value)
	return selected, nil
}

func queryEncodedExtentFits(encoded int, decodedLimit int64) bool {
	minimum := int64(encoded / 3)
	if encoded%3 != 0 {
		minimum++
	}
	return minimum <= decodedLimit
}
