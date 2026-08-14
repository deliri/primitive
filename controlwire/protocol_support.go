package controlwire

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

const (
	// ProtocolCapabilityMaximum is the complete published revision-by-route
	// surface. Adding a revision or route family grows the fixed compiler-owned
	// ceiling; runtime input can never choose a larger set.
	ProtocolCapabilityMaximum = int(revisionLimit-1) * int(routeFamilyLimit-1)

	protocolSupportOutcomeAcceptedDiagnostic        = "supported"
	protocolSupportOutcomeUpgradeRequiredDiagnostic = "client-upgrade-required"
)

func protocolSupportOutcomeDiagnostics() [protocolSupportOutcomeLimit]string {
	return [...]string{
		ProtocolSupportOutcomeUnknown:         "",
		ProtocolSupportOutcomeAccepted:        protocolSupportOutcomeAcceptedDiagnostic,
		ProtocolSupportOutcomeUpgradeRequired: protocolSupportOutcomeUpgradeRequiredDiagnostic,
	}
}

// ProtocolSupportOutcome is the complete result of asking an authority
// whether it still speaks one exact revision on one exact route family.
type ProtocolSupportOutcome uint8

const (
	ProtocolSupportOutcomeUnknown ProtocolSupportOutcome = iota
	ProtocolSupportOutcomeAccepted
	ProtocolSupportOutcomeUpgradeRequired
	protocolSupportOutcomeLimit
)

// ProtocolCapability is one indivisible client/authority agreement: an exact
// published revision on an exact route family.
type ProtocolCapability struct {
	Revision Revision
	Family   RouteFamily
}

// ProtocolSupportRequest supplies the authority's bounded support policy to
// the product-neutral constructor. Primitive validates and closes the set; the
// authority decides which published pairs remain enabled.
type ProtocolSupportRequest struct {
	Capabilities []ProtocolCapability
}

// ProtocolSupport is an immutable fixed-capacity set of route/revision pairs.
// It is deliberately not a map or wire document: it is one authority's local
// policy input to a blind compatibility decision.
type ProtocolSupport struct {
	capabilities [ProtocolCapabilityMaximum]ProtocolCapability
	count        int
}

// ProtocolAssessmentRequest asks one closed support set about one exact pair.
type ProtocolAssessmentRequest struct {
	Support    ProtocolSupport
	Capability ProtocolCapability
}

// ProtocolAssessment is the typed, exhaustive compatibility fact an authority
// must bind into either a normal response issuance or an upgrade refusal.
type ProtocolAssessment struct {
	Capability ProtocolCapability
	Outcome    ProtocolSupportOutcome
}

func (o ProtocolSupportOutcome) Validate() error {
	if o <= ProtocolSupportOutcomeUnknown || o >= protocolSupportOutcomeLimit {
		return protocolSupportError()
	}
	return nil
}

func (o ProtocolSupportOutcome) IsValid() bool { return o.Validate() == nil }

// String returns the local diagnostic name of the closed outcome. It is not a
// wire projection; the signed wire fact remains controlplane.ProductStatus.
func (o ProtocolSupportOutcome) String() string {
	if o >= protocolSupportOutcomeLimit {
		return ""
	}
	return protocolSupportOutcomeDiagnostics()[o]
}

// OffWireEnum declares that the support outcome is authority-local. The signed
// wire projection is ProductStatusUpgradeRequired in controlplane.
func (ProtocolSupportOutcome) OffWireEnum() {}

func (c ProtocolCapability) Validate() error {
	if err := errors.Join(c.Revision.Validate(), c.Family.Validate()); err != nil {
		return protocolSupportError(err)
	}
	return nil
}

func (r ProtocolSupportRequest) Validate() error {
	if len(r.Capabilities) == 0 || len(r.Capabilities) > ProtocolCapabilityMaximum {
		return protocolSupportError()
	}
	for index := range r.Capabilities {
		if err := r.Capabilities[index].Validate(); err != nil {
			return err
		}
		for prior := range index {
			if r.Capabilities[prior] == r.Capabilities[index] {
				return protocolSupportError()
			}
		}
	}
	return nil
}

