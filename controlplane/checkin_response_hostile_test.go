package controlplane_test

import (
	"crypto/ed25519"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
	"github.com/deliri/primitive/v2026/receipt"
)

const (
	// checkInResponseFutureInstant is later than the golden's provider time, so
	// a client holding it has already seen a newer answer from this authority.
	checkInResponseFutureInstant = int64(1_000)
	// checkInResponseOtherGeneration is any generation the golden does not
	// carry.
	checkInResponseOtherGeneration = uint64(2)
	// checkInResponseOtherAccountHex is a well-formed account that is not the
	// golden's.
	checkInResponseOtherAccountHex = "3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c"
)

// issuedCheckInResponse is one genuinely signed authority answer and everything
// a client needs to authenticate it.
type issuedCheckInResponse struct {
	signer      ed25519.PrivateKey
	document    controlplane.CheckInResponseDocument
	trusted     attest.TrustedKeys
	expectation controlplane.ResponseExpectation
}

func (i issuedCheckInResponse) verification() controlplane.CheckInResponseVerification {
	return controlplane.CheckInResponseVerification{
		Document: i.document, Expected: i.expectation, TrustedKeys: i.trusted,
	}
}

// issueTestCheckInResponse builds one genuinely signed check-in answer.
//
// The payload comes from the authority's own fixture rather than from
// hand-assembled parts, so the facts under test are the facts a real authority
// emits. Only the signatures are replaced: the nested lease and the response
// are re-signed with a real Ed25519 key this test holds, because verification
// is the thing being proved and the authority's private key is not in the
// repository.
func issueTestCheckInResponse(t testing.TB) issuedCheckInResponse {
	t.Helper()

	signerPublic, signer := testSigningKey(t, checkInAuthoritySeed)

	var golden controlplane.CheckInResponseDocument
	if err := golden.UnmarshalJSON(readGolden(t, "check_in_response.json")); err != nil {
		t.Fatalf("decoding the golden response error = %v, want nil", err)
	}
	payload := golden.Payload
	payload.Lease = resignLease(t, payload.Lease, signer)

	document, err := controlplane.IssueCheckInResponse(payload, signer)
	if err != nil {
		t.Fatalf("IssueCheckInResponse() error = %v, want nil", err)
	}
	trusted, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{
		Keys: []core.Ed25519PublicKey{signerPublic},
	})
	if err != nil {
		t.Fatalf("NewTrustedKeys() error = %v, want nil", err)
	}
	return issuedCheckInResponse{
		signer: signer, document: document, trusted: trusted,
		expectation: expectationFor(payload.Header),
	}
}

