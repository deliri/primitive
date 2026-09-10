package exchange

import (
	"bytes"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type officialSDKBoundaryConstructor uint8

const (
	officialSDKBoundarySelected officialSDKBoundaryConstructor = iota + 1
	officialSDKBoundaryAllPaths
	officialSDKBoundarySelectedJSON
	officialSDKBoundaryAllPathsJSON
)

type officialSDKBoundaryCase struct {
	wantErr     error
	name        string
	prefix      string
	suffix      string
	constructor officialSDKBoundaryConstructor
	method      Method
}

func TestOfficialSDKResponseTransportConstructionRefusesEveryUnsetDependency(t *testing.T) {
	t.Parallel()

	boundary, boundaryErr := NewOfficialSDKMethodResponseBoundary(OfficialSDKMethodResponseBoundaryRequest{
		Method: MethodGet, Representation: OfficialSDKResponseRepresentationBinary,
	})
	if boundaryErr != nil {
		t.Fatalf("NewOfficialSDKMethodResponseBoundary() error = %v, want nil", boundaryErr)
	}
	cases := []struct {
		wantErr error
		name    string
		request OfficialSDKResponseTransportRequest
	}{
		{name: "positive standard transport and validated boundary are admitted", request: OfficialSDKResponseTransportRequest{Base: http.DefaultTransport, Boundary: boundary}},
		{name: "negative nil transport is refused", request: OfficialSDKResponseTransportRequest{Boundary: boundary}, wantErr: core.ErrExchangeContract},
		{name: "negative zero boundary is refused", request: OfficialSDKResponseTransportRequest{Base: http.DefaultTransport}, wantErr: core.ErrExchangeContract},
		{name: "negative zero request is refused", wantErr: core.ErrExchangeContract},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := NewOfficialSDKResponseTransport(testCase.request)
			if testCase.wantErr != nil {
				if got != nil || !errors.Is(gotErr, testCase.wantErr) {
					t.Fatalf("NewOfficialSDKResponseTransport() = (%v, %v), want nil and errors.Is(..., %v)", got, gotErr, testCase.wantErr)
				}
				return
			}
			if got == nil || gotErr != nil {
				t.Fatalf("NewOfficialSDKResponseTransport() = (%v, %v), want non-nil and nil", got, gotErr)
			}
		})
	}
}

func TestOfficialSDKHTTPClientRefusesNilTransport(t *testing.T) {
	t.Parallel()

	got, gotErr := NewOfficialSDKHTTPClient(nil)
	if got != nil || !errors.Is(gotErr, core.ErrExchangeContract) {
		t.Fatalf("NewOfficialSDKHTTPClient(nil) = (%v, %v), want nil and errors.Is(..., %v)", got, gotErr, core.ErrExchangeContract)
	}
}

func TestOfficialSDKResponseRepresentationExhaustsEveryUint8State(t *testing.T) {
	t.Parallel()

	for raw := range 256 {
		representation := OfficialSDKResponseRepresentation(raw)
		gotErr := representation.Validate()
		var wantErr error = core.ErrExchangeContract
		wantValid := representation == OfficialSDKResponseRepresentationBinary ||
			representation == OfficialSDKResponseRepresentationJSON
		if wantValid {
			wantErr = nil
		}
		if !errors.Is(gotErr, wantErr) {
			t.Fatalf("OfficialSDKResponseRepresentation(%d).Validate() error = %v, want errors.Is(..., %v)", raw, gotErr, wantErr)
		}
		if gotValid := representation.IsValid(); gotValid != wantValid {
			t.Fatalf("OfficialSDKResponseRepresentation(%d).IsValid() = %t, want %t", raw, gotValid, wantValid)
		}
		encoded, marshalErr := representation.MarshalJSON()
		if !wantValid {
			if representation.String() != "" || encoded != nil ||
				!errors.Is(marshalErr, core.ErrJSONContract) ||
				!errors.Is(marshalErr, core.ErrExchangeContract) {
				t.Fatalf("invalid representation projection = (%q, %q, %v), want empty, nil, %v, and %v", representation.String(), encoded, marshalErr, core.ErrJSONContract, core.ErrExchangeContract)
			}
			continue
		}
		if representation.String() == "" || marshalErr != nil {
			t.Fatalf("valid representation projection = (%q, %q, %v), want non-empty canonical token and nil", representation.String(), encoded, marshalErr)
		}
		decoded := OfficialSDKResponseRepresentationUnknown
		unmarshalErr := decoded.UnmarshalJSON(encoded)
		if unmarshalErr != nil || decoded != representation {
			t.Fatalf("representation round trip = (%v, %v), want (%v, nil)", decoded, unmarshalErr, representation)
		}
		second, secondErr := decoded.MarshalJSON()
		if secondErr != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("second representation projection = (%q, %v), want (%q, nil)", second, secondErr, encoded)
		}
	}
}

