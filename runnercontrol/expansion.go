package runnercontrol

import (
	"bytes"
	"context"
	"crypto"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/runprotocol"
)

const (
	GoBuildTagMaximum                    = 64
	GoBuildContextMaximum                = 32
	ExpansionChildMaximum                = 256
	ExpansionManifestMaximumBytes        = 1 << 20
	ExpansionApprovalMaximumBytes        = 1 << 20
	GoASTDiscoveryIdentifier             = "primitive-go-ast"
	GoASTDiscoveryVersion         uint32 = 1
)

type GoBuildTag struct{ value string }

func NewGoBuildTag(value string) (GoBuildTag, error) {
	tag := GoBuildTag{value: value}
	if err := tag.Validate(); err != nil {
		return GoBuildTag{}, err
	}
	return tag, nil
}

func (t GoBuildTag) Validate() error {
	if t.value == "" || len(t.value) > runprotocol.IdentifierMaximumBytes {
		return core.ErrPrimitiveContract
	}
	if strings.ContainsAny(t.value, " \t\r\n,\"'`") || !validGoBuildTagRunes(t.value) {
		return core.ErrPrimitiveContract
	}
	first, _ := utf8.DecodeRuneInString(t.value)
	if !unicode.IsLetter(first) && first != '_' {
		return core.ErrPrimitiveContract
	}
	return nil
}

func validGoBuildTagRunes(text string) bool {
	for _, value := range text {
		if !unicode.IsLetter(value) && !unicode.IsDigit(value) && value != '_' && value != '.' {
			return false
		}
	}
	return true
}

func (t GoBuildTag) String() string {
	if t.Validate() != nil {
		return ""
	}
	return t.value
}

func (t GoBuildTag) MarshalJSON() ([]byte, error) {
	if err := t.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(t.value)
}

func (t *GoBuildTag) UnmarshalJSON(data []byte) error {
	if t == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	candidate, err := NewGoBuildTag(value)
	if err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*t = candidate
	return nil
}

type GoInstrumentation uint8

const (
	GoInstrumentationUnknown GoInstrumentation = iota
	GoInstrumentationOrdinary
	GoInstrumentationRace
	GoInstrumentationDiagnostic
	goInstrumentationLimit
)

func (i GoInstrumentation) Validate() error {
	if i <= GoInstrumentationUnknown || i >= goInstrumentationLimit {
		return core.ErrPrimitiveContract
	}
	return nil
}
func (i GoInstrumentation) IsValid() bool { return i.Validate() == nil }
func (i GoInstrumentation) String() string {
	if !i.IsValid() {
		return invalidEnumString()
	}
	return []string{"", "ordinary", runprotocol.GoRaceText, diagnosticArtifactText}[i]
}
func (i GoInstrumentation) MarshalJSON() ([]byte, error) {
	if err := i.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(i.String())
}
func (i *GoInstrumentation) UnmarshalJSON(data []byte) error {
	if i == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	for candidate := GoInstrumentationUnknown + 1; candidate < goInstrumentationLimit; candidate++ {
		if candidate.String() == value {
			*i = candidate
			return nil
		}
	}
	return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
}

type GoModuleMode uint8

const (
	GoModuleModeUnknown GoModuleMode = iota
	GoModuleModeModule
	GoModuleModeWorkspace
	goModuleModeLimit
)

func (m GoModuleMode) Validate() error {
	if m <= GoModuleModeUnknown || m >= goModuleModeLimit {
		return core.ErrPrimitiveContract
	}
	return nil
}
func (m GoModuleMode) IsValid() bool { return m.Validate() == nil }
func (m GoModuleMode) String() string {
	if m == GoModuleModeModule {
		return sourceModuleScopeText
	}
	if m == GoModuleModeWorkspace {
		return sourceWorkspaceScopeText
	}
	return invalidEnumString()
}
func (m GoModuleMode) MarshalJSON() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(m.String())
}
func (m *GoModuleMode) UnmarshalJSON(data []byte) error {
	if m == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	if value == sourceModuleScopeText {
		*m = GoModuleModeModule
		return nil
	}
	if value == sourceWorkspaceScopeText {
		*m = GoModuleModeWorkspace
		return nil
	}
	return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
}

