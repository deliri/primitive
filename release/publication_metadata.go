package release

import (
	"encoding/json"
	"errors"
	"hash/crc32"
	"io"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/garble"
)

const (
	// MetadataAssetCount is the exact number of non-binary documents bound to
	// every release manifest.
	MetadataAssetCount            = 3
	metadataDependenciesSuffix    = "dependencies.json"
	metadataDocumentationSuffix   = "documentation.zip"
	metadataReleaseNotesSuffix    = "release-notes.md"
	metadataJSONMediaType         = "application/json"
	metadataZIPMediaType          = "application/zip"
	metadataMarkdownMediaType     = "text/markdown; charset=utf-8"
	metadataInspectionBufferBytes = 64 << 10
	// MetadataAssetMaximumBytes bounds each dependency, documentation, or
	// release-note object independently of object size in memory.
	MetadataAssetMaximumBytes = 256 << 20
)

// MetadataKind is the closed set of customer-visible release metadata.
type MetadataKind uint8

const (
	MetadataKindUnknown MetadataKind = iota
	MetadataKindDependencies
	MetadataKindDocumentation
	MetadataKindReleaseNotes
	metadataKindLimit
)

func metadataKindLabels() [metadataKindLimit]string {
	return [...]string{"", "dependencies", "documentation", "release_notes"}
}

func (k MetadataKind) Validate() error {
	if k <= MetadataKindUnknown || k >= metadataKindLimit || metadataKindLabels()[k] == "" {
		return contractError(errors.New("metadata kind is outside the closed domain"))
	}
	return nil
}

func (k MetadataKind) IsValid() bool { return k.Validate() == nil }
func (MetadataKind) OffWireEnum()    {}
func (k MetadataKind) String() string {
	if !k.IsValid() {
		return core.UnknownEnumDiagnostic
	}
	return metadataKindLabels()[k]
}

func (k MetadataKind) MarshalJSON() ([]byte, error) {
	if err := k.Validate(); err != nil {
		return nil, jsonError(err)
	}
	return json.Marshal(k.String())
}

func (k *MetadataKind) UnmarshalJSON(data []byte) error {
	if k == nil {
		return jsonError(errors.New("metadata kind receiver is nil"))
	}
	var token string
	if err := json.Unmarshal(data, &token); err != nil {
		return jsonError(err)
	}
	for candidate := MetadataKindUnknown + 1; candidate < metadataKindLimit; candidate++ {
		if candidate.String() == token {
			canonical, _ := json.Marshal(token)
			if string(canonical) != string(data) {
				return jsonError(errors.New("metadata kind is not canonical"))
			}
			*k = candidate
			return nil
		}
	}
	return jsonError(errors.New("metadata kind is unsupported"))
}

// MetadataAssetRequest supplies integrity for one compiler-owned metadata role.
type MetadataAssetRequest struct {
	Extent core.ByteCount
	SHA256 core.SHA256Digest
	CRC32C core.CRC32C
	Kind   MetadataKind
}

// MetadataInspectionRequest supplies one bounded metadata stream. The exact
// extent is proved against the source before an immutable asset is returned.
type MetadataInspectionRequest struct {
	Source io.ReaderAt
	Extent core.ByteCount
	Kind   MetadataKind
}

func (r MetadataInspectionRequest) Validate() error {
	if r.Source == nil {
		return manifestError(errors.New("metadata inspection source is nil"))
	}
	if err := r.Kind.Validate(); err != nil {
		return manifestError(err)
	}
	extent, err := r.Extent.Uint64()
	if err != nil || extent == 0 || extent > MetadataAssetMaximumBytes {
		return manifestError(errors.New("metadata inspection extent is outside its bound"), err)
	}
	return nil
}

