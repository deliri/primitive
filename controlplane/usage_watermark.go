package controlplane

import (
	json "encoding/json/v2"
	"math"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
)

const (
	// UsageWatermarkJSONMaximumBytes bounds an accepted watermark document.
	UsageWatermarkJSONMaximumBytes = 8 << 10
	// UsageWatermarkInitialGeneration is the generation of a subject that has
	// no accepted usage window yet.
	UsageWatermarkInitialGeneration = uint64(1)
	// usageWatermarkGenesisDomain separates the genesis digest from every other
	// digest this package computes.
	usageWatermarkGenesisDomain = "ogs-control-usage-genesis-2026-1"
	// usageWatermarkWindowDomain binds a digest to one accepted window's
	// canonical bytes, and to nothing else.
	usageWatermarkWindowDomain = "ogs-control-usage-window-2026-1"
	// usageWatermarkChainDomain binds a digest to one chain link. Three
	// distinct domains are the whole point: a genesis, a window, and a chain
	// digest can never be presented as one another, no matter what bytes they
	// were computed over.
	usageWatermarkChainDomain = "ogs-control-usage-chain-2026-1"
	// usageDigestDomainSeparator terminates the domain text so no domain can be
	// a prefix of another domain followed by body bytes.
	usageDigestDomainSeparator = byte(0)
)

// UsageWatermark is the fixed-size accepted-usage high-water fact for one exact
// entitlement and registered device.
//
// The Lease subject is part of the fact rather than context around it, so one
// installation's sequence cannot be replayed as another's. Receipt owns a
// watermark of the same shape scoped to an account and offering; this one is
// scoped per installation and orders accepted usage windows rather than
// accepted evidence.
type UsageWatermark struct {
	Subject      lease.Subject     `json:"subject"`
	Generation   lease.Generation  `json:"generation"`
	WindowDigest core.SHA256Digest `json:"window_digest"`
	ChainDigest  core.SHA256Digest `json:"chain_digest"`
}

type usageWatermarkWire UsageWatermark

// Validate closes identity, generation, and both digest facts.
func (w UsageWatermark) Validate() error {
	if err := w.Subject.Validate(); err != nil {
		return usageWatermarkError(err)
	}
	if err := w.Generation.Validate(); err != nil {
		return usageWatermarkError(err)
	}
	if err := w.WindowDigest.Validate(); err != nil {
		return usageWatermarkError(err)
	}
	if err := w.ChainDigest.Validate(); err != nil {
		return usageWatermarkError(err)
	}
	return nil
}

// NewInitialUsageWatermark creates the only admitted starting point for one
// subject. Generation one represents no accepted usage window yet.
func NewInitialUsageWatermark(subject lease.Subject) (UsageWatermark, error) {
	if err := subject.Validate(); err != nil {
		return UsageWatermark{}, usageWatermarkError(err)
	}
	encoded, err := json.Marshal(subject)
	if err != nil {
		return UsageWatermark{}, usageWatermarkError(err)
	}
	generation, err := lease.NewGeneration(UsageWatermarkInitialGeneration)
	if err != nil {
		return UsageWatermark{}, usageWatermarkError(err)
	}
	digest, err := usageDomainDigest(usageDigestRequest{
		domain: usageWatermarkGenesisDomain,
		first:  encoded,
	})
	if err != nil {
		return UsageWatermark{}, usageWatermarkError(err)
	}
	watermark := UsageWatermark{
		Subject: subject, Generation: generation,
		WindowDigest: digest,
		ChainDigest:  digest,
	}
	return watermark, watermark.Validate()
}

