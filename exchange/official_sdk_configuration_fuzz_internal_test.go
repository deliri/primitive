package exchange

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// All three public configuration doors are invoked in every callback. Their
// result fields are pinned independently; response execution has separate Go
// HTTP differential fuzz targets in the ingress inventory.
func FuzzOfficialSDKBoundaryCeilingAndStreamingConfiguration(f *testing.F) {
	seed := OfficialSDKResponseBoundaryRequest{Method: MethodGet, Representation: OfficialSDKResponseRepresentationBinary, PathPrefix: "/scope/", PathSuffix: "/item"}
	if err := seed.Validate(); err != nil {
		f.Fatal(err)
	}
	stream := OfficialSDKStreamingResponseBoundaryRequest{Method: seed.Method, AggregateRepresentation: seed.Representation, StreamQueryName: sdkQueryFixtureName, StreamQueryValue: sdkQueryFixtureValue}
	if err := stream.Validate(); err != nil {
		f.Fatal(err)
	}
	// These validated structs are the production configuration representation;
	// they have no JSON or text encoding contract to invent for seed generation.
	f.Add(seed.PathPrefix, seed.PathSuffix, stream.StreamQueryName, stream.StreamQueryValue, uint8(seed.Method), uint8(seed.Representation))
	for _, size := range []int{officialSDKPathAffixMaximumBytes - 1, officialSDKPathAffixMaximumBytes, officialSDKPathAffixMaximumBytes + 1} {
		f.Add("/"+strings.Repeat("p", size-1), strings.Repeat("s", size), stream.StreamQueryName, stream.StreamQueryValue, uint8(seed.Method), uint8(seed.Representation))
	}
	for _, size := range []int{officialSDKQueryNameMaximumBytes - 1, officialSDKQueryNameMaximumBytes, officialSDKQueryNameMaximumBytes + 1} {
		f.Add(seed.PathPrefix, seed.PathSuffix, strings.Repeat("n", size), stream.StreamQueryValue, uint8(seed.Method), uint8(seed.Representation))
	}
	for _, size := range []int{officialSDKQueryValueMaximumBytes - 1, officialSDKQueryValueMaximumBytes, officialSDKQueryValueMaximumBytes + 1} {
		f.Add(seed.PathPrefix, seed.PathSuffix, stream.StreamQueryName, strings.Repeat("v", size), uint8(seed.Method), uint8(seed.Representation))
	}
	for _, method := range []Method{MethodUnknown, MethodGet, MethodPost, MethodPut, MethodDelete, MethodPatch, MethodHead, MethodOptions, Method(255)} {
		f.Add(seed.PathPrefix, seed.PathSuffix, stream.StreamQueryName, stream.StreamQueryValue, uint8(method), uint8(seed.Representation))
	}
	for _, representation := range []OfficialSDKResponseRepresentation{OfficialSDKResponseRepresentationUnknown, OfficialSDKResponseRepresentationBinary, OfficialSDKResponseRepresentationJSON, OfficialSDKResponseRepresentation(255)} {
		f.Add(seed.PathPrefix, seed.PathSuffix, stream.StreamQueryName, stream.StreamQueryValue, uint8(seed.Method), uint8(representation))
	}
	f.Add("", "", "", "", uint8(seed.Method), uint8(seed.Representation))
	f.Add("relative", "suffix?query", "name=value", "value&other", uint8(seed.Method), uint8(seed.Representation))
	f.Fuzz(func(t *testing.T, prefix, suffix, name, value string, methodByte, representationByte uint8) {
		if len(prefix) > officialSDKPathAffixMaximumBytes+1 || len(suffix) > officialSDKPathAffixMaximumBytes+1 || len(name) > officialSDKQueryNameMaximumBytes+1 || len(value) > officialSDKQueryValueMaximumBytes+1 {
			return
		}
		method, representation := Method(methodByte), OfficialSDKResponseRepresentation(representationByte)
		validMethod := method == MethodGet || method == MethodPost || method == MethodPut || method == MethodDelete || method == MethodPatch || method == MethodHead || method == MethodOptions
		validRepresentation := representation == OfficialSDKResponseRepresentationBinary || representation == OfficialSDKResponseRepresentationJSON
		validCommon := validMethod && validRepresentation
		validAffixes := len(prefix) > 0 && len(prefix) <= officialSDKPathAffixMaximumBytes && strings.HasPrefix(prefix, "/") && !strings.ContainsAny(prefix, "?#") && len(suffix) > 0 && len(suffix) <= officialSDKPathAffixMaximumBytes && !strings.ContainsAny(suffix, "?#")
		validName := len(name) > 0 && len(name) <= officialSDKQueryNameMaximumBytes && strings.IndexFunc(name, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' || r == '.')
		}) < 0
		validValue := len(value) > 0 && len(value) <= officialSDKQueryValueMaximumBytes && strings.IndexFunc(value, func(r rune) bool { return r < '!' || r > '~' || r == '&' || r == '=' || r == '#' }) < 0
		cases := []struct {
			name         string
			produce      func() (OfficialSDKResponseBoundary, error)
			wantAccepted bool
			want         OfficialSDKResponseBoundary
		}{
			{name: "selected path", produce: func() (OfficialSDKResponseBoundary, error) {
				return NewOfficialSDKResponseBoundary(OfficialSDKResponseBoundaryRequest{Method: method, Representation: representation, PathPrefix: prefix, PathSuffix: suffix})
			}, wantAccepted: validCommon && validAffixes, want: OfficialSDKResponseBoundary{method: method, representation: representation, prefix: prefix, suffix: suffix, scope: officialSDKResponseScopeSelectedPath, set: true}},
			{name: "method ceiling", produce: func() (OfficialSDKResponseBoundary, error) {
				return NewOfficialSDKMethodResponseBoundary(OfficialSDKMethodResponseBoundaryRequest{Method: method, Representation: representation})
			}, wantAccepted: validCommon, want: OfficialSDKResponseBoundary{method: method, representation: representation, scope: officialSDKResponseScopeAllPaths, set: true}},
			{name: "stream query", produce: func() (OfficialSDKResponseBoundary, error) {
				return NewOfficialSDKStreamingResponseBoundary(OfficialSDKStreamingResponseBoundaryRequest{Method: method, AggregateRepresentation: representation, StreamQueryName: name, StreamQueryValue: value})
			}, wantAccepted: validCommon && validName && validValue, want: OfficialSDKResponseBoundary{method: method, representation: representation, streamQueryName: name, streamQueryValue: value, scope: officialSDKResponseScopeAllPaths, streamSuccess: true, set: true}},
		}
		for _, tc := range cases {
			got, err := tc.produce()
			if tc.wantAccepted {
				if err != nil || got != tc.want || got.Validate() != nil {
					t.Fatalf("%s configuration = (%+v,%v), want exact admitted %+v", tc.name, got, err, tc.want)
				}
			} else if !errors.Is(err, core.ErrExchangeContract) || got != (OfficialSDKResponseBoundary{}) {
				t.Fatalf("%s refusal = (%+v,%v), want zero and typed refusal", tc.name, got, err)
			}
			transport, transportErr := NewStandardOfficialSDKResponseTransport(got)
			if tc.wantAccepted {
				owned, ok := transport.(officialSDKResponseTransport)
				if transportErr != nil || !ok || owned.base != http.DefaultTransport || owned.boundary != tc.want {
					t.Fatalf("%s standard transport = (%T,%v), want exact Go default transport and admitted boundary", tc.name, transport, transportErr)
				}
			} else if !errors.Is(transportErr, core.ErrExchangeContract) || transport != nil {
				t.Fatalf("%s standard transport admitted refused boundary: (%T,%v)", tc.name, transport, transportErr)
			}
		}
	})
}