type GoBuildContext struct {
	ArchitectureFeature *runprotocol.Identifier `json:"architecture_feature,omitempty"`
	Toolchain           runprotocol.Identifier  `json:"toolchain"`
	ModuleRoot          runprotocol.SourcePath  `json:"module_root"`
	ReleaseTags         []GoBuildTag            `json:"release_tags"`
	BuildTags           []GoBuildTag            `json:"build_tags"`
	GOExperiment        []GoBuildTag            `json:"goexperiment"`
	OtherInputs         core.SHA256Digest       `json:"other_inputs_digest"`
	GOOS                core.OperatingSystem    `json:"goos"`
	GOARCH              core.CPUArchitecture    `json:"goarch"`
	CGOEnabled          bool                    `json:"cgo_enabled"`
	Instrumentation     GoInstrumentation       `json:"instrumentation"`
	ModuleMode          GoModuleMode            `json:"module_mode"`
}

func (c GoBuildContext) Validate() error {
	if err := errors.Join(c.Toolchain.Validate(), c.GOOS.Validate(), c.GOARCH.Validate(), c.Instrumentation.Validate(), c.ModuleMode.Validate(), c.ModuleRoot.Validate(), c.OtherInputs.Validate()); err != nil {
		return err
	}
	if len(c.ReleaseTags) > GoBuildTagMaximum || len(c.BuildTags) > GoBuildTagMaximum || len(c.GOExperiment) > GoBuildTagMaximum {
		return core.ErrPrimitiveContract
	}
	if err := validateCanonicalGoBuildTags(c.ReleaseTags); err != nil {
		return err
	}
	if err := validateCanonicalGoBuildTags(c.BuildTags); err != nil {
		return err
	}
	if err := validateCanonicalGoBuildTags(c.GOExperiment); err != nil {
		return err
	}
	if c.ArchitectureFeature != nil {
		return c.ArchitectureFeature.Validate()
	}
	return nil
}

func validateCanonicalGoBuildTags(values []GoBuildTag) error {
	previous := ""
	for index := range values {
		if err := values[index].Validate(); err != nil {
			return err
		}
		value := values[index].String()
		if index > 0 && previous >= value {
			return core.ErrPrimitiveContract
		}
		previous = value
	}
	return nil
}
func validateCanonicalIdentifiers(values []runprotocol.Identifier) error {
	var previous []byte
	for index := range values {
		if err := values[index].Validate(); err != nil {
			return err
		}
		encoded, err := values[index].MarshalJSON()
		if err != nil {
			return err
		}
		if index > 0 && bytes.Compare(previous, encoded) >= 0 {
			return core.ErrPrimitiveContract
		}
		previous = encoded
	}
	return nil
}
func (c GoBuildContext) Digest() (core.SHA256Digest, error) {
	if err := c.Validate(); err != nil {
		return core.SHA256Digest{}, err
	}
	encoded, err := core.MarshalCanonicalJSONDocument(c)
	if err != nil {
		return core.SHA256Digest{}, err
	}
	return core.SHA256Of(encoded), nil
}

type GoBuildContextEntry struct {
	Profile runprotocol.ProfileIdentity `json:"profile"`
	Context GoBuildContext              `json:"context"`
	Digest  core.SHA256Digest           `json:"digest"`
	Kind    runprotocol.ProbeKind       `json:"kind"`
}

