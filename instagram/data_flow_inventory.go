package instagram

type protocolFact interface{ instagramProtocolFact() }
type internalFlow interface{ instagramInternalFlow() }
type capabilityWrapper interface{ instagramCapabilityWrapper() }

func (UserID) instagramProtocolFact()              {}
func (MediaListRequest) instagramProtocolFact()    {}
func (Media) instagramProtocolFact()               {}
func (MediaPage) instagramProtocolFact()           {}
func (mediaListWire) instagramInternalFlow()       {}
func (mediaWire) instagramInternalFlow()           {}
func (mediaPagingWire) instagramInternalFlow()     {}
func (mediaCursorsWire) instagramInternalFlow()    {}
func (metaErrorWire) instagramInternalFlow()       {}
func (metaErrorDetailWire) instagramInternalFlow() {}
func (Credential) instagramCapabilityWrapper()     {}
func (Client) instagramCapabilityWrapper()         {}

var (
	_ protocolFact      = MediaPage{}
	_ internalFlow      = mediaListWire{}
	_ capabilityWrapper = Client{}
)
