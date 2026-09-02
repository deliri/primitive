package plunk

type protocolFact interface{ plunkProtocolFact() }
type internalFlow interface{ plunkInternalFlow() }
type capabilityWrapper interface{ plunkCapabilityWrapper() }

func (Request) plunkProtocolFact()            {}
func (Response) plunkProtocolFact()           {}
func (DownloadRequest) plunkProtocolFact()    {}
func (DownloadResponse) plunkProtocolFact()   {}
func (InboundObservation) plunkProtocolFact() {}

func (clientState) plunkInternalFlow()           {}
func (WebhookReceiveRequest) plunkInternalFlow() {}
func (webhookReceiverState) plunkInternalFlow()  {}

func (Credential) plunkCapabilityWrapper()      {}
func (WebhookSecret) plunkCapabilityWrapper()   {}
func (Client) plunkCapabilityWrapper()          {}
func (WebhookReceiver) plunkCapabilityWrapper() {}

var (
	_ protocolFact      = Request{}
	_ protocolFact      = Response{}
	_ protocolFact      = DownloadRequest{}
	_ protocolFact      = DownloadResponse{}
	_ protocolFact      = InboundObservation{}
	_ internalFlow      = clientState{}
	_ internalFlow      = WebhookReceiveRequest{}
	_ internalFlow      = webhookReceiverState{}
	_ capabilityWrapper = Credential{}
	_ capabilityWrapper = WebhookSecret{}
	_ capabilityWrapper = Client{}
	_ capabilityWrapper = WebhookReceiver{}
)
