package core

import (
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestHTTPMethodExhaustsClosedDomain(t *testing.T) {
	t.Parallel()

	for raw := 0; raw <= math.MaxUint8; raw++ {
		method := HTTPMethod(raw)
		gotErr := method.Validate()
		wantValid := method > HTTPMethodUnknown && method < httpMethodLimit
		if gotValid := method.IsValid(); gotValid != wantValid {
			t.Fatalf("HTTPMethod(%d).IsValid() = %t, want %t", raw, gotValid, wantValid)
		}
		if (gotErr == nil) != wantValid {
			t.Fatalf("HTTPMethod(%d).Validate() error = %v, want valid %t", raw, gotErr, wantValid)
		}
		if wantValid {
			gotParsed, gotParseErr := ParseHTTPMethod(method.String())
			if gotParseErr != nil || gotParsed != method {
				t.Fatalf("ParseHTTPMethod(%q) = (%v, %v), want (%v, nil)", method.String(), gotParsed, gotParseErr, method)
			}
			gotJSON, gotJSONErr := json.Marshal(method)
			if gotJSONErr != nil {
				t.Fatalf("json.Marshal(HTTPMethod(%d)) error = %v, want nil", raw, gotJSONErr)
			}
			var gotRoundTrip HTTPMethod
			gotRoundTripErr := json.Unmarshal(gotJSON, &gotRoundTrip)
			if gotRoundTripErr != nil || gotRoundTrip != method {
				t.Fatalf("HTTPMethod(%d) JSON round trip = (%v, %v), want (%v, nil)", raw, gotRoundTrip, gotRoundTripErr, method)
			}
		} else if !errors.Is(gotErr, ErrPrimitiveContract) {
			t.Fatalf("HTTPMethod(%d).Validate() error = %v, want %v", raw, gotErr, ErrPrimitiveContract)
		}

	}
}

func TestHTTPStatusCodeExhaustsProtocolDomain(t *testing.T) {
	t.Parallel()

	for raw := 0; raw <= math.MaxUint16; raw++ {
		got, gotErr := NewHTTPStatusCode(raw)
		wantValid := raw >= HTTPStatusCodeMinimum && raw <= HTTPStatusCodeMaximum
		if (gotErr == nil) != wantValid {
			t.Fatalf("NewHTTPStatusCode(%d) error = %v, want valid %t", raw, gotErr, wantValid)
		}
		if !wantValid {
			if !errors.Is(gotErr, ErrPrimitiveContract) {
				t.Fatalf("NewHTTPStatusCode(%d) error = %v, want %v", raw, gotErr, ErrPrimitiveContract)
			}
			continue
		}
		gotInt, gotIntErr := got.Int()
		if gotIntErr != nil || gotInt != raw {
			t.Fatalf("HTTPStatusCode.Int() = (%d, %v), want (%d, nil)", gotInt, gotIntErr, raw)
		}
		gotJSON, gotJSONErr := json.Marshal(got)
		if gotJSONErr != nil || string(gotJSON) != strconv.Itoa(raw) {
			t.Fatalf("json.Marshal(HTTPStatusCode(%d)) = (%s, %v), want (%d, nil)", raw, gotJSON, gotJSONErr, raw)
		}
		var gotRoundTrip HTTPStatusCode
		gotRoundTripErr := json.Unmarshal(gotJSON, &gotRoundTrip)
		if gotRoundTripErr != nil || gotRoundTrip != got {
			t.Fatalf(
				"HTTPStatusCode(%d) JSON round trip = (%v, %v), want (%v, nil)",
				raw,
				gotRoundTrip,
				gotRoundTripErr,
				got,
			)
		}
		wantInformational := raw >= HTTPStatusCodeMinimum && raw <= HTTPStatusInformationalMaximum
		wantSuccess := raw >= HTTPStatusSuccessMinimum && raw <= HTTPStatusSuccessMaximum
		wantRedirect := raw >= HTTPStatusRedirectMinimum && raw <= HTTPStatusRedirectMaximum
		wantClientError := raw >= HTTPStatusClientErrorMinimum && raw <= HTTPStatusClientErrorMaximum
		wantServerError := raw >= HTTPStatusServerErrorMinimum && raw <= HTTPStatusCodeMaximum
		if got.IsInformational() != wantInformational ||
			got.IsSuccess() != wantSuccess ||
			got.IsRedirect() != wantRedirect ||
			got.IsClientError() != wantClientError ||
			got.IsServerError() != wantServerError {
			t.Fatalf(
				"HTTPStatusCode(%d) classes = (%t,%t,%t,%t,%t), want (%t,%t,%t,%t,%t)",
				raw,
				got.IsInformational(), got.IsSuccess(), got.IsRedirect(),
				got.IsClientError(), got.IsServerError(),
				wantInformational, wantSuccess, wantRedirect,
				wantClientError, wantServerError,
			)
		}
	}
}

func TestHTTPHeaderNameHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		value       string
		want        string
		disposition boundaryDisposition
	}{
		{name: "minimum alphabetic token", value: "a", want: "A", disposition: boundaryAccept},
		{name: "single numeric token", value: "0", want: "0", disposition: boundaryAccept},
		{name: "standard content type", value: "Content-Type", want: "Content-Type", disposition: boundaryAccept},
		{name: "lowercase canonicalizes", value: "content-type", want: "Content-Type", disposition: boundaryAccept},
		{name: "uppercase canonicalizes", value: "CONTENT-TYPE", want: "Content-Type", disposition: boundaryAccept},
		{name: "mixed case canonicalizes", value: "cOnTeNt-TyPe", want: "Content-Type", disposition: boundaryAccept},
		{name: "all admitted punctuation tokens", value: "!#$%&'*+-.^_`|~", want: "!#$%&'*+-.^_`|~", disposition: boundaryAccept},
		{name: "digits and letters", value: "X-Trace-123", want: "X-Trace-123", disposition: boundaryAccept},
		{name: "one below maximum length", value: strings.Repeat("a", HTTPHeaderNameMaximumBytes-1), want: "A" + strings.Repeat("a", HTTPHeaderNameMaximumBytes-2), disposition: boundaryAccept},
		{name: "exact maximum length", value: strings.Repeat("a", HTTPHeaderNameMaximumBytes), want: "A" + strings.Repeat("a", HTTPHeaderNameMaximumBytes-1), disposition: boundaryAccept},
		{name: "empty name is rejected", value: ""},
		{name: "one above maximum length is rejected", value: strings.Repeat("a", HTTPHeaderNameMaximumBytes+1)},
		{name: "far above maximum length is rejected", value: strings.Repeat("a", HTTPHeaderNameMaximumBytes*4)},
		{name: "ASCII space is rejected", value: "Content Type"},
		{name: "leading space is rejected", value: " Content-Type"},
		{name: "trailing space is rejected", value: "Content-Type "},
		{name: "colon is rejected", value: "Content:Type"},
		{name: "slash is rejected", value: "Content/Type"},
		{name: "backslash is rejected", value: `Content\Type`},
		{name: "opening parenthesis is rejected", value: "Content(Type"},
		{name: "closing parenthesis is rejected", value: "Content)Type"},
		{name: "opening bracket is rejected", value: "Content[Type"},
		{name: "closing bracket is rejected", value: "Content]Type"},
		{name: "opening brace is rejected", value: "Content{Type"},
		{name: "closing brace is rejected", value: "Content}Type"},
		{name: "comma is rejected", value: "Content,Type"},
		{name: "semicolon is rejected", value: "Content;Type"},
		{name: "equals is rejected", value: "Content=Type"},
		{name: "question mark is rejected", value: "Content?Type"},
		{name: "at sign is rejected", value: "Content@Type"},
		{name: "double quote is rejected", value: `Content"Type`},
		{name: "tab is rejected", value: "Content\tType"},
		{name: "newline is rejected", value: "Content\nType"},
		{name: "carriage return is rejected", value: "Content\rType"},
		{name: "NUL is rejected", value: "Content\x00Type"},
		{name: "non ASCII letter is rejected", value: "Contént-Type"},
		{name: "invalid UTF8 is rejected", value: string([]byte{'X', '-', 0xff})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := ParseHTTPHeaderName(tc.value)
			wantAccept := tc.disposition == boundaryAccept
			if (gotErr == nil) != wantAccept {
				t.Fatalf("ParseHTTPHeaderName(%q) = (%v, %v), want accept %t", tc.value, got, gotErr, wantAccept)
			}
			if !wantAccept {
				if !errors.Is(gotErr, ErrPrimitiveContract) {
					t.Fatalf("ParseHTTPHeaderName(%q) error = %v, want %v", tc.value, gotErr, ErrPrimitiveContract)
				}
				return
			}
			if got.String() != tc.want {
				t.Fatalf("HTTPHeaderName.String() = %q, want %q", got.String(), tc.want)
			}
			if gotErr := got.Validate(); gotErr != nil {
				t.Fatalf("HTTPHeaderName.Validate() error = %v, want nil", gotErr)
			}
			directWire, directMarshalErr := got.MarshalJSON()
			if directMarshalErr != nil {
				t.Fatalf("HTTPHeaderName.MarshalJSON(%q) error = %v, want nil", got, directMarshalErr)
			}
			if !json.Valid(directWire) {
				t.Fatalf("HTTPHeaderName.MarshalJSON(%q) wire = %q, want valid JSON", got, directWire)
			}
			directRoundTrip, directDecodeErr := DecodeStrictJSON[HTTPHeaderName](
				directWire,
				DefaultStrictJSONLimits(),
			)
			if directDecodeErr != nil || directRoundTrip != got {
				t.Fatalf(
					"HTTPHeaderName direct JSON round trip = (%v, %v), want (%v, nil)",
					directRoundTrip,
					directDecodeErr,
					got,
				)
			}
			gotWire, gotEncodeErr := EncodeValidatedJSON(got, DefaultStrictJSONLimits())
			if gotEncodeErr != nil {
				t.Fatalf("EncodeValidatedJSON(HTTPHeaderName %q) error = %v, want nil", got, gotEncodeErr)
			}
			gotRoundTrip, gotDecodeErr := DecodeStrictJSON[HTTPHeaderName](
				gotWire,
				DefaultStrictJSONLimits(),
			)
			if gotDecodeErr != nil || gotRoundTrip != got {
				t.Fatalf(
					"HTTPHeaderName strict JSON round trip = (%v, %v), want (%v, nil)",
					gotRoundTrip,
					gotDecodeErr,
					got,
				)
			}
		})
	}
}

func TestHTTPFieldValueHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		value     string
		wantValid bool
	}{
		{name: "empty field content is admitted", value: "", wantValid: true},
		{name: "horizontal tab is admitted", value: "a\tb", wantValid: true},
		{name: "space is admitted", value: "a b", wantValid: true},
		{name: "lowest visible byte is admitted", value: "\x20", wantValid: true},
		{name: "highest visible ASCII byte is admitted", value: "\x7e", wantValid: true},
		{name: "first obs-text byte is admitted", value: "\x80", wantValid: true},
		{name: "maximum byte is admitted", value: "\xff", wantValid: true},
		{name: "UTF-8 bytes are admitted as obs-text", value: "grüße", wantValid: true},
		{name: "leading and trailing spaces are admitted", value: " padded ", wantValid: true},
		{name: "visible ASCII punctuation is admitted", value: "!#$%&'*+-.^_`|~", wantValid: true},
		{name: "NUL is rejected", value: "a\x00b"},
		{name: "one below horizontal tab is rejected", value: "\x08"},
		{name: "line feed is rejected", value: "\n"},
		{name: "vertical tab is rejected", value: "\x0b"},
		{name: "form feed is rejected", value: "\x0c"},
		{name: "carriage return is rejected", value: "\r"},
		{name: "escape is rejected", value: "\x1b"},
		{name: "one below visible ASCII is rejected", value: "\x1f"},
		{name: "DEL is rejected", value: "\x7f"},
		{name: "control byte after visible content is rejected", value: "visible\x01"},
		{name: "CRLF field injection is rejected", value: "safe\r\nX-Injected: yes"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := ValidateHTTPFieldValue(tc.value)
			if (gotErr == nil) != tc.wantValid {
				t.Fatalf(
					"ValidateHTTPFieldValue(%q) error = %v, want valid %t",
					tc.value,
					gotErr,
					tc.wantValid,
				)
			}
			if !tc.wantValid && !errors.Is(gotErr, ErrPrimitiveContract) {
				t.Fatalf(
					"ValidateHTTPFieldValue(%q) error = %v, want %v",
					tc.value,
					gotErr,
					ErrPrimitiveContract,
				)
			}
		})
	}
}

