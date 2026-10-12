package gitrepo

type sealedValue interface{ gitRepositorySealedValue() }
type protocolFact interface{ gitRepositoryProtocolFact() }
type internalFlow interface{ gitRepositoryInternalFlow() }
type capabilityWrapper interface{ gitRepositoryCapabilityWrapper() }

func (WorktreeSelection) gitRepositorySealedValue() {}

func (Configuration) gitRepositoryProtocolFact()   {}
func (WorktreeRequest) gitRepositoryProtocolFact() {}
func (StatusRequest) gitRepositoryProtocolFact()   {}
func (HeadRequest) gitRepositoryProtocolFact()     {}
func (HeadObservation) gitRepositoryProtocolFact() {}
func (WorktreeEntry) gitRepositoryProtocolFact()   {}
func (WorktreeSummary) gitRepositoryProtocolFact() {}

func (worktreeEntryWriter) gitRepositoryInternalFlow() {}
func (headOutput) gitRepositoryInternalFlow()          {}

func (Capability) gitRepositoryCapabilityWrapper() {}

var (
	_ sealedValue       = WorktreeSelectionUnknown
	_ protocolFact      = Configuration{}
	_ protocolFact      = WorktreeRequest{}
	_ protocolFact      = StatusRequest{}
	_ protocolFact      = HeadRequest{}
	_ protocolFact      = HeadObservation{}
	_ protocolFact      = WorktreeEntry{}
	_ protocolFact      = WorktreeSummary{}
	_ internalFlow      = worktreeEntryWriter{}
	_ internalFlow      = headOutput{}
	_ capabilityWrapper = Capability{}
)
