package release

import (
	"crypto"
	"encoding/json"
	"errors"
	"hash/crc32"
	"io"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// ManifestIdentity is the nominal digest of one manifest's signed facts.
type ManifestIdentity struct {
	digest core.SHA256Digest
}

func newManifestIdentity(digest core.SHA256Digest) ManifestIdentity {
	return ManifestIdentity{digest: digest}
}

func (i ManifestIdentity) Validate() error { return i.digest.Validate() }
func (i ManifestIdentity) String() string {
	value, _ := i.digest.Hex()
	return value
}
func (i ManifestIdentity) MarshalJSON() ([]byte, error) { return i.digest.MarshalJSON() }
func (i *ManifestIdentity) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError(errors.New("manifest identity receiver is nil"))
	}
	var digest core.SHA256Digest
	if err := json.Unmarshal(data, &digest); err != nil {
		return jsonError(err)
	}
	*i = newManifestIdentity(digest)
	return nil
}

// ManifestDocumentDigest names exact canonical document bytes, including the
// signer and signature.
type ManifestDocumentDigest struct {
	digest core.SHA256Digest
}

func newManifestDocumentDigest(digest core.SHA256Digest) ManifestDocumentDigest {
	return ManifestDocumentDigest{digest: digest}
}

func (d ManifestDocumentDigest) Validate() error { return d.digest.Validate() }
func (d ManifestDocumentDigest) String() string {
	value, _ := d.digest.Hex()
	return value
}

func (d ManifestDocumentDigest) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := d.digest.MarshalJSON()
	if err != nil {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (d *ManifestDocumentDigest) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("manifest document digest receiver is nil"))
	}
	var digest core.SHA256Digest
	if err := json.Unmarshal(data, &digest); err != nil {
		return jsonError(err)
	}
	candidate := newManifestDocumentDigest(digest)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

// SHA256 returns the exact authenticated manifest-document digest.
func (d ManifestDocumentDigest) SHA256() core.SHA256Digest { return d.digest }

// ManifestFactRequest supplies the facts an artifact producer asks a manifest
// authority to sign.
type ManifestFactRequest struct {
	Provenance BuildProvenance
	Artifacts  ArtifactSet
	Metadata   MetadataSet
	CreatedAt  temporal.Instant
	Version    core.ReleaseVersion
	Commit     core.BuildCommit
	Revision   Revision
	Offering   core.Offering
}

// Validate proves every manifest fact and its artifact/build bindings before
// identity derivation.
func (r ManifestFactRequest) Validate() error {
	total, err := r.Artifacts.TotalExtent()
	if err != nil {
		return manifestError(err)
	}
	return (ManifestFact{
		revision: r.Revision, offering: r.Offering, version: r.Version,
		commit: r.Commit, createdAt: r.CreatedAt, totalExtent: total,
		artifacts: r.Artifacts, provenance: r.Provenance, metadata: r.Metadata,
	}).validateWithoutIdentity()
}

// ManifestFact is the immutable canonical body authenticated by Attest.
type ManifestFact struct {
	provenance  BuildProvenance
	artifacts   ArtifactSet
	metadata    MetadataSet
	createdAt   temporal.Instant
	totalExtent core.ByteCount
	version     core.ReleaseVersion
	identity    ManifestIdentity
	commit      core.BuildCommit
	revision    Revision
	offering    core.Offering
	valid       bool
}

type manifestFactWire struct {
	Identity    *ManifestIdentity    `json:"identity"`
	Revision    *Revision            `json:"revision"`
	Offering    *core.Offering       `json:"offering"`
	Version     *core.ReleaseVersion `json:"version"`
	Commit      *core.BuildCommit    `json:"commit"`
	CreatedAt   *temporal.Instant    `json:"created_at"`
	TotalExtent *core.ByteCount      `json:"total_extent_bytes"`
	Artifacts   *ArtifactSet         `json:"artifacts"`
	Provenance  *BuildProvenance     `json:"provenance"`
	Metadata    *MetadataSet         `json:"metadata"`
}

type manifestIdentityWire struct {
	Provenance  BuildProvenance     `json:"provenance"`
	Artifacts   ArtifactSet         `json:"artifacts"`
	Metadata    MetadataSet         `json:"metadata"`
	CreatedAt   temporal.Instant    `json:"created_at"`
	TotalExtent core.ByteCount      `json:"total_extent_bytes"`
	Version     core.ReleaseVersion `json:"version"`
	Commit      core.BuildCommit    `json:"commit"`
	Revision    Revision            `json:"revision"`
	Offering    core.Offering       `json:"offering"`
}

