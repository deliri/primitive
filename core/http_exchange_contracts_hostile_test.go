package core

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestHTTPEndpointHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	exactPrefix := "https://example.test/"
	exactEndpoint := exactPrefix + strings.Repeat(
		"a",
		httpEndpointMaximumBytes-len(exactPrefix),
	)
	cases := []struct {
		wantErr    error
		name       string
		value      string
		wantString string
	}{
		{name: "minimum HTTP authority is accepted", value: "http://a", wantString: "http://a"},
		{name: "minimum HTTPS authority is accepted", value: "https://a", wantString: "https://a"},
		{name: "IPv4 authority is accepted", value: "http://127.0.0.1", wantString: "http://127.0.0.1"},
		{name: "bracketed IPv6 authority is accepted", value: "http://[::1]", wantString: "http://[::1]"},
		{name: "minimum numeric port is accepted", value: "http://example.test:1", wantString: "http://example.test:1"},
		{name: "maximum numeric port is accepted", value: "http://example.test:65535", wantString: "http://example.test:65535"},
		{name: "escaped path remains standard library owned", value: "https://example.test/a%2Fb", wantString: "https://example.test/a%2Fb"},
		{name: "query remains part of the target", value: "https://example.test/search?q=a%20b", wantString: "https://example.test/search?q=a%20b"},
		{name: "empty query marker remains representable", value: "https://example.test/path?", wantString: "https://example.test/path?"},
		{name: "one byte below endpoint bound is accepted", value: exactEndpoint[:len(exactEndpoint)-1], wantString: exactEndpoint[:len(exactEndpoint)-1]},
		{name: "exact endpoint bound is accepted", value: exactEndpoint, wantString: exactEndpoint},
		{name: "empty endpoint is rejected", value: "", wantErr: ErrPrimitiveContract},
		{name: "relative path is rejected", value: "/relative", wantErr: ErrPrimitiveContract},
		{name: "scheme-relative authority is rejected", value: "//example.test/path", wantErr: ErrPrimitiveContract},
		{name: "uppercase scheme follows standard library canonicalization", value: "HTTPS://example.test", wantString: "https://example.test"},
		{name: "non HTTP scheme is rejected", value: "ftp://example.test", wantErr: ErrPrimitiveContract},
		{name: "opaque HTTP target is rejected", value: "http:opaque", wantErr: ErrPrimitiveContract},
		{name: "missing authority is rejected", value: "https:///path", wantErr: ErrPrimitiveContract},
		{name: "username is rejected", value: "https://user@example.test", wantErr: ErrPrimitiveContract},
		{name: "username and password are rejected", value: "https://user:secret@example.test", wantErr: ErrPrimitiveContract},
		{name: "fragment is rejected", value: "https://example.test/path#fragment", wantErr: ErrPrimitiveContract},
		{name: "zero port is rejected", value: "https://example.test:0", wantErr: ErrPrimitiveContract},
		{name: "one above maximum port is rejected", value: "https://example.test:65536", wantErr: ErrPrimitiveContract},
		{name: "alphabetic port is rejected", value: "https://example.test:https", wantErr: ErrPrimitiveContract},
		{name: "authority space is rejected", value: "https://example .test", wantErr: ErrPrimitiveContract},
		{name: "authority tab is rejected", value: "https://example.test\t/path", wantErr: ErrPrimitiveContract},
		{name: "authority CRLF injection is rejected", value: "https://example.test\r\nX-Test:value", wantErr: ErrPrimitiveContract},
		{name: "unterminated IPv6 authority is rejected", value: "https://[::1", wantErr: ErrPrimitiveContract},
		{name: "one byte above endpoint bound is rejected", value: exactEndpoint + "a", wantErr: ErrPrimitiveContract},
		{name: "far above endpoint bound is rejected", value: exactEndpoint + strings.Repeat("a", httpEndpointMaximumBytes), wantErr: ErrPrimitiveContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := ParseHTTPEndpoint(tc.value)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("ParseHTTPEndpoint(%q) error = %v, want %v", tc.value, gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (HTTPEndpoint{}) {
					t.Fatalf("ParseHTTPEndpoint(%q) value = %v, want zero", tc.value, got)
				}
				return
			}
			if got.String() != tc.wantString {
				t.Fatalf("HTTPEndpoint.String() = %q, want %q", got.String(), tc.wantString)
			}
			gotURL := got.HTTPURL()
			if gotURL.String() != tc.wantString {
				t.Fatalf("HTTPEndpoint.HTTPURL().String() = %q, want %q", gotURL.String(), tc.wantString)
			}
			gotWire, gotMarshalErr := json.Marshal(got)
			if gotMarshalErr != nil {
				t.Fatalf("json.Marshal(HTTPEndpoint) error = %v, want nil", gotMarshalErr)
			}
			var gotRoundTrip HTTPEndpoint
			gotUnmarshalErr := json.Unmarshal(gotWire, &gotRoundTrip)
			if gotUnmarshalErr != nil || gotRoundTrip.String() != got.String() {
				t.Fatalf(
					"HTTPEndpoint JSON round trip = (%v, %v), want (%v, nil)",
					gotRoundTrip,
					gotUnmarshalErr,
					got,
				)
			}
		})
	}
}

func TestHTTPEndpointSameOriginHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		left  string
		right string
		want  bool
	}{
		{name: "identical HTTP endpoints share an origin", left: "http://example.test", right: "http://example.test", want: true},
		{name: "paths do not change the origin", left: "https://example.test/a", right: "https://example.test/b", want: true},
		{name: "queries do not change the origin", left: "https://example.test/?a=1", right: "https://example.test/?b=2", want: true},
		{name: "host case is normalized", left: "https://EXAMPLE.test", right: "https://example.test", want: true},
		{name: "implicit HTTP port equals port 80", left: "http://example.test", right: "http://example.test:80", want: true},
		{name: "implicit HTTPS port equals port 443", left: "https://example.test", right: "https://example.test:443", want: true},
		{name: "matching explicit ports share an origin", left: "https://example.test:8443", right: "https://example.test:8443", want: true},
		{name: "IPv6 default ports normalize", left: "http://[::1]", right: "http://[::1]:80", want: true},
		{name: "different schemes do not share an origin", left: "http://example.test", right: "https://example.test"},
		{name: "different hosts do not share an origin", left: "https://example.test", right: "https://other.test"},
		{name: "different explicit ports do not share an origin", left: "https://example.test:8443", right: "https://example.test:9443"},
		{name: "explicit HTTP default is not the HTTPS default", left: "http://example.test:80", right: "https://example.test:443"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			left, leftErr := ParseHTTPEndpoint(tc.left)
			right, rightErr := ParseHTTPEndpoint(tc.right)
			if leftErr != nil || rightErr != nil {
				t.Fatalf(
					"ParseHTTPEndpoint pair errors = (%v, %v), want (nil, nil)",
					leftErr,
					rightErr,
				)
			}
			if got := left.SameOrigin(right); got != tc.want {
				t.Fatalf(
					"HTTPEndpoint(%q).SameOrigin(%q) = %t, want %t",
					tc.left,
					tc.right,
					got,
					tc.want,
				)
			}
		})
	}

	valid, gotErr := ParseHTTPEndpoint("https://example.test")
	if gotErr != nil {
		t.Fatalf("ParseHTTPEndpoint() setup error = %v, want nil", gotErr)
	}
	if (HTTPEndpoint{}).SameOrigin(valid) || valid.SameOrigin(HTTPEndpoint{}) {
		t.Fatal("SameOrigin() with an unset endpoint = true, want false")
	}
}
