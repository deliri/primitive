package github

type sealedValue interface{ githubSealedValue() }
type protocolFact interface{ githubProtocolFact() }
type internalFlow interface{ githubInternalFlow() }
type capabilityWrapper interface{ githubCapabilityWrapper() }

func (Repository) githubSealedValue()     {}
func (UserAgent) githubSealedValue()      {}
func (Reference) githubSealedValue()      {}
func (AppID) githubSealedValue()          {}
func (InstallationID) githubSealedValue() {}

func (Tag) githubProtocolFact()                   {}
func (TagPageRequest) githubProtocolFact()        {}
func (TagPage) githubProtocolFact()               {}
func (HeadRequest) githubProtocolFact()           {}
func (HeadObservation) githubProtocolFact()       {}
func (FileRequest) githubProtocolFact()           {}
func (FileObservation) githubProtocolFact()       {}
func (TarArchiveRequest) githubProtocolFact()     {}
func (TarArchiveObservation) githubProtocolFact() {}
func (TreeEntry) githubProtocolFact()             {}
func (TreeRequest) githubProtocolFact()           {}
func (TreeObservation) githubProtocolFact()       {}

func (credentialState) githubInternalFlow()    {}
func (installationToken) githubInternalFlow()  {}
func (clientConstruction) githubInternalFlow() {}
func (*clientState) githubInternalFlow()       {}
func (jwtHeader) githubInternalFlow()          {}
func (jwtUnixSeconds) githubInternalFlow()     {}
func (jwtClaims) githubInternalFlow()          {}
func (tagCommitWire) githubInternalFlow()      {}
func (tagWire) githubInternalFlow()            {}
func (headWire) githubInternalFlow()           {}
func (contentsLinksWire) githubInternalFlow()  {}
func (contentsWire) githubInternalFlow()       {}
func (boundedRequest) githubInternalFlow()     {}
func (treeEntryWire) githubInternalFlow()      {}
func (treeDecodeState) githubInternalFlow()    {}
func (treeDownloadResult) githubInternalFlow() {}
func (treeDownloadCall) githubInternalFlow()   {}
func (treeDecoder) githubInternalFlow()        {}
func (archiveDestination) githubInternalFlow() {}

func (AppCredential) githubCapabilityWrapper() {}
func (Client) githubCapabilityWrapper()        {}

var (
	_ sealedValue       = Repository{}
	_ sealedValue       = UserAgent{}
	_ sealedValue       = Reference{}
	_ sealedValue       = AppID{}
	_ sealedValue       = InstallationID{}
	_ protocolFact      = Tag{}
	_ protocolFact      = TagPageRequest{}
	_ protocolFact      = TagPage{}
	_ protocolFact      = HeadRequest{}
	_ protocolFact      = HeadObservation{}
	_ protocolFact      = FileRequest{}
	_ protocolFact      = FileObservation{}
	_ protocolFact      = TarArchiveRequest{}
	_ protocolFact      = TarArchiveObservation{}
	_ protocolFact      = TreeEntry{}
	_ protocolFact      = TreeRequest{}
	_ protocolFact      = TreeObservation{}
	_ internalFlow      = credentialState{}
	_ internalFlow      = installationToken{}
	_ internalFlow      = clientConstruction{}
	_ internalFlow      = (*clientState)(nil)
	_ internalFlow      = jwtHeader{}
	_ internalFlow      = jwtUnixSeconds(0)
	_ internalFlow      = jwtClaims{}
	_ internalFlow      = tagCommitWire{}
	_ internalFlow      = tagWire{}
	_ internalFlow      = headWire{}
	_ internalFlow      = contentsLinksWire{}
	_ internalFlow      = contentsWire{}
	_ internalFlow      = boundedRequest{}
	_ internalFlow      = treeEntryWire{}
	_ internalFlow      = treeDecodeState{}
	_ internalFlow      = treeDownloadResult{}
	_ internalFlow      = treeDownloadCall{}
	_ internalFlow      = treeDecoder{}
	_ internalFlow      = archiveDestination{}
	_ capabilityWrapper = AppCredential{}
	_ capabilityWrapper = Client{}
)
