package reviewcontrol

import (
	"context"
	"crypto/ed25519"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
	primitiveid "github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/projectstandards"
	"github.com/deliri/primitive/v2026/proofledger"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/temporal"
)

func reviewUUID(t testing.TB, index int) primitiveid.UUIDv7 {
	t.Helper()
	values := [...]string{
		"01890f42-6a00-7000-8000-000000000011", "01890f42-6a00-7000-8000-000000000012",
		"01890f42-6a00-7000-8000-000000000013", "01890f42-6a00-7000-8000-000000000014",
		"01890f42-6a00-7000-8000-000000000015", "01890f42-6a00-7000-8000-000000000016",
		"01890f42-6a00-7000-8000-000000000017", "01890f42-6a00-7000-8000-000000000018",
		"01890f42-6a00-7000-8000-000000000019", "01890f42-6a00-7000-8000-00000000001a",
	}
	got, err := primitiveid.ParseUUIDv7(values[index])
	if err != nil {
		t.Fatalf("ParseUUIDv7(%q) error = %v, want nil", values[index], err)
	}
	return got
}
func reviewInstant(t testing.TB, index int) temporal.Instant {
	t.Helper()
	values := []string{"2026-09-02T00:00:01Z", "2026-09-02T00:00:02Z", "2026-09-02T00:00:03Z", "2026-09-02T00:00:04Z"}
	got, err := temporal.ParseRFC3339(values[index])
	if err != nil {
		t.Fatalf("ParseRFC3339(%q) error = %v, want nil", values[index], err)
	}
	return got
}
func reviewNonce(t testing.TB, value byte) controlwire.RequestNonce {
	t.Helper()
	var raw [core.SHA256DigestBytes]byte
	raw[0] = value
	got, err := controlwire.NewRequestNonce(raw)
	if err != nil {
		t.Fatalf("NewRequestNonce() error = %v, want nil", err)
	}
	return got
}
func reviewKey(t testing.TB, seed byte) (ed25519.PrivateKey, core.Ed25519PublicKey) {
	t.Helper()
	raw := make([]byte, ed25519.SeedSize)
	raw[0] = seed
	private := ed25519.NewKeyFromSeed(raw)
	public, err := core.NewEd25519PublicKey(private.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatalf("NewEd25519PublicKey() error = %v, want nil", err)
	}
	return private, public
}

func reviewSubject(t testing.TB) Subject {
	t.Helper()
	repository, err := projectstandards.NewRepositoryIdentity("github.com/deliri/primitive")
	if err != nil {
		t.Fatalf("NewRepositoryIdentity() error = %v, want nil", err)
	}
	module, err := gomodule.ParsePath(core.PrimitiveModulePath)
	if err != nil {
		t.Fatalf("ParsePath() error = %v, want nil", err)
	}
	packagePath, err := gomodule.ParseImportPath(core.PrimitiveModulePath + "/reviewcontrol")
	if err != nil {
		t.Fatalf("ParseImportPath() error = %v, want nil", err)
	}
	file, err := projectstandards.ParseSourcePath("reviewcontrol/contract.go")
	if err != nil {
		t.Fatalf("ParseSourcePath() error = %v, want nil", err)
	}
	commit, err := core.ParseBuildCommit("0123456789abcdef0123456789abcdef01234567")
	if err != nil {
		t.Fatalf("ParseBuildCommit() error = %v, want nil", err)
	}
	count, err := core.NewByteCount(512)
	if err != nil {
		t.Fatalf("NewByteCount() error = %v, want nil", err)
	}
	return Subject{
		Project: projectstandards.SubjectIdentity{Project: core.Offering{Token: "primitive"}, Repository: repository},
		Source:  projectstandards.SourceCoordinate{Repository: repository, Commit: commit, Tree: core.SHA256Of([]byte("tree"))},
		Module:  module, Package: packagePath, File: file, SHA256: core.SHA256Of([]byte("source")), Bytes: count,
	}
}

func reviewIdentifier(t testing.TB, value string) projectstandards.Identifier {
	t.Helper()
	got, err := projectstandards.NewIdentifier(value)
	if err != nil {
		t.Fatalf("NewIdentifier(%q) error = %v, want nil", value, err)
	}
	return got
}
func reviewPacket(t testing.TB) Packet {
	t.Helper()
	review, _ := NewReviewIdentity(reviewUUID(t, 0))
	contractID, _ := NewContractIdentity(reviewUUID(t, 1))
	subject := reviewSubject(t)
	title, _ := NewContractTitle("Exact source acceptance")
	problem, _ := NewProblemStatement("A decision could otherwise migrate to different bytes.")
	completion, _ := NewCompletionStatement("The decision binds the exact commit, path, digest, and byte count.")
	contract := Contract{Identity: contractID, Title: title, Problem: problem, Completion: completion, RequiredChecks: []CheckRequirement{{Kind: CheckManualInspection, Scope: subject.File}}, RequiredProof: []ProofRequirement{{Kind: ProofMachineObservation, Rule: reviewIdentifier(t, "test.evidence"), Subject: subject}}, RelatedCode: []projectstandards.CodeReference{{Path: subject.File}}}
	return Packet{Identity: review, Subject: subject, Contract: contract, IssuedBy: projectstandards.EvidenceAuthority{Offering: core.Offering{Token: "forge"}}, IssuedAt: reviewInstant(t, 0)}
}

