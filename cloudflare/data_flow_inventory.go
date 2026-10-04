package cloudflare

type cloudflareProtocolFactRole interface{ cloudflareProtocolFact() }

func (InboundObservation) cloudflareProtocolFact()            {}
func (ServerOptions) cloudflareProtocolFact()                 {}
func (APIIssue) cloudflareProtocolFact()                      {}
func (APIIssueSource) cloudflareProtocolFact()                {}
func (APIRefusal) cloudflareProtocolFact()                    {}
func (apiEnvelope[T]) cloudflareProtocolFact()                {}
func (imageDirectUploadWire) cloudflareProtocolFact()         {}
func (ImageDirectUploadRequest) cloudflareProtocolFact()      {}
func (streamDirectUploadRequestWire) cloudflareProtocolFact() {}
func (streamWatermarkWire) cloudflareProtocolFact()           {}
func (streamDirectUploadWire) cloudflareProtocolFact()        {}
func (StreamDirectUploadRequest) cloudflareProtocolFact()     {}
func (R2PresignRequest) cloudflareProtocolFact()              {}
func (R2GrantInput) cloudflareProtocolFact()                  {}

type cloudflareInternalFlowRole interface{ cloudflareInternalFlow() }

func (WebhookReceiveRequest) cloudflareInternalFlow()       {}
func (StreamWebhookReceiveRequest) cloudflareInternalFlow() {}
func (streamSignature) cloudflareInternalFlow()             {}
func (contextReader) cloudflareInternalFlow()               {}
func (apiServer) cloudflareInternalFlow()                   {}
func (apiIntent) cloudflareInternalFlow()                   {}
func (boundedResponse) cloudflareInternalFlow()             {}
func (R2WriteRequest) cloudflareInternalFlow()              {}
func (MediaUpload) cloudflareInternalFlow()                 {}
func (webhookDestination) cloudflareInternalFlow()          {}
func (*exactSource) cloudflareInternalFlow()                {}

type cloudflareCapabilityWrapperRole interface{ cloudflareCapabilityWrapper() }

func (APIToken) cloudflareCapabilityWrapper()              {}
func (NotificationSecret) cloudflareCapabilityWrapper()    {}
func (StreamWebhookSecret) cloudflareCapabilityWrapper()   {}
func (ImagesWebhookReceiver) cloudflareCapabilityWrapper() {}
func (StreamWebhookReceiver) cloudflareCapabilityWrapper() {}
func (AccountID) cloudflareCapabilityWrapper()             {}
func (ImageID) cloudflareCapabilityWrapper()               {}
func (StreamVideoID) cloudflareCapabilityWrapper()         {}
func (ImagesServer) cloudflareCapabilityWrapper()          {}
func (ImageUpload) cloudflareCapabilityWrapper()           {}
func (ImagesClient) cloudflareCapabilityWrapper()          {}
func (StreamServer) cloudflareCapabilityWrapper()          {}
func (StreamUpload) cloudflareCapabilityWrapper()          {}
func (StreamClient) cloudflareCapabilityWrapper()          {}
func (R2Bucket) cloudflareCapabilityWrapper()              {}
func (R2Key) cloudflareCapabilityWrapper()                 {}
func (R2Credentials) cloudflareCapabilityWrapper()         {}
func (R2Server) cloudflareCapabilityWrapper()              {}
func (R2Grant) cloudflareCapabilityWrapper()               {}
func (R2Client) cloudflareCapabilityWrapper()              {}

var (
	_ cloudflareProtocolFactRole      = InboundObservation{}
	_ cloudflareProtocolFactRole      = ServerOptions{}
	_ cloudflareProtocolFactRole      = APIIssue{}
	_ cloudflareProtocolFactRole      = APIIssueSource{}
	_ cloudflareProtocolFactRole      = APIRefusal{}
	_ cloudflareProtocolFactRole      = apiEnvelope[imageDirectUploadWire]{}
	_ cloudflareProtocolFactRole      = imageDirectUploadWire{}
	_ cloudflareProtocolFactRole      = ImageDirectUploadRequest{}
	_ cloudflareProtocolFactRole      = streamDirectUploadRequestWire{}
	_ cloudflareProtocolFactRole      = streamDirectUploadWire{}
	_ cloudflareProtocolFactRole      = streamWatermarkWire{}
	_ cloudflareProtocolFactRole      = StreamDirectUploadRequest{}
	_ cloudflareProtocolFactRole      = R2PresignRequest{}
	_ cloudflareProtocolFactRole      = R2GrantInput{}
	_ cloudflareInternalFlowRole      = WebhookReceiveRequest{}
	_ cloudflareInternalFlowRole      = StreamWebhookReceiveRequest{}
	_ cloudflareInternalFlowRole      = streamSignature{}
	_ cloudflareInternalFlowRole      = contextReader{}
	_ cloudflareInternalFlowRole      = apiServer{}
	_ cloudflareInternalFlowRole      = apiIntent{}
	_ cloudflareInternalFlowRole      = boundedResponse{}
	_ cloudflareInternalFlowRole      = R2WriteRequest{}
	_ cloudflareInternalFlowRole      = MediaUpload{}
	_ cloudflareInternalFlowRole      = (*exactSource)(nil)
	_ cloudflareInternalFlowRole      = webhookDestination{}
	_ cloudflareCapabilityWrapperRole = APIToken{}
	_ cloudflareCapabilityWrapperRole = NotificationSecret{}
	_ cloudflareCapabilityWrapperRole = StreamWebhookSecret{}
	_ cloudflareCapabilityWrapperRole = ImagesWebhookReceiver{}
	_ cloudflareCapabilityWrapperRole = StreamWebhookReceiver{}
	_ cloudflareCapabilityWrapperRole = AccountID{}
	_ cloudflareCapabilityWrapperRole = ImageID{}
	_ cloudflareCapabilityWrapperRole = StreamVideoID{}
	_ cloudflareCapabilityWrapperRole = ImagesServer{}
	_ cloudflareCapabilityWrapperRole = ImageUpload{}
	_ cloudflareCapabilityWrapperRole = ImagesClient{}
	_ cloudflareCapabilityWrapperRole = StreamServer{}
	_ cloudflareCapabilityWrapperRole = StreamUpload{}
	_ cloudflareCapabilityWrapperRole = StreamClient{}
	_ cloudflareCapabilityWrapperRole = R2Bucket{}
	_ cloudflareCapabilityWrapperRole = R2Key{}
	_ cloudflareCapabilityWrapperRole = R2Credentials{}
	_ cloudflareCapabilityWrapperRole = R2Server{}
	_ cloudflareCapabilityWrapperRole = R2Grant{}
	_ cloudflareCapabilityWrapperRole = R2Client{}
)
