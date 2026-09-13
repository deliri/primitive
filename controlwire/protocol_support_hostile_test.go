package controlwire_test

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
)

func protocolFamilyInventory() [13]controlwire.RouteFamily {
	return [...]controlwire.RouteFamily{
		controlwire.RouteFamilyRegistrations,
		controlwire.RouteFamilyCheckIns,
		controlwire.RouteFamilySubmissions,
		controlwire.RouteFamilySubmissionCompletions,
		controlwire.RouteFamilyChits,
		controlwire.RouteFamilyRetrievals,
		controlwire.RouteFamilyPayments,
		controlwire.RouteFamilyReleaseMaterials,
		controlwire.RouteFamilyReleasePublications,
		controlwire.RouteFamilyReleasePublicationCompletions,
		controlwire.RouteFamilyUpdateChecks,
		controlwire.RouteFamilyUpgrades,
		controlwire.RouteFamilyActivations,
	}
}

// Exhaust the entire published subset domain. Each mask differs in at least
// one admitted capability, so every row can expose a wrong membership decision.
func TestProtocolSupportExhaustsEveryBoundedSubsetWithoutAliasing(t *testing.T) {
	t.Parallel()
	families := protocolFamilyInventory()
	all := protocolCapabilities(families[:])
	for mask := uint16(1); mask < uint16(1)<<len(all); mask++ {
		t.Run(fmt.Sprintf("membership_%013b", mask), func(t *testing.T) {
			t.Parallel()
			wantMembers := protocolCapabilitiesFromMask(mask)
			input := slices.Clone(wantMembers)
			slices.Reverse(input)
			before := slices.Clone(input)
			got, err := controlwire.NewProtocolSupport(controlwire.ProtocolSupportRequest{Capabilities: input})
			if err != nil || got.Validate() != nil || !slices.Equal(input, before) {
				t.Fatalf("support=%v/%v input=%v, want valid and unchanged %v", got, err, input, before)
			}
			clear(input)
			for _, candidate := range all {
				want := controlwire.ProtocolSupportOutcomeUpgradeRequired
				if slices.Contains(wantMembers, candidate) {
					want = controlwire.ProtocolSupportOutcomeAccepted
				}
				assessment, err := controlwire.AssessProtocol(controlwire.ProtocolAssessmentRequest{Support: got, Capability: candidate})
				if err != nil || assessment.Capability != candidate || assessment.Outcome != want {
					t.Fatalf("assessment=%v/%v, want %v/%v", assessment, err, candidate, want)
				}
			}
		})
	}
}

func TestProtocolSupportRejectsMalformedPolicies(t *testing.T) {
	t.Parallel()

	families := protocolFamilyInventory()
	all := protocolCapabilities(families[:])
	aboveMaximum := append(slices.Clone(all), all[0])
	cases := []struct {
		name         string
		capabilities []controlwire.ProtocolCapability
	}{
		{name: "nil policy", capabilities: nil},
		{name: "one above fixed ceiling", capabilities: aboveMaximum},
		{name: "zero capability", capabilities: []controlwire.ProtocolCapability{{}}},
		{name: "zero revision", capabilities: []controlwire.ProtocolCapability{{Family: controlwire.RouteFamilyRegistrations}}},
		{name: "zero route family", capabilities: []controlwire.ProtocolCapability{{Revision: controlwire.Revision2026V1}}},
		{name: "future revision", capabilities: []controlwire.ProtocolCapability{{Revision: controlwire.Revision(math.MaxUint8), Family: controlwire.RouteFamilyRegistrations}}},
		{name: "future route family", capabilities: []controlwire.ProtocolCapability{{Revision: controlwire.Revision2026V1, Family: controlwire.RouteFamily(math.MaxUint8)}}},
		{name: "adjacent duplicate", capabilities: []controlwire.ProtocolCapability{all[0], all[0]}},
		{name: "separated duplicate", capabilities: []controlwire.ProtocolCapability{all[0], all[1], all[0]}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := controlwire.ProtocolSupportRequest{Capabilities: tc.capabilities}
			if err := request.Validate(); !errors.Is(err, core.ErrControlWireProtocolSupport) || !errors.Is(err, core.ErrControlWireContract) {
				t.Fatalf("ProtocolSupportRequest.Validate() error = %v, want %v/%v", err, core.ErrControlWireProtocolSupport, core.ErrControlWireContract)
			}
			got, err := controlwire.NewProtocolSupport(request)
			if !errors.Is(err, core.ErrControlWireProtocolSupport) || !errors.Is(got.Validate(), core.ErrControlWireProtocolSupport) {
				t.Fatalf("NewProtocolSupport() = (%+v, %v), want invalid zero and %v", got, err, core.ErrControlWireProtocolSupport)
			}
		})
	}
}

