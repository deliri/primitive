package googleidentity

type protocolFact interface{ googleIdentityProtocolFact() }
type sealedProjection interface{ googleIdentitySealedProjection() }
type internalFlow interface{ googleIdentityInternalFlow() }
type capabilityWrapper interface{ googleIdentityCapabilityWrapper() }

func (Audience) googleIdentityProtocolFact()                         {}
func (Policy) googleIdentityProtocolFact()                           {}
func (IdentityTokenRequest) googleIdentityProtocolFact()             {}
func (GoogleCloudAccessTokenRequest) googleIdentityProtocolFact()    {}
func (GoogleCloudVerifierConfiguration) googleIdentityProtocolFact() {}

func (GoogleCloudVerifiedIdentity) googleIdentitySealedProjection() {}

func (acquisitionCall) googleIdentityInternalFlow()            {}
func (googleAccessTokenResponse) googleIdentityInternalFlow()  {}
func (googleAccessTokenContracts) googleIdentityInternalFlow() {}
func (googleProtocolContracts) googleIdentityInternalFlow()    {}

func (AccessToken) googleIdentityCapabilityWrapper()         {}
func (Client) googleIdentityCapabilityWrapper()              {}
func (Token) googleIdentityCapabilityWrapper()               {}
func (GoogleCloudVerifier) googleIdentityCapabilityWrapper() {}

var (
	_ protocolFact      = Audience{}
	_ sealedProjection  = GoogleCloudVerifiedIdentity{}
	_ internalFlow      = acquisitionCall{}
	_ capabilityWrapper = AccessToken{}
)
