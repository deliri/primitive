package awsidentity

type protocolFact interface{ awsIdentityProtocolFact() }
type internalFlow interface{ awsIdentityInternalFlow() }
type capabilityWrapper interface{ awsIdentityCapabilityWrapper() }
type typedFailure interface{ awsIdentityTypedFailure() }

func (Audience) awsIdentityProtocolFact()     {}
func (Policy) awsIdentityProtocolFact()       {}
func (RequestInput) awsIdentityProtocolFact() {}

func (acquisitionCall) awsIdentityInternalFlow()         {}
func (amazonResponse) awsIdentityInternalFlow()          {}
func (amazonResult) awsIdentityInternalFlow()            {}
func (amazonUnexpectedElement) awsIdentityInternalFlow() {}
func (amazonTokenElement) awsIdentityInternalFlow()      {}
func (amazonExpirationElement) awsIdentityInternalFlow() {}
func (amazonResponseMetadata) awsIdentityInternalFlow()  {}
func (amazonRequestIDElement) awsIdentityInternalFlow()  {}

func (Client) awsIdentityCapabilityWrapper()  {}
func (Request) awsIdentityCapabilityWrapper() {}
func (Token) awsIdentityCapabilityWrapper()   {}

func (requestError) awsIdentityTypedFailure() {}

var (
	_ protocolFact      = Audience{}
	_ internalFlow      = acquisitionCall{}
	_ capabilityWrapper = Client{}
	_ typedFailure      = requestError{}
)