func NewManifestFact(request ManifestFactRequest) (ManifestFact, error) {
	if err := request.Validate(); err != nil {
		return ManifestFact{}, err
	}
	total, err := request.Artifacts.TotalExtent()
	if err != nil {
		return ManifestFact{}, manifestError(err)
	}
	candidate := ManifestFact{
		revision: request.Revision, offering: request.Offering,
		version: request.Version, commit: request.Commit,
		createdAt: request.CreatedAt, totalExtent: total,
		artifacts: request.Artifacts, provenance: request.Provenance,
		metadata: request.Metadata,
	}
	if err := candidate.validateWithoutIdentity(); err != nil {
		return ManifestFact{}, err
	}
	digest, err := manifestFactDigest(candidate)
	if err != nil {
		return ManifestFact{}, err
	}
	candidate.identity = newManifestIdentity(digest)
	candidate.valid = true
	if err := candidate.Validate(); err != nil {
		return ManifestFact{}, err
	}
	return candidate, nil
}

func (f ManifestFact) validateWithoutIdentity() error {
	for _, err := range []error{
		f.revision.Validate(), f.offering.Validate(), f.version.Validate(),
		f.commit.Validate(), f.createdAt.Validate(), f.totalExtent.Validate(),
		f.artifacts.Validate(), f.provenance.Validate(), f.metadata.Validate(),
	} {
		if err != nil {
			return manifestError(err)
		}
	}
	// f.artifacts.Validate has already proven the set, so read its sealed
	// total and slots directly instead of revalidating once per access.
	if f.artifacts.total != f.totalExtent {
		return manifestError(errors.New("manifest total extent differs from its artifacts"))
	}
	return validateManifestBuildBindings(f)
}

func validateManifestBuildBindings(f ManifestFact) error {
	for _, artifact := range f.artifacts.artifacts {
		build := artifact.Build()
		if build.Offering() != f.offering ||
			build.Version() != f.version ||
			build.Commit() != f.commit {
			return manifestError(errors.New("manifest facts differ from an artifact build"))
		}
	}
	return nil
}

func (f ManifestFact) Validate() error {
	if !f.valid {
		return manifestError(errors.New("manifest fact is unset"))
	}
	if err := f.identity.Validate(); err != nil {
		return manifestError(err)
	}
	if err := f.validateWithoutIdentity(); err != nil {
		return err
	}
	digest, err := manifestFactDigest(f)
	if err != nil || f.identity != newManifestIdentity(digest) {
		return manifestError(errors.New("manifest identity does not name its facts"), err)
	}
	return nil
}

func (f ManifestFact) Identity() ManifestIdentity   { return f.identity }
func (f ManifestFact) Revision() Revision           { return f.revision }
func (f ManifestFact) Offering() core.Offering      { return f.offering }
func (f ManifestFact) Version() core.ReleaseVersion { return f.version }
func (f ManifestFact) Commit() core.BuildCommit     { return f.commit }
func (f ManifestFact) CreatedAt() temporal.Instant  { return f.createdAt }
func (f ManifestFact) TotalExtent() core.ByteCount  { return f.totalExtent }
func (f ManifestFact) Artifacts() ArtifactSet       { return f.artifacts }
func (f ManifestFact) Provenance() BuildProvenance  { return f.provenance }
func (f ManifestFact) Metadata() MetadataSet        { return f.metadata }
func (ManifestFact) AttestationDomain() Domain      { return DomainManifestV1 }

func (f ManifestFact) MarshalJSON() ([]byte, error) {
	if err := f.Validate(); err != nil {
		return nil, err
	}
	identity, revision := f.identity, f.revision
	offering, version, commit := f.offering, f.version, f.commit
	createdAt, total, artifacts := f.createdAt, f.totalExtent, f.artifacts
	provenance, metadata := f.provenance, f.metadata
	return json.Marshal(manifestFactWire{
		Identity: &identity, Revision: &revision, Offering: &offering,
		Version: &version, Commit: &commit, CreatedAt: &createdAt,
		TotalExtent: &total, Artifacts: &artifacts,
		Provenance: &provenance, Metadata: &metadata,
	})
}

func (f *ManifestFact) UnmarshalJSON(data []byte) error {
	if f == nil {
		return jsonError(errors.New("manifest fact receiver is nil"))
	}
	wire, err := decodeStructure[manifestFactWire](data)
	if err != nil {
		return err
	}
	if manifestWireMissing(wire) {
		return jsonError(errors.New("manifest fact field is missing"))
	}
	candidate := ManifestFact{
		identity: *wire.Identity, revision: *wire.Revision,
		offering: *wire.Offering, version: *wire.Version, commit: *wire.Commit,
		createdAt: *wire.CreatedAt, totalExtent: *wire.TotalExtent,
		artifacts: *wire.Artifacts, provenance: *wire.Provenance,
		metadata: *wire.Metadata, valid: true,
	}
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*f = candidate
	return nil
}

