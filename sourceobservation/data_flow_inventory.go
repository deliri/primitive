package sourceobservation

// These roles keep observed facts distinct from the sealed file, package,
// and project projections that index them.
type protocolFact interface{ sourceObservationProtocolFact() }
type sealedProjection interface{ sourceObservationSealedProjection() }
type internalFlow interface{ sourceObservationInternalFlow() }

func (ContextID) sourceObservationProtocolFact()        {}
func (Language) sourceObservationProtocolFact()         {}
func (Symbol) sourceObservationProtocolFact()           {}
func (ImportPath) sourceObservationProtocolFact()       {}
func (EffectName) sourceObservationProtocolFact()       {}
func (Toolchain) sourceObservationProtocolFact()        {}
func (BuildContext) sourceObservationProtocolFact()     {}
func (BuildSelection) sourceObservationProtocolFact()   {}
func (Declaration) sourceObservationProtocolFact()      {}
func (Import) sourceObservationProtocolFact()           {}
func (Effect) sourceObservationProtocolFact()           {}
func (Reference) sourceObservationProtocolFact()        {}
func (FileReference) sourceObservationProtocolFact()    {}
func (PackageReference) sourceObservationProtocolFact() {}

func (fileMembershipConsumer) sourceObservationInternalFlow()    {}
func (packageMembershipConsumer) sourceObservationInternalFlow() {}
func (packageVerifier) sourceObservationInternalFlow()           {}
func (packageFileVerifier) sourceObservationInternalFlow()       {}
func (projectFileVerifier) sourceObservationInternalFlow()       {}

func (Summary) sourceObservationSealedProjection() {}

func (FileMembership) sourceObservationSealedProjection()    {}
func (PackageMembership) sourceObservationSealedProjection() {}
func (File) sourceObservationSealedProjection()              {}
func (Package) sourceObservationSealedProjection()           {}
func (Project) sourceObservationSealedProjection()           {}

var (
	_ protocolFact     = BuildContext{}
	_ sealedProjection = Project{}
	_ internalFlow     = fileMembershipConsumer{}
)