func TestOfficialSDKResponseRepresentationRefusesNilJSONReceiver(t *testing.T) {
	t.Parallel()

	var representation *OfficialSDKResponseRepresentation
	gotErr := representation.UnmarshalJSON([]byte(`"json"`))
	if !errors.Is(gotErr, core.ErrJSONContract) || !errors.Is(gotErr, core.ErrExchangeContract) {
		t.Fatalf("nil representation UnmarshalJSON() error = %v, want %v and %v", gotErr, core.ErrJSONContract, core.ErrExchangeContract)
	}
}

func TestOfficialSDKResponseBoundaryHostileConstructionMatrix(t *testing.T) {
	t.Parallel()

	validCases := []officialSDKBoundaryCase{
		{name: "GET selected provider path is admitted", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/storage/", suffix: "/iam"},
		{name: "HEAD selected provider path is admitted", constructor: officialSDKBoundarySelected, method: MethodHead, prefix: "/storage/", suffix: "/iam"},
		{name: "POST selected JSON provider path is admitted", constructor: officialSDKBoundarySelectedJSON, method: MethodPost, prefix: "/v1/accounts/", suffix: ":signBlob"},
		{name: "PUT selected provider path is admitted", constructor: officialSDKBoundarySelected, method: MethodPut, prefix: "/storage/", suffix: "/iam"},
		{name: "PATCH selected provider path is admitted", constructor: officialSDKBoundarySelected, method: MethodPatch, prefix: "/storage/", suffix: "/object"},
		{name: "DELETE selected provider path is admitted", constructor: officialSDKBoundarySelected, method: MethodDelete, prefix: "/storage/", suffix: "/object"},
		{name: "OPTIONS selected provider path is admitted", constructor: officialSDKBoundarySelected, method: MethodOptions, prefix: "/storage/", suffix: "/object"},
		{name: "GET all-path ceiling is admitted", constructor: officialSDKBoundaryAllPaths, method: MethodGet},
		{name: "POST JSON all-path ceiling is admitted", constructor: officialSDKBoundaryAllPathsJSON, method: MethodPost},
		{name: "DELETE all-path ceiling is admitted", constructor: officialSDKBoundaryAllPaths, method: MethodDelete},
	}
	rejectionCases := []officialSDKBoundaryCase{
		{name: "unknown method is refused", constructor: officialSDKBoundarySelected, method: MethodUnknown, prefix: "/storage/", suffix: "/iam", wantErr: core.ErrExchangeContract},
		{name: "future method is refused", constructor: officialSDKBoundarySelected, method: Method(255), prefix: "/storage/", suffix: "/iam", wantErr: core.ErrExchangeContract},
		{name: "empty prefix is refused", constructor: officialSDKBoundarySelected, method: MethodGet, suffix: "/iam", wantErr: core.ErrExchangeContract},
		{name: "empty suffix is refused", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/storage/", wantErr: core.ErrExchangeContract},
		{name: "relative prefix is refused", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "storage/", suffix: "/iam", wantErr: core.ErrExchangeContract},
		{name: "query-bearing prefix is refused", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/storage/?", suffix: "/iam", wantErr: core.ErrExchangeContract},
		{name: "fragment-bearing suffix is refused", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/storage/", suffix: "/iam#", wantErr: core.ErrExchangeContract},
		{name: "suffix above affix ceiling is refused", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/storage/", suffix: strings.Repeat("x", officialSDKPathAffixMaximumBytes+1), wantErr: core.ErrExchangeContract},
	}
	boundaryCases := []officialSDKBoundaryCase{
		{name: "one byte prefix is admitted", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/", suffix: "x"},
		{name: "prefix at affix ceiling is admitted", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/" + strings.Repeat("p", officialSDKPathAffixMaximumBytes-1), suffix: "x"},
		{name: "prefix above affix ceiling is refused", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/" + strings.Repeat("p", officialSDKPathAffixMaximumBytes), suffix: "x", wantErr: core.ErrExchangeContract},
		{name: "one byte suffix is admitted", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/", suffix: "x"},
		{name: "suffix at affix ceiling is admitted", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/", suffix: strings.Repeat("s", officialSDKPathAffixMaximumBytes)},
		{name: "suffix above affix ceiling is refused", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/", suffix: strings.Repeat("s", officialSDKPathAffixMaximumBytes+1), wantErr: core.ErrExchangeContract},
		{name: "colon-prefixed SDK action suffix is admitted", constructor: officialSDKBoundarySelected, method: MethodPost, prefix: "/v1/accounts/", suffix: ":signBlob"},
		{name: "slash-prefixed resource suffix is admitted", constructor: officialSDKBoundarySelected, method: MethodPut, prefix: "/storage/", suffix: "/iam"},
		{name: "query delimiter at prefix edge is refused", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/?", suffix: "x", wantErr: core.ErrExchangeContract},
		{name: "fragment delimiter at prefix edge is refused", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/#", suffix: "x", wantErr: core.ErrExchangeContract},
		{name: "query delimiter at suffix edge is refused", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/", suffix: "?", wantErr: core.ErrExchangeContract},
		{name: "fragment delimiter at suffix edge is refused", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/", suffix: "#", wantErr: core.ErrExchangeContract},
		{name: "method immediately below domain is refused", constructor: officialSDKBoundaryAllPaths, method: MethodUnknown, wantErr: core.ErrExchangeContract},
		{name: "last supported method is admitted", constructor: officialSDKBoundaryAllPaths, method: MethodOptions},
		{name: "method immediately above domain is refused", constructor: officialSDKBoundaryAllPaths, method: MethodOptions + 1, wantErr: core.ErrExchangeContract},
	}

	runOfficialSDKBoundaryCases(t, validCases)
	runOfficialSDKBoundaryCases(t, rejectionCases)
	runOfficialSDKBoundaryCases(t, boundaryCases)
}

func TestOfficialSDKStreamingResponseBoundaryExhaustsSingleByteQueryDomain(t *testing.T) {
	t.Parallel()

	const admittedNames = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_.-"
	const admittedValues = "!\"$%'()*+,-./0123456789:;<>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`abcdefghijklmnopqrstuvwxyz{|}~"
	for raw := range 256 {
		queryByte := byte(raw)
		nameRequest := OfficialSDKStreamingResponseBoundaryRequest{
			Method: MethodGet, StreamQueryName: string([]byte{queryByte}), StreamQueryValue: "media",
			AggregateRepresentation: OfficialSDKResponseRepresentationJSON,
		}
		nameBoundary, nameErr := NewOfficialSDKStreamingResponseBoundary(nameRequest)
		wantName := strings.ContainsRune(admittedNames, rune(queryByte))
		if wantName != (nameErr == nil) {
			t.Fatalf("single-byte query name 0x%02x admission = (%v, %v), want admitted=%t", raw, nameBoundary, nameErr, wantName)
		}

		valueRequest := OfficialSDKStreamingResponseBoundaryRequest{
			Method: MethodGet, StreamQueryName: "alt", StreamQueryValue: string([]byte{queryByte}),
			AggregateRepresentation: OfficialSDKResponseRepresentationJSON,
		}
		valueBoundary, valueErr := NewOfficialSDKStreamingResponseBoundary(valueRequest)
		wantValue := strings.ContainsRune(admittedValues, rune(queryByte))
		if wantValue != (valueErr == nil) {
			t.Fatalf("single-byte query value 0x%02x admission = (%v, %v), want admitted=%t", raw, valueBoundary, valueErr, wantValue)
		}
	}
}

func TestOfficialSDKStreamingResponseBoundaryLengthAndDependencyBoundaries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr    error
		name       string
		queryName  string
		queryValue string
		maximum    core.ByteCount
		method     Method
	}{
		{name: "one-byte query coordinates are admitted", method: MethodGet, queryName: "a", queryValue: "b"},
		{name: "query name at exact ceiling is admitted", method: MethodGet, queryName: strings.Repeat("a", officialSDKQueryNameMaximumBytes), queryValue: "media"},
		{name: "query name one above ceiling is refused", method: MethodGet, queryName: strings.Repeat("a", officialSDKQueryNameMaximumBytes+1), queryValue: "media", wantErr: core.ErrExchangeContract},
		{name: "query value at exact ceiling is admitted", method: MethodGet, queryName: "alt", queryValue: strings.Repeat("m", officialSDKQueryValueMaximumBytes)},
		{name: "query value one above ceiling is refused", method: MethodGet, queryName: "alt", queryValue: strings.Repeat("m", officialSDKQueryValueMaximumBytes+1), wantErr: core.ErrExchangeContract},
		{name: "unknown method is refused", method: MethodUnknown, queryName: "alt", queryValue: "media", wantErr: core.ErrExchangeContract},
		{name: "future method is refused", method: Method(255), queryName: "alt", queryValue: "media", wantErr: core.ErrExchangeContract},
		{name: "missing query name is refused", method: MethodGet, queryValue: "media", wantErr: core.ErrExchangeContract},
		{name: "missing query value is refused", method: MethodGet, queryName: "alt", wantErr: core.ErrExchangeContract},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := NewOfficialSDKStreamingResponseBoundary(OfficialSDKStreamingResponseBoundaryRequest{
				Method: testCase.method, StreamQueryName: testCase.queryName, StreamQueryValue: testCase.queryValue,
				AggregateRepresentation: OfficialSDKResponseRepresentationJSON,
			})
			if testCase.wantErr != nil {
				if got != (OfficialSDKResponseBoundary{}) || !errors.Is(gotErr, testCase.wantErr) {
					t.Fatalf("NewOfficialSDKStreamingResponseBoundary() = (%v, %v), want zero and %v", got, gotErr, testCase.wantErr)
				}
				return
			}
			if gotErr != nil || got.Validate() != nil {
				t.Fatalf("NewOfficialSDKStreamingResponseBoundary() = (%v, %v), want validated boundary and nil", got, gotErr)
			}
		})
	}
}

func runOfficialSDKBoundaryCases(t *testing.T, cases []officialSDKBoundaryCase) {
	t.Helper()
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var got OfficialSDKResponseBoundary
			var gotErr error
			switch testCase.constructor {
			case officialSDKBoundarySelected:
				got, gotErr = NewOfficialSDKResponseBoundary(OfficialSDKResponseBoundaryRequest{
					Method: testCase.method, PathPrefix: testCase.prefix,
					PathSuffix:     testCase.suffix,
					Representation: OfficialSDKResponseRepresentationBinary,
				})
			case officialSDKBoundaryAllPaths:
				got, gotErr = NewOfficialSDKMethodResponseBoundary(OfficialSDKMethodResponseBoundaryRequest{
					Method: testCase.method, Representation: OfficialSDKResponseRepresentationBinary,
				})
			case officialSDKBoundarySelectedJSON:
				got, gotErr = NewOfficialSDKResponseBoundary(OfficialSDKResponseBoundaryRequest{
					Method: testCase.method, PathPrefix: testCase.prefix,
					PathSuffix:     testCase.suffix,
					Representation: OfficialSDKResponseRepresentationJSON,
				})
			case officialSDKBoundaryAllPathsJSON:
				got, gotErr = NewOfficialSDKMethodResponseBoundary(OfficialSDKMethodResponseBoundaryRequest{
					Method: testCase.method, Representation: OfficialSDKResponseRepresentationJSON,
				})
			default:
				t.Fatalf("official SDK boundary constructor = %d, want a declared test execution path", testCase.constructor)
			}
			if testCase.wantErr != nil {
				if !errors.Is(gotErr, testCase.wantErr) || got != (OfficialSDKResponseBoundary{}) {
					t.Fatalf("official SDK boundary = (%v, %v), want zero and %v", got, gotErr, testCase.wantErr)
				}
				return
			}
			if gotErr != nil || got.Validate() != nil {
				t.Fatalf("official SDK boundary = (%v, %v), want validated boundary and nil", got, gotErr)
			}
		})
	}
}
