package cloudflare

import (
	"encoding/json/v2"
	"errors"
	"math"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func FuzzZoneHostClosure(f *testing.F) {
	host, err := ParseZoneHost(zoneAnalyticsTestHost)
	if err != nil {
		f.Fatal(err)
	}
	if err := host.Validate(); err != nil {
		f.Fatal(err)
	}
	f.Add(host.String())
	for _, seed := range []string{"", "Example.ca", "example.ca.", "192.0.2.1", "a-.b.ca", strings.Repeat("a", 64) + ".ca"} {
		f.Add(seed)
	}
	label := regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
	letter := regexp.MustCompile(`[a-z-]`)
	f.Fuzz(func(t *testing.T, source string) {
		got, err := ParseZoneHost(source)
		labels := strings.Split(source, ".")
		valid := len(source) <= 253 && len(labels) >= 2 && letter.MatchString(labels[len(labels)-1])
		for _, l := range labels {
			valid = valid && label.MatchString(l)
		}
		if valid {
			if err != nil || got.String() != source || got.Validate() != nil {
				t.Fatalf("ParseZoneHost(%q)=(%q,%v), want exact host", source, got, err)
			}
			return
		}
		if !errors.Is(err, core.ErrCloudflareContract) || got != (ZoneHost{}) {
			t.Fatalf("ParseZoneHost(%q)=(%q,%v), want zero and contract refusal", source, got, err)
		}
	})
}

// Expected admission is computed before invoking the SDK. The unchanged Core
// limits gate is trusted only for grammar/resources; native json/v2 independently
// decodes the provider envelope. No SDK traffic projector supplies the answer.
func FuzzZoneAnalyticsResponseClosure(f *testing.F) {
	for _, kind := range []int{0, 1, 2, 3, 4, 5, 6, 7} {
		envelope, _, _ := zoneAnalyticsTypedSeed(f)
		switch kind {
		case 0:
		case 1:
			envelope.Data.Viewer.Zones[0].Groups = []zoneAnalyticsGroupWire{}
		case 2:
			zero := uint64(0)
			group := &envelope.Data.Viewer.Zones[0].Groups[0]
			group.Count = &zero
			group.Sum.Visits = &zero
			group.Sum.EdgeResponseBytes = &zero
		case 3:
			envelope.Errors = []graphQLIssueWire{{Message: "partial"}}
		case 4:
			foreign, err := ParseZoneID(strings.Repeat("d", core.CloudflareIdentityCharacters))
			if err != nil {
				f.Fatal(err)
			}
			envelope.Data.Viewer.Zones[0].ZoneTag = foreign.String()
		case 5:
			envelope.Data.Viewer.Zones[0].Groups[0].Count = nil
		case 6:
			below := math.Nextafter(1, 0)
			envelope.Data.Viewer.Zones[0].Groups[0].Avg.SampleInterval = &below
		case 7:
			envelope.Data = nil
		}
		f.Add(zoneAnalyticsCanonicalResponse(f, envelope))
	}
	// Raw seeds are explicitly malformed or hostile protocol representations.
	f.Add([]byte{})
	f.Add([]byte(`{"data":null,"Data":null}`))
	f.Add([]byte(`{"unexpected":1}`))
	f.Add([]byte(`{"data":`))
	f.Fuzz(func(t *testing.T, document []byte) {
		limits := core.DefaultStrictJSONLimits()
		maximum, limitErr := limits.DocumentMaximumBytes.Uint64()
		if limitErr != nil {
			t.Fatal(limitErr)
		}
		server, request := zoneAnalyticsFixture(t, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
			if _, err := w.Write(document); err != nil && uint64(len(document)) <= maximum {
				t.Error(err)
			}
		})
		want, wantIssues, wantErr := zoneAnalyticsResponseOracle(document, limits, server.zone, request)
		got, err := server.ReadHTTP(t.Context(), request, testPolicy())
		if wantErr == nil {
			if err != nil || got != want {
				t.Fatalf("ReadHTTP=(%+v,%v),want exact admitted %+v/nil", got, err, want)
			}
			if got.Validate() != nil {
				t.Fatalf("accepted traffic=%+v,want valid result", got)
			}
			return
		}
		if !errors.Is(err, wantErr) || got != (ZoneHTTPTraffic{}) {
			t.Fatalf("ReadHTTP=(%+v,%v),want zero/%v", got, err, wantErr)
		}
		refusal, hasRefusal := errors.AsType[GraphQLRefusal](err)
		if hasRefusal != (len(wantIssues) > 0) || len(refusal.Issues) != len(wantIssues) {
			t.Fatalf("GraphQL refusal=%+v/%t,want %d source issues", refusal, hasRefusal, len(wantIssues))
		}
		for index, wantIssue := range wantIssues {
			if refusal.Issues[index] != wantIssue {
				t.Fatalf("GraphQL issue=%+v,want exact source %+v", refusal.Issues[index], wantIssue)
			}
		}
	})
}