func manifestWireMissing(w manifestFactWire) bool {
	return w.Identity == nil || w.Revision == nil || w.Offering == nil ||
		w.Version == nil || w.Commit == nil || w.CreatedAt == nil ||
		w.TotalExtent == nil || w.Artifacts == nil || w.Provenance == nil ||
		w.Metadata == nil
}

func (f ManifestFact) WriteCanonical(destination io.Writer) error {
	if destination == nil {
		return manifestError(errors.New(canonicalDestinationNilDiagnostic))
	}
	encoded, err := f.MarshalJSON()
	if err != nil {
		return err
	}
	written, err := destination.Write(encoded)
	if err != nil {
		return manifestError(err)
	}
	if written != len(encoded) {
		return manifestError(io.ErrShortWrite)
	}
	return nil
}

func manifestFactDigest(f ManifestFact) (core.SHA256Digest, error) {
	body, err := json.Marshal(manifestIdentityWire{
		Revision: f.revision, Offering: f.offering, Version: f.version,
		Commit: f.commit, CreatedAt: f.createdAt,
		TotalExtent: f.totalExtent, Artifacts: f.artifacts,
		Provenance: f.provenance, Metadata: f.metadata,
	})
	if err != nil {
		return core.SHA256Digest{}, manifestError(err)
	}
	return framedDigest(manifestIdentityDomain, body), nil
}

// ManifestDocument is an untrusted fact and structural Attest envelope.
type ManifestDocument struct {
	Fact        ManifestFact            `json:"fact"`
	Attestation attest.Envelope[Domain] `json:"attestation"`
}

func (d ManifestDocument) Validate() error {
	if err := d.Fact.Validate(); err != nil {
		return manifestError(err)
	}
	if err := d.Attestation.Validate(); err != nil {
		return verificationError(err)
	}
	if d.Attestation.Domain != DomainManifestV1 {
		return verificationError(errors.New("manifest attestation domain differs from its body"))
	}
	return nil
}

func (d ManifestDocument) MarshalJSON() ([]byte, error) {
	type wire ManifestDocument
	if err := d.Validate(); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(wire(d))
	if err != nil || len(encoded) > documentExtentMaximum {
		return nil, jsonError(errors.New("manifest document extent exceeded"), err)
	}
	return encoded, nil
}

func (d *ManifestDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("manifest document receiver is nil"))
	}
	type wire struct {
		Fact        *ManifestFact            `json:"fact"`
		Attestation *attest.Envelope[Domain] `json:"attestation"`
	}
	decoded, err := decodeStructure[wire](data)
	if err != nil {
		return err
	}
	if decoded.Fact == nil || decoded.Attestation == nil {
		return jsonError(errors.New("manifest document field is missing"))
	}
	candidate := ManifestDocument{Fact: *decoded.Fact, Attestation: *decoded.Attestation}
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

type IssueManifestRequest struct {
	Signer crypto.Signer
	Fact   ManifestFact
}

// Validate delegates signing-key custody and body validation to Attest.
func (r IssueManifestRequest) Validate() error {
	if err := (attest.SignRequest[Domain]{Body: r.Fact, Signer: r.Signer}).Validate(); err != nil {
		return manifestError(err)
	}
	return nil
}

func IssueManifest(request IssueManifestRequest) (ManifestDocument, error) {
	if err := request.Validate(); err != nil {
		return ManifestDocument{}, err
	}
	envelope, err := attest.Sign(attest.SignRequest[Domain]{Body: request.Fact, Signer: request.Signer})
	if err != nil {
		return ManifestDocument{}, manifestError(err)
	}
	document := ManifestDocument{Fact: request.Fact, Attestation: envelope}
	if err := document.Validate(); err != nil {
		return ManifestDocument{}, err
	}
	return document, nil
}

type VerifyManifestRequest struct {
	Document         ManifestDocument
	TrustedKeys      attest.TrustedKeys
	ExpectedOffering core.Offering
}

// Validate proves document structure, caller authority, and expected stream.
func (r VerifyManifestRequest) Validate() error {
	if err := r.ExpectedOffering.Validate(); err != nil {
		return verificationError(err)
	}
	if err := r.Document.Validate(); err != nil {
		return verificationError(err)
	}
	if err := r.TrustedKeys.Validate(); err != nil {
		return verificationError(err)
	}
	return nil
}

// VerifiedManifest is a private-witness proof that one exact manifest
// document authenticated against caller-selected authority.
type VerifiedManifest struct {
	document  ManifestDocument
	proof     attest.Verified[Domain]
	integrity ArtifactIntegrity
	digest    ManifestDocumentDigest
	seal      verificationSeal
}

