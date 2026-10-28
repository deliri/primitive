package cloudflare

import (
	"bytes"
	"context"
	"encoding"
	"encoding/json"
	"encoding/json/jsontext"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	zoneAnalyticsTestStart = int64(1793222959) // 2026-10-28T21:29:19Z
	zoneAnalyticsTestHost  = "www.example.test"
)

func zoneAnalyticsTestZone() string { return strings.Repeat("c", core.CloudflareIdentityCharacters) }

func zoneAnalyticsWindow(t testing.TB, startNanoseconds, endNanoseconds int64) temporal.Interval {
	t.Helper()
	window, err := temporal.IntervalFromBounds(temporal.IntervalBounds{Start: temporal.InstantFromNanoseconds(startNanoseconds), End: temporal.InstantFromNanoseconds(endNanoseconds)})
	if err != nil {
		t.Fatal(err)
	}
	return window
}

func zoneAnalyticsFixture(t *testing.T, handler http.HandlerFunc) (ZoneAnalyticsServer, ZoneHTTPTrafficRequest) {
	t.Helper()
	token, err := ParseAPIToken([]byte("cloudflare-test-token"))
	if err != nil {
		t.Fatal(err)
	}
	zone, err := ParseZoneID(zoneAnalyticsTestZone())
	if err != nil {
		t.Fatal(err)
	}
	host, err := ParseZoneHost(zoneAnalyticsTestHost)
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewZoneAnalyticsServer(testExchange(t, handler), ZoneAnalyticsServerOptions{Zone: zone, Token: token, ResponseLimits: core.DefaultStrictJSONLimits()})
	if closeErr := token.Close(); err != nil || closeErr != nil {
		t.Fatal(err, closeErr)
	}
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
	})
	second := int64(1_000_000_000)
	return server, ZoneHTTPTrafficRequest{Host: host, Window: zoneAnalyticsWindow(t, zoneAnalyticsTestStart*second, (zoneAnalyticsTestStart+3600)*second)}
}

func zoneAnalyticsDocument(zone, groups string) string {
	return `{"data":{"viewer":{"zones":[{"httpRequestsAdaptiveGroups":` + groups + `,"zoneTag":"` + zone + `"}]}},"errors":null}`
}

// probeGroup is the exact group shape observed from the provider on 2026-10-28.
const probeGroup = `[{"avg":{"sampleInterval":1.0238095238095237},"count":43,"sum":{"edgeResponseBytes":1426319,"visits":9}}]`

