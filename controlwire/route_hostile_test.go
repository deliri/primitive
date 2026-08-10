package controlwire

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// TestRouteFamilyClosesItsEntireByteDomain walks every backing value: the three
// published families must validate, agree with IsValid, and carry unique
// nonempty path suffixes, while all two hundred fifty four others refuse and
// render no suffix a request could be built from.
func TestRouteFamilyClosesItsEntireByteDomain(t *testing.T) {
	t.Parallel()

	seen := map[string]RouteFamily{}
	admitted := 0
	for value := 0; value <= 255; value++ {
		family := RouteFamily(value)
		if err := family.Validate(); err != nil {
			if family.IsValid() {
				t.Fatalf("RouteFamily(%d).IsValid() = true beside a Validate refusal", value)
			}
			if got := family.String(); got != "" {
				t.Fatalf("RouteFamily(%d).String() = %q, want empty text for a refused family", value, got)
			}
			continue
		}
		admitted++
		if !family.IsValid() {
			t.Fatalf("RouteFamily(%d).IsValid() = false beside a nil Validate", value)
		}
		suffix := family.String()
		if suffix == "" {
			t.Fatalf("RouteFamily(%d).String() is empty, want the exact path suffix", value)
		}
		if prior, duplicate := seen[suffix]; duplicate {
			t.Fatalf("RouteFamily(%d) and RouteFamily(%d) share the suffix %q", value, prior, suffix)
		}
		seen[suffix] = family
	}
	if admitted != 3 {
		t.Fatalf("admitted route families = %d, want registrations, check-ins, and submissions", admitted)
	}
}

// TestRouteContractProjectsExactlyItsTwoFacts drives every admitted offering
// through every admitted family and holds the projections to the facts the
// contract was built from: the path is the control prefix plus exactly those
// two spellings, the method is always POST, and the accessors return the
// constructed facts. The zero contract refuses a path, so no request can be
// addressed from a contract nobody constructed.
func TestRouteContractProjectsExactlyItsTwoFacts(t *testing.T) {
	t.Parallel()

	families := []RouteFamily{
		RouteFamilyRegistrations, RouteFamilyCheckIns, RouteFamilySubmissions,
	}
	for value := 0; value <= 255; value++ {
		offering := core.Offering(value)
		if !offering.IsValid() {
			continue
		}
		for _, family := range families {
			contract, err := NewRouteContract(offering, family)
			if err != nil {
				t.Fatalf("NewRouteContract(%v, %v) error = %v, want nil", offering, family, err)
			}
			path, err := contract.Path()
			if err != nil {
				t.Fatalf("Path(%v, %v) error = %v, want nil", offering, family, err)
			}
			if want := routeControlPrefix + offering.String() + family.String(); path != want {
				t.Fatalf("Path(%v, %v) = %q, want %q", offering, family, path, want)
			}
			method, err := contract.Method()
			if err != nil || method != exchange.MethodPost {
				t.Fatalf("Method(%v, %v) = (%v, %v), want (%v, nil)", offering, family, method, err, exchange.MethodPost)
			}
			if got := contract.Offering(); got != offering {
				t.Fatalf("Offering() = %v, want the constructed %v", got, offering)
			}
			if got := contract.Family(); got != family {
				t.Fatalf("Family() = %v, want the constructed %v", got, family)
			}
		}
	}

	if path, err := (RouteContract{}).Path(); !errors.Is(err, core.ErrControlWireContract) || path != "" {
		t.Fatalf("zero RouteContract Path() = (%q, %v), want (empty, errors.Is %v)", path, err, core.ErrControlWireContract)
	}
	if got := (RouteContract{}).Offering(); got.IsValid() {
		t.Fatalf("zero RouteContract Offering() = %v, want the invalid zero fact", got)
	}
	if got := (RouteContract{}).Family(); got.IsValid() {
		t.Fatalf("zero RouteContract Family() = %v, want the invalid zero fact", got)
	}
}
