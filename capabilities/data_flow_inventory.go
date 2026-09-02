package capabilities

// The marker interfaces classify every production struct in this
// trust-boundary package. They are wiring proof, not behavioral proof.
type protocolFact interface{ capabilitiesProtocolFact() }
type sealedProjection interface{ capabilitiesSealedProjection() }

func (Capability) capabilitiesProtocolFact()  {}
func (Identity) capabilitiesProtocolFact()    {}
func (Purpose) capabilitiesProtocolFact()     {}
func (Requirement) capabilitiesProtocolFact() {}
func (Match) capabilitiesSealedProjection()   {}
func (Catalog) capabilitiesSealedProjection() {}

var (
	_ protocolFact     = Capability{}
	_ sealedProjection = Match{}
)