func TestZoneAnalyticsReadHTTPProviderLayerTriad(t *testing.T) {
	t.Parallel()
	zone := zoneAnalyticsTestZone()
	for _, tc := range []struct {
		wantErr     error
		name        string
		document    string
		want        ZoneHTTPTraffic
		status      int
		wantIssues  int
		wantOutcome ZoneAnalyticsOutcome
	}{
		{name: "probe shape yields exact sampled aggregate", document: zoneAnalyticsDocument(zone, probeGroup), wantOutcome: ZoneAnalyticsObserved,
			want: ZoneHTTPTraffic{Requests: 43, Visits: 9, EdgeResponseBytes: 1426319, SampleInterval: 1.0238095238095237}},
		{name: "unsampled interval one is exact", document: zoneAnalyticsDocument(zone, `[{"avg":{"sampleInterval":1},"count":1,"sum":{"edgeResponseBytes":0,"visits":0}}]`), wantOutcome: ZoneAnalyticsObserved,
			want: ZoneHTTPTraffic{Requests: 1, SampleInterval: 1}},
		{name: "empty groups are not proven zero", document: zoneAnalyticsDocument(zone, `[]`), wantOutcome: ZoneAnalyticsNoGroups},
		{name: "partial data with errors is refused", wantErr: core.ErrCloudflareResponse, wantIssues: 1,
			document: `{"data":{"viewer":{"zones":[{"httpRequestsAdaptiveGroups":` + probeGroup + `,"zoneTag":"` + zone + `"}]}},"errors":[{"message":"partial","path":["viewer","zones","0"],"extensions":{"code":"x"}}]}`},
		{name: "errors without data are refused", wantErr: core.ErrCloudflareResponse, wantIssues: 2, document: `{"data":null,"errors":[{"message":"authz"},{"message":"limit"}]}`},
		{name: "null data without errors is refused", wantErr: core.ErrCloudflareResponse, document: `{"data":null,"errors":null}`},
		{name: "foreign zone echo is a binding failure", wantErr: core.ErrCloudflareBinding, document: zoneAnalyticsDocument(strings.Repeat("d", core.CloudflareIdentityCharacters), probeGroup)},
		{name: "no zones is refused", wantErr: core.ErrCloudflareResponse, document: `{"data":{"viewer":{"zones":[]}}}`},
		{name: "two zones are ambiguous", wantErr: core.ErrCloudflareResponse,
			document: `{"data":{"viewer":{"zones":[{"httpRequestsAdaptiveGroups":[],"zoneTag":"` + zone + `"},{"httpRequestsAdaptiveGroups":[],"zoneTag":"` + zone + `"}]}}}`},
		{name: "two groups violate the fixed limit", wantErr: core.ErrCloudflareResponse, document: zoneAnalyticsDocument(zone, strings.TrimSuffix(probeGroup, "]")+","+strings.TrimPrefix(probeGroup, "["))},
		{name: "null groups are not an empty result", wantErr: core.ErrCloudflareResponse, document: zoneAnalyticsDocument(zone, `null`)},
		{name: "missing count cannot become zero", wantErr: core.ErrCloudflareResponse, document: zoneAnalyticsDocument(zone, `[{"avg":{"sampleInterval":1},"sum":{"edgeResponseBytes":1,"visits":1}}]`)},
		{name: "missing sample interval is refused", wantErr: core.ErrCloudflareResponse, document: zoneAnalyticsDocument(zone, `[{"avg":{},"count":1,"sum":{"edgeResponseBytes":1,"visits":1}}]`)},
		{name: "sample interval below one is refused", wantErr: core.ErrCloudflareResponse, document: zoneAnalyticsDocument(zone, `[{"avg":{"sampleInterval":0.5},"count":1,"sum":{"edgeResponseBytes":1,"visits":1}}]`)},
		{name: "negative count is refused", wantErr: core.ErrCloudflareResponse, document: zoneAnalyticsDocument(zone, `[{"avg":{"sampleInterval":1},"count":-1,"sum":{"edgeResponseBytes":0,"visits":0}}]`)},
		{name: "unknown member is refused", wantErr: core.ErrCloudflareResponse, document: `{"data":{"viewer":{"zones":[]}},"extensions":{}}`},
		{name: "case folded duplicate is refused", wantErr: core.ErrCloudflareResponse, document: `{"data":null,"Data":null}`},
		{name: "malformed envelope is refused", wantErr: core.ErrCloudflareResponse, document: `{"data":`},
		{name: "refused status cannot carry success data", status: http.StatusBadGateway, wantErr: core.ErrCloudflareResponse, document: zoneAnalyticsDocument(zone, probeGroup)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int64
			server, request := zoneAnalyticsFixture(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if got := r.Header.Get(exchange.StandardHeaderAuthorization.String()); got != "Bearer cloudflare-test-token" {
					t.Errorf("authorization matches owned token = %t, want true", got == "Bearer cloudflare-test-token")
				}
				if r.Method != http.MethodPost || r.Host != core.CloudflareAPIHost || r.URL.Path != zoneAnalyticsGraphQLPath || r.URL.RawQuery != "" {
					t.Errorf("request=(%s,%s,%s,%q), want POST %s", r.Method, r.Host, r.URL.Path, r.URL.RawQuery, zoneAnalyticsGraphQLPath)
				}
				raw, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				got, err := core.DecodeStrictJSONStructure[zoneAnalyticsQueryWire](raw, core.DefaultStrictJSONLimits())
				want := zoneAnalyticsQueryWire{Query: zoneAnalyticsHTTPTrafficText, Variables: zoneAnalyticsVariablesWire{ZoneTag: zone,
					Filter: zoneAnalyticsFilterWire{DatetimeGEQ: "2026-10-28T21:29:19Z", DatetimeLT: "2026-10-28T22:29:19Z", Host: zoneAnalyticsTestHost, RequestSource: "eyeball"}}}
				if err != nil || got != want {
					t.Errorf("query body=(%+v,%v), want fixed query %+v", got, err, want)
				}
				w.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
				if tc.status != 0 {
					w.WriteHeader(tc.status)
				}
				if _, err := w.Write([]byte(tc.document)); err != nil {
					t.Error(err)
				}
			})
			got, err := server.ReadHTTP(t.Context(), request, testPolicy())
			if !errors.Is(err, tc.wantErr) || calls.Load() != 1 {
				t.Fatalf("ReadHTTP=(%+v,%v,%d calls), want %v and one call", got, err, calls.Load(), tc.wantErr)
			}
			if refusal, ok := errors.AsType[GraphQLRefusal](err); len(refusal.Issues) != tc.wantIssues || ok != (tc.wantIssues != 0) {
				t.Fatalf("GraphQL refusal=(%+v,%t), want %d issues", refusal, ok, tc.wantIssues)
			}
			if tc.wantErr != nil {
				if got != (ZoneHTTPTraffic{}) {
					t.Fatalf("refused traffic=%+v, want zero", got)
				}
				return
			}
			want := tc.want
			want.Zone, want.Host, want.Window, want.Outcome = server.zone, request.Host, request.Window, tc.wantOutcome
			if got != want || got.Validate() != nil {
				t.Fatalf("traffic=%+v, want exact %+v", got, want)
			}
		})
	}
}

