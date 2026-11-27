package searchconsole

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	propertyFixture = "sc-domain:fixture.example"
	pageFixture     = "https://fixture.example/services?one=1&two=2"
	queryFixture    = "fixture café"
	bearerFixture   = "search-console-fixture-bearer"
)

type fixtureSource struct {
	calls atomic.Int32
	last  core.OAuthClientSecret
}

func (f *fixtureSource) BorrowReadAccess(ctx context.Context) (core.OAuthClientSecret, error) {
	if err := ctx.Err(); err != nil {
		return core.OAuthClientSecret{}, err
	}
	f.calls.Add(1)
	token, err := core.NewOAuthClientSecret([]byte(bearerFixture))
	f.last = token
	return token, err
}

func fixtureRequest(t testing.TB) QueryRequest {
	t.Helper()
	property, pErr := ParseProperty(propertyFixture)
	first, startErr := temporal.ParseCivilDate("2026-10-28")
	last, endErr := temporal.ParseCivilDate("2026-11-04")
	timeout, tErr := temporal.DurationFromSeconds(5)
	if err := errors.Join(pErr, startErr, endErr, tErr); err != nil {
		t.Fatal(err)
	}
	return QueryRequest{Property: property, Dates: DateRange{Start: first, End: last}, Type: SearchTypeWeb,
		Aggregation: AggregationAuto, State: DataFinal, Page: PageWindow{Limit: 2},
		Policy: exchange.OperationPolicy{OperationTimeout: timeout, AttemptTimeout: timeout,
			Retry: exchange.RetryPolicy{MaximumAttempts: 1}, Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject}}}
}

func providerNetwork(t testing.TB, handler http.HandlerFunc, metadata *httptest.Server) exchange.Client {
	t.Helper()
	provider := httptest.NewTLSServer(handler)
	t.Cleanup(provider.Close)
	transport := provider.Client().Transport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.TLSClientConfig.ServerName = "example.com"
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		var target string
		switch address {
		case "www.googleapis.com:443", "iamcredentials.googleapis.com:443", "oauth2.googleapis.com:443":
			target = provider.Listener.Addr().String()
		case "metadata.google.internal:80":
			if metadata == nil {
				return nil, core.ErrSearchConsoleContract
			}
			target = metadata.Listener.Addr().String()
		default:
			return nil, core.ErrSearchConsoleContract
		}
		return (&net.Dialer{}).DialContext(ctx, network, target)
	}
	t.Cleanup(transport.CloseIdleConnections)
	client, err := exchange.NewClient(&http.Client{Transport: transport})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func fixtureJSON[T any](t testing.TB, value T) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func writeJSON(t testing.TB, w http.ResponseWriter, status int, raw []byte) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(raw); err != nil {
		t.Error(err)
	}
}

func rowFixture(keys ...string) rowWire {
	clicks, impressions, rate, position := Clicks(2), Impressions(8), ClickRate(.25), Position(12)
	return rowWire{Keys: keys, Clicks: &clicks, Impressions: &impressions, Rate: &rate, Position: &position}
}

