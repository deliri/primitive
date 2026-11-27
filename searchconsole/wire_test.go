package searchconsole

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestSearchConsolePaginationBoundaries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		window   PageWindow
		rows     int
		wantNext uint32
		wantErr  error
	}{
		{name: "first page returns requested row count", window: PageWindow{Limit: 2}, rows: 2, wantNext: 2},
		{name: "short page still proposes next observed row", window: PageWindow{Limit: 2, Offset: 10}, rows: 1, wantNext: 11},
		{name: "empty page terminates after nonzero offset", window: PageWindow{Limit: 2, Offset: 10}},
		{name: "one row with lowest limit", window: PageWindow{Limit: 1}, rows: 1, wantNext: 1},
		{name: "response exceeds requested one-row window", window: PageWindow{Limit: 1}, rows: 2, wantErr: core.ErrSearchConsoleResponse},
		{name: "response exceeds requested two-row window", window: PageWindow{Limit: 2}, rows: 3, wantErr: core.ErrSearchConsoleResponse},
		{name: "one below maximum page rows", window: PageWindow{Limit: PageMaximum - 1}, rows: PageMaximum - 1, wantNext: PageMaximum - 1},
		{name: "exact maximum page rows", window: PageWindow{Limit: PageMaximum}, rows: PageMaximum, wantNext: PageMaximum},
		{name: "one over maximum page rows", window: PageWindow{Limit: PageMaximum}, rows: PageMaximum + 1, wantErr: core.ErrSearchConsoleResponse},
		{name: "exact greatest offset can still be proposed", window: PageWindow{Limit: 1, Offset: OffsetMaximum - 1}, rows: 1, wantNext: OffsetMaximum},
		{name: "greatest offset can return a row without wrap", window: PageWindow{Limit: 1, Offset: OffsetMaximum}, rows: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := fixtureRequest(t)
			r.Page = tc.window
			r.Dimensions = Dimensions{First: DimensionQuery}
			rows := make([]rowWire, 0, tc.rows)
			for i := range tc.rows {
				rows = append(rows, rowFixture(fmt.Sprintf("query %d", i)))
			}
			page, err := decodePage(fixtureJSON(t, pageWire{Rows: rows, Aggregation: "byProperty"}), r)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("decode page error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if !page.isZero() {
					t.Fatalf("refused page has %d rows and offset %d, want zero", len(page.Rows), page.NextOffset)
				}
				return
			}
			if len(page.Rows) != tc.rows || page.NextOffset != tc.wantNext || page.Validate() != nil {
				t.Fatalf("accepted page = %d rows and offset %d, want %d and %d", len(page.Rows), page.NextOffset, tc.rows, tc.wantNext)
			}
		})
	}
}

func TestSearchConsoleDateCoverageAcrossDelay(t *testing.T) {
	t.Parallel()
	r := fixtureRequest(t)
	r.Dimensions = Dimensions{First: DimensionDate}
	r.State = DataAll
	raw := fixtureJSON(t, pageWire{
		Rows: []rowWire{rowFixture("2026-11-01"), rowFixture("2026-11-03")}, Aggregation: "byProperty",
		Metadata: &metadataWire{FirstIncomplete: "2026-11-02"},
	})
	page, err := decodePage(raw, r)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		date    string
		wantErr error
	}{
		{date: "2026-10-31", wantErr: core.ErrSearchConsoleCoverageUnknown},
		{date: "2026-11-01"},
		{date: "2026-11-02", wantErr: core.ErrSearchConsoleNotReady},
		{date: "2026-11-03", wantErr: core.ErrSearchConsoleNotReady},
		{date: "2026-11-04", wantErr: core.ErrSearchConsoleNotReady},
		{date: "2026-11-05", wantErr: core.ErrSearchConsoleContract},
	} {
		t.Run(tc.date, func(t *testing.T) {
			t.Parallel()
			day, err := temporal.ParseCivilDate(tc.date)
			if err != nil {
				t.Fatal(err)
			}
			if got := page.FinalDate(day); !errors.Is(got, tc.wantErr) {
				t.Fatalf("finality for %s = %v, want %v", tc.date, got, tc.wantErr)
			}
		})
	}
}