func TestZoneAnalyticsResponseByteCeiling(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr error
		name    string
		spare   int
	}{
		{name: "one byte over limit yields no traffic", spare: -1, wantErr: core.ErrCloudflareResponse},
		{name: "exact limit retains aggregate"},
		{name: "one byte below limit retains aggregate", spare: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := []byte(zoneAnalyticsDocument(zoneAnalyticsTestZone(), probeGroup))
			server, request := zoneAnalyticsFixture(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
				if _, err := w.Write(body); err != nil {
					t.Error(err)
				}
			})
			var err error
			server.api.limits.DocumentMaximumBytes, err = core.NewByteCount(uint64(len(body) + tc.spare))
			if err != nil {
				t.Fatal(err)
			}
			got, err := server.ReadHTTP(t.Context(), request, testPolicy())
			if !errors.Is(err, tc.wantErr) || err != nil && got != (ZoneHTTPTraffic{}) || err == nil && (got.Validate() != nil || got.Requests != 43) {
				t.Fatalf("bounded read=(%+v,%v), want %v with exact aggregate or zero", got, err, tc.wantErr)
			}
		})
	}
}

func TestZoneAnalyticsRefusedInputNeverReachesProvider(t *testing.T) {
	t.Parallel()
	second := int64(1_000_000_000)
	start := zoneAnalyticsTestStart * second
	for _, tc := range []struct {
		wantErr error
		mutate  func(*testing.T, *ZoneAnalyticsServer, *ZoneHTTPTrafficRequest, *exchange.StreamPolicy)
		name    string
	}{
		{name: "zero host", wantErr: core.ErrCloudflareContract, mutate: func(_ *testing.T, _ *ZoneAnalyticsServer, r *ZoneHTTPTrafficRequest, _ *exchange.StreamPolicy) {
			r.Host = ZoneHost{}
		}},
		{name: "unnormalized host", wantErr: core.ErrCloudflareContract, mutate: func(_ *testing.T, _ *ZoneAnalyticsServer, r *ZoneHTTPTrafficRequest, _ *exchange.StreamPolicy) {
			r.Host = ZoneHost{value: "WWW.example.test"}
		}},
		{name: "zero window", wantErr: core.ErrCloudflareContract, mutate: func(_ *testing.T, _ *ZoneAnalyticsServer, r *ZoneHTTPTrafficRequest, _ *exchange.StreamPolicy) {
			r.Window = temporal.Interval{}
		}},
		{name: "empty window", wantErr: core.ErrCloudflareContract, mutate: func(t *testing.T, _ *ZoneAnalyticsServer, r *ZoneHTTPTrafficRequest, _ *exchange.StreamPolicy) {
			r.Window = zoneAnalyticsWindow(t, start, start)
		}},
		{name: "fractional start second", wantErr: core.ErrCloudflareContract, mutate: func(t *testing.T, _ *ZoneAnalyticsServer, r *ZoneHTTPTrafficRequest, _ *exchange.StreamPolicy) {
			r.Window = zoneAnalyticsWindow(t, start+1, start+second)
		}},
		{name: "fractional end second", wantErr: core.ErrCloudflareContract, mutate: func(t *testing.T, _ *ZoneAnalyticsServer, r *ZoneHTTPTrafficRequest, _ *exchange.StreamPolicy) {
			r.Window = zoneAnalyticsWindow(t, start, start+second+1)
		}},
		{name: "redirect observing policy", wantErr: core.ErrCloudflareBinding, mutate: func(_ *testing.T, _ *ZoneAnalyticsServer, _ *ZoneHTTPTrafficRequest, p *exchange.StreamPolicy) {
			p.Redirect.Mode = exchange.RedirectObserve
		}},
		{name: "closed credential", wantErr: core.ErrCloudflareContract, mutate: func(t *testing.T, s *ZoneAnalyticsServer, _ *ZoneHTTPTrafficRequest, _ *exchange.StreamPolicy) {
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int64
			server, request := zoneAnalyticsFixture(t, func(http.ResponseWriter, *http.Request) { calls.Add(1) })
			policy := testPolicy()
			tc.mutate(t, &server, &request, &policy)
			got, err := server.ReadHTTP(t.Context(), request, policy)
			if !errors.Is(err, tc.wantErr) || got != (ZoneHTTPTraffic{}) || calls.Load() != 0 {
				t.Fatalf("refused ReadHTTP=(%+v,%v,%d calls), want %v, zero and no provider call", got, err, calls.Load(), tc.wantErr)
			}
		})
	}
}

