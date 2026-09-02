package twilio

type protocolFact interface{ twilioProtocolFact() }
type internalFlow interface{ twilioInternalFlow() }
type capabilityWrapper interface{ twilioCapabilityWrapper() }

func (Request) twilioProtocolFact()            {}
func (Response) twilioProtocolFact()           {}
func (DownloadRequest) twilioProtocolFact()    {}
func (DownloadResponse) twilioProtocolFact()   {}
func (InboundObservation) twilioProtocolFact() {}

func (clientState) twilioInternalFlow()            {}
func (WebhookReceiverRequest) twilioInternalFlow() {}
func (WebhookReceiveRequest) twilioInternalFlow()  {}
func (webhookReceiverState) twilioInternalFlow()   {}

func (Credential) twilioCapabilityWrapper()      {}
func (AuthToken) twilioCapabilityWrapper()       {}
func (Client) twilioCapabilityWrapper()          {}
func (WebhookReceiver) twilioCapabilityWrapper() {}

var (
	_ protocolFact      = Request{}
	_ protocolFact      = Response{}
	_ protocolFact      = DownloadRequest{}
	_ protocolFact      = DownloadResponse{}
	_ protocolFact      = InboundObservation{}
	_ internalFlow      = clientState{}
	_ internalFlow      = WebhookReceiverRequest{}
	_ internalFlow      = WebhookReceiveRequest{}
	_ internalFlow      = webhookReceiverState{}
	_ capabilityWrapper = Credential{}
	_ capabilityWrapper = AuthToken{}
	_ capabilityWrapper = Client{}
	_ capabilityWrapper = WebhookReceiver{}
)
