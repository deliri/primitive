package standard

// These unexported marker interfaces are the compiler-visible data-flow
// inventory for standard. Every production struct in this trust-boundary package
// is assigned exactly one role: protocol fact, sealed projection, or internal
// flow carrier.
type protocolFact interface{ standardsProtocolFact() }
type sealedProjection interface{ standardsSealedProjection() }
type internalFlowCarrier interface{ standardsInternalFlowCarrier() }

func (Identifier) standardsProtocolFact()                  {}
func (Name) standardsProtocolFact()                        {}
func (Text) standardsProtocolFact()                        {}
func (SourcePath) standardsProtocolFact()                  {}
func (RepositoryIdentity) standardsProtocolFact()          {}
func (ProfileIdentity) standardsProtocolFact()             {}
func (OriginIdentity) standardsProtocolFact()              {}
func (EvidenceAuthority) standardsProtocolFact()           {}
func (SubjectIdentity) standardsProtocolFact()             {}
func (SourceCoordinate) standardsProtocolFact()            {}
func (SourceEffectSite) standardsProtocolFact()            {}
func (GitOrigin) standardsProtocolFact()                   {}
func (OptionalGitOrigin) standardsProtocolFact()           {}
func (RequestIdentity) standardsProtocolFact()             {}
func (RequestNonce) standardsProtocolFact()                {}
func (RunID) standardsProtocolFact()                       {}
func (ExperimentID) standardsProtocolFact()                {}
func (ObservationID) standardsProtocolFact()               {}
func (MachineID) standardsProtocolFact()                   {}
func (MachineGenerationID) standardsProtocolFact()         {}
func (MachineObservationID) standardsProtocolFact()        {}
func (Boundary) standardsProtocolFact()                    {}
func (Reason) standardsProtocolFact()                      {}
func (Feature) standardsProtocolFact()                     {}
func (UsageStep) standardsProtocolFact()                   {}
func (Usage) standardsProtocolFact()                       {}
func (AssuranceControl) standardsProtocolFact()            {}
func (Assurance) standardsProtocolFact()                   {}
func (Component) standardsProtocolFact()                   {}
func (ProjectKnowledge) standardsProtocolFact()            {}
func (Inventory) standardsProtocolFact()                   {}
func (PrimitiveCapabilityUse) standardsProtocolFact()      {}
func (SourceFileDeclarations) standardsProtocolFact()      {}
func (SourceDeclaration) standardsProtocolFact()           {}
func (DeclarationReference) standardsProtocolFact()        {}
func (PackageDependency) standardsProtocolFact()           {}
func (PackageArchitectureFacts) standardsProtocolFact()    {}
func (PackageRoleDeclaration) standardsProtocolFact()      {}
func (SourceFileEffects) standardsProtocolFact()           {}
func (SourceImport) standardsProtocolFact()                {}
func (SourceFileImports) standardsProtocolFact()           {}
func (SourceFile) standardsProtocolFact()                  {}
func (PackageFileCatalog) standardsProtocolFact()          {}
func (PackageKnowledge) standardsProtocolFact()            {}
func (CapabilityOwnership) standardsProtocolFact()         {}
func (ComplexityBound) standardsProtocolFact()             {}
func (ComplexityInput) standardsProtocolFact()             {}
func (ComplexityAssumption) standardsProtocolFact()        {}
func (CodeReference) standardsProtocolFact()               {}
func (ComplexityClaim) standardsProtocolFact()             {}
func (ComplexitySample) standardsProtocolFact()            {}
func (ComplexityCapture) standardsProtocolFact()           {}
func (GoDeclarationTarget) standardsProtocolFact()         {}
func (GoFileTarget) standardsProtocolFact()                {}
func (GoPackageTarget) standardsProtocolFact()             {}
func (JavaScriptFileTarget) standardsProtocolFact()        {}
func (NamedTarget) standardsProtocolFact()                 {}
func (ToolTarget) standardsProtocolFact()                  {}
func (ProbeTarget) standardsProtocolFact()                 {}
func (EnvironmentRequirement) standardsProtocolFact()      {}
func (AdmittedEnvironment) standardsProtocolFact()         {}
func (SelectionParent) standardsProtocolFact()             {}
func (RequestedProbe) standardsProtocolFact()              {}
func (ProbeIdentity) standardsProtocolFact()               {}
func (Admission) standardsProtocolFact()                   {}
func (Refusal) standardsProtocolFact()                     {}
func (RequestDisposition) standardsProtocolFact()          {}
func (ArtifactReference) standardsProtocolFact()           {}
func (DecimalMeasurement) standardsProtocolFact()          {}
func (BenchmarkMeasurement) standardsProtocolFact()        {}
func (ExecutionAccounting) standardsProtocolFact()         {}
func (ExecutionAttempt) standardsProtocolFact()            {}
func (ExperimentMeasurements) standardsProtocolFact()      {}
func (SelectionObservation) standardsProtocolFact()        {}
func (ExperimentObservation) standardsProtocolFact()       {}
func (InfrastructureObservation) standardsProtocolFact()   {}
func (FunctionUsage) standardsProtocolFact()               {}
func (PackageSourceUsage) standardsProtocolFact()          {}
func (PackageGroup) standardsProtocolFact()                {}
func (PackageContribution) standardsProtocolFact()         {}
func (ProjectCapability) standardsProtocolFact()           {}
func (Package) standardsProtocolFact()                     {}
func (Code) standardsProtocolFact()                        {}
func (Evidence) standardsProtocolFact()                    {}
func (MachineFingerprint) standardsProtocolFact()          {}
func (MachineIdentity) standardsProtocolFact()             {}
func (MachineCompute) standardsProtocolFact()              {}
func (MachineSystem) standardsProtocolFact()               {}
func (MachineStorage) standardsProtocolFact()              {}
func (MachineNetwork) standardsProtocolFact()              {}
func (MachineLifecycleSecurity) standardsProtocolFact()    {}
func (MachineToolchain) standardsProtocolFact()            {}
func (MachineConfiguration) standardsProtocolFact()        {}
func (MachineRuntime) standardsProtocolFact()              {}
func (MachineProbeReport) standardsProtocolFact()          {}
func (MachineProbeExecution) standardsProtocolFact()       {}
func (MachineExecutionSettings) standardsProtocolFact()    {}
func (MachineChange) standardsProtocolFact()               {}
func (MachineGenerationTransition) standardsProtocolFact() {}