func TestZoneAnalyticsCancellationYieldsNoTraffic(t *testing.T) {
	t.Parallel()
	server, request := zoneAnalyticsFixture(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
		if _, err := w.Write([]byte(zoneAnalyticsDocument(zoneAnalyticsTestZone(), probeGroup))); err != nil {
			t.Error(err)
		}
	})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	got, err := server.ReadHTTP(ctx, request, testPolicy())
	if !errors.Is(err, context.Canceled) || got != (ZoneHTTPTraffic{}) {
		t.Fatalf("cancelled ReadHTTP=(%+v,%v), want zero and context.Canceled", got, err)
	}
}

func TestZoneAnalyticsServerNeverPrintsCredential(t *testing.T) {
	t.Parallel()
	server, _ := zoneAnalyticsFixture(t, func(http.ResponseWriter, *http.Request) {})
	for _, verb := range []string{"%v", "%+v", "%#v"} {
		if got := fmt.Sprintf(verb, server); strings.Contains(got, "cloudflare-test-token") {
			t.Fatalf("formatted server with %s leaked the bearer token", verb)
		}
	}
}

func TestParseZoneHostBoundaries(t *testing.T) {
	t.Parallel()
	label63 := strings.Repeat("a", 63)
	host253 := strings.Join([]string{label63, label63, label63, strings.Repeat("b", 61)}, ".")
	for _, tc := range []struct {
		name  string
		value string
		valid bool
	}{
		{name: "apex", value: "example.ca", valid: true},
		{name: "subdomain with digits and hyphen", value: "a-1.b2.example.test", valid: true},
		{name: "punycode label", value: "xn--bcher-kva.example", valid: true},
		{name: "label at 63 bytes", value: label63 + ".test", valid: true},
		{name: "label at 64 bytes", value: label63 + "a.test"},
		{name: "host at 253 bytes", value: host253, valid: true},
		{name: "host at 254 bytes", value: host253 + "b"},
		{name: "empty"},
		{name: "single label", value: "localhost"},
		{name: "uppercase is not normalized", value: "Example.ca"},
		{name: "trailing dot", value: "example.ca."},
		{name: "empty label", value: "example..ca"},
		{name: "leading hyphen", value: "-a.example.ca"},
		{name: "trailing hyphen", value: "a-.example.ca"},
		{name: "underscore", value: "a_b.example.ca"},
		{name: "wildcard", value: "*.example.ca"},
		{name: "port", value: "example.ca:443"},
		{name: "IPv4 literal", value: "192.0.2.1"},
		{name: "non ASCII", value: "bücher.example"},
		{name: "whitespace", value: " example.ca"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseZoneHost(tc.value)
			if tc.valid {
				if err != nil || got.String() != tc.value || got.Validate() != nil {
					t.Fatalf("ParseZoneHost(%q)=(%q,%v), want exact host", tc.value, got, err)
				}
				return
			}
			if !errors.Is(err, core.ErrCloudflareContract) || got != (ZoneHost{}) {
				t.Fatalf("ParseZoneHost(%q)=(%q,%v), want zero and contract refusal", tc.value, got, err)
			}
		})
	}
}

