package timeproof

import (
	"github.com/deliri/primitive/v2026/core"
)

const (
	requestMediaTypeText  = "application/timestamp-query"
	responseMediaTypeText = "application/timestamp-reply"
)

// MediaType is the complete RFC 3161 HTTP representation domain.
type MediaType uint8

const (
	MediaTypeUnknown MediaType = iota
	MediaTypeRequest
	MediaTypeResponse
	mediaTypeLimit
)

func mediaTypeFacts() [mediaTypeLimit]string {
	return [...]string{
		MediaTypeUnknown:  "",
		MediaTypeRequest:  requestMediaTypeText,
		MediaTypeResponse: responseMediaTypeText,
	}
}

// Validate rejects values outside the closed RFC 3161 representation domain.
func (m MediaType) Validate() error {
	if m <= MediaTypeUnknown || m >= mediaTypeLimit || mediaTypeFacts()[m] == "" {
		return contractError(nil)
	}
	return nil
}

// IsValid reports whether m names one RFC 3161 representation.
func (m MediaType) IsValid() bool { return m.Validate() == nil }

// OffWireEnum marks MediaType as a local execution fact; the projected media
// type, not this enum, crosses HTTP.
func (MediaType) OffWireEnum() {}

// String returns the canonical representation or empty text when invalid.
func (m MediaType) String() string {
	if m >= mediaTypeLimit {
		return ""
	}
	return mediaTypeFacts()[m]
}

// HTTPMediaType projects one validated RFC 3161 representation into Core's
// general media-type contract.
func (m MediaType) HTTPMediaType() (core.HTTPMediaType, error) {
	if err := m.Validate(); err != nil {
		return core.HTTPMediaType{}, err
	}
	mediaType, err := core.ParseHTTPMediaType(m.String())
	if err != nil {
		return core.HTTPMediaType{}, contractError(err)
	}
	return mediaType, nil
}

var _ core.OffWireEnum = MediaTypeUnknown