func (e GoBuildContextEntry) Validate() error {
	if err := errors.Join(e.Kind.Validate(), e.Profile.Validate(), e.Context.Validate(), e.Digest.Validate()); err != nil {
		return err
	}
	role, err := e.Kind.Role()
	if err != nil || role != runprotocol.ProbeRoleExperiment {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	digest, err := e.Context.Digest()
	if err != nil || digest != e.Digest {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	return nil
}

type GoBuildContextSet struct {
	Entries []GoBuildContextEntry `json:"entries"`
}

func (s GoBuildContextSet) Validate() error {
	if len(s.Entries) == 0 || len(s.Entries) > GoBuildContextMaximum {
		return core.ErrPrimitiveContract
	}
	var previous []byte
	for index := range s.Entries {
		if err := s.Entries[index].Validate(); err != nil {
			return err
		}
		key, err := core.MarshalCanonicalJSONDocument(struct {
			Profile runprotocol.ProfileIdentity `json:"profile"`
			Kind    runprotocol.ProbeKind       `json:"kind"`
		}{Profile: s.Entries[index].Profile, Kind: s.Entries[index].Kind})
		if err != nil {
			return err
		}
		if index > 0 && bytes.Compare(previous, key) >= 0 {
			return core.ErrPrimitiveContract
		}
		previous = key
	}
	return nil
}
func (s GoBuildContextSet) Digest() (core.SHA256Digest, error) {
	if err := s.Validate(); err != nil {
		return core.SHA256Digest{}, err
	}
	encoded, err := core.MarshalCanonicalJSONDocument(s)
	if err != nil {
		return core.SHA256Digest{}, err
	}
	return core.SHA256Of(encoded), nil
}

func (s GoBuildContextSet) Find(kind runprotocol.ProbeKind, profile runprotocol.ProfileIdentity) (GoBuildContextEntry, bool) {
	if s.Validate() != nil || kind.Validate() != nil || profile.Validate() != nil {
		return GoBuildContextEntry{}, false
	}
	for _, entry := range s.Entries {
		if entry.Kind == kind && entry.Profile == profile {
			return entry, true
		}
	}
	return GoBuildContextEntry{}, false
}

type ExpansionDisposition uint8

const (
	ExpansionDispositionUnknown ExpansionDisposition = iota
	ExpansionAdmitted
	ExpansionRefused
	ExpansionNotApplicable
	expansionDispositionLimit
)

func (d ExpansionDisposition) Validate() error {
	if d <= ExpansionDispositionUnknown || d >= expansionDispositionLimit {
		return core.ErrPrimitiveContract
	}
	return nil
}
func (d ExpansionDisposition) IsValid() bool { return d.Validate() == nil }
func (d ExpansionDisposition) String() string {
	if !d.IsValid() {
		return invalidEnumString()
	}
	return []string{"", "expansion_admitted", "expansion_refused", evidenceNotApplicableText}[d]
}
func (d ExpansionDisposition) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(d.String())
}
func (d *ExpansionDisposition) UnmarshalJSON(data []byte) error {
	if d == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	for candidate := ExpansionDispositionUnknown + 1; candidate < expansionDispositionLimit; candidate++ {
		if candidate.String() == value {
			*d = candidate
			return nil
		}
	}
	return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
}

type ExpansionChild struct {
	Experiment         *runprotocol.ExperimentID  `json:"experiment_id,omitempty"`
	Refusal            *runprotocol.RefusalReason `json:"refusal,omitempty"`
	Probe              runprotocol.ProbeIdentity  `json:"probe"`
	Sequence           uint16                     `json:"sequence"`
	BuildContextDigest core.SHA256Digest          `json:"build_context_digest"`
	Disposition        ExpansionDisposition       `json:"disposition"`
}

// CIExpansionChild is one compiler-owned child intent from product policy.
// The caller derives only the run-bound parent and experiment identity from it.
type CIExpansionChild struct {
	Refusal            *runprotocol.RefusalReason `json:"refusal,omitempty"`
	Target             runprotocol.ProbeTarget    `json:"target"`
	Sequence           uint16                     `json:"sequence"`
	BuildContextDigest core.SHA256Digest          `json:"build_context_digest"`
	Kind               runprotocol.ProbeKind      `json:"kind"`
	Disposition        ExpansionDisposition       `json:"disposition"`
}

func (c CIExpansionChild) Validate() error {
	if c.Sequence == 0 {
		return errors.Join(core.ErrPrimitiveContract, errors.New("ci expansion child sequence is zero"))
	}
	if err := errors.Join(c.Target.Validate(), c.Kind.Validate(), c.BuildContextDigest.Validate(), c.Disposition.Validate()); err != nil {
		return err
	}
	role, err := c.Kind.Role()
	if err != nil || role != runprotocol.ProbeRoleExperiment {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	switch c.Disposition {
	case ExpansionAdmitted, ExpansionNotApplicable:
		if c.Refusal != nil {
			return core.ErrPrimitiveContract
		}
		return nil
	case ExpansionRefused:
		if c.Refusal == nil {
			return core.ErrPrimitiveContract
		}
		return c.Refusal.Validate()
	default:
		return core.ErrPrimitiveContract
	}
}

type CIExpansionPlan struct {
	Identity         runprotocol.Identifier  `json:"identity"`
	Discovery        runprotocol.Identifier  `json:"discovery"`
	RequestedKinds   []runprotocol.ProbeKind `json:"requested_kinds"`
	Children         []CIExpansionChild      `json:"children"`
	DiscoveryVersion uint32                  `json:"discovery_version"`
	SchemaVersion    uint16                  `json:"schema_version"`
}

func (p CIExpansionPlan) Validate() error {
	if p.SchemaVersion != SchemaVersion || p.DiscoveryVersion == 0 || len(p.RequestedKinds) == 0 {
		return core.ErrPrimitiveContract
	}
	if len(p.RequestedKinds) > runprotocol.ProbeKindMaximum || len(p.Children) > ExpansionChildMaximum {
		return core.ErrPrimitiveContract
	}
	if err := errors.Join(p.Identity.Validate(), p.Discovery.Validate()); err != nil {
		return err
	}
	if err := validateCIRequestedKinds(p.RequestedKinds); err != nil {
		return err
	}
	return p.validateChildren()
}

func validateCIRequestedKinds(kinds []runprotocol.ProbeKind) error {
	for index := range kinds {
		if err := kinds[index].Validate(); err != nil {
			return err
		}
		role, err := kinds[index].Role()
		if err != nil || role != runprotocol.ProbeRoleExperiment {
			return errors.Join(core.ErrPrimitiveContract, err)
		}
		if index > 0 && kinds[index-1] >= kinds[index] {
			return core.ErrPrimitiveContract
		}
	}
	return nil
}

func (p CIExpansionPlan) validateChildren() error {
	for index := range p.Children {
		child := p.Children[index]
		if err := child.Validate(); err != nil {
			return err
		}
		sequence, err := core.CheckedUint16FromInt(index + 1)
		if err != nil || child.Sequence != sequence {
			return errors.Join(core.ErrPrimitiveContract, errors.New("ci expansion children are not in declared sequence"))
		}
		if !requestedKindContains(p.RequestedKinds, child.Kind) {
			return core.ErrPrimitiveContract
		}
	}
	return nil
}

func (c ExpansionChild) Validate() error {
	if c.Sequence == 0 {
		return errors.Join(core.ErrPrimitiveContract, errors.New("expansion child sequence is zero"))
	}
	if err := errors.Join(c.Probe.Validate(), c.BuildContextDigest.Validate(), c.Disposition.Validate()); err != nil {
		return err
	}
	if c.Probe.Role != runprotocol.ProbeRoleExperiment || c.Probe.Parent == nil {
		return core.ErrPrimitiveContract
	}
	return c.validateDisposition()
}

func (c ExpansionChild) validateDisposition() error {
	switch c.Disposition {
	case ExpansionAdmitted:
		if c.Experiment == nil || c.Refusal != nil {
			return core.ErrPrimitiveContract
		}
		return c.Experiment.Validate()
	case ExpansionRefused:
		if c.Experiment != nil || c.Refusal == nil {
			return core.ErrPrimitiveContract
		}
		return c.Refusal.Validate()
	case ExpansionNotApplicable:
		if c.Experiment != nil || c.Refusal != nil {
			return core.ErrPrimitiveContract
		}
		return nil
	default:
		return core.ErrPrimitiveContract
	}
}

type ExpansionManifest struct {
	Discovery        runprotocol.Identifier       `json:"discovery"`
	Children         []ExpansionChild             `json:"children"`
	Members          MemberSet                    `json:"member_set"`
	Contexts         GoBuildContextSet            `json:"contexts"`
	RequestedKinds   []runprotocol.ProbeKind      `json:"requested_kinds"`
	Source           runprotocol.SourceCoordinate `json:"source"`
	Parent           runprotocol.ProbeIdentity    `json:"parent"`
	Fence            SchedulingFence              `json:"fence"`
	DiscoveryVersion uint32                       `json:"discovery_version"`
	SchemaVersion    uint16                       `json:"schema_version"`
	Admitted         uint16                       `json:"admitted"`
	Refused          uint16                       `json:"refused"`
	NotApplicable    uint16                       `json:"not_applicable"`
	Identity         core.SHA256Digest            `json:"identity"`
	Request          runprotocol.RequestIdentity  `json:"request_id"`
	Run              runprotocol.RunID            `json:"run_id"`
}

func (m ExpansionManifest) Validate() error {
	if err := m.validateHeader(); err != nil {
		return err
	}
	if err := m.validateRequestedKinds(); err != nil {
		return err
	}
	if err := m.validateChildren(); err != nil {
		return err
	}
	want, err := m.identityDigest()
	if err != nil || want != m.Identity {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	return nil
}

func (m ExpansionManifest) validateHeader() error {
	if m.SchemaVersion != SchemaVersion || m.DiscoveryVersion == 0 {
		return core.ErrPrimitiveContract
	}
	if len(m.Children) > ExpansionChildMaximum {
		return core.ErrPrimitiveContract
	}
	if err := m.validateOwnedValues(); err != nil {
		return err
	}
	if err := m.validateBindings(); err != nil {
		return err
	}
	if err := m.validateSelectionParent(); err != nil {
		return err
	}
	if len(m.RequestedKinds) == 0 || len(m.RequestedKinds) > runprotocol.ProbeKindMaximum {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (m ExpansionManifest) validateBindings() error {
	memberDigest, err := m.Members.Digest()
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	if memberDigest != m.Fence.MemberSetDigest || !m.Members.Contains(m.Run) {
		return core.ErrPrimitiveContract
	}
	if m.Parent.Environment.MachineGeneration != m.Fence.Machine.Generation {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (m ExpansionManifest) validateSelectionParent() error {
	if m.Parent.Role != runprotocol.ProbeRoleSelection || m.Parent.Source != m.Source {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (m ExpansionManifest) validateOwnedValues() error {
	return errors.Join(m.Identity.Validate(), m.Request.Validate(), m.Run.Validate(), m.Fence.Validate(), m.Members.Validate(), m.Parent.Validate(), m.Source.Validate(), m.Discovery.Validate(), m.Contexts.Validate())
}

func (m ExpansionManifest) validateRequestedKinds() error {
	for index := range m.RequestedKinds {
		if err := m.RequestedKinds[index].Validate(); err != nil {
			return err
		}
		if index > 0 && m.RequestedKinds[index-1] >= m.RequestedKinds[index] {
			return core.ErrPrimitiveContract
		}
	}
	return nil
}

func (m ExpansionManifest) validateChildren() error {
	var counts expansionCounts
	for index := range m.Children {
		child := m.Children[index]
		if err := validateExpansionChildAt(child, m, index); err != nil {
			return err
		}
		if err := counts.increment(child.Disposition); err != nil {
			return err
		}
	}
	if counts.admitted != m.Admitted || counts.refused != m.Refused || counts.notApplicable != m.NotApplicable {
		return core.ErrPrimitiveContract
	}
	return nil
}

func validateExpansionChildAt(child ExpansionChild, manifest ExpansionManifest, index int) error {
	if err := child.Validate(); err != nil {
		return err
	}
	if !childMatchesExpansion(child, manifest) {
		return core.ErrPrimitiveContract
	}
	sequence, err := core.CheckedUint16FromInt(index + 1)
	if err != nil || child.Sequence != sequence {
		return errors.Join(core.ErrPrimitiveContract, errors.New("expansion children are not in execution sequence"))
	}
	return nil
}

type expansionCounts struct {
	admitted      uint16
	refused       uint16
	notApplicable uint16
}

func (c *expansionCounts) increment(disposition ExpansionDisposition) error {
	if disposition == ExpansionAdmitted {
		c.admitted++
		return nil
	}
	if disposition == ExpansionRefused {
		c.refused++
		return nil
	}
	if disposition == ExpansionNotApplicable {
		c.notApplicable++
		return nil
	}
	return core.ErrPrimitiveContract
}
func childMatchesExpansion(child ExpansionChild, m ExpansionManifest) bool {
	p := child.Probe
	if !childMatchesExpansionScalars(p, m) || !childMatchesExpansionTarget(p, m) {
		return false
	}
	return requestedKindContains(m.RequestedKinds, p.Kind)
}

func childMatchesExpansionScalars(p runprotocol.ProbeIdentity, m ExpansionManifest) bool {
	return p.Origin == m.Parent.Origin && p.Subject == m.Parent.Subject && p.Source == m.Source && p.Profile == m.Parent.Profile && p.Environment == m.Parent.Environment && p.Parent.Request == m.Request && p.Parent.Kind == m.Parent.Kind && p.Parent.ExpansionDigest == m.Identity
}

func childMatchesExpansionTarget(p runprotocol.ProbeIdentity, m ExpansionManifest) bool {
	left, leftErr := core.MarshalCanonicalJSONDocument(p.Parent.Target)
	right, rightErr := core.MarshalCanonicalJSONDocument(m.Parent.Target)
	return leftErr == nil && rightErr == nil && bytes.Equal(left, right)
}

func requestedKindContains(kinds []runprotocol.ProbeKind, candidate runprotocol.ProbeKind) bool {
	return slices.Contains(kinds, candidate)
}
func (m ExpansionManifest) identityDigest() (core.SHA256Digest, error) {
	type identityProjection struct {
		Discovery        runprotocol.Identifier       `json:"discovery"`
		Members          MemberSet                    `json:"member_set"`
		RequestedKinds   []runprotocol.ProbeKind      `json:"requested_kinds"`
		Contexts         GoBuildContextSet            `json:"contexts"`
		Source           runprotocol.SourceCoordinate `json:"source"`
		Parent           runprotocol.ProbeIdentity    `json:"parent"`
		Fence            SchedulingFence              `json:"fence"`
		DiscoveryVersion uint32                       `json:"discovery_version"`
		Request          runprotocol.RequestIdentity  `json:"request_id"`
		Run              runprotocol.RunID            `json:"run_id"`
	}
	parent := m.Parent
	encoded, err := core.MarshalCanonicalJSONDocument(identityProjection{
		Parent:           parent,
		Discovery:        m.Discovery,
		Members:          m.Members,
		RequestedKinds:   m.RequestedKinds,
		Contexts:         m.Contexts,
		Source:           m.Source,
		Fence:            m.Fence,
		DiscoveryVersion: m.DiscoveryVersion,
		Request:          m.Request,
		Run:              m.Run,
	})
	if err != nil {
		return core.SHA256Digest{}, err
	}
	return core.SHA256Of(encoded), nil
}

func CalculateExpansionIdentity(m ExpansionManifest) (core.SHA256Digest, error) {
	if m.SchemaVersion != SchemaVersion || m.DiscoveryVersion == 0 {
		return core.SHA256Digest{}, core.ErrPrimitiveContract
	}
	if err := errors.Join(m.Request.Validate(), m.Run.Validate(), m.Fence.Validate(), m.Members.Validate(), m.Parent.Validate(), m.Source.Validate(), m.Discovery.Validate(), m.Contexts.Validate()); err != nil {
		return core.SHA256Digest{}, err
	}
	return m.identityDigest()
}
func (m ExpansionManifest) Digest() (core.SHA256Digest, error) {
	encoded, err := m.MarshalJSON()
	if err != nil {
		return core.SHA256Digest{}, err
	}
	return core.SHA256Of(encoded), nil
}

type expansionManifestWire ExpansionManifest

func (m ExpansionManifest) MarshalJSON() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(expansionManifestWire(m))
	if err != nil || len(encoded) > ExpansionManifestMaximumBytes {
		return nil, errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract, err)
	}
	return encoded, nil
}
func (m *ExpansionManifest) UnmarshalJSON(data []byte) error {
	if m == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[expansionManifestWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := ExpansionManifest(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*m = candidate
	return nil
}

func (ExpansionManifest) AttestationDomain() CompletionSigningDomain {
	return CompletionSigningDomainExpansionV1
}
func (m ExpansionManifest) WriteCanonical(destination io.Writer) error {
	if destination == nil {
		return core.ErrPrimitiveContract
	}
	encoded, err := m.MarshalJSON()
	if err != nil {
		return err
	}
	written, err := destination.Write(encoded)
	if err != nil || written != len(encoded) {
		return errors.Join(core.ErrPrimitiveContract, err, io.ErrShortWrite)
	}
	return nil
}

type ExpansionDocument struct {
	Manifest    ExpansionManifest                        `json:"manifest"`
	Attestation attest.Envelope[CompletionSigningDomain] `json:"attestation"`
}

func (d ExpansionDocument) Validate() error {
	if err := errors.Join(d.Manifest.Validate(), d.Attestation.Validate()); err != nil {
		return err
	}
	if d.Attestation.Domain != d.Manifest.AttestationDomain() {
		return core.ErrPrimitiveContract
	}
	return nil
}
func IssueExpansion(manifest ExpansionManifest, signer crypto.Signer) (ExpansionDocument, error) {
	envelope, err := attest.Sign(attest.SignRequest[CompletionSigningDomain]{Body: manifest, Signer: signer})
	if err != nil {
		return ExpansionDocument{}, err
	}
	document := ExpansionDocument{Manifest: manifest, Attestation: envelope}
	return document, document.Validate()
}
func VerifyExpansion(document ExpansionDocument, trusted attest.TrustedKeys) error {
	if err := document.Validate(); err != nil {
		return err
	}
	proof, err := attest.Verify(attest.VerifyRequest[CompletionSigningDomain]{Body: document.Manifest, Envelope: document.Attestation, TrustedKeys: trusted})
	if err != nil {
		return err
	}
	return proof.Validate()
}

type expansionDocumentWire ExpansionDocument

func (d ExpansionDocument) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONDocument(expansionDocumentWire(d))
}
func (d *ExpansionDocument) UnmarshalJSON(data []byte) error {
	if d == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[expansionDocumentWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := ExpansionDocument(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*d = candidate
	return nil
}
func (d ExpansionDocument) IdempotencyKey() (exchange.IdempotencyKey, error) {
	encoded, err := d.MarshalJSON()
	if err != nil {
		return exchange.IdempotencyKey{}, err
	}
	hex, err := core.SHA256Of(encoded).Hex()
	if err != nil {
		return exchange.IdempotencyKey{}, err
	}
	return exchange.ParseIdempotencyKey("expansion:" + hex)
}

type ExpansionRecord struct {
	Canonical []byte
	Document  ExpansionDocument
	Bytes     core.ByteLength
	Digest    core.SHA256Digest
}

func NewExpansionRecord(document ExpansionDocument) (ExpansionRecord, error) {
	canonical, err := document.MarshalJSON()
	if err != nil {
		return ExpansionRecord{}, err
	}
	extent, err := core.NewByteLength(uint64(len(canonical)))
	if err != nil {
		return ExpansionRecord{}, err
	}
	record := ExpansionRecord{Document: document, Canonical: canonical, Digest: core.SHA256Of(canonical), Bytes: extent}
	return record, record.Validate()
}
func (r ExpansionRecord) Validate() error {
	if err := errors.Join(r.Document.Validate(), r.Digest.Validate(), r.Bytes.Validate()); err != nil {
		return err
	}
	encoded, err := r.Document.MarshalJSON()
	if err != nil || !bytes.Equal(encoded, r.Canonical) || r.Digest != core.SHA256Of(r.Canonical) || r.Bytes.Uint64() != uint64(len(r.Canonical)) {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	return nil
}

type ExpansionApproval struct {
	Refusal        *runprotocol.RefusalReason     `json:"refusal,omitempty"`
	Experiments    []ExperimentCapabilityDocument `json:"experiment_capabilities"`
	SchemaVersion  uint16                         `json:"schema_version"`
	ManifestDigest core.SHA256Digest              `json:"manifest_digest"`
	Run            runprotocol.RunID              `json:"run_id"`
	Approved       bool                           `json:"approved"`
}

func (a ExpansionApproval) Validate() error {
	if a.SchemaVersion != SchemaVersion {
		return core.ErrPrimitiveContract
	}
	if err := errors.Join(a.Run.Validate(), a.ManifestDigest.Validate()); err != nil {
		return err
	}
	if a.Approved {
		return a.validateApproved()
	}
	if a.Refusal == nil || len(a.Experiments) != 0 {
		return core.ErrPrimitiveContract
	}
	return a.Refusal.Validate()
}

func (a ExpansionApproval) validateApproved() error {
	if a.Refusal != nil || len(a.Experiments) == 0 || len(a.Experiments) > ExpansionChildMaximum {
		return core.ErrPrimitiveContract
	}
	for index := range a.Experiments {
		if err := validateApprovedExperiment(a.Experiments[index], a); err != nil {
			return err
		}
	}
	return nil
}

func validateApprovedExperiment(document ExperimentCapabilityDocument, approval ExpansionApproval) error {
	if err := document.Validate(); err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	experiment := document.Payload
	if err := experiment.Validate(); err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	if experiment.Run != approval.Run || experiment.ExpansionManifestDigest == nil || *experiment.ExpansionManifestDigest != approval.ManifestDigest {
		return core.ErrPrimitiveContract
	}
	return nil
}

func VerifyExpansionApproval(approval ExpansionApproval, trusted attest.TrustedKeys) error {
	if err := approval.Validate(); err != nil {
		return err
	}
	for index := range approval.Experiments {
		if err := VerifyExperimentCapability(approval.Experiments[index], trusted); err != nil {
			return err
		}
	}
	return nil
}

type expansionApprovalWire ExpansionApproval

func (a ExpansionApproval) MarshalJSON() ([]byte, error) {
	if err := a.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONDocument(expansionApprovalWire(a))
}
func (a *ExpansionApproval) UnmarshalJSON(data []byte) error {
	if a == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	wire, err := core.DecodeStrictJSONStructure[expansionApprovalWire](data, core.DefaultStrictJSONLimits())
	if err != nil {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	candidate := ExpansionApproval(wire)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*a = candidate
	return nil
}

type ExpansionRepository interface {
	ApproveExpansion(context.Context, ExpansionRecord) (ExpansionApproval, error)
}
type ExpansionClient struct{ socket exchange.ClientSocket }

func NewExpansionClient(configuration exchange.ClientSocketConfiguration) (ExpansionClient, error) {
	socket, err := exchange.NewClientSocket(configuration)
	if err != nil {
		return ExpansionClient{}, err
	}
	return ExpansionClient{socket: socket}, nil
}
func (c ExpansionClient) Submit(ctx context.Context, document ExpansionDocument) (exchange.JSONResponse[ExpansionApproval], error) {
	return exchange.SendReplayBoundSocketJSON[ExpansionDocument, ExpansionApproval](ctx, c.socket, document)
}

type ExpansionServer struct {
	repository ExpansionRepository
	socket     exchange.ServerSocket
	trusted    attest.TrustedKeys
}

func NewExpansionServer(contract exchange.JSONSocketContract, repository ExpansionRepository, trusted attest.TrustedKeys) (ExpansionServer, error) {
	if repository == nil || trusted.Validate() != nil {
		return ExpansionServer{}, core.ErrPrimitiveContract
	}
	socket, err := exchange.NewServerSocket(contract)
	if err != nil {
		return ExpansionServer{}, err
	}
	return ExpansionServer{socket: socket, repository: repository, trusted: trusted}, nil
}
func (s ExpansionServer) Serve(call exchange.SocketServerCall) error {
	if s.repository == nil {
		return core.ErrPrimitiveContract
	}
	ctx, err := call.Context()
	if err != nil {
		return err
	}
	received, err := exchange.ReceiveReplayBoundSocketJSON[ExpansionDocument, *ExpansionDocument](s.socket, call)
	if err != nil {
		return err
	}
	manifest := received.Body.Manifest
	machine := manifest.Fence.Machine
	if err := RequireRunnerPeer(ctx, machine.Machine, machine.Generation); err != nil {
		return err
	}
	if err := VerifyExpansion(*received.Body, s.trusted); err != nil {
		return fmt.Errorf("verify expansion for run %v manifest %v: %w", received.Body.Manifest.Run, received.Body.Manifest.Identity, err)
	}
	record, err := NewExpansionRecord(*received.Body)
	if err != nil {
		return err
	}
	approval, err := s.repository.ApproveExpansion(ctx, record)
	if err != nil {
		return err
	}
	if err := validateExpansionApproval(record, approval); err != nil {
		return err
	}
	return exchange.WriteSocketJSON(s.socket, call, approval)
}

func validateExpansionApproval(record ExpansionRecord, approval ExpansionApproval) error {
	manifestDigest, digestErr := record.Document.Manifest.Digest()
	if digestErr != nil || approval.Run != record.Document.Manifest.Run || approval.ManifestDigest != manifestDigest {
		return errors.Join(core.ErrPrimitiveContract, digestErr)
	}
	return nil
}
func ExpansionSocketContract(path exchange.SocketRoutePath) (exchange.JSONSocketContract, error) {
	requestLimit, requestErr := core.NewByteCount(ExpansionManifestMaximumBytes)
	responseLimit, responseErr := core.NewByteCount(ExpansionApprovalMaximumBytes)
	if err := errors.Join(path.Validate(), requestErr, responseErr); err != nil {
		return exchange.JSONSocketContract{}, err
	}
	contract := exchange.JSONSocketContract{Path: path, Route: exchange.RouteSemantics{Method: exchange.MethodPost, Replay: exchange.ReplayIdempotencyKey}, RequestBodyLimit: requestLimit, ResponseBodyLimit: responseLimit, SuccessStatus: core.HTTPStatusOK()}
	return contract, contract.Validate()
}

var (
	_ core.Validatable            = GoInstrumentationUnknown
	_ core.Validatable            = GoBuildTag{}
	_ json.Unmarshaler            = (*GoBuildTag)(nil)
	_ json.Unmarshaler            = (*GoInstrumentation)(nil)
	_ core.Validatable            = GoModuleModeUnknown
	_ json.Unmarshaler            = (*GoModuleMode)(nil)
	_ core.Validatable            = GoBuildContext{}
	_ core.Validatable            = GoBuildContextEntry{}
	_ core.Validatable            = GoBuildContextSet{}
	_ core.Validatable            = ExpansionDispositionUnknown
	_ json.Unmarshaler            = (*ExpansionDisposition)(nil)
	_ core.Validatable            = ExpansionChild{}
	_ core.ValidatedJSONMarshaler = ExpansionManifest{}
	_ json.Unmarshaler            = (*ExpansionManifest)(nil)
	_ core.ValidatedJSONMarshaler = ExpansionDocument{}
	_ json.Unmarshaler            = (*ExpansionDocument)(nil)
	_ exchange.IdempotencyBound   = ExpansionDocument{}
	_ core.Validatable            = ExpansionRecord{}
	_ core.ValidatedJSONMarshaler = ExpansionApproval{}
	_ json.Unmarshaler            = (*ExpansionApproval)(nil)
)