// TestVerifyCheckInResponseAcceptsOnlyAnAnswerToThisExactCheckIn is the
// authority half of the exchange, and the half a customer's machine obeys.
//
// A check-in answer decides whether new work may begin, so the failure that
// matters is not a malformed document. It is an authentic, correctly signed
// answer that belongs to a different request, a different machine, or a
// different binary, and is therefore about somebody else's entitlement.
func TestVerifyCheckInResponseAcceptsOnlyAnAnswerToThisExactCheckIn(t *testing.T) {
	t.Parallel()

	issued := issueTestCheckInResponse(t)

	t.Run("the genuine answer verifies and carries its decision", func(t *testing.T) {
		t.Parallel()

		verified, err := controlplane.VerifyCheckInResponse(issued.verification())
		if err != nil {
			t.Fatalf("VerifyCheckInResponse() error = %v, want nil", err)
		}
		payload, err := verified.Payload()
		if err != nil {
			t.Fatalf("Payload() error = %v, want nil", err)
		}
		if got := payload.Disposition; got != controlplane.UsageDispositionAccepted {
			t.Errorf("verified disposition = %v, want %v", got, controlplane.UsageDispositionAccepted)
		}
		if got := payload.Disposition.AdvancesWatermark(); !got {
			t.Errorf("accepted disposition AdvancesWatermark() = %t, want true", got)
		}
		if got := payload.Disposition.IsValid(); !got {
			t.Errorf("verified disposition %v IsValid() = %t, want true", payload.Disposition, got)
		}
		if _, err := verified.Lease(); err != nil {
			t.Errorf("Lease() error = %v, want nil", err)
		}
	})

	t.Run("an answer to another check-in is refused by nonce", func(t *testing.T) {
		t.Parallel()

		request := issued.verification()
		request.Expected.RequestNonce = otherRequestNonce(t)

		requireResponseBindingRefusal(t, request, controlplane.ResponseHeaderFieldRequestNonce)
	})

	t.Run("an answer for another installation is refused", func(t *testing.T) {
		t.Parallel()

		request := issued.verification()
		_, otherDevice := testDeviceKey(t, checkInOtherDeviceSeed)
		request.Expected.Installation = otherDevice

		requireResponseBindingRefusal(t, request, controlplane.ResponseHeaderFieldInstallation)
	})

	t.Run("an answer for another offering is refused", func(t *testing.T) {
		t.Parallel()

		request := issued.verification()
		request.Expected.Offering = core.OfferingBug

		requireResponseBindingRefusal(t, request, controlplane.ResponseHeaderFieldOffering)
	})

	t.Run("an answer for another account is refused", func(t *testing.T) {
		t.Parallel()

		request := issued.verification()
		other, err := receipt.ParseAccountIdentity(checkInResponseOtherAccountHex)
		if err != nil {
			t.Fatalf("ParseAccountIdentity() error = %v, want nil", err)
		}
		request.Expected.Account = other

		requireResponseBindingRefusal(t, request, controlplane.ResponseHeaderFieldAccount)
	})

	// Revision is the fifth bound fact and has no case here, deliberately.
	// Controlwire publishes exactly one revision, so both the header's value and
	// the client's expectation are always that one value and the comparison
	// cannot disagree. The arm stays in production because a second revision is
	// what it exists for, and removing it would mean writing it again then.
	// TestResponseHeaderFieldNamesExactlyTheBoundFacts still proves the field
	// itself is named, parsed, and round trips.

	t.Run("an answer whose provider time moves backwards is refused", func(t *testing.T) {
		t.Parallel()

		// The client already saw a later instant from this authority. An answer
		// stamped before it is a replay of an older decision, which is how a
		// revoked installation would be handed back its last good answer.
		request := issued.verification()
		request.Expected.PriorProviderTime = testInstant(t, checkInResponseFutureInstant)

		if _, err := controlplane.VerifyCheckInResponse(request); err == nil {
			t.Fatalf("VerifyCheckInResponse() error = nil, want a refusal of a backwards instant")
		}
	})

	t.Run("a stripped attestation authenticates nothing", func(t *testing.T) {
		t.Parallel()

		request := issued.verification()
		request.Document.Attestation = attest.Envelope[controlplane.SigningDomain]{}

		requireCheckInResponseRefusal(t, request)
	})

	t.Run("a trust set holding only some other key is refused", func(t *testing.T) {
		t.Parallel()

		stranger, _ := testSigningKey(t, checkInOtherDeviceSeed)
		keys, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{
			Keys: []core.Ed25519PublicKey{stranger},
		})
		if err != nil {
			t.Fatalf("NewTrustedKeys() error = %v, want nil", err)
		}
		request := issued.verification()
		request.TrustedKeys = keys

		requireCheckInResponseRefusal(t, request)
	})

	t.Run("a lease signed by another key is refused inside a genuine answer", func(t *testing.T) {
		t.Parallel()

		// The response itself is authentic and binds to the request. What is
		// forged is the visa inside it, which is the document that actually
		// decides whether work may continue.
		_, impostor := testSigningKey(t, checkInOtherDeviceSeed)
		payload := issued.document.Payload
		payload.Lease = resignLease(t, payload.Lease, impostor)
		document, err := controlplane.IssueCheckInResponse(payload, issued.signer)
		if err != nil {
			t.Fatalf("IssueCheckInResponse() error = %v, want nil", err)
		}
		request := issued.verification()
		request.Document = document

		requireCheckInResponseRefusal(t, request)
	})
}

