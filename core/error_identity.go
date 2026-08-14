package core

import (
	"errors"
)

const unknownErrorIdentityText = "unknown primitive error identity"

const (
	// errorIdentityMaximumParents is the compile-time parent arity of an identity.
	errorIdentityMaximumParents = 2
	errorIdentityVisitWordBits  = 64
	// errorIdentityVisitWords sizes the traversal's visited set from the closed
	// domain instead of fixing it. A hand-written word count is a ceiling
	// somebody eventually reaches, and reaching it is a compile error in a file
	// nobody was editing; deriving it means adding an identity can never
	// outgrow the set that has to track it.
	errorIdentityVisitWords = (int(errorIdentityLimit) + errorIdentityVisitWordBits - 1) /
		errorIdentityVisitWordBits
)

// errorIdentityVisitSet marks which identities a single Matches traversal has
// already enqueued. It is a value on the stack, sized by the compiler, so the
// traversal allocates nothing and cannot outlive the call.
type errorIdentityVisitSet [errorIdentityVisitWords]uint64

// mark records identity and reports whether this call was the one that first
// visited it. Returning that fact is what keeps a duplicate parent from
// consuming stack capacity in the traversal below.
func (v *errorIdentityVisitSet) mark(identity ErrorIdentity) bool {
	index := uint(identity)
	word := index / errorIdentityVisitWordBits
	if word >= uint(len(v)) {
		return false
	}
	mask := uint64(1) << (index % errorIdentityVisitWordBits)
	wasUnvisited := v[word]&mask == 0
	v[word] |= mask
	return wasUnvisited
}

type errorIdentityParentSet struct {
	values [errorIdentityMaximumParents]ErrorIdentity
	count  uint8
}

// ErrorIdentity is a stable, closed error identity usable with errors.Is.
type ErrorIdentity uint16

