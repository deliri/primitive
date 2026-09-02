package runnercontrol

import (
	"crypto"
	"encoding"
	json "encoding/json/v2"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/standard"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	SourceArchiveEntryMaximum uint32 = 1 << 20
	SourceArchiveDepthMaximum uint16 = 256
)

const SourceArchiveSigningDomainToken = "primitive-runner-source-archive-2026-1"

const SourceArchiveDocumentMaximumBytes = 1 << 20

type SourceSigningDomain uint8

const (
	SourceSigningDomainUnknown SourceSigningDomain = iota
	SourceSigningDomainArchiveV1
	sourceSigningDomainLimit
)

func (d SourceSigningDomain) Validate() error {
	if d != SourceSigningDomainArchiveV1 {
		return core.ErrPrimitiveContract
	}
	return nil
}
func (d SourceSigningDomain) IsValid() bool { return d.Validate() == nil }
func (d SourceSigningDomain) String() string {
	if d == SourceSigningDomainArchiveV1 {
		return SourceArchiveSigningDomainToken
	}
	return invalidEnumString()
}
func (d SourceSigningDomain) MarshalText() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return []byte(d.String()), nil
}
func (SourceSigningDomain) ParseCanonicalText(text []byte) (SourceSigningDomain, error) {
	if len(text) > attest.SigningDomainMaximumBytes || string(text) != SourceArchiveSigningDomainToken {
		return SourceSigningDomainUnknown, core.ErrPrimitiveContract
	}
	return SourceSigningDomainArchiveV1, nil
}
func (d SourceSigningDomain) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(d.String())
}
func (d *SourceSigningDomain) UnmarshalJSON(data []byte) error {
	if d == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	parsed, err := SourceSigningDomainUnknown.ParseCanonicalText([]byte(value))
	if err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*d = parsed
	return nil
}

type SourceArchiveManifest struct {
	Repository       standard.RepositoryIdentity `json:"repository"`
	IssuedAt         temporal.Instant            `json:"issued_at"`
	ExpiresAt        temporal.Instant            `json:"expires_at"`
	ArchiveBytes     core.ByteLength             `json:"archive_bytes"`
	FileMaximumBytes core.ByteCount              `json:"file_maximum_bytes"`
	EntryMaximum     uint32                      `json:"entry_maximum"`
	SchemaVersion    uint16                      `json:"schema_version"`
	DepthMaximum     uint16                      `json:"depth_maximum"`
	Commit           core.BuildCommit            `json:"commit"`
	Tree             core.SHA256Digest           `json:"tree_digest"`
	ArchiveDigest    core.SHA256Digest           `json:"archive_digest"`
}

func (m SourceArchiveManifest) Validate() error {
	if m.SchemaVersion != SchemaVersion || m.EntryMaximum == 0 || m.EntryMaximum > SourceArchiveEntryMaximum || m.DepthMaximum == 0 || m.DepthMaximum > SourceArchiveDepthMaximum {
		return core.ErrPrimitiveContract
	}
	if err := errors.Join(m.Repository.Validate(), m.Commit.Validate(), m.Tree.Validate(), m.ArchiveDigest.Validate(), m.ArchiveBytes.Validate(), m.FileMaximumBytes.Validate(), m.IssuedAt.Validate(), m.ExpiresAt.Validate()); err != nil {
		return err
	}
	comparison, err := m.IssuedAt.Compare(m.ExpiresAt)
	if err != nil || comparison != core.ComparisonLess {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	return nil
}
func (SourceArchiveManifest) AttestationDomain() SourceSigningDomain {
	return SourceSigningDomainArchiveV1
}
func (m SourceArchiveManifest) WriteCanonical(destination io.Writer) error {
	if destination == nil {
		return core.ErrPrimitiveContract
	}
	encoded, err := core.MarshalCanonicalJSONDocument(m)
	if err != nil {
		return err
	}
	written, err := destination.Write(encoded)
	if err != nil || written != len(encoded) {
		return errors.Join(core.ErrPrimitiveContract, err, io.ErrShortWrite)
	}
	return nil
}

type SourceArchiveDocument struct {
	Manifest    SourceArchiveManifest                `json:"manifest"`
	Attestation attest.Envelope[SourceSigningDomain] `json:"attestation"`
}

// SourceArchiveVerification binds one authenticated archive to the exact
// observation instant at which a caller intends to admit it.
type SourceArchiveVerification struct {
	Document    SourceArchiveDocument
	TrustedKeys attest.TrustedKeys
	ObservedAt  temporal.Instant
}

func (v SourceArchiveVerification) Validate() error {
	if err := errors.Join(v.Document.Validate(), v.TrustedKeys.Validate(), v.ObservedAt.Validate()); err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	issued, issuedErr := v.ObservedAt.Compare(v.Document.Manifest.IssuedAt)
	expires, expiresErr := v.ObservedAt.Compare(v.Document.Manifest.ExpiresAt)
	if issuedErr != nil || expiresErr != nil || issued == core.ComparisonLess || expires != core.ComparisonLess {
		return errors.Join(core.ErrPrimitiveContract, issuedErr, expiresErr)
	}
	return nil
}

func (SourceArchiveVerification) runnerControlProtocolFact() {}

func (d SourceArchiveDocument) Validate() error {
	if err := errors.Join(d.Manifest.Validate(), d.Attestation.Validate()); err != nil {
		return err
	}
	if d.Attestation.Domain != d.Manifest.AttestationDomain() {
		return core.ErrPrimitiveContract
	}
	return nil
}
func IssueSourceArchive(manifest SourceArchiveManifest, signer crypto.Signer) (SourceArchiveDocument, error) {
	envelope, err := attest.Sign(attest.SignRequest[SourceSigningDomain]{Body: manifest, Signer: signer})
	if err != nil {
		return SourceArchiveDocument{}, err
	}
	document := SourceArchiveDocument{Manifest: manifest, Attestation: envelope}
	return document, document.Validate()
}
func VerifySourceArchive(verification SourceArchiveVerification) error {
	if err := verification.Validate(); err != nil {
		return err
	}
	proof, err := attest.Verify(attest.VerifyRequest[SourceSigningDomain]{Body: verification.Document.Manifest, Envelope: verification.Document.Attestation, TrustedKeys: verification.TrustedKeys})
	if err != nil {
		return err
	}
	return proof.Validate()
}

type sourceArchiveDocumentWire SourceArchiveDocument

func (d SourceArchiveDocument) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(sourceArchiveDocumentWire(d))
	if err != nil || len(encoded) > SourceArchiveDocumentMaximumBytes {
		return nil, errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract, err)
	}
	return encoded, nil
}

func (d *SourceArchiveDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	if len(data) > SourceArchiveDocumentMaximumBytes {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[sourceArchiveDocumentWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := SourceArchiveDocument(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*d = candidate
	return nil
}

type sourceSigningDomainWitness[D attest.SigningDomain[D]] [0]D

var (
	_ core.Validatable            = SourceSigningDomainUnknown
	_ core.ValidatedJSONMarshaler = SourceSigningDomain(0)
	_ encoding.TextMarshaler      = SourceSigningDomainUnknown
	_ json.Unmarshaler            = (*SourceSigningDomain)(nil)
	_ core.Validatable            = SourceArchiveManifest{}
	_ core.Validatable            = SourceArchiveVerification{}
	_ core.ValidatedJSONMarshaler = SourceArchiveDocument{}
	_ json.Unmarshaler            = (*SourceArchiveDocument)(nil)
	_                             = sourceSigningDomainWitness[SourceSigningDomain]{}
)
