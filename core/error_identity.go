package core

import (
	"errors"
)

const unknownErrorIdentityText = "unknown primitive error identity"

const (
	// ErrorIdentityMaximumParents is the compile-time parent arity of an identity.
	ErrorIdentityMaximumParents = 2
	errorIdentityMaximumLevels  = 4
	errorIdentityTraversalLimit = ErrorIdentityMaximumParents * errorIdentityMaximumLevels
	errorIdentityVisitWordBits  = 64
)

type errorIdentityParentSet struct {
	values [ErrorIdentityMaximumParents]ErrorIdentity
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

	// ErrHostResourceContract identifies a host-resource contract violation.
	ErrHostResourceContract
	// ErrDiskFloorReached identifies insufficient available disk capacity.
	ErrDiskFloorReached

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

	// ErrFuzzContract identifies a fuzz contract violation.
	ErrFuzzContract
	// ErrFuzzFormat identifies an unsupported fuzz format.
	ErrFuzzFormat
	// ErrFuzzObservation identifies failed fuzz observation.
	ErrFuzzObservation

	// ErrLeaseContract identifies a lease contract violation.
	ErrLeaseContract
	// ErrLeaseVerification identifies failed lease verification.
	ErrLeaseVerification
	// ErrLeaseRollback identifies rejected lease rollback.
	ErrLeaseRollback
	// ErrLeaseConflict identifies a lease identity conflict.
	ErrLeaseConflict

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

	// ErrShutdownContract identifies a shutdown contract violation.
	ErrShutdownContract

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
	// ErrWorkloadIdentityContract identifies a workload-identity violation.
	ErrWorkloadIdentityContract
	// ErrUpgradeContract identifies an upgrade contract violation.
	ErrUpgradeContract

	// ErrGovernanceContract identifies a governance contract violation.
	ErrGovernanceContract
	// ErrGovernanceDocumentSource identifies a governance document that could
	// not be read. Core owns this rejection so a caller distinguishes an
	// unreadable document from one whose content violates the contract.
	ErrGovernanceDocumentSource
	// ErrGovernanceDocumentLength identifies a governance document whose byte
	// length is not the exact canonical length.
	ErrGovernanceDocumentLength
	// ErrGovernanceDocumentDigest identifies a governance document whose digest
	// is not the canonical digest.
	ErrGovernanceDocumentDigest

	errorIdentityLimit
)

var _ [2*errorIdentityVisitWordBits - int(errorIdentityLimit)]struct{}

// Error returns the stable diagnostic text for i.
func (i ErrorIdentity) Error() string {
	switch {
	case i <= ErrContextObservation:
		return errorIdentityTextPrimitiveThroughContext(i)
	case i <= ErrKeygenEntropy:
		return errorIdentityTextCurrencyThroughKeygen(i)
	case i <= ErrFilestoreCleanup:
		return errorIdentityTextIsolationThroughFilestoreCleanup(i)
	case i <= ErrExchangeRequest:
		return errorIdentityTextHostResourceThroughExchangeRequest(i)
	case i <= ErrExchangeWrite:
		return errorIdentityTextExchange(i)
	case i <= ErrFuzzFormat:
		return errorIdentityTextFuzzHead(i)
	case i <= ErrReleaseManifest:
		return errorIdentityTextFuzzThroughReleaseManifest(i)
	case i <= ErrObjectStoreIntegrity:
		return errorIdentityTextReleaseThroughObjectIntegrity(i)
	case i <= ErrUpgradeContract:
		return errorIdentityTextObjectTailThroughUpgrade(i)
	default:
		return errorIdentityTextGovernance(i)
	}
}

func errorIdentityTextGovernance(i ErrorIdentity) string {
	switch i {
	case ErrGovernanceContract:
		return "governance contract violation"
	case ErrGovernanceDocumentSource:
		return "governance document source failed"
	case ErrGovernanceDocumentLength:
		return "governance document length rejected"
	case ErrGovernanceDocumentDigest:
		return "governance document digest rejected"
	default:
		return unknownErrorIdentityText
	}
}

func errorIdentityTextPrimitiveThroughContext(i ErrorIdentity) string {
	switch i {
	case ErrPrimitiveContract:
		return "primitive contract violation"
	case ErrJSONContract:
		return "json contract violation"
	case ErrNumericOverflow:
		return "numeric overflow"
	case ErrSecretMaterialAllZero:
		return "secret material is all zero"
	case ErrAttestContract:
		return "attest contract violation"
	case ErrAttestVerification:
		return "attest verification failed"
	case ErrContextStateContract:
		return "context state contract violation"
	case ErrNilContext:
		return "nil context"
	case ErrContextObservation:
		return "context observation failed"
	default:
		return unknownErrorIdentityText
	}
}

