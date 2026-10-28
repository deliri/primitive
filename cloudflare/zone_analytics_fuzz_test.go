package cloudflare

import (
	"encoding/json/v2"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzZoneHostClosure(f *testing.F) {
	for _, seed := range []string{"example.ca", "www.example.test", "", "Example.ca", "example.ca.", "192.0.2.1", "a-.b.ca", strings.Repeat("a", 64) + ".ca"} {
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

// The oracle decodes the same document independently and states the complete
// acceptance rule: no GraphQL errors, exactly one echoed zone, and either an
// empty group list or one complete group with positive count and interval >= 1.
func FuzzZoneAnalyticsResponseClosure(f *testing.F) {
	zone := zoneAnalyticsTestZone()
	for _, seed := range []string{
		zoneAnalyticsDocument(zone, probeGroup),
		zoneAnalyticsDocument(zone, `[]`),
		zoneAnalyticsDocument(zone, `null`),
		zoneAnalyticsDocument(strings.Repeat("d", core.CloudflareIdentityCharacters), probeGroup),
		`{"data":{"viewer":{"zones":[{"httpRequestsAdaptiveGroups":[],"zoneTag":"` + zone + `"}]}},"errors":[{"message":"partial"}]}`,
		`{"data":null,"errors":null}`,
		`{"data":null,"Data":null}`,
		"",
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, document []byte) {
		// The SDK owns the aggregate response bound; cap the oracle's fixture input.
		if len(document) > 65536 {
			return
		}
		server, request := zoneAnalyticsFixture(t, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
			if _, err := w.Write(document); err != nil {
				t.Error(err)
			}
		})
		got, err := server.ReadHTTP(t.Context(), request, testPolicy())
		var observed zoneAnalyticsEnvelope
		decodeErr := json.Unmarshal(document, &observed, json.RejectUnknownMembers(true))
		if err != nil {
			if got != (ZoneHTTPTraffic{}) || !errors.Is(err, core.ErrCloudflareResponse) && !errors.Is(err, core.ErrCloudflareBinding) {
				t.Fatalf("refused=(%+v,%v), want zero and response/binding refusal", got, err)
			}
			if refusal, ok := errors.AsType[GraphQLRefusal](err); ok && (decodeErr != nil || len(refusal.Issues) != len(observed.Errors) || len(refusal.Issues) == 0) {
				t.Fatalf("GraphQL refusal=%+v, want exactly the document's %d errors", refusal, len(observed.Errors))
			}
			return
		}
		if decodeErr != nil || len(observed.Errors) != 0 || observed.Data == nil || observed.Data.Viewer == nil || len(observed.Data.Viewer.Zones) != 1 {
			t.Fatalf("accepted %q without one error-free zone", document)
		}
		echoed := observed.Data.Viewer.Zones[0]
		want := ZoneHTTPTraffic{Zone: server.zone, Host: request.Host, Window: request.Window, Outcome: ZoneAnalyticsNoGroups}
		switch {
		case echoed.ZoneTag != zone || echoed.Groups == nil || len(echoed.Groups) > 1:
			t.Fatalf("accepted foreign zone or ambiguous groups in %q", document)
		case len(echoed.Groups) == 1:
			g := echoed.Groups[0]
			if g.Count == nil || g.Sum == nil || g.Avg == nil || g.Sum.Visits == nil || g.Sum.EdgeResponseBytes == nil || g.Avg.SampleInterval == nil || *g.Count == 0 || !(*g.Avg.SampleInterval >= 1) {
				t.Fatalf("accepted incomplete or unsampled-invalid group in %q", document)
			}
			want.Outcome, want.Requests, want.Visits, want.EdgeResponseBytes = ZoneAnalyticsObserved, *g.Count, *g.Sum.Visits, *g.Sum.EdgeResponseBytes
			want.SampleInterval = ZoneAnalyticsSampleInterval(*g.Avg.SampleInterval)
		}
		if got != want {
			t.Fatalf("traffic=%+v, want %+v", got, want)
		}
	})
}
