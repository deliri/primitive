package timeproof

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
	"time"

	"github.com/deliri/primitive/v2026/temporal"
)

func parseAndVerifyToken(response []byte, digest [sha256.Size]byte, nonce Nonce, authority Authority) (verifiedToken, error) {
	tokenDER, conclusion, err := parseTimestampResponse(response)
	if err != nil {
		return verifiedToken{}, err
	}
	if !conclusion.status.granted() {
		refusal, refusalErr := newRefusal(conclusion)
		if refusalErr != nil {
			return verifiedToken{}, refusalErr
		}
		return verifiedToken{}, refusal
	}
	return verifyTimestampToken(tokenDER, digest, nonce, authority)
}

func verifyTimestampToken(tokenDER []byte, digest [sha256.Size]byte, nonce Nonce, authority Authority) (verifiedToken, error) {
	token, err := parseTimestampToken(tokenDER)
	if err != nil {
		return verifiedToken{}, invalidError(err)
	}
	info, err := parseTSTInfo(token.TSTDER)
	if err != nil {
		return verifiedToken{}, invalidError(err)
	}
	policy, err := verifyTSTBinding(info, digest, nonce, authority)
	if err != nil {
		return verifiedToken{}, err
	}
	if err := verifyTimestampSigner(token, info.GenerationTime, authority); err != nil {
		return verifiedToken{}, invalidError(err)
	}
	if err := verifyTSAName(info.TSASubject, token.Signer); err != nil {
		return verifiedToken{}, err
	}
	if err := verifySigningCertificateAttribute(token.Attributes, token.Signer); err != nil {
		return verifiedToken{}, invalidError(err)
	}
	if err := validateSigningTimeAttribute(token.Attributes); err != nil {
		return verifiedToken{}, err
	}
	signerSum := sha256.Sum256(token.Signer.Raw)
	return verifiedToken{
		SignerSHA256: signerSum, Serial: info.Serial, Policy: policy,
		GenerationTime: info.GenerationTime,
	}, nil
}

// validateSigningTimeAttribute enforces CMS cardinality and canonical time
// encoding. CMS signingTime is the purported signature-operation time; RFC
// 5652 does not require it to equal RFC 3161 genTime.
func validateSigningTimeAttribute(attributes []cmsAttribute) error {
	signingTime, count := countAttribute(attributes, oidSigningTime)
	if count == 0 {
		return nil
	}
	if count != 1 || len(signingTime.Values) != 1 {
		return invalidError(nil)
	}
	var claimed time.Time
	trailing, err := asn1.Unmarshal(signingTime.Values[0].FullBytes, &claimed)
	if err != nil || len(trailing) != 0 {
		return invalidError(err)
	}
	if !canonicalSigningTime(signingTime.Values[0], claimed) {
		return invalidError(nil)
	}
	return nil
}

func canonicalSigningTime(raw asn1.RawValue, claimed time.Time) bool {
	if raw.Class != asn1.ClassUniversal || raw.IsCompound {
		return false
	}
	year := claimed.Year()
	if year >= 1950 && year <= 2049 {
		return raw.Tag == asn1.TagUTCTime &&
			len(raw.Bytes) == len("YYMMDDhhmmssZ") &&
			raw.Bytes[len(raw.Bytes)-1] == 'Z'
	}
	return raw.Tag == asn1.TagGeneralizedTime &&
		len(raw.Bytes) == len("YYYYMMDDhhmmssZ") &&
		raw.Bytes[len(raw.Bytes)-1] == 'Z'
}

func verifyTSAName(subject []byte, signer *x509.Certificate) error {
	if len(subject) == 0 {
		return nil
	}
	if signer == nil || !bytes.Equal(subject, signer.RawSubject) {
		return invalidError(nil)
	}
	return nil
}

func parseTimestampResponse(der []byte) ([]byte, authorityConclusion, error) {
	if len(der) == 0 {
		return nil, authorityConclusion{}, invalidError(nil)
	}
	outer, err := requireSequence(der)
	if err != nil {
		return nil, authorityConclusion{}, err
	}
	conclusion, remaining, err := consumeResponseStatus(outer.Bytes)
	if err != nil {
		return nil, authorityConclusion{}, err
	}
	hasToken := len(remaining) != 0
	if conclusion.status.granted() != hasToken {
		return nil, authorityConclusion{}, invalidError(nil)
	}
	if !hasToken {
		return nil, conclusion, nil
	}
	token, trailing, err := consumeRaw(remaining)
	if err != nil || len(trailing) != 0 || !isUniversal(token, asn1.TagSequence, true) {
		return nil, authorityConclusion{}, invalidError(err)
	}
	return append([]byte(nil), token.FullBytes...), conclusion, nil
}

func consumeResponseStatus(der []byte) (authorityConclusion, []byte, error) {
	statusRaw, remaining, err := consumeRaw(der)
	if err != nil || !isUniversal(statusRaw, asn1.TagSequence, true) {
		return authorityConclusion{}, nil, invalidError(err)
	}
	conclusion, err := parseStatusInfo(statusRaw.Bytes)
	return conclusion, remaining, err
}