const (
	// ErrUnknown is the invalid zero identity.
	ErrUnknown ErrorIdentity = iota
	// ErrPrimitiveContract identifies a shared Primitive contract violation.
	ErrPrimitiveContract
	// ErrJSONContract identifies strict JSON boundary failure.
	ErrJSONContract
	// ErrNumericOverflow identifies a checked numeric overflow.
	ErrNumericOverflow
	// ErrSecretMaterialAllZero identifies secret material whose bytes are all
	// zero. Core owns this rejection, so a caller that must distinguish a failed
	// entropy source from a structural violation asks Core through errors.Is
	// instead of re-deriving the rule over the same buffer.
	ErrSecretMaterialAllZero

	// ErrAttestContract identifies an attest input or state violation.
	ErrAttestContract
	// ErrAttestVerification identifies failed attestation verification.
	ErrAttestVerification

	// ErrContextStateContract identifies a context-state contract violation.
	ErrContextStateContract
	// ErrNilContext identifies a required context that is nil.
	ErrNilContext
	// ErrContextObservation identifies failed context observation.
	ErrContextObservation

	// ErrCurrencyContract identifies a currency contract violation.
	ErrCurrencyContract
	// ErrCurrencyMismatch identifies incompatible currencies.
	ErrCurrencyMismatch
	// ErrCurrencyOverflow identifies currency arithmetic overflow.
	ErrCurrencyOverflow
	// ErrCurrencyDecimal identifies rejected decimal currency input.
	ErrCurrencyDecimal

	// ErrGarbleContract identifies a garble contract violation.
	ErrGarbleContract
	// ErrGarbleDerivation identifies failed garble derivation.
	ErrGarbleDerivation
	// ErrGarbleBuildIntent identifies rejected typed Garble build intent.
	ErrGarbleBuildIntent

	// ErrKeygenContract identifies a key-generation contract violation.
	ErrKeygenContract
	// ErrKeygenEntropy identifies failed key-generation entropy acquisition.
	ErrKeygenEntropy

	// ErrTestIsolationContract identifies a test-isolation violation.
	ErrTestIsolationContract

	// ErrFilestoreContract identifies a file-store contract violation.
	ErrFilestoreContract
	// ErrFilestoreSize identifies a rejected file-store size.
	ErrFilestoreSize
	// ErrFilestoreSource identifies a file-store source failure.
	ErrFilestoreSource
	// ErrFilestoreDestination identifies a file-store destination failure.
	ErrFilestoreDestination
	// ErrFilestoreConflict identifies a file-store namespace conflict.
	ErrFilestoreConflict
	// ErrFilestoreActivation identifies failed file-store activation.
	ErrFilestoreActivation
	// ErrFilestoreActivationIndeterminate identifies uncertain activation state.
	ErrFilestoreActivationIndeterminate
	// ErrFilestoreCleanup identifies failed file-store cleanup.
	ErrFilestoreCleanup

	// ErrHostFacts identifies the neutral host-facts error family.
	ErrHostFacts
	// ErrHostFactsContract identifies a host-facts contract violation.
	ErrHostFactsContract
	// ErrHostFactsObservation identifies a failed host observation.
	ErrHostFactsObservation
	// ErrHostFactsUnsupported identifies an unsupported host observation.
	ErrHostFactsUnsupported
	// ErrHostFactsPressure identifies a reached caller-supplied pressure policy.
	ErrHostFactsPressure
	// ErrHostFactsEvidence identifies invalid persisted host evidence.
	ErrHostFactsEvidence
	// ErrDiskCapacityUnsupported identifies unsupported disk-capacity observation.
	ErrDiskCapacityUnsupported
	// ErrTreeMeasurementUnsupported identifies unsupported tree measurement.
	ErrTreeMeasurementUnsupported
	// ErrDiskFloorReached identifies insufficient available disk capacity.
	ErrDiskFloorReached
	// ErrMemoryLimitReached identifies reached Go-managed-memory pressure.
	ErrMemoryLimitReached

	// ErrTemporalContract identifies a temporal contract violation.
	ErrTemporalContract
	// ErrTemporalOverflow identifies temporal arithmetic outside its exact
	// representable domain.
	ErrTemporalOverflow

	// ErrExchangeContract identifies an exchange contract violation.
	ErrExchangeContract
	// ErrExchangeRequest identifies a rejected exchange request.
	ErrExchangeRequest
	// ErrExchangeResponse identifies a rejected exchange response.
	ErrExchangeResponse
	// ErrExchangeBodyLimit identifies an exceeded exchange body limit.
	ErrExchangeBodyLimit
	// ErrExchangeContentType identifies a rejected exchange content type.
	ErrExchangeContentType
	// ErrExchangeCancelled identifies exchange cancellation.
	ErrExchangeCancelled
	// ErrExchangeRedirect identifies a rejected redirect.
	ErrExchangeRedirect
	// ErrExchangeTransport identifies a transport failure.
	ErrExchangeTransport
	// ErrExchangeRetryExhausted identifies exhausted retry policy.
	ErrExchangeRetryExhausted
	// ErrExchangeWrite identifies an exchange write failure.
	ErrExchangeWrite

	// ErrFuzzFinderContract identifies a fuzz-finder contract violation.
	ErrFuzzFinderContract
	// ErrFuzzFinderFormat identifies an unsupported Go fuzz-artifact format.
	ErrFuzzFinderFormat
	// ErrFuzzFinderObservation identifies failed fuzz-artifact observation.
	ErrFuzzFinderObservation

	// ErrLeaseContract identifies a lease contract violation.
	ErrLeaseContract
	// ErrLeaseVerification identifies failed lease verification.
	ErrLeaseVerification
	// ErrLeaseRollback identifies rejected lease rollback.
	ErrLeaseRollback
	// ErrLeaseConflict identifies a lease identity conflict.
	ErrLeaseConflict
	// ErrLeaseScope identifies a verified lease for a different subject.
	ErrLeaseScope
	// ErrLeaseClock identifies a local clock contradiction.
	ErrLeaseClock
	// ErrLeaseProduct identifies an offering/product projection that is absent
	// from or contradictory within the published Lease product catalog.
	ErrLeaseProduct

	// ErrGateContract identifies a new-work Gate contract violation.
	ErrGateContract
	// ErrGateDenied identifies an authentic Lease state that denies new work.
	ErrGateDenied

	// ErrProcessContract identifies a process contract violation.
	ErrProcessContract
	// ErrProcessStart identifies failure to start a process.
	ErrProcessStart
	// ErrProcessStream identifies a process stream failure.
	ErrProcessStream
	// ErrProcessOutputLimit identifies a reached process output bound.
	ErrProcessOutputLimit
	// ErrProcessWait identifies failure while waiting for a process.
	ErrProcessWait
	// ErrProcessObservation identifies a failed process observation.
	ErrProcessObservation
	// ErrProcessUnsupported identifies an unsupported process operation.
	ErrProcessUnsupported

	// ErrReleaseContract identifies a release contract violation.
	ErrReleaseContract
	// ErrReleaseManifest identifies a rejected release manifest.
	ErrReleaseManifest
	// ErrReleaseVerification identifies failed release verification.
	ErrReleaseVerification
	// ErrReleaseLatest identifies a rejected latest-release decision.
	ErrReleaseLatest
	// ErrReleaseRollback identifies rejected release rollback.
	ErrReleaseRollback
	// ErrReleaseConflict identifies a release identity conflict.
	ErrReleaseConflict
	// ErrDeployContract identifies a release-deployment contract violation.
	ErrDeployContract
	// ErrDistributionContract identifies an authenticated software-distribution
	// agreement violation.
	ErrDistributionContract
	// ErrDistributionVerification identifies a distribution document or nested
	// release authority that failed authentication.
	ErrDistributionVerification
	// ErrDistributionBinding identifies a valid distribution fact attached to
	// the wrong request, capability, release, or lifetime.
	ErrDistributionBinding

	// ErrShutdownContract identifies a shutdown contract violation.
	ErrShutdownContract
	// ErrShutdownStepFailure identifies a cleanup step that returned failure.
	ErrShutdownStepFailure
	// ErrShutdownStepTimeout identifies a cleanup step whose budget expired.
	ErrShutdownStepTimeout
	// ErrShutdownStepPanic identifies a cleanup step whose panic was contained.
	ErrShutdownStepPanic
	// ErrShutdownTotalTimeout identifies cleanup work stopped or skipped by
	// total budget expiry.
	ErrShutdownTotalTimeout
	// ErrShutdownSignalSource identifies failed signal observation.
	ErrShutdownSignalSource
	// ErrShutdownSignalReceived identifies authentic observed shutdown signal.
	ErrShutdownSignalReceived

	// ErrObjectStoreContract identifies an object-store contract violation.
	ErrObjectStoreContract
	// ErrObjectStoreExpired identifies an expired object-store target.
	ErrObjectStoreExpired
	// ErrObjectStoreIntegrity identifies failed object integrity.
	ErrObjectStoreIntegrity
	// ErrObjectStoreSource identifies an object-store source failure.
	ErrObjectStoreSource
	// ErrObjectStoreDestination identifies an object-store destination failure.
	ErrObjectStoreDestination
	// ErrObjectStoreConflict identifies an object-store write conflict.
	ErrObjectStoreConflict
	// ErrObjectStoreSize identifies a rejected object size.
	ErrObjectStoreSize
	// ErrObjectStoreAbsent identifies an absent object.
	ErrObjectStoreAbsent

	// ErrTimeProofContract identifies a time-proof contract violation.
	ErrTimeProofContract
	// ErrTimeProofRefused identifies a valid authority refusal.
	ErrTimeProofRefused
	// ErrTimeProofInvalid identifies evidence that failed verification.
	ErrTimeProofInvalid
	// ErrCloudIdentityContract identifies a cloud-identity violation.
	ErrCloudIdentityContract
	// ErrUpgradeContract identifies an upgrade contract violation.
	ErrUpgradeContract
	// ErrUpgradeDownload identifies a candidate download failure.
	ErrUpgradeDownload
	// ErrUpgradeCapacity identifies insufficient admitted candidate capacity.
	ErrUpgradeCapacity
	// ErrUpgradeVerification identifies candidate or primary bytes that do not
	// match their authenticated Release artifact.
	ErrUpgradeVerification
	// ErrUpgradeTrial identifies a candidate rejected by its product-owned
	// trial.
	ErrUpgradeTrial
	// ErrUpgradePromotion identifies a rejected primary-selection change.
	ErrUpgradePromotion
	// ErrUpgradePersistence identifies unreadable or uncommitted upgrade
	// metadata.
	ErrUpgradePersistence
	// ErrUpgradeCleanup identifies an obsolete slot that could not be removed.
	ErrUpgradeCleanup
	// ErrUpgradeConflict identifies concurrent or stale upgrade authority.
	ErrUpgradeConflict

	// ErrLifecycleIdentityContract identifies an invalid lifecycle identity.
	ErrLifecycleIdentityContract
	// ErrReceiptContract identifies a Receipt contract violation.
	ErrReceiptContract
	// ErrReceiptVerification identifies evidence that failed authentication.
	ErrReceiptVerification
	// ErrReceiptScope identifies authentic evidence for a different expected scope.
	ErrReceiptScope
	// ErrReceiptRollback identifies a rejected watermark rollback.
	ErrReceiptRollback
	// ErrReceiptConflict identifies incompatible watermark histories or scopes.
	ErrReceiptConflict
	// ErrChitContract identifies an invalid customer custody ticket, manifest,
	// catalog snapshot, or selection.
	ErrChitContract
	// ErrChitVerification identifies a chit or catalog fact that failed
	// authority authentication.
	ErrChitVerification
	// ErrChitConflict identifies contradictory collection versions, manifest
	// ordering, catalog pagination, or custody state.
	ErrChitConflict
	// ErrRetrievalContract identifies a download authorization contract failure.
	ErrRetrievalContract
	// ErrRetrievalBinding identifies a signed retrieval grant that does not bind
	// its exact request, object, capability, or lifetime.
	ErrRetrievalBinding
	// ErrPaymentContract identifies an invalid payment-history fact or catalog.
	ErrPaymentContract
	// ErrPaymentVerification identifies payment history that failed authority
	// authentication.
	ErrPaymentVerification

	// ErrControlWireContract identifies a control-wire scalar contract violation.
	ErrControlWireContract
	// ErrControlWireRevision identifies an unsupported control-wire revision.
	ErrControlWireRevision
	// ErrControlWireNonce identifies a rejected control-wire request nonce.
	ErrControlWireNonce
	// ErrControlWireToken identifies a rejected control-wire registration token.
	ErrControlWireToken
	// ErrControlWirePolicyCursor identifies a rejected control-wire policy cursor.
	ErrControlWirePolicyCursor
	// ErrControlWireRoute identifies a rejected control-plane route contract.
	ErrControlWireRoute
	// ErrControlWireProtocolSupport identifies an invalid support set or a
	// compatibility assessment that does not name its exact route/revision pair.
	ErrControlWireProtocolSupport
	// ErrControlWireReplayConflict identifies reuse of one request nonce for a
	// different canonical control request.
	ErrControlWireReplayConflict
	// ErrControlPlaneContract identifies a control-plane document contract violation.
	ErrControlPlaneContract
	// ErrControlPlaneSigningDomain identifies a rejected control-plane signing domain.
	ErrControlPlaneSigningDomain
	// ErrControlPlaneProductStatus identifies a rejected commercial product status.
	ErrControlPlaneProductStatus
	// ErrControlPlaneUsageWatermark identifies a rejected usage watermark.
	ErrControlPlaneUsageWatermark
	// ErrControlPlaneResponseHeader identifies a rejected response header.
	ErrControlPlaneResponseHeader
	// ErrControlPlaneResponseDocument identifies a rejected authenticated
	// response document.
	ErrControlPlaneResponseDocument
	// ErrControlPlaneResponseBinding identifies a response that does not bind to
	// the exact request that produced it.
	ErrControlPlaneResponseBinding
	// ErrControlPlaneUpgradeRequired identifies an authentic authority refusal
	// requiring the caller to upgrade before it can consume a product body.
	ErrControlPlaneUpgradeRequired
	// ErrControlPlaneProviderTimeRollback identifies an authority instant that
	// moved backward from a previously trusted one.
	ErrControlPlaneProviderTimeRollback
	// ErrControlPlaneRegistration identifies a rejected registration document.
	ErrControlPlaneRegistration
	// ErrControlPlaneInstallationBinding identifies an installation identity that
	// its own device key does not derive.
	ErrControlPlaneInstallationBinding
	// ErrControlPlaneDecisionConsistency identifies signed facts that disagree
	// with each other inside one authenticated document.
	ErrControlPlaneDecisionConsistency
	// ErrControlPlaneCheckIn identifies a rejected check-in request document.
	ErrControlPlaneCheckIn
	// ErrControlPlaneCheckInResponse identifies a rejected check-in response.
	ErrControlPlaneCheckInResponse
	// ErrControlPlaneUsageWindow identifies a rejected reported usage window.
	ErrControlPlaneUsageWindow

	// ErrFileLockUnavailable identifies a file-lock effect the operating system
	// refused for a reason other than contention. Contention is a typed outcome
	// rather than an error, so this is how a caller tells a filesystem that
	// cannot lock at all apart from another process that is simply running.
	// Contract violations in that package use ErrPrimitiveContract, because the
	// identity space is one slot from its compiler-witnessed ceiling and this is
	// the distinction a caller acts on at run time.
	ErrFileLockUnavailable

	// ErrIDContract identifies a time-ordered identifier contract violation.
	ErrIDContract

	errorIdentityLimit
)

