package stripe

type protocolFact interface{ stripeProtocolFact() }
type internalFlow interface{ stripeInternalFlow() }
type capabilityWrapper interface{ stripeCapabilityWrapper() }

func (Request) stripeProtocolFact()            {}
func (Response) stripeProtocolFact()           {}
func (DownloadRequest) stripeProtocolFact()    {}
func (DownloadResponse) stripeProtocolFact()   {}
func (InboundObservation) stripeProtocolFact() {}

func (clientState) stripeInternalFlow()           {}
func (WebhookReceiveRequest) stripeInternalFlow() {}
func (webhookReceiverState) stripeInternalFlow()  {}

func (Credential) stripeCapabilityWrapper()      {}
func (WebhookSecret) stripeCapabilityWrapper()   {}
func (Client) stripeCapabilityWrapper()          {}
func (WebhookReceiver) stripeCapabilityWrapper() {}

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