func parseStatusInfo(der []byte) (authorityConclusion, error) {
	statusRaw, fields, err := consumeRaw(der)
	if err != nil || !isUniversal(statusRaw, asn1.TagInteger, false) {
		return authorityConclusion{}, invalidError(err)
	}
	var statusInteger int
	if trailing, decodeErr := asn1.Unmarshal(statusRaw.FullBytes, &statusInteger); decodeErr != nil || len(trailing) != 0 {
		return authorityConclusion{}, invalidError(decodeErr)
	}
	status, err := refusalStatusFromRFC(statusInteger)
	if err != nil {
		return authorityConclusion{}, err
	}
	codes, err := parseStatusInfoOptional(fields)
	if err != nil {
		return authorityConclusion{}, err
	}
	if status.granted() && !codes.isZero() {
		return authorityConclusion{}, invalidError(nil)
	}
	return authorityConclusion{status: status, codes: codes}, nil
}

func parseStatusInfoOptional(fields []byte) (refusalCodeSet, error) {
	var codes refusalCodeSet
	seenText := false
	seenCodes := false
	for len(fields) != 0 {
		field, remaining, err := consumeRaw(fields)
		if err != nil {
			return refusalCodeSet{}, invalidError(err)
		}
		switch {
		case isUniversal(field, asn1.TagSequence, true):
			if seenText || seenCodes {
				return refusalCodeSet{}, invalidError(nil)
			}
			if err := validateStatusText(field.Bytes); err != nil {
				return refusalCodeSet{}, err
			}
			seenText = true
		case isUniversal(field, asn1.TagBitString, false):
			if seenCodes {
				return refusalCodeSet{}, invalidError(nil)
			}
			parsed, parseErr := parseRefusalCodes(field.FullBytes)
			if parseErr != nil {
				return refusalCodeSet{}, parseErr
			}
			codes = parsed
			seenCodes = true
		default:
			return refusalCodeSet{}, invalidError(nil)
		}
		fields = remaining
	}
	return codes, nil
}

func validateStatusText(der []byte) error {
	count := 0
	for len(der) != 0 {
		value, remaining, err := consumeRaw(der)
		if err != nil || !isUniversal(value, asn1.TagUTF8String, false) {
			return invalidError(err)
		}
		count++
		if count > refusalStatusTextCount {
			return invalidError(nil)
		}
		der = remaining
	}
	if count == 0 {
		return invalidError(nil)
	}
	return nil
}

func parseRefusalCodes(der []byte) (refusalCodeSet, error) {
	bits, err := parseFailureBitString(der)
	if err != nil {
		return refusalCodeSet{}, err
	}
	codes, err := failureCodesFromBits(bits)
	if err != nil {
		return refusalCodeSet{}, err
	}
	set, err := newRefusalCodeSet(codes...)
	if err != nil {
		return refusalCodeSet{}, invalidError(nil, err)
	}
	return set, nil
}

// parseFailureBitString decodes the failInfo BIT STRING and bounds its declared
// length by the ceiling derived from the closed failure-code enum.
func parseFailureBitString(der []byte) (asn1.BitString, error) {
	var bits asn1.BitString
	trailing, err := asn1.Unmarshal(der, &bits)
	maximumBit, maximumErr := refusalMaximumRFCBit()
	if err != nil || len(trailing) != 0 ||
		maximumErr != nil ||
		bits.BitLength < 1 || bits.BitLength > int(maximumBit)+1 {
		return asn1.BitString{}, invalidError(err, maximumErr)
	}
	return bits, nil
}

// failureCodesFromBits maps every set bit to its closed enum code. RFC 3161
// carries failInfo only for a request that was not granted, so a present but
// all-clear bit string is malformed: decoded to an empty code set it would be
// indistinguishable from an absent failInfo and would slip past the
// granted/refused exclusivity gate.
func failureCodesFromBits(bits asn1.BitString) ([]RefusalCode, error) {
	codes := make([]RefusalCode, 0, refusalMaximumCodeCount)
	for bit := 0; bit < bits.BitLength; bit++ {
		if bits.At(bit) == 0 {
			continue
		}
		code, err := refusalCodeFromRFCBit(bit)
		if err != nil {
			return nil, invalidError(nil, err)
		}
		codes = append(codes, code)
	}
	if len(codes) == 0 {
		return nil, invalidError(nil)
	}
	return codes, nil
}

func parseTimestampToken(der []byte) (parsedToken, error) {
	contentType, explicitContent, err := parseContentInfo(der)
	if err != nil || !contentType.Equal(oidSignedData) {
		return parsedToken{}, invalidError(err)
	}
	signed, err := parseSignedData(explicitContent)
	if err != nil {
		return parsedToken{}, err
	}
	if !validSignedTokenShape(signed) {
		return parsedToken{}, invalidError(nil)
	}
	signerInfo := signed.Signers[0]
	if !validSignerAlgorithms(signed.DigestAlgorithms, signerInfo) {
		return parsedToken{}, invalidError(nil)
	}
	tstDER, err := explicitOctets(signed.Content.Content)
	if err != nil {
		return parsedToken{}, err
	}
	signer, attributes, err := verifySignedToken(signed, tstDER)
	if err != nil {
		return parsedToken{}, err
	}
	return parsedToken{
		Signer: signer, TSTDER: tstDER, Certificates: signed.Certificates,
		SignerInfo: signerInfo, Attributes: attributes,
	}, nil
}

