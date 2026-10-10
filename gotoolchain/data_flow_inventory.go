package gotoolchain

type sealedValue interface{ goToolchainSealedValue() }
type protocolFact interface{ goToolchainProtocolFact() }
type internalFlow interface{ goToolchainInternalFlow() }
type capabilityWrapper interface{ goToolchainCapabilityWrapper() }

func (ToolchainVersion) goToolchainSealedValue() {}
func (PackageName) goToolchainSealedValue()      {}

func (Limits) goToolchainProtocolFact()             {}
func (Configuration) goToolchainProtocolFact()      {}
func (BuildContext) goToolchainProtocolFact()       {}
func (Package) goToolchainProtocolFact()            {}
func (PackageCatalog) goToolchainProtocolFact()     {}
func (ObservationRequest) goToolchainProtocolFact() {}
func (ListRequest) goToolchainProtocolFact()        {}
func (CompileRequest) goToolchainProtocolFact()     {}
func (AnalysisRequest) goToolchainProtocolFact()    {}
func (PackageAnalysis) goToolchainProtocolFact()    {}
func (Compilation) goToolchainProtocolFact()        {}

func (buildContextWire) goToolchainInternalFlow() {}
func (packageWire) goToolchainInternalFlow()      {}
func (moduleWire) goToolchainInternalFlow()       {}

func (Capability) goToolchainCapabilityWrapper() {}

var (
	_ sealedValue       = ToolchainVersion{}
	_ sealedValue       = PackageName{}
	_ protocolFact      = Limits{}
	_ protocolFact      = Configuration{}
	_ protocolFact      = BuildContext{}
	_ protocolFact      = Package{}
	_ protocolFact      = PackageCatalog{}
	_ protocolFact      = ObservationRequest{}
	_ protocolFact      = ListRequest{}
	_ protocolFact      = CompileRequest{}
	_ protocolFact      = AnalysisRequest{}
	_ protocolFact      = PackageAnalysis{}
	_ protocolFact      = Compilation{}
	_ internalFlow      = buildContextWire{}
	_ internalFlow      = packageWire{}
	_ internalFlow      = moduleWire{}
	_ capabilityWrapper = Capability{}
)
