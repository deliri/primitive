package lease

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

type (
	protocolFact[T any]      struct{}
	internalFlow[T any]      struct{}
	capabilityWrapper[T any] struct{}
	typedFailure[T any]      struct{}
)

type refusalV1Contract struct {
	ContactAfter temporal.Instant `json:"contact_after"`
}

var (
	_ Refusal           = Refusal(refusalV1Contract{})
	_ refusalV1Contract = refusalV1Contract(Refusal{})
)

type leaseContractInventory struct {
	ScopeMismatch             typedFailure[ScopeMismatch]
	ClockContradiction        typedFailure[ClockContradiction]
	identifier                internalFlow[identifier]
	Product                   protocolFact[Product]
	EntitlementID             protocolFact[EntitlementID]
	DeviceID                  protocolFact[DeviceID]
	Generation                protocolFact[Generation]
	enumFact                  internalFlow[enumFact]
	jsonStructureContract     internalFlow[jsonStructureContract]
	Subject                   protocolFact[Subject]
	Header                    protocolFact[Header]
	Grant                     protocolFact[Grant]
	Refusal                   protocolFact[Refusal]
	Revocation                protocolFact[Revocation]
	GrantDecisionRequest      protocolFact[GrantDecisionRequest]
	RefusalDecisionRequest    protocolFact[RefusalDecisionRequest]
	RevocationDecisionRequest protocolFact[RevocationDecisionRequest]
	Decision                  protocolFact[Decision]
	decisionWire              internalFlow[decisionWire]
	Document                  protocolFact[Document]
	VerifyRequest             protocolFact[VerifyRequest]
	Verified                  capabilityWrapper[Verified]
	EvaluateRequest           protocolFact[EvaluateRequest]
	Assessment                capabilityWrapper[Assessment]
	AdvanceRequest            protocolFact[AdvanceRequest]
	AdvanceResult             capabilityWrapper[AdvanceResult]
}

var _ leaseContractInventory

var (
	_ = leaseContractInventory{}.identifier
	_ = leaseContractInventory{}.enumFact
	_ = leaseContractInventory{}.jsonStructureContract
	_ = leaseContractInventory{}.decisionWire
)

func TestProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()

	got, err := productionStructNames()
	if err != nil {
		t.Fatalf("productionStructNames() error = %v, want nil", err)
	}
	want, err := inventoryStructNames()
	if err != nil {
		t.Fatalf("inventoryStructNames() error = %v, want nil", err)
	}
	for _, name := range got {
		if !slices.Contains(want, name) {
			t.Errorf("production struct %q has no compiler-visible data-flow role", name)
		}
	}
	for _, name := range want {
		if !slices.Contains(got, name) {
			t.Errorf("classified struct %q does not exist in production", name)
		}
	}
}

func TestPublicOperationsAreExactIntentEntryPoints(t *testing.T) {
	t.Parallel()

	got, err := exportedFunctionNames()
	if err != nil {
		t.Fatalf("exportedFunctionNames() error = %v, want nil", err)
	}
	want := []string{
		"Advance",
		"Evaluate",
		"NewDeviceID",
		"NewEntitlementID",
		"NewGeneration",
		"NewGrantDecision",
		"NewProduct",
		"NewRefusalDecision",
		"NewRevocationDecision",
		"ParseDeviceID",
		"ParseEntitlementID",
		"ParseGeneration",
		"ParseOutcome",
		"ParseProduct",
		"ParseRevision",
		"ParseRevocationReason",
		"Verify",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("exported top-level functions = %v, want %v", got, want)
	}
}

