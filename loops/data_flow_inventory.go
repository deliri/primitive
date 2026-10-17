package loops

type protocolFact interface{ loopsProtocolFact() }
type internalFlow interface{ loopsInternalFlow() }
type capabilityWrapper interface{ loopsCapabilityWrapper() }

func (Request) loopsProtocolFact()            {}
func (Response) loopsProtocolFact()           {}
func (DownloadRequest) loopsProtocolFact()    {}
func (DownloadResponse) loopsProtocolFact()   {}
func (InboundObservation) loopsProtocolFact() {}

func (clientState) loopsInternalFlow()           {}
func (WebhookReceiveRequest) loopsInternalFlow() {}
func (webhookReceiverState) loopsInternalFlow()  {}

func (Credential) loopsCapabilityWrapper()      {}
func (WebhookSecret) loopsCapabilityWrapper()   {}
func (Client) loopsCapabilityWrapper()          {}
func (WebhookReceiver) loopsCapabilityWrapper() {}

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
