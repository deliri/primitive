package capabilities

// The marker interfaces classify every production struct in this
// trust-boundary package. They are wiring proof, not behavioral proof.
type protocolFact interface{ capabilitiesProtocolFact() }
type sealedProjection interface{ capabilitiesSealedProjection() }

func (Capability) capabilitiesProtocolFact()  {}
func (Identity) capabilitiesProtocolFact()    {}
func (Requirement) capabilitiesProtocolFact() {}
func (Match) capabilitiesSealedProjection()   {}
func (Catalog) capabilitiesSealedProjection() {}

var (
	_ protocolFact     = Capability{}
	_ sealedProjection = Match{}
)

// Symbol contracts and their deterministic catalog rules are classified here.
func (SymbolName) capabilitiesProtocolFact()             {}
func (StandardSymbol) capabilitiesProtocolFact()         {}
func (StandardSymbolFact) capabilitiesSealedProjection() {}

type internalFlow interface{ capabilitiesInternalFlow() }

func (standardSymbolRule) capabilitiesInternalFlow() {}
func (standardMethodRule) capabilitiesInternalFlow() {}

var (
	_ protocolFact     = SymbolName{}
	_ protocolFact     = StandardSymbol{}
	_ sealedProjection = StandardSymbolFact{}
	_ internalFlow     = standardSymbolRule{}
	_ internalFlow     = standardMethodRule{}
)

func (Classification) capabilitiesSealedProjection() {}
func (classificationWire) capabilitiesProtocolFact() {}

var (
	_ sealedProjection = Classification{}
	_ protocolFact     = classificationWire{}
)
