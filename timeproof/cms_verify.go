package timeproof

import (
	"bytes"
	"crypto/sha1" // #nosec G505 -- RFC 3161 ESSCertID uses SHA-1 only to identify the already signed certificate.
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"time"

	"github.com/deliri/primitive/v2026/temporal"
)

func verifyTimestampSigner(
	token parsedToken,
	generation temporal.Instant,
	authority Authority,
) error {
	if token.Signer == nil {
		return invalidError(nil)
	}
	generationTime, err := generation.Time()
	if err != nil {
		return invalidError(err)
	}
	if err := verifyTimestampCertificatePolicy(token.Signer, generationTime); err != nil {
		return err
	}
	registry, err := authorityRegistry(authority)
	if err != nil {
		return err
	}
	root := registry.root
	roots, intermediates := timestampCertificatePools(root, token)
	chain, err := verifyTimestampChain(timestampChainRequest{
		signer: token.Signer, root: root, roots: roots,
		intermediates: intermediates, generation: generationTime,
	})
	if err != nil {
		return err
	}
	return verifyNoConflictingCertificates(token.Certificates, chain)
}

// verifyNoConflictingCertificates rejects a different certificate that reuses
// one verified chain member's issuer-and-serial identity. RFC 5816 permits
// unrelated extra certificates; this check closes ambiguity against the one
// verified path. Signer selection separately rejects multiple embedded matches.
func verifyNoConflictingCertificates(
	embedded []*x509.Certificate,
	chain []*x509.Certificate,
) error {
	for _, certificate := range embedded {
		for _, member := range chain {
			if sameCertificateIdentity(certificate, member) &&
				!certificate.Equal(member) {
				return invalidError(nil)
			}
		}
	}
	return nil
}

func sameCertificateIdentity(left, right *x509.Certificate) bool {
	if left == nil || right == nil ||
		left.SerialNumber == nil || right.SerialNumber == nil {
		return false
	}
	return left.SerialNumber.Cmp(right.SerialNumber) == 0 &&
		bytes.Equal(left.RawIssuer, right.RawIssuer)
}

func timestampCertificatePools(root *x509.Certificate, token parsedToken) (*x509.CertPool, *x509.CertPool) {
	roots := x509.NewCertPool()
	roots.AddCert(root)
	intermediates := x509.NewCertPool()
	for _, certificate := range token.Certificates {
		if certificate.Equal(token.Signer) || certificate.Equal(root) {
			continue
		}
		intermediates.AddCert(certificate)
	}
	return roots, intermediates
}

type timestampChainRequest struct {
	signer        *x509.Certificate
	root          *x509.Certificate
	roots         *x509.CertPool
	intermediates *x509.CertPool
	generation    time.Time
}

func verifyTimestampChain(request timestampChainRequest) ([]*x509.Certificate, error) {
	chains, err := request.signer.Verify(x509.VerifyOptions{
		Roots: request.roots, Intermediates: request.intermediates, CurrentTime: request.generation,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageTimeStamping},
	})
	if err != nil || len(chains) != 1 || len(chains[0]) < 2 ||
		!chains[0][len(chains[0])-1].Equal(request.root) {
		return nil, invalidError(err)
	}
	return chains[0], nil
}

func verifyTimestampCertificatePolicy(certificate *x509.Certificate, generationTime time.Time) error {
	if certificate == nil || certificate.IsCA || !certificate.BasicConstraintsValid ||
		generationTime.Before(certificate.NotBefore) || generationTime.After(certificate.NotAfter) ||
		len(certificate.ExtKeyUsage) != 1 ||
		certificate.ExtKeyUsage[0] != x509.ExtKeyUsageTimeStamping ||
		len(certificate.UnknownExtKeyUsage) != 0 ||
		!criticalExtension(certificate, oidExtendedKeyUsage()) {
		return invalidError(nil)
	}
	return nil
}