func TestProtocolAssessmentExhaustsEveryPublishedExactAndNearMissPair(t *testing.T) {
	t.Parallel()

	families := protocolFamilyInventory()
	all := protocolCapabilities(families[:])
	for supportedIndex, capability := range all {
		support, err := controlwire.NewProtocolSupport(controlwire.ProtocolSupportRequest{
			Capabilities: []controlwire.ProtocolCapability{capability},
		})
		if err != nil {
			t.Fatalf("NewProtocolSupport(exact pair %d) error = %v, want nil", supportedIndex, err)
		}
		for candidateIndex, candidate := range all {
			assessment, assessErr := controlwire.AssessProtocol(controlwire.ProtocolAssessmentRequest{
				Support: support, Capability: candidate,
			})
			want := controlwire.ProtocolSupportOutcomeUpgradeRequired
			if candidateIndex == supportedIndex {
				want = controlwire.ProtocolSupportOutcomeAccepted
			}
			if assessErr != nil || assessment.Validate() != nil || assessment.Capability != candidate || assessment.Outcome != want {
				t.Fatalf("AssessProtocol(support %d, candidate %d) = (%+v, %v), want exact candidate and outcome %v", supportedIndex, candidateIndex, assessment, assessErr, want)
			}
		}
	}
}

func TestProtocolAssessmentRejectsMalformedBoundaries(t *testing.T) {
	t.Parallel()

	support, err := controlwire.PublishedProtocolSupport()
	if err != nil {
		t.Fatalf("PublishedProtocolSupport() error = %v, want nil", err)
	}
	valid := controlwire.ProtocolCapability{Revision: controlwire.Revision2026V1, Family: controlwire.RouteFamilyRegistrations}
	cases := []struct {
		name    string
		request controlwire.ProtocolAssessmentRequest
	}{
		{name: "zero request"},
		{name: "zero support with valid capability", request: controlwire.ProtocolAssessmentRequest{Capability: valid}},
		{name: "valid support with zero capability", request: controlwire.ProtocolAssessmentRequest{Support: support}},
		{name: "zero revision", request: controlwire.ProtocolAssessmentRequest{Support: support, Capability: controlwire.ProtocolCapability{Family: valid.Family}}},
		{name: "zero family", request: controlwire.ProtocolAssessmentRequest{Support: support, Capability: controlwire.ProtocolCapability{Revision: valid.Revision}}},
		{name: "revision one above published", request: controlwire.ProtocolAssessmentRequest{Support: support, Capability: controlwire.ProtocolCapability{Revision: controlwire.Revision2026V1 + 1, Family: valid.Family}}},
		{name: "revision maximum", request: controlwire.ProtocolAssessmentRequest{Support: support, Capability: controlwire.ProtocolCapability{Revision: controlwire.Revision(math.MaxUint8), Family: valid.Family}}},
		{name: "family maximum", request: controlwire.ProtocolAssessmentRequest{Support: support, Capability: controlwire.ProtocolCapability{Revision: valid.Revision, Family: controlwire.RouteFamily(math.MaxUint8)}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := tc.request.Validate(); !errors.Is(err, core.ErrControlWireProtocolSupport) {
				t.Fatalf("ProtocolAssessmentRequest.Validate() error = %v, want %v", err, core.ErrControlWireProtocolSupport)
			}
			assessment, err := controlwire.AssessProtocol(tc.request)
			if !errors.Is(err, core.ErrControlWireProtocolSupport) || !errors.Is(assessment.Validate(), core.ErrControlWireProtocolSupport) {
				t.Fatalf("AssessProtocol() = (%+v, %v), want invalid zero and %v", assessment, err, core.ErrControlWireProtocolSupport)
			}
		})
	}
}

