package projectstandards

// These unexported marker interfaces are the compiler-visible data-flow
// inventory for Projectstandards. Every production struct in this trust-boundary package
// is assigned exactly one role: protocol fact, sealed projection, internal
// flow carrier, or capability wrapper.
type protocolFact interface{ projectStandardsProtocolFact() }
type sealedProjection interface{ projectStandardsSealedProjection() }
type internalFlowCarrier interface{ projectStandardsInternalFlowCarrier() }
type capabilityWrapper interface{ projectStandardsCapabilityWrapper() }

func (Identifier) projectStandardsProtocolFact()                  {}
func (Name) projectStandardsProtocolFact()                        {}
func (Text) projectStandardsProtocolFact()                        {}
func (SourcePath) projectStandardsProtocolFact()                  {}
func (RepositoryIdentity) projectStandardsProtocolFact()          {}
func (ProfileIdentity) projectStandardsProtocolFact()             {}
func (OriginIdentity) projectStandardsProtocolFact()              {}
func (EvidenceAuthority) projectStandardsProtocolFact()           {}
func (SubjectIdentity) projectStandardsProtocolFact()             {}
func (SourceCoordinate) projectStandardsProtocolFact()            {}
func (GitOrigin) projectStandardsProtocolFact()                   {}
func (OptionalGitOrigin) projectStandardsProtocolFact()           {}
func (RequestIdentity) projectStandardsProtocolFact()             {}
func (RequestNonce) projectStandardsProtocolFact()                {}
func (RunID) projectStandardsProtocolFact()                       {}
func (ExperimentID) projectStandardsProtocolFact()                {}
func (ObservationID) projectStandardsProtocolFact()               {}
func (MachineID) projectStandardsProtocolFact()                   {}
func (MachineGenerationID) projectStandardsProtocolFact()         {}
func (MachineObservationID) projectStandardsProtocolFact()        {}
func (Boundary) projectStandardsProtocolFact()                    {}
func (Reason) projectStandardsProtocolFact()                      {}
func (Feature) projectStandardsProtocolFact()                     {}
func (UsageStep) projectStandardsProtocolFact()                   {}
func (Usage) projectStandardsProtocolFact()                       {}
func (AssuranceControl) projectStandardsProtocolFact()            {}
func (Assurance) projectStandardsProtocolFact()                   {}
func (Component) projectStandardsProtocolFact()                   {}
func (ProductKnowledge) projectStandardsProtocolFact()            {}
func (Inventory) projectStandardsProtocolFact()                   {}
func (PrimitiveCapabilityUse) projectStandardsProtocolFact()      {}
func (SourceFileDeclarations) projectStandardsProtocolFact()      {}
func (SourceFileEffects) projectStandardsProtocolFact()           {}
func (SourceFile) projectStandardsProtocolFact()                  {}
func (PackageFileCatalog) projectStandardsProtocolFact()          {}
func (PackageKnowledge) projectStandardsProtocolFact()            {}
func (ComplexityBound) projectStandardsProtocolFact()             {}
func (ComplexityInput) projectStandardsProtocolFact()             {}
func (ComplexityAssumption) projectStandardsProtocolFact()        {}
func (CodeReference) projectStandardsProtocolFact()               {}
func (ComplexityClaim) projectStandardsProtocolFact()             {}
func (ComplexitySample) projectStandardsProtocolFact()            {}
func (ComplexityCapture) projectStandardsProtocolFact()           {}
func (GoDeclarationTarget) projectStandardsProtocolFact()         {}
func (GoFileTarget) projectStandardsProtocolFact()                {}
func (GoPackageTarget) projectStandardsProtocolFact()             {}
func (JavaScriptFileTarget) projectStandardsProtocolFact()        {}
func (NamedTarget) projectStandardsProtocolFact()                 {}
func (ToolTarget) projectStandardsProtocolFact()                  {}
func (ProbeTarget) projectStandardsProtocolFact()                 {}
func (EnvironmentRequirement) projectStandardsProtocolFact()      {}
func (AdmittedEnvironment) projectStandardsProtocolFact()         {}
func (SelectionParent) projectStandardsProtocolFact()             {}
func (RequestedProbe) projectStandardsProtocolFact()              {}
func (ProbeIdentity) projectStandardsProtocolFact()               {}
func (Admission) projectStandardsProtocolFact()                   {}
func (Refusal) projectStandardsProtocolFact()                     {}
func (RequestDisposition) projectStandardsProtocolFact()          {}
func (ArtifactReference) projectStandardsProtocolFact()           {}
func (DecimalMeasurement) projectStandardsProtocolFact()          {}
func (BenchmarkMeasurement) projectStandardsProtocolFact()        {}
func (ExecutionAccounting) projectStandardsProtocolFact()         {}
func (ExecutionAttempt) projectStandardsProtocolFact()            {}
func (ExperimentMeasurements) projectStandardsProtocolFact()      {}
func (SelectionObservation) projectStandardsProtocolFact()        {}
func (ExperimentObservation) projectStandardsProtocolFact()       {}
func (InfrastructureObservation) projectStandardsProtocolFact()   {}
func (FunctionUsage) projectStandardsProtocolFact()               {}
func (PackageSourceUsage) projectStandardsProtocolFact()          {}
func (PackageGroup) projectStandardsProtocolFact()                {}
func (PackageContribution) projectStandardsProtocolFact()         {}
func (ProjectCapability) projectStandardsProtocolFact()           {}
func (Query) projectStandardsProtocolFact()                       {}
func (Package) projectStandardsProtocolFact()                     {}
func (Code) projectStandardsProtocolFact()                        {}
func (Evidence) projectStandardsProtocolFact()                    {}
func (MachineFingerprint) projectStandardsProtocolFact()          {}
func (MachineIdentity) projectStandardsProtocolFact()             {}
func (MachineCompute) projectStandardsProtocolFact()              {}
func (MachineSystem) projectStandardsProtocolFact()               {}
func (MachineStorage) projectStandardsProtocolFact()              {}
func (MachineNetwork) projectStandardsProtocolFact()              {}
func (MachineLifecycleSecurity) projectStandardsProtocolFact()    {}
func (MachineToolchain) projectStandardsProtocolFact()            {}
func (MachineConfiguration) projectStandardsProtocolFact()        {}
func (MachineRuntime) projectStandardsProtocolFact()              {}
func (MachineProbeReport) projectStandardsProtocolFact()          {}
func (MachineProbeExecution) projectStandardsProtocolFact()       {}
func (MachineExecutionSettings) projectStandardsProtocolFact()    {}
func (MachineChange) projectStandardsProtocolFact()               {}
func (MachineGenerationTransition) projectStandardsProtocolFact() {}
func (MachineQuery) projectStandardsProtocolFact()                {}

