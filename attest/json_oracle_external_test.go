package attest_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

// nestedJSONAdmitted uses the standard decoder and a bounded recursive walk.
// It does not call Core's production scanner. Pairwise EqualFold comparisons
// independently check the scanner's sorted, normalized duplicate-name policy.
func nestedJSONAdmitted(raw []byte) bool {
	limits := core.DefaultStrictJSONLimits()
	maximum, err := limits.DocumentMaximumBytes.Uint64()
	if err != nil || len(raw) == 0 || len(raw) > attest.CanonicalBodyMaximumBytes-len(`{"nested":}`) || uint64(len(raw)) > maximum || !utf8.Valid(raw) {
		return false
	}
	decoder := jsontext.NewDecoder(bytes.NewReader(raw))
	if decoder.PeekKind() == jsontext.KindNull || !nestedJSONValueAdmitted(decoder, limits, 0) {
		return false
	}
	_, err = decoder.ReadToken()
	return errors.Is(err, io.EOF)
}

func nestedJSONValueAdmitted(decoder *jsontext.Decoder, limits core.StrictJSONLimits, depth uint16) bool {
	token, err := decoder.ReadToken()
	if err != nil {
		return false
	}
	switch token.Kind() {
	case jsontext.KindBeginObject:
		return depth < limits.NestingDepthMaximum && nestedJSONObjectAdmitted(decoder, limits, depth+1)
	case jsontext.KindBeginArray:
		return depth < limits.NestingDepthMaximum && nestedJSONArrayAdmitted(decoder, limits, depth+1)
	case jsontext.KindString, jsontext.KindNumber, jsontext.KindTrue, jsontext.KindFalse, jsontext.KindNull:
		return true
	default:
		return false
	}
}

func nestedJSONObjectAdmitted(decoder *jsontext.Decoder, limits core.StrictJSONLimits, depth uint16) bool {
	var names []string
	for decoder.PeekKind() != jsontext.KindEndObject {
		name, err := decoder.ReadToken()
		if err != nil || name.Kind() != jsontext.KindString || len(names) >= int(limits.ObjectFieldMaximum) {
			return false
		}
		for _, prior := range names {
			if strings.EqualFold(prior, name.String()) {
				return false
			}
		}
		names = append(names, name.String())
		if !nestedJSONValueAdmitted(decoder, limits, depth) {
			return false
		}
	}
	_, err := decoder.ReadToken()
	return err == nil
}

func nestedJSONArrayAdmitted(decoder *jsontext.Decoder, limits core.StrictJSONLimits, depth uint16) bool {
	for count := uint32(0); decoder.PeekKind() != jsontext.KindEndArray; count++ {
		if count >= limits.ArrayItemMaximum || !nestedJSONValueAdmitted(decoder, limits, depth) {
			return false
		}
	}
	_, err := decoder.ReadToken()
	return err == nil
}

type envelopeJSONFacts struct {
	domain    testDomain
	signer    [ed25519.PublicKeySize]byte
	length    uint64
	digest    [sha256.Size]byte
	signature [ed25519.SignatureSize]byte
}

// Decode into raw typed fields with the standard library, then derive the
// expected facts without calling Envelope, Signature, or Core JSON decoders.
func envelopeJSONOracle(raw []byte) (envelopeJSONFacts, bool) {
	var facts envelopeJSONFacts
	if len(raw) > attest.EnvelopeJSONMaximumBytes {
		return facts, false
	}
	var parts envelopeJSONParts
	if err := json.Unmarshal(raw, &parts, json.RejectUnknownMembers(true)); err != nil {
		return facts, false
	}
	var domain string
	if err := json.Unmarshal(parts.Domain, &domain); err != nil {
		return facts, false
	}
	switch domain {
	case testDomainPrimaryText:
		facts.domain = testDomainPrimary
	case testDomainAlternateText:
		facts.domain = testDomainAlternate
	default:
		return facts, false
	}
	length, err := strconv.ParseUint(string(bytes.TrimSpace(parts.BodyLength)), 10, 64)
	if err != nil || length == 0 || length > attest.CanonicalBodyMaximumBytes {
		return facts, false
	}
	facts.length = length
	admitted := canonicalHexJSONOracle(parts.Signer, facts.signer[:]) &&
		canonicalHexJSONOracle(parts.BodySHA256, facts.digest[:]) &&
		canonicalHexJSONOracle(parts.Signature, facts.signature[:])
	return facts, admitted
}

func canonicalHexJSONOracle(raw []byte, destination []byte) bool {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || len(value) != hex.EncodedLen(len(destination)) {
		return false
	}
	if _, err := hex.Decode(destination, []byte(value)); err != nil {
		return false
	}
	return hex.EncodeToString(destination) == value
}

func envelopeMatchesJSONFacts(t testing.TB, got attest.Envelope[testDomain], want envelopeJSONFacts) {
	t.Helper()
	length, lengthErr := got.BodyLength.Uint64()
	signer, signerErr := got.Signer.Bytes()
	digest, digestErr := got.BodySHA256.Bytes()
	signature, signatureErr := got.Signature.Bytes()
	if lengthErr != nil || signerErr != nil || digestErr != nil || signatureErr != nil {
		t.Fatalf("decoded envelope projections errors = (%v, %v, %v, %v), want nil", lengthErr, signerErr, digestErr, signatureErr)
	}
	if got.Domain != want.domain || length != want.length || !bytes.Equal(signer, want.signer[:]) || digest != want.digest || signature != want.signature {
		t.Fatalf("decoded envelope = %+v, want independently decoded facts %+v", got, want)
	}
}
