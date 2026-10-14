package ai

import "github.com/deliri/primitive/v2026/core"

type protocolFact interface{ aiProtocolFact() }
type capabilityWrapper interface{ aiCapabilityWrapper() }

func (OptionalHeader) aiProtocolFact() {}

func (Route) aiProtocolFact()            {}
func (Request) aiProtocolFact()          {}
func (Response) aiProtocolFact()         {}
func (Credential) aiCapabilityWrapper()  {}
func (Client) aiCapabilityWrapper()      {}
func (*HTTPServer) aiCapabilityWrapper() {}

var (
	_ protocolFact      = OptionalHeader{}
	_ core.OffWireEnum  = OptionalHeaderUnknown
	_ core.OffWireEnum  = APIVersionDefault
	_ protocolFact      = Route{}
	_ protocolFact      = Request{}
	_ protocolFact      = Response{}
	_ capabilityWrapper = Credential{}
	_ capabilityWrapper = Client{}
	_ capabilityWrapper = (*HTTPServer)(nil)
	_ core.Validatable  = Route{}
	_ core.Validatable  = Request{}
	_ core.Validatable  = Response{}
	_ core.Validatable  = Credential{}
	_ core.Validatable  = Client{}
	_ Server            = (*HTTPServer)(nil)
	_ core.OffWireEnum  = ProviderUnknown
	_ core.OffWireEnum  = OperationUnknown
)
