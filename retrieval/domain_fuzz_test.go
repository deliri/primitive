package retrieval_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/retrieval"
)

func TestSigningDomainExhaustsBackingDomainAndCanonicalProjection(t *testing.T) {
	t.Parallel()

	for raw := 0; raw <= 255; raw++ {
		domain := retrieval.SigningDomain(raw)
		wantValid := domain == retrieval.SigningDomainRequestV1 || domain == retrieval.SigningDomainGrantV1
		gotErr := domain.Validate()
		if domain.IsValid() != wantValid || (gotErr == nil) != wantValid || (domain.String() != "") != wantValid {
			t.Fatalf(
				"SigningDomain(%d) validity/string = (%t, %v, %q), want (%t, matching error state, matching token state)",
				raw, domain.IsValid(), gotErr, domain.String(), wantValid,
			)
		}
		if !wantValid {
			if !errors.Is(gotErr, core.ErrRetrievalContract) {
				t.Fatalf("SigningDomain(%d).Validate() error = %v, want %v", raw, gotErr, core.ErrRetrievalContract)
			}
			encoded, marshalErr := domain.MarshalJSON()
			if !errors.Is(marshalErr, core.ErrJSONContract) || encoded != nil {
				t.Fatalf("SigningDomain(%d).MarshalJSON() = (%q, %v), want (nil, %v)", raw, encoded, marshalErr, core.ErrJSONContract)
			}
			continue
		}
		encoded, marshalErr := domain.MarshalJSON()
		var roundTrip retrieval.SigningDomain
		unmarshalErr := roundTrip.UnmarshalJSON(encoded)
		second, secondErr := roundTrip.MarshalJSON()
		if marshalErr != nil || unmarshalErr != nil || secondErr != nil || roundTrip != domain || !bytes.Equal(second, encoded) {
			t.Fatalf(
				"SigningDomain(%d) canonical round trip = (%v, %q, %v, %v, %v), want (%v, %q, nil, nil, nil)",
				raw, roundTrip, second, marshalErr, unmarshalErr, secondErr, domain, encoded,
			)
		}
	}
}

func FuzzSigningDomainJSONSemanticClosure(f *testing.F) {
	for _, domain := range []retrieval.SigningDomain{
		retrieval.SigningDomainRequestV1,
		retrieval.SigningDomainGrantV1,
	} {
		encoded, err := json.Marshal(domain)
		if err != nil {
			f.Fatalf("json.Marshal(SigningDomain seed) error = %v, want nil", err)
		}
		f.Add(encoded)
	}
	for _, seed := range [][]byte{
		nil,
		[]byte(`null`),
		[]byte(`""`),
		[]byte(`"future"`),
		[]byte(`1`),
		[]byte(`[]`),
		[]byte(`"primitive-retrieval-request-2026-1" {}`),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		got := retrieval.SigningDomainRequestV1
		before := got
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) || got != before {
				t.Fatalf("SigningDomain.UnmarshalJSON(%q) = (%v, %v), want preserved %v and %v", data, got, gotErr, before, core.ErrJSONContract)
			}
			return
		}
		if got.Validate() != nil || !got.IsValid() {
			t.Fatalf("SigningDomain.UnmarshalJSON(%q) admitted %v validation %v, want valid", data, got, got.Validate())
		}
		encoded, marshalErr := got.MarshalJSON()
		var roundTrip retrieval.SigningDomain
		roundTripErr := roundTrip.UnmarshalJSON(encoded)
		second, secondErr := roundTrip.MarshalJSON()
		if marshalErr != nil || roundTripErr != nil || secondErr != nil || roundTrip != got || !bytes.Equal(second, encoded) {
			t.Fatalf(
				"SigningDomain accepted closure = (%v, %q, %v, %v, %v), want (%v, %q, nil, nil, nil)",
				roundTrip, second, marshalErr, roundTripErr, secondErr, got, encoded,
			)
		}
	})
}
