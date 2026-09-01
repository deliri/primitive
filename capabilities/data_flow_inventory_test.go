package capabilities

// capabilitiesDataFlowInventory is the compiler-visible wiring inventory for
// every production struct in this trust-boundary package.
type capabilitiesDataFlowInventory struct {
	Capability  protocolFactRole[Capability]
	Purpose     protocolFactRole[Purpose]
	Requirement protocolFactRole[Requirement]
	Match       sealedProjectionRole[Match]
	Catalog     sealedProjectionRole[Catalog]
}

type protocolFactRole[T protocolFact] struct{}
type sealedProjectionRole[T sealedProjection] struct{}

var (
	_ = capabilitiesDataFlowInventory{}.Capability
	_ = capabilitiesDataFlowInventory{}.Purpose
	_ = capabilitiesDataFlowInventory{}.Requirement
	_ = capabilitiesDataFlowInventory{}.Match
	_ = capabilitiesDataFlowInventory{}.Catalog
)
