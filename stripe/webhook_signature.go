package stripe

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

type webhookSignatureField uint8

const (
	webhookSignatureIgnored webhookSignatureField = iota
	webhookSignatureTimestamp
	webhookSignatureDigest
)

func classifyWebhookSignatureField(name string) webhookSignatureField {
	switch name {
	case core.StripeWebhookTimestampField:
		return webhookSignatureTimestamp
	case core.StripeWebhookDigestField:
		return webhookSignatureDigest
	default:
		return webhookSignatureIgnored
	}
}

func signatureSeconds(header string) (int64, error) {
	var seconds int64
	found := false
	for member := range strings.SplitSeq(header, ",") {
		name, value, ok := strings.Cut(member, "=")
		if !ok || strings.Contains(value, "=") {
			return 0, core.ErrStripeVerification
		}
		if classifyWebhookSignatureField(name) != webhookSignatureTimestamp {
			continue
		}
		if found {
			return 0, core.ErrStripeVerification
		}
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return 0, verificationError(err)
		}
		seconds, found = parsed, true
	}
	if !found {
		return 0, core.ErrStripeVerification
	}
	return seconds, nil
}

func verifySignatureDigest(header string, expected []byte) error {
	matched := 0
	for member := range strings.SplitSeq(header, ",") {
		name, value, _ := strings.Cut(member, "=")
		if classifyWebhookSignatureField(name) != webhookSignatureDigest || len(value) != hex.EncodedLen(sha256.Size) {
			continue
		}
		var candidate [sha256.Size]byte
		var encoded [sha256.Size * 2]byte
		// Decode the borrowed public header into fixed scratch. No secret string
		// or collection of rotation signatures survives this iteration.
		copy(encoded[:], value)
		if _, err := hex.Decode(candidate[:], encoded[:]); err != nil {
			continue
		}
		matched |= subtle.ConstantTimeCompare(candidate[:], expected)
	}
	if matched != 1 {
		return core.ErrStripeVerification
	}
	return nil
}