// InspectMetadataAsset streams one exact metadata object through SHA-256 and
// CRC32C without retaining its bytes.
func InspectMetadataAsset(request MetadataInspectionRequest) (MetadataAsset, error) {
	if err := request.Validate(); err != nil {
		return MetadataAsset{}, err
	}
	extent, _ := request.Extent.Int64()
	bounded := io.NewSectionReader(request.Source, 0, extent)
	sha := core.NewDigestWriter()
	crc := crc32.New(crc32.MakeTable(crc32.Castagnoli))
	written, err := io.CopyBuffer(io.MultiWriter(sha, crc), bounded,
		make([]byte, metadataInspectionBufferBytes))
	if err != nil || written != extent {
		return MetadataAsset{}, manifestError(errors.New("stream metadata asset"), err)
	}
	var extra [1]byte
	read, readErr := request.Source.ReadAt(extra[:], extent)
	if read != 0 || !errors.Is(readErr, io.EOF) {
		return MetadataAsset{}, manifestError(errors.New("metadata asset exceeds its declared extent"), readErr)
	}
	shaDigest, _, err := sha.Seal()
	if err != nil {
		return MetadataAsset{}, manifestError(errors.New("stream metadata asset"), err)
	}
	return NewMetadataAsset(MetadataAssetRequest{
		Kind: request.Kind, Extent: request.Extent,
		SHA256: shaDigest, CRC32C: core.NewCRC32C(crc.Sum32()),
	})
}

func (r MetadataAssetRequest) Validate() error {
	if err := r.Kind.Validate(); err != nil {
		return manifestError(err)
	}
	_, err := newArtifactIntegrity(r.Extent, r.SHA256, r.CRC32C)
	return err
}

// MetadataAsset is one immutable dependency, documentation, or release-note
// object. Filename and media type are derived from its closed role.
type MetadataAsset struct {
	integrity ArtifactIntegrity
	kind      MetadataKind
	valid     bool
}

type metadataAssetWire struct {
	Kind      *MetadataKind       `json:"kind"`
	MediaType *core.HTTPMediaType `json:"media_type"`
	Integrity *ArtifactIntegrity  `json:"integrity"`
}

func NewMetadataAsset(request MetadataAssetRequest) (MetadataAsset, error) {
	if err := request.Validate(); err != nil {
		return MetadataAsset{}, err
	}
	integrity, err := newArtifactIntegrity(request.Extent, request.SHA256, request.CRC32C)
	if err != nil {
		return MetadataAsset{}, err
	}
	return MetadataAsset{kind: request.Kind, integrity: integrity, valid: true}, nil
}

func (a MetadataAsset) Validate() error {
	if !a.valid {
		return manifestError(errors.New("metadata asset is unset"))
	}
	if err := a.kind.Validate(); err != nil {
		return manifestError(err)
	}
	return a.integrity.Validate()
}

func (a MetadataAsset) Kind() MetadataKind           { return a.kind }
func (a MetadataAsset) Integrity() ArtifactIntegrity { return a.integrity }

func (a MetadataAsset) ContentType() (core.HTTPMediaType, error) {
	if err := a.Validate(); err != nil {
		return core.HTTPMediaType{}, err
	}
	return metadataContentType(a.kind)
}

func (a MetadataAsset) Filename(
	offering core.Offering,
	version core.ReleaseVersion,
) (BinaryFilename, error) {
	if err := a.Validate(); err != nil {
		return BinaryFilename{}, err
	}
	if err := offering.Validate(); err != nil {
		return BinaryFilename{}, manifestError(err)
	}
	if err := version.Validate(); err != nil {
		return BinaryFilename{}, manifestError(err)
	}
	suffix, err := metadataFilenameSuffix(a.kind)
	if err != nil {
		return BinaryFilename{}, err
	}
	return newBoundedFilename(offering.String() + binaryFilenameSeparator +
		version.String() + binaryFilenameSeparator + suffix)
}

func (a MetadataAsset) MarshalJSON() ([]byte, error) {
	if err := a.Validate(); err != nil {
		return nil, err
	}
	mediaType, err := a.ContentType()
	if err != nil {
		return nil, err
	}
	kind, integrity := a.kind, a.integrity
	return json.Marshal(metadataAssetWire{
		Kind: &kind, MediaType: &mediaType, Integrity: &integrity,
	})
}