func (machineFingerprintIdentityWire) standardsInternalFlowCarrier()      {}
func (machineFingerprintToolchainWire) standardsInternalFlowCarrier()     {}
func (machineConfigurationFingerprintWire) standardsInternalFlowCarrier() {}
func (experimentToolTargetWire) standardsInternalFlowCarrier()            {}
func (experimentProbeTargetWire) standardsInternalFlowCarrier()           {}
func (experimentSelectionParentWire) standardsInternalFlowCarrier()       {}
func (experimentProbeIdentityWire) standardsInternalFlowCarrier()         {}

func (EvidenceSurface) standardsSealedProjection()      {}
func (RequestReference) standardsSealedProjection()     {}
func (ObservationReference) standardsSealedProjection() {}
func (EvidenceSummary) standardsSealedProjection()      {}
func (SourceUsageSummary) standardsSealedProjection()   {}
func (CoverageChange) standardsSealedProjection()       {}
func (InventoryChange) standardsSealedProjection()      {}
func (EvidenceChange) standardsSealedProjection()       {}
func (SourceUsageChange) standardsSealedProjection()    {}
func (PackageEvolution) standardsSealedProjection()     {}
func (PackageSnapshot) standardsSealedProjection()      {}
func (PackageSummary) standardsSealedProjection()       {}
func (Project) standardsSealedProjection()              {}
func (ProjectCode) standardsSealedProjection()          {}
func (Catalog) standardsSealedProjection()              {}
func (MachineObservation) standardsSealedProjection()   {}
func (MachineGeneration) standardsSealedProjection()    {}
func (CurrentMachine) standardsSealedProjection()       {}

func (exactReportWriter) standardsInternalFlowCarrier()   {}
func (inventoryTotals) standardsInternalFlowCarrier()     {}
func (knowledgeLists) standardsInternalFlowCarrier()      {}
func (fileInventoryTotals) standardsInternalFlowCarrier() {}

var (
	_ protocolFact        = Identifier{}
	_ sealedProjection    = EvidenceSurface{}
	_ internalFlowCarrier = exactReportWriter{}
)