func TestSearchConsoleHostilePageProjection(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		makeWire func(*QueryRequest, *pageWire)
		raw      []byte
		wantErr  error
	}{
		{name: "valid query row with all four metrics"},
		{name: "response total lacks aggregation", makeWire: func(_ *QueryRequest, w *pageWire) { w.Aggregation = "" }, wantErr: core.ErrSearchConsoleResponse},
		{name: "new aggregation enum cannot pass as auto", makeWire: func(_ *QueryRequest, w *pageWire) { w.Aggregation = "byNewsShowcasePanel" }, wantErr: core.ErrSearchConsoleResponse},
		{name: "empty response with unsupported observed aggregation", makeWire: func(_ *QueryRequest, w *pageWire) { w.Rows = nil; w.Aggregation = "byNewsShowcasePanel" }, wantErr: core.ErrSearchConsoleResponse},
		{name: "requested explicit aggregation must match", makeWire: func(r *QueryRequest, w *pageWire) { r.Aggregation = AggregationPage }, wantErr: core.ErrSearchConsoleResponse},
		{name: "page context cannot return by-property aggregation", makeWire: func(r *QueryRequest, w *pageWire) {
			r.Dimensions = Dimensions{First: DimensionPage}
			w.Rows[0].Keys = []string{pageFixture}
		}, wantErr: core.ErrSearchConsoleResponse},
		{name: "dimension key missing", makeWire: func(_ *QueryRequest, w *pageWire) { w.Rows[0].Keys = nil }, wantErr: core.ErrSearchConsoleResponse},
		{name: "extra key cannot index outside supported dimensions", makeWire: func(r *QueryRequest, w *pageWire) {
			r.Page.Limit = 3
			r.Dimensions = Dimensions{DimensionPage, DimensionQuery}
			w.Aggregation = "byPage"
			w.Rows[0].Keys = []string{pageFixture, queryFixture, "extraneous"}
		}, wantErr: core.ErrSearchConsoleResponse},
		{name: "dimension keys transposed", makeWire: func(r *QueryRequest, w *pageWire) {
			r.Dimensions = Dimensions{DimensionPage, DimensionQuery}
			w.Aggregation = "byPage"
			w.Rows[0].Keys = []string{queryFixture, pageFixture}
		}, wantErr: core.ErrSearchConsoleResponse},
		{name: "exact query filter must match case", makeWire: func(r *QueryRequest, w *pageWire) { r.Filters.Query, _ = ParseQueryText(strings.ToUpper(queryFixture)) }, wantErr: core.ErrSearchConsoleResponse},
		{name: "exact page filter must match request", makeWire: func(r *QueryRequest, w *pageWire) {
			r.Dimensions = Dimensions{First: DimensionPage}
			r.Filters.Page, _ = ParsePageURL(pageFixture)
			w.Aggregation = "byPage"
			w.Rows[0].Keys = []string{"https://fixture.example/other"}
		}, wantErr: core.ErrSearchConsoleResponse},
		{name: "duplicate grouping facts cannot count twice", makeWire: func(_ *QueryRequest, w *pageWire) { w.Rows = append(w.Rows, rowFixture(queryFixture)) }, wantErr: core.ErrSearchConsoleResponse},
		{name: "ungrouped aggregate cannot return multiple totals", makeWire: func(r *QueryRequest, w *pageWire) {
			r.Dimensions = Dimensions{}
			w.Rows = []rowWire{rowFixture(), rowFixture()}
		}, wantErr: core.ErrSearchConsoleResponse},
		{name: "null metric cannot become zero", makeWire: func(_ *QueryRequest, w *pageWire) { w.Rows[0].Clicks = nil }, wantErr: core.ErrSearchConsoleResponse},
		{name: "negative impressions", makeWire: func(_ *QueryRequest, w *pageWire) { *w.Rows[0].Impressions = -1 }, wantErr: core.ErrSearchConsoleResponse},
		{name: "negative clicks", makeWire: func(_ *QueryRequest, w *pageWire) { *w.Rows[0].Clicks = -1 }, wantErr: core.ErrSearchConsoleResponse},
		{name: "CTR above 100 percent", makeWire: func(_ *QueryRequest, w *pageWire) { *w.Rows[0].Rate = ClickRate(math.Nextafter(1, 2)) }, wantErr: core.ErrSearchConsoleResponse},
		{name: "CTR at 100 percent", makeWire: func(_ *QueryRequest, w *pageWire) { *w.Rows[0].Rate = 1 }},
		{name: "CTR at zero", makeWire: func(_ *QueryRequest, w *pageWire) { *w.Rows[0].Rate = 0 }},
		{name: "position below the first search result", makeWire: func(_ *QueryRequest, w *pageWire) { *w.Rows[0].Position = .99 }, wantErr: core.ErrSearchConsoleResponse},
		{name: "position of the first search result", makeWire: func(_ *QueryRequest, w *pageWire) { *w.Rows[0].Position = 1 }},
		{name: "position zero allowed only with no impressions", makeWire: func(_ *QueryRequest, w *pageWire) {
			*w.Rows[0].Position = 0
			*w.Rows[0].Impressions = 0
			*w.Rows[0].Rate = 0
			*w.Rows[0].Clicks = 0
		}},
		{name: "position zero rejected with impressions", makeWire: func(_ *QueryRequest, w *pageWire) { *w.Rows[0].Position = 0 }, wantErr: core.ErrSearchConsoleResponse},
		{name: "date row outside inclusive window", makeWire: func(r *QueryRequest, w *pageWire) {
			r.Dimensions = Dimensions{First: DimensionDate}
			w.Rows[0].Keys = []string{"2026-10-27"}
		}, wantErr: core.ErrSearchConsoleResponse},
		{name: "valid civil date at start", makeWire: func(r *QueryRequest, w *pageWire) {
			r.Dimensions = Dimensions{First: DimensionDate}
			w.Rows[0].Keys = []string{"2026-10-28"}
		}},
		{name: "valid civil date at end", makeWire: func(r *QueryRequest, w *pageWire) {
			r.Dimensions = Dimensions{First: DimensionDate}
			w.Rows[0].Keys = []string{"2026-11-04"}
		}},
		{name: "unparseable incomplete date", makeWire: func(r *QueryRequest, w *pageWire) {
			r.State = DataAll
			r.Dimensions = Dimensions{First: DimensionDate}
			w.Rows[0].Keys = []string{"2026-11-04"}
			w.Metadata = &metadataWire{FirstIncomplete: "2026-02-30"}
		}, wantErr: core.ErrSearchConsoleResponse},
		{name: "cutoff has no place in a final-only result", makeWire: func(r *QueryRequest, w *pageWire) {
			r.Dimensions = Dimensions{First: DimensionDate}
			w.Rows[0].Keys = []string{"2026-11-04"}
			w.Metadata = &metadataWire{FirstIncomplete: "2026-11-04"}
		}, wantErr: core.ErrSearchConsoleResponse},
		{name: "cutoff before requested fresh date range", makeWire: func(r *QueryRequest, w *pageWire) {
			r.State = DataAll
			r.Dimensions = Dimensions{First: DimensionDate}
			w.Rows[0].Keys = []string{"2026-11-04"}
			w.Metadata = &metadataWire{FirstIncomplete: "2026-10-27"}
		}, wantErr: core.ErrSearchConsoleResponse},
		{name: "cutoff on requested fresh end date", makeWire: func(r *QueryRequest, w *pageWire) {
			r.State = DataAll
			r.Dimensions = Dimensions{First: DimensionDate}
			w.Rows[0].Keys = []string{"2026-11-04"}
			w.Metadata = &metadataWire{FirstIncomplete: "2026-11-04"}
		}},
		{name: "null top-level document", raw: []byte("null"), wantErr: core.ErrJSONContract},
		{name: "scalar top-level document", raw: []byte("17"), wantErr: core.ErrJSONContract},
		{name: "duplicate folded field", raw: []byte(`{"rows":[],"ROWS":[]}`), wantErr: core.ErrJSONContract},
		{name: "unknown field", raw: []byte(`{"resultIsFinal":true}`), wantErr: core.ErrJSONContract},
		{name: "trailing object", raw: []byte("{}{}"), wantErr: core.ErrJSONContract},
		{name: "wrong rows type", raw: []byte(`{"rows":17}`), wantErr: core.ErrJSONContract},
		{name: "numeric overflow", raw: []byte(`{"rows":[{"keys":["x"],"clicks":1e999,"impressions":0,"ctr":0,"position":0}]}`), wantErr: core.ErrJSONContract},
		{name: "reordered valid explicit empty response", raw: []byte(`{"responseAggregationType":"byProperty","rows":[]}`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := fixtureRequest(t)
			r.Dimensions = Dimensions{First: DimensionQuery}
			w := pageWire{Rows: []rowWire{rowFixture(queryFixture)}, Aggregation: "byProperty"}
			if tc.makeWire != nil {
				tc.makeWire(&r, &w)
			}
			body := tc.raw
			if body == nil {
				body = fixtureJSON(t, w)
			}
			got, err := decodePage(body, r)
			if !errors.Is(err, tc.wantErr) || tc.wantErr != nil && !got.isZero() {
				t.Fatalf("decoded %d rows and error %v, want %v with zero result on refusal", len(got.Rows), err, tc.wantErr)
			}
			if err == nil && got.Validate() != nil {
				t.Fatalf("accepted page failed validation: %+v", got)
			}
		})
	}
}