// The visited set covers the whole closed domain by construction of the
// ceiling division above. This witness says so to the compiler anyway, so that
// changing the derivation to something that does not cover it is a build
// failure rather than a traversal that silently stops marking.
var _ [errorIdentityVisitWords*errorIdentityVisitWordBits - int(errorIdentityLimit)]struct{}

// errorIdentityDiagnostic binds one ordinal to its stable text. Keeping both
// in one unkeyed table makes addition a compile error and makes reordering an
// explicit row-identity failure instead of silently changing dispatch.
type errorIdentityDiagnostic struct {
	text     string
	identity ErrorIdentity
}

func errorIdentityDiagnostics() [errorIdentityLimit]errorIdentityDiagnostic {
	return [...]errorIdentityDiagnostic{
		{identity: ErrUnknown, text: unknownErrorIdentityText},
		{identity: ErrPrimitiveContract, text: "primitive contract violation"},
		{identity: ErrJSONContract, text: "json contract violation"},
		{identity: ErrNumericOverflow, text: "numeric overflow"},
		{identity: ErrSecretMaterialAllZero, text: "secret material is all zero"},
		{identity: ErrAttestContract, text: "attest contract violation"},
		{identity: ErrAttestVerification, text: "attest verification failed"},
		{identity: ErrContextStateContract, text: "context state contract violation"},
		{identity: ErrNilContext, text: "nil context"},
		{identity: ErrContextObservation, text: "context observation failed"},
		{identity: ErrCurrencyContract, text: "currency contract violation"},
		{identity: ErrCurrencyMismatch, text: "currency mismatch"},
		{identity: ErrCurrencyOverflow, text: "currency overflow"},
		{identity: ErrCurrencyDecimal, text: "currency decimal rejected"},
		{identity: ErrGarbleContract, text: "garble contract violation"},
		{identity: ErrGarbleDerivation, text: "garble derivation failed"},
		{identity: ErrGarbleBuildIntent, text: "garble build intent rejected"},
		{identity: ErrKeygenContract, text: "key generation contract violation"},
		{identity: ErrKeygenEntropy, text: "key generation entropy failed"},
		{identity: ErrTestIsolationContract, text: "test isolation contract violation"},
		{identity: ErrFilestoreContract, text: "filestore contract violation"},
		{identity: ErrFilestoreSize, text: "filestore size rejected"},
		{identity: ErrFilestoreSource, text: "filestore source failed"},
		{identity: ErrFilestoreDestination, text: "filestore destination failed"},
		{identity: ErrFilestoreConflict, text: "filestore namespace conflict"},
		{identity: ErrFilestoreActivation, text: "filestore activation failed"},
		{identity: ErrFilestoreActivationIndeterminate, text: "filestore activation indeterminate"},
		{identity: ErrFilestoreCleanup, text: "filestore cleanup failed"},
		{identity: ErrHostFacts, text: "host facts failure"},
		{identity: ErrHostFactsContract, text: "host facts contract violation"},
		{identity: ErrHostFactsObservation, text: "host facts observation failed"},
		{identity: ErrHostFactsUnsupported, text: "host facts observation unsupported"},
		{identity: ErrHostFactsPressure, text: "host facts pressure reached"},
		{identity: ErrHostFactsEvidence, text: "host facts evidence invalid"},
		{identity: ErrDiskCapacityUnsupported, text: "disk capacity observation unsupported"},
		{identity: ErrTreeMeasurementUnsupported, text: "tree measurement unsupported"},
		{identity: ErrDiskFloorReached, text: "disk floor reached"},
		{identity: ErrMemoryLimitReached, text: "memory limit reached"},
		{identity: ErrTemporalContract, text: "temporal contract violation"},
		{identity: ErrTemporalOverflow, text: "temporal arithmetic overflow"},
		{identity: ErrExchangeContract, text: "exchange contract violation"},
		{identity: ErrExchangeRequest, text: "exchange request rejected"},
		{identity: ErrExchangeResponse, text: "exchange response rejected"},
		{identity: ErrExchangeBodyLimit, text: "exchange body limit exceeded"},
		{identity: ErrExchangeContentType, text: "exchange content type rejected"},
		{identity: ErrExchangeCancelled, text: "exchange cancelled"},
		{identity: ErrExchangeRedirect, text: "exchange redirect rejected"},
		{identity: ErrExchangeTransport, text: "exchange transport failed"},
		{identity: ErrExchangeRetryExhausted, text: "exchange retry budget exhausted"},
		{identity: ErrExchangeWrite, text: "exchange write failed"},
		{identity: ErrFuzzFinderContract, text: "fuzz finder contract violation"},
		{identity: ErrFuzzFinderFormat, text: "Go fuzz artifact format unsupported"},
		{identity: ErrFuzzFinderObservation, text: "fuzz artifact observation failed"},
		{identity: ErrLeaseContract, text: "lease contract violation"},
		{identity: ErrLeaseVerification, text: "lease verification failed"},
		{identity: ErrLeaseRollback, text: "lease rollback rejected"},
		{identity: ErrLeaseConflict, text: "lease identity conflict"},
		{identity: ErrLeaseScope, text: "lease subject mismatch"},
		{identity: ErrLeaseClock, text: "lease clock contradiction"},
		{identity: ErrLeaseProduct, text: "lease product projection rejected"},
		{identity: ErrGateContract, text: "gate contract violation"},
		{identity: ErrGateDenied, text: "gate denied new work"},
		{identity: ErrProcessContract, text: "process contract violation"},
		{identity: ErrProcessStart, text: "process start failed"},
		{identity: ErrProcessStream, text: "process stream failed"},
		{identity: ErrProcessOutputLimit, text: "process output limit exceeded"},
		{identity: ErrProcessWait, text: "process wait failed"},
		{identity: ErrProcessObservation, text: "process observation failed"},
		{identity: ErrProcessUnsupported, text: "process operation unsupported on this host"},
		{identity: ErrReleaseContract, text: "release contract violation"},
		{identity: ErrReleaseManifest, text: "release manifest rejected"},
		{identity: ErrReleaseVerification, text: "release verification failed"},
		{identity: ErrReleaseLatest, text: "release latest rejected"},
		{identity: ErrReleaseRollback, text: "release rollback rejected"},
		{identity: ErrReleaseConflict, text: "release identity conflict"},
		{identity: ErrDeployContract, text: "deploy contract violation"},
		{identity: ErrDistributionContract, text: "distribution contract violation"},
		{identity: ErrDistributionVerification, text: "distribution verification failed"},
		{identity: ErrDistributionBinding, text: "distribution binding failed"},
		{identity: ErrShutdownContract, text: "shutdown contract violation"},
		{identity: ErrShutdownStepFailure, text: "shutdown step failed"},
		{identity: ErrShutdownStepTimeout, text: "shutdown step timed out"},
		{identity: ErrShutdownStepPanic, text: "shutdown step panicked"},
		{identity: ErrShutdownTotalTimeout, text: "shutdown total budget expired"},
		{identity: ErrShutdownSignalSource, text: "shutdown signal source failed"},
		{identity: ErrShutdownSignalReceived, text: "shutdown signal received"},
		{identity: ErrObjectStoreContract, text: "object store contract violation"},
		{identity: ErrObjectStoreExpired, text: "object store target expired"},
		{identity: ErrObjectStoreIntegrity, text: "object store integrity failed"},
		{identity: ErrObjectStoreSource, text: "object store source failed"},
		{identity: ErrObjectStoreDestination, text: "object store destination failed"},
		{identity: ErrObjectStoreConflict, text: "object store write conflict"},
		{identity: ErrObjectStoreSize, text: "object store size rejected"},
		{identity: ErrObjectStoreAbsent, text: "object store object absent"},
		{identity: ErrTimeProofContract, text: "time proof contract violation"},
		{identity: ErrTimeProofRefused, text: "time proof authority refused"},
		{identity: ErrTimeProofInvalid, text: "time proof evidence invalid"},
		{identity: ErrCloudIdentityContract, text: "cloud identity contract violation"},
		{identity: ErrUpgradeContract, text: "upgrade contract violation"},
		{identity: ErrUpgradeDownload, text: "upgrade candidate download failed"},
		{identity: ErrUpgradeCapacity, text: "upgrade candidate capacity rejected"},
		{identity: ErrUpgradeVerification, text: "upgrade artifact verification failed"},
		{identity: ErrUpgradeTrial, text: "upgrade candidate trial failed"},
		{identity: ErrUpgradePromotion, text: "upgrade promotion failed"},
		{identity: ErrUpgradePersistence, text: "upgrade persistence failed"},
		{identity: ErrUpgradeCleanup, text: "upgrade obsolete slot cleanup failed"},
		{identity: ErrUpgradeConflict, text: "upgrade authority conflict"},
		{identity: ErrLifecycleIdentityContract, text: "lifecycle identity contract violation"},
		{identity: ErrReceiptContract, text: "receipt contract violation"},
		{identity: ErrReceiptVerification, text: "receipt verification failed"},
		{identity: ErrReceiptScope, text: "receipt scope mismatch"},
		{identity: ErrReceiptRollback, text: "receipt watermark rollback rejected"},
		{identity: ErrReceiptConflict, text: "receipt watermark conflict"},
		{identity: ErrChitContract, text: "chit contract violation"},
		{identity: ErrChitVerification, text: "chit verification failed"},
		{identity: ErrChitConflict, text: "chit conflict"},
		{identity: ErrRetrievalContract, text: "retrieval contract violation"},
		{identity: ErrRetrievalBinding, text: "retrieval grant binding failed"},
		{identity: ErrPaymentContract, text: "payment contract violation"},
		{identity: ErrPaymentVerification, text: "payment verification failed"},
		{identity: ErrControlWireContract, text: "control-wire contract violation"},
		{identity: ErrControlWireRevision, text: "control-wire revision unsupported"},
		{identity: ErrControlWireNonce, text: "control-wire request nonce invalid"},
		{identity: ErrControlWireToken, text: "control-wire registration token invalid"},
		{identity: ErrControlWirePolicyCursor, text: "control-wire policy cursor invalid"},
		{identity: ErrControlWireRoute, text: "control-wire route contract invalid"},
		{identity: ErrControlWireProtocolSupport, text: "control-wire protocol support contract invalid"},
		{identity: ErrControlWireReplayConflict, text: "control-wire replay identity conflict"},
		{identity: ErrControlPlaneContract, text: "control-plane contract violation"},
		{identity: ErrControlPlaneSigningDomain, text: "control-plane signing domain invalid"},
		{identity: ErrControlPlaneProductStatus, text: "control-plane product status invalid"},
		{identity: ErrControlPlaneUsageWatermark, text: "control-plane usage watermark invalid"},
		{identity: ErrControlPlaneResponseHeader, text: "control-plane response header invalid"},
		{identity: ErrControlPlaneResponseDocument, text: "control-plane response document invalid"},
		{identity: ErrControlPlaneResponseBinding, text: "control-plane response does not bind to its request"},
		{identity: ErrControlPlaneUpgradeRequired, text: "control-plane upgrade required"},
		{identity: ErrControlPlaneProviderTimeRollback, text: "control-plane provider time moved backward"},
		{identity: ErrControlPlaneRegistration, text: "control-plane registration document invalid"},
		{identity: ErrControlPlaneInstallationBinding, text: "control-plane installation binding invalid"},
		{identity: ErrControlPlaneDecisionConsistency, text: "control-plane decision facts disagree"},
		{identity: ErrControlPlaneCheckIn, text: "control-plane check-in request invalid"},
		{identity: ErrControlPlaneCheckInResponse, text: "control-plane check-in response invalid"},
		{identity: ErrControlPlaneUsageWindow, text: "control-plane usage window invalid"},
		{identity: ErrFileLockUnavailable, text: "filelock unavailable"},
		{identity: ErrIDContract, text: "id contract violation"},
	}
}

