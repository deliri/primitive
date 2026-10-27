package cloudbilling

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/currency"
)

func fixtureLimits(t testing.TB) PageLimits {
	t.Helper()
	maximum, err := core.NewByteCount(1 << 20)
	if err != nil {
		t.Fatal(err)
	}
	token, err := core.NewByteCount(64)
	if err != nil {
		t.Fatal(err)
	}
	limits := core.ExtensibleJSONLimits()
	limits.DocumentMaximumBytes = maximum
	limits.ArrayItemMaximum = 8
	limits.NestingDepthMaximum = 16
	limits.ObjectFieldMaximum = 32
	return PageLimits{JSON: limits, TokenMaximumBytes: token}
}

func fixturePage() Page {
	return Page{SKUs: []SKU{{Name: "services/service/skus/sku", ID: "sku", Description: "provider facts", Provider: "Google", Category: Category{Service: "service", Group: "group", Usage: "OnDemand"}, Geo: Geography{Kind: GeographyRegional, Regions: []RegionName{"us-central1"}}, Pricing: []PricingInfo{{Effective: "2026-01-01T00:00:00Z", Expression: PricingExpression{Unit: "count", Base: "count", Factor: 1, Tiers: []TierRate{{Price: Money{Currency: currency.CodeUSD, Nanos: 300}}}}}}}}}
}

