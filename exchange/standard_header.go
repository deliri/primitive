package exchange

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

const (
	authorizationHeaderNameText = "Authorization"
	cacheControlHeaderNameText  = "Cache-Control"
	retryAfterHeaderNameText    = "Retry-After"
)

// StandardHeader is Exchange's closed set of compiler-owned standard fields
// not already projected by Core's message-framing helpers.
type StandardHeader uint8

const (
	StandardHeaderUnknown StandardHeader = iota
	StandardHeaderAuthorization
	StandardHeaderCacheControl
	StandardHeaderRetryAfter
	standardHeaderLimit
)

func standardHeaderFacts() [standardHeaderLimit]string {
	return [...]string{
		StandardHeaderUnknown:       "",
		StandardHeaderAuthorization: authorizationHeaderNameText,
		StandardHeaderCacheControl:  cacheControlHeaderNameText,
		StandardHeaderRetryAfter:    retryAfterHeaderNameText,
	}
}

// Validate rejects values outside the closed standard-header domain.
func (h StandardHeader) Validate() error {
	if h <= StandardHeaderUnknown || h >= standardHeaderLimit ||
		standardHeaderFacts()[h] == "" {
		return core.ErrExchangeContract
	}
	return nil
}

// IsValid reports whether h names one admitted standard field.
func (h StandardHeader) IsValid() bool { return h.Validate() == nil }

// OffWireEnum marks the domain as an execution fact, not a wire value.
func (StandardHeader) OffWireEnum() {}

// String returns the canonical field name or empty text when invalid.
func (h StandardHeader) String() string {
	if h >= standardHeaderLimit {
		return ""
	}
	return standardHeaderFacts()[h]
}

// Name projects one validated field into Core's general header-name contract.
func (h StandardHeader) Name() (core.HTTPHeaderName, error) {
	if err := h.Validate(); err != nil {
		return core.HTTPHeaderName{}, err
	}
	name, err := core.ParseHTTPHeaderName(h.String())
	if err != nil {
		return core.HTTPHeaderName{}, errors.Join(core.ErrExchangeContract, err)
	}
	return name, nil
}

var _ core.OffWireEnum = StandardHeaderUnknown