func TestSearchConsoleLoopbackDimensionProjection(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		dims      Dimensions
		keys      []string
		filtered  bool
		aggregate Aggregation
	}{
		{name: "ungrouped property totals", aggregate: AggregationProperty},
		{name: "date only across Pacific fall clock change", dims: Dimensions{First: DimensionDate}, keys: []string{"2026-11-01"}, aggregate: AggregationProperty},
		{name: "page only", dims: Dimensions{First: DimensionPage}, keys: []string{pageFixture}, aggregate: AggregationPage},
		{name: "query only", dims: Dimensions{First: DimensionQuery}, keys: []string{queryFixture}, aggregate: AggregationProperty},
		{name: "page before query with both exact filters", dims: Dimensions{DimensionPage, DimensionQuery}, keys: []string{pageFixture, queryFixture}, filtered: true, aggregate: AggregationPage},
		{name: "query before page", dims: Dimensions{DimensionQuery, DimensionPage}, keys: []string{queryFixture, pageFixture}, aggregate: AggregationPage},
		{name: "ungrouped filtered page coverage", filtered: true, aggregate: AggregationPage},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := fixtureRequest(t)
			request.Dimensions = tc.dims
			if tc.filtered {
				request.Filters.Page, _ = ParsePageURL(pageFixture)
				if request.Dimensions.count() > 0 {
					request.Filters.Query, _ = ParseQueryText(queryFixture)
				}
			}
			source := &fixtureSource{}
			var calls atomic.Int32
			response := fixtureJSON(t, pageWire{Rows: []rowWire{rowFixture(tc.keys...)}, Aggregation: tc.aggregate.String()})
			network := providerNetwork(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Host != "www.googleapis.com" || r.Method != http.MethodPost ||
					r.URL.Path != "/webmasters/v3/sites/"+propertyFixture+"/searchAnalytics/query" ||
					r.Header.Get("Authorization") != "Bearer "+bearerFixture ||
					r.Header.Get("Content-Type") != "application/json" || r.URL.RawQuery != "" {
					t.Errorf("request target = %s %s, want bounded authorized Search Analytics POST", r.Method, r.URL)
				}
				var got queryWire
				data, err := io.ReadAll(io.LimitReader(r.Body, 16*1024))
				if err != nil || json.Unmarshal(data, &got) != nil {
					t.Errorf("wire query bytes = %d/%v, want complete typed body", len(data), err)
				}
				var wantDimensions []string
				for _, dimension := range []Dimension{tc.dims.First, tc.dims.Second} {
					if dimension.IsValid() {
						wantDimensions = append(wantDimensions, dimension.String())
					}
				}
				if !slices.Equal(got.Dimensions, wantDimensions) || got.StartDate != "2026-10-28" || got.EndDate != "2026-11-04" ||
					got.Type != "web" || got.Aggregation != "auto" || got.State != "final" || got.Limit != 2 || got.Offset != 0 {
					t.Errorf("query body = %+v, want precise date, page, enum and ordered dimension contract", got)
				}
				if tc.filtered {
					want := []filterWire{{Dimension: "page", Operator: "equals", Expression: pageFixture}}
					if tc.dims.count() > 0 {
						want = append(want, filterWire{Dimension: "query", Operator: "equals", Expression: queryFixture})
					}
					if len(got.FilterGroups) != 1 || got.FilterGroups[0].Type != "and" || !slices.Equal(got.FilterGroups[0].Filters, want) {
						t.Errorf("native filters = %+v, want %+v", got.FilterGroups, want)
					}
				} else if len(got.FilterGroups) != 0 {
					t.Errorf("unfiltered query groups = %+v, want none", got.FilterGroups)
				}
				writeJSON(t, w, http.StatusOK, response)
			}, nil)
			server, err := NewServer(network, source)
			if err != nil {
				t.Fatal(err)
			}
			page, err := server.Query().Execute(t.Context(), request)
			if err != nil || len(page.Rows) != 1 || page.Aggregation != tc.aggregate || page.NextOffset != 1 || calls.Load() != 1 || source.calls.Load() != 1 {
				t.Fatalf("projected page = %+v/%v, provider calls %d, want one row, next index 1 and one call", page, err, calls.Load())
			}
			row := page.Rows[0]
			var date temporal.CivilDate
			var address PageURL
			var query QueryText
			if tc.dims.has(DimensionDate) {
				date, _ = temporal.ParseCivilDate("2026-11-01")
			}
			if tc.dims.has(DimensionPage) {
				address, _ = ParsePageURL(pageFixture)
			}
			if tc.dims.has(DimensionQuery) {
				query, _ = ParseQueryText(queryFixture)
			}
			if row != (Row{Date: date, Page: address, Query: query, Clicks: 2, Impressions: 8, Rate: .25, Position: 12}) {
				t.Fatalf("row = %+v, want exact ordered key projection and reported metrics", row)
			}
			if source.last.Validate() == nil {
				t.Fatal("borrowed bearer remains live, want closed after query")
			}
		})
	}
}

