package id

import (
	"encoding/binary"
	"math"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	// identityBytes is the exact extent of both identity values.
	identityBytes = 16
	// entropyBytes is the entropy extent both identities consume from the
	// request's material.
	entropyBytes = 10
	// timestampBytes is the extent of the leading big-endian millisecond
	// stamp both identities carry, so byte order is time order.
	timestampBytes = 6
	// timestampMaximumMilliseconds is the largest instant forty-eight
	// big-endian bits can carry.
	timestampMaximumMilliseconds = uint64(1<<48 - 1)
)

// Temporal's exact int64 nanosecond domain projects to far fewer than
// forty-eight bits of milliseconds, so no valid observation can overflow the
// stamp. This witness breaks the build if either side ever widens, which is
// where that fact belongs: a runtime ceiling branch here would be a branch no
// caller can exercise.
var _ [timestampMaximumMilliseconds - math.MaxInt64/temporal.NanosecondsPerMillisecond]struct{}

// Request carries the two facts every time-ordered identity is built from:
// the one authoritative wall reading and the caller-drawn entropy.
// Observation is the clock fact temporal already observed; Entropy remains
// owned by the caller and is copied only for one bounded construction.
// Both identities consume exactly the first ten entropy bytes; the
// remaining six of the minimum draw are never read.
type Request struct {
	Entropy     core.SecretMaterial
	Observation temporal.Observation
}

// Validate rejects an observation whose wall projection is invalid and
// entropy that is not exactly the keygen minimum secret extent, so every
// request in the fleet has one spelling: temporal.Observe beside
// keygen.GenerateSecret of core.SecretMaterialMinimumBytes.
func (r Request) Validate() error {
	if err := r.Observation.Validate(); err != nil {
		return contractCause("request observation is invalid", err)
	}
	if err := r.Entropy.Validate(); err != nil {
		return contractCause("request entropy is invalid", err)
	}
	count, err := r.Entropy.ByteCount()
	if err != nil {
		return contractCause("request entropy extent is unreadable", err)
	}
	bytes, err := count.Uint64()
	if err != nil || bytes != core.SecretMaterialMinimumBytes {
		return contractCause("request entropy is not the minimum secret extent", err)
	}
	return nil
}

var _ core.Validatable = Request{}

// observedMilliseconds projects the request's wall reading to the
// forty-eight bit millisecond domain both identities carry.
func observedMilliseconds(observation temporal.Observation) (uint64, error) {
	instant, err := observation.Instant()
	if err != nil {
		return 0, contractCause("request instant is invalid", err)
	}
	nanoseconds, err := instant.Nanoseconds()
	if err != nil {
		return 0, contractCause("request instant is unreadable", err)
	}
	if nanoseconds < 0 {
		return 0, contractError("request instant precedes the epoch")
	}
	return uint64(nanoseconds) / temporal.NanosecondsPerMillisecond, nil
}

// putTimestamp writes milliseconds into the first six destination bytes in
// big-endian order.
func putTimestamp(destination []byte, milliseconds uint64) {
	var stamp [8]byte
	binary.BigEndian.PutUint64(stamp[:], milliseconds)
	copy(destination[:timestampBytes], stamp[2:])
}