// TestVerifiedCheckInResponseCannotBeManufactured proves the sealed proof is
// sealed. A caller that declared the type instead of verifying must not be able
// to present it as an authority's decision.
func TestVerifiedCheckInResponseCannotBeManufactured(t *testing.T) {
	t.Parallel()

	var forged controlplane.VerifiedCheckInResponse
	if err := forged.Validate(); err == nil {
		t.Fatalf("zero VerifiedCheckInResponse Validate() error = nil, want a refusal")
	}
	if payload, err := forged.Payload(); err == nil {
		t.Errorf("zero VerifiedCheckInResponse Payload() = %v, error = nil, want a refusal", payload)
	}
	if granted, err := forged.Lease(); err == nil {
		t.Errorf("zero VerifiedCheckInResponse Lease() = %v, error = nil, want a refusal", granted)
	}
}

// TestIssueCheckInResponseRefusesEveryPayloadValidateRefuses proves the issuing
// door and the gate agree.
//
// An authority that signed a payload its own validator refuses would put a
// document on the wire that every client must then reject, and the authority's
// signature would be on it.
func TestIssueCheckInResponseRefusesEveryPayloadValidateRefuses(t *testing.T) {
	t.Parallel()

	issued := issueTestCheckInResponse(t)
	_, signer := testSigningKey(t, checkInAuthoritySeed)

	cases := []struct {
		mutate func(*controlplane.CheckInResponsePayload)
		name   string
	}{
		{
			name: "the zero payload carries no decision at all",
			mutate: func(payload *controlplane.CheckInResponsePayload) {
				*payload = controlplane.CheckInResponsePayload{}
			},
		},
		{
			name: "an unset disposition says nothing about the window",
			mutate: func(payload *controlplane.CheckInResponsePayload) {
				payload.Disposition = controlplane.UsageDispositionUnknown
			},
		},
		{
			name: "a watermark for another subject than the lease it arrived with",
			mutate: func(payload *controlplane.CheckInResponsePayload) {
				_, other := testDeviceKey(t, checkInOtherDeviceSeed)
				payload.Watermark.Subject.DeviceID = other
			},
		},
		{
			name: "a generation that disagrees with the decision it reports",
			mutate: func(payload *controlplane.CheckInResponsePayload) {
				generation, err := lease.NewGeneration(checkInResponseOtherGeneration)
				if err != nil {
					t.Fatalf("NewGeneration() error = %v, want nil", err)
				}
				payload.Watermark.Generation = generation
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			payload := issued.document.Payload
			testCase.mutate(&payload)
			if err := payload.Validate(); err == nil {
				t.Fatalf("the fixture must be invalid: Validate() error = nil")
			}
			document, err := controlplane.IssueCheckInResponse(payload, signer)
			if err == nil {
				t.Fatalf("IssueCheckInResponse() = %v, want a refusal", document)
			}
			if !errors.Is(err, core.ErrControlPlaneContract) {
				t.Fatalf("IssueCheckInResponse() error = %v, want the control-plane contract identity", err)
			}
		})
	}
}

