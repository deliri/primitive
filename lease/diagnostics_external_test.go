package lease_test

import (
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
	"github.com/deliri/primitive/v2026/temporal"
)

// TestDiagnosticLabelsAreExhaustiveAndClosed proves every enum and state label
// over its complete underlying domain. A label is the only projection an
// operator sees, so a value that silently renders as another member's label
// would misreport a lease's real status, and a member added without a label
// would render as "unknown" while validating.
func TestDiagnosticLabelsAreExhaustiveAndClosed(t *testing.T) {
	t.Parallel()

	cases := []struct {
		label  func(uint8) string
		valid  func(uint8) bool
		known  map[uint8]string
		name   string
		domain string
	}{
		{
			name: "state",
			known: map[uint8]string{
				uint8(lease.StateNotYetValid): "not-yet-valid",
				uint8(lease.StateCurrent):     "current",
				uint8(lease.StateContinuity):  "continuity",
				uint8(lease.StateExpired):     "expired",
				uint8(lease.StateRefused):     "refused",
				uint8(lease.StateRevoked):     "revoked",
			},
			label: func(value uint8) string { return lease.State(value).String() },
			valid: func(value uint8) bool { return lease.State(value).IsValid() },
		},
		{
			name: "contact state",
			known: map[uint8]string{
				uint8(lease.ContactStateNotDue):     "not-due",
				uint8(lease.ContactStateDue):        "due",
				uint8(lease.ContactStateProhibited): "prohibited",
			},
			label: func(value uint8) string { return lease.ContactState(value).String() },
			valid: func(value uint8) bool { return lease.ContactState(value).IsValid() },
		},
		{
			name: "advance state",
			known: map[uint8]string{
				uint8(lease.AdvanceStateUnchanged): "unchanged",
				uint8(lease.AdvanceStateAdvanced):  "advanced",
			},
			label: func(value uint8) string { return lease.AdvanceState(value).String() },
			valid: func(value uint8) bool { return lease.AdvanceState(value).IsValid() },
		},
		{
			name:  "revision",
			known: map[uint8]string{uint8(lease.RevisionV1): "v1"},
			label: func(value uint8) string { return lease.Revision(value).String() },
			valid: func(value uint8) bool { return lease.Revision(value).IsValid() },
		},
		{
			name: "outcome",
			known: map[uint8]string{
				uint8(lease.OutcomeGrant):      "grant",
				uint8(lease.OutcomeRefusal):    "refusal",
				uint8(lease.OutcomeRevocation): "revocation",
			},
			label: func(value uint8) string { return lease.Outcome(value).String() },
			valid: func(value uint8) bool { return lease.Outcome(value).IsValid() },
		},
		{
			name: "revocation reason",
			known: map[uint8]string{
				uint8(lease.RevocationReasonLicenceBreach):          "licence-breach",
				uint8(lease.RevocationReasonUnlawfulOrAbusiveUse):   "unlawful-or-abusive-use",
				uint8(lease.RevocationReasonSecurityOrPlatformRisk): "security-or-platform-risk",
				uint8(lease.RevocationReasonInsolvency):             "insolvency",
			},
			label: func(value uint8) string { return lease.RevocationReason(value).String() },
			valid: func(value uint8) bool { return lease.RevocationReason(value).IsValid() },
		},
		{
			name:  "signing domain",
			known: map[uint8]string{uint8(lease.DomainDecisionV1): "primitive-lease-decision-v1"},
			label: func(value uint8) string { return lease.Domain(value).String() },
			valid: func(value uint8) bool { return lease.Domain(value).IsValid() },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			seen := make(map[string]uint8, len(tc.known))
			for value := 0; value <= math.MaxUint8; value++ {
				member := uint8(value)
				want, known := tc.known[member]
				if !known {
					want = core.UnknownEnumDiagnostic
				}
				if got := tc.label(member); got != want {
					t.Errorf("%s(%d).String() = %q, want %q", tc.name, member, got, want)
				}
				if got := tc.valid(member); got != known {
					t.Errorf("%s(%d).IsValid() = %t, want %t", tc.name, member, got, known)
				}
				if !known {
					continue
				}
				if duplicate, repeated := seen[want]; repeated {
					t.Errorf("%s label %q names both %d and %d", tc.name, want, duplicate, member)
				}
				seen[want] = member
			}
			if len(seen) != len(tc.known) {
				t.Errorf("%s labelled %d members, want %d", tc.name, len(seen), len(tc.known))
			}
		})
	}
}

// TestUnsetNominalValuesProjectEmptyText proves an unset opaque value renders
// as empty text rather than a plausible-looking identifier, so a diagnostic can
// never present a zero value as a real product, entitlement, installation, or
// generation.
func TestUnsetNominalValuesProjectEmptyText(t *testing.T) {
	t.Parallel()

	cases := []struct {
		project func() string
		name    string
	}{
		{name: "product", project: lease.Product{}.String},
		{name: "entitlement id", project: lease.EntitlementID{}.String},
		{name: "device id", project: lease.DeviceID{}.String},
		{name: "generation", project: lease.Generation{}.String},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := tc.project(); got != "" {
				t.Fatalf("unset %s String() = %q, want empty", tc.name, got)
			}
		})
	}
}