func criticalExtension(certificate *x509.Certificate, oid asn1.ObjectIdentifier) bool {
	for _, extension := range certificate.Extensions {
		if extension.Id.Equal(oid) {
			return extension.Critical
		}
	}
	return false
}

func verifySigningCertificateAttribute(attributes []cmsAttribute, signer *x509.Certificate) error {
	v1, v1Count := countAttribute(attributes, oidSigningCertificate())
	v2, v2Count := countAttribute(attributes, oidSigningCertificateV2())
	if signer == nil || v1Count+v2Count == 0 ||
		v1Count > 1 || v2Count > 1 {
		return invalidError(nil)
	}
	if v1Count == 1 {
		if err := verifySigningCertificateV1(v1, signer); err != nil {
			return err
		}
	}
	if v2Count == 1 {
		return verifySigningCertificateV2(v2, signer)
	}
	return nil
}

func countAttribute(attributes []cmsAttribute, oid asn1.ObjectIdentifier) (cmsAttribute, int) {
	var found cmsAttribute
	count := 0
	for _, attribute := range attributes {
		if attribute.Type.Equal(oid) {
			found = attribute
			count++
		}
	}
	return found, count
}

func verifySigningCertificateV1(attribute cmsAttribute, signer *x509.Certificate) error {
	certificateID, err := signingCertificateID(attribute)
	if err != nil {
		return err
	}
	hashRaw, fields, err := consumeRaw(certificateID.Bytes)
	if err != nil || !isUniversal(hashRaw, asn1.TagOctetString, false) {
		return invalidError(err)
	}
	var got []byte
	if trailing, decodeErr := asn1.Unmarshal(hashRaw.FullBytes, &got); decodeErr != nil || len(trailing) != 0 {
		return invalidError(decodeErr)
	}
	want := essCertIDV1CertificateHash(signer.Raw)
	if subtle.ConstantTimeCompare(got, want[:]) != 1 {
		return invalidError(nil)
	}
	return verifyOptionalIssuerSerial(fields, signer)
}

// essCertIDV1CertificateHash is the only SHA-1 owner in Primitive. RFC 2634's
// ESSCertID v1 fixes this certificate identifier to SHA-1; CMS signature
// authentication remains independently verified before this comparison.
func essCertIDV1CertificateHash(certificateDER []byte) [sha1.Size]byte {
	return sha1.Sum(certificateDER) // #nosec G401 -- the protocol fixes ESSCertID v1 to SHA-1.
}

func verifySigningCertificateV2(attribute cmsAttribute, signer *x509.Certificate) error {
	certificateID, err := signingCertificateID(attribute)
	if err != nil {
		return err
	}
	hashOID, hashRaw, remaining, err := consumeESSV2Hash(certificateID.Bytes)
	if err != nil {
		return err
	}
	got, err := decodeOctetString(hashRaw)
	if err != nil {
		return invalidError(err)
	}
	want, err := certificateDigest(hashOID, signer.Raw)
	if err != nil || subtle.ConstantTimeCompare(got, want) != 1 {
		return invalidError(err)
	}
	return verifyOptionalIssuerSerial(remaining, signer)
}

func consumeESSV2Hash(fields []byte) (asn1.ObjectIdentifier, asn1.RawValue, []byte, error) {
	hashOID := oidSHA256()
	first, remaining, err := consumeRaw(fields)
	if err != nil {
		return nil, asn1.RawValue{}, nil, invalidError(err)
	}
	if !isUniversal(first, asn1.TagSequence, true) {
		return hashOID, first, remaining, nil
	}
	var algorithm pkix.AlgorithmIdentifier
	if trailing, decodeErr := asn1.Unmarshal(first.FullBytes, &algorithm); decodeErr != nil || len(trailing) != 0 {
		return nil, asn1.RawValue{}, nil, invalidError(decodeErr)
	}
	// RFC 5035 defines SHA-256 as ESSCertIDv2's DEFAULT. DER requires a
	// sequence component equal to its default to be omitted, so an explicit
	// SHA-256 AlgorithmIdentifier is an alternate spelling of the omitted form.
	if algorithm.Algorithm.Equal(oidSHA256()) {
		return nil, asn1.RawValue{}, nil, invalidError(nil)
	}
	hashRaw, remaining, err := consumeRaw(remaining)
	if err != nil {
		return nil, asn1.RawValue{}, nil, invalidError(err)
	}
	return algorithm.Algorithm, hashRaw, remaining, nil
}

