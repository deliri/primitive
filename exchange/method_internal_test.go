package exchange

import (
	json "encoding/json/v2"
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestMethodExhaustsClosedDomain(t *testing.T) {
	t.Parallel()

	for raw := 0; raw <= math.MaxUint8; raw++ {
		method := Method(raw)
		gotErr := method.Validate()
		wantValid := method > MethodUnknown && method < methodLimit
		if gotValid := method.IsValid(); gotValid != wantValid {
			t.Fatalf("Method(%d).IsValid() = %t, want %t", raw, gotValid, wantValid)
		}
		if (gotErr == nil) != wantValid {
			t.Fatalf("Method(%d).Validate() error = %v, want valid %t", raw, gotErr, wantValid)
		}
		if !wantValid {
			if !errors.Is(gotErr, core.ErrExchangeContract) {
				t.Fatalf("Method(%d).Validate() error = %v, want %v", raw, gotErr, core.ErrExchangeContract)
			}
			continue
		}
		gotParsed, gotParseErr := parseMethod(method.String())
		if gotParseErr != nil || gotParsed != method {
			t.Fatalf("parseMethod(%q) = (%v, %v), want (%v, nil)", method.String(), gotParsed, gotParseErr, method)
		}
		wire, gotMarshalErr := json.Marshal(method)
		var decoded Method
		gotUnmarshalErr := json.Unmarshal(wire, &decoded)
		if gotMarshalErr != nil || gotUnmarshalErr != nil || decoded != method {
			t.Fatalf("Method(%d) JSON round trip = (%s, %v, %v, %v), want exact", raw, wire, decoded, gotMarshalErr, gotUnmarshalErr)
		}
	}

	got, gotErr := parseMethod("get")
	if got != MethodUnknown || !errors.Is(gotErr, core.ErrExchangeContract) {
		t.Fatalf("parseMethod(%q) = (%v, %v), want (%v, %v)", "get", got, gotErr, MethodUnknown, core.ErrExchangeContract)
	}
	before := MethodGet
	if gotErr := json.Unmarshal([]byte(`"get"`), &before); !errors.Is(gotErr, core.ErrJSONContract) || before != MethodGet {
		t.Fatalf("json.Unmarshal(lowercase method) = (%v, %v), want preserved %v and %v", before, gotErr, MethodGet, core.ErrJSONContract)
	}
	if gotErr := (*Method)(nil).UnmarshalJSON(nil); !errors.Is(gotErr, core.ErrJSONContract) {
		t.Fatalf("nil Method.UnmarshalJSON() error = %v, want %v", gotErr, core.ErrJSONContract)
	}
}