// validSignedTokenShape pins the RFC 5652 versions this parser accepts. The
// encapsulated content is id-ct-TSTInfo, which fixes SignedData at version 3,
// and the only signer identifier accepted here is issuerAndSerialNumber, which
// fixes SignerInfo at version 1.
func validSignedTokenShape(signed parsedSignedData) bool {
	return signed.Content.ContentType.Equal(oidTSTInfo) &&
		len(signed.Signers) == signerMaximumCount &&
		signed.Version == cmsSignedDataVersion &&
		signed.Signers[0].Version == cmsSignerInfoVersion
}

func validSignerAlgorithms(algorithms []pkix.AlgorithmIdentifier, signer cmsSignerInfo) bool {
	return digestAlgorithmDeclared(algorithms, signer.DigestAlgorithm.Algorithm) &&
		signatureDigestMatches(signer)
}

func verifySignedToken(signed parsedSignedData, content []byte) (*x509.Certificate, []cmsAttribute, error) {
	signerInfo := signed.Signers[0]
	signer, err := findSignerCertificate(signerInfo, signed.Certificates)
	if err != nil {
		return nil, nil, invalidError(err)
	}
	attributes, err := parseSignedAttributes(signerInfo.SignedAttributes)
	if err != nil {
		return nil, nil, err
	}
	if err := verifyRequiredSignedAttributes(attributes, signerInfo, content); err != nil {
		return nil, nil, invalidError(err)
	}
	if err := verifySignerSignature(signerInfo, signer); err != nil {
		return nil, nil, invalidError(err)
	}
	return signer, attributes, nil
}

func digestAlgorithmDeclared(algorithms []pkix.AlgorithmIdentifier, want asn1.ObjectIdentifier) bool {
	count := 0
	for _, algorithm := range algorithms {
		if algorithm.Algorithm.Equal(want) {
			count++
		}
	}
	return count == 1
}

func signatureDigestMatches(signer cmsSignerInfo) bool {
	signature := signer.SignatureAlgorithm.Algorithm
	digest := signer.DigestAlgorithm.Algorithm
	switch {
	case signature.Equal(oidECDSAWithSHA256), signature.Equal(oidSHA256WithRSA):
		return digest.Equal(oidSHA256)
	case signature.Equal(oidECDSAWithSHA384), signature.Equal(oidSHA384WithRSA):
		return digest.Equal(oidSHA384)
	case signature.Equal(oidECDSAWithSHA512), signature.Equal(oidSHA512WithRSA):
		return digest.Equal(oidSHA512)
	case signature.Equal(oidEd25519):
		return digest.Equal(oidSHA256) || digest.Equal(oidSHA384) || digest.Equal(oidSHA512)
	default:
		return false
	}
}

func parseContentInfo(der []byte) (asn1.ObjectIdentifier, asn1.RawValue, error) {
	sequence, err := requireSequence(der)
	if err != nil {
		return nil, asn1.RawValue{}, err
	}
	oidRaw, fields, err := consumeRaw(sequence.Bytes)
	if err != nil {
		return nil, asn1.RawValue{}, invalidError(err)
	}
	var contentType asn1.ObjectIdentifier
	if trailing, decodeErr := asn1.Unmarshal(oidRaw.FullBytes, &contentType); decodeErr != nil || len(trailing) != 0 {
		return nil, asn1.RawValue{}, invalidError(decodeErr)
	}
	content, trailing, err := consumeRaw(fields)
	if err != nil || len(trailing) != 0 || content.Class != asn1.ClassContextSpecific ||
		content.Tag != 0 || !content.IsCompound {
		return nil, asn1.RawValue{}, invalidError(err)
	}
	return contentType, content, nil
}

func parseSignedData(explicit asn1.RawValue) (parsedSignedData, error) {
	sequence, err := requireSequence(explicit.Bytes)
	if err != nil {
		return parsedSignedData{}, err
	}
	fields := sequence.Bytes
	version, fields, err := consumeInteger(fields)
	if err != nil {
		return parsedSignedData{}, err
	}
	digests, fields, err := consumeAlgorithmSet(fields)
	if err != nil || len(digests) == 0 {
		return parsedSignedData{}, invalidError(err)
	}
	content, fields, err := consumeEncapsulatedContent(fields)
	if err != nil {
		return parsedSignedData{}, err
	}
	certificatesRaw, fields, err := consumeContextField(fields, 0, true)
	if err != nil {
		return parsedSignedData{}, err
	}
	_, fields, err = consumeContextField(fields, 1, false)
	if err != nil {
		return parsedSignedData{}, err
	}
	signers, err := consumeFinalSignerSet(fields)
	if err != nil {
		return parsedSignedData{}, err
	}
	certificates, err := parseCertificates(certificatesRaw)
	if err != nil {
		return parsedSignedData{}, err
	}
	return parsedSignedData{
		Version: version, DigestAlgorithms: digests, Content: content,
		Certificates: certificates, Signers: signers,
	}, nil
}

func consumeFinalSignerSet(fields []byte) ([]cmsSignerInfo, error) {
	signers, trailing, err := consumeSignerSet(fields)
	if err != nil || len(trailing) != 0 {
		return nil, invalidError(err)
	}
	return signers, nil
}