func (a *MetadataAsset) UnmarshalJSON(data []byte) error {
	if a == nil {
		return jsonError(errors.New("metadata asset receiver is nil"))
	}
	wire, err := decodeStructure[metadataAssetWire](data)
	if err != nil {
		return err
	}
	if wire.Kind == nil || wire.MediaType == nil || wire.Integrity == nil {
		return jsonError(errors.New("metadata asset field is missing"))
	}
	wantMediaType, err := metadataContentType(*wire.Kind)
	if err != nil || *wire.MediaType != wantMediaType {
		return jsonError(errors.New("metadata media type differs from its kind"), err)
	}
	candidate := MetadataAsset{kind: *wire.Kind, integrity: *wire.Integrity, valid: true}
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*a = candidate
	return nil
}

func metadataContentType(kind MetadataKind) (core.HTTPMediaType, error) {
	var value string
	switch kind {
	case MetadataKindDependencies:
		value = metadataJSONMediaType
	case MetadataKindDocumentation:
		value = metadataZIPMediaType
	case MetadataKindReleaseNotes:
		value = metadataMarkdownMediaType
	default:
		return core.HTTPMediaType{}, contractError(errors.New("metadata kind has no media type"))
	}
	parsed, err := core.ParseHTTPMediaType(value)
	if err != nil {
		return core.HTTPMediaType{}, manifestError(err)
	}
	return parsed, nil
}

func metadataFilenameSuffix(kind MetadataKind) (string, error) {
	switch kind {
	case MetadataKindDependencies:
		return metadataDependenciesSuffix, nil
	case MetadataKindDocumentation:
		return metadataDocumentationSuffix, nil
	case MetadataKindReleaseNotes:
		return metadataReleaseNotesSuffix, nil
	default:
		return "", contractError(errors.New("metadata kind has no filename"))
	}
}

// MetadataSetRequest supplies the exact ordered metadata slots.
type MetadataSetRequest struct {
	Assets [MetadataAssetCount]MetadataAsset
}

func (r MetadataSetRequest) Validate() error { return validateMetadataAssets(r.Assets) }

// MetadataSet is fixed storage for one asset per metadata role.
type MetadataSet struct {
	assets [MetadataAssetCount]MetadataAsset
	valid  bool
}

func NewMetadataSet(request MetadataSetRequest) (MetadataSet, error) {
	if err := request.Validate(); err != nil {
		return MetadataSet{}, err
	}
	return MetadataSet{assets: request.Assets, valid: true}, nil
}

func (s MetadataSet) Validate() error {
	if !s.valid {
		return manifestError(errors.New("metadata set is unset"))
	}
	return validateMetadataAssets(s.assets)
}

func validateMetadataAssets(assets [MetadataAssetCount]MetadataAsset) error {
	for index, asset := range assets {
		if err := asset.Validate(); err != nil {
			return err
		}
		if asset.kind != MetadataKind(index+1) {
			return manifestError(errors.New("metadata asset occupies the wrong role slot"))
		}
	}
	return nil
}

func (s MetadataSet) At(index int) (MetadataAsset, bool) {
	if s.Validate() != nil || index < 0 || index >= MetadataAssetCount {
		return MetadataAsset{}, false
	}
	return s.assets[index], true
}

func (s MetadataSet) MarshalJSON() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(s.assets)
}

func (s *MetadataSet) UnmarshalJSON(data []byte) error {
	if s == nil {
		return jsonError(errors.New("metadata set receiver is nil"))
	}
	decoded, err := decodeStructure[[]MetadataAsset](data)
	if err != nil {
		return err
	}
	if len(decoded) != MetadataAssetCount {
		return jsonError(errors.New("metadata set cardinality is invalid"))
	}
	var assets [MetadataAssetCount]MetadataAsset
	copy(assets[:], decoded)
	candidate, err := NewMetadataSet(MetadataSetRequest{Assets: assets})
	if err != nil {
		return jsonError(err)
	}
	*s = candidate
	return nil
}

// BuildProvenanceRequest binds public reproducibility facts to one exact plan
// and one verified pair of build executables. The Garble seed remains secret;
// callers reproduce it from custody and the named derivation generation.
type BuildProvenanceRequest struct {
	Tools                VerifiedBuildTools
	Plan                 BuildPlan
	DerivationGeneration garble.DerivationGeneration
}

