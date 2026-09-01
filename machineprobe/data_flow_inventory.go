package machineprobe

// The collector is a trust boundary. These roles make every production data
// carrier visible to the compiler and to the package inventory ratchet.
type protocolFact interface{ machineProbeProtocolFact() }
type sealedProjection interface{ machineProbeSealedProjection() }
type internalFlow interface{ machineProbeInternalFlow() }

func (Request) machineProbeProtocolFact()            {}
func (Failure) machineProbeSealedProjection()        {}
func (failureInput) machineProbeInternalFlow()       {}
func (executionFactInput) machineProbeInternalFlow() {}

var (
	_ protocolFact     = Request{}
	_ sealedProjection = Failure{}
	_ internalFlow     = failureInput{}
	_ internalFlow     = executionFactInput{}
)