func decodeOctetString(raw asn1.RawValue) ([]byte, error) {
	if !isUniversal(raw, asn1.TagOctetString, false) {
		return nil, invalidError(nil)
	}
	var value []byte
	if trailing, err := asn1.Unmarshal(raw.FullBytes, &value); err != nil || len(trailing) != 0 {
		return nil, invalidError(err)
	}
	return value, nil
}

func signingCertificateID(attribute cmsAttribute) (asn1.RawValue, error) {
	if len(attribute.Values) != 1 {
		return asn1.RawValue{}, invalidError(nil)
	}
	signingCertificate, err := requireSequence(attribute.Values[0].FullBytes)
	if err != nil {
		return asn1.RawValue{}, err
	}
	certificates, trailing, err := consumeRaw(signingCertificate.Bytes)
	if err != nil || !isUniversal(certificates, asn1.TagSequence, true) {
		return asn1.RawValue{}, invalidError(err)
	}
	if len(trailing) != 0 {
		return asn1.RawValue{}, invalidError(nil)
	}
	certificateID, remaining, err := consumeRaw(certificates.Bytes)
	if err != nil || len(remaining) != 0 || !isUniversal(certificateID, asn1.TagSequence, true) {
		return asn1.RawValue{}, invalidError(err)
	}
	return certificateID, nil
}

func certificateDigest(oid asn1.ObjectIdentifier, raw []byte) ([]byte, error) {
	switch {
	case oid.Equal(oidSHA256()):
		sum := sha256.Sum256(raw)
		return sum[:], nil
	case oid.Equal(oidSHA384()):
		sum := sha512.Sum384(raw)
		return sum[:], nil
	case oid.Equal(oidSHA512()):
		sum := sha512.Sum512(raw)
		return sum[:], nil
	default:
		return nil, invalidError(nil)
	}
}

func verifyOptionalIssuerSerial(fields []byte, signer *x509.Certificate) error {
	if len(fields) == 0 {
		return nil
	}
	issuerSerial, trailing, err := consumeRaw(fields)
	if err != nil || len(trailing) != 0 || !isUniversal(issuerSerial, asn1.TagSequence, true) {
		return invalidError(err)
	}
	generalNames, remainder, err := consumeRaw(issuerSerial.Bytes)
	if err != nil || !isUniversal(generalNames, asn1.TagSequence, true) {
		return invalidError(err)
	}
	if err := validateIssuerSerial(remainder, signer); err != nil {
		return err
	}
	if err := validateGeneralNamesIssuer(generalNames.Bytes, signer.RawIssuer); err != nil {
		return err
	}
	return nil
}

func validateIssuerSerial(fields []byte, signer *x509.Certificate) error {
	serial, trailing, err := consumePositiveInteger(fields, SerialMaximumBits)
	if err != nil {
		return err
	}
	if len(trailing) != 0 || serial.Cmp(signer.SerialNumber) != 0 {
		return invalidError(nil)
	}
	return nil
}

func validateGeneralNamesIssuer(fields []byte, rawIssuer []byte) error {
	found := false
	for len(fields) != 0 {
		name, remaining, err := consumeRaw(fields)
		if err != nil {
			return err
		}
		if name.Class == asn1.ClassContextSpecific && name.Tag == generalNameDirectoryNameTag &&
			bytes.Equal(name.Bytes, rawIssuer) {
			found = true
		}
		fields = remaining
	}
	if !found {
		return invalidError(nil)
	}
	return nil
}