type BuildProvenance struct {
	mainPackage            MainPackage
	linkerAssignments      LinkerAssignments
	buildTags              BuildTags
	goExecutableDigest     core.SHA256Digest
	garbleExecutableDigest core.SHA256Digest
	garbleTool             garble.ToolIdentity
	goToolchain            GoToolchainIdentity
	moduleMode             BuildModuleMode
	literals               garble.LiteralPolicy
	diagnostics            garble.DiagnosticPolicy
	derivationGeneration   garble.DerivationGeneration
	valid                  bool
}

type linkerAssignmentWire struct {
	Symbol string `json:"symbol"`
	Value  string `json:"value"`
}

type buildProvenanceWire struct {
	GarbleDerivation       string                 `json:"garble_derivation"`
	GarbleModule           string                 `json:"garble_module"`
	GarbleVersion          string                 `json:"garble_version"`
	GarbleRevision         string                 `json:"garble_revision"`
	GarbleModuleSum        string                 `json:"garble_module_sum"`
	GarbleLiterals         string                 `json:"garble_literals"`
	GarbleDiagnostics      string                 `json:"garble_diagnostics"`
	GoToolchain            string                 `json:"go_toolchain"`
	MainPackage            string                 `json:"main_package"`
	ModuleMode             string                 `json:"module_mode"`
	BuildTags              []string               `json:"build_tags"`
	LinkerAssignments      []linkerAssignmentWire `json:"linker_assignments"`
	GoExecutableSHA256     core.SHA256Digest      `json:"go_executable_sha256"`
	GarbleExecutableSHA256 core.SHA256Digest      `json:"garble_executable_sha256"`
}

func (r BuildProvenanceRequest) Validate() error {
	_, err := buildProvenance(r)
	return err
}

func NewBuildProvenance(request BuildProvenanceRequest) (BuildProvenance, error) {
	if err := request.Validate(); err != nil {
		return BuildProvenance{}, err
	}
	return buildProvenance(request)
}

func buildProvenance(request BuildProvenanceRequest) (BuildProvenance, error) {
	if err := request.Plan.Validate(); err != nil {
		return BuildProvenance{}, manifestError(err)
	}
	if err := request.Tools.Validate(); err != nil {
		return BuildProvenance{}, manifestError(err)
	}
	if err := request.DerivationGeneration.Validate(); err != nil {
		return BuildProvenance{}, manifestError(err)
	}
	if request.DerivationGeneration != garble.CurrentDerivationGeneration() {
		return BuildProvenance{}, manifestError(errors.New("build provenance does not use the current derivation"))
	}
	plan := request.Plan.request
	literals, err := plan.Garble.LiteralPolicy()
	if err != nil {
		return BuildProvenance{}, manifestError(err)
	}
	diagnostics, err := plan.Garble.DiagnosticPolicy()
	if err != nil {
		return BuildProvenance{}, manifestError(err)
	}
	value := BuildProvenance{
		linkerAssignments: plan.LinkerAssignments, buildTags: plan.BuildTags,
		mainPackage:            plan.MainPackage,
		goExecutableDigest:     request.Tools.GoExecutableDigest(),
		garbleExecutableDigest: request.Tools.GarbleExecutableDigest(),
		garbleTool:             request.Tools.GarbleTool(), goToolchain: request.Tools.GoToolchain(),
		moduleMode: plan.ModuleMode, literals: literals, diagnostics: diagnostics,
		derivationGeneration: request.DerivationGeneration, valid: true,
	}
	if err := value.Validate(); err != nil {
		return BuildProvenance{}, err
	}
	return value, nil
}