// TestSigningDomainTextRoundTripsExactly proves the one Attest domain token is
// stable in both directions and that nothing else parses. The token separates
// lease decisions from every other Primitive attestation, so a drifting or
// permissive projection would let a signature cross domains.
func TestSigningDomainTextRoundTripsExactly(t *testing.T) {
	t.Parallel()

	text, err := lease.DomainDecisionV1.MarshalText()
	if err != nil {
		t.Fatalf("Domain.MarshalText() error = %v, want nil", err)
	}
	if string(text) != "primitive-lease-decision-v1" {
		t.Fatalf("Domain.MarshalText() = %q, want %q", text, "primitive-lease-decision-v1")
	}
	parsed, err := lease.DomainDecisionV1.ParseCanonicalText(text)
	if err != nil || parsed != lease.DomainDecisionV1 {
		t.Fatalf("Domain.ParseCanonicalText() = (%v, %v), want (%v, nil)", parsed, err, lease.DomainDecisionV1)
	}

	rejected := [][]byte{
		nil,
		[]byte(""),
		[]byte("primitive-lease-decision-v2"),
		[]byte("primitive-lease-decision-v1 "),
		[]byte(" primitive-lease-decision-v1"),
		[]byte("Primitive-Lease-Decision-V1"),
		[]byte("primitive-lease-decision-v1\x00"),
		[]byte("primitive-release-decision-v1"),
		[]byte("unknown"),
	}
	for _, candidate := range rejected {
		got, parseErr := lease.DomainDecisionV1.ParseCanonicalText(candidate)
		if !errors.Is(parseErr, core.ErrLeaseContract) {
			t.Errorf("Domain.ParseCanonicalText(%q) error = %v, want %v", candidate, parseErr, core.ErrLeaseContract)
		}
		if got != lease.DomainUnknown {
			t.Errorf("Domain.ParseCanonicalText(%q) = %v, want %v", candidate, got, lease.DomainUnknown)
		}
	}
	if _, unsetErr := lease.DomainUnknown.MarshalText(); !errors.Is(unsetErr, core.ErrLeaseContract) {
		t.Fatalf("unset Domain.MarshalText() error = %v, want %v", unsetErr, core.ErrLeaseContract)
	}
}

// The four unset-carrier tests below prove no proof-carrying or assessed value
// can be read out of its zero form. Each carrier's unset flag is the only thing
// separating "OGS decided this" from "Go allocated this", so every accessor
// must refuse rather than return a plausible zero.

func TestUnsetVerifiedRefusesEveryProjection(t *testing.T) {
	t.Parallel()

	var verified lease.Verified
	if err := verified.Validate(); !errors.Is(err, core.ErrLeaseVerification) {
		t.Fatalf("Verified.Validate() error = %v, want %v", err, core.ErrLeaseVerification)
	}
	gotDecision, decisionErr := verified.Decision()
	if !errors.Is(decisionErr, core.ErrLeaseVerification) || gotDecision != (lease.Decision{}) {
		t.Fatalf("Verified.Decision() = (%v, %v), want zero and verification error", gotDecision, decisionErr)
	}
	gotEnvelope, envelopeErr := verified.Envelope()
	if !errors.Is(envelopeErr, core.ErrLeaseVerification) ||
		gotEnvelope != (attest.Envelope[lease.Domain]{}) {
		t.Fatalf("Verified.Envelope() = (%v, %v), want zero and verification error", gotEnvelope, envelopeErr)
	}
	gotSubject, subjectErr := verified.Subject()
	if !errors.Is(subjectErr, core.ErrLeaseVerification) || gotSubject != (lease.Subject{}) {
		t.Fatalf("Verified.Subject() = (%v, %v), want zero and verification error", gotSubject, subjectErr)
	}
}

func TestUnsetAssessmentRefusesEveryProjection(t *testing.T) {
	t.Parallel()

	var assessment lease.Assessment
	if err := assessment.Validate(); !errors.Is(err, core.ErrLeaseContract) {
		t.Fatalf("Assessment.Validate() error = %v, want %v", err, core.ErrLeaseContract)
	}
	gotDecision, decisionErr := assessment.Decision()
	if !errors.Is(decisionErr, core.ErrLeaseContract) || gotDecision != (lease.Decision{}) {
		t.Fatalf("Assessment.Decision() = (%v, %v), want zero and contract error", gotDecision, decisionErr)
	}
	gotEffective, effectiveErr := assessment.EffectiveAt()
	if !errors.Is(effectiveErr, core.ErrLeaseContract) || gotEffective != (temporal.Instant{}) {
		t.Fatalf("Assessment.EffectiveAt() = (%v, %v), want zero and contract error", gotEffective, effectiveErr)
	}
	if assessment.State() != lease.StateUnknown {
		t.Fatalf("Assessment.State() = %v, want %v", assessment.State(), lease.StateUnknown)
	}
	if assessment.ContactState() != lease.ContactStateUnknown {
		t.Fatalf("Assessment.ContactState() = %v, want %v", assessment.ContactState(), lease.ContactStateUnknown)
	}
}

