package chit

import (
	json "encoding/json/v2"
	"errors"
	"math"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/receipt"
)

const manifestFrameDomain = "primitive/chit/manifest-entry/v1"

// ObjectCount is a positive number of objects in one manifest.
type ObjectCount struct{ value uint64 }

func NewObjectCount(value uint64) (ObjectCount, error) {
	candidate := ObjectCount{value: value}
	if err := candidate.Validate(); err != nil {
		return ObjectCount{}, err
	}
	return candidate, nil
}

func (c ObjectCount) Validate() error {
	if c.value == 0 {
		return contractError(errors.New("manifest object count is zero"))
	}
	return nil
}

func (c ObjectCount) Uint64() uint64 { return c.value }

func (c ObjectCount) MarshalJSON() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return json.Marshal(c.value)
}

func (c *ObjectCount) UnmarshalJSON(data []byte) error {
	if c == nil {
		return jsonError(errors.New("nil object count receiver"))
	}
	var value uint64
	if err := json.Unmarshal(data, &value); err != nil {
		return jsonError(err)
	}
	candidate, err := NewObjectCount(value)
	if err != nil {
		return jsonError(err)
	}
	canonical, _ := candidate.MarshalJSON()
	if string(canonical) != string(data) {
		return jsonError(errors.New("object count is not canonical"))
	}
	*c = candidate
	return nil
}

// EntrySequence is a positive contiguous manifest position.
type EntrySequence struct{ value uint64 }

func NewEntrySequence(value uint64) (EntrySequence, error) {
	candidate := EntrySequence{value: value}
	if err := candidate.Validate(); err != nil {
		return EntrySequence{}, err
	}
	return candidate, nil
}

func (s EntrySequence) Validate() error {
	if s.value == 0 {
		return contractError(errors.New("manifest entry sequence is zero"))
	}
	return nil
}

func (s EntrySequence) Uint64() uint64 { return s.value }

func (s EntrySequence) MarshalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return json.Marshal(s.value)
}

func (s *EntrySequence) UnmarshalJSON(data []byte) error {
	if s == nil {
		return jsonError(errors.New("nil entry sequence receiver"))
	}
	var value uint64
	if err := json.Unmarshal(data, &value); err != nil {
		return jsonError(err)
	}
	candidate, err := NewEntrySequence(value)
	if err != nil {
		return jsonError(err)
	}
	canonical, _ := candidate.MarshalJSON()
	if string(canonical) != string(data) {
		return jsonError(errors.New("entry sequence is not canonical"))
	}
	*s = candidate
	return nil
}

// ManifestEntry is one consumer-visible object in a logical uploaded version.
// Name is a product-owned safe display path, never an inferred local path.
type ManifestEntry struct {
	Name        EntryName                `json:"name"`
	ContentType core.HTTPMediaType       `json:"content_type"`
	Evidence    receipt.EvidenceDocument `json:"evidence"`
	Sequence    EntrySequence            `json:"sequence"`
}

// ManifestAddition is the issuance-only proof that one wire entry carries the
// exact Receipt document already authenticated by Receipt's sealed verifier.
type ManifestAddition struct {
	Entry    ManifestEntry
	Evidence receipt.VerifiedEvidence
}

// Validate refuses structurally plausible but unauthenticated receipt
// documents at the manifest-construction boundary.
func (a ManifestAddition) Validate() error {
	if err := errors.Join(a.Entry.Validate(), a.Evidence.Validate()); err != nil {
		return contractError(err)
	}
	document, err := a.Evidence.Document()
	if err != nil {
		return contractError(err)
	}
	if document != a.Entry.Evidence {
		return conflictError(errors.New("manifest entry differs from verified receipt evidence"))
	}
	return nil
}

func (e ManifestEntry) Validate() error {
	if err := e.Sequence.Validate(); err != nil {
		return err
	}
	if err := e.Name.Validate(); err != nil {
		return contractError(errors.New("manifest entry name is invalid"), err)
	}
	if err := e.ContentType.Validate(); err != nil {
		return contractError(errors.New("manifest entry media type is invalid"), err)
	}
	if err := e.Evidence.Validate(); err != nil {
		return contractError(errors.New("manifest entry evidence is invalid"), err)
	}
	return nil
}

// ManifestDigest is the domain-separated digest of the canonical entry stream.
type ManifestDigest struct {
	value core.SHA256Digest
}

