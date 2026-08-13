package release

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/garble"
	"github.com/deliri/primitive/v2026/keygen"
)

// MaterialRequest asks a release authority for the exact secret capabilities
// needed to build and publish one offering at one commit. The authenticated
// transport is deliberately outside this fact: Cloudidentity and Exchange own
// that execution on both sides.
type MaterialRequest struct {
	Version   core.ReleaseVersion      `json:"version"`
	Commit    core.BuildCommit         `json:"commit"`
	Offering  core.Offering            `json:"offering"`
	Nonce     controlwire.RequestNonce `json:"nonce"`
	Revision  controlwire.Revision     `json:"revision"`
	Primitive ProjectVersion           `json:"primitive"`
	Garble    garble.ToolProvenance    `json:"garble"`
}

type materialRequestWire struct {
	Version   *core.ReleaseVersion      `json:"version"`
	Commit    *core.BuildCommit         `json:"commit"`
	Offering  *core.Offering            `json:"offering"`
	Nonce     *controlwire.RequestNonce `json:"nonce"`
	Revision  *controlwire.Revision     `json:"revision"`
	Primitive *ProjectVersion           `json:"primitive"`
	Garble    *garble.ToolProvenance    `json:"garble"`
}

type MaterialRequestInput struct {
	Version  core.ReleaseVersion
	Commit   core.BuildCommit
	Offering core.Offering
	Nonce    controlwire.RequestNonce
}

func NewMaterialRequest(input MaterialRequestInput) (MaterialRequest, error) {
	provenance, err := garble.CurrentTool().Provenance()
	if err != nil {
		return MaterialRequest{}, contractError(err)
	}
	request := MaterialRequest{
		Version: input.Version, Commit: input.Commit, Offering: input.Offering,
		Nonce: input.Nonce, Revision: controlwire.Revision2026V1,
		Primitive: PrimitiveVersion, Garble: provenance,
	}
	return request, request.Validate()
}

func (r MaterialRequest) Validate() error {
	if err := errors.Join(
		r.Version.Validate(), r.Commit.Validate(), r.Offering.Validate(),
		r.Nonce.Validate(), r.Revision.Validate(), r.Primitive.Validate(), r.Garble.Validate(),
	); err != nil {
		return contractError(err)
	}
	if r.Primitive != PrimitiveVersion {
		return contractError(errors.New("release material names a different Primitive version"))
	}
	return nil
}

// ControlRoute projects the sole route admitted by this release request.
func (r MaterialRequest) ControlRoute() (controlwire.RouteContract, error) {
	return controlwire.NewRouteContract(r.Offering, controlwire.RouteFamilyReleaseMaterials)
}

// ControlRevision projects the exact revision carried by this request.
func (r MaterialRequest) ControlRevision() controlwire.Revision { return r.Revision }

// ControlNonce projects the request identity carried by the document.
func (r MaterialRequest) ControlNonce() controlwire.RequestNonce { return r.Nonce }

func (MaterialRequest) ControlRequestBodyLimit() (core.ByteCount, error) {
	return core.NewByteCount(documentExtentMaximum)
}

func (r MaterialRequest) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, jsonError(err)
	}
	version, commit, offering := r.Version, r.Commit, r.Offering
	nonce, revision, primitive, tool := r.Nonce, r.Revision, r.Primitive, r.Garble
	return json.Marshal(materialRequestWire{
		Version: &version, Commit: &commit, Offering: &offering,
		Nonce: &nonce, Revision: &revision, Primitive: &primitive, Garble: &tool,
	})
}