// NewProtocolSupport copies and canonicalizes one bounded authority policy.
func NewProtocolSupport(request ProtocolSupportRequest) (ProtocolSupport, error) {
	if err := request.Validate(); err != nil {
		return ProtocolSupport{}, err
	}
	var support ProtocolSupport
	support.count = len(request.Capabilities)
	copy(support.capabilities[:], request.Capabilities)
	sortProtocolCapabilities(support.capabilities[:support.count])
	return support, support.Validate()
}

// PublishedProtocolSupport returns every pair this Primitive build implements.
// An authority may pass a strict subset to NewProtocolSupport when it retires
// one route/revision pair, but it cannot invent an unpublished pair.
func PublishedProtocolSupport() (ProtocolSupport, error) {
	var support ProtocolSupport
	for revision := RevisionUnknown + 1; revision < revisionLimit; revision++ {
		for family := RouteFamilyUnknown + 1; family < routeFamilyLimit; family++ {
			support.capabilities[support.count] = ProtocolCapability{Revision: revision, Family: family}
			support.count++
		}
	}
	return support, support.Validate()
}

func (s ProtocolSupport) Validate() error {
	if s.count == 0 || s.count > len(s.capabilities) {
		return protocolSupportError()
	}
	for index := range s.count {
		if err := s.capabilities[index].Validate(); err != nil {
			return err
		}
		if index > 0 && !protocolCapabilityLess(s.capabilities[index-1], s.capabilities[index]) {
			return protocolSupportError()
		}
	}
	for index := s.count; index < len(s.capabilities); index++ {
		if s.capabilities[index] != (ProtocolCapability{}) {
			return protocolSupportError()
		}
	}
	return nil
}

func (r ProtocolAssessmentRequest) Validate() error {
	if err := errors.Join(r.Support.Validate(), r.Capability.Validate()); err != nil {
		return protocolSupportError(err)
	}
	return nil
}

func (a ProtocolAssessment) Validate() error {
	if err := errors.Join(a.Capability.Validate(), a.Outcome.Validate()); err != nil {
		return protocolSupportError(err)
	}
	return nil
}

// AssessProtocol returns Accepted only for an exact member of the authority's
// set. A well-formed published pair outside that set is not an error or an
// existence oracle; it is the single UpgradeRequired outcome.
func AssessProtocol(request ProtocolAssessmentRequest) (ProtocolAssessment, error) {
	if err := request.Validate(); err != nil {
		return ProtocolAssessment{}, err
	}
	outcome := ProtocolSupportOutcomeUpgradeRequired
	for index := range request.Support.count {
		if request.Support.capabilities[index] == request.Capability {
			outcome = ProtocolSupportOutcomeAccepted
			break
		}
	}
	assessment := ProtocolAssessment{Capability: request.Capability, Outcome: outcome}
	return assessment, assessment.Validate()
}

func sortProtocolCapabilities(capabilities []ProtocolCapability) {
	for index := 1; index < len(capabilities); index++ {
		value := capabilities[index]
		position := index
		for position > 0 && protocolCapabilityLess(value, capabilities[position-1]) {
			capabilities[position] = capabilities[position-1]
			position--
		}
		capabilities[position] = value
	}
}

func protocolCapabilityLess(left, right ProtocolCapability) bool {
	if left.Revision != right.Revision {
		return left.Revision < right.Revision
	}
	return left.Family < right.Family
}

func protocolSupportError(causes ...error) error {
	return scalarError(core.ErrControlWireProtocolSupport, causes...)
}

var (
	_ core.Validatable = ProtocolSupportOutcomeUnknown
	_ core.OffWireEnum = ProtocolSupportOutcomeUnknown
	_ core.Validatable = ProtocolCapability{}
	_ core.Validatable = ProtocolSupportRequest{}
	_ core.Validatable = ProtocolSupport{}
	_ core.Validatable = ProtocolAssessmentRequest{}
	_ core.Validatable = ProtocolAssessment{}
)