// TestResponseHeaderFieldNamesExactlyTheBoundFacts exhausts the closed domain a
// binding failure may name.
//
// The field travels in a rejection a caller switches on, so an unnamed value
// reaching a caller would be a refusal it cannot act on, and two fields sharing
// a token would let one disagreement be read as another.
func TestResponseHeaderFieldNamesExactlyTheBoundFacts(t *testing.T) {
	t.Parallel()

	fields := []controlplane.ResponseHeaderField{
		controlplane.ResponseHeaderFieldRequestNonce,
		controlplane.ResponseHeaderFieldAccount,
		controlplane.ResponseHeaderFieldInstallation,
		controlplane.ResponseHeaderFieldRevision,
		controlplane.ResponseHeaderFieldOffering,
	}
	seen := make(map[string]controlplane.ResponseHeaderField, len(fields))
	for _, field := range fields {
		if !field.IsValid() {
			t.Fatalf("ResponseHeaderField(%d).IsValid() = false, want a named bound fact", field)
		}
		token := field.String()
		if token == "" {
			t.Fatalf("ResponseHeaderField(%d).String() = %q, want a token", field, token)
		}
		if previous, repeated := seen[token]; repeated {
			t.Fatalf("field %v shares the token %q with %v, want distinct facts", field, token, previous)
		}
		seen[token] = field

		parsed, err := controlplane.ParseResponseHeaderField(token)
		if err != nil {
			t.Fatalf("ParseResponseHeaderField(%q) error = %v, want nil", token, err)
		}
		if parsed != field {
			t.Fatalf("ParseResponseHeaderField(%q) = %v, want %v", token, parsed, field)
		}

		encoded, err := field.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON() for %v error = %v, want nil", field, err)
		}
		var decoded controlplane.ResponseHeaderField
		if err := decoded.UnmarshalJSON(encoded); err != nil {
			t.Fatalf("UnmarshalJSON(%s) error = %v, want nil", encoded, err)
		}
		if decoded != field {
			t.Fatalf("round trip of %v = %v, want %v", field, decoded, field)
		}
	}
}

// TestResponseHeaderFieldRefusesEveryValueOutsideTheDomain walks the whole byte
// space, so a field added without a token cannot reach a caller as a zero and no
// unnamed value can be parsed, marshalled, or decoded into something a caller
// would switch on.
func TestResponseHeaderFieldRefusesEveryValueOutsideTheDomain(t *testing.T) {
	t.Parallel()

	for value := 0; value <= maximumByteOrdinal; value++ {
		field := controlplane.ResponseHeaderField(value)
		if field.IsValid() {
			continue
		}
		if err := field.Validate(); err == nil {
			t.Fatalf("ResponseHeaderField(%d).Validate() error = nil, want a refusal", value)
		}
		if got := field.String(); got != "" {
			t.Fatalf("ResponseHeaderField(%d).String() = %q, want empty text", value, got)
		}
		if encoded, err := field.MarshalJSON(); err == nil {
			t.Fatalf("ResponseHeaderField(%d).MarshalJSON() = %s, want a refusal", value, encoded)
		}
		// A binding failure that named nothing would be a refusal a caller
		// cannot switch on, so the constructor returns the header identity
		// instead of a binding error it cannot describe.
		var named controlplane.ResponseBindingError
		if errors.As(controlplane.NewResponseBindingError(field), &named) {
			t.Fatalf("NewResponseBindingError(%d) produced a binding failure naming no field", value)
		}
	}
	if _, err := controlplane.ParseResponseHeaderField(""); err == nil {
		t.Fatalf("ParseResponseHeaderField(\"\") error = nil, want a refusal")
	}
	if _, err := controlplane.ParseResponseHeaderField("not-a-header-field"); err == nil {
		t.Fatalf("ParseResponseHeaderField() of an unknown token error = nil, want a refusal")
	}
}

func requireResponseBindingRefusal(
	t *testing.T,
	request controlplane.CheckInResponseVerification,
	want controlplane.ResponseHeaderField,
) {
	t.Helper()

	_, err := controlplane.VerifyCheckInResponse(request)
	if !errors.Is(err, core.ErrControlPlaneResponseBinding) {
		t.Fatalf("VerifyCheckInResponse() error = %v, want %v", err, core.ErrControlPlaneResponseBinding)
	}
	var binding controlplane.ResponseBindingError
	if !errors.As(err, &binding) {
		t.Fatalf("VerifyCheckInResponse() error = %v, want a ResponseBindingError", err)
	}
	if got := binding.Field(); got != want {
		t.Fatalf("binding failure named %v, want %v", got, want)
	}
}