func encodedFixture(t testing.TB, page Page) []byte {
	t.Helper()
	if err := page.Validate(); err != nil {
		t.Fatal(err)
	}
	data, err := core.MarshalCanonicalJSONDocument(page)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestMoneyExactSignedProviderContract(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		value   Money
		wantErr bool
	}{
		{name: "zero USD", value: Money{Currency: currency.CodeUSD}},
		{name: "positive fraction", value: Money{Currency: currency.CodeUSD, Nanos: 1}},
		{name: "negative fraction at zero units", value: Money{Currency: currency.CodeUSD, Nanos: -1}},
		{name: "positive units", value: Money{Currency: currency.CodeUSD, Units: 1}},
		{name: "negative units", value: Money{Currency: currency.CodeUSD, Units: -1}},
		{name: "positive units and fraction", value: Money{Currency: currency.CodeUSD, Units: 1, Nanos: 1}},
		{name: "negative units and fraction", value: Money{Currency: currency.CodeUSD, Units: -1, Nanos: -1}},
		{name: "CAD exact nanos despite cents", value: Money{Currency: currency.CodeCAD, Nanos: 3}},
		{name: "JPY exact nanos despite no minor units", value: Money{Currency: currency.CodeJPY, Nanos: 3}},
		{name: "CLF exact nanos despite four minor digits", value: Money{Currency: currency.CodeCLF, Nanos: 3}},
		{name: "whole signed maximum", value: Money{Currency: currency.CodeUSD, Units: math.MaxInt64}},
		{name: "whole signed minimum", value: Money{Currency: currency.CodeUSD, Units: math.MinInt64}},
		{name: "fraction positive maximum below", value: Money{Currency: currency.CodeUSD, Nanos: 999999998}},
		{name: "fraction positive maximum exact", value: Money{Currency: currency.CodeUSD, Nanos: 999999999}},
		{name: "fraction positive maximum above", value: Money{Currency: currency.CodeUSD, Nanos: 1000000000}, wantErr: true},
		{name: "fraction negative minimum above", value: Money{Currency: currency.CodeUSD, Nanos: -999999998}},
		{name: "fraction negative minimum exact", value: Money{Currency: currency.CodeUSD, Nanos: -999999999}},
		{name: "fraction negative minimum below", value: Money{Currency: currency.CodeUSD, Nanos: -1000000000}, wantErr: true},
		{name: "positive whole negative fraction", value: Money{Currency: currency.CodeUSD, Units: 1, Nanos: -1}, wantErr: true},
		{name: "negative whole positive fraction", value: Money{Currency: currency.CodeUSD, Units: -1, Nanos: 1}, wantErr: true},
		{name: "positive maximum negative fraction", value: Money{Currency: currency.CodeUSD, Units: math.MaxInt64, Nanos: -1}, wantErr: true},
		{name: "negative minimum positive fraction", value: Money{Currency: currency.CodeUSD, Units: math.MinInt64, Nanos: 1}, wantErr: true},
		{name: "unknown currency", value: Money{}, wantErr: true},
		{name: "currency after closed domain", value: Money{Currency: currency.Code(255)}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			data, err := tc.value.MarshalJSON()
			if tc.wantErr {
				if !errors.Is(err, core.ErrCloudBillingResponse) || len(data) != 0 {
					t.Fatalf("MarshalJSON() = (%q,%v), want zero and response refusal", data, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var got Money
			if err := got.UnmarshalJSON(data); err != nil || got != tc.value {
				t.Fatalf("money roundtrip = (%+v,%v), want %+v", got, err, tc.value)
			}
		})
	}
}

func TestPageIngressRetainsProviderFactsAndCallerBounds(t *testing.T) {
	t.Parallel()
	base := encodedFixture(t, fixturePage())
	limits := fixtureLimits(t)
	cases := []struct {
		name         string
		mutate       func([]byte) []byte
		changeLimits func(*PageLimits)
		wantErr      bool
	}{
		{name: "canonical provider facts"},
		{name: "whitespace preserves facts", mutate: func(b []byte) []byte { return append(append([]byte(" \n"), b...), '\n') }},
		{name: "empty sku page neutral", mutate: func([]byte) []byte { return encodedFixture(t, Page{SKUs: []SKU{}}) }},
		{name: "all geography arms", mutate: func([]byte) []byte {
			p := fixturePage()
			p.SKUs[0].Geo.Kind = GeographyGlobal
			return encodedFixture(t, p)
		}},
		{name: "unspecified geography is fact not eligible policy", mutate: func([]byte) []byte {
			p := fixturePage()
			p.SKUs[0].Geo.Kind = GeographyUnspecified
			return encodedFixture(t, p)
		}},
		{name: "multi regional fact", mutate: func([]byte) []byte {
			p := fixturePage()
			p.SKUs[0].Geo.Kind = GeographyMultiRegional
			return encodedFixture(t, p)
		}},
		{name: "future resource group remains provider fact", mutate: func([]byte) []byte {
			p := fixturePage()
			p.SKUs[0].Category.Group = "FutureServiceGroup"
			return encodedFixture(t, p)
		}},
		{name: "negative price is SDK fact", mutate: func([]byte) []byte {
			p := fixturePage()
			p.SKUs[0].Pricing[0].Expression.Tiers[0].Price = Money{Currency: currency.CodeUSD, Units: -2, Nanos: -3}
			return encodedFixture(t, p)
		}},
		{name: "non USD price is SDK fact", mutate: func([]byte) []byte {
			p := fixturePage()
			p.SKUs[0].Pricing[0].Expression.Tiers[0].Price.Currency = currency.CodeEUR
			return encodedFixture(t, p)
		}},
		{name: "token requires percent escaping", mutate: func([]byte) []byte { p := fixturePage(); p.Next = "a/+?=&"; return encodedFixture(t, p) }},
		{name: "body limit below extent", changeLimits: func(l *PageLimits) { l.JSON.DocumentMaximumBytes, _ = core.NewByteCount(uint64(len(base) - 1)) }, wantErr: true},
		{name: "body limit exact extent", changeLimits: func(l *PageLimits) { l.JSON.DocumentMaximumBytes, _ = core.NewByteCount(uint64(len(base))) }},
		{name: "body limit above extent", changeLimits: func(l *PageLimits) { l.JSON.DocumentMaximumBytes, _ = core.NewByteCount(uint64(len(base) + 1)) }},
		{name: "token below limit", mutate: func([]byte) []byte {
			p := fixturePage()
			p.Next = PageToken(strings.Repeat("x", 63))
			return encodedFixture(t, p)
		}},
		{name: "token at limit", mutate: func([]byte) []byte {
			p := fixturePage()
			p.Next = PageToken(strings.Repeat("x", 64))
			return encodedFixture(t, p)
		}},
		{name: "token above limit", mutate: func([]byte) []byte {
			p := fixturePage()
			p.Next = PageToken(strings.Repeat("x", 65))
			return encodedFixture(t, p)
		}, wantErr: true},
		{name: "sku count below array limit", mutate: func([]byte) []byte { p := Page{SKUs: make([]SKU, 7)}; return encodedFixture(t, p) }},
		{name: "sku count at array limit", mutate: func([]byte) []byte { p := Page{SKUs: make([]SKU, 8)}; return encodedFixture(t, p) }},
		{name: "sku count above array limit", mutate: func([]byte) []byte { p := Page{SKUs: make([]SKU, 9)}; return encodedFixture(t, p) }, wantErr: true},
		{name: "depth below actual nine containers", changeLimits: func(l *PageLimits) { l.JSON.NestingDepthMaximum = 8 }, wantErr: true},
		{name: "depth at actual nine containers", changeLimits: func(l *PageLimits) { l.JSON.NestingDepthMaximum = 9 }},
		{name: "depth above actual nine containers", changeLimits: func(l *PageLimits) { l.JSON.NestingDepthMaximum = 10 }},
		{name: "object fields below eight SKU fields", changeLimits: func(l *PageLimits) { l.JSON.ObjectFieldMaximum = 7 }, wantErr: true},
		{name: "object fields at eight SKU fields", changeLimits: func(l *PageLimits) { l.JSON.ObjectFieldMaximum = 8 }},
		{name: "object fields above eight SKU fields", changeLimits: func(l *PageLimits) { l.JSON.ObjectFieldMaximum = 9 }},
		{name: "negative nanos at signed int32 minimum refused", mutate: func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"nanos":300`), []byte(`"nanos":-2147483648`), 1)
		}, wantErr: true},
		{name: "nanos beyond signed int32 representation refused", mutate: func(b []byte) []byte { return bytes.Replace(b, []byte(`"nanos":300`), []byte(`"nanos":2147483648`), 1) }, wantErr: true},
		{name: "empty response", mutate: func([]byte) []byte { return nil }, wantErr: true},
		{name: "null response", mutate: func([]byte) []byte { return []byte("null") }, wantErr: true},
		{name: "truncated response", mutate: func(b []byte) []byte { return b[:len(b)-1] }, wantErr: true},
		{name: "second document", mutate: func(b []byte) []byte { return append(b, b...) }, wantErr: true},
		{name: "unknown field", mutate: func(b []byte) []byte { return bytes.Replace(b, []byte("{"), []byte(`{"unknown":1,`), 1) }, wantErr: true},
		{name: "duplicate page token", mutate: func(b []byte) []byte { return bytes.Replace(b, []byte("{"), []byte(`{"nextPageToken":"",`), 1) }, wantErr: true},
		{name: "foreign geography token", mutate: func(b []byte) []byte { return bytes.Replace(b, []byte(`"REGIONAL"`), []byte(`"FOREIGN"`), 1) }, wantErr: true},
		{name: "units overflow signed domain", mutate: func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"units":"0"`), []byte(`"units":"9223372036854775808"`), 1)
		}, wantErr: true},
		{name: "units numeric wire instead of string", mutate: func(b []byte) []byte { return bytes.Replace(b, []byte(`"units":"0"`), []byte(`"units":0`), 1) }, wantErr: true},
		{name: "empty units text is not default zero", mutate: func(b []byte) []byte { return bytes.Replace(b, []byte(`"units":"0"`), []byte(`"units":""`), 1) }, wantErr: true},
		{name: "omitted units retains protobuf default zero", mutate: func(b []byte) []byte { return bytes.Replace(b, []byte(`,"units":"0"`), nil, 1) }},
		{name: "fraction outside signed agreement", mutate: func(b []byte) []byte { return bytes.Replace(b, []byte(`"nanos":300`), []byte(`"nanos":1000000000`), 1) }, wantErr: true},
		{name: "wrong nanos JSON type", mutate: func(b []byte) []byte { return bytes.Replace(b, []byte(`"nanos":300`), []byte(`"nanos":"300"`), 1) }, wantErr: true},
		{name: "unknown currency", mutate: func(b []byte) []byte { return bytes.Replace(b, []byte(`"USD"`), []byte(`"ZZZ"`), 1) }, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			data := slices.Clone(base)
			if tc.mutate != nil {
				data = tc.mutate(data)
				if bytes.Equal(data, base) {
					t.Fatal("requested provider mutation left the source unchanged")
				}
			}
			chosen := limits
			if tc.changeLimits != nil {
				tc.changeLimits(&chosen)
			}
			got, err := DecodePage(data, chosen)
			if tc.wantErr {
				if !errors.Is(err, core.ErrCloudBillingResponse) || got.SKUs != nil || got.Next != "" {
					t.Fatalf("DecodePage()=(%+v,%v), want zero and response refusal", got, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			canonical, err := core.MarshalCanonicalJSONDocument(got)
			if err != nil {
				t.Fatal(err)
			}
			again, err := DecodePage(canonical, limits)
			if err != nil {
				t.Fatal(err)
			}
			second, err := core.MarshalCanonicalJSONDocument(again)
			if err != nil || !bytes.Equal(canonical, second) {
				t.Fatalf("canonical fixed point=(%q,%v), want %q", second, err, canonical)
			}
		})
	}
}

func FuzzPageAndMoneySemanticClosure(f *testing.F) {
	limits := fixtureLimits(f)
	f.Add(encodedFixture(f, fixturePage()))
	f.Add(encodedFixture(f, Page{SKUs: []SKU{}}))
	f.Add([]byte("null"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := DecodePage(data, limits)
		if err != nil {
			if !errors.Is(err, core.ErrCloudBillingResponse) || got.SKUs != nil || got.Next != "" {
				t.Fatalf("rejected page=(%+v,%v), want zero and typed refusal", got, err)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatal(err)
		}
		// The standard decoder independently observes the exact accepted provider
		// document; a canonical fixed point also preserves each signed Money fact.
		var native Page
		if err := json.Unmarshal(data, &native); err != nil {
			t.Fatal(err)
		}
		canonical, err := core.MarshalCanonicalJSONDocument(got)
		if err != nil {
			t.Fatal(err)
		}
		nativeCanonical, err := core.MarshalCanonicalJSONDocument(native)
		if err != nil || !bytes.Equal(canonical, nativeCanonical) {
			t.Fatalf("native projection differs: (%q,%v), want %q", nativeCanonical, err, canonical)
		}
		again, err := DecodePage(canonical, limits)
		if err != nil {
			t.Fatal(err)
		}
		second, err := core.MarshalCanonicalJSONDocument(again)
		if err != nil || !bytes.Equal(canonical, second) {
			t.Fatalf("canonical fixed point=(%q,%v), want %q", second, err, canonical)
		}
	})
}
