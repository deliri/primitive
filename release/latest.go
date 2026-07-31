package release

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

const latestIdentityDomain = "primitive-release-latest-identity-v1"

type verificationSeal uint8

const (
	verificationSealUnknown verificationSeal = iota
	verificationSealAuthenticated
)

// Generation is a positive monotonic Latest generation.
type Generation struct {
	value uint64
}

func NewGeneration(value uint64) (Generation, error) {
	generation := Generation{value: value}
	if err := generation.Validate(); err != nil {
		return Generation{}, err
	}
	return generation, nil
}

func (g Generation) Validate() error {
	if g.value == 0 {
		return latestError(errors.New("generation must be positive"))
	}
	return nil
}

func (g Generation) Uint64() uint64 { return g.value }

func (g Generation) MarshalJSON() ([]byte, error) {
	if err := g.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(g.value)
}

func (g *Generation) UnmarshalJSON(data []byte) error {
	if g == nil {
		return jsonError(errors.New("generation receiver is nil"))
	}
	var value uint64
	if err := json.Unmarshal(data, &value); err != nil {
		return jsonError(err)
	}
	parsed, err := NewGeneration(value)
	if err != nil {
		return jsonError(err)
	}
	canonical, _ := parsed.MarshalJSON()
	if string(canonical) != string(data) {
		return jsonError(errors.New("generation is not canonical"))
	}
	*g = parsed
	return nil
}

// LatestIdentity names the stable selection stream for one offering.
type LatestIdentity struct {
	digest core.SHA256Digest
}

func newLatestIdentity(digest core.SHA256Digest) LatestIdentity {
	return LatestIdentity{digest: digest}
}

func (i LatestIdentity) Validate() error { return i.digest.Validate() }
func (i LatestIdentity) String() string {
	value, _ := i.digest.Hex()
	return value
}
func (i LatestIdentity) MarshalJSON() ([]byte, error) { return i.digest.MarshalJSON() }
func (i *LatestIdentity) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError(errors.New("latest identity receiver is nil"))
	}
	var digest core.SHA256Digest
	if err := json.Unmarshal(data, &digest); err != nil {
		return jsonError(err)
	}
	*i = newLatestIdentity(digest)
	return nil
}

// LatestFact is the immutable canonical release-selection body.
type LatestFact struct {
	manifest   ManifestDocument
	issuedAt   temporal.Instant
	validFrom  temporal.Instant
	validUntil temporal.Instant
	generation Generation
	identity   LatestIdentity
	revision   Revision
	offering   core.Offering
	valid      bool
}

type latestFactWire struct {
	Identity   *LatestIdentity   `json:"identity"`
	Revision   *Revision         `json:"revision"`
	Generation *Generation       `json:"generation"`
	Offering   *core.Offering    `json:"offering"`
	Manifest   *ManifestDocument `json:"manifest"`
	IssuedAt   *temporal.Instant `json:"issued_at"`
	ValidFrom  *temporal.Instant `json:"valid_from"`
	ValidUntil *temporal.Instant `json:"valid_until"`
}

func (f LatestFact) Validate() error {
	if !f.valid {
		return latestError(errors.New("latest fact is unset"))
	}
	for _, err := range []error{
		f.identity.Validate(), f.revision.Validate(), f.generation.Validate(),
		f.offering.Validate(), f.manifest.Validate(), f.issuedAt.Validate(),
		f.validFrom.Validate(), f.validUntil.Validate(),
	} {
		if err != nil {
			return latestError(err)
		}
	}
	if err := f.validateBindings(); err != nil {
		return err
	}
	return f.validateTimeline()
}

func (f LatestFact) validateBindings() error {
	expected, err := latestIdentity(f.revision, f.offering)
	if err != nil || f.identity != expected ||
		f.manifest.Fact.Offering() != f.offering {
		return latestError(errors.New("latest identity or manifest binding differs"), err)
	}
	return nil
}

func (f LatestFact) validateTimeline() error {
	issueFrom, err := f.issuedAt.Compare(f.validFrom)
	if err != nil || issueFrom == core.ComparisonGreater {
		return latestError(errors.New("latest issue instant follows valid-from"), err)
	}
	fromUntil, err := f.validFrom.Compare(f.validUntil)
	if err != nil || fromUntil != core.ComparisonLess {
		return latestError(errors.New("latest validity interval is empty or reversed"), err)
	}
	lifetime, err := f.validUntil.Since(f.validFrom)
	if err != nil || lifetime.Nanoseconds() > ReleaseLatestMaximumLifetimeNanoseconds {
		return latestError(errors.New("latest validity exceeds its maximum"), err)
	}
	return nil
}