func consumeEncapsulatedContent(der []byte) (cmsEncapsulatedContent, []byte, error) {
	raw, remaining, err := consumeRaw(der)
	if err != nil || !isUniversal(raw, asn1.TagSequence, true) {
		return cmsEncapsulatedContent{}, nil, invalidError(err)
	}
	oidRaw, fields, err := consumeRaw(raw.Bytes)
	if err != nil {
		return cmsEncapsulatedContent{}, nil, invalidError(err)
	}
	var contentType asn1.ObjectIdentifier
	if trailing, decodeErr := asn1.Unmarshal(oidRaw.FullBytes, &contentType); decodeErr != nil || len(trailing) != 0 {
		return cmsEncapsulatedContent{}, nil, invalidError(decodeErr)
	}
	content, err := consumeExplicitContent(fields)
	if err != nil {
		return cmsEncapsulatedContent{}, nil, invalidError(err)
	}
	return cmsEncapsulatedContent{ContentType: contentType, Content: content}, remaining, nil
}

func consumeExplicitContent(fields []byte) (asn1.RawValue, error) {
	content, trailing, err := consumeRaw(fields)
	if err != nil || len(trailing) != 0 || content.Class != asn1.ClassContextSpecific ||
		content.Tag != 0 || !content.IsCompound {
		return asn1.RawValue{}, invalidError(err)
	}
	return content, nil
}

func consumeAlgorithmSet(der []byte) ([]pkix.AlgorithmIdentifier, []byte, error) {
	raw, remaining, err := consumeRaw(der)
	if err != nil || !isUniversal(raw, asn1.TagSet, true) {
		return nil, nil, invalidError(err)
	}
	var algorithms []pkix.AlgorithmIdentifier
	for fields := raw.Bytes; len(fields) != 0; {
		if len(algorithms) >= digestAlgorithmMaximumCount {
			return nil, nil, invalidError(nil)
		}
		var algorithm pkix.AlgorithmIdentifier
		var decodeErr error
		fields, decodeErr = asn1.Unmarshal(fields, &algorithm)
		if decodeErr != nil {
			return nil, nil, invalidError(decodeErr)
		}
		algorithms = append(algorithms, algorithm)
	}
	return algorithms, remaining, nil
}

func consumeContextField(der []byte, tag int, required bool) (asn1.RawValue, []byte, error) {
	if len(der) == 0 {
		if required {
			return asn1.RawValue{}, nil, invalidError(nil)
		}
		return asn1.RawValue{}, der, nil
	}
	raw, remaining, err := consumeRaw(der)
	if err != nil {
		return asn1.RawValue{}, nil, invalidError(err)
	}
	if raw.Class != asn1.ClassContextSpecific || raw.Tag != tag {
		if required {
			return asn1.RawValue{}, nil, invalidError(nil)
		}
		return asn1.RawValue{}, der, nil
	}
	return raw, remaining, nil
}

func consumeSignerSet(der []byte) ([]cmsSignerInfo, []byte, error) {
	raw, remaining, err := consumeRaw(der)
	if err != nil || !isUniversal(raw, asn1.TagSet, true) {
		return nil, nil, invalidError(err)
	}
	var signers []cmsSignerInfo
	for fields := raw.Bytes; len(fields) != 0; {
		if len(signers) >= signerMaximumCount {
			return nil, nil, invalidError(nil)
		}
		signer, trailing, parseErr := parseSignerInfo(fields)
		if parseErr != nil {
			return nil, nil, parseErr
		}
		signers = append(signers, signer)
		fields = trailing
	}
	return signers, remaining, nil
}

func parseSignerInfo(der []byte) (cmsSignerInfo, []byte, error) {
	raw, remaining, err := consumeRaw(der)
	if err != nil || !isUniversal(raw, asn1.TagSequence, true) {
		return cmsSignerInfo{}, nil, invalidError(err)
	}
	fields := raw.Bytes
	version, fields, err := consumeInteger(fields)
	if err != nil {
		return cmsSignerInfo{}, nil, err
	}
	sid, fields, err := consumeIssuerAndSerial(fields)
	if err != nil {
		return cmsSignerInfo{}, nil, err
	}
	digest, fields, err := consumeAlgorithm(fields)
	if err != nil {
		return cmsSignerInfo{}, nil, err
	}
	attributes, fields, err := consumeContextField(fields, 0, true)
	if err != nil {
		return cmsSignerInfo{}, nil, err
	}
	signatureAlgorithm, fields, err := consumeAlgorithm(fields)
	if err != nil {
		return cmsSignerInfo{}, nil, err
	}
	signature, err := consumeFinalSignature(fields)
	if err != nil {
		return cmsSignerInfo{}, nil, err
	}
	return cmsSignerInfo{
		Version: version, SID: sid, DigestAlgorithm: digest,
		SignedAttributes: attributes, SignatureAlgorithm: signatureAlgorithm,
		Signature: signature,
	}, remaining, nil
}

