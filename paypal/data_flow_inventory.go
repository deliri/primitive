package paypal

type protocolFact interface{ payPalProtocolFact() }
type internalFlow interface{ payPalInternalFlow() }
type capabilityWrapper interface{ payPalCapabilityWrapper() }

func (PayPalAccessGrant) payPalProtocolFact()  {}
func (Request) payPalProtocolFact()            {}
func (Response) payPalProtocolFact()           {}
func (DownloadRequest) payPalProtocolFact()    {}
func (DownloadResponse) payPalProtocolFact()   {}
func (InboundObservation) payPalProtocolFact() {}

func (PayPalAccessGrantRequest) payPalInternalFlow()            {}
func (payPalOAuthResponse) payPalInternalFlow()                 {}
func (payPalOAuthRequest) payPalInternalFlow()                  {}
func (clientState) payPalInternalFlow()                         {}
func (payPalWebhookVerificationStatusFact) payPalInternalFlow() {}
func (payPalWebhookVerificationResponse) payPalInternalFlow()   {}
func (payPalWebhookVerificationProjection) payPalInternalFlow() {}
func (payPalWebhookReceiver) payPalInternalFlow()               {}
func (PayPalWebhookReceiveRequest) payPalInternalFlow()         {}

func (AccessToken) payPalCapabilityWrapper()           {}
func (ClientCredential) payPalCapabilityWrapper()      {}
func (Client) payPalCapabilityWrapper()                {}
func (PayPalWebhookReceiver) payPalCapabilityWrapper() {}

var (
	_ protocolFact      = PayPalAccessGrant{}
	_ protocolFact      = Request{}
	_ protocolFact      = Response{}
	_ protocolFact      = DownloadRequest{}
	_ protocolFact      = DownloadResponse{}
	_ protocolFact      = InboundObservation{}
	_ internalFlow      = PayPalAccessGrantRequest{}
	_ internalFlow      = payPalOAuthResponse{}
	_ internalFlow      = payPalOAuthRequest{}
	_ internalFlow      = clientState{}
	_ internalFlow      = payPalWebhookVerificationStatusFact{}
	_ internalFlow      = payPalWebhookVerificationResponse{}
	_ internalFlow      = payPalWebhookVerificationProjection{}
	_ internalFlow      = payPalWebhookReceiver{}
	_ internalFlow      = PayPalWebhookReceiveRequest{}
	_ capabilityWrapper = AccessToken{}
	_ capabilityWrapper = ClientCredential{}
	_ capabilityWrapper = Client{}
	_ capabilityWrapper = PayPalWebhookReceiver{}
)