// Error returns the stable diagnostic text for i.
func (i ErrorIdentity) Error() string {
	if !isAdmittedErrorIdentity(i) {
		return unknownErrorIdentityText
	}
	diagnostic := errorIdentityDiagnostics()[i]
	if diagnostic.identity != i || diagnostic.text == "" {
		return unknownErrorIdentityText
	}
	return diagnostic.text
}

// String returns the stable diagnostic text for i.
func (i ErrorIdentity) String() string { return i.Error() }

// IsValid reports whether i belongs to the closed error domain.
func (i ErrorIdentity) IsValid() bool { return i.Validate() == nil }

// MarshalJSON emits the stable identity text as a JSON string.
func (i ErrorIdentity) MarshalJSON() ([]byte, error) {
	if err := i.Validate(); err != nil {
		return nil, errors.Join(ErrJSONContract, err)
	}
	return MarshalCanonicalJSONString(i.String())
}

// UnmarshalJSON accepts only the stable text of an admitted identity.
func (i *ErrorIdentity) UnmarshalJSON(data []byte) error {
	if i == nil {
		return errors.Join(ErrJSONContract, errorIdentityContractError("nil error identity receiver"))
	}
	value, err := DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	for identity := ErrPrimitiveContract; identity < errorIdentityLimit; identity++ {
		if identity.String() == value {
			*i = identity
			return nil
		}
	}
	return errors.Join(ErrJSONContract, errorIdentityContractError("error identity text is not admitted"))
}