func (f LatestFact) Identity() LatestIdentity     { return f.identity }
func (f LatestFact) Revision() Revision           { return f.revision }
func (f LatestFact) Generation() Generation       { return f.generation }
func (f LatestFact) Offering() core.Offering      { return f.offering }
func (f LatestFact) Manifest() ManifestDocument   { return f.manifest }
func (f LatestFact) IssuedAt() temporal.Instant   { return f.issuedAt }
func (f LatestFact) ValidFrom() temporal.Instant  { return f.validFrom }
func (f LatestFact) ValidUntil() temporal.Instant { return f.validUntil }
func (LatestFact) AttestationDomain() Domain      { return DomainLatestV1 }

func (f LatestFact) MarshalJSON() ([]byte, error) {
	if err := f.Validate(); err != nil {
		return nil, err
	}
	identity, revision, generation := f.identity, f.revision, f.generation
	offering, manifest := f.offering, f.manifest
	issuedAt, validFrom, validUntil := f.issuedAt, f.validFrom, f.validUntil
	return json.Marshal(latestFactWire{
		Identity: &identity, Revision: &revision, Generation: &generation,
		Offering: &offering, Manifest: &manifest, IssuedAt: &issuedAt,
		ValidFrom: &validFrom, ValidUntil: &validUntil,
	})
}

func (f *LatestFact) UnmarshalJSON(data []byte) error {
	if f == nil {
		return jsonError(errors.New("latest fact receiver is nil"))
	}
	wire, err := decodeStructure[latestFactWire](data)
	if err != nil {
		return err
	}
	if latestWireMissing(wire) {
		return jsonError(errors.New("latest fact field is missing"))
	}
	candidate := LatestFact{
		identity: *wire.Identity, revision: *wire.Revision,
		generation: *wire.Generation, offering: *wire.Offering,
		manifest: *wire.Manifest, issuedAt: *wire.IssuedAt,
		validFrom: *wire.ValidFrom, validUntil: *wire.ValidUntil, valid: true,
	}
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*f = candidate
	return nil
}

func latestWireMissing(w latestFactWire) bool {
	return w.Identity == nil || w.Revision == nil || w.Generation == nil ||
		w.Offering == nil || w.Manifest == nil || w.IssuedAt == nil ||
		w.ValidFrom == nil || w.ValidUntil == nil
}

func (f LatestFact) WriteCanonical(destination io.Writer) error {
	if destination == nil {
		return latestError(errors.New(canonicalDestinationNilDiagnostic))
	}
	encoded, err := f.MarshalJSON()
	if err != nil {
		return err
	}
	written, err := destination.Write(encoded)
	if err != nil {
		return latestError(err)
	}
	if written != len(encoded) {
		return latestError(io.ErrShortWrite)
	}
	return nil
}

func latestIdentity(revision Revision, offering core.Offering) (LatestIdentity, error) {
	if err := revision.Validate(); err != nil {
		return LatestIdentity{}, latestError(err)
	}
	if err := offering.Validate(); err != nil {
		return LatestIdentity{}, latestError(err)
	}
	body, err := json.Marshal(struct {
		Revision Revision      `json:"revision"`
		Offering core.Offering `json:"offering"`
	}{Revision: revision, Offering: offering})
	if err != nil {
		return LatestIdentity{}, latestError(err)
	}
	sum := sha256.New()
	writeDigestFrame(sum, latestIdentityDomain, body)
	var value [sha256.Size]byte
	copy(value[:], sum.Sum(nil))
	return newLatestIdentity(core.NewSHA256Digest(value)), nil
}

// LatestDocument is one untrusted Latest fact and structural Attest envelope.
type LatestDocument struct {
	Fact        LatestFact              `json:"fact"`
	Attestation attest.Envelope[Domain] `json:"attestation"`
}

func (d LatestDocument) Validate() error {
	if err := d.Fact.Validate(); err != nil {
		return latestError(err)
	}
	if err := d.Attestation.Validate(); err != nil {
		return verificationError(err)
	}
	if d.Attestation.Domain != DomainLatestV1 {
		return verificationError(errors.New("latest attestation domain differs from its body"))
	}
	return nil
}

func (d LatestDocument) MarshalJSON() ([]byte, error) {
	type wire LatestDocument
	if err := d.Validate(); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(wire(d))
	if err != nil || len(encoded) > documentExtentMaximum {
		return nil, jsonError(errors.New("latest document extent exceeded"), err)
	}
	return encoded, nil
}

