package objectstore

import (
	json "encoding/json/v2"
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	// TransferEvidenceJSONMaximumBytes is the derived wire ceiling for every
	// admitted provider version plus the widest fixed evidence document.
	TransferEvidenceJSONMaximumBytes = canonicalJSONStringMaximumExpansion*AmazonS3VersionIDMaximumBytes +
		transferEvidenceDocumentSyntaxMaximumBytes
	transferEvidenceDocumentSyntaxMaximumBytes = len(
		`{"provider":"google_cloud_storage","direction":"download",` +
			`"version":"","bytes":9223372036854775807,` +
			`"sha256":"0000000000000000000000000000000000000000000000000000000000000000",` +
			`"crc32c":"AAAAAAAA"}`,
	)
	transferEvidenceRequiredMemberErrorText = "transfer evidence required member is absent"
)

type transferEvidenceWire struct {
	Provider  *string            `json:"provider"`
	Direction *string            `json:"direction"`
	Version   *string            `json:"version,omitempty"`
	Bytes     *core.ByteLength   `json:"bytes"`
	SHA256    *core.SHA256Digest `json:"sha256"`
	CRC32C    *core.CRC32C       `json:"crc32c"`
}

// TransferEvidence is the receive-only exact fact that one provider transfer
// completed and passed Objectstore integrity verification.
type TransferEvidence struct {
	version   ProviderVersion
	bytes     core.ByteLength
	sha256    core.SHA256Digest
	crc32c    core.CRC32C
	provider  Provider
	direction Direction
	set       bool
}

// TransferEvidenceProjection is the issue-only form created solely from a
// confirmed Transfer. It contains no URL, bearer, header, path, or content.
type TransferEvidenceProjection struct{ evidence TransferEvidence }

// ProviderUploadObservationRequest supplies provider-read facts for one exact
// upload. Provider-specific packages obtain these facts from their official
// SDK; Objectstore owns their comparison with client transfer evidence.
type ProviderUploadObservationRequest struct {
	ContentType core.HTTPMediaType
	Version     ProviderVersion
	Evidence    TransferEvidence
	OccurredAt  temporal.Instant
	Bytes       core.ByteLength
	CRC32C      core.CRC32C
}

// VerifiedProviderUpload is sealed provider-neutral proof that independently
// observed object facts agree with one exact client upload.
type VerifiedProviderUpload struct {
	request ProviderUploadObservationRequest
	set     bool
}

// Validate closes the independently observed upload facts and their binding
// to the authenticated client evidence.
func (r ProviderUploadObservationRequest) Validate() error {
	if err := errors.Join(
		r.Evidence.Validate(), r.Version.Validate(), r.Bytes.Validate(),
		r.CRC32C.Validate(), r.ContentType.Validate(), r.OccurredAt.Validate(),
	); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	evidenceVersion, present := r.Evidence.Version()
	if r.Evidence.Direction() != DirectionUpload || !present ||
		r.Version != evidenceVersion || r.Bytes != r.Evidence.Bytes() ||
		r.CRC32C != r.Evidence.CRC32C() {
		return errors.Join(core.ErrObjectStoreIntegrity, core.ErrObjectStoreSource)
	}
	return nil
}

// VerifyProviderUpload releases provider-neutral proof only for exact facts.
func VerifyProviderUpload(request ProviderUploadObservationRequest) (VerifiedProviderUpload, error) {
	if err := request.Validate(); err != nil {
		return VerifiedProviderUpload{}, err
	}
	verified := VerifiedProviderUpload{request: request, set: true}
	if err := verified.Validate(); err != nil {
		return VerifiedProviderUpload{}, err
	}
	return verified, nil
}

// Validate rechecks the sealed provider-neutral proof.
func (v VerifiedProviderUpload) Validate() error {
	if !v.set {
		return core.ErrObjectStoreContract
	}
	return v.request.Validate()
}

// Evidence returns the exact client transfer evidence observed by the
// provider.
func (v VerifiedProviderUpload) Evidence() (TransferEvidence, error) {
	if err := v.Validate(); err != nil {
		return TransferEvidence{}, err
	}
	return v.request.Evidence, nil
}

// ContentType returns the independently observed media type.
func (v VerifiedProviderUpload) ContentType() (core.HTTPMediaType, error) {
	if err := v.Validate(); err != nil {
		return core.HTTPMediaType{}, err
	}
	return v.request.ContentType, nil
}

// OccurredAt returns the provider's immutable creation instant.
func (v VerifiedProviderUpload) OccurredAt() (temporal.Instant, error) {
	if err := v.Validate(); err != nil {
		return temporal.Instant{}, err
	}
	return v.request.OccurredAt, nil
}

// Evidence projects confirmed transfer proof for a higher signed protocol.
func (t Transfer) Evidence() (TransferEvidenceProjection, error) {
	if err := t.Validate(); err != nil {
		return TransferEvidenceProjection{}, err
	}
	evidence := TransferEvidence{
		version: t.version, bytes: t.bytes, sha256: t.sha256, crc32c: t.crc32c,
		provider: t.provider, direction: t.direction, set: true,
	}
	projection := TransferEvidenceProjection{evidence: evidence}
	return projection, projection.Validate()
}

