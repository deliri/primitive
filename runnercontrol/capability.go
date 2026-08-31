package runnercontrol

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/projectstandards"
	"github.com/deliri/primitive/v2026/temporal"
)

const SchedulingMemberCapabilityMaximum = SchedulingMemberMaximum

type SchedulingCapability struct {
	SchemaVersion    uint16                                `json:"schema_version"`
	Observation      projectstandards.MachineObservationID `json:"machine_observation_id"`
	Fence            SchedulingFence                       `json:"fence"`
	Members          MemberSet                             `json:"members"`
	Source           projectstandards.SourceCoordinate     `json:"source"`
	SourceGrant      SourceGrantIdentity                   `json:"source_grant"`
	RepositoryGrant  core.SHA256Digest                     `json:"repository_grant"`
	DeliveryGrant    core.SHA256Digest                     `json:"delivery_grant"`
	IsolationPolicy  core.SHA256Digest                     `json:"isolation_policy"`
	AggregateBudget  temporal.Duration                     `json:"aggregate_budget"`
	AbsoluteDeadline temporal.Instant                      `json:"absolute_deadline"`
	ExpiresAt        temporal.Instant                      `json:"expires_at"`
}

func (c SchedulingCapability) Validate() error {
	if c.SchemaVersion != SchemaVersion {
		return core.ErrPrimitiveContract
	}
	if err := errors.Join(c.Observation.Validate(), c.Fence.Validate(), c.Members.Validate(), c.Source.Validate(), c.SourceGrant.Validate(), c.RepositoryGrant.Validate(), c.DeliveryGrant.Validate(), c.IsolationPolicy.Validate(), c.AggregateBudget.Validate(), c.AbsoluteDeadline.Validate(), c.ExpiresAt.Validate()); err != nil {
		return err
	}
	memberDigest, err := c.Members.Digest()
	if err != nil || memberDigest != c.Fence.MemberSetDigest || c.AggregateBudget.IsZero() {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	expiryDeadline, err := c.ExpiresAt.Compare(c.AbsoluteDeadline)
	expiryFence, fenceErr := c.ExpiresAt.Compare(c.Fence.Machine.ExpiresAt)
	if err != nil || fenceErr != nil || expiryDeadline == core.ComparisonGreater || expiryFence == core.ComparisonGreater {
		return errors.Join(core.ErrPrimitiveContract, err, fenceErr)
	}
	return nil
}

func (c SchedulingCapability) Digest() (core.SHA256Digest, error) {
	if err := c.Validate(); err != nil {
		return core.SHA256Digest{}, err
	}
	encoded, err := core.MarshalCanonicalJSONDocument(c)
	if err != nil {
		return core.SHA256Digest{}, err
	}
	return core.SHA256Of(encoded), nil
}

type MemberCapability struct {
	SchemaVersion     uint16                           `json:"schema_version"`
	SchedulingDigest  core.SHA256Digest                `json:"scheduling_digest"`
	Fence             SchedulingFence                  `json:"fence"`
	Request           projectstandards.RequestIdentity `json:"request_id"`
	Run               projectstandards.RunID           `json:"run_id"`
	AdmittedRunDigest core.SHA256Digest                `json:"admitted_run_digest"`
	Probe             projectstandards.ProbeIdentity   `json:"probe"`
	Limits            RunLimits                        `json:"limits"`
	BuildContexts     *GoBuildContextSet               `json:"build_contexts,omitempty"`
	CIExpansion       *CIExpansionPlan                 `json:"ci_expansion,omitempty"`
	Nonce             core.SHA256Digest                `json:"nonce"`
	ExpiresAt         temporal.Instant                 `json:"expires_at"`
}

func (c MemberCapability) Validate() error {
	if c.SchemaVersion != SchemaVersion {
		return core.ErrPrimitiveContract
	}
	if err := errors.Join(c.SchedulingDigest.Validate(), c.Fence.Validate(), c.Request.Validate(), c.Run.Validate(), c.AdmittedRunDigest.Validate(), c.Probe.Validate(), c.Limits.Validate(), c.Nonce.Validate(), c.ExpiresAt.Validate()); err != nil {
		return err
	}
	if err := c.validateBuildContexts(); err != nil {
		return err
	}
	if c.Probe.Environment.MachineGeneration != c.Fence.Machine.Generation {
		return core.ErrPrimitiveContract
	}
	comparison, err := c.ExpiresAt.Compare(c.Fence.Machine.ExpiresAt)
	if err != nil || comparison == core.ComparisonGreater {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	return nil
}

func (c MemberCapability) validateBuildContexts() error {
	if c.Probe.Role != projectstandards.ProbeRoleSelection {
		if c.BuildContexts != nil || c.CIExpansion != nil {
			return core.ErrPrimitiveContract
		}
		return nil
	}
	if c.BuildContexts == nil {
		return core.ErrPrimitiveContract
	}
	if err := c.BuildContexts.Validate(); err != nil {
		return err
	}
	if c.Probe.Kind == projectstandards.ProbeKindCISelection {
		return c.validateCIExpansion()
	}
	if c.CIExpansion != nil {
		return core.ErrPrimitiveContract
	}
	return validateSelectionBuildContexts(c.Probe, *c.BuildContexts)
}

func (c MemberCapability) validateCIExpansion() error {
	if c.CIExpansion == nil || c.Probe.Target.CI == nil {
		return core.ErrPrimitiveContract
	}
	if err := c.CIExpansion.Validate(); err != nil {
		return err
	}
	if c.CIExpansion.Identity != c.Probe.Target.CI.Identity {
		return core.ErrPrimitiveContract
	}
	for _, kind := range c.CIExpansion.RequestedKinds {
		if _, ok := c.BuildContexts.Find(kind, c.Probe.Profile); !ok {
			return core.ErrPrimitiveContract
		}
	}
	return nil
}

func validateSelectionBuildContexts(probe projectstandards.ProbeIdentity, contexts GoBuildContextSet) error {
	var kinds []projectstandards.ProbeKind
	if probe.Target.GoFile != nil {
		kinds = probe.Target.GoFile.ChildKinds
	}
	if probe.Target.GoPackage != nil {
		kinds = probe.Target.GoPackage.ChildKinds
	}
	if len(kinds) == 0 {
		return nil
	}
	if len(contexts.Entries) != len(kinds) {
		return core.ErrPrimitiveContract
	}
	for _, kind := range kinds {
		if _, ok := contexts.Find(kind, probe.Profile); !ok {
			return core.ErrPrimitiveContract
		}
	}
	return nil
}

func (c MemberCapability) Digest() (core.SHA256Digest, error) {
	if err := c.Validate(); err != nil {
		return core.SHA256Digest{}, err
	}
	encoded, err := core.MarshalCanonicalJSONDocument(c)
	if err != nil {
		return core.SHA256Digest{}, err
	}
	return core.SHA256Of(encoded), nil
}

type ExperimentCapability struct {
	SchemaVersion           uint16                            `json:"schema_version"`
	MemberCapabilityDigest  core.SHA256Digest                 `json:"member_capability_digest"`
	Fence                   SchedulingFence                   `json:"fence"`
	Run                     projectstandards.RunID            `json:"run_id"`
	Experiment              projectstandards.ExperimentID     `json:"experiment_id"`
	Probe                   projectstandards.ProbeIdentity    `json:"probe"`
	Source                  projectstandards.SourceCoordinate `json:"source"`
	Execution               ExperimentExecution               `json:"execution"`
	Resources               ResourceRequirement               `json:"resources"`
	BuildContextDigest      core.SHA256Digest                 `json:"build_context_digest"`
	ExpansionManifestDigest *core.SHA256Digest                `json:"expansion_manifest_digest,omitempty"`
	ExpiresAt               temporal.Instant                  `json:"expires_at"`
}

// Digest binds the exact experiment execution contract, including its fence,
// source, process plan, resource ceiling, build context, and expiry.
func (c ExperimentCapability) Digest() (core.SHA256Digest, error) {
	if err := c.Validate(); err != nil {
		return core.SHA256Digest{}, err
	}
	encoded, err := core.MarshalCanonicalJSONDocument(c)
	if err != nil {
		return core.SHA256Digest{}, err
	}
	return core.SHA256Of(encoded), nil
}

func (c ExperimentCapability) Validate() error {
	if c.SchemaVersion != SchemaVersion {
		return core.ErrPrimitiveContract
	}
	if err := errors.Join(c.MemberCapabilityDigest.Validate(), c.Fence.Validate(), c.Run.Validate(), c.Experiment.Validate(), c.Probe.Validate(), c.Source.Validate(), c.Execution.Validate(), c.Resources.Validate(), c.BuildContextDigest.Validate(), c.ExpiresAt.Validate()); err != nil {
		return err
	}
	if err := c.validateIdentityClosure(); err != nil {
		return err
	}
	if err := c.validateEgressClosure(); err != nil {
		return err
	}
	return validateCapabilityExpiry(c.ExpiresAt, c.Fence.Machine.ExpiresAt)
}

func (c ExperimentCapability) validateEgressClosure() error {
	digest, err := c.Resources.Egress.Digest()
	if err != nil {
		return err
	}
	if digest != c.Execution.Subject.EgressPolicyIdentity {
		return errors.Join(core.ErrPrimitiveContract, errors.New("subject network boundary differs from experiment egress policy"))
	}
	if c.Resources.Egress.Mode == EgressDenied && c.Execution.Subject.NetworkNamespace != nil {
		return errors.Join(core.ErrPrimitiveContract, errors.New("deny-all experiment carries a network namespace"))
	}
	if c.Resources.Egress.Mode == EgressPinned && c.Execution.Subject.NetworkNamespace == nil {
		return errors.Join(core.ErrPrimitiveContract, errors.New("pinned-egress experiment lacks a prepared network namespace"))
	}
	return nil
}

func (c ExperimentCapability) validateIdentityClosure() error {
	if c.Probe.Role != projectstandards.ProbeRoleExperiment || c.Probe.Source != c.Source || c.Probe.Environment.MachineGeneration != c.Fence.Machine.Generation {
		return core.ErrPrimitiveContract
	}
	if (c.Probe.Parent == nil) != (c.ExpansionManifestDigest == nil) {
		return core.ErrPrimitiveContract
	}
	if c.ExpansionManifestDigest != nil {
		if err := c.ExpansionManifestDigest.Validate(); err != nil || *c.ExpansionManifestDigest != c.Probe.Parent.ExpansionDigest {
			return errors.Join(core.ErrPrimitiveContract, err)
		}
	}
	return nil
}

func validateCapabilityExpiry(expiresAt, fenceExpiry temporal.Instant) error {
	comparison, err := expiresAt.Compare(fenceExpiry)
	if err != nil || comparison == core.ComparisonGreater {
		return errors.Join(core.ErrPrimitiveContract, err)
	}
	return nil
}

type SchedulingClaim struct {
	Capability SchedulingCapability   `json:"scheduling_capability"`
	Members    []MemberCapability     `json:"member_capabilities"`
	Direct     []ExperimentCapability `json:"direct_experiment_capabilities"`
}

func (c SchedulingClaim) Validate() error {
	if err := c.Capability.Validate(); err != nil {
		return err
	}
	if len(c.Members) == 0 || len(c.Members) > SchedulingMemberCapabilityMaximum || len(c.Members) != len(c.Capability.Members.Entries) {
		return core.ErrPrimitiveContract
	}
	if err := validateSchedulingMembers(c); err != nil {
		return err
	}
	return validateDirectExperimentCapabilities(c)
}

func validateSchedulingMembers(c SchedulingClaim) error {
	digest, err := c.Capability.Digest()
	if err != nil {
		return err
	}
	for index := range c.Members {
		member := c.Members[index]
		if err := member.Validate(); err != nil {
			return err
		}
		if member.SchedulingDigest != digest || member.Fence != c.Capability.Fence || member.Run != c.Capability.Members.Entries[index] {
			return core.ErrPrimitiveContract
		}
	}
	return nil
}

func validateDirectExperimentCapabilities(claim SchedulingClaim) error {
	directIndex := 0
	for memberIndex := range claim.Members {
		member := claim.Members[memberIndex]
		if member.Probe.Role != projectstandards.ProbeRoleExperiment {
			continue
		}
		if directIndex >= len(claim.Direct) {
			return core.ErrPrimitiveContract
		}
		if err := validateDirectExperiment(claim.Capability, member, claim.Direct[directIndex]); err != nil {
			return err
		}
		directIndex++
	}
	if directIndex != len(claim.Direct) {
		return core.ErrPrimitiveContract
	}
	return nil
}

func validateDirectExperiment(scheduling SchedulingCapability, member MemberCapability, experiment ExperimentCapability) error {
	memberDigest, err := member.Digest()
	if err != nil {
		return err
	}
	if err := experiment.Validate(); err != nil {
		return err
	}
	if experiment.Run != member.Run || experiment.Fence != member.Fence || experiment.MemberCapabilityDigest != memberDigest || experiment.Source != scheduling.Source || experiment.Execution.Subject.PolicyIdentity != scheduling.IsolationPolicy || experiment.ExpansionManifestDigest != nil || equalProbeIdentity(experiment.Probe, member.Probe) != nil {
		return core.ErrPrimitiveContract
	}
	return nil
}

var (
	_ core.Validatable = SchedulingCapability{}
	_ core.Validatable = MemberCapability{}
	_ core.Validatable = ExperimentCapability{}
	_ core.Validatable = SchedulingClaim{}
)
