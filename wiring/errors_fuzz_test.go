package wiring

import (
	"bytes"
	jsonv2 "encoding/json/v2"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzErrorKindJSONSemanticOracle(f *testing.F) {
	for kind := ErrorKindUnknown + 1; kind < errorKindLimit; kind++ {
		canonical, err := kind.MarshalJSON()
		if err != nil {
			f.Fatalf("ErrorKind(%d).MarshalJSON() seed error = %v, want nil", kind, err)
		}
		f.Add(canonical)
		spaced := append([]byte(" \n"), canonical...)
		spaced = append(spaced, '\t')
		f.Add(spaced)
	}
	f.Add([]byte{})
	f.Add([]byte(` `))
	f.Add([]byte(`""`))
	f.Add([]byte(`null`))
	f.Add([]byte(`true`))
	f.Add([]byte(`0`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`[]`))
	f.Add([]byte(`"request`))
	f.Add([]byte(`"request"{}`))
	f.Add([]byte(`"primitive-door"`))

	f.Fuzz(func(t *testing.T, data []byte) {
		before := ErrorKindCycle
		var gotFresh ErrorKind
		gotFreshErr := gotFresh.UnmarshalJSON(data)
		gotPopulated := before
		gotPopulatedErr := gotPopulated.UnmarshalJSON(data)
		if gotFreshErr != nil || gotPopulatedErr != nil {
			if gotFresh != ErrorKindUnknown {
				t.Fatalf("fresh ErrorKind after rejection = %v, want %v", gotFresh, ErrorKindUnknown)
			}
			if gotPopulated != before {
				t.Fatalf("populated ErrorKind after rejection = %v, want preserved %v", gotPopulated, before)
			}
			if !errors.Is(gotFreshErr, core.ErrJSONContract) ||
				!errors.Is(gotPopulatedErr, core.ErrJSONContract) {
				t.Fatalf(
					"ErrorKind.UnmarshalJSON(rejected input) errors = (%v, %v), want %v for both",
					gotFreshErr,
					gotPopulatedErr,
					core.ErrJSONContract,
				)
			}
			var token string
			if decodeErr := jsonv2.Unmarshal(data, &token); decodeErr == nil &&
				!bytes.Equal(bytes.TrimSpace(data), []byte("null")) &&
				!knownErrorKindToken(token) &&
				(!errors.Is(gotFreshErr, core.ErrPrimitiveContract) ||
					!errors.Is(gotPopulatedErr, core.ErrPrimitiveContract)) {
				t.Fatalf(
					"ErrorKind.UnmarshalJSON(unknown token %q) errors = (%v, %v), want %v for both",
					token,
					gotFreshErr,
					gotPopulatedErr,
					core.ErrPrimitiveContract,
				)
			}
			return
		}
		if gotFresh != gotPopulated {
			t.Fatalf("ErrorKind.UnmarshalJSON(accepted) receivers = (%v, %v), want equal", gotFresh, gotPopulated)
		}
		var wantToken string
		if gotErr := jsonv2.Unmarshal(data, &wantToken); gotErr != nil || wantToken != gotFresh.String() {
			t.Fatalf(
				"standard JSON token = (%q, %v), want (%q, nil)",
				wantToken,
				gotErr,
				gotFresh.String(),
			)
		}
		if gotErr := gotFresh.Validate(); gotErr != nil {
			t.Fatalf("ErrorKind.UnmarshalJSON(accepted input) produced invalid %v: %v", gotFresh, gotErr)
		}
		canonical, marshalErr := gotFresh.MarshalJSON()
		if marshalErr != nil {
			t.Fatalf("accepted ErrorKind.MarshalJSON() error = %v, want nil", marshalErr)
		}
		second := ErrorKindUnknown
		if roundTripErr := second.UnmarshalJSON(canonical); roundTripErr != nil || second != gotFresh {
			t.Fatalf("ErrorKind canonical round trip = (%v, %v), want (%v, nil)", second, roundTripErr, gotFresh)
		}
		secondCanonical, gotErr := second.MarshalJSON()
		if gotErr != nil || !bytes.Equal(secondCanonical, canonical) {
			t.Fatalf(
				"ErrorKind second canonical projection = (%q, %v), want (%q, nil)",
				secondCanonical,
				gotErr,
				canonical,
			)
		}
	})
}

func knownErrorKindToken(token string) bool {
	for kind := ErrorKindUnknown + 1; kind < errorKindLimit; kind++ {
		if kind.String() == token {
			return true
		}
	}
	return false
}