// Is implements error matching through the typed parent graph.
func (i ErrorIdentity) Is(target error) bool {
	identity, ok := target.(ErrorIdentity)
	return ok && i.Matches(identity)
}

// Matches reports whether i is target or descends from target.
func (i ErrorIdentity) Matches(target ErrorIdentity) bool {
	if !isAdmittedErrorIdentity(i) || !isAdmittedErrorIdentity(target) {
		return false
	}
	// At most errorIdentityLimit-1 admitted identities can be enqueued. Marking
	// on admission prevents duplicate parents from consuming stack capacity, so
	// the compiler-sized closed-domain stack cannot exhaust.
	var pending [errorIdentityLimit]ErrorIdentity
	var visited errorIdentityVisitSet
	pending[0] = i
	visited.mark(i)
	count := 1
	for count > 0 {
		count--
		current := pending[count]
		if current == target {
			return true
		}
		parents := errorIdentityParents(current)
		for index := 0; index < parents.countValues(); index++ {
			parent, ok := parents.at(index)
			if !ok || !isAdmittedErrorIdentity(parent) {
				return false
			}
			if !visited.mark(parent) {
				continue
			}
			pending[count] = parent
			count++
		}
	}
	return false
}

// Validate rejects identities outside the closed error domain.
func (i ErrorIdentity) Validate() error {
	if !isAdmittedErrorIdentity(i) {
		return errorIdentityContractError("error identity is outside the admitted domain")
	}
	return nil
}