func TestUnsetAdvanceResultRefusesItsSelection(t *testing.T) {
	t.Parallel()

	var result lease.AdvanceResult
	if err := result.Validate(); !errors.Is(err, core.ErrLeaseContract) {
		t.Fatalf("AdvanceResult.Validate() error = %v, want %v", err, core.ErrLeaseContract)
	}
	selected, selectedErr := result.Verified()
	if !errors.Is(selectedErr, core.ErrLeaseContract) || selected != (lease.Verified{}) {
		t.Fatalf("AdvanceResult.Verified() = (%v, %v), want zero and contract error", selected, selectedErr)
	}
	if result.State() != lease.AdvanceStateUnknown {
		t.Fatalf("AdvanceResult.State() = %v, want %v", result.State(), lease.AdvanceStateUnknown)
	}
}

func TestUnsetDecisionRefusesItsHeaderAndEveryVariant(t *testing.T) {
	t.Parallel()

	var decision lease.Decision
	if err := decision.Validate(); !errors.Is(err, core.ErrLeaseContract) {
		t.Fatalf("Decision.Validate() error = %v, want %v", err, core.ErrLeaseContract)
	}
	gotHeader, headerErr := decision.Header()
	if !errors.Is(headerErr, core.ErrLeaseContract) || gotHeader != (lease.Header{}) {
		t.Fatalf("Decision.Header() = (%v, %v), want zero and contract error", gotHeader, headerErr)
	}
	if decision.Outcome() != lease.OutcomeUnknown {
		t.Fatalf("Decision.Outcome() = %v, want %v", decision.Outcome(), lease.OutcomeUnknown)
	}
	if _, err := decision.Grant(); !errors.Is(err, core.ErrLeaseContract) {
		t.Fatalf("Decision.Grant() error = %v, want %v", err, core.ErrLeaseContract)
	}
	if _, err := decision.Refusal(); !errors.Is(err, core.ErrLeaseContract) {
		t.Fatalf("Decision.Refusal() error = %v, want %v", err, core.ErrLeaseContract)
	}
	if _, err := decision.Revocation(); !errors.Is(err, core.ErrLeaseContract) {
		t.Fatalf("Decision.Revocation() error = %v, want %v", err, core.ErrLeaseContract)
	}
	if decision.AttestationDomain() != lease.DomainDecisionV1 {
		t.Fatalf("Decision.AttestationDomain() = %v, want %v", decision.AttestationDomain(), lease.DomainDecisionV1)
	}
}

// TestHeaderValidationRejectsEveryUnsetComponent proves the common signed
// header closes each of its four facts independently. A header that accepted
// one unset component would let an unbound or unsequenced decision reach
// verification.
func TestHeaderValidationRejectsEveryUnsetComponent(t *testing.T) {
	t.Parallel()

	subject := fixtureSubject(t, 39)
	valid := fixtureHeader(t, subject, 3, 1_000)

	cases := []struct {
		mutate  func(lease.Header) lease.Header
		wantErr error
		name    string
	}{
		{name: "complete header closes", mutate: func(h lease.Header) lease.Header { return h }},
		{
			name:    "unset revision",
			mutate:  func(h lease.Header) lease.Header { h.Revision = lease.Revision(0); return h },
			wantErr: core.ErrLeaseContract,
		},
		{
			name:    "future revision",
			mutate:  func(h lease.Header) lease.Header { h.Revision = lease.Revision(2); return h },
			wantErr: core.ErrLeaseContract,
		},
		{
			name:    "unset product",
			mutate:  func(h lease.Header) lease.Header { h.Subject.Product = lease.Product{}; return h },
			wantErr: core.ErrLeaseContract,
		},
		{
			name:    "unset entitlement",
			mutate:  func(h lease.Header) lease.Header { h.Subject.EntitlementID = lease.EntitlementID{}; return h },
			wantErr: core.ErrLeaseContract,
		},
		{
			name:    "unset device",
			mutate:  func(h lease.Header) lease.Header { h.Subject.DeviceID = lease.DeviceID{}; return h },
			wantErr: core.ErrLeaseContract,
		},
		{
			name:    "unset generation",
			mutate:  func(h lease.Header) lease.Header { h.Generation = lease.Generation{}; return h },
			wantErr: core.ErrLeaseContract,
		},
		{
			name:    "unset issuance",
			mutate:  func(h lease.Header) lease.Header { h.IssuedAt = temporal.Instant{}; return h },
			wantErr: core.ErrLeaseContract,
		},
		{
			name:   "minimum representable issuance",
			mutate: func(h lease.Header) lease.Header { h.IssuedAt = fixtureInstant(math.MinInt64); return h },
		},
		{
			name:   "maximum representable issuance",
			mutate: func(h lease.Header) lease.Header { h.IssuedAt = fixtureInstant(math.MaxInt64); return h },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := tc.mutate(valid)
			if err := got.Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("Header.Validate() error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}
