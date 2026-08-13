package receipt

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type receiptJSONDoor uint8

const (
	receiptJSONDoorUnknown receiptJSONDoor = iota
	receiptJSONDoorEvidenceBody
	receiptJSONDoorHeader
	receiptJSONDoorEvidencePayload
	receiptJSONDoorEvidenceDocument
	receiptJSONDoorAccountIdentity
	receiptJSONDoorOfferingIdentity
	receiptJSONDoorSubmissionIdentity
	receiptJSONDoorObjectIdentity
	receiptJSONDoorRevision
	receiptJSONDoorReceiptID
	receiptJSONDoorGeneration
	receiptJSONDoorCursorDigest
	receiptJSONDoorChainHash
	receiptJSONDoorScope
	receiptJSONDoorWatermark
	receiptJSONDoorLimit
)

func (d receiptJSONDoor) receiverName() string {
	switch d {
	case receiptJSONDoorEvidenceBody:
		return "EvidenceBody"
	case receiptJSONDoorHeader:
		return "Header"
	case receiptJSONDoorEvidencePayload:
		return "EvidencePayload"
	case receiptJSONDoorEvidenceDocument:
		return "EvidenceDocument"
	case receiptJSONDoorAccountIdentity:
		return "AccountIdentity"
	case receiptJSONDoorOfferingIdentity:
		return "OfferingIdentity"
	case receiptJSONDoorSubmissionIdentity:
		return "SubmissionIdentity"
	case receiptJSONDoorObjectIdentity:
		return "ObjectIdentity"
	case receiptJSONDoorRevision:
		return "Revision"
	case receiptJSONDoorReceiptID:
		return "ReceiptID"
	case receiptJSONDoorGeneration:
		return "Generation"
	case receiptJSONDoorCursorDigest:
		return "CursorDigest"
	case receiptJSONDoorChainHash:
		return "ChainHash"
	case receiptJSONDoorScope:
		return "Scope"
	case receiptJSONDoorWatermark:
		return "Watermark"
	case receiptJSONDoorUnknown, receiptJSONDoorLimit:
		return ""
	default:
		return ""
	}
}

type receiptJSONDoorFixtures struct {
	body       EvidenceBody
	header     Header
	payload    EvidencePayload
	document   EvidenceDocument
	account    AccountIdentity
	offering   OfferingIdentity
	submission SubmissionIdentity
	object     ObjectIdentity
	revision   Revision
	receipt    ReceiptID
	generation Generation
	cursor     CursorDigest
	chain      ChainHash
	scope      Scope
	watermark  Watermark
	fixture    receiptFixture
}

type receiptJSONDoorSeed struct {
	document []byte
	door     receiptJSONDoor
}

func FuzzReceiptExternalJSONDoorInventory(f *testing.F) {
	fixtures := receiptJSONFixturesForFuzz(f)
	for _, seed := range receiptJSONSeedsForFuzz(f, fixtures) {
		f.Add(uint8(seed.door), seed.document)
	}
	for raw := 0; raw <= 255; raw++ {
		offering := core.Offering(raw)
		if !offering.IsValid() {
			continue
		}
		identity, err := OfferingIdentityFor(offering)
		if err != nil {
			f.Fatalf("OfferingIdentityFor(%v) error = %v, want nil", offering, err)
		}
		canonical, err := identity.MarshalJSON()
		if err != nil {
			f.Fatalf("OfferingIdentity.MarshalJSON(%v) error = %v, want nil", offering, err)
		}
		f.Add(uint8(receiptJSONDoorOfferingIdentity), canonical)
	}
	for _, hostile := range [][]byte{
		nil, {}, []byte(`null`), []byte(`{}`), []byte(`[]`), []byte(`""`),
		[]byte(`0`), []byte(`true`), []byte(`"00000000000000000000000000000000"`),
		[]byte(`"ffffffffffffffffffffffffffffffff"`), []byte(`"FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF"`),
		[]byte{'"', 0xff, '"'}, bytes.Repeat([]byte{' '}, core.JSONDocumentMaximumBytes+1),
	} {
		f.Add(uint8(receiptJSONDoorOfferingIdentity), hostile)
	}
	for _, hostile := range [][]byte{
		nil, {}, []byte(`null`), []byte(`{}`), []byte(`[]`),
		[]byte(`""`), []byte(`0`), []byte(`true`), []byte(`{`),
		bytes.Repeat([]byte(`[`), core.JSONNestingDepthMaximum+1),
	} {
		f.Add(uint8(receiptJSONDoorEvidenceDocument), hostile)
	}

	f.Fuzz(func(t *testing.T, rawDoor uint8, data []byte) {
		switch receiptJSONDoor(rawDoor) {
		case receiptJSONDoorEvidenceBody:
			fuzzReceiptJSONValue(t, data, fixtures.body)
		case receiptJSONDoorHeader:
			fuzzReceiptJSONValue(t, data, fixtures.header)
		case receiptJSONDoorEvidencePayload:
			fuzzReceiptJSONValue(t, data, fixtures.payload)
		case receiptJSONDoorEvidenceDocument:
			fuzzReceiptEvidenceDocument(t, data, fixtures)
		case receiptJSONDoorAccountIdentity:
			fuzzReceiptJSONValue(t, data, fixtures.account)
		case receiptJSONDoorOfferingIdentity:
			fuzzReceiptOfferingIdentity(t, data, fixtures.offering)
		case receiptJSONDoorSubmissionIdentity:
			fuzzReceiptJSONValue(t, data, fixtures.submission)
		case receiptJSONDoorObjectIdentity:
			fuzzReceiptJSONValue(t, data, fixtures.object)
		case receiptJSONDoorRevision:
			fuzzReceiptJSONValue(t, data, fixtures.revision)
		case receiptJSONDoorReceiptID:
			fuzzReceiptJSONValue(t, data, fixtures.receipt)
		case receiptJSONDoorGeneration:
			fuzzReceiptJSONValue(t, data, fixtures.generation)
		case receiptJSONDoorCursorDigest:
			fuzzReceiptJSONValue(t, data, fixtures.cursor)
		case receiptJSONDoorChainHash:
			fuzzReceiptJSONValue(t, data, fixtures.chain)
		case receiptJSONDoorScope:
			fuzzReceiptJSONValue(t, data, fixtures.scope)
		case receiptJSONDoorWatermark:
			fuzzReceiptJSONValue(t, data, fixtures.watermark)
		case receiptJSONDoorUnknown, receiptJSONDoorLimit:
			return
		default:
			return
		}
	})
}