func TestSearchConsoleProviderByteBoundaries(t *testing.T) {
	t.Parallel()
	r := fixtureRequest(t)
	validSite := fixtureJSON(t, siteWire{Property: propertyFixture, Permission: PermissionFull.String()})
	validPage := fixtureJSON(t, pageWire{Rows: []rowWire{rowFixture()}, Aggregation: "byProperty"})
	for _, tc := range []struct {
		name  string
		max   int
		raw   []byte
		check func([]byte) error
	}{
		{name: "analytics", max: responseMaximum, raw: validPage, check: func(b []byte) error { _, err := decodePage(b, r); return err }},
		{name: "sites", max: siteMaximum, raw: validSite, check: func(b []byte) error { _, err := decodeSite(b, r.Property); return err }},
	} {
		for _, delta := range []int{-1, 0, 1} {
			t.Run(fmt.Sprintf("%s:%+d", tc.name, delta), func(t *testing.T) {
				t.Parallel()
				raw := append(bytes.Repeat([]byte{' '}, tc.max-len(tc.raw)+delta), tc.raw...)
				var wantErr error
				if delta > 0 {
					wantErr = core.ErrExchangeBodyLimit
				}
				if err := tc.check(raw); !errors.Is(err, wantErr) {
					t.Fatalf("length %d (%+d) error = %v, want %v", len(raw), delta, err, wantErr)
				}
			})
		}
	}
}

func samePage(a, b QueryPage) bool {
	return slices.Equal(a.Rows, b.Rows) && a.Dates == b.Dates && a.Dimensions == b.Dimensions &&
		a.State == b.State && a.Aggregation == b.Aggregation && a.FirstIncomplete == b.FirstIncomplete && a.NextOffset == b.NextOffset
}