func isAdmittedErrorIdentity(identity ErrorIdentity) bool {
	return identity > ErrUnknown && identity < errorIdentityLimit
}

func errorIdentityParents(identity ErrorIdentity) errorIdentityParentSet {
	switch identity {
	case ErrJSONContract, ErrNumericOverflow, ErrSecretMaterialAllZero,
		ErrAttestContract,
		ErrContextStateContract, ErrCurrencyContract, ErrGarbleContract,
		ErrKeygenContract, ErrTestIsolationContract, ErrFilestoreContract,
		ErrTemporalContract, ErrExchangeContract,
		ErrFuzzFinderContract, ErrLeaseContract, ErrGateContract,
		ErrProcessContract,
		ErrReleaseContract, ErrDeployContract,
		ErrDistributionContract,
		ErrShutdownContract, ErrObjectStoreContract, ErrTimeProofContract,
		ErrCloudIdentityContract, ErrUpgradeContract,
		ErrLifecycleIdentityContract, ErrReceiptContract, ErrChitContract,
		ErrRetrievalContract, ErrPaymentContract, ErrControlWireContract,
		ErrControlPlaneContract, ErrIDContract:
		return oneErrorIdentityParent(ErrPrimitiveContract)
	case ErrControlWireRevision, ErrControlWireNonce, ErrControlWireToken,
		ErrControlWirePolicyCursor, ErrControlWireRoute, ErrControlWireProtocolSupport,
		ErrControlWireReplayConflict,
		ErrControlPlaneSigningDomain, ErrControlPlaneProductStatus,
		ErrControlPlaneUsageWatermark, ErrControlPlaneResponseHeader,
		ErrControlPlaneResponseDocument,
		ErrControlPlaneResponseBinding, ErrControlPlaneUpgradeRequired,
		ErrControlPlaneProviderTimeRollback,
		ErrControlPlaneRegistration, ErrControlPlaneInstallationBinding,
		ErrControlPlaneDecisionConsistency, ErrControlPlaneCheckIn,
		ErrControlPlaneCheckInResponse, ErrControlPlaneUsageWindow:
		return errorIdentityParentsControlExchange(identity)
	case ErrAttestVerification, ErrNilContext, ErrContextObservation,
		ErrCurrencyMismatch, ErrCurrencyDecimal, ErrCurrencyOverflow,
		ErrGarbleDerivation, ErrGarbleBuildIntent, ErrKeygenEntropy:
		return errorIdentityParentsAttestThroughKeygen(identity)
	case ErrHostFacts, ErrHostFactsContract, ErrHostFactsObservation,
		ErrHostFactsUnsupported, ErrHostFactsPressure, ErrHostFactsEvidence,
		ErrDiskCapacityUnsupported, ErrTreeMeasurementUnsupported,
		ErrDiskFloorReached, ErrMemoryLimitReached:
		return errorIdentityParentsHostFacts(identity)
	default:
		return errorIdentityParentsFilestoreThroughUpgrade(identity)
	}
}

