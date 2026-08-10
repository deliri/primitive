package release

import (
	"encoding/json"
	"errors"
	"math"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

const (
	artifactIdentityDomain      = "primitive-release-artifact-v1"
	manifestIdentityDomain      = "primitive-release-manifest-v1"
	documentExtentMaximum       = 64 << 10
	binaryFilenameSeparator     = "-"
	binaryFilenameWindowsSuffix = ".exe"
	// BinaryFilenameMaximumBytes bounds the only derived artifact filename.
	BinaryFilenameMaximumBytes = 64
)

// ArtifactIdentity is the nominal digest of immutable artifact facts.
type ArtifactIdentity struct {
	digest core.SHA256Digest
}

// BinaryFilename is a bounded, derived release artifact basename.
type BinaryFilename struct {
	value  [BinaryFilenameMaximumBytes]byte
	length uint32
}

func newBinaryFilename(build core.BuildIdentity) (BinaryFilename, error) {
	if err := build.Validate(); err != nil {
		return BinaryFilename{}, manifestError(err)
	}
	value := build.Offering().String() + binaryFilenameSeparator +
		build.Version().String() + binaryFilenameSeparator + build.Platform().String()
	if build.Platform().OperatingSystem == core.OperatingSystemWindows {
		value += binaryFilenameWindowsSuffix
	}
	return newBoundedFilename(value)
}

func newBoundedFilename(value string) (BinaryFilename, error) {
	if len(value) == 0 || len(value) > BinaryFilenameMaximumBytes {
		return BinaryFilename{}, manifestError(errors.New("derived binary filename exceeds its bound"))
	}
	length, err := core.CheckedUint32FromInt(len(value))
	if err != nil {
		return BinaryFilename{}, manifestError(err)
	}
	var filename BinaryFilename
	copy(filename.value[:], value)
	filename.length = length
	return filename, nil
}

// Validate proves canonical derivation characters, length, and zero padding.
func (f BinaryFilename) Validate() error {
	if f.length == 0 || int(f.length) > len(f.value) {
		return manifestError(errors.New("binary filename is unset"))
	}
	value := string(f.value[:f.length])
	if strings.ContainsAny(value, `/\`) {
		return manifestError(errors.New("binary filename contains a path separator"))
	}
	for _, padding := range f.value[f.length:] {
		if padding != 0 {
			return manifestError(errors.New("binary filename padding is nonzero"))
		}
	}
	return nil
}

// String returns the derived basename, or empty text when invalid.
func (f BinaryFilename) String() string {
	if f.Validate() != nil {
		return ""
	}
	return string(f.value[:f.length])
}

func (f BinaryFilename) MarshalJSON() ([]byte, error) {
	if err := f.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONString(f.String())
	if err != nil {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (f *BinaryFilename) UnmarshalJSON(data []byte) error {
	if f == nil {
		return jsonError(errors.New("binary filename receiver is nil"))
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return jsonError(err)
	}
	candidate, err := newBoundedFilename(value)
	if err != nil {
		return jsonError(err)
	}
	*f = candidate
	return nil
}

func newArtifactIdentity(digest core.SHA256Digest) ArtifactIdentity {
	return ArtifactIdentity{digest: digest}
}

func (i ArtifactIdentity) Validate() error { return i.digest.Validate() }
func (i ArtifactIdentity) String() string {
	value, _ := i.digest.Hex()
	return value
}
func (i ArtifactIdentity) MarshalJSON() ([]byte, error) { return i.digest.MarshalJSON() }
func (i *ArtifactIdentity) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError(errors.New("artifact identity receiver is nil"))
	}
	var digest core.SHA256Digest
	if err := json.Unmarshal(data, &digest); err != nil {
		return jsonError(err)
	}
	*i = newArtifactIdentity(digest)
	return nil
}

// ArtifactIntegrity is the complete fixed-size transfer verification contract.
type ArtifactIntegrity struct {
	extent core.ByteCount
	sha256 core.SHA256Digest
	crc32c core.CRC32C
}

type artifactIntegrityWire struct {
	Extent *core.ByteCount    `json:"extent_bytes"`
	SHA256 *core.SHA256Digest `json:"sha256"`
	CRC32C *core.CRC32C       `json:"crc32c"`
}

func newArtifactIntegrity(extent core.ByteCount, sha core.SHA256Digest, crc core.CRC32C) (ArtifactIntegrity, error) {
	value := ArtifactIntegrity{extent: extent, sha256: sha, crc32c: crc}
	if err := value.Validate(); err != nil {
		return ArtifactIntegrity{}, err
	}
	return value, nil
}

func (i ArtifactIntegrity) Validate() error {
	for _, err := range []error{i.extent.Validate(), i.sha256.Validate(), i.crc32c.Validate()} {
		if err != nil {
			return manifestError(err)
		}
	}
	return nil
}

func (i ArtifactIntegrity) Extent() core.ByteCount    { return i.extent }
func (i ArtifactIntegrity) SHA256() core.SHA256Digest { return i.sha256 }
func (i ArtifactIntegrity) CRC32C() core.CRC32C       { return i.crc32c }
func (i ArtifactIntegrity) MarshalJSON() ([]byte, error) {
	if err := i.Validate(); err != nil {
		return nil, err
	}
	extent, sha, crc := i.extent, i.sha256, i.crc32c
	return json.Marshal(artifactIntegrityWire{Extent: &extent, SHA256: &sha, CRC32C: &crc})
}

func (i *ArtifactIntegrity) UnmarshalJSON(data []byte) error {
	if i == nil {
		return jsonError(errors.New("artifact integrity receiver is nil"))
	}
	wire, err := decodeStructure[artifactIntegrityWire](data)
	if err != nil {
		return err
	}
	if wire.Extent == nil || wire.SHA256 == nil || wire.CRC32C == nil {
		return jsonError(errors.New("artifact integrity field is missing"))
	}
	candidate, err := newArtifactIntegrity(*wire.Extent, *wire.SHA256, *wire.CRC32C)
	if err != nil {
		return jsonError(err)
	}
	*i = candidate
	return nil
}

// ArtifactRequest supplies one artifact's immutable facts.
type ArtifactRequest struct {
	Extent core.ByteCount
	Build  core.BuildIdentity
	CRC32C core.CRC32C
	SHA256 core.SHA256Digest
}

// Validate proves every immutable artifact fact before identity derivation.
func (r ArtifactRequest) Validate() error {
	if err := r.Build.Validate(); err != nil {
		return manifestError(err)
	}
	_, err := newArtifactIntegrity(r.Extent, r.SHA256, r.CRC32C)
	return err
}

// Artifact is one immutable release object. Its filename is derived.
type Artifact struct {
	integrity ArtifactIntegrity
	build     core.BuildIdentity
	identity  ArtifactIdentity
	valid     bool
}

type artifactWire struct {
	Identity  *ArtifactIdentity   `json:"identity"`
	Build     *core.BuildIdentity `json:"build"`
	Integrity *ArtifactIntegrity  `json:"integrity"`
}

func NewArtifact(request ArtifactRequest) (Artifact, error) {
	if err := request.Validate(); err != nil {
		return Artifact{}, err
	}
	integrity, err := newArtifactIntegrity(request.Extent, request.SHA256, request.CRC32C)
	if err != nil {
		return Artifact{}, err
	}
	candidate := Artifact{build: request.Build, integrity: integrity, valid: true}
	digest, err := artifactDigest(candidate)
	if err != nil {
		return Artifact{}, err
	}
	candidate.identity = newArtifactIdentity(digest)
	if err := candidate.Validate(); err != nil {
		return Artifact{}, err
	}
	return candidate, nil
}

func (a Artifact) Validate() error {
	if !a.valid {
		return manifestError(errors.New("artifact is unset"))
	}
	if err := a.identity.Validate(); err != nil {
		return manifestError(err)
	}
	if err := a.build.Validate(); err != nil {
		return manifestError(err)
	}
	if err := a.integrity.Validate(); err != nil {
		return err
	}
	digest, err := artifactDigest(a)
	if err != nil || a.identity != newArtifactIdentity(digest) {
		return manifestError(errors.New("artifact identity does not name its facts"), err)
	}
	return nil
}

func (a Artifact) Identity() ArtifactIdentity   { return a.identity }
func (a Artifact) Build() core.BuildIdentity    { return a.build }
func (a Artifact) Target() core.Platform        { return a.build.Platform() }
func (a Artifact) Integrity() ArtifactIntegrity { return a.integrity }

// Filename returns the basename derived solely from the immutable build.
func (a Artifact) Filename() (BinaryFilename, error) {
	if err := a.Validate(); err != nil {
		return BinaryFilename{}, err
	}
	return newBinaryFilename(a.build)
}

func (a Artifact) MarshalJSON() ([]byte, error) {
	if err := a.Validate(); err != nil {
		return nil, err
	}
	identity, build, integrity := a.identity, a.build, a.integrity
	return json.Marshal(artifactWire{Identity: &identity, Build: &build, Integrity: &integrity})
}

func (a *Artifact) UnmarshalJSON(data []byte) error {
	if a == nil {
		return jsonError(errors.New("artifact receiver is nil"))
	}
	wire, err := decodeStructure[artifactWire](data)
	if err != nil {
		return err
	}
	if wire.Identity == nil || wire.Build == nil || wire.Integrity == nil {
		return jsonError(errors.New("artifact field is missing"))
	}
	candidate := Artifact{
		identity: *wire.Identity, build: *wire.Build,
		integrity: *wire.Integrity, valid: true,
	}
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*a = candidate
	return nil
}

type TargetSet struct {
	values [TargetCount]core.Platform
	valid  bool
}

func Targets() TargetSet {
	return TargetSet{values: [TargetCount]core.Platform{
		{OperatingSystem: core.OperatingSystemWindows, Architecture: core.CPUArchitectureAMD64},
		{OperatingSystem: core.OperatingSystemDarwin, Architecture: core.CPUArchitectureARM64},
		{OperatingSystem: core.OperatingSystemLinux, Architecture: core.CPUArchitectureAMD64},
		{OperatingSystem: core.OperatingSystemLinux, Architecture: core.CPUArchitectureARM64},
	}, valid: true}
}

func (s TargetSet) Validate() error {
	if !s.valid || s != Targets() {
		return manifestError(errors.New("target set is not canonical"))
	}
	return nil
}

func (s TargetSet) At(index int) (core.Platform, bool) {
	if s.Validate() != nil || index < 0 || index >= TargetCount {
		return core.Platform{}, false
	}
	return s.values[index], true
}

// ArtifactSetRequest supplies the exact ordered target slots.
type ArtifactSetRequest struct {
	Artifacts [TargetCount]Artifact
}

// Validate proves every fixed target slot and the one-release closure.
func (r ArtifactSetRequest) Validate() error {
	_, err := artifactSetTotalExtent(r.Artifacts)
	return err
}

// ArtifactSet is fixed storage for exactly one artifact per release target.
type ArtifactSet struct {
	artifacts [TargetCount]Artifact
	total     core.ByteCount
	valid     bool
}

func NewArtifactSet(request ArtifactSetRequest) (ArtifactSet, error) {
	if err := request.Validate(); err != nil {
		return ArtifactSet{}, err
	}
	total, err := artifactSetTotalExtent(request.Artifacts)
	if err != nil {
		return ArtifactSet{}, err
	}
	return ArtifactSet{artifacts: request.Artifacts, total: total, valid: true}, nil
}

// artifactSetTotalExtent proves every slot, its target, and the release
// closure, then returns the exact checked total extent. It is the single
// owner of artifact-set closure for both construction and validation.
func artifactSetTotalExtent(artifacts [TargetCount]Artifact) (core.ByteCount, error) {
	var total uint64
	targets := Targets()
	for index, artifact := range artifacts {
		if err := artifact.Validate(); err != nil {
			return core.ByteCount{}, manifestError(err)
		}
		target, _ := targets.At(index)
		if artifact.Target() != target {
			return core.ByteCount{}, manifestError(errors.New("artifact occupies the wrong target slot"))
		}
		extent, err := artifact.Integrity().Extent().Uint64()
		if err != nil || extent > math.MaxUint64-total {
			return core.ByteCount{}, manifestError(core.ErrNumericOverflow, err)
		}
		total += extent
	}
	if err := validateBuildClosure(artifacts); err != nil {
		return core.ByteCount{}, err
	}
	count, err := core.NewByteCount(total)
	if err != nil {
		return core.ByteCount{}, manifestError(err)
	}
	return count, nil
}

func validateBuildClosure(artifacts [TargetCount]Artifact) error {
	first := artifacts[0].Build()
	for _, artifact := range artifacts[1:] {
		build := artifact.Build()
		if build.Offering() != first.Offering() ||
			build.Version() != first.Version() ||
			build.Commit() != first.Commit() {
			return manifestError(errors.New("artifact builds do not describe one release"))
		}
	}
	return nil
}

func (s ArtifactSet) Validate() error {
	if !s.valid {
		return manifestError(errors.New("artifact set is unset"))
	}
	total, err := artifactSetTotalExtent(s.artifacts)
	if err != nil {
		return manifestError(errors.New("artifact set closure failed"), err)
	}
	if total != s.total {
		return manifestError(errors.New("artifact set total extent differs from its artifacts"))
	}
	return nil
}

func (s ArtifactSet) At(index int) (Artifact, bool) {
	if s.Validate() != nil || index < 0 || index >= TargetCount {
		return Artifact{}, false
	}
	return s.artifacts[index], true
}

func (s ArtifactSet) ForPlatform(platform core.Platform) (Artifact, bool) {
	if s.Validate() != nil || platform.Validate() != nil {
		return Artifact{}, false
	}
	for _, artifact := range s.artifacts {
		if artifact.Target() == platform {
			return artifact, true
		}
	}
	return Artifact{}, false
}

func (s ArtifactSet) TotalExtent() (core.ByteCount, error) {
	if err := s.Validate(); err != nil {
		return core.ByteCount{}, err
	}
	return s.total, nil
}

func (s ArtifactSet) MarshalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(s.artifacts)
}

func (s *ArtifactSet) UnmarshalJSON(data []byte) error {
	if s == nil {
		return jsonError(errors.New("artifact set receiver is nil"))
	}
	decoded, err := decodeStructure[[]Artifact](data)
	if err != nil {
		return err
	}
	if len(decoded) != TargetCount {
		return jsonError(errors.New("artifact set does not carry exactly one artifact per target"))
	}
	var artifacts [TargetCount]Artifact
	copy(artifacts[:], decoded)
	candidate, err := NewArtifactSet(ArtifactSetRequest{Artifacts: artifacts})
	if err != nil {
		return jsonError(err)
	}
	*s = candidate
	return nil
}

func artifactDigest(artifact Artifact) (core.SHA256Digest, error) {
	if err := artifact.build.Validate(); err != nil {
		return core.SHA256Digest{}, manifestError(err)
	}
	if err := artifact.integrity.Validate(); err != nil {
		return core.SHA256Digest{}, err
	}
	body, err := json.Marshal(struct {
		Build     core.BuildIdentity `json:"build"`
		Integrity ArtifactIntegrity  `json:"integrity"`
	}{Build: artifact.build, Integrity: artifact.integrity})
	if err != nil {
		return core.SHA256Digest{}, manifestError(err)
	}
	return framedDigest(artifactIdentityDomain, body), nil
}

// framedDigest binds a digest to one compiler-owned domain. The framing is
// injective because each domain is NUL-free and the body begins after the one
// NUL separator. The bounded input is assembled once and hashed through
// Core's one whole-buffer door; every body here is already held complete
// under this package's document ceilings, so nothing streams.
func framedDigest(domain string, body []byte) core.SHA256Digest {
	input := make([]byte, 0, len(domain)+1+len(body))
	input = append(input, domain...)
	input = append(input, 0)
	input = append(input, body...)
	return core.SHA256Of(input)
}
