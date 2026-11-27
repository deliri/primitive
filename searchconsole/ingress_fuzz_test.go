package searchconsole

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"encoding/json/v2"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func FuzzSearchConsoleNominalInputs(f *testing.F) {
	r := fixtureRequest(f)
	page, err := ParsePageURL(pageFixture)
	if err != nil {
		f.Fatal(err)
	}
	query, err := ParseQueryText(queryFixture)
	if err != nil {
		f.Fatal(err)
	}
	for _, seed := range []string{r.Property.String(), page.String(), query.String(), "", "sc-domain:1.2.3.4",
		"sc-domain:-bad.example", "https://user:pass@fixture.example/", "https://fixture.example/#fragment", "bad\x00query",
		strings.Repeat("x", filterMaximum), strings.Repeat("x", filterMaximum+1)} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		property, pErr := ParseProperty(s)
		if pErr == nil {
			replayed, err := ParseProperty(property.String())
			if property.Validate() != nil || property.String() != s || !strings.HasPrefix(s, "sc-domain:") || replayed != property || err != nil {
				t.Fatalf("admitted property = %q, want an exact canonical domain property", property.String())
			}
		} else if property != (Property{}) || !errors.Is(pErr, core.ErrSearchConsoleContract) {
			t.Fatalf("property rejection = %+v/%v, want zero/contract", property, pErr)
		}
		page, uErr := ParsePageURL(s)
		if uErr == nil {
			replayed, err := ParsePageURL(page.String())
			if page.Validate() != nil || page.String() != s || len(s) > filterMaximum || replayed != page || err != nil {
				t.Fatalf("admitted page value changed = %t, want exact bounded URL", page.String() != s)
			}
		} else if page != (PageURL{}) || !errors.Is(uErr, core.ErrSearchConsoleContract) {
			t.Fatalf("page rejection = %+v/%v, want zero/contract", page, uErr)
		}
		query, qErr := ParseQueryText(s)
		if qErr == nil {
			replayed, err := ParseQueryText(query.String())
			if query.Validate() != nil || query.String() != s || len(s) > filterMaximum || replayed != query || err != nil {
				t.Fatalf("admitted query text changed = %t, want exact bounded text", query.String() != s)
			}
		} else if query != (QueryText{}) || !errors.Is(qErr, core.ErrSearchConsoleContract) {
			t.Fatalf("query text rejection = %+v/%v, want zero/contract", query, qErr)
		}
	})
}

func FuzzSearchConsolePageDecoder(f *testing.F) {
	request := fixtureRequest(f)
	layouts := [...]Dimensions{{}, {First: DimensionDate}, {First: DimensionPage}, {First: DimensionQuery}, {DimensionPage, DimensionQuery}, {DimensionQuery, DimensionPage}}
	for i, dimensions := range layouts {
		var keys []string
		for _, dimension := range []Dimension{dimensions.First, dimensions.Second} {
			switch dimension {
			case DimensionDate:
				keys = append(keys, "2026-11-01")
			case DimensionPage:
				keys = append(keys, pageFixture)
			case DimensionQuery:
				keys = append(keys, queryFixture)
			}
		}
		f.Add(uint8(i), fixtureJSON(f, pageWire{Rows: []rowWire{rowFixture(keys...)}, Aggregation: "auto"}))
	}
	f.Add(uint8(1), fixtureJSON(f, pageWire{
		Rows: []rowWire{rowFixture("2026-11-01")}, Aggregation: "byProperty", Metadata: &metadataWire{FirstIncomplete: "2026-11-02"},
	}))
	f.Add(uint8(0), fixtureJSON(f, pageWire{}))
	for _, raw := range []string{"", "null", `{"rows":[`, `{"rows":[],"rows":[]}`, `{"unknown":[]}`, `{"rows":null}`, `{"rows":[]}{ }`} {
		f.Add(uint8(0), []byte(raw))
	}
	f.Fuzz(func(t *testing.T, mode uint8, data []byte) {
		r := request
		r.Dimensions = layouts[int(mode)%len(layouts)]
		r.State = DataAll
		got, err := decodePage(data, r)
		if err != nil {
			if !got.isZero() || !errors.Is(err, core.ErrSearchConsoleResponse) {
				t.Fatalf("wire refusal = %+v/%v, want zero/response", got, err)
			}
			return
		}
		var wire pageWire
		if json.Unmarshal(data, &wire) != nil || got.Validate() != nil || len(data) > responseMaximum || len(got.Rows) != len(wire.Rows) ||
			got.Dimensions != r.Dimensions || got.State != r.State || got.Dates != r.Dates || got.Aggregation.String() != wire.Aggregation ||
			got.NextOffset != uint32(len(wire.Rows)) {
			t.Fatalf("admitted page has %d rows, next %d, want the exact bounded projection of %d wire rows", len(got.Rows), got.NextOffset, len(wire.Rows))
		}
		for i, row := range got.Rows {
			original := wire.Rows[i]
			if original.Clicks == nil || original.Impressions == nil || original.Rate == nil || original.Position == nil ||
				row.Clicks != *original.Clicks || row.Impressions != *original.Impressions || row.Rate != *original.Rate || row.Position != *original.Position {
				t.Fatalf("row %d metrics changed, want exact input facts", i)
			}
			var keys []string
			for _, d := range []Dimension{r.Dimensions.First, r.Dimensions.Second} {
				switch d {
				case DimensionDate:
					text, _ := row.Date.Text()
					keys = append(keys, text)
				case DimensionQuery:
					keys = append(keys, row.Query.String())
				case DimensionPage:
					keys = append(keys, row.Page.String())
				}
			}
			if strings.Join(keys, "\x00") != strings.Join(original.Keys, "\x00") || len(keys) != len(original.Keys) {
				t.Fatalf("row %d projected key count = %d, want exact %d keys in request order", i, len(keys), len(original.Keys))
			}
		}
		if wire.Metadata != nil && wire.Metadata.FirstIncomplete != "" {
			day, dErr := temporal.ParseCivilDate(wire.Metadata.FirstIncomplete)
			if dErr != nil || got.FirstIncomplete != day || !errors.Is(got.FinalDate(day), core.ErrSearchConsoleNotReady) {
				t.Fatalf("provider cutoff = %v, want explicit not-ready at %s", got.FirstIncomplete, wire.Metadata.FirstIncomplete)
			}
		}
		encoded := fixtureJSON(t, wire)
		again, err := decodePage(encoded, r)
		if err != nil || !samePage(got, again) {
			t.Fatalf("canonical replay has %d rows/%v, want equal accepted projection", len(again.Rows), err)
		}
	})
}

