package runprotocol

import (
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"errors"

	"github.com/deliri/primitive/v2026/core"
	primitiveid "github.com/deliri/primitive/v2026/id"
)

type OriginIdentity struct {
	Offering core.Offering `json:"offering"`
}

func (i OriginIdentity) Validate() error {
	if err := i.Offering.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// EvidenceAuthority names one producer or verifier authority. Field position
// supplies the role; the offering supplies the independently checkable owner.
type EvidenceAuthority struct {
	Offering core.Offering `json:"offering"`
}

func (a EvidenceAuthority) Validate() error {
	if err := a.Offering.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

type SubjectIdentity struct {
	Project    core.Offering      `json:"project"`
	Repository RepositoryIdentity `json:"repository"`
}

func (i SubjectIdentity) Validate() error {
	if err := errors.Join(i.Project.Validate(), i.Repository.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

type SourceCoordinate struct {
	Repository RepositoryIdentity `json:"repository"`
	Commit     core.BuildCommit   `json:"commit"`
	Tree       core.SHA256Digest  `json:"tree_digest"`
}

func (c SourceCoordinate) Validate() error {
	if err := errors.Join(c.Repository.Validate(), c.Commit.Validate(), c.Tree.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

type RequestIdentity struct{ value primitiveid.UUIDv7 }
type RequestNonce struct{ value primitiveid.UUIDv7 }
type RunID struct{ value primitiveid.UUIDv7 }
type ExperimentID struct{ value primitiveid.UUIDv7 }
type ObservationID struct{ value primitiveid.UUIDv7 }
type MachineID struct{ value primitiveid.UUIDv7 }
type MachineGenerationID struct{ value primitiveid.UUIDv7 }
type MachineObservationID struct{ value primitiveid.UUIDv7 }

func NewRequestIdentity(value primitiveid.UUIDv7) (RequestIdentity, error) {
	candidate := RequestIdentity{value: value}
	return candidate, candidate.Validate()
}

// NewRequestNonce admits one caller-owned stable replay coordinate. The
// caller retains the same nonce only when retrying the exact same request.
func NewRequestNonce(value primitiveid.UUIDv7) (RequestNonce, error) {
	candidate := RequestNonce{value: value}
	return candidate, candidate.Validate()
}

// DeriveRequestIdentity binds one caller-owned nonce to the exact origin that
// accepted it. The nonce supplies the UUIDv7 time coordinate; the canonical
// origin and nonce facts supply the remaining identity bits.
func DeriveRequestIdentity(origin OriginIdentity, nonce RequestNonce) (RequestIdentity, error) {
	if err := errors.Join(origin.Validate(), nonce.Validate()); err != nil {
		return RequestIdentity{}, contractError(err)
	}
	// Field order is identity material and is pinned by the canonical fixture
	// test; it must not be mechanically field-aligned.
	canonical, err := core.MarshalCanonicalJSONDocument(struct {
		Origin OriginIdentity `json:"origin"`
		Nonce  RequestNonce   `json:"request_nonce"`
	}{Origin: origin, Nonce: nonce})
	if err != nil {
		return RequestIdentity{}, contractError(err)
	}
	requestBytes, err := uuidIdentityBytes(nonce.value.String())
	if err != nil {
		return RequestIdentity{}, err
	}
	digest := sha256.Sum256(canonical)
	copy(requestBytes[6:], digest[:10])
	requestBytes[6] = 0x70 | (requestBytes[6] & 0x0f)
	requestBytes[8] = 0x80 | (requestBytes[8] & 0x3f)
	uuid, err := primitiveidFromBytes(requestBytes)
	if err != nil {
		return RequestIdentity{}, contractError(err)
	}
	return NewRequestIdentity(uuid)
}

func NewRunID(value primitiveid.UUIDv7) (RunID, error) {
	candidate := RunID{value: value}
	return candidate, candidate.Validate()
}

func NewExperimentID(value primitiveid.UUIDv7) (ExperimentID, error) {
	candidate := ExperimentID{value: value}
	return candidate, candidate.Validate()
}

// Experiment identity projections keep the original canonical protocol order
// independent of in-memory field alignment. Every field is pointer-sized, so
// mechanical layout optimization cannot silently rewrite identity bytes.
type experimentToolTargetWire struct {
	Identity *Identifier `json:"identity"`
	Module   *Identifier `json:"module,omitempty"`
}

type experimentProbeTargetWire struct {
	Kind          *ProbeTargetKind          `json:"kind"`
	GoDeclaration *GoDeclarationTarget      `json:"go_declaration,omitempty"`
	GoFile        *GoFileTarget             `json:"go_file,omitempty"`
	GoPackage     *GoPackageTarget          `json:"go_package,omitempty"`
	JavaScript    *JavaScriptFileTarget     `json:"javascript_file,omitempty"`
	ExternalSuite *NamedTarget              `json:"external_suite,omitempty"`
	Tool          *experimentToolTargetWire `json:"tool,omitempty"`
	CI            *NamedTarget              `json:"ci_plan,omitempty"`
}

type experimentSelectionParentWire struct {
	Request         *RequestIdentity           `json:"request_id"`
	Kind            *ProbeKind                 `json:"kind"`
	Target          *experimentProbeTargetWire `json:"target"`
	ExpansionDigest *core.SHA256Digest         `json:"expansion_digest"`
}

type experimentProbeIdentityWire struct {
	Origin      *OriginIdentity                `json:"origin"`
	Subject     *SubjectIdentity               `json:"subject"`
	Source      *SourceCoordinate              `json:"source"`
	Role        *ProbeRole                     `json:"role"`
	Kind        *ProbeKind                     `json:"kind"`
	Target      *experimentProbeTargetWire     `json:"target"`
	Profile     *ProfileIdentity               `json:"profile"`
	Environment *AdmittedEnvironment           `json:"admitted_environment"`
	Parent      *experimentSelectionParentWire `json:"selection_parent,omitempty"`
}

func experimentProbeTargetProjection(target *ProbeTarget) experimentProbeTargetWire {
	var tool *experimentToolTargetWire
	if target.Tool != nil {
		tool = &experimentToolTargetWire{Identity: &target.Tool.Identity, Module: target.Tool.Module}
	}
	return experimentProbeTargetWire{
		Kind: &target.Kind, GoDeclaration: target.GoDeclaration, GoFile: target.GoFile,
		GoPackage: target.GoPackage, JavaScript: target.JavaScript,
		ExternalSuite: target.ExternalSuite, Tool: tool, CI: target.CI,
	}
}

func experimentProbeIdentityProjection(probe *ProbeIdentity) experimentProbeIdentityWire {
	target := experimentProbeTargetProjection(&probe.Target)
	var parent *experimentSelectionParentWire
	if probe.Parent != nil {
		parentTarget := experimentProbeTargetProjection(&probe.Parent.Target)
		parent = &experimentSelectionParentWire{
			Request: &probe.Parent.Request, Kind: &probe.Parent.Kind, Target: &parentTarget,
			ExpansionDigest: &probe.Parent.ExpansionDigest,
		}
	}
	return experimentProbeIdentityWire{
		Origin: &probe.Origin, Subject: &probe.Subject, Source: &probe.Source,
		Role: &probe.Role, Kind: &probe.Kind, Target: &target, Profile: &probe.Profile,
		Environment: &probe.Environment, Parent: parent,
	}
}

// DeriveExperimentID deterministically binds one child experiment identity to
// its admitted run and complete probe identity. The run supplies the UUIDv7
// time coordinate; the canonical typed facts supply all remaining bits.
func DeriveExperimentID(run RunID, probe ProbeIdentity) (ExperimentID, error) {
	if err := errors.Join(run.Validate(), probe.Validate()); err != nil {
		return ExperimentID{}, contractError(err)
	}
	if probe.Role != ProbeRoleExperiment {
		return ExperimentID{}, contractError(errors.New("run protocol derived experiment probe is not an experiment"))
	}
	probeWire := experimentProbeIdentityProjection(&probe)
	canonical, err := core.MarshalCanonicalJSONDocument(struct {
		Run   *RunID                       `json:"run_id"`
		Probe *experimentProbeIdentityWire `json:"probe"`
	}{Run: &run, Probe: &probeWire})
	if err != nil {
		return ExperimentID{}, contractError(err)
	}
	runBytes, err := uuidIdentityBytes(run.value.String())
	if err != nil {
		return ExperimentID{}, err
	}
	digest := sha256.Sum256(canonical)
	copy(runBytes[6:], digest[:10])
	runBytes[6] = 0x70 | (runBytes[6] & 0x0f)
	runBytes[8] = 0x80 | (runBytes[8] & 0x3f)
	uuid, err := primitiveidFromBytes(runBytes)
	if err != nil {
		return ExperimentID{}, contractError(err)
	}
	return NewExperimentID(uuid)
}

func uuidIdentityBytes(value string) ([16]byte, error) {
	var decoded [16]byte
	var compact [32]byte
	position := 0
	for index := range value {
		if value[index] != '-' {
			if position >= len(compact) {
				return [16]byte{}, contractError(errors.New("uuid identity is oversized"))
			}
			compact[position] = value[index]
			position++
		}
	}
	if position != len(compact) {
		return [16]byte{}, contractError(errors.New("uuid identity has an invalid extent"))
	}
	count, err := hex.Decode(decoded[:], compact[:])
	if err != nil || count != len(decoded) {
		return [16]byte{}, contractError(err)
	}
	return decoded, nil
}

func primitiveidFromBytes(value [16]byte) (primitiveid.UUIDv7, error) {
	var encoded [36]byte
	var compact [32]byte
	hex.Encode(compact[:], value[:])
	copy(encoded[0:8], compact[0:8])
	encoded[8] = '-'
	copy(encoded[9:13], compact[8:12])
	encoded[13] = '-'
	copy(encoded[14:18], compact[12:16])
	encoded[18] = '-'
	copy(encoded[19:23], compact[16:20])
	encoded[23] = '-'
	copy(encoded[24:36], compact[20:32])
	return primitiveid.ParseUUIDv7(string(encoded[:]))
}

func NewObservationID(value primitiveid.UUIDv7) (ObservationID, error) {
	candidate := ObservationID{value: value}
	return candidate, candidate.Validate()
}

func NewMachineID(value primitiveid.UUIDv7) (MachineID, error) {
	candidate := MachineID{value: value}
	return candidate, candidate.Validate()
}

func NewMachineGenerationID(value primitiveid.UUIDv7) (MachineGenerationID, error) {
	candidate := MachineGenerationID{value: value}
	return candidate, candidate.Validate()
}

func NewMachineObservationID(value primitiveid.UUIDv7) (MachineObservationID, error) {
	candidate := MachineObservationID{value: value}
	return candidate, candidate.Validate()
}

func (i RequestIdentity) Validate() error { return validateUUIDIdentity(i.value, "request identity") }
func (i RequestNonce) Validate() error    { return validateUUIDIdentity(i.value, "request nonce") }
func (i RunID) Validate() error           { return validateUUIDIdentity(i.value, "run identity") }
func (i ExperimentID) Validate() error    { return validateUUIDIdentity(i.value, canonicalExperimentText) }
func (i ObservationID) Validate() error   { return validateUUIDIdentity(i.value, "observation") }
func (i MachineID) Validate() error       { return validateUUIDIdentity(i.value, "machine") }
func (i MachineGenerationID) Validate() error {
	return validateUUIDIdentity(i.value, "machine generation")
}
func (i MachineObservationID) Validate() error {
	return validateUUIDIdentity(i.value, "machine observation")
}

// String returns the canonical experiment coordinate for compiler-owned host
// resource names and diagnostics.
func (i ExperimentID) String() string { return i.value.String() }

// String returns the caller-owned nonce for compiler-owned replay keys and
// diagnostics.
func (i RequestNonce) String() string { return i.value.String() }

func validateUUIDIdentity(value primitiveid.UUIDv7, kind string) error {
	if err := value.Validate(); err != nil {
		return contractError(errors.New("run protocol "+kind+" identity is invalid"), err)
	}
	return nil
}

func (i RequestIdentity) MarshalJSON() ([]byte, error) {
	return marshalUUIDIdentity(i.value, i.Validate)
}
func (i RequestNonce) MarshalJSON() ([]byte, error) {
	return marshalUUIDIdentity(i.value, i.Validate)
}
func (i RunID) MarshalJSON() ([]byte, error) { return marshalUUIDIdentity(i.value, i.Validate) }
func (i ExperimentID) MarshalJSON() ([]byte, error) {
	return marshalUUIDIdentity(i.value, i.Validate)
}
func (i ObservationID) MarshalJSON() ([]byte, error) { return marshalUUIDIdentity(i.value, i.Validate) }
func (i MachineID) MarshalJSON() ([]byte, error)     { return marshalUUIDIdentity(i.value, i.Validate) }
func (i MachineGenerationID) MarshalJSON() ([]byte, error) {
	return marshalUUIDIdentity(i.value, i.Validate)
}
func (i MachineObservationID) MarshalJSON() ([]byte, error) {
	return marshalUUIDIdentity(i.value, i.Validate)
}

func marshalUUIDIdentity(value primitiveid.UUIDv7, validate func() error) ([]byte, error) {
	if err := validate(); err != nil {
		return nil, jsonError(err)
	}
	return value.MarshalJSON()
}

func (i *RequestIdentity) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError(errors.New("nil run protocol request identity receiver"))
	}
	var value primitiveid.UUIDv7
	if err := json.Unmarshal(data, &value); err != nil {
		return jsonError(err)
	}
	candidate, err := NewRequestIdentity(value)
	if err != nil {
		return jsonError(err)
	}
	*i = candidate
	return nil
}

func (i *RequestNonce) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError(errors.New("nil run protocol request nonce receiver"))
	}
	var value primitiveid.UUIDv7
	if err := json.Unmarshal(data, &value); err != nil {
		return jsonError(err)
	}
	candidate, err := NewRequestNonce(value)
	if err != nil {
		return jsonError(err)
	}
	*i = candidate
	return nil
}

func (i *RunID) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError(errors.New("nil run protocol run identity receiver"))
	}
	var value primitiveid.UUIDv7
	if err := json.Unmarshal(data, &value); err != nil {
		return jsonError(err)
	}
	candidate, err := NewRunID(value)
	if err != nil {
		return jsonError(err)
	}
	*i = candidate
	return nil
}