func zoneAnalyticsTypedSeed(t testing.TB) (zoneAnalyticsEnvelope, ZoneID, ZoneHTTPTrafficRequest) {
	t.Helper()
	zone, err := ParseZoneID(zoneAnalyticsTestZone())
	if err != nil {
		t.Fatal(err)
	}
	host, err := ParseZoneHost(zoneAnalyticsTestHost)
	if err != nil {
		t.Fatal(err)
	}
	second := int64(temporal.NanosecondsPerSecond)
	request := ZoneHTTPTrafficRequest{Host: host, Window: zoneAnalyticsWindow(t, zoneAnalyticsTestStart*second, (zoneAnalyticsTestStart+3600)*second)}
	if err := request.Validate(); err != nil {
		t.Fatal(err)
	}
	traffic := ZoneHTTPTraffic{Zone: zone, Host: request.Host, Window: request.Window, Requests: 3, Visits: 1, EdgeResponseBytes: 7, SampleInterval: 1, Outcome: ZoneAnalyticsObserved}
	if err := traffic.Validate(); err != nil {
		t.Fatal(err)
	}
	interval := float64(traffic.SampleInterval)
	envelope := zoneAnalyticsEnvelope{Data: &zoneAnalyticsDataWire{Viewer: &zoneAnalyticsViewerWire{Zones: []zoneAnalyticsZoneWire{{ZoneTag: zone.String(), Groups: []zoneAnalyticsGroupWire{{Count: &traffic.Requests, Sum: &zoneAnalyticsSumWire{Visits: &traffic.Visits, EdgeResponseBytes: &traffic.EdgeResponseBytes}, Avg: &zoneAnalyticsAvgWire{SampleInterval: &interval}}}}}}}}
	return envelope, zone, request
}

func zoneAnalyticsCanonicalResponse(t testing.TB, envelope zoneAnalyticsEnvelope) []byte {
	t.Helper()
	document, err := core.MarshalCanonicalJSONDocument(envelope)
	if err != nil {
		t.Fatal(err)
	}
	return document
}

func zoneAnalyticsResponseOracle(document []byte, limits core.StrictJSONLimits, zone ZoneID, request ZoneHTTPTrafficRequest) (ZoneHTTPTraffic, []GraphQLIssue, error) {
	var observed zoneAnalyticsEnvelope
	if limits.ValidateDocument(document) != nil || json.Unmarshal(document, &observed, json.RejectUnknownMembers(true)) != nil {
		return ZoneHTTPTraffic{}, nil, core.ErrCloudflareResponse
	}
	if len(observed.Errors) > 0 {
		issues := make([]GraphQLIssue, len(observed.Errors))
		for index, issue := range observed.Errors {
			issues[index] = GraphQLIssue{Message: issue.Message}
		}
		return ZoneHTTPTraffic{}, issues, core.ErrCloudflareResponse
	}
	if observed.Data == nil || observed.Data.Viewer == nil || len(observed.Data.Viewer.Zones) != 1 {
		return ZoneHTTPTraffic{}, nil, core.ErrCloudflareResponse
	}
	row := observed.Data.Viewer.Zones[0]
	if row.ZoneTag != zone.String() {
		return ZoneHTTPTraffic{}, nil, core.ErrCloudflareBinding
	}
	if row.Groups == nil || len(row.Groups) > 1 {
		return ZoneHTTPTraffic{}, nil, core.ErrCloudflareResponse
	}
	want := ZoneHTTPTraffic{Zone: zone, Host: request.Host, Window: request.Window, Outcome: ZoneAnalyticsNoGroups}
	if len(row.Groups) == 0 {
		return want, nil, nil
	}
	group := row.Groups[0]
	if group.Count == nil || group.Sum == nil || group.Avg == nil || group.Sum.Visits == nil || group.Sum.EdgeResponseBytes == nil || group.Avg.SampleInterval == nil {
		return ZoneHTTPTraffic{}, nil, core.ErrCloudflareResponse
	}
	interval := *group.Avg.SampleInterval
	if math.IsNaN(interval) || math.IsInf(interval, 0) || interval < 1 {
		return ZoneHTTPTraffic{}, nil, core.ErrCloudflareResponse
	}
	want.Outcome = ZoneAnalyticsObserved
	want.Requests = *group.Count
	want.Visits = *group.Sum.Visits
	want.EdgeResponseBytes = *group.Sum.EdgeResponseBytes
	want.SampleInterval = ZoneAnalyticsSampleInterval(interval)
	return want, nil, nil
}