func FuzzSearchConsoleSitesDecoder(f *testing.F) {
	property := fixtureRequest(f).Property
	for permission := PermissionRestricted; permission <= PermissionUnverified; permission++ {
		f.Add(fixtureJSON(f, siteWire{Property: property.String(), Permission: permission.String()}))
	}
	f.Add(fixtureJSON(f, siteWire{Property: "sc-domain:foreign.example", Permission: "siteOwner"}))
	f.Add([]byte(`{"siteUrl":null,"permissionLevel":7}`))
	f.Add([]byte(`{"siteUrl":"sample","siteUrl":"duplicate"}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := decodeSite(data, property)
		if err != nil {
			if got != (Site{}) || !errors.Is(err, core.ErrSearchConsoleResponse) && !errors.Is(err, core.ErrSearchConsoleProperty) {
				t.Fatalf("sites refusal = %+v/%v, want zero and typed provider error", got, err)
			}
			return
		}
		var wire siteWire
		if json.Unmarshal(data, &wire) != nil || got.Validate() != nil || got.Property != property || got.Property.String() != wire.Property ||
			got.Permission.String() != wire.Permission || got.Permission == PermissionUnverified || len(data) > siteMaximum {
			t.Fatalf("site recovered = %+v, want exact authorized property grant", got)
		}
		again, err := decodeSite(fixtureJSON(t, wire), property)
		if err != nil || again != got {
			t.Fatalf("canonical Sites replay = %+v/%v, want %+v", again, err, got)
		}
	})
}

func FuzzSearchConsoleFaultDecoder(f *testing.F) {
	f.Add(true, fixtureJSON(f, faultWire{Error: &faultDetail{Code: 403, Reasons: []faultReason{{Reason: "quotaExceeded"}, {Reason: "forbidden"}}}}))
	f.Add(true, fixtureJSON(f, faultWire{Error: &faultDetail{Code: 403, Reasons: []faultReason{{Reason: "forbidden"}}}}))
	f.Add(false, fixtureJSON(f, faultWire{Error: &faultDetail{Code: 404, Reasons: []faultReason{{Reason: "notFound"}}}}))
	f.Add(true, []byte(`{"error":null}`))
	f.Fuzz(func(t *testing.T, forbidden bool, data []byte) {
		status := http.StatusNotFound
		if forbidden {
			status = http.StatusForbidden
		}
		got := decodeFault(data, status)
		if got == faultUnclassified {
			return
		}
		var wire faultWire
		if len(data) > failureMaximum || json.Unmarshal(data, &wire) != nil || wire.Error == nil || wire.Error.Code != status || len(wire.Error.Reasons) == 0 {
			t.Fatalf("classified status %d with %d bytes, want a bounded error bound to HTTP status", status, len(data))
		}
		again := decodeFault(fixtureJSON(t, wire), status)
		if again != got || decodeFault(data, http.StatusUnauthorized) != faultUnclassified ||
			status == http.StatusNotFound && got != faultProperty {
			t.Fatalf("classification = %v, canonical replay %v, want %v for HTTP %d only", got, again, got, status)
		}
	})
}
