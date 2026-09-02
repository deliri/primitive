package exchange

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

const plainTextMediaTypeText = "text/plain"

// StandardMediaType is Exchange's closed set of compiler-owned standard
// representation facts. Product-specific vendor types remain caller-owned.
type StandardMediaType uint8

const (
	StandardMediaTypeUnknown StandardMediaType = iota
	StandardMediaTypeJSON
	StandardMediaTypePlainText
	standardMediaTypeLimit
)

func standardMediaTypeFacts() [standardMediaTypeLimit]string {
	return [...]string{
		StandardMediaTypeUnknown:   "",
		StandardMediaTypeJSON:      core.HTTPMediaTypeJSON().String(),
		StandardMediaTypePlainText: plainTextMediaTypeText,
	}
}

// Validate rejects values outside the closed standard-media domain.
func (m StandardMediaType) Validate() error {
	if m <= StandardMediaTypeUnknown || m >= standardMediaTypeLimit ||
		standardMediaTypeFacts()[m] == "" {
		return core.ErrExchangeContract
	}
	return nil
}

// IsValid reports whether m names one admitted standard representation.
func (m StandardMediaType) IsValid() bool { return m.Validate() == nil }

// OffWireEnum marks the domain as an execution fact, not a wire value.
func (StandardMediaType) OffWireEnum() {}

// String returns the canonical media type or empty text when invalid.
func (m StandardMediaType) String() string {
	if m >= standardMediaTypeLimit {
		return ""
	}
	return standardMediaTypeFacts()[m]
}

// HTTPMediaType projects one validated standard representation into Core's
// general media-type contract.
func (m StandardMediaType) HTTPMediaType() (core.HTTPMediaType, error) {
	if err := m.Validate(); err != nil {
		return core.HTTPMediaType{}, err
	}
	mediaType, err := core.ParseHTTPMediaType(m.String())
	if err != nil {
		return core.HTTPMediaType{}, errors.Join(core.ErrExchangeContract, err)
	}
	return mediaType, nil
}

var _ core.OffWireEnum = StandardMediaTypeUnknown