func newManifestDigest(value core.SHA256Digest) (ManifestDigest, error) {
	candidate := ManifestDigest{value: value}
	if err := candidate.Validate(); err != nil {
		return ManifestDigest{}, err
	}
	return candidate, nil
}

func (d ManifestDigest) Validate() error {
	if err := d.value.Validate(); err != nil {
		return contractError(errors.New("manifest digest is invalid"), err)
	}
	return nil
}

func (d ManifestDigest) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return d.value.MarshalJSON()
}

func (d *ManifestDigest) UnmarshalJSON(data []byte) error {
	if d == nil {
		return jsonError(errors.New("nil manifest digest receiver"))
	}
	var value core.SHA256Digest
	if err := value.UnmarshalJSON(data); err != nil {
		return jsonError(err)
	}
	candidate, err := newManifestDigest(value)
	if err != nil {
		return jsonError(err)
	}
	*d = candidate
	return nil
}

// ManifestSummary closes the exact ordered object set without materializing it.
type ManifestSummary struct {
	Digest     ManifestDigest  `json:"digest"`
	TotalBytes core.ByteLength `json:"total_bytes"`
	Objects    ObjectCount     `json:"objects"`
}

func (s ManifestSummary) Validate() error {
	if err := s.Objects.Validate(); err != nil {
		return err
	}
	if err := s.TotalBytes.Validate(); err != nil {
		return contractError(errors.New("manifest total extent is invalid"), err)
	}
	if s.TotalBytes.Uint64() == 0 {
		return contractError(errors.New("manifest total extent is zero"))
	}
	return s.Digest.Validate()
}

// ManifestAccumulator closes entries in one pass and O(1) retained memory.
type ManifestAccumulator struct {
	digest     *core.DigestWriter
	totalBytes uint64
	objects    uint64
	sealed     bool
}

// ManifestEntryVerifier recomputes one persisted manifest stream while
// retaining only the exact requested entry. Its memory is constant in the
// number and extent of manifest objects.
type ManifestEntryVerifier struct {
	accumulator *ManifestAccumulator
	selected    ManifestAddition
	sequence    EntrySequence
	found       bool
}

// VerifiedManifestEntry is sealed proof that one authenticated receipt entry
// participated at its exact sequence in one exact manifest summary.
type VerifiedManifestEntry struct {
	addition ManifestAddition
	summary  ManifestSummary
}

const (
	manifestAccumulatorUnsetDiagnostic  = "manifest accumulator is unset"
	manifestAccumulatorSealedDiagnostic = "manifest accumulator is sealed"
)

func NewManifestAccumulator() *ManifestAccumulator {
	digest := core.NewDigestWriter()
	_, _ = digest.Write([]byte(manifestFrameDomain))
	_, _ = digest.Write([]byte{0})
	return &ManifestAccumulator{digest: digest}
}

// NewManifestEntryVerifier begins one O(1) lookup over a manifest stream.
func NewManifestEntryVerifier(sequence EntrySequence) (*ManifestEntryVerifier, error) {
	if err := sequence.Validate(); err != nil {
		return nil, contractError(err)
	}
	return &ManifestEntryVerifier{accumulator: NewManifestAccumulator(), sequence: sequence}, nil
}

// Add admits the next authenticated manifest entry and retains it only when it
// is the requested sequence.
func (v *ManifestEntryVerifier) Add(addition ManifestAddition) error {
	if v == nil || v.accumulator == nil {
		return contractError(errors.New(manifestAccumulatorUnsetDiagnostic))
	}
	if err := v.accumulator.Add(addition); err != nil {
		return err
	}
	if addition.Entry.Sequence == v.sequence {
		v.selected = addition
		v.found = true
	}
	return nil
}

// Seal proves the recomputed stream equals the authenticated summary before
// releasing the selected entry. The underlying accumulator owns terminal use.
func (v *ManifestEntryVerifier) Seal(expected ManifestSummary) (VerifiedManifestEntry, error) {
	if v == nil || v.accumulator == nil {
		return VerifiedManifestEntry{}, contractError(errors.New(manifestAccumulatorUnsetDiagnostic))
	}
	if err := expected.Validate(); err != nil {
		return VerifiedManifestEntry{}, contractError(err)
	}
	got, err := v.accumulator.Seal()
	if err != nil {
		return VerifiedManifestEntry{}, err
	}
	if got != expected || !v.found {
		return VerifiedManifestEntry{}, conflictError(errors.New("manifest stream does not prove the requested entry"))
	}
	proof := VerifiedManifestEntry{addition: v.selected, summary: got}
	return proof, proof.Validate()
}

