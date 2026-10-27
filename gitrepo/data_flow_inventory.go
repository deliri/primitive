package gitrepo

type sealedValue interface{ gitRepositorySealedValue() }
type protocolFact interface{ gitRepositoryProtocolFact() }
type internalFlow interface{ gitRepositoryInternalFlow() }
type capabilityWrapper interface{ gitRepositoryCapabilityWrapper() }

func (WorktreeSelection) gitRepositorySealedValue()    {}
func (StatusObservation) gitRepositorySealedValue()    {}
func (ReferenceLabel) gitRepositorySealedValue()       {}
func (RootRequest) gitRepositoryProtocolFact()         {}
func (RootObservation) gitRepositoryProtocolFact()     {}
func (BranchRequest) gitRepositoryProtocolFact()       {}
func (BranchObservation) gitRepositoryProtocolFact()   {}
func (ExactTagRequest) gitRepositoryProtocolFact()     {}
func (ExactTagObservation) gitRepositoryProtocolFact() {}
func (CommandExitError) gitRepositoryInternalFlow()    {}

func (Configuration) gitRepositoryProtocolFact()     {}
func (WorktreeRequest) gitRepositoryProtocolFact()   {}
func (StatusRequest) gitRepositoryProtocolFact()     {}
func (HeadRequest) gitRepositoryProtocolFact()       {}
func (HeadObservation) gitRepositoryProtocolFact()   {}
func (WorktreeEntry) gitRepositoryProtocolFact()     {}
func (WorktreeSummary) gitRepositoryProtocolFact()   {}
func (InventoryRequest) gitRepositoryProtocolFact()  {}
func (ReferenceEntry) gitRepositoryProtocolFact()    {}
func (WorktreeListEntry) gitRepositoryProtocolFact() {}

func (worktreeEntryWriter) gitRepositoryInternalFlow() {}
func (recordWriter) gitRepositoryInternalFlow()        {}
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