func errorIdentityTextCurrencyThroughKeygen(i ErrorIdentity) string {
	switch i {
	case ErrCurrencyContract:
		return "currency contract violation"
	case ErrCurrencyMismatch:
		return "currency mismatch"
	case ErrCurrencyOverflow:
		return "currency overflow"
	case ErrCurrencyDecimal:
		return "currency decimal rejected"
	case ErrGarbleContract:
		return "garble contract violation"
	case ErrGarbleDerivation:
		return "garble derivation failed"
	case ErrGarbleBuildIntent:
		return "garble build intent rejected"
	case ErrKeygenContract:
		return "key generation contract violation"
	case ErrKeygenEntropy:
		return "key generation entropy failed"
	default:
		return unknownErrorIdentityText
	}
}

func errorIdentityTextIsolationThroughFilestoreCleanup(i ErrorIdentity) string {
	switch i {
	case ErrTestIsolationContract:
		return "test isolation contract violation"
	case ErrFilestoreContract:
		return "filestore contract violation"
	case ErrFilestoreSize:
		return "filestore size rejected"
	case ErrFilestoreSource:
		return "filestore source failed"
	case ErrFilestoreDestination:
		return "filestore destination failed"
	case ErrFilestoreConflict:
		return "filestore namespace conflict"
	case ErrFilestoreActivation:
		return "filestore activation failed"
	case ErrFilestoreActivationIndeterminate:
		return "filestore activation indeterminate"
	case ErrFilestoreCleanup:
		return "filestore cleanup failed"
	default:
		return unknownErrorIdentityText
	}
}

func errorIdentityTextHostResourceThroughExchangeRequest(i ErrorIdentity) string {
	switch i {
	case ErrHostResourceContract:
		return "host resource contract violation"
	case ErrDiskFloorReached:
		return "disk floor reached"
	case ErrTemporalContract:
		return "temporal contract violation"
	case ErrTemporalOverflow:
		return "temporal arithmetic overflow"
	case ErrExchangeContract:
		return "exchange contract violation"
	case ErrExchangeRequest:
		return "exchange request rejected"
	default:
		return unknownErrorIdentityText
	}
}

func errorIdentityTextExchange(i ErrorIdentity) string {
	switch i {
	case ErrExchangeResponse:
		return "exchange response rejected"
	case ErrExchangeBodyLimit:
		return "exchange body limit exceeded"
	case ErrExchangeContentType:
		return "exchange content type rejected"
	case ErrExchangeCancelled:
		return "exchange cancelled"
	case ErrExchangeRedirect:
		return "exchange redirect rejected"
	case ErrExchangeTransport:
		return "exchange transport failed"
	case ErrExchangeRetryExhausted:
		return "exchange retry budget exhausted"
	case ErrExchangeWrite:
		return "exchange write failed"
	default:
		return unknownErrorIdentityText
	}
}

func errorIdentityTextFuzzHead(i ErrorIdentity) string {
	switch i {
	case ErrFuzzContract:
		return "fuzz contract violation"
	case ErrFuzzFormat:
		return "fuzz format unsupported"
	default:
		return unknownErrorIdentityText
	}
}

func errorIdentityTextFuzzThroughReleaseManifest(i ErrorIdentity) string {
	switch i {
	case ErrFuzzObservation:
		return "fuzz observation failed"
	case ErrLeaseContract:
		return "lease contract violation"
	case ErrLeaseVerification:
		return "lease verification failed"
	case ErrLeaseRollback:
		return "lease rollback rejected"
	case ErrLeaseConflict:
		return "lease identity conflict"
	case ErrReleaseContract:
		return "release contract violation"
	case ErrReleaseManifest:
		return "release manifest rejected"
	default:
		return unknownErrorIdentityText
	}
}

func errorIdentityTextReleaseThroughObjectIntegrity(i ErrorIdentity) string {
	switch i {
	case ErrReleaseVerification:
		return "release verification failed"
	case ErrReleaseLatest:
		return "release latest rejected"
	case ErrReleaseRollback:
		return "release rollback rejected"
	case ErrReleaseConflict:
		return "release identity conflict"
	case ErrShutdownContract:
		return "shutdown contract violation"
	case ErrObjectStoreContract:
		return "object store contract violation"
	case ErrObjectStoreExpired:
		return "object store target expired"
	case ErrObjectStoreIntegrity:
		return "object store integrity failed"
	default:
		return unknownErrorIdentityText
	}
}

func errorIdentityTextObjectTailThroughUpgrade(i ErrorIdentity) string {
	switch i {
	case ErrObjectStoreSource:
		return "object store source failed"
	case ErrObjectStoreDestination:
		return "object store destination failed"
	case ErrObjectStoreConflict:
		return "object store write conflict"
	case ErrObjectStoreSize:
		return "object store size rejected"
	case ErrObjectStoreAbsent:
		return "object store object absent"
	case ErrTimeProofContract:
		return "time proof contract violation"
	case ErrWorkloadIdentityContract:
		return "workload identity contract violation"
	case ErrUpgradeContract:
		return "upgrade contract violation"
	default:
		return unknownErrorIdentityText
	}
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
	return marshalJSONString(i.String())
}