func (e TransferEvidence) Validate() error {
	if !e.set {
		return core.ErrObjectStoreContract
	}
	if err := errors.Join(
		e.provider.Validate(), e.direction.Validate(), e.bytes.Validate(),
		e.sha256.Validate(), e.crc32c.Validate(),
	); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	if err := validateProviderDirection(e.provider, e.direction); err != nil {
		return errors.Join(core.ErrObjectStoreContract, err)
	}
	if e.provider == ProviderGoogleCloudStorage && e.direction == DirectionUpload && e.version.IsZero() {
		return core.ErrObjectStoreContract
	}
	if !e.version.IsZero() {
		if err := e.version.Validate(); err != nil || e.version.Provider() != e.provider {
			return errors.Join(core.ErrObjectStoreContract, err)
		}
	}
	return nil
}

func (p TransferEvidenceProjection) Validate() error { return p.evidence.Validate() }

func (e TransferEvidence) Provider() Provider        { return e.provider }
func (e TransferEvidence) Direction() Direction      { return e.direction }
func (e TransferEvidence) Bytes() core.ByteLength    { return e.bytes }
func (e TransferEvidence) SHA256() core.SHA256Digest { return e.sha256 }
func (e TransferEvidence) CRC32C() core.CRC32C       { return e.crc32c }
func (e TransferEvidence) Version() (ProviderVersion, bool) {
	return e.version, !e.version.IsZero()
}

func (p TransferEvidenceProjection) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return marshalTransferEvidence(p.evidence)
}

// MarshalJSON re-emits authenticated receive-side evidence canonically so a
// containing signed protocol can verify the exact body issued by Projection.
func (e TransferEvidence) MarshalJSON() ([]byte, error) {
	if err := e.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return marshalTransferEvidence(e)
}

func marshalTransferEvidence(evidence TransferEvidence) ([]byte, error) {
	encoded, err := core.MarshalCanonicalJSONDocument(transferEvidenceWireFrom(evidence))
	if err != nil || len(encoded) > TransferEvidenceJSONMaximumBytes {
		return nil, errors.Join(core.ErrJSONContract, core.ErrObjectStoreContract, err)
	}
	return encoded, nil
}

func (e *TransferEvidence) UnmarshalJSON(data []byte) error {
	if e == nil {
		return errors.Join(core.ErrJSONContract, core.ErrObjectStoreContract)
	}
	limit, err := core.NewByteCount(uint64(TransferEvidenceJSONMaximumBytes))
	if err != nil {
		return errors.Join(core.ErrJSONContract, core.ErrObjectStoreContract, err)
	}
	limits := core.DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes = limit
	wire, err := core.DecodeStrictJSONStructure[transferEvidenceWire](data, limits)
	if err != nil {
		return errors.Join(core.ErrJSONContract, core.ErrObjectStoreContract, err)
	}
	candidate, err := transferEvidenceFromWire(wire)
	if err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*e = candidate
	return nil
}

func transferEvidenceWireFrom(evidence TransferEvidence) transferEvidenceWire {
	provider := evidence.provider.String()
	direction := evidence.direction.String()
	bytes := evidence.bytes
	sha256 := evidence.sha256
	crc32c := evidence.crc32c
	wire := transferEvidenceWire{
		Provider: &provider, Direction: &direction,
		Bytes: &bytes, SHA256: &sha256, CRC32C: &crc32c,
	}
	if !evidence.version.IsZero() {
		value := evidence.version.String()
		wire.Version = &value
	}
	return wire
}

func transferEvidenceFromWire(wire transferEvidenceWire) (TransferEvidence, error) {
	if err := validateTransferEvidenceWire(wire); err != nil {
		return TransferEvidence{}, err
	}
	provider, err := parseProviderToken(*wire.Provider)
	if err != nil {
		return TransferEvidence{}, err
	}
	direction, err := parseDirectionToken(*wire.Direction)
	if err != nil {
		return TransferEvidence{}, err
	}
	evidence := TransferEvidence{
		provider: provider, direction: direction, bytes: *wire.Bytes,
		sha256: *wire.SHA256, crc32c: *wire.CRC32C, set: true,
	}
	evidence.version, err = transferEvidenceVersion(provider, wire.Version)
	if err != nil {
		return TransferEvidence{}, err
	}
	if err := evidence.Validate(); err != nil {
		return TransferEvidence{}, err
	}
	return evidence, nil
}

func validateTransferEvidenceWire(wire transferEvidenceWire) error {
	if wire.Provider == nil || wire.Direction == nil || wire.Bytes == nil ||
		wire.SHA256 == nil || wire.CRC32C == nil {
		return errors.Join(core.ErrObjectStoreContract,
			errors.New(transferEvidenceRequiredMemberErrorText))
	}
	return nil
}

func transferEvidenceVersion(
	provider Provider,
	encoded *string,
) (ProviderVersion, error) {
	if encoded == nil {
		return ProviderVersion{}, nil
	}
	return newProviderVersion(provider, *encoded)
}

func parseDirectionToken(value string) (Direction, error) {
	for direction := DirectionUnknown + 1; direction < directionLimit; direction++ {
		if direction.String() == value {
			return direction, nil
		}
	}
	return DirectionUnknown, core.ErrObjectStoreContract
}

var (
	_ core.Validatable            = TransferEvidence{}
	_ core.Validatable            = TransferEvidenceProjection{}
	_ core.Validatable            = ProviderUploadObservationRequest{}
	_ core.Validatable            = VerifiedProviderUpload{}
	_ core.ValidatedJSONMarshaler = TransferEvidence{}
	_ core.ValidatedJSONMarshaler = TransferEvidenceProjection{}
	_ json.Unmarshaler            = (*TransferEvidence)(nil)
)
