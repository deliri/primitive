package gmail

type protocolFact interface{ gmailProtocolFact() }
type sealedProjection interface{ gmailSealedProjection() }
type capabilityWrapper interface{ gmailCapabilityWrapper() }
type internalFlow interface{ gmailInternalFlow() }

func (ThreadListRequest) gmailProtocolFact()  {}
func (ThreadReadRequest) gmailProtocolFact()  {}
func (MessageReadRequest) gmailProtocolFact() {}
func (RefreshRequest) gmailProtocolFact()     {}

func (ThreadPage) gmailSealedProjection()    {}
func (Thread) gmailSealedProjection()        {}
func (MessageHeader) gmailSealedProjection() {}
func (Recipient) gmailSealedProjection()     {}
func (Recipients) gmailSealedProjection()    {}
func (Message) gmailSealedProjection()       {}

func (Client) gmailCapabilityWrapper()                       {}
func (Server) gmailCapabilityWrapper()                       {}
func (Operation[Request, Response]) gmailCapabilityWrapper() {}
func (RefreshToken) gmailCapabilityWrapper()                 {}
func (Access) gmailCapabilityWrapper()                       {}

func (responseBuffer) gmailInternalFlow()      {}
func (threadReferenceWire) gmailInternalFlow() {}
func (threadPageWire) gmailInternalFlow()      {}
func (threadWire) gmailInternalFlow()          {}
func (messageHeaderWire) gmailInternalFlow()   {}
func (metadataWire) gmailInternalFlow()        {}
func (headerWire) gmailInternalFlow()          {}
func (messageWire) gmailInternalFlow()         {}
func (mimePartWire) gmailInternalFlow()        {}
func (mimeBodyWire) gmailInternalFlow()        {}
func (tokenWire) gmailInternalFlow()           {}
func (oauthErrorWire) gmailInternalFlow()      {}
func (mailboxWire) gmailInternalFlow()         {}

var (
	_ protocolFact      = ThreadListRequest{}
	_ protocolFact      = ThreadReadRequest{}
	_ protocolFact      = MessageReadRequest{}
	_ protocolFact      = RefreshRequest{}
	_ sealedProjection  = ThreadPage{}
	_ sealedProjection  = Thread{}
	_ sealedProjection  = MessageHeader{}
	_ sealedProjection  = Message{}
	_ capabilityWrapper = Client{}
	_ capabilityWrapper = Server{}
	_ capabilityWrapper = Operation[ThreadListRequest, ThreadPage]{}
	_ capabilityWrapper = RefreshToken{}
	_ capabilityWrapper = Access{}
	_ internalFlow      = tokenWire{}
)
