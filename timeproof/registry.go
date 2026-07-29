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
)

//go:embed trust_anchors/freetsa_root.pem
var trustAnchors embed.FS

type authorityContract struct {
	root      *x509.Certificate
	policyOID asn1.ObjectIdentifier
	policy    TimestampPolicy
}

func authorityRegistry(authority Authority) (authorityContract, error) {
	if authority != AuthorityFreeTSA {
		return authorityContract{}, contractError(nil)
	}
	root, err := freeTSARoot()
	if err != nil {
		return authorityContract{}, err
	}
	return authorityContract{
		root:      root,
		policyOID: freeTSAPolicyOID,
		policy:    TimestampPolicyFreeTSA,
	}, nil
}

// freeTSARoot decodes and re-proves the pinned anchor once per process. The
// anchor is a compile-time fact, so repeating the parse and the self-signature
// check on every validation would buy no additional guarantee.
var freeTSARoot = sync.OnceValues(loadFreeTSARoot)

func loadFreeTSARoot() (*x509.Certificate, error) {
	raw, err := trustAnchors.ReadFile("trust_anchors/freetsa_root.pem")
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
	if err := verifyFreeTSARoot(root); err != nil {
		return nil, err
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