// errorIdentityParentsFilestoreThroughUpgrade continues the family chain. The
// dispatch is split by family so no single switch structurally grows toward
// the complexity ceiling as new identities join; the next family lands in the
// helper with headroom, not in a function already at the line.
func errorIdentityParentsFilestoreThroughUpgrade(identity ErrorIdentity) errorIdentityParentSet {
	switch identity {
	case ErrDistributionVerification, ErrDistributionBinding:
		return oneErrorIdentityParent(ErrDistributionContract)
	case ErrUpgradeDownload, ErrUpgradeCapacity, ErrUpgradeVerification, ErrUpgradeTrial,
		ErrUpgradePromotion, ErrUpgradePersistence, ErrUpgradeCleanup,
		ErrUpgradeConflict:
		return oneErrorIdentityParent(ErrUpgradeContract)
	case ErrFilestoreSize, ErrFilestoreSource, ErrFilestoreDestination,
		ErrFilestoreConflict, ErrFilestoreActivation, ErrFilestoreCleanup:
		return oneErrorIdentityParent(ErrFilestoreContract)
	case ErrFilestoreActivationIndeterminate:
		return oneErrorIdentityParent(ErrFilestoreActivation)
	case ErrTemporalOverflow:
		return twoErrorIdentityParents(ErrTemporalContract, ErrNumericOverflow)
	case ErrExchangeRequest, ErrExchangeResponse, ErrExchangeBodyLimit,
		ErrExchangeContentType, ErrExchangeCancelled, ErrExchangeRedirect,
		ErrExchangeTransport, ErrExchangeRetryExhausted, ErrExchangeWrite:
		return oneErrorIdentityParent(ErrExchangeContract)
	default:
		return errorIdentityParentsReceipt(identity)
	}
}

func errorIdentityParentsReceipt(identity ErrorIdentity) errorIdentityParentSet {
	switch identity {
	case ErrReceiptVerification, ErrReceiptScope, ErrReceiptRollback,
		ErrReceiptConflict:
		return oneErrorIdentityParent(ErrReceiptContract)
	case ErrChitVerification, ErrChitConflict:
		return oneErrorIdentityParent(ErrChitContract)
	case ErrRetrievalBinding:
		return oneErrorIdentityParent(ErrRetrievalContract)
	case ErrPaymentVerification:
		return oneErrorIdentityParent(ErrPaymentContract)
	default:
		return errorIdentityParentsFuzzFinderThroughObjectStore(identity)
	}
}

func errorIdentityParentsHostFacts(identity ErrorIdentity) errorIdentityParentSet {
	switch identity {
	case ErrHostFacts:
		return errorIdentityParentSet{}
	case ErrHostFactsContract:
		return twoErrorIdentityParents(ErrHostFacts, ErrPrimitiveContract)
	case ErrHostFactsObservation, ErrHostFactsUnsupported, ErrHostFactsPressure,
		ErrHostFactsEvidence:
		return oneErrorIdentityParent(ErrHostFacts)
	case ErrDiskCapacityUnsupported, ErrTreeMeasurementUnsupported:
		return oneErrorIdentityParent(ErrHostFactsUnsupported)
	case ErrDiskFloorReached, ErrMemoryLimitReached:
		return oneErrorIdentityParent(ErrHostFactsPressure)
	default:
		return errorIdentityParentSet{}
	}
}