func consumeFinalSignature(fields []byte) ([]byte, error) {
	signatureRaw, trailing, err := consumeRaw(fields)
	if err != nil || !isUniversal(signatureRaw, asn1.TagOctetString, false) || len(trailing) != 0 {
		return nil, invalidError(err)
	}
	var signature []byte
	if rest, decodeErr := asn1.Unmarshal(signatureRaw.FullBytes, &signature); decodeErr != nil || len(rest) != 0 {
		return nil, invalidError(decodeErr)
	}
	return signature, nil
}

func consumeIssuerAndSerial(der []byte) (cmsIssuerAndSerial, []byte, error) {
	raw, remaining, err := consumeRaw(der)
	if err != nil || !isUniversal(raw, asn1.TagSequence, true) {
		return cmsIssuerAndSerial{}, nil, invalidError(err)
	}
	issuer, fields, err := consumeRaw(raw.Bytes)
	if err != nil || !isUniversal(issuer, asn1.TagSequence, true) {
		return cmsIssuerAndSerial{}, nil, invalidError(err)
	}
	serialRaw, trailing, err := consumeRaw(fields)
	if err != nil || len(trailing) != 0 {
		return cmsIssuerAndSerial{}, nil, invalidError(err)
	}
	serial, err := parsePositiveSerial(serialRaw.FullBytes)
	if err != nil {
		return cmsIssuerAndSerial{}, nil, err
	}
	return cmsIssuerAndSerial{Issuer: issuer, Serial: serial}, remaining, nil
}

func parsePositiveSerial(der []byte) (*big.Int, error) {
	var serial *big.Int
	rest, err := asn1.Unmarshal(der, &serial)
	if err != nil || len(rest) != 0 || serial == nil || serial.Sign() <= 0 {
		return nil, invalidError(err)
	}
	return serial, nil
}

func consumeAlgorithm(der []byte) (pkix.AlgorithmIdentifier, []byte, error) {
	var algorithm pkix.AlgorithmIdentifier
	remaining, err := asn1.Unmarshal(der, &algorithm)
	if err != nil {
		return pkix.AlgorithmIdentifier{}, nil, invalidError(err)
	}
	return algorithm, remaining, nil
}

func parseCertificates(raw asn1.RawValue) ([]*x509.Certificate, error) {
	if len(raw.Bytes) == 0 {
		return nil, invalidError(nil)
	}
	var certificates []*x509.Certificate
	for fields := raw.Bytes; len(fields) != 0; {
		if len(certificates) >= certificateMaximumCount {
			return nil, invalidError(nil)
		}
		certificateRaw, remaining, err := consumeRaw(fields)
		if err != nil || !isUniversal(certificateRaw, asn1.TagSequence, true) {
			return nil, invalidError(err)
		}
		certificate, err := x509.ParseCertificate(certificateRaw.FullBytes)
		if err != nil {
			return nil, invalidError(err)
		}
		for _, prior := range certificates {
			if bytes.Equal(prior.Raw, certificate.Raw) {
				return nil, invalidError(nil)
			}
		}
		certificates = append(certificates, certificate)
		fields = remaining
	}
	return certificates, nil
}

func findSignerCertificate(signer cmsSignerInfo, certificates []*x509.Certificate) (*x509.Certificate, error) {
	var match *x509.Certificate
	for _, certificate := range certificates {
		if certificate.SerialNumber.Cmp(signer.SID.Serial) != 0 ||
			!bytes.Equal(certificate.RawIssuer, signer.SID.Issuer.FullBytes) {
			continue
		}
		if match != nil {
			return nil, invalidError(nil)
		}
		match = certificate
	}
	if match == nil {
		return nil, invalidError(nil)
	}
	return match, nil
}

func parseSignedAttributes(raw asn1.RawValue) ([]cmsAttribute, error) {
	if raw.Class != asn1.ClassContextSpecific || raw.Tag != 0 || !raw.IsCompound ||
		len(raw.Bytes) == 0 {
		return nil, invalidError(nil)
	}
	var attributes []cmsAttribute
	for fields := raw.Bytes; len(fields) != 0; {
		if len(attributes) >= signedAttributeMaximumCount {
			return nil, invalidError(nil)
		}
		var attribute cmsAttribute
		remaining, err := asn1.Unmarshal(fields, &attribute)
		if err != nil {
			return nil, invalidError(err)
		}
		attributes = append(attributes, attribute)
		fields = remaining
	}
	return attributes, nil
}

func verifyRequiredSignedAttributes(attributes []cmsAttribute, signer cmsSignerInfo, content []byte) error {
	contentType, err := uniqueAttribute(attributes, oidContentType)
	if err != nil {
		return invalidError(err)
	}
	if err := validateContentTypeAttribute(contentType); err != nil {
		return err
	}
	messageDigest, err := uniqueAttribute(attributes, oidMessageDigest)
	if err != nil {
		return invalidError(err)
	}
	return validateMessageDigestAttribute(messageDigest, signer, content)
}

func validateContentTypeAttribute(attribute cmsAttribute) error {
	if len(attribute.Values) != 1 {
		return invalidError(nil)
	}
	var got asn1.ObjectIdentifier
	trailing, err := asn1.Unmarshal(attribute.Values[0].FullBytes, &got)
	if err != nil || len(trailing) != 0 || !got.Equal(oidTSTInfo) {
		return invalidError(err)
	}
	return nil
}