func (ProjectStandardsEvidenceSurface) projectStandardsSealedProjection()      {}
func (ProjectStandardsRequestReference) projectStandardsSealedProjection()     {}
func (ProjectStandardsObservationReference) projectStandardsSealedProjection() {}
func (EvidenceSummary) projectStandardsSealedProjection()                      {}
func (SourceUsageSummary) projectStandardsSealedProjection()                   {}
func (CoverageChange) projectStandardsSealedProjection()                       {}
func (InventoryChange) projectStandardsSealedProjection()                      {}
func (EvidenceChange) projectStandardsSealedProjection()                       {}
func (SourceUsageChange) projectStandardsSealedProjection()                    {}
func (PackageEvolution) projectStandardsSealedProjection()                     {}
func (PackageSnapshot) projectStandardsSealedProjection()                      {}
func (PackageSummary) projectStandardsSealedProjection()                       {}
func (Project) projectStandardsSealedProjection()                              {}
func (ProjectCode) projectStandardsSealedProjection()                          {}
func (Catalog) projectStandardsSealedProjection()                              {}
func (Response) projectStandardsSealedProjection()                             {}
func (FetchResult) projectStandardsSealedProjection()                          {}
func (MachineObservation) projectStandardsSealedProjection()                   {}
func (MachineGeneration) projectStandardsSealedProjection()                    {}
func (CurrentMachine) projectStandardsSealedProjection()                       {}
func (MachineResponse) projectStandardsSealedProjection()                      {}
func (MachineFetchResult) projectStandardsSealedProjection()                   {}

func (exactReportWriter) projectStandardsInternalFlowCarrier()   {}
func (inventoryTotals) projectStandardsInternalFlowCarrier()     {}
func (knowledgeLists) projectStandardsInternalFlowCarrier()      {}
func (fileInventoryTotals) projectStandardsInternalFlowCarrier() {}

func (Service) projectStandardsCapabilityWrapper()        {}
func (Client) projectStandardsCapabilityWrapper()         {}
func (Server) projectStandardsCapabilityWrapper()         {}
func (MachineService) projectStandardsCapabilityWrapper() {}
func (MachineClient) projectStandardsCapabilityWrapper()  {}
func (MachineServer) projectStandardsCapabilityWrapper()  {}

var (
	_ protocolFact        = Query{}
	_ sealedProjection    = Response{}
	_ internalFlowCarrier = exactReportWriter{}
	_ capabilityWrapper   = Service{}
)
