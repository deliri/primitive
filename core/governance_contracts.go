package core

import (
	"crypto/sha256"
	"errors"
	"io"
)

const (
	testingProtocolRelativePathText              = "_docs/testing_protocol.md"
	testingProtocolSHA256HexText                 = "190958a7943de8a5d37b1f5a08632331f8ddedd3beb0da9f3ac7e00c7b39c533"
	testingProtocolBytes                  uint64 = 50_558
	governanceDocumentIdentityDiagnostic         = "governance document identity is not admitted"
	governanceContractCanonicalDiagnostic        = "governance document contract is not canonical"
	governanceDocumentNilSourceDiagnostic        = "governance document source is nil"
	governanceDocumentLengthDiagnostic           = "governance document length is not the canonical length"
	governanceDocumentDigestDiagnostic           = "governance document digest is not the canonical digest"
)

// GovernanceDocument is one closed Primitive governance-document identity.
type GovernanceDocument uint8

const (
	// GovernanceDocumentUnknown is the invalid zero document identity.
	GovernanceDocumentUnknown GovernanceDocument = iota
	// GovernanceDocumentTestingProtocol identifies Primitive's canonical
	// testing doctrine.
	GovernanceDocumentTestingProtocol
	governanceDocumentLimit
)

// GovernanceDocumentContract binds one document identity to its canonical
// repository path, exact content length, and content digest. Verify is the
// single admitted way to decide whether a candidate document satisfies it, so
// no consumer re-derives the streaming length-and-digest rule.
type GovernanceDocumentContract struct {
	// Path is the document's root-relative repository path.
	Path RelativePath
	// Bytes is the exact canonical document length.
	Bytes ByteLength
	// SHA256 is the digest of the exact canonical document bytes.
	SHA256 SHA256Digest
	// Document is the canonical document identity.
	Document GovernanceDocument
}

// Validate rejects unknown and future governance-document identities.
func (d GovernanceDocument) Validate() error {
	if d <= GovernanceDocumentUnknown || d >= governanceDocumentLimit {
		return governanceContractError(governanceDocumentIdentityDiagnostic)
	}
	return nil
}

// Contract returns the complete canonical contract for d.
func (d GovernanceDocument) Contract() (GovernanceDocumentContract, error) {
	if err := d.Validate(); err != nil {
		return GovernanceDocumentContract{}, err
	}
	contract, err := canonicalGovernanceDocumentContract(d)
	if err != nil {
		return GovernanceDocumentContract{}, err
	}
	if err := contract.Validate(); err != nil {
		return GovernanceDocumentContract{}, err
	}
	return contract, nil
}

// Validate rejects incomplete or noncanonical document contracts.
func (c GovernanceDocumentContract) Validate() error {
	if err := c.Document.Validate(); err != nil {
		return err
	}
	// Path and SHA256 own their own field rules, but a governance caller must be
	// able to catch every contract failure through one identity. Wrapping keeps
	// the owning type's diagnostic while making the governance identity total.
	if err := c.Path.Validate(); err != nil {
		return errors.Join(ErrGovernanceContract, err)
	}
	if err := c.SHA256.Validate(); err != nil {
		return errors.Join(ErrGovernanceContract, err)
	}
	canonical, err := canonicalGovernanceDocumentContract(c.Document)
	if err != nil {
		return err
	}
	if c != canonical {
		return governanceContractError(governanceContractCanonicalDiagnostic)
	}
	return nil
}

// Verify streams document and admits it only when its length and digest both
// equal the canonical values. It reads at most one byte beyond the canonical
// length, so an extended document is rejected without consuming the excess, and
// it retains no document content: memory stays O(1) regardless of size.
//
// A length violation is reported before a digest violation because a truncated
// or extended document has no canonical digest to disagree with.
func (c GovernanceDocumentContract) Verify(document io.Reader) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if document == nil {
		return governanceDocumentError(ErrGovernanceDocumentSource, governanceDocumentNilSourceDiagnostic)
	}
	canonicalBytes, err := c.Bytes.Int64()
	if err != nil {
		return err
	}
	hasher := sha256.New()
	streamed, err := io.Copy(hasher, io.LimitReader(document, canonicalBytes+1))
	if err != nil {
		return errors.Join(ErrGovernanceDocumentSource, err)
	}
	if streamed != canonicalBytes {
		return governanceDocumentError(ErrGovernanceDocumentLength, governanceDocumentLengthDiagnostic)
	}
	var streamedDigest [sha256.Size]byte
	copy(streamedDigest[:], hasher.Sum(nil))
	if NewSHA256Digest(streamedDigest) != c.SHA256 {
		return governanceDocumentError(ErrGovernanceDocumentDigest, governanceDocumentDigestDiagnostic)
	}
	return nil
}

func canonicalGovernanceDocumentContract(document GovernanceDocument) (GovernanceDocumentContract, error) {
	switch document {
	case GovernanceDocumentTestingProtocol:
		return testingProtocolGovernanceContract()
	default:
		return GovernanceDocumentContract{}, governanceContractError(governanceDocumentIdentityDiagnostic)
	}
}

func testingProtocolGovernanceContract() (GovernanceDocumentContract, error) {
	path, err := ParseRelativePath(testingProtocolRelativePathText)
	if err != nil {
		return GovernanceDocumentContract{}, err
	}
	digest, err := ParseSHA256Hex(testingProtocolSHA256HexText)
	if err != nil {
		return GovernanceDocumentContract{}, err
	}
	return GovernanceDocumentContract{
		Path:     path,
		Bytes:    NewByteLength(testingProtocolBytes),
		SHA256:   digest,
		Document: GovernanceDocumentTestingProtocol,
	}, nil
}

func governanceContractError(message string) error {
	return governanceDocumentError(ErrGovernanceContract, message)
}

func governanceDocumentError(identity ErrorIdentity, message string) error {
	return errors.Join(identity, errors.New(message))
}