func fuzzReceiptOfferingIdentity(t *testing.T, data []byte, seed OfferingIdentity) {
	t.Helper()

	candidate := seed
	decodeErr := candidate.UnmarshalJSON(data)
	if decodeErr != nil {
		if !errors.Is(decodeErr, core.ErrReceiptContract) ||
			!errors.Is(decodeErr, core.ErrJSONContract) ||
			!errors.Is(decodeErr, core.ErrLifecycleIdentityContract) {
			t.Fatalf("OfferingIdentity.UnmarshalJSON() error = %v, want Receipt+JSON+lifecycle identities", decodeErr)
		}
		if candidate != seed {
			t.Fatalf("OfferingIdentity.UnmarshalJSON(rejected) receiver = %v, want preserved %v", candidate, seed)
		}
		return
	}
	if err := candidate.Validate(); err != nil {
		t.Fatalf("OfferingIdentity.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
	}
	inputToken, err := core.DecodeJSONStringToken(data)
	if err != nil || inputToken != candidate.String() {
		t.Fatalf("OfferingIdentity accepted projection = (%q, %v), want exact input token %q and nil", candidate.String(), err, inputToken)
	}
	matched := false
	for raw := 0; raw <= 255; raw++ {
		offering := core.Offering(raw)
		if !offering.IsValid() {
			continue
		}
		admitted, deriveErr := OfferingIdentityFor(offering)
		if deriveErr != nil {
			t.Fatalf("OfferingIdentityFor(%v) error = %v, want nil", offering, deriveErr)
		}
		matched = matched || admitted == candidate
	}
	if !matched {
		t.Fatalf("OfferingIdentity.UnmarshalJSON() admitted %v outside compiler-owned core.Offering projections", candidate)
	}
	canonical, err := candidate.MarshalJSON()
	if err != nil || len(canonical) > core.JSONDocumentMaximumBytes {
		t.Fatalf("OfferingIdentity.MarshalJSON(accepted) = (%d bytes, %v), want bounded and nil", len(canonical), err)
	}
	var roundTrip OfferingIdentity
	if err := roundTrip.UnmarshalJSON(canonical); err != nil || roundTrip != candidate {
		t.Fatalf("OfferingIdentity canonical round trip = (%v, %v), want (%v, nil)", roundTrip, err, candidate)
	}
	second, err := roundTrip.MarshalJSON()
	if err != nil || !bytes.Equal(second, canonical) {
		t.Fatalf("OfferingIdentity second canonical projection = (%q, %v), want (%q, nil)", second, err, canonical)
	}
}

type receiptTextDoor uint8

const (
	receiptTextDoorUnknown receiptTextDoor = iota
	receiptTextDoorAccountIdentity
	receiptTextDoorSubmissionIdentity
	receiptTextDoorObjectIdentity
	receiptTextDoorReceiptID
	receiptTextDoorLimit
)

func (d receiptTextDoor) functionName() string {
	switch d {
	case receiptTextDoorAccountIdentity:
		return "ParseAccountIdentity"
	case receiptTextDoorSubmissionIdentity:
		return "ParseSubmissionIdentity"
	case receiptTextDoorObjectIdentity:
		return "ParseObjectIdentity"
	case receiptTextDoorReceiptID:
		return "ParseReceiptID"
	case receiptTextDoorUnknown, receiptTextDoorLimit:
		return ""
	default:
		return ""
	}
}

func FuzzReceiptExternalTextDoorInventory(f *testing.F) {
	fixture := newReceiptFixture(f, 151)
	f.Add(uint8(receiptTextDoorAccountIdentity), fixture.account.String())
	f.Add(uint8(receiptTextDoorSubmissionIdentity), fixture.submission.String())
	f.Add(uint8(receiptTextDoorObjectIdentity), fixture.object.String())
	f.Add(uint8(receiptTextDoorReceiptID), fixture.receipt.String())
	for _, hostile := range []string{"", " ", "0", "A", "\x00", "\xff"} {
		f.Add(uint8(receiptTextDoorReceiptID), hostile)
	}

	f.Fuzz(func(t *testing.T, rawDoor uint8, value string) {
		switch receiptTextDoor(rawDoor) {
		case receiptTextDoorAccountIdentity:
			got, err := ParseAccountIdentity(value)
			fuzzReceiptTextOutcome(t, receiptTextOutcome{
				input: value, projection: got.String(), err: err,
				validate: got.Validate,
			})
		case receiptTextDoorSubmissionIdentity:
			got, err := ParseSubmissionIdentity(value)
			fuzzReceiptTextOutcome(t, receiptTextOutcome{
				input: value, projection: got.String(), err: err,
				validate: got.Validate,
			})
		case receiptTextDoorObjectIdentity:
			got, err := ParseObjectIdentity(value)
			fuzzReceiptTextOutcome(t, receiptTextOutcome{
				input: value, projection: got.String(), err: err,
				validate: got.Validate,
			})
		case receiptTextDoorReceiptID:
			got, err := ParseReceiptID(value)
			fuzzReceiptTextOutcome(t, receiptTextOutcome{
				input: value, projection: got.String(), err: err,
				validate: got.Validate,
			})
		case receiptTextDoorUnknown, receiptTextDoorLimit:
			return
		default:
			return
		}
	})
}

func fuzzReceiptJSONValue[T core.ValidatedJSONMarshaler](
	t *testing.T,
	data []byte,
	seed T,
) {
	t.Helper()

	before, err := seed.MarshalJSON()
	if err != nil {
		t.Fatalf("receipt seed MarshalJSON() error = %v, want nil", err)
	}
	candidate := seed
	decoder, ok := any(&candidate).(json.Unmarshaler)
	if !ok {
		t.Fatalf("receipt JSON receiver %T lacks json.Unmarshaler", &candidate)
	}
	decodeErr := decoder.UnmarshalJSON(data)
	if decodeErr != nil {
		if !errors.Is(decodeErr, core.ErrReceiptContract) ||
			!errors.Is(decodeErr, core.ErrJSONContract) {
			t.Fatalf("receipt JSON door error = %v, want %v and %v",
				decodeErr, core.ErrReceiptContract, core.ErrJSONContract)
		}
		after, marshalErr := candidate.MarshalJSON()
		if marshalErr != nil || !bytes.Equal(after, before) {
			t.Fatalf("rejected receipt JSON door changed its receiver: marshal error %v",
				marshalErr)
		}
		return
	}
	if err := candidate.Validate(); err != nil {
		t.Fatalf("accepted receipt JSON validation error = %v, want nil", err)
	}
	canonical, err := candidate.MarshalJSON()
	if err != nil || len(canonical) > core.JSONDocumentMaximumBytes {
		t.Fatalf("receipt canonical JSON = (%d bytes, %v), want bounded and nil",
			len(canonical), err)
	}
	var roundTrip T
	roundTripDecoder, ok := any(&roundTrip).(json.Unmarshaler)
	if !ok {
		t.Fatalf("receipt round-trip receiver %T lacks json.Unmarshaler", &roundTrip)
	}
	if err := roundTripDecoder.UnmarshalJSON(canonical); err != nil {
		t.Fatalf("receipt canonical JSON decode error = %v, want nil", err)
	}
	second, err := roundTrip.MarshalJSON()
	if err != nil || !bytes.Equal(second, canonical) {
		t.Fatalf("receipt JSON door lacks a canonical fixed point: marshal error %v", err)
	}
}

func fuzzReceiptEvidenceDocument(
	t *testing.T,
	data []byte,
	fixtures receiptJSONDoorFixtures,
) {
	t.Helper()
	fuzzReceiptJSONValue(t, data, fixtures.document)

	candidate := fixtures.document
	if err := candidate.UnmarshalJSON(data); err != nil {
		return
	}
	proof, err := VerifyEvidence(VerifyEvidenceRequest{
		Document: candidate, TrustedKeys: fixtures.fixture.trusted,
		Expected: EvidenceExpectation{
			Account:  candidate.Payload.Header.Account,
			Offering: candidate.Payload.Header.Offering,
			Body:     candidate.Payload.Body,
		},
	})
	if err != nil {
		if !errors.Is(err, core.ErrReceiptVerification) || proof != (VerifiedEvidence{}) {
			t.Fatalf("VerifyEvidence(fuzz document) = (%v, %v), want typed refusal and zero proof",
				proof, err)
		}
		return
	}
	if proof.Validate() != nil || candidate != fixtures.document {
		t.Fatalf("VerifyEvidence(fuzz document) authenticated facts outside the signed seed")
	}
}

func receiptJSONFixturesForFuzz(t testing.TB) receiptJSONDoorFixtures {
	t.Helper()

	fixture := newReceiptFixture(t, 152)
	document := issueFixture(t, fixture)
	scope := Scope{Account: fixture.account, Offering: fixture.offering}
	watermark := watermarkFixture(t, scope, 1, "fuzz")
	return receiptJSONDoorFixtures{
		body: document.Payload.Body, header: document.Payload.Header,
		payload: document.Payload, document: document,
		account: fixture.account, offering: fixture.offering,
		submission: fixture.submission, object: fixture.object,
		revision: RevisionV1, receipt: fixture.receipt,
		generation: watermark.Generation, cursor: watermark.CursorDigest,
		chain: watermark.ChainHash, scope: scope, watermark: watermark,
		fixture: fixture,
	}
}

func receiptJSONSeedsForFuzz(
	t testing.TB,
	fixtures receiptJSONDoorFixtures,
) []receiptJSONDoorSeed {
	t.Helper()

	return []receiptJSONDoorSeed{
		receiptJSONSeedForFuzz(t, receiptJSONDoorEvidenceBody, fixtures.body),
		receiptJSONSeedForFuzz(t, receiptJSONDoorHeader, fixtures.header),
		receiptJSONSeedForFuzz(t, receiptJSONDoorEvidencePayload, fixtures.payload),
		receiptJSONSeedForFuzz(t, receiptJSONDoorEvidenceDocument, fixtures.document),
		receiptJSONSeedForFuzz(t, receiptJSONDoorAccountIdentity, fixtures.account),
		receiptJSONSeedForFuzz(t, receiptJSONDoorOfferingIdentity, fixtures.offering),
		receiptJSONSeedForFuzz(t, receiptJSONDoorSubmissionIdentity, fixtures.submission),
		receiptJSONSeedForFuzz(t, receiptJSONDoorObjectIdentity, fixtures.object),
		receiptJSONSeedForFuzz(t, receiptJSONDoorRevision, fixtures.revision),
		receiptJSONSeedForFuzz(t, receiptJSONDoorReceiptID, fixtures.receipt),
		receiptJSONSeedForFuzz(t, receiptJSONDoorGeneration, fixtures.generation),
		receiptJSONSeedForFuzz(t, receiptJSONDoorCursorDigest, fixtures.cursor),
		receiptJSONSeedForFuzz(t, receiptJSONDoorChainHash, fixtures.chain),
		receiptJSONSeedForFuzz(t, receiptJSONDoorScope, fixtures.scope),
		receiptJSONSeedForFuzz(t, receiptJSONDoorWatermark, fixtures.watermark),
	}
}

func receiptJSONSeedForFuzz(
	t testing.TB,
	door receiptJSONDoor,
	value core.ValidatedJSONMarshaler,
) receiptJSONDoorSeed {
	t.Helper()

	document, err := value.MarshalJSON()
	if err != nil {
		t.Fatalf("receipt fuzz seed MarshalJSON(%d) error = %v, want nil", door, err)
	}
	return receiptJSONDoorSeed{door: door, document: document}
}

type receiptTextOutcome struct {
	input      string
	projection string
	err        error
	validate   func() error
}

func fuzzReceiptTextOutcome(t *testing.T, outcome receiptTextOutcome) {
	t.Helper()

	if outcome.err != nil {
		if !errors.Is(outcome.err, core.ErrReceiptContract) || outcome.projection != "" {
			t.Fatalf("receipt text refusal = (%q, %v), want empty and %v",
				outcome.projection, outcome.err, core.ErrReceiptContract)
		}
		return
	}
	if outcome.validate() != nil || outcome.projection != outcome.input {
		t.Fatalf("receipt text acceptance = (%q, %v), want exact %q and nil",
			outcome.projection, outcome.validate(), outcome.input)
	}
}
