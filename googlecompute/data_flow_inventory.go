package googlecompute

type protocolFact interface{ googleComputeProtocolFact() }
type sealedProjection interface{ googleComputeSealedProjection() }
type capabilityWrapper interface{ googleComputeCapabilityWrapper() }

func (ResourceName) googleComputeProtocolFact()             {}
func (OperationName) googleComputeProtocolFact()            {}
func (InstanceReference) googleComputeProtocolFact()        {}
func (InstanceReadRequest) googleComputeProtocolFact()      {}
func (InstanceStartRequest) googleComputeProtocolFact()     {}
func (InstanceStopRequest) googleComputeProtocolFact()      {}
func (OperationReadRequest) googleComputeProtocolFact()     {}
func (InstanceObservation) googleComputeSealedProjection()  {}
func (OperationObservation) googleComputeSealedProjection() {}
func (Client) googleComputeCapabilityWrapper()              {}

var (
	_ protocolFact      = InstanceReference{}
	_ sealedProjection  = OperationObservation{}
	_ capabilityWrapper = Client{}
)