// Validate accepts every explicitly admitted historical tool set. Construction
// separately pins new builds to the current toolchain, Garble tool, and seed
// derivation; reading a signed older manifest must not consult those selectors.
func (p BuildProvenance) Validate() error {
	if !p.valid {
		return manifestError(errors.New("build provenance is unset"))
	}
	for _, err := range []error{
		p.linkerAssignments.Validate(), p.buildTags.Validate(), p.mainPackage.Validate(),
		p.goExecutableDigest.Validate(), p.garbleExecutableDigest.Validate(),
		p.garbleTool.Validate(), p.goToolchain.Validate(), p.moduleMode.Validate(),
		p.literals.Validate(), p.diagnostics.Validate(), p.derivationGeneration.Validate(),
	} {
		if err != nil {
			return manifestError(errors.New("build provenance is invalid"), err)
		}
	}
	if !admittedBuildProvenanceToolSet(
		p.goToolchain, p.garbleTool, p.derivationGeneration,
	) {
		return manifestError(errors.New("build provenance tool set is not admitted"))
	}
	return nil
}

func admittedBuildProvenanceToolSet(
	goToolchain GoToolchainIdentity,
	garbleTool garble.ToolIdentity,
	derivation garble.DerivationGeneration,
) bool {
	return goToolchain == GoToolchainPrimitive2026 &&
		garbleTool == garble.ToolIdentityPrimitive2026 &&
		derivation == garble.DerivationGenerationOne
}

func (p BuildProvenance) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	wire, err := p.wire()
	if err != nil {
		return nil, err
	}
	return json.Marshal(wire)
}

func (p BuildProvenance) wire() (buildProvenanceWire, error) {
	goVersion, err := p.goToolchain.Version()
	if err != nil {
		return buildProvenanceWire{}, err
	}
	module, err := p.garbleTool.ModulePath()
	if err != nil {
		return buildProvenanceWire{}, err
	}
	version, err := p.garbleTool.Version()
	if err != nil {
		return buildProvenanceWire{}, err
	}
	revision, err := p.garbleTool.Revision()
	if err != nil {
		return buildProvenanceWire{}, err
	}
	sum, err := p.garbleTool.ModuleSum()
	if err != nil {
		return buildProvenanceWire{}, err
	}
	assignments := make([]linkerAssignmentWire, p.linkerAssignments.count)
	for index := range p.linkerAssignments.count {
		value := p.linkerAssignments.values[index]
		assignments[index] = linkerAssignmentWire{Symbol: value.symbol, Value: value.value}
	}
	tags := make([]string, p.buildTags.count)
	for index := range p.buildTags.count {
		tags[index] = p.buildTags.values[index].value
	}
	return buildProvenanceWire{
		BuildTags:   tags,
		GoToolchain: goVersion, GoExecutableSHA256: p.goExecutableDigest,
		GarbleModule: module, GarbleVersion: version, GarbleRevision: revision,
		GarbleModuleSum: sum, GarbleExecutableSHA256: p.garbleExecutableDigest,
		GarbleLiterals: p.literals.String(), GarbleDiagnostics: p.diagnostics.String(),
		GarbleDerivation: p.derivationGeneration.String(), MainPackage: p.mainPackage.String(),
		ModuleMode: p.moduleMode.String(), LinkerAssignments: assignments,
	}, nil
}

func (p *BuildProvenance) UnmarshalJSON(data []byte) error {
	if p == nil {
		return jsonError(errors.New("build provenance receiver is nil"))
	}
	wire, err := decodeStructure[buildProvenanceWire](data)
	if err != nil {
		return err
	}
	candidate, err := buildProvenanceFromWire(wire)
	if err != nil {
		return jsonError(err)
	}
	*p = candidate
	return nil
}

func buildProvenanceFromWire(w buildProvenanceWire) (BuildProvenance, error) {
	goToolchain, garbleTool, derivation, err := parseBuildProvenanceTools(w)
	if err != nil {
		return BuildProvenance{}, err
	}
	mainPackage, err := ParseMainPackage(w.MainPackage)
	if err != nil {
		return BuildProvenance{}, err
	}
	linkers, tags, err := buildSelectorsFromWire(w)
	if err != nil {
		return BuildProvenance{}, err
	}
	moduleMode, err := parseBuildModuleMode(w.ModuleMode)
	if err != nil {
		return BuildProvenance{}, err
	}
	literals, err := garble.ParseLiteralPolicy(w.GarbleLiterals)
	if err != nil {
		return BuildProvenance{}, manifestError(err)
	}
	diagnostics, err := garble.ParseDiagnosticPolicy(w.GarbleDiagnostics)
	if err != nil {
		return BuildProvenance{}, manifestError(err)
	}
	candidate := BuildProvenance{
		linkerAssignments: linkers, buildTags: tags, mainPackage: mainPackage,
		goExecutableDigest: w.GoExecutableSHA256, garbleExecutableDigest: w.GarbleExecutableSHA256,
		garbleTool: garbleTool, goToolchain: goToolchain,
		moduleMode: moduleMode, literals: literals, diagnostics: diagnostics,
		derivationGeneration: derivation, valid: true,
	}
	if err := candidate.Validate(); err != nil {
		return BuildProvenance{}, err
	}
	return candidate, nil
}

