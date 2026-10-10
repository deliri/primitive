package runprotocol

// These unexported marker interfaces are the compiler-visible data-flow
// inventory for runprotocol. Every production struct in this trust-boundary package
// is assigned exactly one role: protocol fact, sealed projection, or internal
// flow carrier.
type protocolFact interface{ runProtocolFact() }
type sealedProjection interface{ runProtocolSealedProjection() }
type internalFlowCarrier interface{ runProtocolInternalFlowCarrier() }

func (Identifier) runProtocolFact()                  {}
func (Name) runProtocolFact()                        {}
func (Text) runProtocolFact()                        {}
func (SourcePath) runProtocolFact()                  {}
func (RepositoryIdentity) runProtocolFact()          {}
func (ProfileIdentity) runProtocolFact()             {}
func (OriginIdentity) runProtocolFact()              {}
func (EvidenceAuthority) runProtocolFact()           {}
func (SubjectIdentity) runProtocolFact()             {}
func (SourceCoordinate) runProtocolFact()            {}
func (RequestIdentity) runProtocolFact()             {}
func (RequestNonce) runProtocolFact()                {}
func (RunID) runProtocolFact()                       {}
func (ExperimentID) runProtocolFact()                {}
func (ObservationID) runProtocolFact()               {}
func (MachineID) runProtocolFact()                   {}
func (MachineGenerationID) runProtocolFact()         {}
func (MachineObservationID) runProtocolFact()        {}
func (ScalingSample) runProtocolFact()               {}
func (ScalingCapture) runProtocolFact()              {}
func (GoDeclarationTarget) runProtocolFact()         {}
func (GoFileTarget) runProtocolFact()                {}
func (GoPackageTarget) runProtocolFact()             {}
func (JavaScriptFileTarget) runProtocolFact()        {}
func (NamedTarget) runProtocolFact()                 {}
func (ToolTarget) runProtocolFact()                  {}
func (ProbeTarget) runProtocolFact()                 {}
func (EnvironmentRequirement) runProtocolFact()      {}
func (AdmittedEnvironment) runProtocolFact()         {}
func (SelectionParent) runProtocolFact()             {}
func (RequestedProbe) runProtocolFact()              {}
func (ProbeIdentity) runProtocolFact()               {}
func (Admission) runProtocolFact()                   {}
func (Refusal) runProtocolFact()                     {}
func (RequestDisposition) runProtocolFact()          {}
func (ArtifactReference) runProtocolFact()           {}
func (DecimalMeasurement) runProtocolFact()          {}
func (BenchmarkMeasurement) runProtocolFact()        {}
func (ExecutionAccounting) runProtocolFact()         {}
func (ExecutionAttempt) runProtocolFact()            {}
func (ExperimentMeasurements) runProtocolFact()      {}
func (SelectionObservation) runProtocolFact()        {}
func (ExperimentObservation) runProtocolFact()       {}
func (InfrastructureObservation) runProtocolFact()   {}
func (MachineFingerprint) runProtocolFact()          {}
func (MachineIdentity) runProtocolFact()             {}
func (MachineCompute) runProtocolFact()              {}
func (MachineSystem) runProtocolFact()               {}
func (MachineStorage) runProtocolFact()              {}
func (MachineNetwork) runProtocolFact()              {}
func (MachineLifecycleSecurity) runProtocolFact()    {}
func (MachineToolchain) runProtocolFact()            {}
func (MachineConfiguration) runProtocolFact()        {}
func (MachineRuntime) runProtocolFact()              {}
func (MachineProbeReport) runProtocolFact()          {}
func (MachineProbeExecution) runProtocolFact()       {}
func (MachineExecutionSettings) runProtocolFact()    {}
func (MachineChange) runProtocolFact()               {}
func (MachineGenerationTransition) runProtocolFact() {}

func (machineFingerprintIdentityWire) runProtocolInternalFlowCarrier()      {}
func (machineFingerprintToolchainWire) runProtocolInternalFlowCarrier()     {}
func (machineConfigurationFingerprintWire) runProtocolInternalFlowCarrier() {}
func (experimentToolTargetWire) runProtocolInternalFlowCarrier()            {}
func (experimentProbeTargetWire) runProtocolInternalFlowCarrier()           {}
func (experimentSelectionParentWire) runProtocolInternalFlowCarrier()       {}
func (experimentProbeIdentityWire) runProtocolInternalFlowCarrier()         {}

func (EvidenceSurface) runProtocolSealedProjection()      {}
func (RequestReference) runProtocolSealedProjection()     {}
func (ObservationReference) runProtocolSealedProjection() {}
func (MachineObservation) runProtocolSealedProjection()   {}
func (MachineGeneration) runProtocolSealedProjection()    {}
func (CurrentMachine) runProtocolSealedProjection()       {}

var (
	_ protocolFact        = Identifier{}
	_ sealedProjection    = ObservationReference{}
	_ internalFlowCarrier = machineFingerprintIdentityWire{}
)
