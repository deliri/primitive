package exchange

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestHTTPContentCodingHostileBoundary(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		value      string
		want       string
		wantReject bool
	}{
		{name: "identity", value: "identity", want: "identity"},
		{name: "uppercase identity canonicalizes", value: "IDENTITY", want: "identity"},
		{name: "registered coding", value: "gzip", want: "gzip"},
		{name: "extension coding", value: "x-example", want: "x-example"},
		{name: "token punctuation", value: "!#$%&'*+-.^_`|~", want: "!#$%&'*+-.^_`|~"},
		{name: "empty", wantReject: true},
		{name: "space", value: "gzip br", wantReject: true},
		{name: "list", value: "gzip,br", wantReject: true},
		{name: "parameter", value: "gzip;q=1", wantReject: true},
		{name: "slash", value: "application/gzip", wantReject: true},
		{name: "equals", value: "gzip=1", wantReject: true},
		{name: "CRLF", value: "gzip\r\nX-Test:value", wantReject: true},
		{name: "NUL", value: "gzip\x00", wantReject: true},
		{name: "non ASCII", value: "gżip", wantReject: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := parseHTTPContentCoding(tc.value)
			if tc.wantReject {
				if got != (httpContentCoding{}) || !errors.Is(gotErr, core.ErrExchangeContract) {
					t.Fatalf("parseHTTPContentCoding(%q) = (%v, %v), want (zero, %v)", tc.value, got, gotErr, core.ErrExchangeContract)
				}
				return
			}
			if gotErr != nil || got.String() != tc.want || got.Validate() != nil {
				t.Fatalf("parseHTTPContentCoding(%q) = (%v, %v), want (%q, nil)", tc.value, got, gotErr, tc.want)
			}
		})
	}
	if err := (httpContentCoding{value: "IDENTITY"}).Validate(); !errors.Is(err, core.ErrExchangeContract) {
		t.Fatalf("noncanonical HTTP content coding Validate() error = %v, want %v", err, core.ErrExchangeContract)
	}
}
