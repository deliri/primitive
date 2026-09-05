package capabilities

// capabilitiesDataFlowInventory is the compiler-visible wiring inventory for
// every production struct in this trust-boundary package.
type capabilitiesDataFlowInventory struct {
	Identity            protocolFactRole[Identity]
	SymbolName          protocolFactRole[SymbolName]
	StandardSymbol      protocolFactRole[StandardSymbol]
	StandardSymbolFact  sealedProjectionRole[StandardSymbolFact]
	Classification      sealedProjectionRole[Classification]
	ClassificationWire  protocolFactRole[classificationWire]
	StandardSymbolRule  internalFlowRole[standardSymbolRule]
	StandardMethodRule  internalFlowRole[standardMethodRule]
	OperationContract   sealedProjectionRole[OperationContract]
	OperationDefinition internalFlowRole[operationDefinition]

	Capability  protocolFactRole[Capability]
	Requirement protocolFactRole[Requirement]
	Match       sealedProjectionRole[Match]
	Catalog     sealedProjectionRole[Catalog]
}

type internalFlowRole[T internalFlow] struct{}

type protocolFactRole[T protocolFact] struct{}
type sealedProjectionRole[T sealedProjection] struct{}

var (
	_ = capabilitiesDataFlowInventory{}.Capability
	_ = capabilitiesDataFlowInventory{}.Requirement
	_ = capabilitiesDataFlowInventory{}.Match
	_ = capabilitiesDataFlowInventory{}.Catalog
)
