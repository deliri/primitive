package exchange_test

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"golang.org/x/net/http/httpguts"
)

// FuzzHeaderValueMatchesNetHTTPAndExchangeBounds uses Go's independent HTTP
// grammar implementation as the semantic oracle. Acceptance must also obey
// Exchange's compiler-owned extent ceiling, preserve the exact typed value,
// and remain valid when placed inside the owning Header structure.
func FuzzHeaderValueMatchesNetHTTPAndExchangeBounds(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte("ordinary"))
	f.Add([]byte(" padded\tvalue "))
	f.Add([]byte{0})
	f.Add([]byte{'a', '\r', '\n', 'b'})
	f.Add([]byte{0x7f})
	f.Add([]byte{0x80, 0xff})
	f.Add(bytes.Repeat([]byte{'a'}, exchange.HeaderValueMaximumBytes))
	f.Add(bytes.Repeat([]byte{'a'}, exchange.HeaderValueMaximumBytes+1))

	name, err := core.ParseHTTPHeaderName("X-Primitive-Fuzz")
	if err != nil {
		f.Fatalf("core.ParseHTTPHeaderName(seed) error = %v, want nil", err)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		value, gotErr := exchange.NewHeaderValue(string(data))
		standardAccepted := httpguts.ValidHeaderFieldValue(string(data))
		wantAccepted := len(data) <= exchange.HeaderValueMaximumBytes && standardAccepted

		rendered := fmt.Sprintf("%v|%+v|%#v|%s|%q", value, value, value, value, value)
		if strings.Count(rendered, core.RedactedValueText) != 5 {
			t.Fatalf("HeaderValue formatting disclosed an external value: %q", rendered)
		}
		if wantAccepted {
			if gotErr != nil {
				t.Fatalf("HeaderValue.Validate() error = %v, want nil for a bounded net/http value", gotErr)
			}
			projected, projectedErr := value.Value()
			if projectedErr != nil || projected != string(data) {
				t.Fatalf("HeaderValue.Value() changed %d admitted bytes", len(data))
			}
			header := exchange.Header{Name: name, Values: []exchange.HeaderValue{value}}
			if err := header.Validate(); err != nil {
				t.Fatalf("Header.Validate(admitted value) error = %v, want nil", err)
			}
			return
		}

		if !errors.Is(gotErr, core.ErrExchangeContract) || value != (exchange.HeaderValue{}) {
			t.Fatalf("NewHeaderValue(refused) = (%v, %v), want zero and errors.Is %v",
				value, gotErr, core.ErrExchangeContract)
		}
		if projected, projectedErr := value.Value(); !errors.Is(projectedErr, core.ErrExchangeContract) || projected != "" {
			t.Fatalf("HeaderValue.Value(refused) = (%q, %v), want empty and errors.Is %v",
				projected, projectedErr, core.ErrExchangeContract)
		}
		header := exchange.Header{Name: name, Values: []exchange.HeaderValue{value}}
		if err := header.Validate(); !errors.Is(err, core.ErrExchangeContract) {
			t.Fatalf("Header.Validate(refused value) error = %v, want errors.Is %v", err, core.ErrExchangeContract)
		}
	})
}