func VerifyManifest(request VerifyManifestRequest) (VerifiedManifest, error) {
	if err := request.Validate(); err != nil {
		return VerifiedManifest{}, err
	}
	if request.Document.Fact.Offering() != request.ExpectedOffering {
		return VerifiedManifest{}, offeringMismatchError(
			request.Document.Fact.Offering(),
			request.ExpectedOffering,
		)
	}
	proof, err := attest.Verify(attest.VerifyRequest[Domain]{
		Body: request.Document.Fact, Envelope: request.Document.Attestation,
		TrustedKeys: request.TrustedKeys,
	})
	if err != nil {
		return VerifiedManifest{}, verificationError(err)
	}
	integrity, err := integrityManifestDocument(request.Document)
	if err != nil {
		return VerifiedManifest{}, verificationError(err)
	}
	result := VerifiedManifest{
		document: request.Document, integrity: integrity,
		digest: newManifestDocumentDigest(integrity.SHA256()),
		proof:  proof,
	}
	result.seal = verificationSealAuthenticated
	return result, nil
}

// Validate proves that VerifyManifest issued the private witness and that its
// complete closure was authenticated. Authentication and document hashing run
// once, before the seal is issued, rather than on every accessor.
func (v VerifiedManifest) Validate() error {
	if v.seal != verificationSealAuthenticated {
		return verificationError(errors.New("verified manifest proof is unset"))
	}
	return nil
}

// Accessors project the immutable authenticated value. Operations accepting a
// VerifiedManifest validate its private seal once at ingress.
func (v VerifiedManifest) Document() ManifestDocument             { return v.document }
func (v VerifiedManifest) Identity() ManifestIdentity             { return v.document.Fact.Identity() }
func (v VerifiedManifest) DocumentDigest() ManifestDocumentDigest { return v.digest }
func (v VerifiedManifest) DocumentIntegrity() ArtifactIntegrity   { return v.integrity }
func (v VerifiedManifest) Offering() core.Offering                { return v.document.Fact.Offering() }
func (v VerifiedManifest) Version() core.ReleaseVersion           { return v.document.Fact.Version() }
func (v VerifiedManifest) Artifacts() ArtifactSet                 { return v.document.Fact.Artifacts() }
func (v VerifiedManifest) Provenance() BuildProvenance            { return v.document.Fact.Provenance() }
func (v VerifiedManifest) Metadata() MetadataSet                  { return v.document.Fact.Metadata() }
func (v VerifiedManifest) TotalExtent() core.ByteCount            { return v.document.Fact.TotalExtent() }

// PublicationIntegrity returns the exact integrity occupying one release
// publication role. Release owns this projection so every publisher and
// authority shares the manifest's compiler-visible slot order.
func (v VerifiedManifest) PublicationIntegrity(role PublicationRole) (ArtifactIntegrity, error) {
	if err := v.Validate(); err != nil {
		return ArtifactIntegrity{}, verificationError(err)
	}
	index, err := role.Index()
	if err != nil {
		return ArtifactIntegrity{}, manifestError(err)
	}
	switch {
	case role >= PublicationRoleWindowsAMD64 && role <= PublicationRoleLinuxARM64:
		artifact, ok := v.Artifacts().At(index)
		if !ok {
			return ArtifactIntegrity{}, manifestError(errors.New("publication artifact role is absent"))
		}
		return artifact.Integrity(), nil
	case role == PublicationRoleManifest:
		return v.DocumentIntegrity(), nil
	case role >= PublicationRoleDependencies && role <= PublicationRoleReleaseNotes:
		metadataIndex := index - int(PublicationRoleDependencies-1)
		asset, ok := v.Metadata().At(metadataIndex)
		if !ok {
			return ArtifactIntegrity{}, manifestError(errors.New("publication metadata role is absent"))
		}
		return asset.Integrity(), nil
	default:
		return ArtifactIntegrity{}, manifestError(errors.New("publication role escaped its domain"))
	}
}

func integrityManifestDocument(document ManifestDocument) (ArtifactIntegrity, error) {
	encoded, err := document.MarshalJSON()
	if err != nil {
		return ArtifactIntegrity{}, err
	}
	extent, err := core.NewByteCount(uint64(len(encoded)))
	if err != nil {
		return ArtifactIntegrity{}, err
	}
	return newArtifactIntegrity(
		extent,
		core.SHA256Of(encoded),
		core.NewCRC32C(crc32.Checksum(encoded, crc32.MakeTable(crc32.Castagnoli))),
	)
}

var _ attest.CanonicalBody[Domain] = ManifestFact{}