// Validate closes the sealed entry and the exact stream summary that proved it.
func (v VerifiedManifestEntry) Validate() error {
	if err := errors.Join(v.addition.Validate(), v.summary.Validate()); err != nil {
		return contractError(err)
	}
	if v.addition.Entry.Sequence.Uint64() > v.summary.Objects.Uint64() {
		return conflictError(errors.New("verified manifest entry exceeds its summary"))
	}
	return nil
}

// Addition returns the exact authenticated entry admitted by the stream.
func (v VerifiedManifestEntry) Addition() (ManifestAddition, error) {
	if err := v.Validate(); err != nil {
		return ManifestAddition{}, err
	}
	return v.addition, nil
}

// Summary returns the exact recomputed manifest closure.
func (v VerifiedManifestEntry) Summary() (ManifestSummary, error) {
	if err := v.Validate(); err != nil {
		return ManifestSummary{}, err
	}
	return v.summary, nil
}

// Add admits exactly the next authenticated receipt and sequence, then folds
// the entry's canonical wire bytes.
func (a *ManifestAccumulator) Add(addition ManifestAddition) error {
	if a == nil || a.digest == nil {
		return contractError(errors.New(manifestAccumulatorUnsetDiagnostic))
	}
	if a.sealed {
		return contractError(errors.New(manifestAccumulatorSealedDiagnostic))
	}
	if err := addition.Validate(); err != nil {
		return err
	}
	entry := addition.Entry
	if entry.Sequence.Uint64() != a.objects+1 {
		return conflictError(errors.New("manifest sequence is not contiguous"))
	}
	body := entry.Evidence.Payload.Body
	extent := body.Extent.Uint64()
	if a.totalBytes > math.MaxInt64-extent {
		return errors.Join(core.ErrNumericOverflow, contractError(errors.New("manifest total extent overflow")))
	}
	encoded, err := core.MarshalCanonicalJSONDocument(entry)
	if err != nil {
		return jsonError(err)
	}
	if _, err := a.digest.Write(encoded); err != nil {
		return contractError(err)
	}
	if _, err := a.digest.Write([]byte{0}); err != nil {
		return contractError(err)
	}
	a.totalBytes += extent
	a.objects++
	return nil
}

// Seal returns the immutable stream closure. Sealing twice is refused because
// the accumulator represents one construction boundary, not a reusable cache.
func (a *ManifestAccumulator) Seal() (ManifestSummary, error) {
	if a == nil || a.digest == nil {
		return ManifestSummary{}, contractError(errors.New(manifestAccumulatorUnsetDiagnostic))
	}
	if a.sealed {
		return ManifestSummary{}, contractError(errors.New(manifestAccumulatorSealedDiagnostic))
	}
	a.sealed = true
	if a.objects == 0 {
		return ManifestSummary{}, contractError(errors.New("manifest is empty"))
	}
	digest, _, err := a.digest.Seal()
	if err != nil {
		return ManifestSummary{}, contractError(err)
	}
	total, err := core.NewByteLength(a.totalBytes)
	if err != nil {
		return ManifestSummary{}, contractError(err)
	}
	manifestDigest, err := newManifestDigest(digest)
	if err != nil {
		return ManifestSummary{}, err
	}
	objects, err := NewObjectCount(a.objects)
	if err != nil {
		return ManifestSummary{}, err
	}
	summary := ManifestSummary{Digest: manifestDigest, TotalBytes: total, Objects: objects}
	return summary, summary.Validate()
}

var (
	_ core.Validatable            = ManifestEntry{}
	_ core.Validatable            = ManifestAddition{}
	_ core.Validatable            = ObjectCount{}
	_ core.Validatable            = EntrySequence{}
	_ core.Validatable            = ManifestDigest{}
	_ core.Validatable            = ManifestSummary{}
	_ core.Validatable            = VerifiedManifestEntry{}
	_ core.ValidatedJSONMarshaler = ManifestDigest{}
	_ core.ValidatedJSONMarshaler = ObjectCount{}
	_ core.ValidatedJSONMarshaler = EntrySequence{}
)