func reviewEvidence(t testing.TB) EvidenceReference {
	t.Helper()
	observation, _ := projectstandards.NewObservationID(reviewUUID(t, 6))
	run, _ := projectstandards.NewRunID(reviewUUID(t, 7))
	digest := core.SHA256Of([]byte("observation"))
	return EvidenceReference{Requirement: reviewIdentifier(t, "test.evidence"), Observation: observation, Receipt: runnercontrol.ObservationDeliveryReceipt{SchemaVersion: runnercontrol.SchemaVersion, Identity: runnercontrol.ObservationDeliveryIdentity{Observation: digest, Manifest: core.SHA256Of([]byte("manifest"))}, Run: run, PagesStored: 1, Published: true}}
}

func reviewObservation(t testing.TB, packet Packet) Observation {
	t.Helper()
	identity, _ := NewObservationIdentity(reviewUUID(t, 2))
	reviewerID, _ := NewReviewerIdentity(reviewUUID(t, 3))
	return Observation{Identity: identity, Review: packet.Identity, Subject: packet.Subject, Reviewer: Reviewer{Identity: reviewerID, Kind: ReviewerAgent, Producer: reviewIdentifier(t, "codex")}, Verdict: VerdictPass, Evidence: []EvidenceReference{reviewEvidence(t)}, ObservedAt: reviewInstant(t, 1)}
}

func verifiedHuman(t testing.TB, kind AuthorityKind) VerifiedHumanAuthority {
	t.Helper()
	principal, _ := NewPrincipalIdentity(reviewUUID(t, 4))
	authority, _ := NewAuthorityIdentity(reviewUUID(t, 5))
	claim := HumanAuthorityClaim{Principal: principal, Authority: authority, Kind: kind}
	private, public := reviewKey(t, 1)
	envelope, err := attest.Sign(attest.SignRequest[HumanAuthoritySigningDomain]{Body: claim, Signer: private})
	if err != nil {
		t.Fatalf("attest.Sign() error = %v, want nil", err)
	}
	keys, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{Keys: []core.Ed25519PublicKey{public}})
	if err != nil {
		t.Fatalf("NewTrustedKeys() error = %v, want nil", err)
	}
	proof, err := attest.Verify(attest.VerifyRequest[HumanAuthoritySigningDomain]{Body: claim, Envelope: envelope, TrustedKeys: keys})
	if err != nil {
		t.Fatalf("attest.Verify() error = %v, want nil", err)
	}
	got, err := NewVerifiedHumanAuthority(claim, proof)
	if kind == AuthorityHuman && err != nil {
		t.Fatalf("NewVerifiedHumanAuthority() error = %v, want nil", err)
	}
	if kind != AuthorityHuman && !errors.Is(err, core.ErrReviewControlNonHumanAuthority) {
		t.Fatalf("NewVerifiedHumanAuthority(non-human) error = %v, want %v", err, core.ErrReviewControlNonHumanAuthority)
	}
	return got
}

func reviewDecision(t testing.TB, packet Packet, observation Observation, kind DecisionKind) DecisionIntent {
	t.Helper()
	digest, err := packet.ContractDigest()
	if err != nil {
		t.Fatalf("Packet.ContractDigest() error = %v, want nil", err)
	}
	reason, _ := NewDecisionReason("Reviewed against the exact contract.")
	identity := observation.Identity
	intent := DecisionIntent{Request: reviewNonce(t, 1), Review: packet.Identity, Subject: packet.Subject, Contract: digest, Kind: kind, Reason: reason}
	if kind == DecisionAccept {
		intent.Observation = &identity
	}
	return intent
}

