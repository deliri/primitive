package release

import (
	"encoding/base64"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"

	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/keygen"
)

// MaterialRequest asks a release authority for the exact secret capabilities
// needed to build and publish one offering at one commit. The authenticated
// transport is deliberately outside this fact: Cloudidentity and Exchange own
// that execution on both sides.
type MaterialRequest struct {
	Primitive ProjectVersion           `json:"primitive"`
	Offering  core.Offering            `json:"offering"`
	Version   core.ReleaseVersion      `json:"version"`
	Commit    core.BuildCommit         `json:"commit"`
	Nonce     controlwire.RequestNonce `json:"nonce"`
	Revision  controlwire.Revision     `json:"revision"`
}

type materialRequestWire struct {
	Version   *core.ReleaseVersion      `json:"version"`
	Commit    *core.BuildCommit         `json:"commit"`
	Offering  *core.Offering            `json:"offering"`
	Nonce     *controlwire.RequestNonce `json:"nonce"`
	Revision  *controlwire.Revision     `json:"revision"`
	Primitive *ProjectVersion           `json:"primitive"`
}

type MaterialRequestInput struct {
	Offering core.Offering
	Version  core.ReleaseVersion
	Commit   core.BuildCommit
	Nonce    controlwire.RequestNonce
}

func NewMaterialRequest(input MaterialRequestInput) (MaterialRequest, error) {
	request := MaterialRequest{
		Version: input.Version, Commit: input.Commit, Offering: input.Offering,
		Nonce: input.Nonce, Revision: controlwire.Revision2026V1,
		Primitive: PrimitiveVersion,
	}
	return request, request.Validate()
}

func (r MaterialRequest) Validate() error {
	if err := errors.Join(
		r.Version.Validate(), r.Commit.Validate(), r.Offering.Validate(),
		r.Nonce.Validate(), r.Revision.Validate(), r.Primitive.Validate(),
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
	nonce, revision, primitive := r.Nonce, r.Revision, r.Primitive
	return json.Marshal(materialRequestWire{
		Version: &version, Commit: &commit, Offering: &offering,
		Nonce: &nonce, Revision: &revision, Primitive: &primitive,
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
	if !wire.complete() {
		return jsonError(errors.New("release material request field is missing"))
	}
	candidate := MaterialRequest{
		Version: *wire.Version, Commit: *wire.Commit, Offering: *wire.Offering,
		Nonce: *wire.Nonce, Revision: *wire.Revision,
		Primitive: *wire.Primitive,
	}
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*r = candidate
	return nil
}

func (w materialRequestWire) complete() bool {
	return w.Version != nil && w.Commit != nil && w.Offering != nil &&
		w.Nonce != nil && w.Revision != nil && w.Primitive != nil
}

type ReleaseSigningSeed struct {
	material core.SecretMaterial
}

func NewReleaseSigningSeed(value [keygen.SeedSize]byte) (ReleaseSigningSeed, error) {
	material, err := core.NewSecretMaterial(value[:])
	if err != nil {
		return ReleaseSigningSeed{}, contractError(err)
	}
	seed := ReleaseSigningSeed{material: material}
	if err := seed.Validate(); err != nil {
		_ = material.Destroy()
		return ReleaseSigningSeed{}, err
	}
	return seed, nil
}

func (s ReleaseSigningSeed) Validate() error {
	if err := s.material.Validate(); err != nil {
		return contractError(err)
	}
	count, err := s.material.ByteCount()
	if err != nil {
		return contractError(err)
	}
	extent, err := count.Uint64()
	if err != nil || extent != keygen.SeedSize {
		return contractError(errors.New("release signing seed has invalid extent"), err)
	}
	return nil
}

func (s ReleaseSigningSeed) SigningKey() (keygen.SigningKey, error) {
	if err := s.Validate(); err != nil {
		return keygen.SigningKey{}, err
	}
	raw, err := s.material.CopyBytes()
	if err != nil {
		return keygen.SigningKey{}, contractError(err)
	}
	defer clear(raw)
	var seed [keygen.SeedSize]byte
	copy(seed[:], raw)
	defer clear(seed[:])
	return keygen.AdoptSigningKey(seed)
}

func (s ReleaseSigningSeed) MarshalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, jsonError(err)
	}
	raw, err := s.material.CopyBytes()
	if err != nil {
		return nil, jsonError(contractError(err))
	}
	defer clear(raw)
	return json.Marshal(base64.StdEncoding.EncodeToString(raw))
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
	if s.material != (core.SecretMaterial{}) {
		if err := s.Destroy(); err != nil {
			_ = parsed.Destroy()
			return jsonError(err)
		}
	}
	*s = parsed
	return nil
}

// Destroy clears the release-signing seed and invalidates every copied handle.
func (s ReleaseSigningSeed) Destroy() error {
	if err := s.material.Destroy(); err != nil {
		return contractError(err)
	}
	return nil
}

func (ReleaseSigningSeed) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

type MaterialResponse struct {
	ReleaseSigningSeed ReleaseSigningSeed    `json:"release_signing_seed"`
	Request            MaterialRequest       `json:"request"`
	ServerPublicKey    core.Ed25519PublicKey `json:"server_public_key"`
}

type materialResponseWire struct {
	Request            *MaterialRequest       `json:"request"`
	ReleaseSigningSeed *ReleaseSigningSeed    `json:"release_signing_seed"`
	ServerPublicKey    *core.Ed25519PublicKey `json:"server_public_key"`
}

const materialResponseNilReceiverDiagnostic = "release material response receiver is nil"

func (r MaterialResponse) Validate() error {
	if err := errors.Join(
		r.Request.Validate(), r.ReleaseSigningSeed.Validate(),
		r.ServerPublicKey.Validate(),
	); err != nil {
		return contractError(err)
	}
	return nil
}

func (r MaterialResponse) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, jsonError(err)
	}
	request, signing, server := r.Request, r.ReleaseSigningSeed, r.ServerPublicKey
	return json.Marshal(materialResponseWire{
		Request: &request, ReleaseSigningSeed: &signing,
		ServerPublicKey: &server,
	})
}

