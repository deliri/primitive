package attest

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

const (
	panicAtConsumerBoundaryErrorText   = "attest consumer callback panicked"
	signerMissingErrorText             = "ed25519 signer is missing"
	signerPublicKeyTypeErrorText       = "signer public key is not ed25519"
	privateKeyLengthErrorText          = "ed25519 private key has invalid length"
	privateKeyPublicHalfErrorText      = "ed25519 private key public half is inconsistent"
	bodyMissingErrorText               = "canonical body is missing"
	bodyEmptyErrorText                 = "canonical body is empty"
	bodyLimitErrorText                 = "canonical body exceeds its byte limit"
	writerClosedErrorText              = "canonical body writer is closed"
	signatureUnsetErrorText            = "ed25519 signature is unset"
	signatureLengthErrorText           = "ed25519 signature has invalid length"
	signatureEncodingErrorText         = "ed25519 signature is not canonical lowercase hexadecimal"
	domainCanonicalErrorText           = "signing domain is not canonical"
	frameExtentErrorText               = "attestation frame extent is outside the supported range"
	trustedKeyCountErrorText           = "trusted key count is outside the supported range"
	trustedKeyDuplicateErrorText       = "trusted keys contain a duplicate"
	trustedKeySmallOrderErrorText      = "trusted key is the all-zero small-order point"
	trustedKeyStorageErrorText         = "trusted key storage is not closed"
	canonicalObjectUnopenedErrorText   = "canonical object was never opened"
	canonicalObjectClosedErrorText     = "canonical object is closed"
	canonicalObjectEmptyErrorText      = "canonical object has no members"
	canonicalObjectExtentErrorText     = "canonical object exceeds its byte limit"
	canonicalObjectFieldCountErrorText = "canonical object exceeds its member limit"
	canonicalObjectDuplicateErrorText  = "canonical object repeats a member name"
	canonicalFieldNameExtentErrorText  = "canonical member name extent is outside the supported range"
	canonicalFieldNameGrammarErrorText = "canonical member name is not canonical"
	canonicalMemberMissingErrorText    = "canonical member value is missing"
	canonicalMemberExtentErrorText     = "canonical member extent is outside the supported range"
	canonicalMemberEncodingErrorText   = "canonical member encoding is not valid json"
	canonicalMemberNullErrorText       = "canonical member encoding is null"
	envelopeWireMissingErrorText       = "envelope wire field is missing"
	envelopeBodyLengthErrorText        = "envelope body length is outside the supported range"
	envelopeDomainMismatchErrorText    = "envelope domain does not match the canonical body"
	envelopeLengthMismatchErrorText    = "envelope body length does not match the canonical body"
	envelopeDigestMismatchErrorText    = "envelope body digest does not match the canonical body"
	envelopeSignerUntrustedErrorText   = "envelope signer is not trusted"
	envelopeSignatureErrorText         = "envelope signature does not verify"
	postSignVerificationErrorText      = "new signature did not verify"
	verifiedProofUnsetErrorText        = "attestation proof is unset"
)

func contractError(errs ...error) error {
	return joinIdentity(core.ErrAttestContract, errs...)
}

func verificationError(errs ...error) error {
	return joinIdentity(core.ErrAttestVerification, errs...)
}

func envelopeJSONError(errs ...error) error {
	return errors.Join(core.ErrJSONContract, contractError(errs...))
}

func joinIdentity(identity error, errs ...error) error {
	values := make([]error, 1, len(errs)+1)
	values[0] = identity
	for _, err := range errs {
		if err != nil {
			values = append(values, err)
		}
	}
	return errors.Join(values...)
}