func validateMessageDigestAttribute(
	attribute cmsAttribute,
	signer cmsSignerInfo,
	content []byte,
) error {
	if len(attribute.Values) != 1 {
		return invalidError(nil)
	}
	var gotDigest []byte
	trailing, err := asn1.Unmarshal(attribute.Values[0].FullBytes, &gotDigest)
	if err != nil || len(trailing) != 0 {
		return invalidError(err)
	}
	wantDigest, err := digestForOID(signer.DigestAlgorithm.Algorithm, content)
	if err != nil || subtle.ConstantTimeCompare(gotDigest, wantDigest) != 1 {
		return invalidError(err)
	}
	return nil
}

func verifySignerSignature(signer cmsSignerInfo, certificate *x509.Certificate) error {
	if certificate == nil || len(signer.Signature) == 0 {
		return invalidError(nil)
	}
	algorithm, err := signatureAlgorithmForOID(signer.SignatureAlgorithm.Algorithm)
	if err != nil {
		return err
	}
	signedDER := derTagged(byte(asn1.TagSet)|derConstructed, signer.SignedAttributes.Bytes)
	if err := certificate.CheckSignature(algorithm, signedDER, signer.Signature); err != nil {
		return invalidError(err)
	}
	return nil
}

func digestForOID(oid asn1.ObjectIdentifier, content []byte) ([]byte, error) {
	switch {
	case oid.Equal(oidSHA256):
		sum := sha256.Sum256(content)
		return sum[:], nil
	case oid.Equal(oidSHA384):
		sum := sha512.Sum384(content)
		return sum[:], nil
	case oid.Equal(oidSHA512):
		sum := sha512.Sum512(content)
		return sum[:], nil
	default:
		return nil, invalidError(nil)
	}
}

func signatureAlgorithmForOID(oid asn1.ObjectIdentifier) (x509.SignatureAlgorithm, error) {
	switch {
	case oid.Equal(oidECDSAWithSHA256):
		return x509.ECDSAWithSHA256, nil
	case oid.Equal(oidECDSAWithSHA384):
		return x509.ECDSAWithSHA384, nil
	case oid.Equal(oidECDSAWithSHA512):
		return x509.ECDSAWithSHA512, nil
	case oid.Equal(oidSHA256WithRSA):
		return x509.SHA256WithRSA, nil
	case oid.Equal(oidSHA384WithRSA):
		return x509.SHA384WithRSA, nil
	case oid.Equal(oidSHA512WithRSA):
		return x509.SHA512WithRSA, nil
	case oid.Equal(oidEd25519):
		return x509.PureEd25519, nil
	default:
		return x509.UnknownSignatureAlgorithm, invalidError(nil)
	}
}

func parseTSTInfo(der []byte) (parsedTSTInfo, error) {
	sequence, err := requireSequence(der)
	if err != nil {
		return parsedTSTInfo{}, err
	}
	fields := sequence.Bytes
	version, fields, err := consumeInteger(fields)
	if err != nil {
		return parsedTSTInfo{}, err
	}
	policy, fields, err := consumeOID(fields)
	if err != nil {
		return parsedTSTInfo{}, err
	}
	imprint, fields, err := consumeMessageImprint(fields)
	if err != nil {
		return parsedTSTInfo{}, err
	}
	serial, fields, err := consumePositiveInteger(fields, SerialMaximumBits)
	if err != nil {
		return parsedTSTInfo{}, err
	}
	generationTime, fields, err := consumeGenerationTime(fields)
	if err != nil {
		return parsedTSTInfo{}, err
	}
	info := parsedTSTInfo{
		Version: version, Policy: policy, MessageImprint: imprint,
		Serial: serial, GenerationTime: generationTime,
	}
	if err := parseTSTOptionalFields(fields, &info); err != nil {
		return parsedTSTInfo{}, err
	}
	if info.Nonce == nil {
		return parsedTSTInfo{}, invalidError(nil)
	}
	return info, nil
}

func parseTSTOptionalFields(fields []byte, info *parsedTSTInfo) error {
	var err error
	fields, err = consumeOptionalAccuracy(fields)
	if err != nil {
		return err
	}
	fields, err = consumeOptionalOrdering(fields)
	if err != nil {
		return err
	}
	fields, err = consumeOptionalNonce(fields, info)
	if err != nil {
		return err
	}
	fields, err = consumeOptionalTSA(fields, info)
	if err != nil {
		return err
	}
	if len(fields) != 0 {
		return invalidError(nil)
	}
	return nil
}

func consumeOptionalAccuracy(
	fields []byte,
) ([]byte, error) {
	raw, remaining, present, err := peekTSTField(fields)
	if err != nil || !present ||
		!isUniversal(raw, asn1.TagSequence, true) {
		return fields, err
	}
	if _, err := parseAccuracy(raw); err != nil {
		return nil, err
	}
	return remaining, nil
}

func consumeOptionalOrdering(
	fields []byte,
) ([]byte, error) {
	raw, remaining, present, err := peekTSTField(fields)
	if err != nil || !present ||
		!isUniversal(raw, asn1.TagBoolean, false) {
		return fields, err
	}
	var ordering bool
	if trailing, decodeErr := asn1.Unmarshal(
		raw.FullBytes,
		&ordering,
	); decodeErr != nil || len(trailing) != 0 || !ordering {
		return nil, invalidError(decodeErr)
	}
	return remaining, nil
}

