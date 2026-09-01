package runworkspace

type capabilityWrapper interface{ runWorkspaceCapabilityWrapper() }
type protocolFact interface{ runWorkspaceProtocolFact() }
type internalFlow interface{ runWorkspaceInternalFlow() }

func (Configuration) runWorkspaceProtocolFact()                   {}
func (Unit) runWorkspaceProtocolFact()                            {}
func (Member) runWorkspaceProtocolFact()                          {}
func (Experiment) runWorkspaceProtocolFact()                      {}
func (Residue) runWorkspaceProtocolFact()                         {}
func (VerifiedSource) runWorkspaceProtocolFact()                  {}
func (SourceArchiveAcquisitionRequest) runWorkspaceProtocolFact() {}
func (CaptureEvidence) runWorkspaceProtocolFact()                 {}
func (ArtifactEvidence) runWorkspaceProtocolFact()                {}
func (GoDeclaration) runWorkspaceProtocolFact()                   {}
func (GoDiscoveredDeclaration) runWorkspaceProtocolFact()         {}
func (GoDiscovery) runWorkspaceProtocolFact()                     {}
func (GoFileDiscoveryRequest) runWorkspaceProtocolFact()          {}
func (GoPackageDiscoveredDeclaration) runWorkspaceProtocolFact()  {}
func (GoPackageDiscovery) runWorkspaceProtocolFact()              {}
func (GoPackageDiscoveryRequest) runWorkspaceProtocolFact()       {}
func (ResidueProbe) runWorkspaceProtocolFact()                    {}
func (LinuxResidueConfiguration) runWorkspaceProtocolFact()       {}
func (Manager) runWorkspaceCapabilityWrapper()                    {}
func (Capture) runWorkspaceCapabilityWrapper()                    {}
func (ProcessResidueSource) runWorkspaceCapabilityWrapper()       {}
func (LinuxResidueSource) runWorkspaceCapabilityWrapper()         {}
func (SourceDownloadRequest) runWorkspaceInternalFlow()           {}
func (sourceExtraction) runWorkspaceInternalFlow()                {}
func (goDeclarationCandidate) runWorkspaceInternalFlow()          {}
func (sourceFileWrite) runWorkspaceInternalFlow()                 {}
func (treeEntry) runWorkspaceInternalFlow()                       {}

var (
	_ protocolFact      = Configuration{}
	_ protocolFact      = Unit{}
	_ protocolFact      = Member{}
	_ protocolFact      = Experiment{}
	_ protocolFact      = Residue{}
	_ protocolFact      = VerifiedSource{}
	_ protocolFact      = SourceArchiveAcquisitionRequest{}
	_ protocolFact      = CaptureEvidence{}
	_ protocolFact      = ArtifactEvidence{}
	_ protocolFact      = GoDeclaration{}
	_ protocolFact      = GoDiscoveredDeclaration{}
	_ protocolFact      = GoDiscovery{}
	_ protocolFact      = GoFileDiscoveryRequest{}
	_ protocolFact      = GoPackageDiscoveredDeclaration{}
	_ protocolFact      = GoPackageDiscovery{}
	_ protocolFact      = GoPackageDiscoveryRequest{}
	_ protocolFact      = ResidueProbe{}
	_ protocolFact      = LinuxResidueConfiguration{}
	_ capabilityWrapper = Manager{}
	_ capabilityWrapper = Capture{}
	_ capabilityWrapper = ProcessResidueSource{}
	_ capabilityWrapper = LinuxResidueSource{}
	_ internalFlow      = SourceDownloadRequest{}
	_ internalFlow      = sourceExtraction{}
	_ internalFlow      = goDeclarationCandidate{}
	_ internalFlow      = sourceFileWrite{}
	_ internalFlow      = treeEntry{}
)
