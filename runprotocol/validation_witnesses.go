package runprotocol

import "github.com/deliri/primitive/v2026/core"

var (
	_ core.ValidatedJSONMarshaler = Identifier{}
	_ core.ValidatedJSONMarshaler = Name{}
	_ core.ValidatedJSONMarshaler = Text{}
	_ core.ValidatedJSONMarshaler = SourcePath{}
	_ core.ValidatedJSONMarshaler = RepositoryIdentity{}
	_ core.ValidatedJSONMarshaler = ProfileIdentity{}
	_ core.ValidatedJSONMarshaler = RequestIdentity{}
	_ core.ValidatedJSONMarshaler = RequestNonce{}
	_ core.ValidatedJSONMarshaler = RunID{}
	_ core.ValidatedJSONMarshaler = ExperimentID{}
	_ core.ValidatedJSONMarshaler = ObservationID{}
	_ core.ValidatedJSONMarshaler = MachineID{}
	_ core.ValidatedJSONMarshaler = MachineGenerationID{}
	_ core.ValidatedJSONMarshaler = MachineObservationID{}
	_ core.ValidatedJSONMarshaler = Outcome(0)
	_ core.ValidatedJSONMarshaler = DispositionKind(0)
	_ core.ValidatedJSONMarshaler = RefusalReason(0)
	_ core.ValidatedJSONMarshaler = TerminalState(0)
	_ core.ValidatedJSONMarshaler = ObservationKind(0)
	_ core.ValidatedJSONMarshaler = InfrastructureStage(0)
	_ core.ValidatedJSONMarshaler = CachePosture(0)
	_ core.ValidatedJSONMarshaler = ProbeRole(0)
	_ core.ValidatedJSONMarshaler = ProbeKind(0)
	_ core.ValidatedJSONMarshaler = ProbeTargetKind(0)
	_ core.ValidatedJSONMarshaler = MachineToolchainKind(0)
	_ core.ValidatedJSONMarshaler = MachineInstallMode(0)
	_ core.ValidatedJSONMarshaler = MachineProvisioningModel(0)
	_ core.ValidatedJSONMarshaler = MachineMaintenancePolicy(0)
	_ core.ValidatedJSONMarshaler = MachineChangeField(0)
	_ core.ValidatedJSONMarshaler = MachineProbeReport{}
)
