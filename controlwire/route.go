package controlwire

import (
	json "encoding/json/v2"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

const (
	// routeControlPrefix is the exact control-plane path prefix. Both ends of
	// the exchange derive their path from it, so an installation cannot address
	// a route the authority does not mount.
	routeControlPrefix = "/v2026/control/"
	routeSeparator     = "/"

	routeRegistrationsToken                 = "registrations"
	routeCheckInsToken                      = "check-ins"
	routeSubmissionsToken                   = "submissions"
	routeSubmissionCompletionsToken         = "submission-completions"
	routeChitsToken                         = "chits"
	routeRetrievalsToken                    = "retrievals"
	routePaymentsToken                      = "payments"
	routeReleaseMaterialsToken              = "release-materials"
	routeReleasePublicationsToken           = "release-publications"
	routeReleasePublicationCompletionsToken = "release-publication-completions"
	routeUpdateChecksToken                  = "update-checks"
	routeUpgradesToken                      = "upgrades"

	routeRegistrationsSuffix                 = routeSeparator + routeRegistrationsToken
	routeCheckInsSuffix                      = routeSeparator + routeCheckInsToken
	routeSubmissionsSuffix                   = routeSeparator + routeSubmissionsToken
	routeSubmissionCompletionsSuffix         = routeSeparator + routeSubmissionCompletionsToken
	routeChitsSuffix                         = routeSeparator + routeChitsToken
	routeRetrievalsSuffix                    = routeSeparator + routeRetrievalsToken
	routePaymentsSuffix                      = routeSeparator + routePaymentsToken
	routeReleaseMaterialsSuffix              = routeSeparator + routeReleaseMaterialsToken
	routeReleasePublicationsSuffix           = routeSeparator + routeReleasePublicationsToken
	routeReleasePublicationCompletionsSuffix = routeSeparator + routeReleasePublicationCompletionsToken
	routeUpdateChecksSuffix                  = routeSeparator + routeUpdateChecksToken
	routeUpgradesSuffix                      = routeSeparator + routeUpgradesToken
)

// RouteFamily is the closed set of control-plane route families.
type RouteFamily uint8

const (
	// RouteFamilyUnknown is the unset family and addresses nothing.
	RouteFamilyUnknown RouteFamily = iota
	// RouteFamilyRegistrations is first contact, before any credential exists.
	RouteFamilyRegistrations
	// RouteFamilyCheckIns is every later exchange, all of which present one.
	RouteFamilyCheckIns
	// RouteFamilySubmissions requests authority for one declared evidence object.
	RouteFamilySubmissions
	// RouteFamilySubmissionCompletions reports one attempted granted upload.
	RouteFamilySubmissionCompletions
	// RouteFamilyChits lists or selects customer custody tickets.
	RouteFamilyChits
	// RouteFamilyRetrievals requests exact-object download capabilities.
	RouteFamilyRetrievals
	// RouteFamilyPayments lists or selects customer payment receipts.
	RouteFamilyPayments
	// RouteFamilyReleaseMaterials provides one request-bound maintainer capability bundle.
	RouteFamilyReleaseMaterials
	// RouteFamilyReleasePublications requests authority for one exact release publication.
	RouteFamilyReleasePublications
	// RouteFamilyReleasePublicationCompletions reports one complete release publication.
	RouteFamilyReleasePublicationCompletions
	// RouteFamilyUpdateChecks requests the authenticated current release selection.
	RouteFamilyUpdateChecks
	// RouteFamilyUpgrades requests one exact candidate download capability.
	RouteFamilyUpgrades
	routeFamilyLimit
)

func routeSuffixes() [routeFamilyLimit]string {
	return [...]string{
		RouteFamilyUnknown:                       "",
		RouteFamilyRegistrations:                 routeRegistrationsSuffix,
		RouteFamilyCheckIns:                      routeCheckInsSuffix,
		RouteFamilySubmissions:                   routeSubmissionsSuffix,
		RouteFamilySubmissionCompletions:         routeSubmissionCompletionsSuffix,
		RouteFamilyChits:                         routeChitsSuffix,
		RouteFamilyRetrievals:                    routeRetrievalsSuffix,
		RouteFamilyPayments:                      routePaymentsSuffix,
		RouteFamilyReleaseMaterials:              routeReleaseMaterialsSuffix,
		RouteFamilyReleasePublications:           routeReleasePublicationsSuffix,
		RouteFamilyReleasePublicationCompletions: routeReleasePublicationCompletionsSuffix,
		RouteFamilyUpdateChecks:                  routeUpdateChecksSuffix,
		RouteFamilyUpgrades:                      routeUpgradesSuffix,
	}
}

func routeFamilyTokens() [routeFamilyLimit]string {
	return [...]string{
		RouteFamilyUnknown:                       "",
		RouteFamilyRegistrations:                 routeRegistrationsToken,
		RouteFamilyCheckIns:                      routeCheckInsToken,
		RouteFamilySubmissions:                   routeSubmissionsToken,
		RouteFamilySubmissionCompletions:         routeSubmissionCompletionsToken,
		RouteFamilyChits:                         routeChitsToken,
		RouteFamilyRetrievals:                    routeRetrievalsToken,
		RouteFamilyPayments:                      routePaymentsToken,
		RouteFamilyReleaseMaterials:              routeReleaseMaterialsToken,
		RouteFamilyReleasePublications:           routeReleasePublicationsToken,
		RouteFamilyReleasePublicationCompletions: routeReleasePublicationCompletionsToken,
		RouteFamilyUpdateChecks:                  routeUpdateChecksToken,
		RouteFamilyUpgrades:                      routeUpgradesToken,
	}
}

// Validate rejects the unset family and every family outside the set.
func (f RouteFamily) Validate() error {
	if f <= RouteFamilyUnknown || f >= routeFamilyLimit || routeSuffixes()[f] == "" {
		return routeError()
	}
	return nil
}

// IsValid reports whether f is a declared route family.
func (f RouteFamily) IsValid() bool { return f.Validate() == nil }

// String returns the family's exact path suffix, or empty text when unset.
func (f RouteFamily) String() string {
	if f >= routeFamilyLimit {
		return ""
	}
	return routeSuffixes()[f]
}

// ParseRouteFamily accepts one exact compiler-owned wire token.
func ParseRouteFamily(value string) (RouteFamily, error) {
	tokens := routeFamilyTokens()
	for family := RouteFamilyUnknown + 1; family < routeFamilyLimit; family++ {
		if tokens[family] == value {
			return family, nil
		}
	}
	return RouteFamilyUnknown, routeError()
}

// MarshalJSON emits the family token, not its URL suffix.
func (f RouteFamily) MarshalJSON() ([]byte, error) {
	if err := f.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONString(routeFamilyTokens()[f])
	if err != nil {
		return nil, jsonError(routeError(err))
	}
	return encoded, nil
}

// UnmarshalJSON accepts one exact family token and preserves the receiver on
// every rejection.
func (f *RouteFamily) UnmarshalJSON(data []byte) error {
	if f == nil {
		return jsonError(routeError())
	}
	token, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return jsonError(routeError(err))
	}
	parsed, err := ParseRouteFamily(token)
	if err != nil {
		return jsonError(err)
	}
	*f = parsed
	return nil
}