// AdvanceUsageWatermark accepts one typed usage window and returns the next
// watermark in the chain.
//
// The window arrives as the type that owns it, not as bytes: this door derives
// the one canonical form itself, so a caller can neither hand it a document
// that never validated nor smuggle arbitrary bytes into the chain. What this
// owns beyond that is that the sequence cannot go backward, cannot skip, and
// cannot wrap: a saturated generation is refused rather than rolled over to
// one, which would make an installation's oldest accepted window
// indistinguishable from its newest.
func AdvanceUsageWatermark(current UsageWatermark, window UsageWindow) (UsageWatermark, error) {
	if err := current.Validate(); err != nil {
		return UsageWatermark{}, usageWatermarkError(err)
	}
	canonicalWindow, err := window.MarshalJSON()
	if err != nil {
		return UsageWatermark{}, usageWatermarkError(err)
	}
	generation, err := nextUsageGeneration(current.Generation)
	if err != nil {
		return UsageWatermark{}, err
	}
	previousChain, err := current.ChainDigest.Bytes()
	if err != nil {
		return UsageWatermark{}, usageWatermarkError(err)
	}
	windowDigest, err := usageDomainDigest(usageDigestRequest{
		domain: usageWatermarkWindowDomain,
		first:  canonicalWindow,
	})
	if err != nil {
		return UsageWatermark{}, usageWatermarkError(err)
	}
	windowBytes, err := windowDigest.Bytes()
	if err != nil {
		return UsageWatermark{}, usageWatermarkError(err)
	}
	chainDigest, err := usageDomainDigest(usageDigestRequest{
		domain: usageWatermarkChainDomain,
		first:  previousChain[:],
		second: windowBytes[:],
	})
	if err != nil {
		return UsageWatermark{}, usageWatermarkError(err)
	}
	next := UsageWatermark{
		Subject: current.Subject, Generation: generation,
		WindowDigest: windowDigest,
		ChainDigest:  chainDigest,
	}
	return next, next.Validate()
}

// nextUsageGeneration refuses to wrap a saturated counter.
func nextUsageGeneration(current lease.Generation) (lease.Generation, error) {
	value, err := current.Uint64()
	if err != nil {
		return lease.Generation{}, usageWatermarkError(err)
	}
	if value == math.MaxUint64 {
		return lease.Generation{}, usageWatermarkError()
	}
	generation, err := lease.NewGeneration(value + 1)
	if err != nil {
		return lease.Generation{}, usageWatermarkError(err)
	}
	return generation, nil
}

// usageDomainDigest binds a digest to one exact domain, so a genesis, window,
// or chain digest can never be presented as one of the others. The bounded
// input is assembled once and hashed through Core's one whole-buffer door.
type usageDigestRequest struct {
	domain string
	first  []byte
	second []byte
}

func usageDomainDigest(request usageDigestRequest) (core.SHA256Digest, error) {
	writer := core.NewDigestWriter()
	if _, err := writer.Write([]byte(request.domain)); err != nil {
		return core.SHA256Digest{}, err
	}
	if _, err := writer.Write([]byte{usageDigestDomainSeparator}); err != nil {
		return core.SHA256Digest{}, err
	}
	if _, err := writer.Write(request.first); err != nil {
		return core.SHA256Digest{}, err
	}
	if _, err := writer.Write(request.second); err != nil {
		return core.SHA256Digest{}, err
	}
	digest, _, err := writer.Seal()
	return digest, err
}

// MarshalJSON emits the complete validated watermark.
func (w UsageWatermark) MarshalJSON() ([]byte, error) {
	if err := w.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(usageWatermarkWire(w))
	if err != nil || len(encoded) > UsageWatermarkJSONMaximumBytes {
		return nil, jsonError(usageWatermarkError(err))
	}
	return encoded, nil
}

// UnmarshalJSON strictly decodes without mutating the receiver on rejection.
func (w *UsageWatermark) UnmarshalJSON(data []byte) error {
	if w == nil {
		return jsonError(usageWatermarkError())
	}
	limits, err := documentJSONLimits(UsageWatermarkJSONMaximumBytes)
	if err != nil {
		return jsonError(usageWatermarkError(err))
	}
	wire, err := core.DecodeStrictJSONStructure[usageWatermarkWire](data, limits)
	if err != nil {
		return jsonError(usageWatermarkError(err))
	}
	candidate := UsageWatermark(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*w = candidate
	return nil
}

var (
	_ core.Validatable = UsageWatermark{}
	_ json.Marshaler   = UsageWatermark{}
	_ json.Unmarshaler = (*UsageWatermark)(nil)
)