func requireCheckInResponseRefusal(t *testing.T, request controlplane.CheckInResponseVerification) {
	t.Helper()

	verified, err := controlplane.VerifyCheckInResponse(request)
	if err == nil {
		t.Fatalf("VerifyCheckInResponse() = %v, want a refusal", verified)
	}
	if !errors.Is(err, core.ErrControlPlaneContract) {
		t.Fatalf("VerifyCheckInResponse() error = %v, want the control-plane contract identity", err)
	}
}

// TestUsageDispositionClosesItsEntireByteDomain walks every backing value the
// way the product-status walk does, so a disposition added to the enum joins
// the proof without anyone remembering to add a row. The three published
// members must validate, carry unique canonical tokens, round-trip through
// parse and JSON, and answer the watermark question exactly; all two hundred
// fifty three others must refuse everything, and above all must never claim
// to advance a watermark nobody issued.
func TestUsageDispositionClosesItsEntireByteDomain(t *testing.T) {
	t.Parallel()

	wantAdvances := map[controlplane.UsageDisposition]bool{
		controlplane.UsageDispositionAccepted: true,
		controlplane.UsageDispositionReplay:   false,
		controlplane.UsageDispositionConflict: false,
	}
	seenTokens := map[string]controlplane.UsageDisposition{}
	for value := 0; value <= 255; value++ {
		disposition := controlplane.UsageDisposition(value)
		wantAdvance, member := wantAdvances[disposition]
		if !member {
			if err := disposition.Validate(); !errors.Is(err, core.ErrControlPlaneCheckInResponse) {
				t.Fatalf("UsageDisposition(%d).Validate() error = %v, want %v",
					value, err, core.ErrControlPlaneCheckInResponse)
			}
			if disposition.AdvancesWatermark() {
				t.Fatalf("UsageDisposition(%d).AdvancesWatermark() = true, want false for an unpublished result", value)
			}
			if _, err := disposition.MarshalJSON(); err == nil {
				t.Fatalf("UsageDisposition(%d).MarshalJSON() error = nil, want a refusal", value)
			}
			continue
		}
		if err := disposition.Validate(); err != nil {
			t.Fatalf("UsageDisposition(%d).Validate() error = %v, want nil", value, err)
		}
		token := disposition.String()
		if token == "" {
			t.Fatalf("UsageDisposition(%d).String() is empty, want the canonical token", value)
		}
		if prior, duplicate := seenTokens[token]; duplicate {
			t.Fatalf("UsageDisposition(%d) and UsageDisposition(%d) share the token %q", value, prior, token)
		}
		seenTokens[token] = disposition
		if got := disposition.AdvancesWatermark(); got != wantAdvance {
			t.Fatalf("UsageDisposition(%d).AdvancesWatermark() = %t, want %t", value, got, wantAdvance)
		}
		parsed, err := controlplane.ParseUsageDisposition(token)
		if err != nil || parsed != disposition {
			t.Fatalf("ParseUsageDisposition(%q) = (%v, %v), want (%v, nil)", token, parsed, err, disposition)
		}
		encoded, err := disposition.MarshalJSON()
		if err != nil {
			t.Fatalf("UsageDisposition(%d).MarshalJSON() error = %v, want nil", value, err)
		}
		var decoded controlplane.UsageDisposition
		if err := decoded.UnmarshalJSON(encoded); err != nil || decoded != disposition {
			t.Fatalf("UnmarshalJSON(%s) = (%v, %v), want (%v, nil)", encoded, decoded, err, disposition)
		}
	}

	for _, raw := range []string{"", "ACCEPTED", " accepted", "accepted ", "acceptedx", "unknown"} {
		if _, err := controlplane.ParseUsageDisposition(raw); !errors.Is(err, core.ErrControlPlaneCheckInResponse) {
			t.Fatalf("ParseUsageDisposition(%q) error = %v, want %v", raw, err, core.ErrControlPlaneCheckInResponse)
		}
	}
}
