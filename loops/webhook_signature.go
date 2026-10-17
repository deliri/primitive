package loops

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// signatureDigestTextBytes is the strict padded base64 spelling of one
// HMAC-SHA256 digest.
const signatureDigestTextBytes = (sha256.Size + 2) / 3 * 4

// DeliveryID is the Standard Webhooks webhook-id: one provider delivery
// identity for caller-owned idempotency. It is bounded printable ASCII and
// excludes the signed-content separator so no id can absorb a timestamp.
// https://loops.so/docs/webhooks
type DeliveryID struct{ value string }

func ParseDeliveryID(value string) (DeliveryID, error) {
	candidate := DeliveryID{value: value}
	if err := candidate.Validate(); err != nil {
		return DeliveryID{}, err
	}
	return candidate, nil
}

func (d DeliveryID) Validate() error {
	if !validVisibleASCII([]byte(d.value), 1, core.LoopsWebhookIDMaximumBytes) ||
		strings.Contains(d.value, core.LoopsWebhookSignedContentSeparator) {
		return core.ErrLoopsVerification
	}
	return nil
}
func (d DeliveryID) String() string { return d.value }

// parseSignatureTimestamp admits canonical unsigned decimal Unix seconds whose
// nanosecond projection fits a temporal Instant. The signed content uses the
// exact header text, so non-canonical spellings are refused, not normalized.
func parseSignatureTimestamp(text string) (temporal.Instant, error) {
	if text == "" || len(text) > core.LoopsWebhookTimestampMaximumBytes || text[0] == '0' && len(text) != 1 {
		return temporal.Instant{}, core.ErrLoopsVerification
	}
	for index := range len(text) {
		if text[index] < '0' || text[index] > '9' {
			return temporal.Instant{}, core.ErrLoopsVerification
		}
	}
	seconds, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return temporal.Instant{}, verificationError(err)
	}
	// Temporal owns the signed-nanosecond projection and its overflow refusal.
	signed, err := temporal.InstantFromUnixSeconds(seconds)
	if err != nil {
		return temporal.Instant{}, verificationError(err)
	}
	return signed, nil
}

// validateSignatureTime admits a signed instant within the inclusive
// Standard Webhooks tolerance of the caller-observed instant, in either
// direction. No wall clock is read here.
func validateSignatureTime(signed, observed temporal.Instant) error {
	tolerance, err := temporal.DurationFromSeconds(core.LoopsWebhookToleranceSeconds)
	if err != nil {
		return contractError(err)
	}
	comparison, err := observed.Compare(signed)
	if err != nil {
		return verificationError(err)
	}
	var age temporal.Duration
	if comparison == core.ComparisonLess {
		age, err = signed.Since(observed)
	} else {
		age, err = observed.Since(signed)
	}
	if err != nil {
		return verificationError(err)
	}
	order, err := age.Compare(tolerance)
	if err != nil || order == core.ComparisonGreater {
		return verificationError(err)
	}
	return nil
}

// validateSignatureHeader checks the whole space-separated header before any
// body byte is read. Every entry must be version,value; unknown versions are
// ignored, but each v1 value must be a strict digest spelling and at least one
// v1 entry must exist. Nothing is retained across entries.
func validateSignatureHeader(header string) error {
	versioned := false
	for entry := range strings.SplitSeq(header, core.LoopsWebhookSignatureEntrySeparator) {
		version, encoded, ok := strings.Cut(entry, core.LoopsWebhookSignatureVersionSeparator)
		if !ok || version == "" || encoded == "" {
			return core.ErrLoopsVerification
		}
		if version != core.LoopsWebhookSignatureVersion {
			continue
		}
		var digest [sha256.Size]byte
		if !decodeSignatureDigest(encoded, &digest) {
			return core.ErrLoopsVerification
		}
		versioned = true
	}
	if !versioned {
		return core.ErrLoopsVerification
	}
	return nil
}

// verifySignatureDigest accepts when any v1 entry equals expected. During a
// secret rotation Loops may send one entry per valid secret; every entry is
// compared in constant time and no early exit reveals which matched.
func verifySignatureDigest(header string, expected []byte) error {
	matched := 0
	for entry := range strings.SplitSeq(header, core.LoopsWebhookSignatureEntrySeparator) {
		version, encoded, _ := strings.Cut(entry, core.LoopsWebhookSignatureVersionSeparator)
		if version != core.LoopsWebhookSignatureVersion {
			continue
		}
		var candidate [sha256.Size]byte
		if !decodeSignatureDigest(encoded, &candidate) {
			return core.ErrLoopsVerification
		}
		matched |= subtle.ConstantTimeCompare(candidate[:], expected)
	}
	if matched != 1 {
		return core.ErrLoopsVerification
	}
	return nil
}

func decodeSignatureDigest(encoded string, digest *[sha256.Size]byte) bool {
	if len(encoded) != signatureDigestTextBytes {
		return false
	}
	var text [signatureDigestTextBytes]byte
	copy(text[:], encoded)
	var decoded [signatureDigestTextBytes]byte
	count, err := base64.StdEncoding.Strict().Decode(decoded[:], text[:])
	if err != nil || count != sha256.Size {
		return false
	}
	copy(digest[:], decoded[:count])
	return true
}

func (DeliveryID) loopsProtocolFact() {}

var (
	_ core.Validatable = DeliveryID{}
	_ protocolFact     = DeliveryID{}
)