func consumeOptionalNonce(
	fields []byte,
	info *parsedTSTInfo,
) ([]byte, error) {
	raw, remaining, present, err := peekTSTField(fields)
	if err != nil || !present ||
		!isUniversal(raw, asn1.TagInteger, false) {
		return fields, err
	}
	nonce, err := decodePositiveInteger(raw.FullBytes, 8*NonceBytes)
	if err != nil {
		return nil, err
	}
	info.Nonce = nonce
	return remaining, nil
}

func consumeOptionalTSA(
	fields []byte,
	info *parsedTSTInfo,
) ([]byte, error) {
	raw, remaining, present, err := peekTSTField(fields)
	if err != nil {
		return nil, err
	}
	if !present {
		return fields, nil
	}
	if raw.Class != asn1.ClassContextSpecific || raw.Tag != 0 {
		return fields, err
	}
	subject, err := parseTSAField(raw)
	if err != nil {
		return nil, err
	}
	info.TSASubject = subject
	return remaining, nil
}

func parseTSAField(raw asn1.RawValue) ([]byte, error) {
	if !raw.IsCompound || len(raw.Bytes) == 0 {
		return nil, invalidError(nil)
	}
	name, trailing, err := consumeRaw(raw.Bytes)
	if err != nil {
		return nil, err
	}
	if len(trailing) != 0 {
		return nil, invalidError(nil)
	}
	if name.Class != asn1.ClassContextSpecific ||
		name.Tag != generalNameDirectoryNameTag || !name.IsCompound {
		return nil, invalidError(nil)
	}
	subject, err := requireSequence(name.Bytes)
	if err != nil {
		return nil, err
	}
	return subject.FullBytes, nil
}

func peekTSTField(
	fields []byte,
) (asn1.RawValue, []byte, bool, error) {
	if len(fields) == 0 {
		return asn1.RawValue{}, fields, false, nil
	}
	raw, remaining, err := consumeRaw(fields)
	if err != nil {
		return asn1.RawValue{}, nil, false, invalidError(err)
	}
	return raw, remaining, true, nil
}

func parseAccuracy(raw asn1.RawValue) (temporal.Duration, error) {
	wire, err := decodeAccuracyWire(raw)
	if err != nil {
		return temporal.Duration{}, err
	}
	return accuracyFromWire(wire)
}

func decodeAccuracyWire(raw asn1.RawValue) (accuracyWire, error) {
	var wire accuracyWire
	trailing, err := asn1.Unmarshal(raw.FullBytes, &wire)
	if err != nil || len(trailing) != 0 {
		return accuracyWire{}, invalidError(err)
	}
	canonical, marshalErr := asn1.Marshal(wire)
	if marshalErr != nil || !bytes.Equal(canonical, raw.FullBytes) {
		return accuracyWire{}, invalidError(marshalErr)
	}
	if !validAccuracyWire(wire) {
		return accuracyWire{}, invalidError(nil)
	}
	return wire, nil
}

func accuracyFromWire(wire accuracyWire) (temporal.Duration, error) {
	if !validAccuracyWire(wire) {
		return temporal.Duration{}, invalidError(nil)
	}
	// ASN.1 INTEGER decodes to int. validAccuracyWire proves every value is
	// nonnegative before these exact, widening conversions.
	seconds, err := temporal.DurationFromSeconds(uint64(wire.Seconds)) // #nosec G115 -- validated nonnegative int widens exactly to uint64
	if err != nil {
		return temporal.Duration{}, invalidError(nil, err)
	}
	millis, err := temporal.DurationFromMilliseconds(uint64(wire.Millis)) // #nosec G115 -- validated nonnegative int widens exactly to uint64
	if err != nil {
		return temporal.Duration{}, invalidError(nil, err)
	}
	micros, err := temporal.DurationFromMicroseconds(uint64(wire.Micros)) // #nosec G115 -- validated nonnegative int widens exactly to uint64
	if err != nil {
		return temporal.Duration{}, invalidError(nil, err)
	}
	combined, err := seconds.Add(millis)
	if err != nil {
		return temporal.Duration{}, invalidError(nil, err)
	}
	combined, err = combined.Add(micros)
	if err != nil {
		return temporal.Duration{}, invalidError(nil, err)
	}
	if combined.IsZero() {
		return temporal.Duration{}, invalidError(nil)
	}
	return combined, nil
}

func validAccuracyWire(wire accuracyWire) bool {
	return wire.Seconds >= 0 &&
		wire.Millis >= 0 && wire.Millis <= 999 &&
		wire.Micros >= 0 && wire.Micros <= 999
}

