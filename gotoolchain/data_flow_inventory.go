package gotoolchain

type sealedValue interface{ goToolchainSealedValue() }
type protocolFact interface{ goToolchainProtocolFact() }
type internalFlow interface{ goToolchainInternalFlow() }
type capabilityWrapper interface{ goToolchainCapabilityWrapper() }

func (ToolchainVersion) goToolchainSealedValue() {}
func (PackageName) goToolchainSealedValue()      {}
func (TestName) goToolchainSealedValue()         {}

func (Limits) goToolchainProtocolFact()              {}
func (Configuration) goToolchainProtocolFact()       {}
func (BuildContext) goToolchainProtocolFact()        {}
func (Package) goToolchainProtocolFact()             {}
func (ObservationRequest) goToolchainProtocolFact()  {}
func (PackageVisitRequest) goToolchainProtocolFact() {}
func (ListRequest) goToolchainProtocolFact()         {}
func (CompileRequest) goToolchainProtocolFact()      {}
func (AnalysisRequest) goToolchainProtocolFact()     {}
func (PackageAnalysis) goToolchainProtocolFact()     {}
func (Compilation) goToolchainProtocolFact()         {}
func (TestTableRequest) goToolchainProtocolFact()    {}
func (TestTables) goToolchainProtocolFact()          {}

func (buildContextWire) goToolchainInternalFlow() {}
func (packageWire) goToolchainInternalFlow()      {}
func (moduleWire) goToolchainInternalFlow()       {}

func (Capability) goToolchainCapabilityWrapper() {}

var (
	_ sealedValue       = ToolchainVersion{}
	_ sealedValue       = PackageName{}
	_ sealedValue       = TestName{}
	_ protocolFact      = Limits{}
	_ protocolFact      = Configuration{}
	_ protocolFact      = BuildContext{}
	_ protocolFact      = Package{}
	_ protocolFact      = ObservationRequest{}
	_ protocolFact      = PackageVisitRequest{}
	_ protocolFact      = ListRequest{}
	_ protocolFact      = CompileRequest{}
	_ protocolFact      = AnalysisRequest{}
	_ protocolFact      = PackageAnalysis{}
	_ protocolFact      = Compilation{}
	_ protocolFact      = TestTableRequest{}
	_ protocolFact      = TestTables{}
	_ internalFlow      = buildContextWire{}
	_ internalFlow      = packageWire{}
	_ internalFlow      = moduleWire{}
	_ capabilityWrapper = Capability{}
)
