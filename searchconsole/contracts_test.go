package searchconsole

import (
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestSearchConsolePropertyAdmission(t *testing.T) {
	t.Parallel()
	label := strings.Repeat("a", 63)
	domainMaximum := label + "." + label + "." + label + "." + strings.Repeat("a", 61)
	for _, tc := range []struct {
		name     string
		property string
		wantErr  error
	}{
		{name: "Deliri-shaped domain property", property: "sc-domain:fixture.example"},
		{name: "three-label property", property: "sc-domain:branch.fixture.example"},
		{name: "hyphen inside a label", property: "sc-domain:domain-example.com"},
		{name: "punycoded domain", property: "sc-domain:xn--fixtre-6wa.example"},
		{name: "label exactly at maximum", property: "sc-domain:" + label + ".example"},
		{name: "one over label maximum", property: "sc-domain:" + label + "a.example", wantErr: core.ErrSearchConsoleContract},
		{name: "one below DNS maximum", property: "sc-domain:" + domainMaximum[:len(domainMaximum)-1]},
		{name: "exact DNS maximum", property: "sc-domain:" + domainMaximum},
		{name: "one above DNS maximum", property: "sc-domain:" + domainMaximum + "a", wantErr: core.ErrSearchConsoleContract},
		{name: "bare DNS name does not name a property", property: "fixture.example", wantErr: core.ErrSearchConsoleContract},
		{name: "URL-prefix property is outside domain SDK", property: "https://fixture.example/", wantErr: core.ErrSearchConsoleContract},
		{name: "path cannot alter target route", property: "sc-domain:fixture.example/path", wantErr: core.ErrSearchConsoleContract},
		{name: "raw IPv4 address is not a domain", property: "sc-domain:127.0.0.1", wantErr: core.ErrSearchConsoleContract},
		{name: "single-label hostname is not a domain property", property: "sc-domain:localhost", wantErr: core.ErrSearchConsoleContract},
		{name: "dot-separated empty label", property: "sc-domain:fixture..example", wantErr: core.ErrSearchConsoleContract},
		{name: "leading label hyphen", property: "sc-domain:-fixture.example", wantErr: core.ErrSearchConsoleContract},
		{name: "trailing label hyphen", property: "sc-domain:fixture-.example", wantErr: core.ErrSearchConsoleContract},
		{name: "empty identifier", wantErr: core.ErrSearchConsoleContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseProperty(tc.property)
			if !errors.Is(err, tc.wantErr) || tc.wantErr != nil && got != (Property{}) ||
				tc.wantErr == nil && (got.String() != tc.property || got.Validate() != nil) {
				t.Fatalf("property = %+v/%v, want exact %q or zero/%v", got, err, tc.property, tc.wantErr)
			}
		})
	}
}

func TestSearchConsoleRequestAdmission(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		change  func(*QueryRequest)
		wantErr error
	}{
		{name: "final dates and default grouping", change: func(_ *QueryRequest) {}},
		{name: "explicit by page without page context", change: func(r *QueryRequest) { r.Aggregation = AggregationPage }},
		{name: "explicit by property without page context", change: func(r *QueryRequest) { r.Aggregation = AggregationProperty }},
		{name: "inclusive same-day query", change: func(r *QueryRequest) { r.Dates.End = r.Dates.Start }},
		{name: "reversed date range", change: func(r *QueryRequest) { r.Dates.Start, r.Dates.End = r.Dates.End, r.Dates.Start }, wantErr: core.ErrSearchConsoleContract},
		{name: "explicit property aggregation with page dimension", change: func(r *QueryRequest) {
			r.Dimensions = Dimensions{First: DimensionPage}
			r.Aggregation = AggregationProperty
		}, wantErr: core.ErrSearchConsoleContract},
		{name: "explicit page aggregation with page filter", change: func(r *QueryRequest) { r.Filters.Page, _ = ParsePageURL(pageFixture); r.Aggregation = AggregationPage }, wantErr: core.ErrSearchConsoleContract},
		{name: "fresh date probe cannot claim filtered completeness", change: func(r *QueryRequest) {
			r.Dimensions = Dimensions{First: DimensionDate}
			r.State = DataAll
			r.Filters.Query, _ = ParseQueryText(queryFixture)
		}, wantErr: core.ErrSearchConsoleContract},
		{name: "second slot without first", change: func(r *QueryRequest) { r.Dimensions = Dimensions{Second: DimensionPage} }, wantErr: core.ErrSearchConsoleContract},
		{name: "repeated query dimension", change: func(r *QueryRequest) { r.Dimensions = Dimensions{DimensionQuery, DimensionQuery} }, wantErr: core.ErrSearchConsoleContract},
		{name: "mixed date and query unsupported", change: func(r *QueryRequest) { r.Dimensions = Dimensions{DimensionDate, DimensionQuery} }, wantErr: core.ErrSearchConsoleContract},
		{name: "future dimension refused", change: func(r *QueryRequest) { r.Dimensions.First = Dimension(255) }, wantErr: core.ErrSearchConsoleContract},
		{name: "future search type refused", change: func(r *QueryRequest) { r.Type = SearchType(255) }, wantErr: core.ErrSearchConsoleContract},
		{name: "future aggregation refused", change: func(r *QueryRequest) { r.Aggregation = Aggregation(255) }, wantErr: core.ErrSearchConsoleContract},
		{name: "future data state refused", change: func(r *QueryRequest) { r.State = DataState(255) }, wantErr: core.ErrSearchConsoleContract},
		{name: "zero request page limit", change: func(r *QueryRequest) { r.Page.Limit = 0 }, wantErr: core.ErrSearchConsoleContract},
		{name: "one over request page limit", change: func(r *QueryRequest) { r.Page.Limit = PageMaximum + 1 }, wantErr: core.ErrSearchConsoleContract},
		{name: "one over offset field", change: func(r *QueryRequest) { r.Page.Offset = OffsetMaximum + 1 }, wantErr: core.ErrSearchConsoleContract},
		{name: "retry-capable POST policy refused", change: func(r *QueryRequest) { r.Policy.Retry.MaximumAttempts = 2 }, wantErr: core.ErrSearchConsoleContract},
		{name: "redirect-capable transport refused", change: func(r *QueryRequest) { r.Policy.Redirect.Mode = exchange.RedirectSameOrigin }, wantErr: core.ErrSearchConsoleContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := fixtureRequest(t)
			tc.change(&request)
			if got := request.Validate(); !errors.Is(got, tc.wantErr) {
				t.Fatalf("request validation = %v, want %v", got, tc.wantErr)
			}
		})
	}
}