// An observed numeric zero remains a provider fact; absence is a different
// outcome. Missing load-bearing values and invalid sampling still refuse.
func TestZoneAnalyticsObservedZeroCountLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		kind        int
		wantErr     error
		wantOutcome ZoneAnalyticsOutcome
	}{
		{name: "observed numeric zero retains observed outcome", wantOutcome: ZoneAnalyticsObserved},
		{name: "no groups remain unknown absence", kind: 1, wantOutcome: ZoneAnalyticsNoGroups},
		{name: "missing count cannot be invented as zero", kind: 2, wantErr: core.ErrCloudflareResponse},
		{name: "zero count does not waive invalid sample interval", kind: 3, wantErr: core.ErrCloudflareResponse},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			envelope, _, _ := zoneAnalyticsTypedSeed(t)
			zero := uint64(0)
			group := &envelope.Data.Viewer.Zones[0].Groups[0]
			group.Count = &zero
			group.Sum.Visits = &zero
			group.Sum.EdgeResponseBytes = &zero
			switch tc.kind {
			case 0:
			case 1:
				envelope.Data.Viewer.Zones[0].Groups = []zoneAnalyticsGroupWire{}
			case 2:
				group.Count = nil
			case 3:
				interval := float64(0)
				group.Avg.SampleInterval = &interval
			default:
				t.Fatalf("zero count fixture kind=%d,want enumerated branch", tc.kind)
			}
			document := zoneAnalyticsCanonicalResponse(t, envelope)
			server, request := zoneAnalyticsFixture(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
				if _, err := w.Write(document); err != nil {
					t.Error(err)
				}
			})
			got, err := server.ReadHTTP(t.Context(), request, testPolicy())
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || got != (ZoneHTTPTraffic{}) {
					t.Fatalf("zero count refusal=%+v/%v,want zero/%v", got, err, tc.wantErr)
				}
				return
			}
			want := ZoneHTTPTraffic{Zone: server.zone, Host: request.Host, Window: request.Window, Outcome: tc.wantOutcome}
			if tc.wantOutcome == ZoneAnalyticsObserved {
				want.SampleInterval = 1
			}
			if err != nil || got != want || got.Validate() != nil {
				t.Fatalf("zero count traffic=%+v/%v,want exact %+v/nil", got, err, want)
			}
		})
	}
}

var _ core.OffWireEnum = ZoneAnalyticsObserved

func TestZoneAnalyticsOutcomeCompleteDomain(t *testing.T) {
	t.Parallel()
	for raw := 0; raw < 256; raw++ {
		t.Run(strconv.Itoa(raw), func(t *testing.T) {
			t.Parallel()
			got := ZoneAnalyticsOutcome(raw)
			want := ""
			if raw == 1 {
				want = "observed"
			}
			if raw == 2 {
				want = "no_groups"
			}
			if want != "" {
				if got.Validate() != nil || !got.IsValid() || got.String() != want {
					t.Fatalf("outcome=%d valid=%t label=%q,want admitted %q", raw, got.IsValid(), got.String(), want)
				}
				return
			}
			if !errors.Is(got.Validate(), core.ErrCloudflareResponse) || got.IsValid() || got.String() != "" {
				t.Fatalf("outcome=%d valid=%t label=%q,want typed refusal/false/empty", raw, got.IsValid(), got.String())
			}
		})
	}
	for _, receiver := range []reflect.Type{reflect.TypeFor[ZoneAnalyticsOutcome](), reflect.TypeFor[*ZoneAnalyticsOutcome]()} {
		for _, wire := range []reflect.Type{reflect.TypeFor[json.Marshaler](), reflect.TypeFor[json.Unmarshaler](), reflect.TypeFor[encoding.TextMarshaler](), reflect.TypeFor[encoding.TextUnmarshaler](), reflect.TypeFor[encoding.BinaryMarshaler](), reflect.TypeFor[encoding.BinaryUnmarshaler]()} {
			if receiver.Implements(wire) {
				t.Fatalf("receiver=%v implements %v,want derived outcome without generic wire codec", receiver, wire)
			}
		}
	}
}