// UnmarshalJSON accepts only the stable text of an admitted identity.
func (i *ErrorIdentity) UnmarshalJSON(data []byte) error {
	if i == nil {
		return errors.Join(ErrJSONContract, errorIdentityContractError("nil error identity receiver"))
	}
	value, err := decodeJSONString(data)
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
	var pending [errorIdentityTraversalLimit]ErrorIdentity
	var visitedLow uint64
	var visitedHigh uint64
	pending[0] = i
	count := 1
	for count > 0 {
		count--
		current := pending[count]
		if current == target {
			return true
		}
		if !markErrorIdentityVisited(current, &visitedLow, &visitedHigh) {
			continue
		}
		parents := errorIdentityParents(current)
		for index := 0; index < parents.countValues(); index++ {
			parent, ok := parents.at(index)
			if !ok || !isAdmittedErrorIdentity(parent) || count >= len(pending) {
				return false
			}
			pending[count] = parent
			count++
		}
	}
	return false
}

func markErrorIdentityVisited(identity ErrorIdentity, low, high *uint64) bool {
	index := uint(identity)
	if index < errorIdentityVisitWordBits {
		mask := uint64(1) << index
		wasUnvisited := *low&mask == 0
		*low |= mask
		return wasUnvisited
	}
	highIndex := index - errorIdentityVisitWordBits
	if highIndex >= errorIdentityVisitWordBits {
		return false
	}
	mask := uint64(1) << highIndex
	wasUnvisited := *high&mask == 0
	*high |= mask
	return wasUnvisited
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
		ErrHostResourceContract, ErrTemporalContract, ErrExchangeContract,
		ErrFuzzContract, ErrLeaseContract, ErrReleaseContract,
		ErrShutdownContract, ErrObjectStoreContract, ErrTimeProofContract,
		ErrWorkloadIdentityContract, ErrUpgradeContract, ErrGovernanceContract:
		return oneErrorIdentityParent(ErrPrimitiveContract)
	case ErrGovernanceDocumentSource, ErrGovernanceDocumentLength,
		ErrGovernanceDocumentDigest:
		return oneErrorIdentityParent(ErrGovernanceContract)
	case ErrAttestVerification, ErrNilContext, ErrContextObservation,
		ErrCurrencyMismatch, ErrCurrencyDecimal, ErrCurrencyOverflow,
		ErrGarbleDerivation, ErrGarbleBuildIntent, ErrKeygenEntropy:
		return errorIdentityParentsAttestThroughKeygen(identity)
	case ErrFilestoreSize, ErrFilestoreSource, ErrFilestoreDestination,
		ErrFilestoreConflict, ErrFilestoreActivation, ErrFilestoreCleanup:
		return oneErrorIdentityParent(ErrFilestoreContract)
	case ErrFilestoreActivationIndeterminate:
		return oneErrorIdentityParent(ErrFilestoreActivation)
	case ErrDiskFloorReached:
		return oneErrorIdentityParent(ErrHostResourceContract)
	case ErrTemporalOverflow:
		return twoErrorIdentityParents(ErrTemporalContract, ErrNumericOverflow)
	case ErrExchangeRequest, ErrExchangeResponse, ErrExchangeBodyLimit,
		ErrExchangeContentType, ErrExchangeCancelled, ErrExchangeRedirect,
		ErrExchangeTransport, ErrExchangeRetryExhausted, ErrExchangeWrite:
		return oneErrorIdentityParent(ErrExchangeContract)
	default:
		return errorIdentityParentsFuzzThroughObjectStore(identity)
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

func errorIdentityParentsFuzzThroughObjectStore(identity ErrorIdentity) errorIdentityParentSet {
	switch identity {
	case ErrFuzzFormat, ErrFuzzObservation:
		return oneErrorIdentityParent(ErrFuzzContract)
	case ErrLeaseVerification, ErrLeaseRollback, ErrLeaseConflict:
		return oneErrorIdentityParent(ErrLeaseContract)
	case ErrReleaseManifest, ErrReleaseVerification, ErrReleaseLatest,
		ErrReleaseRollback, ErrReleaseConflict:
		return oneErrorIdentityParent(ErrReleaseContract)
	case ErrObjectStoreExpired, ErrObjectStoreIntegrity, ErrObjectStoreSource,
		ErrObjectStoreDestination, ErrObjectStoreConflict, ErrObjectStoreSize,
		ErrObjectStoreAbsent:
		return oneErrorIdentityParent(ErrObjectStoreContract)
	default:
		return errorIdentityParentSet{}
	}
}

func oneErrorIdentityParent(parent ErrorIdentity) errorIdentityParentSet {
	return errorIdentityParentSet{
		values: [ErrorIdentityMaximumParents]ErrorIdentity{parent},
		count:  1,
	}
}

func twoErrorIdentityParents(first, second ErrorIdentity) errorIdentityParentSet {
	return errorIdentityParentSet{
		values: [ErrorIdentityMaximumParents]ErrorIdentity{first, second},
		count:  ErrorIdentityMaximumParents,
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