func (r *MaterialRequest) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError(errors.New("release material request receiver is nil"))
	}
	wire, err := decodeStructure[materialRequestWire](data)
	if err != nil {
		return err
	}
	if wire.Version == nil || wire.Commit == nil || wire.Offering == nil ||
		wire.Nonce == nil || wire.Revision == nil || wire.Primitive == nil || wire.Garble == nil {
		return jsonError(errors.New("release material request field is missing"))
	}
	candidate := MaterialRequest{
		Version: *wire.Version, Commit: *wire.Commit, Offering: *wire.Offering,
		Nonce: *wire.Nonce, Revision: *wire.Revision,
		Primitive: *wire.Primitive, Garble: *wire.Garble,
	}
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*r = candidate
	return nil
}

type ReleaseSigningSeed struct {
	value [keygen.SeedSize]byte
}

func NewReleaseSigningSeed(value [keygen.SeedSize]byte) (ReleaseSigningSeed, error) {
	seed := ReleaseSigningSeed{value: value}
	if err := seed.Validate(); err != nil {
		return ReleaseSigningSeed{}, err
	}
	return seed, nil
}

func (s ReleaseSigningSeed) Validate() error {
	if s.value == ([keygen.SeedSize]byte{}) {
		return contractError(errors.New("release signing seed is zero"))
	}
	return nil
}

func (s ReleaseSigningSeed) SigningKey() (keygen.SigningKey, error) {
	if err := s.Validate(); err != nil {
		return keygen.SigningKey{}, err
	}
	return keygen.AdoptSigningKey(s.value)
}

func (s ReleaseSigningSeed) MarshalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return json.Marshal(base64.StdEncoding.EncodeToString(s.value[:]))
}

func (s *ReleaseSigningSeed) UnmarshalJSON(data []byte) error {
	if s == nil {
		return jsonError(errors.New("release signing seed receiver is nil"))
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return jsonError(err)
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(decoded) != keygen.SeedSize || base64.StdEncoding.EncodeToString(decoded) != value {
		clear(decoded)
		return jsonError(errors.New("release signing seed is not canonical base64"), err)
	}
	defer clear(decoded)
	var fixed [keygen.SeedSize]byte
	copy(fixed[:], decoded)
	parsed, err := NewReleaseSigningSeed(fixed)
	clear(fixed[:])
	if err != nil {
		return jsonError(err)
	}
	*s = parsed
	return nil
}

func (ReleaseSigningSeed) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

type GarbleCustodySeed struct {
	value [garble.CustodyBytes]byte
}

func NewGarbleCustodySeed(value [garble.CustodyBytes]byte) (GarbleCustodySeed, error) {
	seed := GarbleCustodySeed{value: value}
	if err := seed.Validate(); err != nil {
		return GarbleCustodySeed{}, err
	}
	return seed, nil
}

func (s GarbleCustodySeed) Validate() error {
	if s.value == ([garble.CustodyBytes]byte{}) {
		return contractError(errors.New("garble custody seed is zero"))
	}
	return nil
}

func (s GarbleCustodySeed) Custody() (garble.Custody, core.SecretMaterial, error) {
	if err := s.Validate(); err != nil {
		return garble.Custody{}, core.SecretMaterial{}, err
	}
	material, err := core.NewSecretMaterial(s.value[:])
	if err != nil {
		return garble.Custody{}, core.SecretMaterial{}, err
	}
	custody, err := garble.NewCustody(material)
	if err != nil {
		return garble.Custody{}, core.SecretMaterial{}, errors.Join(err, material.Destroy())
	}
	return custody, material, nil
}

func (s GarbleCustodySeed) MarshalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return json.Marshal(base64.StdEncoding.EncodeToString(s.value[:]))
}