// RouteContract owns the two facts that decide one control-plane route.
//
// The path, method, and replay semantics are projections of those two facts and
// are never stored, so a contract whose parts disagree with each other is
// unrepresentable rather than merely rejected. This is the type that stops a
// client addressing one route while the authority mounted another: both ends
// build their string from the same pair.
type RouteContract struct {
	offering core.Offering
	family   RouteFamily
}

// NewRouteContract owns the only admitted offering and family pair.
func NewRouteContract(offering core.Offering, family RouteFamily) (RouteContract, error) {
	contract := RouteContract{offering: offering, family: family}
	if err := contract.Validate(); err != nil {
		return RouteContract{}, err
	}
	return contract, nil
}

// Validate closes both facts the contract is built from.
func (c RouteContract) Validate() error {
	if err := c.offering.Validate(); err != nil {
		return routeError(err)
	}
	if err := c.family.Validate(); err != nil {
		return routeError(err)
	}
	return nil
}

// Path projects the exact HTTP path this contract addresses.
func (c RouteContract) Path() (string, error) {
	if err := c.Validate(); err != nil {
		return "", err
	}
	return routeControlPrefix + c.offering.String() + c.family.String(), nil
}

// Method projects the exact HTTP method. Every control-plane route submits a
// signed document, so all of them are POST and none is safe to retry blindly.
func (c RouteContract) Method() (exchange.Method, error) {
	if err := c.Validate(); err != nil {
		return exchange.Method(0), err
	}
	return exchange.MethodPost, nil
}

// Offering returns the offering this route serves.
func (c RouteContract) Offering() core.Offering { return c.offering }

// Family returns the route family this contract addresses.
func (c RouteContract) Family() RouteFamily { return c.family }

// ProtocolCapability joins this exact route family to the revision carried by
// its request. The offering remains a separate tenant/product scope and does
// not change whether two peers speak the same protocol.
func (c RouteContract) ProtocolCapability(revision Revision) (ProtocolCapability, error) {
	if err := c.Validate(); err != nil {
		return ProtocolCapability{}, err
	}
	capability := ProtocolCapability{Revision: revision, Family: c.family}
	return capability, capability.Validate()
}

var (
	_ core.Validatable            = RouteFamily(RouteFamilyUnknown)
	_ core.ValidatedJSONMarshaler = RouteFamily(0)
	_ core.Validatable            = RouteContract{}
	_ json.Marshaler              = RouteFamilyUnknown
	_ json.Unmarshaler            = (*RouteFamily)(nil)
)