func TestCoreHTTPHeaderConstantsAreValidated(t *testing.T) {
	t.Parallel()

	headers := [...]HTTPHeaderName{
		HTTPHeaderContentType(),
		HTTPHeaderAccept(),
		HTTPHeaderAuthorization(),
		HTTPHeaderRetryAfter(),
		HTTPHeaderContentLength(),
		HTTPHeaderContentRange(),
		HTTPHeaderContentEncoding(),
		HTTPHeaderAcceptEncoding(),
		HTTPHeaderIdempotencyKey(),
		HTTPHeaderLocation(),
		HTTPHeaderCacheControl(),
	}
	seen := make(map[string]int, len(headers))
	for index, header := range headers {
		if gotErr := header.Validate(); gotErr != nil {
			t.Fatalf("HTTP header constant index %d Validate() error = %v, want nil", index, gotErr)
		}
		parsed, gotParseErr := ParseHTTPHeaderName(header.String())
		if gotParseErr != nil || parsed != header {
			t.Fatalf("HTTP header constant index %d parser round trip = (%v, %v), want (%v, nil)", index, parsed, gotParseErr, header)
		}
		if prior, duplicate := seen[header.String()]; duplicate {
			t.Fatalf("HTTP header constant index %d repeats %q from index %d, want one accessor per field name", index, header.String(), prior)
		}
		seen[header.String()] = index
	}
	if gotDeclared := countHTTPHeaderNameAccessors(t); gotDeclared != len(headers) {
		t.Fatalf(
			"declared HTTPHeaderName accessors = %d, want the %d this table validates",
			gotDeclared,
			len(headers),
		)
	}
}

