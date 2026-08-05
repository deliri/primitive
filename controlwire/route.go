package controlwire

import (
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

const (
	// routeControlPrefix is the exact control-plane path prefix. Both ends of
	// the exchange derive their path from it, so an installation cannot address
	// a route the authority does not mount.
	routeControlPrefix = "/v2026/control/"

	routeRegistrationsSuffix = "/registrations"
	routeCheckInsSuffix      = "/check-ins"
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
	routeFamilyLimit
)

func routeSuffixes() [routeFamilyLimit]string {
	return [...]string{
		RouteFamilyUnknown:       "",
		RouteFamilyRegistrations: routeRegistrationsSuffix,
		RouteFamilyCheckIns:      routeCheckInsSuffix,
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

var (
	_ core.Validatable = RouteFamily(RouteFamilyUnknown)
	_ core.Validatable = RouteContract{}
)