// TestDecisionV1CanonicalBodyVector pins the exact signed body without a
// second string protocol. The typed SHA-256 vector fails on any member,
// spelling, order, or scalar-projection drift while the production structs
// remain the sole readable wire schema.
func TestDecisionV1CanonicalBodyVector(t *testing.T) {
	t.Parallel()

	decision, err := NewGrantDecision(GrantDecisionRequest{
		Header: fixtureInternalHeader(t),
		Grant:  fixtureInternalGrant(),
	})
	if err != nil {
		t.Fatalf("NewGrantDecision() error = %v, want nil", err)
	}
	encoded, err := decision.MarshalJSON()
	if err != nil {
		t.Fatalf("Decision.MarshalJSON() error = %v, want nil", err)
	}
	got := sha256.Sum256(encoded)
	want := [sha256.Size]byte{
		0xe9, 0x66, 0x34, 0x99, 0xc0, 0x73, 0x39, 0x60,
		0x7c, 0x1b, 0x02, 0x1f, 0x0a, 0x1e, 0xed, 0x0f,
		0x31, 0x4c, 0xac, 0x4b, 0x0a, 0x5b, 0x60, 0x7f,
		0x66, 0x5e, 0xa9, 0xec, 0x71, 0xb3, 0x56, 0x85,
	}
	if got != want {
		t.Fatalf("SHA-256(Decision v1 canonical body) = %x, want %x", got, want)
	}
}

func TestDecisionUnionRejectsEveryDormantPayload(t *testing.T) {
	t.Parallel()

	header := fixtureInternalHeader(t)
	grant := fixtureInternalGrant()
	refusal := Refusal{ContactAfter: fixtureInternalInstant(6_000)}
	revocation := Revocation{Reason: RevocationReasonLicenceBreach}
	cases := []struct {
		name     string
		decision Decision
	}{
		{name: "grant plus refusal", decision: Decision{header: header, outcome: OutcomeGrant, grant: grant, refusal: refusal}},
		{name: "grant plus revocation", decision: Decision{header: header, outcome: OutcomeGrant, grant: grant, revocation: revocation}},
		{name: "refusal plus grant", decision: Decision{header: header, outcome: OutcomeRefusal, grant: grant, refusal: refusal}},
		{name: "refusal plus revocation", decision: Decision{header: header, outcome: OutcomeRefusal, refusal: refusal, revocation: revocation}},
		{name: "revocation plus grant", decision: Decision{header: header, outcome: OutcomeRevocation, grant: grant, revocation: revocation}},
		{name: "revocation plus refusal", decision: Decision{header: header, outcome: OutcomeRevocation, refusal: refusal, revocation: revocation}},
		{name: "unknown with all payloads", decision: Decision{header: header, grant: grant, refusal: refusal, revocation: revocation}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if err := tc.decision.Validate(); !errors.Is(err, core.ErrLeaseContract) {
				t.Fatalf("Decision.Validate() error = %v, want %v", err, core.ErrLeaseContract)
			}
		})
	}
}

