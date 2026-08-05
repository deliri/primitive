package controlplane

import (
	"crypto/sha256"
	"encoding/json"
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
	// usageWatermarkChainDomain separates chain digests likewise.
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
	digest := usageDomainDigest(usageWatermarkGenesisDomain, encoded)
	watermark := UsageWatermark{
		Subject: subject, Generation: generation,
		WindowDigest: core.NewSHA256Digest(digest),
		ChainDigest:  core.NewSHA256Digest(digest),
	}
	return watermark, watermark.Validate()
}

// AdvanceUsageWatermark accepts one exact canonical usage window and returns
// the next watermark in the chain.
//
// The caller owns validating the window itself. What this owns is that the
// sequence cannot go backward, cannot skip, and cannot wrap: a saturated
// generation is refused rather than rolled over to one, which would make an
// installation's oldest accepted window indistinguishable from its newest.
func AdvanceUsageWatermark(current UsageWatermark, canonicalWindow []byte) (UsageWatermark, error) {
	if err := current.Validate(); err != nil {
		return UsageWatermark{}, usageWatermarkError(err)
	}
	if len(canonicalWindow) == 0 {
		return UsageWatermark{}, usageWatermarkError()
	}
	generation, err := nextUsageGeneration(current.Generation)
	if err != nil {
		return UsageWatermark{}, err
	}
	previousChain, err := current.ChainDigest.Bytes()
	if err != nil {
		return UsageWatermark{}, usageWatermarkError(err)
	}
	windowDigest := usageDomainDigest(usageWatermarkChainDomain, canonicalWindow)
	chainInput := make([]byte, 0, len(previousChain)+len(windowDigest))
	chainInput = append(chainInput, previousChain[:]...)
	chainInput = append(chainInput, windowDigest[:]...)
	next := UsageWatermark{
		Subject: current.Subject, Generation: generation,
		WindowDigest: core.NewSHA256Digest(windowDigest),
		ChainDigest:  core.NewSHA256Digest(usageDomainDigest(usageWatermarkChainDomain, chainInput)),
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

// usageDomainDigest binds a digest to one exact domain, so a window digest can
// never be presented as a chain digest.
func usageDomainDigest(domain string, body []byte) [sha256.Size]byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte(domain))
	_, _ = hash.Write([]byte{usageDigestDomainSeparator})
	_, _ = hash.Write(body)
	var digest [sha256.Size]byte
	copy(digest[:], hash.Sum(nil))
	return digest
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