func parseBuildProvenanceTools(
	w buildProvenanceWire,
) (GoToolchainIdentity, garble.ToolIdentity, garble.DerivationGeneration, error) {
	goToolchain, err := parseGoToolchainVersion(w.GoToolchain)
	if err != nil {
		return GoToolchainUnknown, garble.ToolIdentityUnknown, garble.DerivationGenerationUnknown, err
	}
	garbleTool, err := garble.ResolveTool(garble.ToolProvenance{
		ModulePath: w.GarbleModule, Version: w.GarbleVersion,
		Revision: w.GarbleRevision, ModuleSum: w.GarbleModuleSum,
	})
	if err != nil {
		return GoToolchainUnknown, garble.ToolIdentityUnknown, garble.DerivationGenerationUnknown,
			manifestError(err)
	}
	derivation, err := garble.ParseDerivationGeneration(w.GarbleDerivation)
	if err != nil {
		return GoToolchainUnknown, garble.ToolIdentityUnknown, garble.DerivationGenerationUnknown,
			manifestError(err)
	}
	return goToolchain, garbleTool, derivation, nil
}

// buildSelectorsFromWire reconstructs the two compiler-owned selector sets that
// decide what the release build compiled. A published provenance document is
// already canonical, so a reordered or duplicated set is rejected by the owning
// constructor rather than silently canonicalized.
func buildSelectorsFromWire(w buildProvenanceWire) (LinkerAssignments, BuildTags, error) {
	linkers, err := linkerAssignmentsFromWire(w.LinkerAssignments)
	if err != nil {
		return LinkerAssignments{}, BuildTags{}, err
	}
	tags, err := buildTagsFromWire(w.BuildTags)
	if err != nil {
		return LinkerAssignments{}, BuildTags{}, err
	}
	return linkers, tags, nil
}

func buildTagsFromWire(wire []string) (BuildTags, error) {
	tags := make([]BuildTag, len(wire))
	for index, value := range wire {
		tag, err := ParseBuildTag(value)
		if err != nil {
			return BuildTags{}, err
		}
		if index > 0 && tags[index-1].value >= tag.value {
			return BuildTags{}, manifestError(errors.New(buildTagsOrderingDiagnostic))
		}
		tags[index] = tag
	}
	return NewBuildTags(tags)
}

func linkerAssignmentsFromWire(wire []linkerAssignmentWire) (LinkerAssignments, error) {
	assignments := make([]LinkerAssignment, len(wire))
	for index, value := range wire {
		assignment, err := NewLinkerAssignment(value.Symbol, value.Value)
		if err != nil {
			return LinkerAssignments{}, err
		}
		if index > 0 && assignments[index-1].symbol >= assignment.symbol {
			return LinkerAssignments{}, manifestError(errors.New(linkerAssignmentsOrderingDiagnostic))
		}
		assignments[index] = assignment
	}
	return NewLinkerAssignments(assignments)
}

func parseBuildModuleMode(value string) (BuildModuleMode, error) {
	for mode := BuildModuleUnknown + 1; mode < buildModuleLimit; mode++ {
		if mode.String() == value {
			return mode, nil
		}
	}
	return BuildModuleUnknown, manifestError(errors.New("build module mode is unsupported"))
}

var (
	_ core.OffWireEnum = MetadataKindUnknown
	_ core.OffWireEnum = GoToolchainUnknown
)