func TestReviewControlDecisionAuthorityAndSourceLayerTriad(t *testing.T) {
	t.Parallel()
	packet := reviewPacket(t)
	observation := reviewObservation(t, packet)
	authority := verifiedHuman(t, AuthorityHuman)
	intent := reviewDecision(t, packet, observation, DecisionAccept)
	t.Run("positive verified human accepts exact source", func(t *testing.T) {
		t.Parallel()
		if gotErr := ValidateDecision(DecisionValidation{Packet: packet, Observation: &observation, Authority: authority, Intent: intent}); gotErr != nil {
			t.Fatalf("ValidateDecision() error = %v, want nil", gotErr)
		}
	})
	t.Run("negative changed digest remains stale source", func(t *testing.T) {
		t.Parallel()
		changed := intent
		changed.Subject.SHA256 = core.SHA256Of([]byte("changed"))
		if gotErr := ValidateDecision(DecisionValidation{Packet: packet, Observation: &observation, Authority: authority, Intent: changed}); !errors.Is(gotErr, core.ErrReviewControlStaleSource) {
			t.Fatalf("ValidateDecision(changed digest) error = %v, want %v", gotErr, core.ErrReviewControlStaleSource)
		}
	})
	t.Run("neutral refusal needs no passing observation", func(t *testing.T) {
		t.Parallel()
		refusal := reviewDecision(t, packet, observation, DecisionRefuse)
		if gotErr := ValidateDecision(DecisionValidation{Packet: packet, Authority: authority, Intent: refusal}); gotErr != nil {
			t.Fatalf("ValidateDecision(refusal) error = %v, want nil", gotErr)
		}
	})
}

func TestReviewControlAuthorityCannotComeFromClaimantJSON(t *testing.T) {
	t.Parallel()
	packet := reviewPacket(t)
	observation := reviewObservation(t, packet)
	intent := reviewDecision(t, packet, observation, DecisionAccept)
	if gotErr := ValidateDecision(DecisionValidation{Packet: packet, Observation: &observation, Intent: intent}); !errors.Is(gotErr, core.ErrReviewControlUnauthorizedAuthority) {
		t.Fatalf("ValidateDecision(wire-only actor) error = %v, want %v", gotErr, core.ErrReviewControlUnauthorizedAuthority)
	}
	verifiedHuman(t, AuthorityAgent)
}

func TestReviewControlPacketCanonicalJSONAndHostileMembers(t *testing.T) {
	t.Parallel()
	packet := reviewPacket(t)
	encoded, err := packet.MarshalJSON()
	if err != nil {
		t.Fatalf("Packet.MarshalJSON() error = %v, want nil", err)
	}
	var got Packet
	if err := got.UnmarshalJSON(encoded); err != nil {
		t.Fatalf("Packet.UnmarshalJSON(canonical) error = %v, want nil", err)
	}
	second, err := got.MarshalJSON()
	if err != nil || string(second) != string(encoded) {
		t.Fatalf("Packet canonical round trip = (%q, %v), want (%q, nil)", second, err, encoded)
	}
	cases := []struct {
		name string
		data []byte
	}{
		{name: "unknown member is refused", data: append(append([]byte(nil), encoded[:len(encoded)-1]...), []byte(`,"unknown":1}`)...)},
		{name: "duplicate member is refused", data: append(append([]byte(nil), encoded[:len(encoded)-1]...), []byte(`,"identity":"`+packet.Identity.String()+`"}`)...)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			preserved := packet
			if err := preserved.UnmarshalJSON(tc.data); !errors.Is(err, core.ErrJSONContract) || preserved.Identity != packet.Identity {
				t.Fatalf("Packet.UnmarshalJSON(hostile) = (%v, %v), want preserved identity and %v", preserved.Identity, err, core.ErrJSONContract)
			}
		})
	}
}

func proofLedgerIdentity(t testing.TB) proofledger.LedgerIdentity {
	t.Helper()
	got, err := proofledger.NewLedgerIdentity(reviewUUID(t, 6))
	if err != nil {
		t.Fatalf("NewLedgerIdentity() error = %v, want nil", err)
	}
	return got
}
func proofEventIdentity(t testing.TB, index int) proofledger.EventIdentity {
	t.Helper()
	got, err := proofledger.NewEventIdentity(reviewUUID(t, index+1))
	if err != nil {
		t.Fatalf("NewEventIdentity() error = %v, want nil", err)
	}
	return got
}

func reviewEvent(t testing.TB, head proofledger.Head, index int, payload EventPayload) proofledger.Envelope[EventPayload] {
	t.Helper()
	_, actor := reviewKey(t, 2)
	got, err := proofledger.NewEnvelope(proofledger.Issue[EventPayload]{Intent: proofledger.AppendIntent[EventPayload]{Request: reviewNonce(t, byte(index+2)), Ledger: head.Ledger, ExpectedHead: head, Actor: actor, Payload: payload}, Event: proofEventIdentity(t, index), RecordedAt: reviewInstant(t, index)})
	if err != nil {
		t.Fatalf("proofledger.NewEnvelope() error = %v, want nil", err)
	}
	return got
}

