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
	name        string
	constructor officialSDKBoundaryConstructor
	method      Method
	prefix      string
	suffix      string
	limit       uint64
	wantErr     error
}

func TestOfficialSDKResponseTransportConstructionRefusesEveryUnsetDependency(t *testing.T) {
	t.Parallel()

	limit, limitErr := core.NewByteCount(1)
	if limitErr != nil {
		t.Fatalf("core.NewByteCount() error = %v, want nil", limitErr)
	}
	boundary, boundaryErr := NewOfficialSDKResponseCeiling(OfficialSDKResponseCeilingRequest{
		Method: MethodGet, Representation: OfficialSDKResponseRepresentationBinary,
		MaximumBytes: limit,
	})
	if boundaryErr != nil {
		t.Fatalf("NewOfficialSDKResponseCeiling() error = %v, want nil", boundaryErr)
	}
	cases := []struct {
		name    string
		request OfficialSDKResponseTransportRequest
		wantErr error
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

	for raw := 0; raw <= 255; raw++ {
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
		{name: "GET selected provider path is admitted", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/storage/", suffix: "/iam", limit: 1024},
		{name: "HEAD selected provider path is admitted", constructor: officialSDKBoundarySelected, method: MethodHead, prefix: "/storage/", suffix: "/iam", limit: 1024},
		{name: "POST selected JSON provider path is admitted", constructor: officialSDKBoundarySelectedJSON, method: MethodPost, prefix: "/v1/accounts/", suffix: ":signBlob", limit: 4096},
		{name: "PUT selected provider path is admitted", constructor: officialSDKBoundarySelected, method: MethodPut, prefix: "/storage/", suffix: "/iam", limit: 1024},
		{name: "PATCH selected provider path is admitted", constructor: officialSDKBoundarySelected, method: MethodPatch, prefix: "/storage/", suffix: "/object", limit: 1024},
		{name: "DELETE selected provider path is admitted", constructor: officialSDKBoundarySelected, method: MethodDelete, prefix: "/storage/", suffix: "/object", limit: 1024},
		{name: "OPTIONS selected provider path is admitted", constructor: officialSDKBoundarySelected, method: MethodOptions, prefix: "/storage/", suffix: "/object", limit: 1024},
		{name: "GET all-path ceiling is admitted", constructor: officialSDKBoundaryAllPaths, method: MethodGet, limit: 1024},
		{name: "POST JSON all-path ceiling is admitted", constructor: officialSDKBoundaryAllPathsJSON, method: MethodPost, limit: 4096},
		{name: "DELETE all-path ceiling is admitted", constructor: officialSDKBoundaryAllPaths, method: MethodDelete, limit: OfficialSDKResponseMaximumBytes},
	}
	rejectionCases := []officialSDKBoundaryCase{
		{name: "unknown method is refused", constructor: officialSDKBoundarySelected, method: MethodUnknown, prefix: "/storage/", suffix: "/iam", limit: 1024, wantErr: core.ErrExchangeContract},
		{name: "future method is refused", constructor: officialSDKBoundarySelected, method: Method(255), prefix: "/storage/", suffix: "/iam", limit: 1024, wantErr: core.ErrExchangeContract},
		{name: "zero limit is refused", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/storage/", suffix: "/iam", wantErr: core.ErrExchangeContract},
		{name: "limit above aggregate ceiling is refused", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/storage/", suffix: "/iam", limit: OfficialSDKResponseMaximumBytes + 1, wantErr: core.ErrExchangeContract},
		{name: "empty prefix is refused", constructor: officialSDKBoundarySelected, method: MethodGet, suffix: "/iam", limit: 1024, wantErr: core.ErrExchangeContract},
		{name: "empty suffix is refused", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/storage/", limit: 1024, wantErr: core.ErrExchangeContract},
		{name: "relative prefix is refused", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "storage/", suffix: "/iam", limit: 1024, wantErr: core.ErrExchangeContract},
		{name: "query-bearing prefix is refused", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/storage/?", suffix: "/iam", limit: 1024, wantErr: core.ErrExchangeContract},
		{name: "fragment-bearing suffix is refused", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/storage/", suffix: "/iam#", limit: 1024, wantErr: core.ErrExchangeContract},
		{name: "suffix above affix ceiling is refused", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/storage/", suffix: strings.Repeat("x", officialSDKPathAffixMaximumBytes+1), limit: 1024, wantErr: core.ErrExchangeContract},
	}
	boundaryCases := []officialSDKBoundaryCase{
		{name: "one byte limit is admitted", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/", suffix: "/", limit: 1},
		{name: "one below aggregate ceiling is admitted", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/", suffix: "/", limit: OfficialSDKResponseMaximumBytes - 1},
		{name: "aggregate ceiling is admitted", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/", suffix: "/", limit: OfficialSDKResponseMaximumBytes},
		{name: "one above aggregate ceiling is refused", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/", suffix: "/", limit: OfficialSDKResponseMaximumBytes + 1, wantErr: core.ErrExchangeContract},
		{name: "one byte prefix is admitted", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/", suffix: "x", limit: 1},
		{name: "prefix at affix ceiling is admitted", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/" + strings.Repeat("p", officialSDKPathAffixMaximumBytes-1), suffix: "x", limit: 1},
		{name: "prefix above affix ceiling is refused", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/" + strings.Repeat("p", officialSDKPathAffixMaximumBytes), suffix: "x", limit: 1, wantErr: core.ErrExchangeContract},
		{name: "one byte suffix is admitted", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/", suffix: "x", limit: 1},
		{name: "suffix at affix ceiling is admitted", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/", suffix: strings.Repeat("s", officialSDKPathAffixMaximumBytes), limit: 1},
		{name: "suffix above affix ceiling is refused", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/", suffix: strings.Repeat("s", officialSDKPathAffixMaximumBytes+1), limit: 1, wantErr: core.ErrExchangeContract},
		{name: "colon-prefixed SDK action suffix is admitted", constructor: officialSDKBoundarySelected, method: MethodPost, prefix: "/v1/accounts/", suffix: ":signBlob", limit: 1},
		{name: "slash-prefixed resource suffix is admitted", constructor: officialSDKBoundarySelected, method: MethodPut, prefix: "/storage/", suffix: "/iam", limit: 1},
		{name: "query delimiter at prefix edge is refused", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/?", suffix: "x", limit: 1, wantErr: core.ErrExchangeContract},
		{name: "fragment delimiter at prefix edge is refused", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/#", suffix: "x", limit: 1, wantErr: core.ErrExchangeContract},
		{name: "query delimiter at suffix edge is refused", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/", suffix: "?", limit: 1, wantErr: core.ErrExchangeContract},
		{name: "fragment delimiter at suffix edge is refused", constructor: officialSDKBoundarySelected, method: MethodGet, prefix: "/", suffix: "#", limit: 1, wantErr: core.ErrExchangeContract},
		{name: "method immediately below domain is refused", constructor: officialSDKBoundaryAllPaths, method: MethodUnknown, limit: 1, wantErr: core.ErrExchangeContract},
		{name: "last supported method is admitted", constructor: officialSDKBoundaryAllPaths, method: MethodOptions, limit: 1},
		{name: "method immediately above domain is refused", constructor: officialSDKBoundaryAllPaths, method: MethodOptions + 1, limit: 1, wantErr: core.ErrExchangeContract},
		{name: "all-path ceiling at one byte is admitted", constructor: officialSDKBoundaryAllPaths, method: MethodGet, limit: 1},
	}

	runOfficialSDKBoundaryCases(t, validCases)
	runOfficialSDKBoundaryCases(t, rejectionCases)
	runOfficialSDKBoundaryCases(t, boundaryCases)
}

func runOfficialSDKBoundaryCases(t *testing.T, cases []officialSDKBoundaryCase) {
	t.Helper()
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			limit := core.ByteCount{}
			var limitErr error
			if testCase.limit != 0 {
				limit, limitErr = core.NewByteCount(testCase.limit)
			}
			var got OfficialSDKResponseBoundary
			var gotErr error
			if limitErr != nil {
				gotErr = limitErr
			} else {
				switch testCase.constructor {
				case officialSDKBoundarySelected:
					got, gotErr = NewOfficialSDKResponseBoundary(OfficialSDKResponseBoundaryRequest{
						Method: testCase.method, PathPrefix: testCase.prefix,
						PathSuffix:     testCase.suffix,
						Representation: OfficialSDKResponseRepresentationBinary,
						MaximumBytes:   limit,
					})
				case officialSDKBoundaryAllPaths:
					got, gotErr = NewOfficialSDKResponseCeiling(OfficialSDKResponseCeilingRequest{
						Method: testCase.method, Representation: OfficialSDKResponseRepresentationBinary,
						MaximumBytes: limit,
					})
				case officialSDKBoundarySelectedJSON:
					got, gotErr = NewOfficialSDKResponseBoundary(OfficialSDKResponseBoundaryRequest{
						Method: testCase.method, PathPrefix: testCase.prefix,
						PathSuffix:     testCase.suffix,
						Representation: OfficialSDKResponseRepresentationJSON,
						MaximumBytes:   limit,
					})
				case officialSDKBoundaryAllPathsJSON:
					got, gotErr = NewOfficialSDKResponseCeiling(OfficialSDKResponseCeilingRequest{
						Method: testCase.method, Representation: OfficialSDKResponseRepresentationJSON,
						MaximumBytes: limit,
					})
				default:
					t.Fatalf("official SDK boundary constructor = %d, want a declared test execution path", testCase.constructor)
				}
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
