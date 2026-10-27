package plunk

type protocolFact interface{ plunkProtocolFact() }
type internalFlow interface{ plunkInternalFlow() }
type capabilityWrapper interface{ plunkCapabilityWrapper() }

func (Request) plunkProtocolFact()            {}
func (Response) plunkProtocolFact()           {}
func (DownloadRequest) plunkProtocolFact()    {}
func (DownloadResponse) plunkProtocolFact()   {}
func (InboundObservation) plunkProtocolFact() {}

func (SendRequest[Data]) plunkProtocolFact()      {}
func (SendResponse) plunkProtocolFact()           {}
func (SendData) plunkProtocolFact()               {}
func (SentEmail) plunkProtocolFact()              {}
func (SendContact) plunkProtocolFact()            {}
func (SendError) plunkProtocolFact()              {}
func (SendTimestamp) plunkProtocolFact()          {}
func (SendResponseShapeError) plunkInternalFlow() {}

func (TemplateMetadata) plunkProtocolFact()         {}
func (WebhookJSONRequest) plunkInternalFlow()       {}
func (WebhookDocument[Payload]) plunkProtocolFact() {}
func (webhookJSONBuffer) plunkInternalFlow()        {}
func (wireDocumentRefusal) plunkInternalFlow()      {}

func (clientState) plunkInternalFlow()           {}
func (WebhookReceiveRequest) plunkInternalFlow() {}
func (webhookReceiverState) plunkInternalFlow()  {}

func (Credential) plunkCapabilityWrapper()      {}
func (WebhookSecret) plunkCapabilityWrapper()   {}
func (Client) plunkCapabilityWrapper()          {}
func (WebhookReceiver) plunkCapabilityWrapper() {}

var (
	_ protocolFact      = TemplateMetadata{}
	_ protocolFact      = WebhookDocument[struct{}]{}
	_ internalFlow      = WebhookJSONRequest{}
	_ internalFlow      = webhookJSONBuffer{}
	_ internalFlow      = wireDocumentRefusal{}
	_ protocolFact      = SendRequest[struct{}]{}
	_ protocolFact      = SendResponse{}
	_ protocolFact      = SendData{}
	_ protocolFact      = SentEmail{}
	_ protocolFact      = SendContact{}
	_ protocolFact      = SendError{}
	_ protocolFact      = SendTimestamp{}
	_ internalFlow      = SendResponseShapeError{}
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