func (s *GarbleCustodySeed) UnmarshalJSON(data []byte) error {
	if s == nil {
		return jsonError(errors.New("garble custody seed receiver is nil"))
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return jsonError(err)
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(decoded) != garble.CustodyBytes || base64.StdEncoding.EncodeToString(decoded) != value {
		clear(decoded)
		return jsonError(errors.New("garble custody seed is not canonical base64"), err)
	}
	defer clear(decoded)
	var fixed [garble.CustodyBytes]byte
	copy(fixed[:], decoded)
	parsed, err := NewGarbleCustodySeed(fixed)
	clear(fixed[:])
	if err != nil {
		return jsonError(err)
	}
	*s = parsed
	return nil
}

func (GarbleCustodySeed) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

type MaterialResponse struct {
	Request            MaterialRequest       `json:"request"`
	ReleaseSigningSeed ReleaseSigningSeed    `json:"release_signing_seed"`
	GarbleCustodySeed  GarbleCustodySeed     `json:"garble_custody_seed"`
	ServerPublicKey    core.Ed25519PublicKey `json:"server_public_key"`
}

type materialResponseWire struct {
	Request            *MaterialRequest       `json:"request"`
	ReleaseSigningSeed *ReleaseSigningSeed    `json:"release_signing_seed"`
	GarbleCustodySeed  *GarbleCustodySeed     `json:"garble_custody_seed"`
	ServerPublicKey    *core.Ed25519PublicKey `json:"server_public_key"`
}

func (r MaterialResponse) Validate() error {
	if err := errors.Join(
		r.Request.Validate(), r.ReleaseSigningSeed.Validate(),
		r.GarbleCustodySeed.Validate(), r.ServerPublicKey.Validate(),
	); err != nil {
		return contractError(err)
	}
	return nil
}

func (r MaterialResponse) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, jsonError(err)
	}
	request, signing, custody, server := r.Request, r.ReleaseSigningSeed, r.GarbleCustodySeed, r.ServerPublicKey
	return json.Marshal(materialResponseWire{
		Request: &request, ReleaseSigningSeed: &signing,
		GarbleCustodySeed: &custody, ServerPublicKey: &server,
	})
}

func (r *MaterialResponse) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError(errors.New("release material response receiver is nil"))
	}
	wire, err := decodeStructure[materialResponseWire](data)
	if err != nil {
		return err
	}
	if wire.Request == nil || wire.ReleaseSigningSeed == nil || wire.GarbleCustodySeed == nil || wire.ServerPublicKey == nil {
		return jsonError(errors.New("release material response field is missing"))
	}
	candidate := MaterialResponse{
		Request: *wire.Request, ReleaseSigningSeed: *wire.ReleaseSigningSeed,
		GarbleCustodySeed: *wire.GarbleCustodySeed, ServerPublicKey: *wire.ServerPublicKey,
	}
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*r = candidate
	return nil
}

type Material struct {
	SigningKey      keygen.SigningKey
	Custody         garble.Custody
	ServerPublicKey core.Ed25519PublicKey
	custodyMaterial core.SecretMaterial
}

func (r MaterialResponse) Open() (Material, error) {
	if err := r.Validate(); err != nil {
		return Material{}, err
	}
	signing, err := r.ReleaseSigningSeed.SigningKey()
	if err != nil {
		return Material{}, err
	}
	custody, material, err := r.GarbleCustodySeed.Custody()
	if err != nil {
		return Material{}, errors.Join(err, signing.Destroy())
	}
	opened := Material{
		SigningKey: signing, Custody: custody,
		ServerPublicKey: r.ServerPublicKey, custodyMaterial: material,
	}
	return opened, opened.Validate()
}

func (m Material) Validate() error {
	if err := errors.Join(m.SigningKey.Validate(), m.Custody.Validate(), m.ServerPublicKey.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

func (m *Material) Destroy() error {
	if m == nil {
		return contractError(errors.New("release material receiver is nil"))
	}
	return errors.Join(m.SigningKey.Destroy(), m.custodyMaterial.Destroy())
}

func (Material) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

var (
	_ controlwire.RoutedJSONRequest = MaterialRequest{}
	_ core.Validatable              = MaterialRequest{}
	_ core.Validatable              = MaterialResponse{}
	_ core.Validatable              = Material{}
	_ json.Unmarshaler              = (*MaterialRequest)(nil)
	_ json.Unmarshaler              = (*MaterialResponse)(nil)
)
