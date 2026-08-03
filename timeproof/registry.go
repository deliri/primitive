package timeproof

import (
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"embed"
	"encoding/asn1"
	"encoding/pem"
	"math/big"
	"sync"
	"time"

	"github.com/deliri/primitive/v2026/core"
)

const (
	freeTSAEndpointText  = "https://freetsa.org/tsr"
	digiCertEndpointText = "http://timestamp.digicert.com"
)

//go:embed trust_anchors/freetsa_root.pem trust_anchors/digicert_trusted_root_g4.pem
var trustAnchors embed.FS

type authorityContract struct {
	root      *x509.Certificate
	policyOID asn1.ObjectIdentifier
	endpoint  core.HTTPEndpoint
	policy    TimestampPolicy
}

func authorityRegistry(authority Authority) (authorityContract, error) {
	var (
		endpointText string
		root         *x509.Certificate
		policyOID    asn1.ObjectIdentifier
		policy       TimestampPolicy
		err          error
	)
	switch authority {
	case AuthorityFreeTSA:
		endpointText = freeTSAEndpointText
		root, err = freeTSARoot()
		policyOID = freeTSAPolicyOID
		policy = TimestampPolicyFreeTSA
	case AuthorityDigiCert:
		endpointText = digiCertEndpointText
		root, err = digiCertRoot()
		policyOID = digiCertPolicyOID
		policy = TimestampPolicyDigiCert
	default:
		return authorityContract{}, contractError(nil)
	}
	if err != nil {
		return authorityContract{}, err
	}
	endpoint, err := core.ParseHTTPEndpoint(endpointText)
	if err != nil {
		return authorityContract{}, contractError(err)
	}
	return authorityContract{
		endpoint: endpoint, root: root, policyOID: policyOID, policy: policy,
	}, nil
}

// freeTSARoot decodes and re-proves the pinned anchor once per process. The
// anchor is a compile-time fact, so repeating the parse and the self-signature
// check on every validation would buy no additional guarantee.
var freeTSARoot = sync.OnceValues(loadFreeTSARoot)

func loadFreeTSARoot() (*x509.Certificate, error) {
	root, err := loadEmbeddedRoot("trust_anchors/freetsa_root.pem")
	if err != nil {
		return nil, err
	}
	if err := verifyFreeTSARoot(root); err != nil {
		return nil, err
	}
	return root, nil
}

// digiCertRoot decodes and re-proves the reviewed DigiCert root once per
// process. Verification never delegates authority identity to the host trust
// store, whose contents vary by machine and time.
var digiCertRoot = sync.OnceValues(loadDigiCertRoot)

func loadDigiCertRoot() (*x509.Certificate, error) {
	root, err := loadEmbeddedRoot(
		"trust_anchors/digicert_trusted_root_g4.pem",
	)
	if err != nil {
		return nil, err
	}
	if err := verifyDigiCertRoot(root); err != nil {
		return nil, err
	}
	return root, nil
}

func loadEmbeddedRoot(path string) (*x509.Certificate, error) {
	raw, err := trustAnchors.ReadFile(path)
	if err != nil {
		return nil, invalidError(err)
	}
	block, trailing := pem.Decode(raw)
	if block == nil || block.Type != "CERTIFICATE" ||
		len(bytes.TrimSpace(trailing)) != 0 {
		return nil, invalidError(nil)
	}
	root, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, invalidError(err)
	}
	return root, nil
}

func verifyFreeTSARoot(root *x509.Certificate) error {
	if root == nil {
		return invalidError(nil)
	}
	wantDigest := [sha256.Size]byte{
		0xa6, 0x37, 0x9e, 0x7c, 0xec, 0xc0, 0x5f, 0xaa,
		0x3c, 0xbf, 0x07, 0x60, 0x13, 0xd7, 0x45, 0xe3,
		0x27, 0xbb, 0xba, 0xa3, 0x8c, 0x0b, 0x9a, 0xf2,
		0x24, 0x69, 0xd4, 0x70, 0x1d, 0x18, 0xaa, 0xbc,
	}
	wantSerial := new(big.Int).SetBytes(
		[]byte{0xc1, 0xe9, 0x86, 0x16, 0x0d, 0xa8, 0xe9, 0x80},
	)
	wantBefore := time.Date(2016, time.March, 13, 1, 52, 13, 0, time.UTC)
	wantAfter := time.Date(2041, time.March, 7, 1, 52, 13, 0, time.UTC)
	if sha256.Sum256(root.Raw) != wantDigest ||
		root.SerialNumber.Cmp(wantSerial) != 0 ||
		!root.NotBefore.Equal(wantBefore) ||
		!root.NotAfter.Equal(wantAfter) {
		return invalidError(nil)
	}
	if err := root.CheckSignatureFrom(root); err != nil {
		return invalidError(err)
	}
	return nil
}

func verifyDigiCertRoot(root *x509.Certificate) error {
	if root == nil {
		return invalidError(nil)
	}
	wantDigest := [sha256.Size]byte{
		0x55, 0x2f, 0x7b, 0xdc, 0xf1, 0xa7, 0xaf, 0x9e,
		0x6c, 0xe6, 0x72, 0x01, 0x7f, 0x4f, 0x12, 0xab,
		0xf7, 0x72, 0x40, 0xc7, 0x8e, 0x76, 0x1a, 0xc2,
		0x03, 0xd1, 0xd9, 0xd2, 0x0a, 0xc8, 0x99, 0x88,
	}
	wantSerial := new(big.Int).SetBytes(
		[]byte{
			0x05, 0x9b, 0x1b, 0x57, 0x9e, 0x8e, 0x21, 0x32,
			0xe2, 0x39, 0x07, 0xbd, 0xa7, 0x77, 0x75, 0x5c,
		},
	)
	wantBefore := time.Date(2013, time.August, 1, 12, 0, 0, 0, time.UTC)
	wantAfter := time.Date(2038, time.January, 15, 12, 0, 0, 0, time.UTC)
	if sha256.Sum256(root.Raw) != wantDigest ||
		root.SerialNumber.Cmp(wantSerial) != 0 ||
		!root.NotBefore.Equal(wantBefore) ||
		!root.NotAfter.Equal(wantAfter) {
		return invalidError(nil)
	}
	if err := root.CheckSignatureFrom(root); err != nil {
		return invalidError(err)
	}
	return nil
}
