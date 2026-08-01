package exchange_test

import (
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// FuzzAPIEnvelopeDecodeAcceptsOnlyStableSingleArmDocuments drives arbitrary
// bytes through the real strict decode boundary. The oracle is not "did not
// panic": an accepted document must carry exactly one arm, must re-encode to
// bytes that decode back to the same typed value, and must then re-encode to
// identical bytes, so no wire document can decode into a value the producer
// could not have written.
func FuzzAPIEnvelopeDecodeAcceptsOnlyStableSingleArmDocuments(f *testing.F) {
	f.Add(`{"data":{"message":"payload"},"request_id":"trace-41"}`)
	f.Add(`{"error":{"message":"Not found.","code":"not_found"},"request_id":"trace-41"}`)
	f.Add(`{"error":{"message":"m","tip":"t","code":"internal"},"request_id":"missing"}`)
	f.Add(`{"data":{"message":"p"},"error":{"message":"m","code":"internal"},"request_id":"t"}`)
	f.Add(`{"request_id":"trace-41"}`)
	f.Add(`{"data":null,"error":null,"request_id":"trace-41"}`)
	f.Add(`{"data":{"message":""},"request_id":"trace-41"}`)
	f.Add(`{"data":{"message":"p"},"request_id":""}`)
	f.Add(`{"data":{"message":"p"},"request_id":" trace "}`)
	f.Add(`{"error":{"message":"m","code":"teapot"},"request_id":"t"}`)
	f.Add(`{"data":{"message":"p"},"request_id":"a","Request_ID":"b"}`)
	f.Add(`null`)
	f.Add(``)

	f.Fuzz(func(t *testing.T, document string) {
		decoded, err := core.DecodeStrictJSON[exchange.APIEnvelope[transportDocument]](
			[]byte(document), core.DefaultStrictJSONLimits(),
		)
		if err != nil {
			if _, outcomeErr := decoded.Outcome(); outcomeErr == nil {
				t.Fatalf("rejected DecodeStrictJSON(%q) left a readable envelope, want the zero envelope",
					document)
			}
			if got := decoded.RequestID().String(); got != "" {
				t.Fatalf("rejected DecodeStrictJSON(%q) left request identifier %q, want the zero envelope",
					document, got)
			}
			return
		}

		if validateErr := decoded.Validate(); validateErr != nil {
			t.Fatalf("accepted envelope Validate() error = %v, want nil for %q", validateErr, document)
		}
		outcome, outcomeErr := decoded.Outcome()
		if outcomeErr != nil || outcome.Validate() != nil {
			t.Fatalf("accepted envelope Outcome() = (%v, %v), want a closed reading for %q",
				outcome, outcomeErr, document)
		}
		_, payloadErr := decoded.Payload()
		_, failureErr := decoded.Failure()
		if (payloadErr == nil) == (failureErr == nil) {
			t.Fatalf("accepted envelope arms for %q = (payload %v, failure %v), want exactly one",
				document, payloadErr, failureErr)
		}
		if (outcome == exchange.APIOutcomeSuccess) != (payloadErr == nil) {
			t.Fatalf("Outcome() = %v but payload readable = %t for %q",
				outcome, payloadErr == nil, document)
		}

		encoded, encodeErr := exchange.MarshalAPIEnvelope(decoded)
		if encodeErr != nil {
			t.Fatalf("accepted envelope MarshalAPIEnvelope() error = %v, want nil for %q",
				encodeErr, document)
		}
		round, roundErr := core.DecodeStrictJSON[exchange.APIEnvelope[transportDocument]](
			encoded, core.DefaultStrictJSONLimits(),
		)
		if roundErr != nil {
			t.Fatalf("re-decode of %s error = %v, want nil", encoded, roundErr)
		}
		requireAPIArmsMatch(t, round, decoded)
		if round.RequestID() != decoded.RequestID() {
			t.Fatalf("re-decoded RequestID = %q, want %q",
				round.RequestID().String(), decoded.RequestID().String())
		}
		reencoded, reencodeErr := exchange.MarshalAPIEnvelope(round)
		if reencodeErr != nil {
			t.Fatalf("re-encode error = %v, want nil for %s", reencodeErr, encoded)
		}
		if string(reencoded) != string(encoded) {
			t.Fatalf("re-encoded envelope = %s, want %s", reencoded, encoded)
		}
	})
}

// FuzzNewAPIRequestIDAlwaysProducesOneCanonicalIdentifier proves the documented
// repair contract holds for every input: the result is always admissible, is
// always reachable through the strict parser, and repairing an already repaired
// value changes nothing, so a correlation identifier cannot drift by being
// normalized twice.
func FuzzNewAPIRequestIDAlwaysProducesOneCanonicalIdentifier(f *testing.F) {
	f.Add("trace-41")
	f.Add("")
	f.Add("   ")
	f.Add("\x00\x01\x02")
	f.Add(" \ttrace\r\n41 ")
	f.Add("missing")
	f.Add("追跡識別子")
	f.Add("\xff\xfe")
	f.Add("�")

	f.Fuzz(func(t *testing.T, value string) {
		got := exchange.NewAPIRequestID(value)
		if err := got.Validate(); err != nil {
			t.Fatalf("NewAPIRequestID(%q).Validate() error = %v, want nil", value, err)
		}
		parsed, parseErr := exchange.ParseAPIRequestID(got.String())
		if parseErr != nil || parsed != got {
			t.Fatalf("ParseAPIRequestID(NewAPIRequestID(%q)) = (%q, %v), want (%q, nil)",
				value, parsed.String(), parseErr, got.String())
		}
		if again := exchange.NewAPIRequestID(got.String()); again != got {
			t.Fatalf("NewAPIRequestID is not idempotent for %q: %q then %q",
				value, got.String(), again.String())
		}
		encoded, encodeErr := got.MarshalJSON()
		if encodeErr != nil {
			t.Fatalf("NewAPIRequestID(%q).MarshalJSON() error = %v, want nil", value, encodeErr)
		}
		var decoded exchange.APIRequestID
		if decodeErr := decoded.UnmarshalJSON(encoded); decodeErr != nil {
			t.Fatalf("UnmarshalJSON(%s) error = %v, want nil", encoded, decodeErr)
		}
		if decoded != got {
			t.Fatalf("decoded identifier = %q, want %q", decoded.String(), got.String())
		}
	})
}