func TestZoneAnalyticsOpaqueDiagnosticsLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		kind          int
		wantIssues    int
		wantJSONError bool
	}{
		{name: "heterogeneous path and unknown metadata remain opaque owned copies", wantIssues: 1},
		{name: "absent optional diagnostics retain the message", kind: 1, wantIssues: 1},
		{name: "malformed path JSON cannot become a provider refusal fact", kind: 2, wantJSONError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			envelope, _, _ := zoneAnalyticsTypedSeed(t)
			var path bytes.Buffer
			writer := jsontext.NewEncoder(&path)
			for _, token := range []jsontext.Token{jsontext.BeginArray, jsontext.String("viewer"), jsontext.String("zones"), jsontext.Int(0), jsontext.EndArray} {
				if err := writer.WriteToken(token); err != nil {
					t.Fatal(err)
				}
			}
			locations, err := core.MarshalCanonicalJSONDocument([]struct {
				Line   int `json:"line"`
				Column int `json:"column"`
			}{{Line: 1, Column: 2}})
			if err != nil {
				t.Fatal(err)
			}
			extensions, err := core.MarshalCanonicalJSONDocument(struct {
				Code      string `json:"code"`
				Timestamp string `json:"timestamp"`
				Future    []int  `json:"future"`
			}{Code: "fixture", Timestamp: "2026-10-28T22:29:19Z", Future: []int{1, 2}})
			if err != nil {
				t.Fatal(err)
			}
			issue := graphQLIssueWire{Message: "fixture provider refusal", Path: bytes.Clone(bytes.TrimSpace(path.Bytes())), Locations: locations, Extensions: extensions}
			if tc.kind == 1 {
				issue = graphQLIssueWire{Message: issue.Message}
			}
			envelope.Errors = []graphQLIssueWire{issue}
			document := zoneAnalyticsCanonicalResponse(t, envelope)
			// Standard RawMessage must own its scalar bytes after caller buffer changes.
			input := bytes.Clone(document)
			decoded, decodeErr := core.DecodeStrictJSONStructure[zoneAnalyticsEnvelope](input, core.DefaultStrictJSONLimits())
			if decodeErr != nil {
				t.Fatal(decodeErr)
			}
			for index := range input {
				input[index] ^= 0xff
			}
			gotIssue := decoded.Errors[0]
			if gotIssue.Message != issue.Message || !bytes.Equal(gotIssue.Path, issue.Path) || !bytes.Equal(gotIssue.Locations, issue.Locations) || !bytes.Equal(gotIssue.Extensions, issue.Extensions) {
				t.Fatalf("decoded diagnostics=%+v,want owned exact message/path/location/extensions", gotIssue)
			}
			if tc.kind == 2 {
				replacement := []byte("[\"viewer\",")
				changed := bytes.Replace(document, issue.Path, replacement, 1)
				if bytes.Equal(changed, document) {
					t.Fatalf("malformed diagnostic bytes=%q,want a changed path from %q", changed, document)
				}
				document = changed
			}
			server, request := zoneAnalyticsFixture(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
				if _, err := w.Write(document); err != nil {
					t.Error(err)
				}
			})
			got, readErr := server.ReadHTTP(t.Context(), request, testPolicy())
			refusal, hasRefusal := errors.AsType[GraphQLRefusal](readErr)
			if got != (ZoneHTTPTraffic{}) || !errors.Is(readErr, core.ErrCloudflareResponse) || len(refusal.Issues) != tc.wantIssues || hasRefusal != (tc.wantIssues > 0) {
				t.Fatalf("diagnostic refusal=%+v/%v/%+v,want zero/typed response/%d issues", got, readErr, refusal, tc.wantIssues)
			}
			if tc.wantJSONError && !errors.Is(readErr, core.ErrJSONContract) {
				t.Fatalf("malformed diagnostic error=%v,want retained JSON contract", readErr)
			}
			if tc.wantIssues > 0 && refusal.Issues[0].Message != issue.Message {
				t.Fatalf("provider message=%q,want exact %q", refusal.Issues[0].Message, issue.Message)
			}
		})
	}
}