func verifyTSTBinding(
	info parsedTSTInfo,
	digest [sha256.Size]byte,
	nonce Nonce,
	authority Authority,
) (TimestampPolicy, error) {
	if info.Version != tstInfoVersion {
		return TimestampPolicyUnknown, invalidError(nil)
	}
	policy, err := policyForAuthority(authority)
	if err != nil || !info.Policy.Equal(policy.oid) {
		return TimestampPolicyUnknown, invalidError(err)
	}
	if !info.MessageImprint.HashAlgorithm.Algorithm.Equal(oidSHA256) ||
		len(info.MessageImprint.HashedMessage) != sha256.Size {
		return TimestampPolicyUnknown, invalidError(nil)
	}
	if subtle.ConstantTimeCompare(info.MessageImprint.HashedMessage, digest[:]) != 1 {
		return TimestampPolicyUnknown, invalidError(nil)
	}
	if !nonce.matches(info.Nonce) {
		return TimestampPolicyUnknown, invalidError(nil)
	}
	return policy.policy, nil
}

func consumeMessageImprint(der []byte) (messageImprint, []byte, error) {
	var imprint messageImprint
	remaining, err := asn1.Unmarshal(der, &imprint)
	if err != nil {
		return messageImprint{}, nil, invalidError(err)
	}
	return imprint, remaining, nil
}

func consumeGenerationTime(der []byte) (temporal.Instant, []byte, error) {
	raw, remaining, err := consumeRaw(der)
	if err != nil || !isUniversal(raw, asn1.TagGeneralizedTime, false) {
		return temporal.Instant{}, nil, invalidError(err)
	}
	var value time.Time
	if trailing, decodeErr := asn1.Unmarshal(raw.FullBytes, &value); decodeErr != nil || len(trailing) != 0 {
		return temporal.Instant{}, nil, invalidError(decodeErr)
	}
	instant, err := temporal.NewInstant(value)
	if err != nil {
		return temporal.Instant{}, nil, invalidError(nil, err)
	}
	return instant, remaining, nil
}

func consumePositiveInteger(der []byte, maximumBits int) (*big.Int, []byte, error) {
	raw, remaining, err := consumeRaw(der)
	if err != nil {
		return nil, nil, invalidError(err)
	}
	value, err := decodePositiveInteger(raw.FullBytes, maximumBits)
	if err != nil {
		return nil, nil, err
	}
	return value, remaining, nil
}

func decodePositiveInteger(der []byte, maximumBits int) (*big.Int, error) {
	var value *big.Int
	trailing, err := asn1.Unmarshal(der, &value)
	if err != nil || len(trailing) != 0 || value == nil || value.Sign() <= 0 ||
		value.BitLen() > maximumBits {
		return nil, invalidError(err)
	}
	return value, nil
}

func consumeInteger(der []byte) (int, []byte, error) {
	var value int
	remaining, err := asn1.Unmarshal(der, &value)
	if err != nil {
		return 0, nil, invalidError(err)
	}
	return value, remaining, nil
}

func consumeOID(der []byte) (asn1.ObjectIdentifier, []byte, error) {
	var value asn1.ObjectIdentifier
	remaining, err := asn1.Unmarshal(der, &value)
	if err != nil {
		return nil, nil, invalidError(err)
	}
	return value, remaining, nil
}

func explicitOctets(raw asn1.RawValue) ([]byte, error) {
	var value []byte
	trailing, err := asn1.Unmarshal(raw.Bytes, &value)
	if err != nil || len(trailing) != 0 || len(value) == 0 {
		return nil, invalidError(err)
	}
	return append([]byte(nil), value...), nil
}

func uniqueAttribute(attributes []cmsAttribute, oid asn1.ObjectIdentifier) (cmsAttribute, error) {
	var found cmsAttribute
	count := 0
	for _, attribute := range attributes {
		if attribute.Type.Equal(oid) {
			found = attribute
			count++
		}
	}
	if count != 1 {
		return cmsAttribute{}, invalidError(nil)
	}
	return found, nil
}

func requireSequence(der []byte) (asn1.RawValue, error) {
	raw, trailing, err := consumeRaw(der)
	if err != nil || len(trailing) != 0 || !isUniversal(raw, asn1.TagSequence, true) {
		return asn1.RawValue{}, invalidError(err)
	}
	return raw, nil
}

func consumeRaw(der []byte) (asn1.RawValue, []byte, error) {
	var raw asn1.RawValue
	remaining, err := asn1.Unmarshal(der, &raw)
	if err != nil {
		return asn1.RawValue{}, nil, invalidError(err)
	}
	return raw, remaining, nil
}

func isUniversal(raw asn1.RawValue, tag int, compound bool) bool {
	return raw.Class == asn1.ClassUniversal && raw.Tag == tag && raw.IsCompound == compound
}

func derTagged(tag byte, body []byte) []byte {
	encoded := make([]byte, 0, 1+derLengthBytes(len(body))+len(body))
	encoded = append(encoded, tag)
	encoded = appendDERLength(encoded, len(body))
	return append(encoded, body...)
}

func derLengthBytes(length int) int {
	if length <= 127 {
		return 1
	}
	count := 0
	for value := length; value > 0; value >>= 8 {
		count++
	}
	return 1 + count
}

func appendDERLength(destination []byte, length int) []byte {
	if length <= 127 {
		// #nosec G115 -- the branch proves length fits in one DER short-form byte.
		return append(destination, byte(length))
	}
	var encoded [8]byte
	index := len(encoded)
	for value := length; value > 0; value >>= 8 {
		index--
		encoded[index] = byte(value)
	}
	destination = append(destination, byte(0x80|len(encoded)-index))
	return append(destination, encoded[index:]...)
}