func (d *LatestDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("latest document receiver is nil"))
	}
	type wire struct {
		Fact        *LatestFact              `json:"fact"`
		Attestation *attest.Envelope[Domain] `json:"attestation"`
	}
	decoded, err := decodeStructure[wire](data)
	if err != nil {
		return err
	}
	if decoded.Fact == nil || decoded.Attestation == nil {
		return jsonError(errors.New("latest document field is missing"))
	}
	candidate := LatestDocument{Fact: *decoded.Fact, Attestation: *decoded.Attestation}
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

type IssueLatestRequest struct {
	Key        ed25519.PrivateKey
	Manifest   VerifiedManifest
	IssuedAt   temporal.Instant
	ValidFrom  temporal.Instant
	ValidUntil temporal.Instant
	Generation Generation
}

func IssueLatest(request IssueLatestRequest) (LatestDocument, error) {
	if err := request.Manifest.Validate(); err != nil {
		return LatestDocument{}, latestError(err)
	}
	manifest, err := request.Manifest.Document()
	if err != nil {
		return LatestDocument{}, latestError(err)
	}
	offering, err := request.Manifest.Offering()
	if err != nil {
		return LatestDocument{}, latestError(err)
	}
	identity, err := latestIdentity(Revision2026V1, offering)
	if err != nil {
		return LatestDocument{}, err
	}
	fact := LatestFact{
		identity: identity, revision: Revision2026V1,
		generation: request.Generation, offering: offering, manifest: manifest,
		issuedAt: request.IssuedAt, validFrom: request.ValidFrom,
		validUntil: request.ValidUntil, valid: true,
	}
	if err := fact.Validate(); err != nil {
		return LatestDocument{}, err
	}
	envelope, err := attest.Sign(attest.SignRequest[Domain]{Body: fact, Key: request.Key})
	if err != nil {
		return LatestDocument{}, latestError(err)
	}
	document := LatestDocument{Fact: fact, Attestation: envelope}
	if err := document.Validate(); err != nil {
		return LatestDocument{}, err
	}
	return document, nil
}

type VerifyLatestRequest struct {
	Document         LatestDocument
	LatestKeys       attest.TrustedKeys
	ManifestKeys     attest.TrustedKeys
	ExpectedOffering core.Offering
}

// VerifiedLatest proves both the outer selection and nested manifest.
type VerifiedLatest struct {
	document LatestDocument
	manifest VerifiedManifest
	proof    attest.Verified[Domain]
	seal     verificationSeal
}

func VerifyLatest(request VerifyLatestRequest) (VerifiedLatest, error) {
	if err := request.ExpectedOffering.Validate(); err != nil {
		return VerifiedLatest{}, verificationError(err)
	}
	if err := request.Document.Validate(); err != nil {
		return VerifiedLatest{}, verificationError(err)
	}
	if request.Document.Fact.Offering() != request.ExpectedOffering {
		return VerifiedLatest{}, offeringMismatchError(
			request.Document.Fact.Offering(),
			request.ExpectedOffering,
		)
	}
	manifest, err := VerifyManifest(VerifyManifestRequest{
		Document: request.Document.Fact.Manifest(), TrustedKeys: request.ManifestKeys,
		ExpectedOffering: request.ExpectedOffering,
	})
	if err != nil {
		return VerifiedLatest{}, verificationError(err)
	}
	proof, err := attest.Verify(attest.VerifyRequest[Domain]{
		Body: request.Document.Fact, Envelope: request.Document.Attestation,
		TrustedKeys: request.LatestKeys,
	})
	if err != nil {
		return VerifiedLatest{}, verificationError(err)
	}
	result := VerifiedLatest{
		document: request.Document, manifest: manifest, proof: proof,
	}
	result.seal = verificationSealAuthenticated
	return result, nil
}

// Validate proves that VerifyLatest issued the private witness and that its
// complete closure was authenticated. Cryptographic and canonical body
// verification run exactly once before the witness is sealed.
func (v VerifiedLatest) Validate() error {
	if v.seal != verificationSealAuthenticated {
		return verificationError(errors.New("verified latest proof is unset"))
	}
	return nil
}

func (v VerifiedLatest) Fact() (LatestFact, error) {
	if err := v.Validate(); err != nil {
		return LatestFact{}, err
	}
	return v.document.Fact, nil
}

func (v VerifiedLatest) Manifest() (VerifiedManifest, error) {
	if err := v.Validate(); err != nil {
		return VerifiedManifest{}, err
	}
	return v.manifest, nil
}

func (v VerifiedLatest) Document() (LatestDocument, error) {
	if err := v.Validate(); err != nil {
		return LatestDocument{}, err
	}
	return v.document, nil
}

var _ attest.CanonicalBody[Domain] = LatestFact{}