func FuzzProtocolAssessmentMatchesExactBoundedMembership(f *testing.F) {
	f.Add(uint16(1), uint8(controlwire.Revision2026V1), uint8(controlwire.RouteFamilyRegistrations), false)
	f.Add(uint16(1), uint8(controlwire.Revision2026V1), uint8(controlwire.RouteFamilyCheckIns), false)
	f.Add(uint16(math.MaxUint16), uint8(controlwire.Revision2026V1), uint8(controlwire.RouteFamilyUpgrades), false)
	f.Add(uint16(0), uint8(controlwire.Revision2026V1), uint8(controlwire.RouteFamilyRegistrations), false)
	f.Add(uint16(1), uint8(controlwire.RevisionUnknown), uint8(controlwire.RouteFamilyRegistrations), false)
	f.Add(uint16(1), uint8(controlwire.Revision2026V1), uint8(math.MaxUint8), false)
	f.Add(uint16(1), uint8(controlwire.Revision2026V1), uint8(controlwire.RouteFamilyRegistrations), true)

	f.Fuzz(func(t *testing.T, rawMask uint16, rawRevision, rawFamily uint8, duplicate bool) {
		capabilities := protocolCapabilitiesFromMask(rawMask)
		if duplicate && len(capabilities) != 0 {
			capabilities = append(capabilities, capabilities[0])
		}
		support, supportErr := controlwire.NewProtocolSupport(controlwire.ProtocolSupportRequest{Capabilities: capabilities})
		if len(capabilities) == 0 || duplicate {
			if !errors.Is(supportErr, core.ErrControlWireProtocolSupport) || !errors.Is(support.Validate(), core.ErrControlWireProtocolSupport) {
				t.Fatalf("NewProtocolSupport(mask=%d duplicate=%t) = (%+v, %v), want typed invalid zero", rawMask, duplicate, support, supportErr)
			}
			return
		}
		if supportErr != nil || support.Validate() != nil {
			t.Fatalf("NewProtocolSupport(mask=%d) = (%+v, %v), want valid and nil", rawMask, support, supportErr)
		}
		candidate := controlwire.ProtocolCapability{Revision: controlwire.Revision(rawRevision), Family: controlwire.RouteFamily(rawFamily)}
		assessment, assessErr := controlwire.AssessProtocol(controlwire.ProtocolAssessmentRequest{Support: support, Capability: candidate})
		families := protocolFamilyInventory()
		if candidate.Revision != controlwire.Revision2026V1 || !slices.Contains(families[:], candidate.Family) {
			if !errors.Is(assessErr, core.ErrControlWireProtocolSupport) || !errors.Is(assessment.Validate(), core.ErrControlWireProtocolSupport) {
				t.Fatalf("AssessProtocol(invalid %+v) = (%+v, %v), want typed invalid zero", candidate, assessment, assessErr)
			}
			return
		}
		familyBit := uint(candidate.Family - controlwire.RouteFamilyRegistrations)
		want := controlwire.ProtocolSupportOutcomeUpgradeRequired
		if rawMask&(uint16(1)<<familyBit) != 0 {
			want = controlwire.ProtocolSupportOutcomeAccepted
		}
		if assessErr != nil || assessment.Validate() != nil || assessment.Capability != candidate || assessment.Outcome != want {
			t.Fatalf("AssessProtocol(mask=%d candidate=%+v) = (%+v, %v), want exact outcome %v", rawMask, candidate, assessment, assessErr, want)
		}
	})
}

func protocolCapabilities(families []controlwire.RouteFamily) []controlwire.ProtocolCapability {
	capabilities := make([]controlwire.ProtocolCapability, len(families))
	for index, family := range families {
		capabilities[index] = controlwire.ProtocolCapability{Revision: controlwire.Revision2026V1, Family: family}
	}
	return capabilities
}

func protocolCapabilitiesFromMask(mask uint16) []controlwire.ProtocolCapability {
	families := protocolFamilyInventory()
	capabilities := make([]controlwire.ProtocolCapability, 0, len(families))
	for index, family := range families {
		if mask&(uint16(1)<<uint(index)) != 0 {
			capabilities = append(capabilities, controlwire.ProtocolCapability{Revision: controlwire.Revision2026V1, Family: family})
		}
	}
	return capabilities
}