func TestReviewControlTypedLedgerFoldPreservesSupersession(t *testing.T) {
	t.Parallel()
	packet := reviewPacket(t)
	observation := reviewObservation(t, packet)
	authority := verifiedHuman(t, AuthorityHuman)
	intent := reviewDecision(t, packet, observation, DecisionAccept)
	record, err := NewDecisionRecord(authority, intent)
	if err != nil {
		t.Fatalf("NewDecisionRecord() error = %v, want nil", err)
	}
	genesis, _ := proofledger.NewGenesisHead(proofLedgerIdentity(t))
	issued := reviewEvent(t, genesis, 0, EventPayload{Kind: EventReviewIssued, Review: &packet})
	observed := reviewEvent(t, issued.Head(), 1, EventPayload{Kind: EventObservationRecorded, Observation: &observation})
	accepted := reviewEvent(t, observed.Head(), 2, EventPayload{Kind: EventHumanAccepted, Decision: &record})
	reason, _ := NewDecisionReason("A later exact-source review replaces this decision.")
	supersession := DecisionSuperseded{Previous: accepted.Event, Reason: reason}
	superseded := reviewEvent(t, accepted.Head(), 3, EventPayload{Kind: EventDecisionSuperseded, Supersession: &supersession})
	verifier, _ := proofledger.NewVerifier[EventPayload](genesis)
	var fold Fold
	for _, event := range []proofledger.Envelope[EventPayload]{issued, observed, accepted, superseded} {
		if err := verifier.Observe(event); err != nil {
			t.Fatalf("Verifier.Observe() error = %v, want nil", err)
		}
		if err := fold.Observe(event); err != nil {
			t.Fatalf("Fold.Observe() error = %v, want nil", err)
		}
	}
	if err := verifier.Finish(superseded.Head()); err != nil {
		t.Fatalf("Verifier.Finish() error = %v, want nil", err)
	}
	projection, err := fold.Projection()
	if err != nil || projection.Current || projection.LatestDecision == nil || *projection.LatestDecision != DecisionAccept {
		t.Fatalf("Fold.Projection() = (%+v, %v), want superseded prior acceptance retained and non-current", projection, err)
	}
	_, producer := reviewKey(t, 3)
	receipt, err := proofledger.NewReceipt(superseded, producer)
	if err != nil {
		t.Fatalf("NewReceipt() error = %v, want nil", err)
	}
	if err := proofledger.VerifyReceipt(superseded, receipt); err != nil {
		t.Fatalf("VerifyReceipt() error = %v, want nil", err)
	}
}

func reviewReceipt(t testing.TB, packet Packet) proofledger.ReceiptDocument {
	t.Helper()
	genesis, err := proofledger.NewGenesisHead(proofLedgerIdentity(t))
	if err != nil {
		t.Fatalf("NewGenesisHead() error = %v, want nil", err)
	}
	payload := EventPayload{Kind: EventReviewIssued, Review: &packet}
	event := reviewEvent(t, genesis, 0, payload)
	private, producer := reviewKey(t, 3)
	got, err := proofledger.IssueReceipt(proofledger.ReceiptIssuance[EventPayload]{Event: event, Producer: producer, Signer: private})
	if err != nil {
		t.Fatalf("IssueReceipt() error = %v, want nil", err)
	}
	return got
}

type decisionServiceProbe struct{ called bool }

func (s *decisionServiceProbe) RecordDecision(_ context.Context, authority VerifiedHumanAuthority, request RecordDecisionRequest) (RecordDecisionResponse, error) {
	s.called = true
	if err := errors.Join(authority.Validate(), request.Validate()); err != nil {
		return RecordDecisionResponse{}, err
	}
	return RecordDecisionResponse{}, core.ErrProofLedgerAppendIndeterminate
}

func TestReviewControlDecisionServiceReceivesAuthoritySeparately(t *testing.T) {
	t.Parallel()
	service := &decisionServiceProbe{}
	var recorder HumanDecisionRecorder = service
	packet := reviewPacket(t)
	observation := reviewObservation(t, packet)
	_, err := recorder.RecordDecision(context.Background(), verifiedHuman(t, AuthorityHuman), RecordDecisionRequest{Intent: reviewDecision(t, packet, observation, DecisionAccept)})
	if !errors.Is(err, core.ErrProofLedgerAppendIndeterminate) || !service.called {
		t.Fatalf("HumanDecisionRecorder.RecordDecision() = (called=%t, %v), want (true, %v)", service.called, err, core.ErrProofLedgerAppendIndeterminate)
	}
}

func BenchmarkReviewPacketCanonicalJSON(b *testing.B) {
	packet := reviewPacket(b)
	b.ReportAllocs()
	b.ResetTimer()
	var sink []byte
	var err error
	for range b.N {
		sink, err = packet.MarshalJSON()
		if err != nil {
			b.Fatalf("Packet.MarshalJSON() error = %v, want nil", err)
		}
	}
	if len(sink) == 0 {
		b.Fatalf("Packet.MarshalJSON() byte count = %d, want a nonzero observed result", len(sink))
	}
}
