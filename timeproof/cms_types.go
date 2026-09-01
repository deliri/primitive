package timeproof

import (
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"math/big"
)

type cmsEncapsulatedContent struct {
	ContentType asn1.ObjectIdentifier
	Content     asn1.RawValue `asn1:"tag:0,explicit,optional"`
}

type cmsIssuerAndSerial struct {
	Serial *big.Int
	Issuer asn1.RawValue
}

type cmsSignerInfo struct {
	SID                cmsIssuerAndSerial
	DigestAlgorithm    pkix.AlgorithmIdentifier
	SignedAttributes   asn1.RawValue
	SignatureAlgorithm pkix.AlgorithmIdentifier
	Signature          []byte
	Version            int
}

type cmsAttribute struct {
	Type   asn1.ObjectIdentifier
	Values []asn1.RawValue `asn1:"set"`
}

type parsedSignedData struct {
	Content          cmsEncapsulatedContent
	DigestAlgorithms []pkix.AlgorithmIdentifier
	Certificates     []*x509.Certificate
	Signers          []cmsSignerInfo
	Version          int
}

type parsedToken struct {
	Signer       *x509.Certificate
	TSTDER       []byte
	Certificates []*x509.Certificate
	Attributes   []cmsAttribute
	SignerInfo   cmsSignerInfo
}

type parsedTSTInfo struct {
	Serial         *big.Int
	Nonce          *big.Int
	MessageImprint messageImprint
	Policy         asn1.ObjectIdentifier
	TSASubject     []byte
	Time           AuthoritativeTime
	Version        int
}

type verifiedToken struct {
	Serial       *big.Int
	Time         AuthoritativeTime
	SignerSHA256 [sha256.Size]byte
	Policy       TimestampPolicy
}

type accuracyWire struct {
	Seconds int `asn1:"optional"`
	Millis  int `asn1:"tag:0,optional"`
	Micros  int `asn1:"tag:1,optional"`
}
