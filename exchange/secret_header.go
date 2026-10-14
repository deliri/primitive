package exchange

import (
	"fmt"
	"io"
	"net/http"

	"github.com/deliri/primitive/v2026/core"
)

// SecretHeaderScheme is an off-wire choice of native header encoding.
type SecretHeaderScheme uint8

const (
	SecretHeaderUnknown SecretHeaderScheme = iota
	SecretHeaderLiteral
	SecretHeaderBearer
)

func (SecretHeaderScheme) OffWireEnum() {}

func (s SecretHeaderScheme) Validate() error {
	switch s {
	case SecretHeaderLiteral, SecretHeaderBearer:
		return nil
	default:
		return core.ErrExchangeContract
	}
}

// SecretHeader borrows secret bytes until the native request finishes. Unlike
// HeaderValue, it never retains a string copy. net/http requires strings at its
// wire boundary; those native copies are immutable and cannot be zeroed by Go.
type SecretHeader struct {
	Name   core.HTTPHeaderName
	Value  []byte
	Scheme SecretHeaderScheme
}

func (h SecretHeader) Validate() error {
	if h.Name.Validate() != nil || managedHeader(h.Name) {
		return core.ErrExchangeContract
	}
	switch h.Scheme {
	case SecretHeaderBearer:
		if h.Name != coreHeaderAuthorization() {
			return core.ErrExchangeContract
		}
		return (BearerAuthorization{Token: h.Value}).Validate()
	case SecretHeaderLiteral:
		if len(h.Value) == 0 || len(h.Value) > HeaderValueMaximumBytes {
			return core.ErrExchangeContract
		}
		for _, value := range h.Value {
			if value < 0x21 || value > 0x7e {
				return core.ErrExchangeContract
			}
		}
		return nil
	default:
		return core.ErrExchangeContract
	}
}

func coreHeaderAuthorization() core.HTTPHeaderName {
	name, _ := StandardHeaderAuthorization.Name()
	return name
}

func (SecretHeader) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

func validateSecretHeaders(headers []SecretHeader, ordinary Headers) error {
	if len(headers)+len(ordinary.Values) > HeaderMaximumCount {
		return core.ErrExchangeContract
	}
	for index, header := range headers {
		if err := header.Validate(); err != nil {
			return err
		}
		for prior := range index {
			if headers[prior].Name == header.Name {
				return core.ErrExchangeContract
			}
		}
		for _, other := range ordinary.Values {
			if other.Name == header.Name {
				return core.ErrExchangeContract
			}
		}
	}
	return nil
}

func applySecretHeaders(request *http.Request, headers []SecretHeader) {
	for _, header := range headers {
		switch header.Scheme {
		case SecretHeaderLiteral:
			request.Header.Set(header.Name.String(), string(header.Value))
		case SecretHeaderBearer:
			request.Header.Set(header.Name.String(), BearerAuthorizationScheme+" "+string(header.Value))
		default:
			panic(core.ErrExchangeContract) // Preflight validation excludes this arm.
		}
	}
}