func TestSearchConsoleLoopbackRefusalsAndSites(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		site    bool
		status  int
		raw     []byte
		wantErr error
	}{
		{name: "restricted property proof", site: true, status: 200, raw: fixtureJSON(t, siteWire{Property: propertyFixture, Permission: "siteRestrictedUser"})},
		{name: "unverified grant", site: true, status: 200, raw: fixtureJSON(t, siteWire{Property: propertyFixture, Permission: "siteUnverifiedUser"}), wantErr: core.ErrSearchConsoleProperty},
		{name: "foreign property lookup", site: true, status: 200, raw: fixtureJSON(t, siteWire{Property: "sc-domain:other.example", Permission: "siteOwner"}), wantErr: core.ErrSearchConsoleResponse},
		{name: "HTTP 429 with no JSON error", status: 429, raw: []byte("quota"), wantErr: core.ErrSearchConsoleQuota},
		{name: "quota 403 typed by reason", status: 403, raw: []byte(`{"error":{"code":403,"errors":[{"domain":"global","reason":"quotaExceeded"}]}}`), wantErr: core.ErrSearchConsoleQuota},
		{name: "forbidden property", site: true, status: 403, raw: []byte(`{"error":{"code":403,"errors":[{"reason":"forbidden"}]}}`), wantErr: core.ErrSearchConsoleProperty},
		{name: "missing property", site: true, status: 404, raw: []byte(`{"error":{"code":404,"errors":[{"reason":"notFound"}]}}`), wantErr: core.ErrSearchConsoleProperty},
		{name: "insufficient permissions remains ambiguous", status: 403, raw: []byte(`{"error":{"code":403,"errors":[{"reason":"insufficientPermissions"}]}}`), wantErr: core.ErrSearchConsoleResponse},
		{name: "quota body status contradicts HTTP", status: 403, raw: []byte(`{"error":{"code":429,"errors":[{"reason":"quotaExceeded"}]}}`), wantErr: core.ErrSearchConsoleResponse},
		{name: "quota body exceeds diagnostic bound", status: 403, raw: bytes.Repeat([]byte{' '}, failureMaximum+1), wantErr: core.ErrExchangeBodyLimit},
		{name: "query JSON truncated", status: 200, raw: []byte(`{"rows":[`), wantErr: core.ErrJSONContract},
		{name: "query overflow by one byte", status: 200, raw: bytes.Repeat([]byte{' '}, responseMaximum+1), wantErr: core.ErrExchangeBodyLimit},
		{name: "sites overflow by one byte", site: true, status: 200, raw: bytes.Repeat([]byte{' '}, siteMaximum+1), wantErr: core.ErrExchangeBodyLimit},
		{name: "empty query returns no evidence", status: 200, raw: []byte("{}")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := fixtureRequest(t)
			var calls atomic.Int32
			network := providerNetwork(t, func(w http.ResponseWriter, req *http.Request) {
				calls.Add(1)
				if tc.site && (req.Method != http.MethodGet || req.URL.Path != "/webmasters/v3/sites/"+propertyFixture || req.ContentLength > 0) {
					t.Errorf("Sites resource call = %s %s/%d, want property GET with no body", req.Method, req.URL, req.ContentLength)
				}
				writeJSON(t, w, tc.status, tc.raw)
			}, nil)
			c, err := NewClient(network, &fixtureSource{})
			if err != nil {
				t.Fatal(err)
			}
			if tc.site {
				got, err := c.GetSite(t.Context(), SiteRequest{Property: r.Property, Policy: r.Policy})
				if !errors.Is(err, tc.wantErr) || (tc.wantErr != nil && got != (Site{})) ||
					(tc.wantErr == nil && got != (Site{Property: r.Property, Permission: PermissionRestricted})) {
					t.Fatalf("Sites result = %+v/%v, want exact Restricted property or zero/%v", got, err, tc.wantErr)
				}
			} else {
				page, err := c.Query(t.Context(), r)
				if !errors.Is(err, tc.wantErr) || len(page.Rows) != 0 || page.NextOffset != 0 ||
					(tc.wantErr != nil && !page.isZero()) {
					t.Fatalf("query result = %+v/%v, want no rows, no cursor and %v", page, err, tc.wantErr)
				}
			}
			if calls.Load() != 1 {
				t.Errorf("provider calls = %d, want one single-attempt call", calls.Load())
			}
		})
	}
}

func (p QueryPage) isZero() bool {
	return p.Rows == nil && p.Dates == (DateRange{}) && p.Dimensions == (Dimensions{}) &&
		p.FirstIncomplete == (temporal.CivilDate{}) && p.State == DataInvalid && p.Aggregation == AggregationInvalid && p.NextOffset == 0
}

func TestSearchConsoleCancellationAndInvalidRequestsHaveNoEffects(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	network := providerNetwork(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}, nil)
	source := &fixtureSource{}
	c, err := NewClient(network, source)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	r := fixtureRequest(t)
	if got, err := c.Query(ctx, r); !errors.Is(err, context.Canceled) || !got.isZero() {
		t.Fatalf("pre-cancelled Query = %+v/%v, want zero/cancelled", got, err)
	}
	r.Policy.Redirect.Mode = exchange.RedirectSameOrigin
	if got, err := c.Query(t.Context(), r); !errors.Is(err, core.ErrSearchConsoleContract) || !got.isZero() {
		t.Fatalf("redirect-capable query = %+v/%v, want zero/contract", got, err)
	}
	if calls.Load() != 0 || source.calls.Load() != 0 {
		t.Fatalf("provider calls = %d, bearer requests = %d, want no effects", calls.Load(), source.calls.Load())
	}
}
