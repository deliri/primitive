package controlwire

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// Exhaustive typed membership prevents a swapped or widened enum admission
// from passing merely because the total number of accepted bytes stayed equal.
func TestRouteFamilyClosesItsEntireByteDomain(t *testing.T) {
	t.Parallel()
	published := []RouteFamily{RouteFamilyRegistrations, RouteFamilyCheckIns, RouteFamilySubmissions, RouteFamilySubmissionCompletions, RouteFamilyChits, RouteFamilyRetrievals, RouteFamilyPayments, RouteFamilyReleaseMaterials, RouteFamilyReleasePublications, RouteFamilyReleasePublicationCompletions, RouteFamilyUpdateChecks, RouteFamilyUpgrades}
	for raw := range 256 {
		t.Run(fmt.Sprintf("backing_byte_%d", raw), func(t *testing.T) {
			t.Parallel()
			got := RouteFamily(raw)
			wantValid := slices.Contains(published, got)
			err := got.Validate()
			if (err == nil) != wantValid || got.IsValid() != wantValid {
				t.Fatalf("family=%v validation=%v valid=%v, want valid=%v", raw, err, got.IsValid(), wantValid)
			}
			if !wantValid {
				if !errors.Is(err, core.ErrControlWireRoute) || got.String() != "" {
					t.Fatalf("family refusal=%v/%q, want route identity and empty", err, got.String())
				}
				return
			}
			if got.String() != routeSuffixes()[got] {
				t.Fatalf("suffix=%q, want %q", got.String(), routeSuffixes()[got])
			}
		})
	}
}

func TestProtocolSupportOutcomeExhaustsBackingByte(t *testing.T) {
	t.Parallel()
	for raw := range 256 {
		t.Run(fmt.Sprintf("outcome_byte_%d", raw), func(t *testing.T) {
			t.Parallel()
			got := ProtocolSupportOutcome(raw)
			want := ""
			switch got {
			case ProtocolSupportOutcomeAccepted:
				want = protocolSupportOutcomeAcceptedDiagnostic
			case ProtocolSupportOutcomeUpgradeRequired:
				want = protocolSupportOutcomeUpgradeRequiredDiagnostic
			}
			err := got.Validate()
			if (err == nil) != (want != "") || got.IsValid() != (want != "") || got.String() != want {
				t.Fatalf("outcome=%q/%v valid=%v, want %q valid=%v", got.String(), err, got.IsValid(), want, want != "")
			}
			if want == "" && !errors.Is(err, core.ErrControlWireProtocolSupport) {
				t.Fatalf("outcome refusal=%v, want %v", err, core.ErrControlWireProtocolSupport)
			}
		})
	}
}

func TestRouteFamilyWireContractAcceptsEveryPublishedTokenAndRejectsHostileDocuments(t *testing.T) {
	t.Parallel()

	families := []RouteFamily{
		RouteFamilyRegistrations, RouteFamilyCheckIns, RouteFamilySubmissions,
		RouteFamilySubmissionCompletions, RouteFamilyChits, RouteFamilyRetrievals,
		RouteFamilyPayments, RouteFamilyReleaseMaterials, RouteFamilyReleasePublications,
		RouteFamilyReleasePublicationCompletions, RouteFamilyUpdateChecks, RouteFamilyUpgrades,
	}
	for _, family := range families {
		t.Run(fmt.Sprintf("published_family_%d", family), func(t *testing.T) {
			t.Parallel()
			encoded, err := family.MarshalJSON()
			if err != nil {
				t.Fatalf("RouteFamily(%v).MarshalJSON() error = %v, want nil", family, err)
			}
			token, err := core.DecodeJSONStringToken(encoded)
			if err != nil || token == "" || strings.HasPrefix(token, routeSeparator) {
				t.Fatalf("published route token = (%q, %v), want unique non-path token", token, err)
			}
			parsed, err := ParseRouteFamily(token)
			if err != nil || parsed != family {
				t.Fatalf("ParseRouteFamily(MarshalJSON(%v)) = (%v, %v), want exact family and nil", family, parsed, err)
			}
			var roundTrip RouteFamily
			if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != family {
				t.Fatalf("RouteFamily.UnmarshalJSON(MarshalJSON(%v)) = (%v, %v), want exact family and nil", family, roundTrip, err)
			}
			second, err := roundTrip.MarshalJSON()
			if err != nil || !bytes.Equal(second, encoded) {
				t.Fatalf("route family canonical fixed point = (%s, %v), want %s", second, err, encoded)
			}
		})
	}
	base := RouteFamilyRegistrations
	baseJSON, err := base.MarshalJSON()
	if err != nil {
		t.Fatalf("RouteFamilyRegistrations.MarshalJSON() error = %v, want nil", err)
	}
	baseToken, err := core.DecodeJSONStringToken(baseJSON)
	if err != nil {
		t.Fatalf("DecodeJSONStringToken(registration family) error = %v, want nil", err)
	}
	stringDocument := func(value string) []byte {
		encoded, encodeErr := core.MarshalCanonicalJSONString(value)
		if encodeErr != nil {
			t.Fatalf("MarshalCanonicalJSONString(hostile route token) error = %v, want nil", encodeErr)
		}
		return encoded
	}
	hostile := []struct {
		name     string
		document []byte
	}{
		{name: "neutral absent input", document: nil},
		{name: "whitespace cannot name a route", document: []byte{' '}},
		{name: "null cannot name a route", document: []byte("null")},
		{name: "object cannot name a scalar route", document: []byte("{}")},
		{name: "array cannot name a scalar route", document: []byte("[]")},
		{name: "boolean cannot select a route", document: []byte("true")},
		{name: "number cannot select a route", document: []byte("0")},
		{name: "truncated document", document: []byte{'{'}},
		{name: "invalid UTF8", document: []byte{0xff}},
		{name: "empty token", document: stringDocument("")},
		{name: "unpublished token", document: stringDocument("unknown")},
		{name: "path suffix is not a wire token", document: stringDocument(base.String())},
		{name: "case-folded token", document: stringDocument(strings.ToUpper(baseToken))},
		{name: "leading token whitespace", document: stringDocument(" " + baseToken)},
		{name: "trailing token whitespace", document: stringDocument(baseToken + " ")},
		{name: "trailing route separator", document: stringDocument(baseToken + routeSeparator)},
		{name: "trailing scalar document", document: append(bytes.Clone(baseJSON), '0')},
		{name: "second complete document", document: append(bytes.Clone(baseJSON), baseJSON...)},
	}
	for _, tc := range hostile {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := RouteFamilyPayments
			err := got.UnmarshalJSON(tc.document)
			if !errors.Is(err, core.ErrControlWireRoute) || !errors.Is(err, core.ErrJSONContract) || got != RouteFamilyPayments {
				t.Fatalf("route=%v/%v, want preserved %v and route/JSON refusal", got, err, RouteFamilyPayments)
			}
		})
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
		RouteFamilySubmissionCompletions, RouteFamilyChits,
		RouteFamilyRetrievals, RouteFamilyPayments,
		RouteFamilyReleaseMaterials,
		RouteFamilyReleasePublications, RouteFamilyReleasePublicationCompletions,
		RouteFamilyUpdateChecks, RouteFamilyUpgrades,
	}
	for _, offering := range []core.Offering{
		{Token: "a"},
		{Token: strings.Repeat("a", core.OfferingCanonicalJSONMaximumBytes-len(`""`))},
		{Token: "a-9"},
	} {
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
