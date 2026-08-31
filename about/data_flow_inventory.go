package about

// These unexported marker interfaces are the compiler-visible data-flow
// inventory for About. Every production struct in this trust-boundary package
// is assigned exactly one role: protocol fact, sealed projection, internal
// flow carrier, or capability wrapper.
type protocolFact interface{ aboutProtocolFact() }
type sealedProjection interface{ aboutSealedProjection() }
type internalFlowCarrier interface{ aboutInternalFlowCarrier() }
type capabilityWrapper interface{ aboutCapabilityWrapper() }

func (Identifier) aboutProtocolFact()                  {}
func (Name) aboutProtocolFact()                        {}
func (Text) aboutProtocolFact()                        {}
func (SourcePath) aboutProtocolFact()                  {}
func (RepositoryIdentity) aboutProtocolFact()          {}
func (ProfileIdentity) aboutProtocolFact()             {}
func (OriginIdentity) aboutProtocolFact()              {}
func (EvidenceAuthority) aboutProtocolFact()           {}
func (SubjectIdentity) aboutProtocolFact()             {}
func (SourceCoordinate) aboutProtocolFact()            {}
func (GitOrigin) aboutProtocolFact()                   {}
func (OptionalGitOrigin) aboutProtocolFact()           {}
func (RequestIdentity) aboutProtocolFact()             {}
func (RunID) aboutProtocolFact()                       {}
func (ExperimentID) aboutProtocolFact()                {}
func (ObservationID) aboutProtocolFact()               {}
func (MachineID) aboutProtocolFact()                   {}
func (MachineGenerationID) aboutProtocolFact()         {}
func (MachineObservationID) aboutProtocolFact()        {}
func (Boundary) aboutProtocolFact()                    {}
func (Reason) aboutProtocolFact()                      {}
func (Feature) aboutProtocolFact()                     {}
func (UsageStep) aboutProtocolFact()                   {}
func (Usage) aboutProtocolFact()                       {}
func (AssuranceControl) aboutProtocolFact()            {}
func (Assurance) aboutProtocolFact()                   {}
func (Component) aboutProtocolFact()                   {}
func (ProductKnowledge) aboutProtocolFact()            {}
func (Inventory) aboutProtocolFact()                   {}
func (PackageKnowledge) aboutProtocolFact()            {}
func (ComplexityBound) aboutProtocolFact()             {}
func (ComplexityInput) aboutProtocolFact()             {}
func (ComplexityAssumption) aboutProtocolFact()        {}
func (CodeReference) aboutProtocolFact()               {}
func (ComplexityClaim) aboutProtocolFact()             {}
func (ComplexitySample) aboutProtocolFact()            {}
func (ComplexityCapture) aboutProtocolFact()           {}
func (GoDeclarationTarget) aboutProtocolFact()         {}
func (GoFileTarget) aboutProtocolFact()                {}
func (GoPackageTarget) aboutProtocolFact()             {}
func (JavaScriptFileTarget) aboutProtocolFact()        {}
func (NamedTarget) aboutProtocolFact()                 {}
func (ToolTarget) aboutProtocolFact()                  {}
func (ProbeTarget) aboutProtocolFact()                 {}
func (EnvironmentRequirement) aboutProtocolFact()      {}
func (AdmittedEnvironment) aboutProtocolFact()         {}
func (SelectionParent) aboutProtocolFact()             {}
func (RequestedProbe) aboutProtocolFact()              {}
func (ProbeIdentity) aboutProtocolFact()               {}
func (Admission) aboutProtocolFact()                   {}
func (Refusal) aboutProtocolFact()                     {}
func (RequestDisposition) aboutProtocolFact()          {}
func (ArtifactReference) aboutProtocolFact()           {}
func (BenchmarkMeasurement) aboutProtocolFact()        {}
func (ExperimentMeasurements) aboutProtocolFact()      {}
func (SelectionObservation) aboutProtocolFact()        {}
func (ExperimentObservation) aboutProtocolFact()       {}
func (InfrastructureObservation) aboutProtocolFact()   {}
func (FunctionUsage) aboutProtocolFact()               {}
func (PackageSourceUsage) aboutProtocolFact()          {}
func (PackageGroup) aboutProtocolFact()                {}
func (PackageContribution) aboutProtocolFact()         {}
func (ProjectCapability) aboutProtocolFact()           {}
func (Query) aboutProtocolFact()                       {}
func (Package) aboutProtocolFact()                     {}
func (Code) aboutProtocolFact()                        {}
func (Evidence) aboutProtocolFact()                    {}
func (MachineFingerprint) aboutProtocolFact()          {}
func (MachineIdentity) aboutProtocolFact()             {}
func (MachineCompute) aboutProtocolFact()              {}
func (MachineSystem) aboutProtocolFact()               {}
func (MachineStorage) aboutProtocolFact()              {}
func (MachineNetwork) aboutProtocolFact()              {}
func (MachineLifecycleSecurity) aboutProtocolFact()    {}
func (MachineToolchain) aboutProtocolFact()            {}
func (MachineConfiguration) aboutProtocolFact()        {}
func (MachineRuntime) aboutProtocolFact()              {}
func (MachineProbeReport) aboutProtocolFact()          {}
func (MachineProbeExecution) aboutProtocolFact()       {}
func (MachineChange) aboutProtocolFact()               {}
func (MachineGenerationTransition) aboutProtocolFact() {}
func (MachineQuery) aboutProtocolFact()                {}

func (AboutEvidenceSurface) aboutSealedProjection()      {}
func (AboutRequestReference) aboutSealedProjection()     {}
func (AboutObservationReference) aboutSealedProjection() {}
func (EvidenceSummary) aboutSealedProjection()           {}
func (SourceUsageSummary) aboutSealedProjection()        {}
func (PackageSnapshot) aboutSealedProjection()           {}
func (PackageSummary) aboutSealedProjection()            {}
func (Project) aboutSealedProjection()                   {}
func (ProjectCode) aboutSealedProjection()               {}
func (Catalog) aboutSealedProjection()                   {}
func (Response) aboutSealedProjection()                  {}
func (FetchResult) aboutSealedProjection()               {}
func (MachineObservation) aboutSealedProjection()        {}
func (MachineGeneration) aboutSealedProjection()         {}
func (CurrentMachine) aboutSealedProjection()            {}
func (MachineResponse) aboutSealedProjection()           {}
func (MachineFetchResult) aboutSealedProjection()        {}

func (exactReportWriter) aboutInternalFlowCarrier() {}
func (inventoryTotals) aboutInternalFlowCarrier()   {}
func (knowledgeLists) aboutInternalFlowCarrier()    {}

func (Service) aboutCapabilityWrapper()        {}
func (Client) aboutCapabilityWrapper()         {}
func (Server) aboutCapabilityWrapper()         {}
func (MachineService) aboutCapabilityWrapper() {}
func (MachineClient) aboutCapabilityWrapper()  {}
func (MachineServer) aboutCapabilityWrapper()  {}

var (
	_ protocolFact        = Query{}
	_ sealedProjection    = Response{}
	_ internalFlowCarrier = exactReportWriter{}
	_ capabilityWrapper   = Service{}
)