func (r *MaterialResponse) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError(errors.New(materialResponseNilReceiverDiagnostic))
	}
	wire, err := decodeStructure[materialResponseWire](data)
	if err != nil {
		return err
	}
	if wire.Request == nil || wire.ReleaseSigningSeed == nil || wire.ServerPublicKey == nil {
		return jsonError(errors.New("release material response field is missing"))
	}
	candidate := MaterialResponse{
		Request: *wire.Request, ReleaseSigningSeed: *wire.ReleaseSigningSeed,
		ServerPublicKey: *wire.ServerPublicKey,
	}
	if err := candidate.Validate(); err != nil {
		return jsonError(errors.Join(err, candidate.Destroy()))
	}
	if err := r.Destroy(); err != nil {
		_ = candidate.Destroy()
		return jsonError(err)
	}
	*r = candidate
	return nil
}

type Material struct {
	SigningKey      keygen.SigningKey
	ServerPublicKey core.Ed25519PublicKey
}

// Destroy clears the response seed and invalidates every copied handle.
func (r *MaterialResponse) Destroy() error {
	if r == nil {
		return contractError(errors.New(materialResponseNilReceiverDiagnostic))
	}
	if *r == (MaterialResponse{}) {
		return nil
	}
	err := r.ReleaseSigningSeed.Destroy()
	*r = MaterialResponse{}
	return err
}

// Open consumes one material response into the longer-lived typed
// capabilities. The response seeds are destroyed on every terminal path.
func (r *MaterialResponse) Open() (Material, error) {
	if r == nil {
		return Material{}, contractError(errors.New(materialResponseNilReceiverDiagnostic))
	}
	if err := r.Validate(); err != nil {
		return Material{}, errors.Join(err, r.Destroy())
	}
	signing, err := r.ReleaseSigningSeed.SigningKey()
	if err != nil {
		_ = r.Destroy()
		return Material{}, err
	}
	opened := Material{
		SigningKey: signing, ServerPublicKey: r.ServerPublicKey,
	}
	if err := opened.Validate(); err != nil {
		return Material{}, errors.Join(err, opened.Destroy(), r.Destroy())
	}
	if err := r.Destroy(); err != nil {
		return Material{}, errors.Join(err, opened.Destroy())
	}
	return opened, nil
}

func (m Material) Validate() error {
	if err := errors.Join(m.SigningKey.Validate(), m.ServerPublicKey.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

func (m *Material) Destroy() error {
	if m == nil {
		return contractError(errors.New("release material receiver is nil"))
	}
	if err := m.SigningKey.Destroy(); err != nil {
		return contractError(err)
	}
	return nil
}

// Format redacts the unopened response because it carries both secret seeds.
func (MaterialResponse) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
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
