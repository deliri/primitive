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

func TestHeaderValueRedactsEveryFormattingPath(t *testing.T) {
	t.Parallel()

	const marker = "EXTERNALHEADERVALUEPOINTERDISCLOSURE"
	value, err := exchange.NewHeaderValue(marker)
	if err != nil {
		t.Fatalf("exchange.NewHeaderValue() setup error = %v, want nil", err)
	}
	formats := []struct {
		name      string
		pattern   string
		wantExact bool
	}{
		{name: "default value", pattern: "%v", wantExact: true},
		{name: "field value", pattern: "%+v", wantExact: true},
		{name: "Go syntax", pattern: "%#v", wantExact: true},
		{name: "string", pattern: "%s", wantExact: true},
		{name: "quoted string", pattern: "%q", wantExact: true},
		{name: "binary", pattern: "%b", wantExact: true},
		{name: "character", pattern: "%c", wantExact: true},
		{name: "decimal", pattern: "%d", wantExact: true},
		{name: "octal", pattern: "%o", wantExact: true},
		{name: "prefixed octal", pattern: "%O", wantExact: true},
		{name: "lower hexadecimal", pattern: "%x", wantExact: true},
		{name: "upper hexadecimal", pattern: "%X", wantExact: true},
		{name: "Unicode", pattern: "%U", wantExact: true},
		{name: "boolean", pattern: "%t", wantExact: true},
		{name: "lower exponent", pattern: "%e", wantExact: true},
		{name: "upper exponent", pattern: "%E", wantExact: true},
		{name: "lower decimal point", pattern: "%f", wantExact: true},
		{name: "upper decimal point", pattern: "%F", wantExact: true},
		{name: "compact lower exponent", pattern: "%g", wantExact: true},
		{name: "compact upper exponent", pattern: "%G", wantExact: true},
		{name: "left width", pattern: "%-20v", wantExact: true},
		{name: "zero width", pattern: "%020v", wantExact: true},
		{name: "precision", pattern: "%.3v", wantExact: true},
		{name: "space flag", pattern: "% v", wantExact: true},
		{name: "dynamic type", pattern: "%T"},
		{name: "pointer identity", pattern: "%p"},
	}
	for _, tc := range formats {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := fmt.Sprintf(tc.pattern, value)
			if tc.wantExact && got != core.RedactedValueText {
				t.Fatalf("fmt.Sprintf(%q) = %q, want %q", tc.pattern, got, core.RedactedValueText)
			}
			if strings.Contains(got, marker) {
				t.Fatalf("fmt.Sprintf(%q) disclosed the header value", tc.pattern)
			}
		})
	}
}