// countHTTPHeaderNameAccessors counts every package-level accessor returning a
// HTTPHeaderName. Reflection cannot enumerate package functions, so the count is
// read from the source: without it, adding a typed field name and forgetting to
// list it above would leave an unvalidated protocol constant in the package.
func countHTTPHeaderNameAccessors(t *testing.T) int {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("os.ReadDir() error = %v, want nil", err)
	}
	positions := token.NewFileSet()
	count := 0
	for _, entry := range entries {
		if entry.IsDir() ||
			!strings.HasSuffix(entry.Name(), ".go") ||
			strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(positions, entry.Name(), nil, 0)
		if parseErr != nil {
			t.Fatalf("parser.ParseFile(%q) error = %v, want nil", entry.Name(), parseErr)
		}
		count += httpHeaderAccessorsInFile(file)
	}
	return count
}

func httpHeaderAccessorsInFile(file *ast.File) int {
	count := 0
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Recv != nil ||
			function.Type.Params.NumFields() != 0 ||
			function.Type.Results.NumFields() != 1 {
			continue
		}
		result, ok := function.Type.Results.List[0].Type.(*ast.Ident)
		if ok && result.Name == "HTTPHeaderName" {
			count++
		}
	}
	return count
}

func TestHTTPMediaTypeParsesRealHeaderValues(t *testing.T) {
	t.Parallel()

	longParameterPrefix := httpMediaTypeJSONText + `; note="`
	longParameterSuffix := `"`
	exactMaximumParameter := strings.Repeat(
		"a",
		HTTPMediaTypeMaximumBytes-len(longParameterPrefix)-len(longParameterSuffix),
	)
	cases := []struct {
		name        string
		value       string
		wantBase    HTTPMediaType
		disposition boundaryDisposition
	}{
		{name: "bare canonical json base is accepted", value: httpMediaTypeJSONText, wantBase: HTTPMediaTypeJSON(), disposition: boundaryAccept},
		{name: "bare canonical octet-stream base is accepted", value: httpMediaTypeOctetStreamText, wantBase: HTTPMediaTypeOctetStream(), disposition: boundaryAccept},
		{name: "bare canonical text base is accepted", value: httpMediaTypeTextPlainText, wantBase: HTTPMediaTypeTextPlain(), disposition: boundaryAccept},
		{name: "bare canonical timestamp-query base is accepted", value: httpMediaTypeTimestampQueryText, wantBase: HTTPMediaTypeTimestampQuery(), disposition: boundaryAccept},
		{name: "bare canonical timestamp-reply base is accepted", value: httpMediaTypeTimestampReplyText, wantBase: HTTPMediaTypeTimestampReply(), disposition: boundaryAccept},
		{name: "bare canonical PKIX CRL base is accepted", value: httpMediaTypePKIXCRLText, wantBase: HTTPMediaTypePKIXCRL(), disposition: boundaryAccept},
		{name: "uppercase type and subtype normalize to json", value: "APPLICATION/JSON", wantBase: HTTPMediaTypeJSON(), disposition: boundaryAccept},
		{name: "mixed-case type and subtype normalize to octet stream", value: "Application/Octet-Stream", wantBase: HTTPMediaTypeOctetStream(), disposition: boundaryAccept},
		{name: "json charset parameter with separator spacing is accepted", value: "application/json; charset=utf-8", wantBase: HTTPMediaTypeJSON(), disposition: boundaryAccept},
		{name: "json compact uppercase parameter value is accepted", value: "application/json;charset=UTF-8", wantBase: HTTPMediaTypeJSON(), disposition: boundaryAccept},
		{name: "quoted parameter containing semicolon is accepted", value: `text/plain; note="a;b"`, wantBase: HTTPMediaTypeTextPlain(), disposition: boundaryAccept},
		{name: "empty quoted parameter value is accepted", value: `text/plain; note=""`, wantBase: HTTPMediaTypeTextPlain(), disposition: boundaryAccept},
		{name: "two distinct parameters are accepted", value: "application/json; charset=utf-8; profile=v1", wantBase: HTTPMediaTypeJSON(), disposition: boundaryAccept},
		{name: "reordered distinct parameters preserve the base", value: "application/json; profile=v1; charset=utf-8", wantBase: HTTPMediaTypeJSON(), disposition: boundaryAccept},
		{name: "leading and trailing optional whitespace is accepted", value: " application/json ; charset=utf-8 ", wantBase: HTTPMediaTypeJSON(), disposition: boundaryAccept},
		{name: "timestamp query with a parameter preserves its base", value: "application/timestamp-query; version=1", wantBase: HTTPMediaTypeTimestampQuery(), disposition: boundaryAccept},
		{name: "PKIX CRL with a quoted parameter preserves its base", value: `application/pkix-crl; source="issuer"`, wantBase: HTTPMediaTypePKIXCRL(), disposition: boundaryAccept},
		{name: "unknown application vendor media type remains transportable", value: "application/vnd.example.exchange+json", wantBase: mustParseHTTPMediaTypeForTest(t, "application/vnd.example.exchange+json"), disposition: boundaryAccept},
		{name: "unknown text media type remains transportable", value: "text/csv", wantBase: mustParseHTTPMediaTypeForTest(t, "text/csv"), disposition: boundaryAccept},
		{
			name:        "one byte below parser bound is accepted",
			value:       longParameterPrefix + exactMaximumParameter[:len(exactMaximumParameter)-1] + longParameterSuffix,
			wantBase:    HTTPMediaTypeJSON(),
			disposition: boundaryAccept,
		},
		{
			name:        "exact parser bound is accepted",
			value:       longParameterPrefix + exactMaximumParameter + longParameterSuffix,
			wantBase:    HTTPMediaTypeJSON(),
			disposition: boundaryAccept,
		},
		{name: "minimum parameter name and value are accepted", value: "application/json;a=b", wantBase: HTTPMediaTypeJSON(), disposition: boundaryAccept},
		{name: "empty value is rejected", value: ""},
		{name: "ASCII whitespace only is rejected", value: " \t"},
		{name: "missing slash and subtype is rejected", value: "application"},
		{name: "missing type before slash is rejected", value: "/json"},
		{name: "missing subtype after slash is rejected", value: "application/"},
		{name: "extra slash segment is rejected", value: "application/json/extra"},
		{name: "unknown XML media type remains transportable", value: "application/xml", wantBase: mustParseHTTPMediaTypeForTest(t, "application/xml"), disposition: boundaryAccept},
		{name: "unknown HTML media type remains transportable", value: "text/html", wantBase: mustParseHTTPMediaTypeForTest(t, "text/html"), disposition: boundaryAccept},
		{name: "unterminated quoted parameter is rejected", value: `application/json; note="open`},
		{name: "duplicate exact parameter name is rejected", value: "application/json; charset=utf-8; charset=ascii"},
		{name: "duplicate case-folded parameter name is rejected", value: "application/json; charset=utf-8; CHARSET=ascii"},
		{name: "unquoted parameter separator is rejected", value: "application/json; note=a;b"},
		{name: "parameter missing equals and value is rejected", value: "application/json; charset"},
		{name: "parameter missing name is rejected", value: "application/json; =utf-8"},
		{name: "standard library tolerates a trailing parameter separator", value: "application/json;", wantBase: HTTPMediaTypeJSON(), disposition: boundaryAccept},
		{name: "leading comma list syntax is rejected", value: ",application/json"},
		{name: "comma-separated media type list is rejected", value: "application/json,text/plain"},
		{name: "NUL in subtype is rejected", value: "application/j\x00son"},
		{name: "CRLF suffix is rejected", value: "application/json\r\nX-Test: value"},
		{name: "non-ASCII type token is rejected", value: "applicatiön/json"},
		{name: "quote after unquoted value is rejected", value: `application/json; note=value"`},
		{name: "one byte above parser bound is rejected", value: strings.Repeat("a", HTTPMediaTypeMaximumBytes+1)},
		{name: "far above parser bound is rejected", value: strings.Repeat("a", HTTPMediaTypeMaximumBytes*4)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := ParseHTTPMediaType(tc.value)
			wantAccept := tc.disposition == boundaryAccept
			if (gotErr == nil) != wantAccept {
				t.Fatalf("ParseHTTPMediaType(%q) = (%v, %v), want accept %t", tc.value, got, gotErr, wantAccept)
			}
			if !wantAccept {
				if !errors.Is(gotErr, ErrPrimitiveContract) {
					t.Fatalf("ParseHTTPMediaType(%q) error = %v, want %v", tc.value, gotErr, ErrPrimitiveContract)
				}
				return
			}
			gotSameBase, gotBaseErr := got.SameBase(tc.wantBase)
			if gotBaseErr != nil || !gotSameBase {
				t.Fatalf(
					"HTTPMediaType.SameBase(%q) = (%t, %v), want (true, nil)",
					tc.value,
					gotSameBase,
					gotBaseErr,
				)
			}
			if gotErr := got.Validate(); gotErr != nil {
				t.Fatalf("HTTPMediaType.Validate(%q) error = %v, want nil", tc.value, gotErr)
			}
			gotWire, gotMarshalErr := got.MarshalJSON()
			if gotMarshalErr != nil {
				t.Fatalf("HTTPMediaType.MarshalJSON(%q) error = %v, want nil", tc.value, gotMarshalErr)
			}
			var gotRoundTrip HTTPMediaType
			gotUnmarshalErr := json.Unmarshal(gotWire, &gotRoundTrip)
			if gotUnmarshalErr != nil || gotRoundTrip != got {
				t.Fatalf(
					"HTTPMediaType JSON round trip = (%v, %v), want (%v, nil)",
					gotRoundTrip,
					gotUnmarshalErr,
					got,
				)
			}
		})
	}
}

func mustParseHTTPMediaTypeForTest(t *testing.T, value string) HTTPMediaType {
	t.Helper()

	got, gotErr := ParseHTTPMediaType(value)
	if gotErr != nil {
		t.Fatalf("ParseHTTPMediaType(%q) setup error = %v, want nil", value, gotErr)
	}
	return got
}
