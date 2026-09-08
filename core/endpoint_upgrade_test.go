package core

import (
	"errors"
	"net/url"
	"strings"
	"testing"
)

func TestHTTPEndpointCanonicalExtentClosure(t *testing.T) {
	t.Parallel()
	const prefix = "https://example.invalid/"
	cases := []struct {
		name    string
		source  string
		wantErr error
	}{
		{name: "neutral/short empty path", source: prefix},
		{name: "positive/escaped space retains its Go spelling", source: prefix + " "},
		{name: "positive/unicode uses Go escaping", source: prefix + "é"},
		{name: "positive/existing escape spelling is retained", source: prefix + "%2f"},
		{name: "negative/invalid escape remains rejected", source: prefix + "%", wantErr: ErrPrimitiveContract},
	}
	widths := []struct {
		name    string
		size    int
		wantErr error
	}{
		{name: "one below", size: httpEndpointMaximumBytes - 1},
		{name: "exact", size: httpEndpointMaximumBytes},
		{name: "one above", size: httpEndpointMaximumBytes + 1, wantErr: ErrPrimitiveContract},
	}
	for _, width := range widths {
		for _, shape := range []struct {
			name, unit   string
			encodedWidth int
		}{
			{name: "ASCII", unit: "a", encodedWidth: 1},
			{name: "spaces", unit: " ", encodedWidth: 3},
			{name: "two-byte UTF8", unit: "é", encodedWidth: 6},
			{name: "four-byte UTF8", unit: "🙂", encodedWidth: 12},
		} {
			budget := width.size - len(prefix)
			source := prefix + strings.Repeat(shape.unit, budget/shape.encodedWidth) + strings.Repeat("a", budget%shape.encodedWidth)
			parsed, err := url.Parse(source)
			if err != nil || len(parsed.String()) != width.size {
				t.Fatalf("fixture URL=%v, %v; want canonical extent %d", parsed, err, width.size)
			}
			cases = append(cases, struct {
				name    string
				source  string
				wantErr error
			}{name: "boundary/" + shape.name + "/" + width.name, source: source, wantErr: width.wantErr})
		}
	}
	// The shortcut must agree with Go on both sides of its conservative
	// threshold. Raw byte extent and encoded extent are distinct axes.
	for _, delta := range []struct {
		name  string
		value int
	}{
		{name: "one below", value: -1}, {name: "exact"}, {name: "one above", value: 1},
	} {
		for _, shape := range []struct{ name, unit string }{
			{name: "ASCII", unit: "a"}, {name: "escaped byte", unit: " "},
			{name: "multibyte", unit: "é"}, {name: "existing escape", unit: "%2F"},
		} {
			size := httpEndpointMaximumBytes/httpPercentEncodedByteWidth + delta.value
			budget := size - len(prefix)
			source := prefix + strings.Repeat(shape.unit, budget/len(shape.unit)) + strings.Repeat("a", budget%len(shape.unit))
			cases = append(cases, struct {
				name    string
				source  string
				wantErr error
			}{name: "short-input threshold/" + shape.name + "/" + delta.name, source: source})
		}
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseHTTPEndpoint(tc.source)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("parse source %d bytes=%v; want error %v", len(tc.source), err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (HTTPEndpoint{}) {
					t.Fatalf("refused endpoint=%v; want zero", got)
				}
				return
			}
			native, nativeErr := url.Parse(tc.source)
			if nativeErr != nil {
				t.Fatal(nativeErr)
			}
			wire := got.String()
			if wire != native.String() || len(wire) > httpEndpointMaximumBytes {
				t.Fatalf("publication=%d bytes; want exact bounded Go URL of %d bytes", len(wire), len(native.String()))
			}
			again, againErr := ParseHTTPEndpoint(wire)
			if againErr != nil || again.String() != wire || !got.SameOrigin(again) {
				t.Fatalf("reparse=%v, %v; want exact publication closure", again, againErr)
			}
			owned := got.HTTPURL()
			owned.Host = "changed.invalid"
			owned.Path = "/changed"
			if got.String() != wire {
				t.Fatalf("copy mutation changed owner to %q; want %q", got.String(), wire)
			}
		})
	}
}
