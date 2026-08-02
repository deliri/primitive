package exchange

import "github.com/deliri/primitive/v2026/core"

const (
	contentCodingIdentityText  = "identity"
	contentCodingGzipText      = "gzip"
	contentCodingBrotliText    = "br"
	contentCodingZstandardText = "zstd"
)

// StandardContentCoding is Exchange's closed set of compiler-owned HTTP
// content codings. Product-specific transfer codings remain caller-owned.
type StandardContentCoding uint8

const (
	StandardContentCodingUnknown StandardContentCoding = iota
	StandardContentCodingIdentity
	StandardContentCodingGzip
	StandardContentCodingBrotli
	StandardContentCodingZstandard
	standardContentCodingLimit
)

func standardContentCodingFacts() [standardContentCodingLimit]string {
	return [...]string{
		StandardContentCodingUnknown:   "",
		StandardContentCodingIdentity:  contentCodingIdentityText,
		StandardContentCodingGzip:      contentCodingGzipText,
		StandardContentCodingBrotli:    contentCodingBrotliText,
		StandardContentCodingZstandard: contentCodingZstandardText,
	}
}

// Validate rejects values outside the closed standard-content-coding domain.
func (c StandardContentCoding) Validate() error {
	if c <= StandardContentCodingUnknown || c >= standardContentCodingLimit ||
		standardContentCodingFacts()[c] == "" {
		return core.ErrExchangeContract
	}
	return nil
}

// IsValid reports whether c names one admitted standard content coding.
func (c StandardContentCoding) IsValid() bool { return c.Validate() == nil }

// OffWireEnum marks the domain as an execution fact, not a wire value.
func (StandardContentCoding) OffWireEnum() {}

// String returns the canonical lower-case coding or empty text when invalid.
func (c StandardContentCoding) String() string {
	if c >= standardContentCodingLimit {
		return ""
	}
	return standardContentCodingFacts()[c]
}

var _ core.OffWireEnum = StandardContentCodingUnknown
