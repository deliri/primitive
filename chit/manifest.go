package chit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"math"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/keygen"
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
	if err := validateScalarJSONExtent(data); err != nil {
		return err
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
	if err := validateScalarJSONExtent(data); err != nil {
		return err
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
	if err := validateScalarJSONExtent(data); err != nil {
		return err
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
// Objects must be positive; TotalBytes may be zero for authenticated empty objects.
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

const manifestAdmissionDomain = "primitive/chit/manifest-admission/v1"

// ManifestAdmission is an opaque, persistable ticket for one addition admitted
// by a ManifestAdmissionAccumulator. It is not membership proof until the same
// accumulator seals the complete stream against an authenticated summary.
type ManifestAdmission struct {
	entry ManifestEntry
	tag   [sha256.Size]byte
}

type manifestAdmissionWire struct {
	Entry  ManifestEntry `json:"entry"`
	Ticket string        `json:"ticket"`
}

// ManifestAdmissionAccumulator folds one manifest while issuing constant-size
// tickets that callers may spool instead of retaining one verifier per entry.
type ManifestAdmissionAccumulator struct {
	manifest *ManifestAccumulator
	key      core.SecretMaterial
	sealed   bool
}

// VerifiedManifestAdmission is the sealed capability that admits tickets
// issued during one exact manifest fold.
type VerifiedManifestAdmission struct {
	key      core.SecretMaterial
	summary  ManifestSummary
	verified bool
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

// NewManifestAdmissionAccumulator begins one constant-memory manifest fold.
func NewManifestAdmissionAccumulator() (*ManifestAdmissionAccumulator, error) {
	size, err := core.NewByteCount(sha256.Size)
	if err != nil {
		return nil, contractError(err)
	}
	key, err := keygen.GenerateSecret(keygen.SecretRequest{Size: size})
	if err != nil {
		return nil, contractError(err)
	}
	return &ManifestAdmissionAccumulator{manifest: NewManifestAccumulator(), key: key}, nil
}

// Add admits one authenticated addition and returns its persistable ticket.
func (a *ManifestAdmissionAccumulator) Add(addition ManifestAddition) (ManifestAdmission, error) {
	if a == nil || a.manifest == nil || a.sealed {
		return ManifestAdmission{}, contractError(errors.New(manifestAccumulatorUnsetDiagnostic))
	}
	if err := a.manifest.Add(addition); err != nil {
		return ManifestAdmission{}, err
	}
	tag, err := manifestAdmissionTag(a.key, addition.Entry)
	if err != nil {
		return ManifestAdmission{}, err
	}
	admission := ManifestAdmission{entry: addition.Entry, tag: tag}
	return admission, admission.Validate()
}

// Seal authenticates the complete fold before releasing the ticket verifier.
func (a *ManifestAdmissionAccumulator) Seal(expected ManifestSummary) (VerifiedManifestAdmission, error) {
	if a == nil || a.manifest == nil || a.sealed {
		return VerifiedManifestAdmission{}, contractError(errors.New(manifestAccumulatorUnsetDiagnostic))
	}
	if err := expected.Validate(); err != nil {
		return VerifiedManifestAdmission{}, contractError(err)
	}
	a.sealed = true
	got, err := a.manifest.Seal()
	if err != nil {
		return VerifiedManifestAdmission{}, errors.Join(err, a.Destroy())
	}
	if got != expected {
		return VerifiedManifestAdmission{}, errors.Join(
			conflictError(errors.New("manifest admission stream differs from expected summary")),
			a.Destroy(),
		)
	}
	verified := VerifiedManifestAdmission{key: a.key, summary: got, verified: true}
	a.key = core.SecretMaterial{}
	return verified, verified.Validate()
}

// Destroy abandons an unsealed accumulator's ticket authority. After a
// successful Seal, ownership has moved to VerifiedManifestAdmission.
func (a *ManifestAdmissionAccumulator) Destroy() error {
	if a == nil || a.key.Validate() != nil {
		return nil
	}
	return a.key.Destroy()
}

func manifestAdmissionTag(key core.SecretMaterial, entry ManifestEntry) ([sha256.Size]byte, error) {
	var zero [sha256.Size]byte
	rawKey, err := key.CopyBytes()
	if err != nil {
		return zero, contractError(err)
	}
	defer clear(rawKey)
	encoded, err := core.MarshalCanonicalJSONDocument(entry)
	if err != nil {
		return zero, jsonError(err)
	}
	digest := hmac.New(sha256.New, rawKey)
	_, _ = digest.Write([]byte(manifestAdmissionDomain))
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write(encoded)
	copy(zero[:], digest.Sum(nil))
	return zero, nil
}

func (a ManifestAdmission) Validate() error {
	return a.entry.Validate()
}

func (a ManifestAdmission) Entry() (ManifestEntry, error) {
	if err := a.Validate(); err != nil {
		return ManifestEntry{}, err
	}
	return a.entry, nil
}

func (a ManifestAdmission) MarshalJSON() ([]byte, error) {
	if err := a.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return core.MarshalCanonicalJSONDocument(manifestAdmissionWire{
		Entry: a.entry, Ticket: hex.EncodeToString(a.tag[:]),
	})
}

func (a *ManifestAdmission) UnmarshalJSON(data []byte) error {
	if a == nil {
		return jsonError(errors.New("nil manifest admission receiver"))
	}
	wire, err := decodeStrict[manifestAdmissionWire](data, core.JSONDocumentMaximumBytes)
	if err != nil {
		return err
	}
	if len(wire.Ticket) != hex.EncodedLen(sha256.Size) {
		return jsonError(errors.New("manifest admission ticket extent differs"))
	}
	var tag [sha256.Size]byte
	decoded, err := hex.Decode(tag[:], []byte(wire.Ticket))
	if err != nil || decoded != len(tag) || hex.EncodeToString(tag[:]) != wire.Ticket {
		return jsonError(errors.New("manifest admission ticket is not canonical"), err)
	}
	candidate := ManifestAdmission{entry: wire.Entry, tag: tag}
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*a = candidate
	return nil
}

func (v VerifiedManifestAdmission) Validate() error {
	if !v.verified {
		return contractError(errors.New("manifest admission proof is unset"))
	}
	return errors.Join(v.key.Validate(), v.summary.Validate())
}

// Destroy invalidates this manifest ticket verifier and every copied handle.
func (v VerifiedManifestAdmission) Destroy() error {
	if !v.verified || v.key.Validate() != nil {
		return nil
	}
	return v.key.Destroy()
}

// Verify authenticates one replayed ticket and evidence document as a member
// of the complete manifest already sealed by this capability.
func (v VerifiedManifestAdmission) Verify(admission ManifestAdmission, evidence receipt.VerifiedEvidence) (VerifiedManifestEntry, error) {
	if err := errors.Join(v.Validate(), admission.Validate(), evidence.Validate()); err != nil {
		return VerifiedManifestEntry{}, contractError(err)
	}
	document, err := evidence.Document()
	if err != nil || document != admission.entry.Evidence {
		return VerifiedManifestEntry{}, conflictError(errors.New("manifest admission evidence differs"), err)
	}
	want, err := manifestAdmissionTag(v.key, admission.entry)
	if err != nil {
		return VerifiedManifestEntry{}, err
	}
	if !hmac.Equal(want[:], admission.tag[:]) {
		return VerifiedManifestEntry{}, conflictError(errors.New("manifest admission ticket differs"))
	}
	proof := VerifiedManifestEntry{
		addition: ManifestAddition{Entry: admission.entry, Evidence: evidence},
		summary:  v.summary,
	}
	return proof, proof.Validate()
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
	_ core.Validatable            = ManifestAdmission{}
	_ core.Validatable            = VerifiedManifestAdmission{}
	_ core.ValidatedJSONMarshaler = ManifestDigest{}
	_ core.ValidatedJSONMarshaler = ManifestAdmission{}
	_ core.ValidatedJSONMarshaler = ObjectCount{}
	_ core.ValidatedJSONMarshaler = EntrySequence{}
)