func errorIdentityParentsAttestThroughKeygen(identity ErrorIdentity) errorIdentityParentSet {
	switch identity {
	case ErrAttestVerification:
		return oneErrorIdentityParent(ErrAttestContract)
	case ErrNilContext, ErrContextObservation:
		return oneErrorIdentityParent(ErrContextStateContract)
	case ErrCurrencyMismatch, ErrCurrencyDecimal:
		return oneErrorIdentityParent(ErrCurrencyContract)
	case ErrCurrencyOverflow:
		return twoErrorIdentityParents(ErrCurrencyContract, ErrNumericOverflow)
	case ErrGarbleDerivation, ErrGarbleBuildIntent:
		return oneErrorIdentityParent(ErrGarbleContract)
	case ErrKeygenEntropy:
		return oneErrorIdentityParent(ErrKeygenContract)
	default:
		return errorIdentityParentSet{}
	}
}

// errorIdentityParentsControlExchange owns the parents of both halves of the
// control exchange: the scalars Controlwire carries and the documents
// Controlplane assembles from them. They are one function because they are one
// exchange, and separating them here would say two ends of the same protocol
// are unrelated.
func errorIdentityParentsControlExchange(identity ErrorIdentity) errorIdentityParentSet {
	switch identity {
	case ErrControlWireRevision, ErrControlWireNonce, ErrControlWireToken,
		ErrControlWirePolicyCursor, ErrControlWireRoute, ErrControlWireProtocolSupport,
		ErrControlWireReplayConflict:
		return oneErrorIdentityParent(ErrControlWireContract)
	case ErrControlPlaneSigningDomain, ErrControlPlaneProductStatus,
		ErrControlPlaneUsageWatermark, ErrControlPlaneResponseHeader,
		ErrControlPlaneResponseDocument,
		ErrControlPlaneResponseBinding, ErrControlPlaneUpgradeRequired,
		ErrControlPlaneProviderTimeRollback,
		ErrControlPlaneRegistration, ErrControlPlaneInstallationBinding,
		ErrControlPlaneDecisionConsistency, ErrControlPlaneCheckIn,
		ErrControlPlaneCheckInResponse, ErrControlPlaneUsageWindow:
		return oneErrorIdentityParent(ErrControlPlaneContract)
	default:
		return errorIdentityParentSet{}
	}
}

func errorIdentityParentsFuzzFinderThroughObjectStore(identity ErrorIdentity) errorIdentityParentSet {
	switch identity {
	case ErrFuzzFinderFormat, ErrFuzzFinderObservation:
		return oneErrorIdentityParent(ErrFuzzFinderContract)
	case ErrLeaseVerification, ErrLeaseRollback, ErrLeaseConflict,
		ErrLeaseScope, ErrLeaseClock, ErrLeaseProduct:
		return oneErrorIdentityParent(ErrLeaseContract)
	case ErrGateDenied:
		return oneErrorIdentityParent(ErrGateContract)
	case ErrProcessStart, ErrProcessStream, ErrProcessWait,
		ErrProcessObservation, ErrProcessUnsupported:
		return oneErrorIdentityParent(ErrProcessContract)
	case ErrProcessOutputLimit:
		return oneErrorIdentityParent(ErrProcessStream)
	case ErrReleaseManifest, ErrReleaseVerification, ErrReleaseLatest,
		ErrReleaseRollback, ErrReleaseConflict:
		return oneErrorIdentityParent(ErrReleaseContract)
	case ErrShutdownStepFailure, ErrShutdownStepTimeout, ErrShutdownStepPanic,
		ErrShutdownTotalTimeout, ErrShutdownSignalSource,
		ErrShutdownSignalReceived:
		return oneErrorIdentityParent(ErrShutdownContract)
	case ErrObjectStoreExpired, ErrObjectStoreIntegrity, ErrObjectStoreSource,
		ErrObjectStoreDestination, ErrObjectStoreConflict, ErrObjectStoreSize,
		ErrObjectStoreAbsent:
		return oneErrorIdentityParent(ErrObjectStoreContract)
	case ErrTimeProofRefused, ErrTimeProofInvalid:
		return oneErrorIdentityParent(ErrTimeProofContract)
	default:
		return errorIdentityParentSet{}
	}
}

func oneErrorIdentityParent(parent ErrorIdentity) errorIdentityParentSet {
	return errorIdentityParentSet{
		values: [errorIdentityMaximumParents]ErrorIdentity{parent},
		count:  1,
	}
}

func twoErrorIdentityParents(first, second ErrorIdentity) errorIdentityParentSet {
	return errorIdentityParentSet{
		values: [errorIdentityMaximumParents]ErrorIdentity{first, second},
		count:  errorIdentityMaximumParents,
	}
}

func (p errorIdentityParentSet) countValues() int {
	return int(p.count)
}

func (p errorIdentityParentSet) at(index int) (ErrorIdentity, bool) {
	if index < 0 || index >= int(p.count) || index >= len(p.values) {
		return ErrUnknown, false
	}
	return p.values[index], true
}

func errorIdentityContractError(message string) error {
	return errors.Join(ErrPrimitiveContract, errors.New(message))
}