func TestDecisionTaggedUnionRejectsContradictoryWireBodies(t *testing.T) {
	t.Parallel()

	header := fixtureInternalHeader(t)
	grant := fixtureInternalGrant()
	valid, err := NewGrantDecision(GrantDecisionRequest{Header: header, Grant: grant})
	if err != nil {
		t.Fatalf("NewGrantDecision() error = %v, want nil", err)
	}
	validJSON, err := valid.MarshalJSON()
	if err != nil {
		t.Fatalf("Decision.MarshalJSON() error = %v, want nil", err)
	}
	grantJSON, err := grant.MarshalJSON()
	if err != nil {
		t.Fatalf("Grant.MarshalJSON() error = %v, want nil", err)
	}
	refusalJSON, err := (Refusal{
		ContactAfter: fixtureInternalInstant(6_000),
	}).MarshalJSON()
	if err != nil {
		t.Fatalf("Refusal.MarshalJSON() error = %v, want nil", err)
	}
	revision := header.Revision
	subject := header.Subject
	generation := header.Generation
	issuedAt := header.IssuedAt
	outcome := OutcomeGrant
	grantBody := json.RawMessage(grantJSON)
	base := decisionWire{
		Revision: &revision, Subject: &subject,
		Generation: &generation, IssuedAt: &issuedAt,
		Outcome: &outcome, Body: &grantBody,
	}
	refusalRaw := json.RawMessage(refusalJSON)
	refusalBody, err := json.Marshal(decisionWire{
		Revision: base.Revision, Subject: base.Subject,
		Generation: base.Generation, IssuedAt: base.IssuedAt,
		Outcome: base.Outcome, Body: &refusalRaw,
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v, want nil", err)
	}
	nullRaw := json.RawMessage("null")
	nullBody, err := json.Marshal(decisionWire{
		Revision: base.Revision, Subject: base.Subject,
		Generation: base.Generation, IssuedAt: base.IssuedAt,
		Outcome: base.Outcome, Body: &nullRaw,
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v, want nil", err)
	}
	emptyRaw := json.RawMessage("{}")
	emptyBody, err := json.Marshal(decisionWire{
		Revision: base.Revision, Subject: base.Subject,
		Generation: base.Generation, IssuedAt: base.IssuedAt,
		Outcome: base.Outcome, Body: &emptyRaw,
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v, want nil", err)
	}
	futureOutcome := bytes.Replace(
		validJSON,
		[]byte(`"outcome":"`+outcomeGrantToken+`"`),
		[]byte(`"outcome":"future"`),
		1,
	)
	type missingBodyWire struct {
		IssuedAt   temporal.Instant `json:"issued_at"`
		Generation Generation       `json:"generation"`
		Subject    Subject          `json:"subject"`
		Revision   Revision         `json:"revision"`
		Outcome    Outcome          `json:"outcome"`
	}
	missingBody, err := json.Marshal(missingBodyWire{
		Revision: revision, Subject: subject,
		Generation: generation, IssuedAt: issuedAt,
		Outcome: *base.Outcome,
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v, want nil", err)
	}
	cases := []struct {
		name string
		data []byte
	}{
		{name: "grant discriminator with refusal body", data: refusalBody},
		{name: "grant discriminator with null body", data: nullBody},
		{name: "grant discriminator with empty body", data: emptyBody},
		{name: "future discriminator", data: futureOutcome},
		{name: "missing body", data: missingBody},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := valid
			if err := got.UnmarshalJSON(tc.data); !errors.Is(err, core.ErrLeaseContract) {
				t.Fatalf("Decision.UnmarshalJSON() error = %v, want %v", err, core.ErrLeaseContract)
			}
			if got != valid {
				t.Fatalf("rejected Decision mutated receiver")
			}
		})
	}
}

func TestOffWireDomainsExhaustAllUnderlyingValues(t *testing.T) {
	t.Parallel()

	for value := uint16(0); value <= 255; value++ {
		state := State(value)
		wantState := state > StateUnknown && state < stateLimit
		if state.IsValid() != wantState {
			t.Errorf("State(%d).IsValid() = %t, want %t", value, state.IsValid(), wantState)
		}
		contact := ContactState(value)
		wantContact := contact > ContactStateUnknown && contact < contactStateLimit
		if contact.IsValid() != wantContact {
			t.Errorf("ContactState(%d).IsValid() = %t, want %t", value, contact.IsValid(), wantContact)
		}
		advance := AdvanceState(value)
		wantAdvance := advance > AdvanceStateUnknown && advance < advanceStateLimit
		if advance.IsValid() != wantAdvance {
			t.Errorf("AdvanceState(%d).IsValid() = %t, want %t", value, advance.IsValid(), wantAdvance)
		}
		domain := Domain(value)
		wantDomain := domain == DomainDecisionV1
		if domain.IsValid() != wantDomain {
			t.Errorf("Domain(%d).IsValid() = %t, want %t", value, domain.IsValid(), wantDomain)
		}
	}
}

// TestAssessmentRejectsATamperedProjection proves Assessment.Validate
// recomputes its classification from the decision and effective instant rather
// than trusting the stored fields. Assessment is the value a consumer branches
// on before creating paid work, so a carrier whose stored state disagrees with
// its own facts must refuse every projection.
func TestAssessmentRejectsATamperedProjection(t *testing.T) {
	t.Parallel()

	header := fixtureInternalHeader(t)
	decision, err := NewGrantDecision(GrantDecisionRequest{
		Header: header, Grant: fixtureInternalGrant(),
	})
	if err != nil {
		t.Fatalf("NewGrantDecision() error = %v, want nil", err)
	}
	// The fixture grant runs [2000, 4000) with contact at 3000, so an effective
	// instant of 2500 is exactly StateCurrent and ContactStateNotDue.
	effective := fixtureInternalInstant(2_500)
	honest := Assessment{
		decision: decision, effectiveAt: effective,
		state: StateCurrent, contact: ContactStateNotDue, valid: true,
	}
	if err := honest.Validate(); err != nil {
		t.Fatalf("consistent Assessment.Validate() error = %v, want nil", err)
	}

	cases := []struct {
		mutate func(Assessment) Assessment
		name   string
	}{
		{
			name:   "unset carrier",
			mutate: func(a Assessment) Assessment { a.valid = false; return a },
		},
		{
			name:   "state claims expired while current",
			mutate: func(a Assessment) Assessment { a.state = StateExpired; return a },
		},
		{
			name:   "state claims not yet valid while current",
			mutate: func(a Assessment) Assessment { a.state = StateNotYetValid; return a },
		},
		{
			name:   "state claims revoked while granted",
			mutate: func(a Assessment) Assessment { a.state = StateRevoked; return a },
		},
		{
			name:   "state outside the closed domain",
			mutate: func(a Assessment) Assessment { a.state = stateLimit; return a },
		},
		{
			name:   "contact claims due while not due",
			mutate: func(a Assessment) Assessment { a.contact = ContactStateDue; return a },
		},
		{
			name:   "contact claims prohibited while granted",
			mutate: func(a Assessment) Assessment { a.contact = ContactStateProhibited; return a },
		},
		{
			name:   "contact outside the closed domain",
			mutate: func(a Assessment) Assessment { a.contact = contactStateLimit; return a },
		},
		{
			name: "effective instant moved past expiry without restating the state",
			mutate: func(a Assessment) Assessment {
				a.effectiveAt = fixtureInternalInstant(9_000)
				return a
			},
		},
		{
			name: "effective instant unset",
			mutate: func(a Assessment) Assessment {
				a.effectiveAt = temporal.Instant{}
				return a
			},
		},
		{
			name: "decision unset",
			mutate: func(a Assessment) Assessment {
				a.decision = Decision{}
				return a
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := tc.mutate(honest)
			if err := got.Validate(); !errors.Is(err, core.ErrLeaseContract) {
				t.Fatalf("Assessment.Validate() error = %v, want %v", err, core.ErrLeaseContract)
			}
			if _, decisionErr := got.Decision(); !errors.Is(decisionErr, core.ErrLeaseContract) {
				t.Fatalf("Assessment.Decision() error = %v, want %v", decisionErr, core.ErrLeaseContract)
			}
			if _, effectiveErr := got.EffectiveAt(); !errors.Is(effectiveErr, core.ErrLeaseContract) {
				t.Fatalf("Assessment.EffectiveAt() error = %v, want %v", effectiveErr, core.ErrLeaseContract)
			}
		})
	}
}

// TestOutcomeOutsideTheClosedDomainRefusesEveryPath proves each outcome switch
// refuses an out-of-domain discriminator instead of falling through to a
// default that would treat it as one of the real variants.
func TestOutcomeOutsideTheClosedDomainRefusesEveryPath(t *testing.T) {
	t.Parallel()

	header := fixtureInternalHeader(t)
	rogue := Decision{header: header, outcome: outcomeLimit, grant: fixtureInternalGrant()}

	if err := rogue.Validate(); !errors.Is(err, core.ErrLeaseContract) {
		t.Fatalf("Decision.Validate() error = %v, want %v", err, core.ErrLeaseContract)
	}
	if err := rogue.validateUnion(); !errors.Is(err, core.ErrLeaseContract) {
		t.Fatalf("Decision.validateUnion() error = %v, want %v", err, core.ErrLeaseContract)
	}
	if err := rogue.validateIssuedAt(); !errors.Is(err, core.ErrLeaseContract) {
		t.Fatalf("Decision.validateIssuedAt() error = %v, want %v", err, core.ErrLeaseContract)
	}
	if _, err := rogue.marshalBody(); !errors.Is(err, core.ErrLeaseContract) {
		t.Fatalf("Decision.marshalBody() error = %v, want %v", err, core.ErrLeaseContract)
	}
	if _, err := rogue.MarshalJSON(); !errors.Is(err, core.ErrLeaseContract) {
		t.Fatalf("Decision.MarshalJSON() error = %v, want %v", err, core.ErrLeaseContract)
	}
	state, contact, err := classifyDecision(rogue, fixtureInternalInstant(2_500))
	if !errors.Is(err, core.ErrLeaseContract) {
		t.Fatalf("classifyDecision() error = %v, want %v", err, core.ErrLeaseContract)
	}
	if state != StateUnknown || contact != ContactStateUnknown {
		t.Fatalf("classifyDecision() = (%v, %v), want the unknown members", state, contact)
	}
	revision := header.Revision
	subject := header.Subject
	generation := header.Generation
	issuedAt := header.IssuedAt
	outcome := outcomeLimit
	body := json.RawMessage(`{}`)
	if _, err := decisionFromWire(decisionWire{
		Revision: &revision, Subject: &subject,
		Generation: &generation, IssuedAt: &issuedAt,
		Outcome: &outcome, Body: &body,
	}); !errors.Is(err, core.ErrLeaseContract) {
		t.Fatalf("decisionFromWire() error = %v, want %v", err, core.ErrLeaseContract)
	}
}

func productionStructNames() ([]string, error) {
	files, err := productionFiles()
	if err != nil {
		return nil, err
	}
	var names []string
	set := token.NewFileSet()
	for _, name := range files {
		file, err := parser.ParseFile(set, name, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		for _, declaration := range file.Decls {
			generic, ok := declaration.(*ast.GenDecl)
			if !ok || generic.Tok != token.TYPE {
				continue
			}
			for _, raw := range generic.Specs {
				spec := raw.(*ast.TypeSpec)
				if _, ok := spec.Type.(*ast.StructType); ok {
					names = append(names, spec.Name.Name)
				}
			}
		}
	}
	slices.Sort(names)
	return names, nil
}

func inventoryStructNames() ([]string, error) {
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, "internal_contract_test.go", nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	for _, declaration := range file.Decls {
		generic, ok := declaration.(*ast.GenDecl)
		if !ok || generic.Tok != token.TYPE {
			continue
		}
		for _, raw := range generic.Specs {
			spec := raw.(*ast.TypeSpec)
			if spec.Name.Name != "leaseContractInventory" {
				continue
			}
			structure := spec.Type.(*ast.StructType)
			var names []string
			for _, field := range structure.Fields.List {
				for _, name := range field.Names {
					names = append(names, name.Name)
				}
			}
			slices.Sort(names)
			return names, nil
		}
	}
	return nil, core.ErrLeaseContract
}

func exportedFunctionNames() ([]string, error) {
	files, err := productionFiles()
	if err != nil {
		return nil, err
	}
	set := token.NewFileSet()
	var names []string
	for _, name := range files {
		file, err := parser.ParseFile(set, name, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if ok && function.Recv == nil && function.Name.IsExported() {
				names = append(names, function.Name.Name)
			}
		}
	}
	slices.Sort(names)
	return names, nil
}

func productionFiles() ([]string, error) {
	entries, err := os.ReadDir(".")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") &&
			!strings.HasSuffix(entry.Name(), "_test.go") {
			names = append(names, entry.Name())
		}
	}
	slices.Sort(names)
	return names, nil
}