func (i *ExperimentID) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError(errors.New("nil run protocol experiment identity receiver"))
	}
	var value primitiveid.UUIDv7
	if err := json.Unmarshal(data, &value); err != nil {
		return jsonError(err)
	}
	candidate, err := NewExperimentID(value)
	if err != nil {
		return jsonError(err)
	}
	*i = candidate
	return nil
}

func (i *ObservationID) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError(errors.New("nil run protocol observation identity receiver"))
	}
	var value primitiveid.UUIDv7
	if err := json.Unmarshal(data, &value); err != nil {
		return jsonError(err)
	}
	candidate, err := NewObservationID(value)
	if err != nil {
		return jsonError(err)
	}
	*i = candidate
	return nil
}

func (i *MachineID) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError(errors.New("nil run protocol machine identity receiver"))
	}
	var value primitiveid.UUIDv7
	if err := json.Unmarshal(data, &value); err != nil {
		return jsonError(err)
	}
	candidate, err := NewMachineID(value)
	if err != nil {
		return jsonError(err)
	}
	*i = candidate
	return nil
}

func (i *MachineGenerationID) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError(errors.New("nil run protocol machine generation identity receiver"))
	}
	var value primitiveid.UUIDv7
	if err := json.Unmarshal(data, &value); err != nil {
		return jsonError(err)
	}
	candidate, err := NewMachineGenerationID(value)
	if err != nil {
		return jsonError(err)
	}
	*i = candidate
	return nil
}

func (i *MachineObservationID) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError(errors.New("nil run protocol machine observation identity receiver"))
	}
	var value primitiveid.UUIDv7
	if err := json.Unmarshal(data, &value); err != nil {
		return jsonError(err)
	}
	candidate, err := NewMachineObservationID(value)
	if err != nil {
		return jsonError(err)
	}
	*i = candidate
	return nil
}

func sameSubject(left, right SubjectIdentity) bool {
	return left.Project == right.Project && left.Repository.value == right.Repository.value
}

var (
	_ json.Marshaler   = RequestIdentity{}
	_ json.Unmarshaler = (*RequestIdentity)(nil)
	_ json.Marshaler   = RequestNonce{}
	_ json.Unmarshaler = (*RequestNonce)(nil)
	_ json.Marshaler   = ObservationID{}
	_ json.Unmarshaler = (*ObservationID)(nil)
)
